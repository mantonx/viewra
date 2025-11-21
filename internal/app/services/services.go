package services

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/viewra/viewra/internal/app/config"
	"github.com/viewra/viewra/internal/app/repositories"
	"github.com/viewra/viewra/internal/application/transcode"
	infraimages "github.com/viewra/viewra/internal/infrastructure/images"
	"github.com/viewra/viewra/internal/infrastructure/pathbrowser"
	"github.com/viewra/viewra/internal/infrastructure/transcoding"
)

// Services holds all infrastructure and domain services.
// Groups all service implementations for dependency injection.
type Services struct {
	// Image services
	ImageCache       *infraimages.CacheService
	ImageTransformer *infraimages.Transformer

	// Media services
	PathBrowser      *pathbrowser.Service
	TranscodeService transcoding.Service
	TranscodeQueue   *transcode.Queue
	CleanupService   *transcode.CleanupService
	SessionManager   *transcoding.SessionManager
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
	transcodeConfig := transcoding.DefaultTranscodeConfig()
	sessionManager := transcoding.NewSessionManager(transcodeConfig, logger)

	// Start comprehensive background cleanup:
	// 1. Idle sessions: Check every 15s, clean sessions idle >30s
	// 2. Old transcode caches: Check every 5min, remove caches >1 hour old
	// This ensures responsive resource cleanup and prevents disk space bloat
	_ = sessionManager.StartPeriodicCleanup(
		15*time.Second,  // Session check interval
		30*time.Second,  // Session idle timeout
		5*time.Minute,   // Transcode cache check interval
		1*time.Hour,     // Transcode cache max age
		cfg.Media.TranscodeOutputDir,
	)

	return &Services{
		ImageCache:       imageCacheService,
		ImageTransformer: imageTransformer,
		PathBrowser:      browserService,
		TranscodeService: transcodeService,
		TranscodeQueue:   transcodeQueue,
		CleanupService:   cleanupService,
		SessionManager:   sessionManager,
	}, nil
}

// ensureDirectory creates a directory if it doesn't exist
func ensureDirectory(path string) error {
	return os.MkdirAll(path, 0755)
}
