package services

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/mantonx/viewra/internal/app/config"
	"github.com/mantonx/viewra/internal/app/repositories"
	"github.com/mantonx/viewra/internal/application/settings"
	"github.com/mantonx/viewra/internal/application/transcode"
	domaintranscode "github.com/mantonx/viewra/internal/domain/transcode"
	"github.com/mantonx/viewra/internal/infrastructure/auth"
	infraimages "github.com/mantonx/viewra/internal/infrastructure/images"
	"github.com/mantonx/viewra/internal/infrastructure/pathbrowser"
	"github.com/mantonx/viewra/internal/infrastructure/subtitles"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding"
	transcodeconfig "github.com/mantonx/viewra/internal/infrastructure/transcoding/config"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/logging"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/session"
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
}

// BuildServices creates and initializes all infrastructure services.
// Creates required directories and initializes services with proper error handling.
func BuildServices(
	cfg *config.Config,
	repos *repositories.Repositories,
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

	// Initialize settings service
	settingsService := settings.NewService(repos.SystemSettings, repos.UserSettings)

	// Wire system profile into settings service for read-only system info
	if cfg.SystemProfile != nil {
		settingsService.SetSystemProfile(cfg.SystemProfile)
	}

	// Wire settings service into session manager for dynamic config
	// This enables runtime changes to tone mapping settings without restart
	configProvider := transcode.NewSettingsConfigProvider(settingsService, transcodeConfig)
	sessionManager.SetConfigProvider(configProvider)

	// Initialize subtitle converter (for WebVTT conversion)
	subtitleCacheDir := cfg.Media.TranscodeOutputDir // Reuse transcode output dir for subtitle cache
	subtitleConverter := subtitles.NewConverter(subtitleCacheDir)

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
	}, nil
}

// ensureDirectory creates a directory if it doesn't exist
func ensureDirectory(path string) error {
	return os.MkdirAll(path, 0755)
}
