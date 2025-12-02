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

	// Use cases (exposed for startup tasks)
	UseCases *usecases.UseCases
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
		registerTasks(taskScheduler, cfg, cases, svcs, logger)
	}

	// Build handlers (returns *api.Handlers directly)
	infra := &apphandlers.InfrastructureDeps{
		DB:                 db,
		Scheduler:          taskScheduler,
		TranscodeOutputDir: cfg.Media.TranscodeOutputDir,
	}
	handlers := apphandlers.BuildHandlers(infra, svcs, cases, logger)

	// Create HTTP server
	server := api.NewServer(cfg.Server.ToAPIServerConfig(), logger, handlers)

	return &Container{
		Server:         server,
		Scheduler:      taskScheduler,
		TranscodeQueue: svcs.TranscodeQueue,
		UseCases:       cases,
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

// registerTasks registers all scheduled tasks with the unified scheduler.
// Uses use cases for business logic and services for infrastructure operations.
func registerTasks(
	taskScheduler *scheduler.Scheduler,
	cfg *appconfig.Config,
	cases *usecases.UseCases,
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

			// Get all libraries via use case
			resp, err := cases.Library.Service.List(ctx)
			if err != nil {
				return err
			}

			for _, lib := range resp.Libraries {
				// Clean up old scan jobs via use case
				if err := cases.ScanJob.DeleteOld(ctx, lib.ID, retentionMinutes); err != nil {
					logger.Error("Failed to clean scan jobs for library",
						"library_id", lib.ID,
						"error", err)
					// Continue with other libraries even if one fails
				}
			}

			logger.Info("Scan job cleanup completed", "libraries_processed", len(resp.Libraries))
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

	// Register automatic library scanning task
	if cfg.Media.AutoScanEnabled {
		err = taskScheduler.RegisterTask(scheduler.Task{
			ID:          "auto-library-scan",
			Name:        "Automatic Library Scan",
			Description: "Automatically scan all libraries for new, modified, or deleted files",
			Schedule:    cfg.Media.AutoScanInterval,
			Enabled:     true,
			Handler: func(ctx context.Context) error {
				logger.Info("Starting automatic library scan")

				// Get all libraries via use case
				resp, err := cases.Library.Service.List(ctx)
				if err != nil {
					logger.Error("Failed to list libraries for auto scan", "error", err)
					return err
				}

				// Scan each library (incremental scan will detect changes efficiently)
				for _, lib := range resp.Libraries {
					logger.Info("Auto-scanning library",
						"library_id", lib.ID,
						"name", lib.Name,
						"path", lib.Path)

					// Trigger scan using the scan use case
					if _, err := cases.Library.Scan.StartScan(ctx, lib.ID); err != nil {
						logger.Error("Auto scan failed for library",
							"library_id", lib.ID,
							"library_name", lib.Name,
							"error", err)
						// Continue scanning other libraries even if one fails
						continue
					}

					logger.Info("Auto scan triggered for library",
						"library_id", lib.ID,
						"library_name", lib.Name)
				}

				logger.Info("Automatic library scan completed", "libraries_scanned", len(resp.Libraries))
				return nil
			},
		})
		if err != nil {
			logger.Error("Failed to register auto library scan task", "error", err)
		} else {
			logger.Info("Registered automatic library scan task",
				"interval", cfg.Media.AutoScanInterval,
				"enabled", cfg.Media.AutoScanEnabled)
		}
	}

	// Register transcode cleanup tasks (if transcode is enabled)
	if svcs.CleanupService != nil {
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
					svcs.TranscodeRepo,
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
