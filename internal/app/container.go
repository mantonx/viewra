package app

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/mantonx/viewra/internal/api"
	appconfig "github.com/mantonx/viewra/internal/app/config"
	apphandlers "github.com/mantonx/viewra/internal/app/handlers"
	"github.com/mantonx/viewra/internal/app/repositories"
	"github.com/mantonx/viewra/internal/app/services"
	"github.com/mantonx/viewra/internal/app/usecases"
	"github.com/mantonx/viewra/internal/application/common"
	"github.com/mantonx/viewra/internal/application/transcode"
	"github.com/mantonx/viewra/internal/infrastructure/scheduler"
)

// Container holds all application dependencies
type Container struct {
	// HTTP Server
	Server *api.Server

	// Background services
	Scheduler      *scheduler.Scheduler
	TranscodeQueue *transcode.Queue
}

// NewContainer creates and wires up all application dependencies
func NewContainer(db *sql.DB, dbDriver string, cfg *appconfig.Config, logger *slog.Logger) *Container {
	// Build all layers using builder functions
	repos := repositories.BuildRepositories(db, dbDriver)
	svcs, err := services.BuildServices(cfg, repos, logger)
	if err != nil {
		logger.Error("Failed to build services", "error", err)
	}

	// Create transaction manager for use cases
	txManager := common.NewTxManager(db)

	cases := usecases.BuildUseCases(cfg, repos, svcs, txManager, logger)

	// Create unified task scheduler
	execLogger := scheduler.NewDBExecutionLogger(db, logger)
	taskScheduler, err := scheduler.New(
		scheduler.DefaultConfig(),
		logger,
		execLogger,
	)
	if err != nil {
		logger.Error("Failed to create task scheduler", "error", err)
	}

	// Register scheduled tasks with unified scheduler
	if taskScheduler != nil {
		registerTasks(taskScheduler, cfg, cases, repos, svcs, logger)
	}

	// Build handlers
	handlers := apphandlers.BuildHandlers(
		db,
		repos,
		svcs,
		cases,
		taskScheduler,
		cfg.Media.TranscodeOutputDir,
		logger,
	)

	// Create HTTP server
	server := api.NewServer(
		cfg.Server.ToAPIServerConfig(),
		logger,
		handlers.Health,
		handlers.Browser,
		handlers.ScanJob,
		handlers.Progress,
		handlers.Transcode,
		handlers.Images,
		handlers.Scheduler,
		cases.Library.Create,
		cases.Library.Update,
		cases.Library.Delete,
		cases.Library.Get,
		cases.Library.List,
		cases.Library.Scan,
		cases.Media.Get,
		cases.Media.List,
		cases.Movies.List,
		cases.Movies.Get,
		cases.Movies.Search,
		cases.Movies.ListIDs,
		cases.TV.ListShows,
		cases.TV.GetShow,
		cases.TV.ListEpisodes,
		cases.TV.GetEpisode,
		cases.TV.SearchEpisodes,
		cases.TV.ListShowIDs,
		cases.TV.GetNextEpisode,
		cases.Music.ListArtists,
		cases.Music.ListAlbumsByArtist,
		cases.Music.ListTracksByAlbum,
		cases.Music.GetTrack,
		cases.Music.SearchTracks,
		cases.Music.ListArtistIDs,
	)

	return &Container{
		Server:         server,
		Scheduler:      taskScheduler,
		TranscodeQueue: svcs.TranscodeQueue,
	}
}

// Shutdown gracefully shuts down all container services
func (c *Container) Shutdown(ctx context.Context) error {
	var firstErr error

	// Stop transcode queue first
	if c.TranscodeQueue != nil {
		c.TranscodeQueue.Stop()
	}

	// Stop scheduler
	if c.Scheduler != nil {
		c.Scheduler.Stop()
	}

	// Shutdown server last
	if c.Server != nil {
		if err := c.Server.Shutdown(ctx); err != nil {
			firstErr = err
		}
	}

	return firstErr
}

