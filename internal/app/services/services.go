package services

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"time"

	"github.com/mantonx/viewra/internal/app/config"
	"github.com/mantonx/viewra/internal/app/repositories"
	"github.com/mantonx/viewra/internal/application/enrichment"
	"github.com/mantonx/viewra/internal/application/enrichment/builtin"
	"github.com/mantonx/viewra/internal/application/enrichment/pipeline"
	appHome "github.com/mantonx/viewra/internal/application/home"
	"github.com/mantonx/viewra/internal/application/library/monitor"
	appSearch "github.com/mantonx/viewra/internal/application/search"
	"github.com/mantonx/viewra/internal/application/settings"
	"github.com/mantonx/viewra/internal/application/transcode"
	appTrending "github.com/mantonx/viewra/internal/application/trending"
	domaintranscode "github.com/mantonx/viewra/internal/domain/transcode"

	"github.com/mantonx/viewra/internal/infrastructure/auth"
	"github.com/mantonx/viewra/internal/infrastructure/crypto"
	"github.com/mantonx/viewra/internal/infrastructure/events"
	infraimages "github.com/mantonx/viewra/internal/infrastructure/images"
	"github.com/mantonx/viewra/internal/infrastructure/pathbrowser"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/location"
	"github.com/mantonx/viewra/internal/infrastructure/plugins"
	"github.com/mantonx/viewra/internal/infrastructure/subtitles"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding"
	transcodeconfig "github.com/mantonx/viewra/internal/infrastructure/transcoding/config"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/logging"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/session"
	"github.com/mantonx/viewra/internal/infrastructure/weather"
	"github.com/mantonx/viewra/internal/version"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

// DiskMonitoringRepo defines the repository interface needed for disk monitoring cleanup.
// This matches the inline interface in transcode.PerformDiskMonitoring.
type DiskMonitoringRepo interface {
	GetTotalSize(ctx context.Context) (int64, error)
	ListByLRU(ctx context.Context, limit int) ([]*domaintranscode.TranscodeJob, error)
}

// Services holds all infrastructure and domain services.
// Groups all service implementations for dependency injection.
type Services struct {
	// Image services
	ImageCache       *infraimages.CacheService
	ImageTransformer *infraimages.Transformer

	// Media services
	PathBrowser       *pathbrowser.Service
	TranscodeService  transcoding.Service
	TranscodeQueue    *transcode.Queue
	CleanupService    *transcode.CleanupService
	SessionManager    *session.Manager
	SubtitleConverter *subtitles.Converter

	// Transcode repository (for disk monitoring cleanup tasks)
	TranscodeRepo DiskMonitoringRepo

	// Auth services
	PasswordHasher *auth.PasswordHasher
	TokenService   *auth.TokenService

	// Settings service
	Settings *settings.Service

	// Encryption service for sensitive data
	Encryptor *crypto.Encryptor

	// Event Bus for pub/sub messaging
	EventBus *events.Bus

	// Enrichment pipeline manager
	PipelineManager *pipeline.Manager

	// EnqueueBuffer batches enrichment enqueue operations during scans.
	// This wraps PipelineManager and should be passed to ScanLibraryUseCase.
	EnqueueBuffer *pipeline.EnqueueBuffer

	// Enricher registry for tracking all enrichers
	EnricherRegistry *enrichment.Registry

	// Plugin manager for external plugins
	PluginManager *plugins.Manager

	// Location services for weather context
	LocationRepo   *location.Repository
	WeatherService *weather.Service

	// File system monitoring service
	FileMonitor *monitor.Service

	// Search service for fallback text search when no semantic search plugin is available
	Search *appSearch.Service

	// Home screen service for aggregating widgets
	Home *appHome.Service

	// Trending service for trending media data
	Trending *appTrending.Service
}

