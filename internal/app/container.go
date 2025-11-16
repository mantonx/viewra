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
	"github.com/viewra/viewra/internal/application/images"
	"github.com/viewra/viewra/internal/application/library"
	"github.com/viewra/viewra/internal/application/media"
	"github.com/viewra/viewra/internal/application/movies"
	"github.com/viewra/viewra/internal/application/music"
	"github.com/viewra/viewra/internal/application/transcode"
	"github.com/viewra/viewra/internal/application/tv"
	infraimages "github.com/viewra/viewra/internal/infrastructure/images"
	"github.com/viewra/viewra/internal/infrastructure/pathbrowser"
	"github.com/viewra/viewra/internal/infrastructure/persistence/common"
	imageRepo "github.com/viewra/viewra/internal/infrastructure/persistence/image"
	libraryRepo "github.com/viewra/viewra/internal/infrastructure/persistence/library"
	mediaRepo "github.com/viewra/viewra/internal/infrastructure/persistence/media"
	movieRepo "github.com/viewra/viewra/internal/infrastructure/persistence/movie"
	musicRepo "github.com/viewra/viewra/internal/infrastructure/persistence/music"
	progressRepo "github.com/viewra/viewra/internal/infrastructure/persistence/progress"
	scanJobRepo "github.com/viewra/viewra/internal/infrastructure/persistence/scanjob"
	transcodeRepo "github.com/viewra/viewra/internal/infrastructure/persistence/transcode"
	tvRepo "github.com/viewra/viewra/internal/infrastructure/persistence/tvshow"
	"github.com/viewra/viewra/internal/infrastructure/scheduler"
	"github.com/viewra/viewra/internal/infrastructure/transcoding"
)

// Container holds all application dependencies
type Container struct {
	// HTTP Server
	Server *api.Server

	// Background services
	CleanupScheduler *transcode.CleanupScheduler
	Scheduler        *scheduler.Scheduler
}

