package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/viewra/viewra/internal/api"
	"github.com/viewra/viewra/internal/api/handlers"
	"github.com/viewra/viewra/internal/app/noop"
	"github.com/viewra/viewra/internal/application/library"
	"github.com/viewra/viewra/internal/application/media"
	"github.com/viewra/viewra/internal/application/transcode"
	"github.com/viewra/viewra/internal/infrastructure/pathbrowser"
	libraryRepo "github.com/viewra/viewra/internal/infrastructure/persistence/library"
	mediaRepo "github.com/viewra/viewra/internal/infrastructure/persistence/media"
	progressRepo "github.com/viewra/viewra/internal/infrastructure/persistence/progress"
	scanJobRepo "github.com/viewra/viewra/internal/infrastructure/persistence/scanjob"
	transcodeRepo "github.com/viewra/viewra/internal/infrastructure/persistence/transcode"
	"github.com/viewra/viewra/internal/infrastructure/transcoding"
)

// Container holds all application dependencies
type Container struct {
	// HTTP Server
	Server *api.Server

	// Background services
	CleanupScheduler *transcode.CleanupScheduler
}

// NewContainer creates and wires up all application dependencies
func NewContainer(db *sql.DB, dbDriver string, config api.ServerConfig, logger *slog.Logger) *Container {
	// Initialize repositories
	libraryRepository := libraryRepo.NewRepository(db, dbDriver)
	mediaRepository := mediaRepo.NewRepository(db, dbDriver)
	progressRepository := progressRepo.NewRepository(db, dbDriver)
	scanJobRepository := scanJobRepo.NewRepository(db, dbDriver)
	transcodeRepository := transcodeRepo.NewRepository(db, dbDriver)

	// Initialize library use cases (they use repositories directly)
	createLibrary := library.NewCreateLibraryUseCase(libraryRepository)
	updateLibrary := library.NewUpdateLibraryUseCase(libraryRepository)
	deleteLibrary := library.NewDeleteLibraryUseCase(libraryRepository)
	getLibrary := library.NewGetLibraryUseCase(libraryRepository)
	listLibraries := library.NewListLibrariesUseCase(libraryRepository)

	// Initialize scan library use case
	// NOTE: Movie/TV/Music-specific repositories (Phase 3) not yet implemented.
	// Using no-op implementations to prevent nil pointer panics during scanning.
	scanLibrary := library.NewScanLibraryUseCase(
		libraryRepository,
		mediaRepository,
		noop.NewMovieRepository(), // Phase 3 - no-op for now
		noop.NewTVRepository(),    // Phase 3 - no-op for now
		noop.NewMusicRepository(), // Phase 3 - no-op for now
		scanJobRepository,
	)

	// Initialize media use cases
	getMedia := media.NewGetMediaUseCase(mediaRepository)
	listMedia := media.NewListMediaUseCase(mediaRepository)

	// Initialize path browser service
	browserService := pathbrowser.NewService(
		config.Browser.AllowedBasePaths,
		config.Browser.DefaultBasePath,
	)

	// Initialize transcode service and queue
	transcodeService, err := transcoding.NewService(transcodeRepository, logger)
	if err != nil {
		logger.Error("Failed to initialize transcode service", "error", err)
		// Continue without transcode service - it's not critical for basic functionality
		transcodeService = nil
	}

	// Transcode output directory
	transcodeOutputDir := "./data/transcode"

	// Ensure transcode output directory exists
	if err := ensureDirectory(transcodeOutputDir); err != nil {
		logger.Error("Failed to create transcode output directory", "error", err, "path", transcodeOutputDir)
	}

	var transcodeQueue *transcode.Queue
	if transcodeService != nil {
		queueConfig := &transcode.QueueConfig{
			WorkerCount:   2, // TODO: Make configurable
			PollInterval:  10 * time.Second,
			IdleTimeout:   5 * time.Minute, // Cancel transcodes after 5 minutes of no segment requests
			OutputBaseDir: transcodeOutputDir,
			MediaFileGetter: func(ctx context.Context, mediaID int64) (string, error) {
				// Get media entity to get file path
				media, err := mediaRepository.GetByID(ctx, mediaID)
				if err != nil {
					return "", err
				}

				// file_path in database is absolute, so use it directly
				return media.FilePath, nil
			},
		}
		transcodeQueue = transcode.NewQueue(queueConfig, transcodeRepository, transcodeService, logger)

		// Start queue with background context
		if err := transcodeQueue.Start(context.Background()); err != nil {
			logger.Error("Failed to start transcode queue", "error", err)
		}
	}

	// Create handlers
	healthHandler := handlers.NewHealthHandler(db)
	browserHandler := handlers.NewBrowserHandler(browserService)
	scanJobHandler := handlers.NewScanJobHandler(scanJobRepository)
	progressHandler := handlers.NewProgressHandler(progressRepository)

	var transcodeHandler *handlers.TranscodeHandler
	if transcodeQueue != nil {
		// Create use cases
		createJobUseCase := transcode.NewCreateJobUseCase(transcodeRepository, transcodeQueue)
		getStatusUseCase := transcode.NewGetJobStatusUseCase(transcodeRepository)
		serveManifestUseCase := transcode.NewServeManifestUseCase(
			transcodeRepository,
			mediaRepository,
			libraryRepository,
			createJobUseCase,
		)
		cleanupService := transcode.NewCleanupService(transcodeRepository, transcodeOutputDir)

		transcodeHandler = handlers.NewTranscodeHandler(
			createJobUseCase,
			getStatusUseCase,
			serveManifestUseCase,
			transcodeQueue,
			cleanupService,
			transcodeOutputDir,
		)
	}

	// Create cleanup scheduler with configuration from environment
	var cleanupScheduler *transcode.CleanupScheduler
	if transcodeRepository != nil {
		cleanupConfig := getCleanupConfig()
		cleanupService := transcode.NewCleanupService(transcodeRepository, transcodeOutputDir)
		cleanupScheduler = transcode.NewCleanupScheduler(
			cleanupConfig,
			cleanupService,
			transcodeRepository,
			transcodeOutputDir,
			logger,
		)
	}

	// Create HTTP server
	server := api.NewServer(
		config,
		logger,
		healthHandler,
		browserHandler,
		scanJobHandler,
		progressHandler,
		transcodeHandler,
		createLibrary,
		updateLibrary,
		deleteLibrary,
		getLibrary,
		listLibraries,
		scanLibrary,
		getMedia,
		listMedia,
	)

	return &Container{
		Server:           server,
		CleanupScheduler: cleanupScheduler,
	}
}

