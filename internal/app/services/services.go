package services

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/mantonx/viewra/internal/app/config"
	"github.com/mantonx/viewra/internal/app/repositories"
	"github.com/mantonx/viewra/internal/application/enrichment"
	"github.com/mantonx/viewra/internal/application/enrichment/builtin"
	"github.com/mantonx/viewra/internal/application/enrichment/pipeline"
	"github.com/mantonx/viewra/internal/application/settings"
	"github.com/mantonx/viewra/internal/application/transcode"
	domaintranscode "github.com/mantonx/viewra/internal/domain/transcode"
	"github.com/mantonx/viewra/internal/infrastructure/ai/providers"
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

	// Event Bus for pub/sub messaging
	EventBus *events.Bus

	// Enrichment pipeline manager
	PipelineManager *pipeline.Manager

	// Enricher registry for tracking all enrichers
	EnricherRegistry *enrichment.Registry

	// Plugin manager for external plugins
	PluginManager *plugins.Manager

	// Location services for weather context
	LocationRepo   *location.Repository
	WeatherService *weather.Service
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
	// Create image cache directory
	if err := ensureDirectory(cfg.Images.CacheDir); err != nil {
		return nil, fmt.Errorf("failed to create image cache directory %s: %w", cfg.Images.CacheDir, err)
	}

	// Create image services
	imageCacheService := infraimages.NewCacheService(cfg.Images.CacheDir)
	imageTransformer := infraimages.NewTransformer(imageCacheService)

	// Create path browser service
	browserService := pathbrowser.NewService(
		cfg.Server.AllowedBasePaths,
		cfg.Server.DefaultBasePath,
	)

	// Create transcode output directory
	if err := ensureDirectory(cfg.Media.TranscodeOutputDir); err != nil {
		return nil, fmt.Errorf("failed to create transcode output directory %s: %w", cfg.Media.TranscodeOutputDir, err)
	}

	// Initialize transcode service
	transcodeService, err := transcoding.NewService(repos.Transcode, logger)
	if err != nil {
		logger.Error("Failed to initialize transcode service", "error", err)
		// Continue without transcode service - it's not critical for basic functionality
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
				// Get media entity to get file path
				media, err := repos.Media.GetByID(ctx, mediaID)
				if err != nil {
					return "", err
				}
				return media.FilePath, nil
			},
		}
		transcodeQueue = transcode.NewQueue(queueConfig, repos.Transcode, transcodeService, logger)
		// Queue must be started by caller using Start() method
	}

	// Initialize cleanup service
	var cleanupService *transcode.CleanupService
	if repos.Transcode != nil {
		cleanupService = transcode.NewCleanupService(repos.Transcode, cfg.Media.TranscodeOutputDir)
	}

	// Initialize session manager for progressive transcoding
	// Use system profile for hardware acceleration if available
	var transcodeConfig *transcodeconfig.TranscodeConfig
	if cfg.SystemProfile != nil {
		settings := cfg.SystemProfile.Calculate()
		transcodeConfig = transcodeconfig.DefaultFromProfile(settings.HardwareAccel, nil)
		logger.Info("Using profile-based transcode config",
			"hardware_accel", settings.HardwareAccel)
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

	// Initialize FFmpeg log store for debugging (if enabled)
	if transcodeConfig.FFmpegLogEnabled {
		logRetention := time.Duration(transcodeConfig.FFmpegLogRetentionHours) * time.Hour
		logStore, err := logging.NewFFmpegLogStore(cfg.Media.TranscodeOutputDir, logRetention, logger)
		if err != nil {
			logger.Warn("Failed to initialize FFmpeg log store",
				"error", err,
				"note", "FFmpeg logging will be disabled")
		} else {
			sessionManager.SetLogStore(logStore)
			logger.Info("FFmpeg log store initialized",
				"retention_hours", transcodeConfig.FFmpegLogRetentionHours)
		}
	}

	// Clean up orphaned output directories from previous server runs.
	// This handles the case where FFmpeg processes (spawned via systemd-run)
	// continue running after a server restart, leaving stale output directories
	// that would conflict with new sessions for the same media/quality.
	if cleanedCount, freedBytes, err := sessionManager.CleanupOrphanedOutputDirs(cfg.Media.TranscodeOutputDir); err != nil {
		logger.Warn("Failed to clean orphaned transcode directories",
			"error", err)
	} else if cleanedCount > 0 {
		logger.Info("Cleaned orphaned transcode directories on startup",
			"directories_cleaned", cleanedCount,
			"bytes_freed", freedBytes)
	}

	// Start comprehensive background cleanup:
	// 1. Idle sessions: Check every 15s, clean sessions idle >30s
	// 2. Old transcode caches: Check every 5min, remove caches >1 hour old
	// This ensures responsive resource cleanup and prevents disk space bloat
	_ = sessionManager.StartPeriodicCleanup(
		15*time.Second, // Session check interval
		30*time.Second, // Session idle timeout
		5*time.Minute,  // Transcode cache check interval
		1*time.Hour,    // Transcode cache max age
		cfg.Media.TranscodeOutputDir,
	)

	// Pre-warm GPU for HDR tone mapping (eliminates first-run delay)
	sessionManager.WarmupGPU()

	// Initialize auth services
	passwordHasher := auth.NewPasswordHasher(nil) // Use default Argon2id params

	// Get or generate JWT secret
	jwtSecret := []byte(cfg.Auth.JWTSecret)
	if len(jwtSecret) == 0 {
		// In production, this should have been caught by config validation.
		// In development, generate a temporary secret with a warning.
		var err error
		jwtSecret, err = auth.GenerateSecret()
		if err != nil {
			return nil, fmt.Errorf("failed to generate JWT secret: %w", err)
		}
		logger.Warn("Generated temporary JWT secret for development",
			"hint", "Sessions will be invalidated on restart. Set JWT_SECRET env var for persistence.",
			"note", "In production mode (ENVIRONMENT=production), JWT_SECRET is required.")
	}

	tokenConfig := &auth.TokenConfig{
		Secret:          jwtSecret,
		AccessTokenTTL:  cfg.Auth.AccessTokenTTL,
		RefreshTokenTTL: cfg.Auth.RefreshTokenTTL,
		Issuer:          "viewra",
	}
	tokenService := auth.NewTokenService(tokenConfig)

	// Initialize encryption for sensitive settings
	var encryptor *crypto.Encryptor
	encryptor, err = crypto.NewEncryptor(cfg.DataDir)
	if err != nil {
		logger.Warn("Failed to initialize encryption, sensitive settings will be stored in plaintext",
			"error", err)
		encryptor = nil
	} else {
		logger.Info("Encryption initialized for sensitive settings")
	}

	// Initialize settings service
	settingsService := settings.NewService(repos.SystemSettings, repos.UserSettings, encryptor)

	// Wire system profile into settings service for read-only system info
	if cfg.SystemProfile != nil {
		settingsService.SetSystemProfile(cfg.SystemProfile)
	}

	// Create AI config reader for HostLLMServer integration
	aiConfigReader := settings.NewAIConfigReader(settingsService)

	// Wire settings service into session manager for dynamic config
	// This enables runtime changes to tone mapping settings without restart
	configProvider := transcode.NewSettingsConfigProvider(settingsService, transcodeConfig)
	sessionManager.SetConfigProvider(configProvider)

	// Initialize subtitle converter (for WebVTT conversion)
	subtitleCacheDir := cfg.Media.TranscodeOutputDir // Reuse transcode output dir for subtitle cache
	subtitleConverter := subtitles.NewConverter(subtitleCacheDir)

	// Initialize Event Bus for pub/sub messaging
	// Ring buffer size of 10000 provides ~10min of event history for replay
	eventBus := events.NewBus(10000, logger)

	// Wire event bus to session manager for transcode lifecycle events
	sessionManager.SetPublisher(eventBus)

	// Initialize image downloader for remote images (TMDB, TVDB, etc.)
	imageDownloader := infraimages.NewDownloader(infraimages.DownloaderConfig{
		CacheDir:  cfg.Images.CacheDir,
		RateLimit: 5.0, // 5 requests per second for API rate limiting
		Logger:    logger.With("component", "image-downloader"),
	})

	// Create metadata extractor adapter that converts ImageInfo to ImageMetadata
	metadataExtractor := infraimages.NewMetadataExtractor()
	metadataAdapter := &metadataExtractorAdapter{extractor: metadataExtractor}

	// Initialize enrichment pipeline manager
	pipelineLogger := logger.With("component", "enrichment-pipeline")
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
			// Image processing dependencies
			MetadataExtractor: metadataAdapter,
			Transformer:       imageTransformer,
			Downloader:        imageDownloader,
			// People/credits dependencies
			PeopleRepo: repos.People,
			// Studios dependencies
			StudioRepo: repos.Studios,
			// Keywords dependencies
			KeywordRepo: repos.Keywords,
		},
		&pipeline.TypedMediaRepos{
			Movie: repos.Movie,
			TV:    repos.TV,
			Music: repos.Music,
		},
	)

	// Create enricher registry and register built-in enrichers
	enricherRegistry := enrichment.NewRegistry()
	imageExtractor := infraimages.NewExtractor()

	builtinEnrichers := []enrichment.Enricher{
		builtin.NewNFOEnricher(),
		builtin.NewLocalImagesEnricher(imageExtractor, logger),
	}

	for _, e := range builtinEnrichers {
		if err := enricherRegistry.RegisterBuiltin(e); err != nil {
			logger.Warn("Failed to register builtin enricher", "stage", e.Stage(), "error", err)
		}
		pipelineManager.RegisterEnricher(e)
	}

	// Initialize location repository and weather service
	// These are used for both location settings API and AI context enrichment
	locationRepo := location.NewRepository(db, dbDriver)
	weatherService := weather.NewService(logger.With("component", "weather"))

	// Initialize plugin manager for external plugins
	var pluginManager *plugins.Manager
	if cfg.Plugins.Enabled {
		pluginLogger := logger.With("component", "plugin-manager")

		// Create HostStorageServer for plugin KV storage
		var hostStorageServer *plugins.HostStorageServer
		if db != nil {
			var storageErr error
			hostStorageServer, storageErr = plugins.NewHostStorageServer(plugins.HostStorageConfig{
				BaseDir: cfg.Plugins.StorageDir,
			}, db, dbDriver, logger.With("component", "host-storage"))
			if storageErr != nil {
				logger.Warn("Failed to create host storage server", "error", storageErr)
			}
		}

		// Create AI host servers for plugins (embedding, LLM)
		var hostLLMServer *plugins.HostLLMServer
		var hostEmbeddingsServer *plugins.HostEmbeddingsServer

		// Create provider factory for LLM/embedding providers
		providerFactory := providers.NewFactory()

		// Create HostLLMServer with provider factory
		// Default to Ollama on localhost for local AI inference
		hostLLMServer = plugins.NewHostLLMServer(plugins.HostLLMConfig{
			OllamaBaseURL: "http://localhost:11434",
		}, providerFactory, logger.With("component", "host-llm"))

		// Wire AI config reader for dynamic settings from database
		hostLLMServer.SetConfigReader(aiConfigReader)
		hostLLMServer.RefreshConfig(context.Background())

		// Create HostEmbeddingsServer if embedding repository is available
		if repos.Embedding != nil {
			hostEmbeddingsServer = plugins.NewHostEmbeddingsServer(
				repos.Embedding,
				logger.With("component", "host-embeddings"),
			)
		}

		// Create HostWeatherServer for location-aware AI context enrichment
		// This allows plugins to get weather data when users have opted in to location sharing
		hostWeatherServer := plugins.NewHostWeatherServer(
			locationRepo,
			weatherService,
			logger.With("component", "host-weather"),
		)

		var err error
		pluginManager, err = plugins.NewManager(plugins.ManagerConfig{
			PluginDir:            cfg.Plugins.Dir,
			StorageDir:           cfg.Plugins.StorageDir,
			HostVersion:          version.Version,
			MediaQuerier:         repos.PluginMediaQuerier,
			HostStorageServer:    hostStorageServer,
			HostLLMServer:        hostLLMServer,
			HostEmbeddingsServer: hostEmbeddingsServer,
			HostWeatherServer:    hostWeatherServer,
		}, pluginLogger)
		if err != nil {
			logger.Warn("Failed to create plugin manager", "error", err)
		}

		// Wire event bus to plugin manager for lifecycle events
		if pluginManager != nil && eventBus != nil {
			pluginManager.SetPublisher(eventBus)
		}

		// Subscribe to AI settings changes for dynamic config refresh
		if hostLLMServer != nil && eventBus != nil {
			go subscribeToAISettingsChanges(eventBus, hostLLMServer, logger)
		}
	}

	// Create AI services
	// Embedding provider is nil initially - configured later via settings UI
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
		EventBus:          eventBus,
		PipelineManager:   pipelineManager,
		EnricherRegistry:  enricherRegistry,
		PluginManager:     pluginManager,
		LocationRepo:      locationRepo,
		WeatherService:    weatherService,
	}, nil
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

// subscribeToAISettingsChanges listens for AI settings changes and refreshes the LLM config.
// This runs as a background goroutine and automatically refreshes HostLLMServer config
// when AI-related settings are changed via the settings API.
func subscribeToAISettingsChanges(
	eventBus *events.Bus,
	hostLLMServer *plugins.HostLLMServer,
	logger *slog.Logger,
) {
	sub := eventBus.Subscribe(events.WithEventTypes(events.EventSettingsChanged))
	defer sub.Close()

	for event := range sub.Events() {
		// Check if this is an AI settings change
		category, ok := event.Data["category"].(string)
		if !ok {
			continue
		}

		if category == "ai" {
			logger.Debug("AI settings changed, refreshing HostLLMServer config")
			hostLLMServer.RefreshConfig(context.Background())
		}
	}
}