// NewContainer creates and wires up all application dependencies
func NewContainer(db *sql.DB, dbDriver string, config api.ServerConfig, logger *slog.Logger) *Container {
	// Initialize base repository for dual-database support
	baseRepo := common.NewBaseRepository(db, dbDriver)

	// Initialize repositories
	libraryRepository := libraryRepo.NewRepository(db, dbDriver)
	mediaRepository := mediaRepo.NewRepository(db, dbDriver)
	progressRepository := progressRepo.NewRepository(db, dbDriver)
	scanJobRepository := scanJobRepo.NewRepository(db, dbDriver)
	transcodeRepository := transcodeRepo.NewRepository(db, dbDriver)

	// Initialize media-type specific repositories
	// Pass mediaRepository for proper dependency injection
	movieRepository := movieRepo.NewRepository(baseRepo, mediaRepository)
	tvRepository := tvRepo.NewRepository(baseRepo, mediaRepository)
	musicRepository := musicRepo.NewRepository(baseRepo, mediaRepository)

	// Initialize library use cases (they use repositories directly)
	createLibrary := library.NewCreateLibraryUseCase(libraryRepository)
	updateLibrary := library.NewUpdateLibraryUseCase(libraryRepository)
	getLibrary := library.NewGetLibraryUseCase(libraryRepository)
	listLibraries := library.NewListLibrariesUseCase(libraryRepository)

	// Initialize image repository (used by scanner and API handlers)
	imageRepository := imageRepo.NewRepository(baseRepo)

	// Initialize image cache service (needed for image extraction)
	imageCacheDir := "./data/cache/images"
	if err := ensureDirectory(imageCacheDir); err != nil {
		logger.Error("Failed to create image cache directory", "error", err, "path", imageCacheDir)
	}
	imageCacheService := infraimages.NewCacheService(imageCacheDir)

	// Initialize image transformer (needed for WebP conversion during scan)
	imageTransformer := infraimages.NewTransformer(imageCacheService)

	// Initialize image extraction use cases (needed for scanner)
	extractMovieImages := images.NewExtractMovieImagesUseCase(imageRepository, imageCacheService, imageTransformer)
	extractEpisodeImages := images.NewExtractTVEpisodeImagesUseCase(imageRepository, imageCacheService, imageTransformer)
	extractMusicImages := images.NewExtractMusicAlbumImagesUseCase(imageRepository, imageCacheService, imageTransformer)

	// Initialize media use cases
	getMedia := media.NewGetMediaUseCase(mediaRepository)
	listMedia := media.NewListMediaUseCase(mediaRepository)

	// Initialize movie use cases
	listMovies := movies.NewListMoviesUseCase(movieRepository)
	getMovie := movies.NewGetMovieUseCase(movieRepository)
	searchMovies := movies.NewSearchMoviesUseCase(movieRepository)

	// Initialize TV use cases
	listTVShows := tv.NewListTVShowsUseCase(tvRepository)
	getTVShow := tv.NewGetTVShowUseCase(tvRepository)
	listTVEpisodes := tv.NewListTVEpisodesUseCase(tvRepository)
	getTVEpisode := tv.NewGetTVEpisodeUseCase(tvRepository)
	searchTVEpisodes := tv.NewSearchTVEpisodesUseCase(tvRepository)

	// Initialize music use cases
	listArtists := music.NewListArtistsUseCase(musicRepository)
	listAlbums := music.NewListAlbumsUseCase(musicRepository)
	listTracks := music.NewListTracksUseCase(musicRepository)
	getTrack := music.NewGetTrackUseCase(musicRepository)
	searchTracks := music.NewSearchTracksUseCase(musicRepository)

	// Initialize image query use cases (for API handlers)
	getImage := images.NewGetImageUseCase(imageRepository)
	getMediaImages := images.NewGetMediaImagesUseCase(imageRepository)
	getEntityImages := images.NewGetEntityImagesUseCase(imageRepository)

	// Initialize image cleanup use case (reuses imageCacheService from above)
	imageCleanup := images.NewCleanupUseCase(imageRepository, imageCacheDir, logger)

	// Initialize media use cases (with image cleanup)
	// NOTE: deleteMedia is available but not yet wired to API endpoints
	// It will be used when we add DELETE /api/media/:id or during library re-scans
	deleteMedia := media.NewDeleteMediaUseCase(mediaRepository, imageRepository, imageCleanup)
	_ = deleteMedia // Prevent unused variable error until API endpoint is added

	// Initialize library deletion with image cleanup
	deleteLibrary := library.NewDeleteLibraryUseCase(libraryRepository, imageRepository, imageCleanup)

	// Initialize scan library use case with image cleanup
	scanLibrary := library.NewScanLibraryUseCase(
		libraryRepository,
		mediaRepository,
		movieRepository,
		tvRepository,
		musicRepository,
		scanJobRepository,
		extractMovieImages,
		extractEpisodeImages,
		extractMusicImages,
		imageRepository,
		imageCleanup,
	)

	// Note: Image cleanup task will be registered with unified scheduler after it's created
	// See scheduler registration section below

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
	transcodeOutputDir := "./data/cache/transcodes"

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

	// Image handler (with caching and transformation support)
	imagesHandler := handlers.NewImagesHandler(getImage, getMediaImages, getEntityImages, imageTransformer, imageCacheService)

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

	// Create unified task scheduler
	execLogger := scheduler.NewDBExecutionLogger(db, logger)
	taskScheduler, err := scheduler.New(
		scheduler.DefaultConfig(),
		logger,
		execLogger,
	)
	if err != nil {
		logger.Error("Failed to create task scheduler", "error", err)
		// Continue without scheduler rather than crashing
	}

	// Register scheduled tasks with unified scheduler
	if taskScheduler != nil {
		// Register image cache cleanup task
		err := taskScheduler.RegisterTask(scheduler.Task{
			ID:          "image-cache-cleanup",
			Name:        "Image Cache Cleanup",
			Description: "Remove orphaned image cache files that are no longer referenced in the database",
			Schedule:    "0 3 * * *", // Daily at 3 AM
			Enabled:     true,
			Handler: func(ctx context.Context) error {
				_, err := imageCleanup.CleanOrphanedImages(ctx)
				return err
			},
		})
		if err != nil {
			logger.Error("Failed to register image cleanup task", "error", err)
		} else {
			logger.Info("Registered image cleanup task with scheduler")
		}
	}

	// Create scheduler handler
	schedulerHandler := handlers.NewSchedulerHandler(taskScheduler)

	// Create HTTP server
	server := api.NewServer(
		config,
		logger,
		healthHandler,
		browserHandler,
		scanJobHandler,
		progressHandler,
		transcodeHandler,
		imagesHandler,
		schedulerHandler,
		createLibrary,
		updateLibrary,
		deleteLibrary,
		getLibrary,
		listLibraries,
		scanLibrary,
		getMedia,
		listMedia,
		listMovies,
		getMovie,
		searchMovies,
		listTVShows,
		getTVShow,
		listTVEpisodes,
		getTVEpisode,
		searchTVEpisodes,
		listArtists,
		listAlbums,
		listTracks,
		getTrack,
		searchTracks,
	)

	return &Container{
		Server:           server,
		CleanupScheduler: cleanupScheduler,
		Scheduler:        taskScheduler,
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