// getCleanupConfig reads cleanup configuration from environment variables.
func getCleanupConfig() *transcode.CleanupSchedulerConfig {
	config := transcode.DefaultCleanupSchedulerConfig()

	// Read environment variables with fallback to defaults
	if val := os.Getenv("TRANSCODE_CLEANUP_ENABLED"); val != "" {
		config.Enabled = val == "true" || val == "1"
	}
	if val := os.Getenv("TRANSCODE_CLEANUP_INTERVAL_HOURS"); val != "" {
		if hours, err := time.ParseDuration(val + "h"); err == nil {
			config.Interval = hours
		}
	}
	if val := os.Getenv("TRANSCODE_CLEANUP_DISK_THRESHOLD"); val != "" {
		if threshold, err := parseInt(val); err == nil {
			config.DiskThresholdPercent = threshold
		}
	}
	if val := os.Getenv("TRANSCODE_CLEANUP_DISK_WARNING"); val != "" {
		if warning, err := parseInt(val); err == nil {
			config.DiskWarningPercent = warning
		}
	}
	if val := os.Getenv("TRANSCODE_MIN_FREE_SPACE_GB"); val != "" {
		if gb, err := parseInt64(val); err == nil {
			config.MinFreeSpaceGB = gb
		}
	}
	if val := os.Getenv("TRANSCODE_MAX_AGE_DAYS"); val != "" {
		if days, err := parseInt(val); err == nil {
			config.MaxAgeHours = days * 24
		}
	}
	if val := os.Getenv("TRANSCODE_MAX_IDLE_DAYS"); val != "" {
		if days, err := parseInt(val); err == nil {
			config.MaxIdleHours = days * 24
		}
	}
	if val := os.Getenv("TRANSCODE_MAX_STORAGE_GB"); val != "" {
		if gb, err := parseInt64(val); err == nil {
			config.MaxStorageGB = gb
		}
	}
	if val := os.Getenv("TRANSCODE_CLEANUP_BATCH_SIZE"); val != "" {
		if size, err := parseInt(val); err == nil {
			config.CleanupBatchSize = size
		}
	}
	if val := os.Getenv("TRANSCODE_KEEP_FAILED_HOURS"); val != "" {
		if hours, err := parseInt(val); err == nil {
			config.KeepFailedHours = hours
		}
	}

	return config
}

func parseInt(s string) (int, error) {
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

func parseInt64(s string) (int64, error) {
	var v int64
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

// ensureDirectory creates a directory if it doesn't exist.
func ensureDirectory(path string) error {
	return os.MkdirAll(path, 0755)
}