// BuildServices creates and initializes all infrastructure services.
// Creates required directories and initializes services with proper error handling.
func BuildServices(
	cfg *config.Config,
	repos *repositories.Repositories,
	db *sql.DB,
	dbDriver string,
	logger *slog.Logger,
) (*Services, error) {
	// Create required directories
	if err := ensureDirectory(cfg.Images.CacheDir); err != nil {
		return nil, fmt.Errorf("failed to create image cache directory %s: %w", cfg.Images.CacheDir, err)
	}
	if err := ensureDirectory(cfg.Media.TranscodeOutputDir); err != nil {
		return nil, fmt.Errorf("failed to create transcode output directory %s: %w", cfg.Media.TranscodeOutputDir, err)
	}

	// Initialize core services
	imageCacheService := infraimages.NewCacheService(cfg.Images.CacheDir)
	imageTransformer := infraimages.NewTransformer(imageCacheService)
	browserService := pathbrowser.NewService(cfg.Server.AllowedBasePaths, cfg.Server.DefaultBasePath)
	eventBus := events.NewBus(10000, logger)

	// Initialize transcode services
	transcodeService, transcodeQueue, cleanupService, sessionManager, transcodeConfig := initTranscodeServices(cfg, repos, logger)

	// Initialize auth services
	passwordHasher, tokenService, err := initAuthServices(cfg, logger)
	if err != nil {
		return nil, err
	}

	// Initialize encryption and settings
	encryptor := initEncryptor(cfg, logger)
	settingsService := settings.NewService(repos.SystemSettings, repos.UserSettings, encryptor)
	if cfg.SystemProfile != nil {
		settingsService.SetSystemProfile(cfg.SystemProfile)
	}

	// Wire settings to session manager for dynamic config
	if sessionManager != nil && transcodeConfig != nil {
		configProvider := transcode.NewSettingsConfigProvider(settingsService, transcodeConfig)
		sessionManager.SetConfigProvider(configProvider)
		sessionManager.SetPublisher(eventBus)
	}

	// Initialize subtitle converter
	subtitleConverter := subtitles.NewConverter(cfg.Media.TranscodeOutputDir)

	// Initialize enrichment pipeline
	pipelineManager, enqueueBuffer, enricherRegistry := initEnrichmentPipeline(cfg, repos, imageTransformer, eventBus, logger)

	// Initialize location and weather services
	locationRepo := location.NewRepository(db, dbDriver)
	weatherService := weather.NewService(logger.With("component", "weather"))

	// Initialize plugin manager
	pluginManager := initPluginManager(cfg, repos, db, dbDriver, locationRepo, weatherService, eventBus, settingsService, logger)

	// Initialize file monitor service
	fileMonitor := monitor.NewService(repos.Library, repos.Media, pipelineManager, eventBus, logger.With("component", "file-monitor"))

	// Initialize search service for fallback text search
	searchService := appSearch.NewService(repos.Search, logger.With("component", "search"))

	return &Services{
		ImageCache:        imageCacheService,
		ImageTransformer:  imageTransformer,
		PathBrowser:       browserService,
		TranscodeService:  transcodeService,
		TranscodeQueue:    transcodeQueue,
		CleanupService:    cleanupService,
		SessionManager:    sessionManager,
		SubtitleConverter: subtitleConverter,
		TranscodeRepo:     repos.Transcode,
		PasswordHasher:    passwordHasher,
		TokenService:      tokenService,
		Settings:          settingsService,
		Encryptor:         encryptor,
		EventBus:          eventBus,
		PipelineManager:   pipelineManager,
		EnqueueBuffer:     enqueueBuffer,
		EnricherRegistry:  enricherRegistry,
		PluginManager:     pluginManager,
		LocationRepo:      locationRepo,
		WeatherService:    weatherService,
		FileMonitor:       fileMonitor,
		Search:            searchService,
	}, nil
}