// registerTasks registers all scheduled tasks with the unified scheduler
func registerTasks(
	taskScheduler *scheduler.Scheduler,
	cfg *appconfig.Config,
	cases *usecases.UseCases,
	repos *repositories.Repositories,
	svcs *services.Services,
	logger *slog.Logger,
) {
	// Register scan job cleanup task
	err := taskScheduler.RegisterTask(scheduler.Task{
		ID:          "scan-job-cleanup",
		Name:        "Scan Job Cleanup",
		Description: "Delete old scan jobs and their checkpoints based on retention policy",
		Schedule:    "*/30 * * * *", // Every 30 minutes
		Enabled:     true,
		Handler: func(ctx context.Context) error {
			retentionMinutes := cfg.Media.ScanJobRetentionMinutes
			logger.Info("Running scan job cleanup", "retention_minutes", retentionMinutes)

			// Get all libraries and clean up their old scan jobs
			libraries, err := repos.Library.List(ctx)
			if err != nil {
				return err
			}

			for _, lib := range libraries {
				// The repository's DeleteOld method handles CASCADE deletion of checkpoints
				if err := repos.ScanJob.DeleteOld(ctx, lib.ID, retentionMinutes); err != nil {
					logger.Error("Failed to clean scan jobs for library",
						"library_id", lib.ID,
						"error", err)
					// Continue with other libraries even if one fails
				}
			}

			logger.Info("Scan job cleanup completed", "libraries_processed", len(libraries))
			return nil
		},
	})
	if err != nil {
		logger.Error("Failed to register scan job cleanup task", "error", err)
	} else {
		logger.Info("Registered scan job cleanup task with scheduler")
	}

	// Register image cache cleanup task
	err = taskScheduler.RegisterTask(scheduler.Task{
		ID:          "image-cache-cleanup",
		Name:        "Image Cache Cleanup",
		Description: "Remove orphaned image cache files that are no longer referenced in the database",
		Schedule:    "0 3 * * *", // Daily at 3 AM
		Enabled:     true,
		Handler: func(ctx context.Context) error {
			_, err := cases.Images.Cleanup.CleanOrphanedImages(ctx)
			return err
		},
	})
	if err != nil {
		logger.Error("Failed to register image cleanup task", "error", err)
	} else {
		logger.Info("Registered image cleanup task with scheduler")
	}

	// Register transcode cleanup tasks (if transcode is enabled)
	if repos.Transcode != nil && svcs.CleanupService != nil {
		cleanupConfig := cfg.Transcode.ToCleanupSchedulerConfig()

		// Task 1: Policy-based cleanup (failed, old, idle, orphans)
		err = taskScheduler.RegisterTask(scheduler.Task{
			ID:          "transcode-cleanup-policy",
			Name:        "Transcode Policy Cleanup",
			Description: "Clean failed/old/idle/orphaned transcodes based on policy rules",
			Schedule:    "0 */6 * * *", // Every 6 hours at :00
			Enabled:     cleanupConfig.Enabled,
			Handler: func(ctx context.Context) error {
				return transcode.PerformPolicyCleanup(ctx, svcs.CleanupService, cleanupConfig)
			},
		})
		if err != nil {
			logger.Error("Failed to register transcode policy cleanup task", "error", err)
		} else {
			logger.Info("Registered transcode policy cleanup task with scheduler")
		}

		// Task 2: Disk threshold monitoring and LRU cleanup
		err = taskScheduler.RegisterTask(scheduler.Task{
			ID:          "transcode-cleanup-disk-check",
			Name:        "Transcode Disk Monitor",
			Description: "Monitor disk usage and perform LRU cleanup if threshold exceeded",
			Schedule:    "*/30 * * * *", // Every 30 minutes
			Enabled:     cleanupConfig.Enabled,
			Handler: func(ctx context.Context) error {
				return transcode.PerformDiskMonitoring(
					ctx,
					svcs.CleanupService,
					repos.Transcode,
					cleanupConfig,
					cfg.Media.TranscodeOutputDir,
				)
			},
		})
		if err != nil {
			logger.Error("Failed to register transcode disk monitor task", "error", err)
		} else {
			logger.Info("Registered transcode disk monitor task with scheduler")
		}
	}
}