// initTranscodeServices initializes all transcoding-related services.
func initTranscodeServices(
	cfg *config.Config,
	repos *repositories.Repositories,
	logger *slog.Logger,
) (transcoding.Service, *transcode.Queue, *transcode.CleanupService, *session.Manager, *transcodeconfig.TranscodeConfig) {
	// Initialize transcode service
	transcodeService, err := transcoding.NewService(repos.Transcode, logger)
	if err != nil {
		logger.Error("Failed to initialize transcode service", "error", err)
		transcodeService = nil
	}

	// Initialize transcode queue
	var transcodeQueue *transcode.Queue
	if transcodeService != nil {
		queueConfig := &transcode.QueueConfig{
			WorkerCount:   cfg.Media.TranscodeWorkers,
			PollInterval:  cfg.Media.TranscodePollInterval,
			IdleTimeout:   cfg.Media.TranscodeIdleTimeout,
			OutputBaseDir: cfg.Media.TranscodeOutputDir,
			MediaFileGetter: func(ctx context.Context, mediaID int64) (string, error) {
				media, err := repos.Media.GetByID(ctx, mediaID)
				if err != nil {
					return "", err
				}
				return media.FilePath, nil
			},
		}
		transcodeQueue = transcode.NewQueue(queueConfig, repos.Transcode, transcodeService, logger)
	}

	// Initialize cleanup service
	var cleanupService *transcode.CleanupService
	if repos.Transcode != nil {
		cleanupService = transcode.NewCleanupService(repos.Transcode, cfg.Media.TranscodeOutputDir)
	}

	// Initialize session manager with transcode config
	var transcodeConfig *transcodeconfig.TranscodeConfig
	if cfg.SystemProfile != nil {
		settings := cfg.SystemProfile.Calculate()
		transcodeConfig = transcodeconfig.DefaultFromProfile(settings.HardwareAccel, nil)
		logger.Info("Using profile-based transcode config", "hardware_accel", settings.HardwareAccel)
	} else {
		transcodeConfig = transcodeconfig.Default()
	}

	sessionManager := session.NewManager(&session.ManagerConfig{
		FFmpegPaths:                transcodeConfig.FFmpegPaths,
		HardwareAccel:              string(transcodeConfig.HardwareAccel),
		HardwareDevice:             transcodeConfig.HardwareDevice,
		OutputBaseDir:              transcodeConfig.OutputBaseDir,
		MinFreeDiskGB:              transcodeConfig.MinFreeDiskGB,
		MaxCPUPercent:              transcodeConfig.MaxCPUPercent,
		MaxMemoryMB:                transcodeConfig.MaxMemoryMB,
		ProcessGroupKill:           transcodeConfig.ProcessGroupKill,
		ToneMappingEnabled:         transcodeConfig.ToneMappingEnabled,
		ToneMappingAlgorithm:       transcodeConfig.ToneMappingAlgorithm,
		ToneMappingBackend:         transcodeConfig.ToneMappingBackend,
		LibPlaceboPeakDetect:       transcodeConfig.LibPlaceboPeakDetect,
		LibPlaceboContrastRecovery: transcodeConfig.LibPlaceboContrastRecovery,
	}, logger)

	// Initialize FFmpeg log store
	if transcodeConfig.FFmpegLogEnabled {
		logRetention := time.Duration(transcodeConfig.FFmpegLogRetentionHours) * time.Hour
		logStore, err := logging.NewFFmpegLogStore(cfg.Media.TranscodeOutputDir, logRetention, logger)
		if err != nil {
			logger.Warn("Failed to initialize FFmpeg log store", "error", err)
		} else {
			sessionManager.SetLogStore(logStore)
			logger.Info("FFmpeg log store initialized", "retention_hours", transcodeConfig.FFmpegLogRetentionHours)
		}
	}

	// Cleanup orphaned directories and start periodic cleanup
	if cleanedCount, freedBytes, err := sessionManager.CleanupOrphanedOutputDirs(cfg.Media.TranscodeOutputDir); err != nil {
		logger.Warn("Failed to clean orphaned transcode directories", "error", err)
	} else if cleanedCount > 0 {
		logger.Info("Cleaned orphaned transcode directories", "count", cleanedCount, "bytes_freed", freedBytes)
	}

	_ = sessionManager.StartPeriodicCleanup(15*time.Second, 30*time.Second, 5*time.Minute, 1*time.Hour, cfg.Media.TranscodeOutputDir)
	sessionManager.WarmupGPU()

	return transcodeService, transcodeQueue, cleanupService, sessionManager, transcodeConfig
}

// initAuthServices initializes authentication services.
func initAuthServices(cfg *config.Config, logger *slog.Logger) (*auth.PasswordHasher, *auth.TokenService, error) {
	passwordHasher := auth.NewPasswordHasher(nil)

	jwtSecret := []byte(cfg.Auth.JWTSecret)
	if len(jwtSecret) == 0 {
		var err error
		jwtSecret, err = auth.GenerateSecret()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to generate JWT secret: %w", err)
		}
		logger.Warn("Generated temporary JWT secret for development",
			"hint", "Set JWT_SECRET env var for persistence")
	}

	tokenService := auth.NewTokenService(&auth.TokenConfig{
		Secret:          jwtSecret,
		AccessTokenTTL:  cfg.Auth.AccessTokenTTL,
		RefreshTokenTTL: cfg.Auth.RefreshTokenTTL,
		Issuer:          "viewra",
	})

	return passwordHasher, tokenService, nil
}

// initEncryptor initializes the encryption service.
func initEncryptor(cfg *config.Config, logger *slog.Logger) *crypto.Encryptor {
	encryptor, err := crypto.NewEncryptor(cfg.DataDir)
	if err != nil {
		logger.Warn("Failed to initialize encryption", "error", err)
		return nil
	}
	logger.Info("Encryption initialized for sensitive settings")
	return encryptor
}

// initEnrichmentPipeline initializes the enrichment pipeline and related services.
func initEnrichmentPipeline(
	cfg *config.Config,
	repos *repositories.Repositories,
	imageTransformer *infraimages.Transformer,
	eventBus *events.Bus,
	logger *slog.Logger,
) (*pipeline.Manager, *pipeline.EnqueueBuffer, *enrichment.Registry) {
	pipelineLogger := logger.With("component", "enrichment-pipeline")

	// Create image downloader and metadata extractor.
	// Rate limit is set higher (20/sec) because image CDNs (e.g., BunnyCDN for TMDB)
	// can handle high request rates. Burst of 20 allows processing multiple images
	// per job without rate limiter context timeouts.
	imageDownloader := infraimages.NewDownloader(infraimages.DownloaderConfig{
		CacheDir:  cfg.Images.CacheDir,
		RateLimit: 20.0,
		RateBurst: 20,
		Logger:    logger.With("component", "image-downloader"),
	})
	metadataExtractor := infraimages.NewMetadataExtractor()
	metadataAdapter := &metadataExtractorAdapter{extractor: metadataExtractor}

	// Create pipeline manager
	pipelineManager := pipeline.NewManager(
		&pipeline.Deps{
			QueueRepo:          repos.EnrichmentQueue,
			StatusRepo:         repos.EnrichmentStatus,
			PipelineRepo:       repos.EnrichmentPipeline,
			ExternalIDRepo:     repos.EnrichmentExternalID,
			MetadataSourceRepo: repos.EnrichmentMetadataSource,
			MediaRepo:          repos.Media,
			ImageRepo:          repos.Image,
			EventBus:           eventBus,
			Logger:             pipelineLogger,
			MetadataExtractor:  metadataAdapter,
			Transformer:        imageTransformer,
			Downloader:         imageDownloader,
			PeopleRepo:         repos.People,
			StudioRepo:         repos.Studios,
			KeywordRepo:        repos.Keywords,
		},
		&pipeline.TypedMediaRepos{Movie: repos.Movie, TV: repos.TV, Music: repos.Music},
	)

	enqueueBuffer := pipeline.NewEnqueueBuffer(pipelineManager, pipelineLogger)

	// Create and register enrichers
	enricherRegistry := enrichment.NewRegistry()
	imageExtractor := infraimages.NewExtractor()

	// Register builtin enrichers with their pipeline positions.
	// These run BEFORE external plugins (position 0 = nfo, position 1 = local-images).
	// External plugins will be added at position 2+ when they register.
	builtinEnrichers := []struct {
		enricher enrichment.Enricher
		position int
	}{
		{builtin.NewNFOEnricher(), 0},
		{builtin.NewLocalImagesEnricher(imageExtractor, logger), 1},
	}

	ctx := context.Background()
	for _, be := range builtinEnrichers {
		if err := enricherRegistry.RegisterBuiltin(be.enricher); err != nil {
			logger.Warn("Failed to register builtin enricher", "stage", be.enricher.Stage(), "error", err)
		}
		if err := pipelineManager.RegisterBuiltinEnricher(ctx, be.enricher, be.position); err != nil {
			logger.Warn("Failed to register builtin pipeline stage", "stage", be.enricher.Stage(), "error", err)
		}
	}

	return pipelineManager, enqueueBuffer, enricherRegistry
}

// initPluginManager initializes the plugin manager and related services.
func initPluginManager(
	cfg *config.Config,
	repos *repositories.Repositories,
	db *sql.DB,
	dbDriver string,
	locationRepo *location.Repository,
	weatherService *weather.Service,
	eventBus *events.Bus,
	settingsService *settings.Service,
	logger *slog.Logger,
) *plugins.Manager {
	if !cfg.Plugins.Enabled {
		return nil
	}

	pluginLogger := logger.With("component", "plugin-manager")

	// Create host servers
	var hostStorageServer *plugins.HostStorageServer
	if db != nil {
		var err error
		hostStorageServer, err = plugins.NewHostStorageServer(
			plugins.HostStorageConfig{BaseDir: cfg.Plugins.StorageDir},
			db, dbDriver, logger.With("component", "host-storage"),
		)
		if err != nil {
			logger.Warn("Failed to create host storage server", "error", err)
		}
	}

	hostWeatherServer := plugins.NewHostWeatherServer(locationRepo, weatherService, logger.With("component", "host-weather"))

	// Create host ratings server for user preferences access
	hostRatingsServer := plugins.NewHostRatingsServer(repos.Ratings, logger.With("component", "host-ratings"))

	// Create host data server for media querying
	var hostDataServer *plugins.HostDataServer
	if repos.PluginMediaQuerier != nil {
		hostDataServer = plugins.NewHostDataServer(repos.PluginMediaQuerier, logger.With("component", "host-data"))
	}

	// Create plugin manager
	pluginManager, err := plugins.NewManager(plugins.ManagerConfig{
		PluginDir:         cfg.Plugins.Dir,
		StorageDir:        cfg.Plugins.StorageDir,
		HostVersion:       version.Version,
		HostStorageServer: hostStorageServer,
		HostWeatherServer: hostWeatherServer,
		HostRatingsServer: hostRatingsServer,
	}, pluginLogger)
	if err != nil {
		logger.Warn("Failed to create plugin manager", "error", err)
		return nil
	}

	// Set the plugin factory for creating gRPC plugin instances
	pluginManager.SetPluginFactory(plugins.NewFactory())

	// Wire host data server for media querying
	if hostDataServer != nil {
		pluginManager.SetHostDataServer(hostDataServer)
	}

	// Wire event bus
	pluginManager.SetPublisher(eventBus)

	// Wire system profile
	if cfg.SystemProfile != nil {
		sysInfo := &pluginv1.SystemInfo{
			RamBytes:          cfg.SystemProfile.Memory.TotalBytes,
			RamAvailableBytes: cfg.SystemProfile.Memory.AvailableBytes,
			HasGpu:            cfg.SystemProfile.GPU.Available,
			VramBytes:         cfg.SystemProfile.GPU.VRAMBytes,
			CpuCores:          int32(cfg.SystemProfile.CPU.NumPhysical),
			CpuModel:          cfg.SystemProfile.CPU.Model,
			Os:                runtime.GOOS,
			Arch:              runtime.GOARCH,
		}
		if len(cfg.SystemProfile.GPU.DeviceNames) > 0 {
			sysInfo.GpuName = cfg.SystemProfile.GPU.DeviceNames[0]
		}
		pluginManager.SetSystemInfo(sysInfo)
		logger.Info("System info configured for plugins",
			"ram_gb", cfg.SystemProfile.Memory.TotalBytes/(1024*1024*1024),
			"cpu_cores", cfg.SystemProfile.CPU.NumPhysical,
			"has_gpu", cfg.SystemProfile.GPU.Available)
	}

	// Create and set HTTP proxy for plugin routes
	httpProxy := plugins.NewHTTPProxy(
		pluginManager,
		pluginManager.GetRouteRegistry(),
		pluginManager.GetCapabilityRegistry(),
		pluginManager.GetRateLimiter(),
		logger.With("component", "http-proxy"),
	)
	pluginManager.SetHTTPProxy(httpProxy)

	return pluginManager
}

// ensureDirectory creates a directory if it doesn't exist
func ensureDirectory(path string) error {
	return os.MkdirAll(path, 0755)
}

// metadataExtractorAdapter adapts infraimages.MetadataExtractor to pipeline.MetadataExtractor.
// This bridges the infrastructure type (ImageInfo) to the pipeline interface type (ImageMetadata).
type metadataExtractorAdapter struct {
	extractor *infraimages.MetadataExtractor
}

// ExtractMetadata extracts metadata and converts to the pipeline's expected type.
func (a *metadataExtractorAdapter) ExtractMetadata(imagePath string) (*pipeline.ImageMetadata, error) {
	info, err := a.extractor.ExtractMetadata(imagePath)
	if err != nil {
		return nil, err
	}
	return &pipeline.ImageMetadata{
		Width:         info.Width,
		Height:        info.Height,
		FileSizeBytes: info.FileSizeBytes,
		MimeType:      info.MimeType,
		FileHash:      info.FileHash,
	}, nil
}
