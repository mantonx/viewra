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
	"github.com/mantonx/viewra/internal/application/auth"
	"github.com/mantonx/viewra/internal/application/common"
	appplugins "github.com/mantonx/viewra/internal/application/plugins"
	"github.com/mantonx/viewra/internal/application/transcode"
	"github.com/mantonx/viewra/internal/infrastructure/plugins"
	"github.com/mantonx/viewra/internal/infrastructure/scheduler"
)

// Container holds all application dependencies
type Container struct {
	// HTTP Server
	Server *api.Server

	// Background services
	Scheduler      *scheduler.Scheduler
	TranscodeQueue *transcode.Queue
	Services       *services.Services

	// Use cases (exposed for startup tasks)
	UseCases *usecases.UseCases
}

// NewContainer creates and wires up all application dependencies
func NewContainer(db *sql.DB, dbDriver string, cfg *appconfig.Config, logger *slog.Logger) *Container {
	// Build all layers using builder functions
	repos := repositories.BuildRepositories(db, dbDriver)
	svcs, err := services.BuildServices(cfg, repos, db, dbDriver, logger)
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
		registerTasks(taskScheduler, cfg, cases, svcs, repos, logger)
	}

	// Seed dev user in development mode (before handlers so auth works immediately)
	seedDevUser(context.Background(), cfg, repos, svcs, logger)

	// Warm settings cache on startup
	if svcs.Settings != nil {
		if err := svcs.Settings.WarmCache(context.Background()); err != nil {
			logger.Warn("Failed to warm settings cache", "error", err)
		} else {
			logger.Info("Settings cache warmed")
		}
	}

	// Build handlers (returns *api.Handlers directly)
	infra := &apphandlers.InfrastructureDeps{
		DB:                 db,
		Scheduler:          taskScheduler,
		TranscodeOutputDir: cfg.Media.TranscodeOutputDir,
		Repos:              repos,
		Config:             cfg,
	}
	handlers := apphandlers.BuildHandlers(infra, svcs, cases, logger)

	// Create HTTP server
	server := api.NewServer(cfg.Server.ToAPIServerConfig(), logger, handlers)

	// Load and register external plugins before starting the pipeline
	if svcs.PluginManager != nil && svcs.PipelineManager != nil {
		loadExternalPlugins(context.Background(), svcs, repos, logger)
	}

	// Start the enrichment pipeline manager (background workers)
	if svcs.PipelineManager != nil {
		if err := svcs.PipelineManager.Start(context.Background()); err != nil {
			logger.Error("Failed to start enrichment pipeline", "error", err)
		} else {
			logger.Info("Enrichment pipeline manager started")
		}
	}

	// Start transcode analytics service (event bus subscription)
	if cases.TranscodeAnalytics != nil {
		cases.TranscodeAnalytics.Start()
		logger.Info("Transcode analytics service started")
	}

	return &Container{
		Server:         server,
		Scheduler:      taskScheduler,
		TranscodeQueue: svcs.TranscodeQueue,
		Services:       svcs,
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

	// Stop transcode analytics service
	if c.UseCases != nil && c.UseCases.TranscodeAnalytics != nil {
		c.UseCases.TranscodeAnalytics.Stop()
	}

	// Stop enrichment pipeline manager
	if c.Services != nil && c.Services.PipelineManager != nil {
		c.Services.PipelineManager.Stop()
	}

	// Stop scheduler
	if c.Scheduler != nil {
		c.Scheduler.Stop()
	}

	// Close event bus (closes all subscriptions)
	if c.Services != nil && c.Services.EventBus != nil {
		c.Services.EventBus.Close()
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
	repos *repositories.Repositories,
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

	// Register expired session cleanup task
	err = taskScheduler.RegisterTask(scheduler.Task{
		ID:          "session-cleanup",
		Name:        "Session Cleanup",
		Description: "Remove expired user sessions from the database",
		Schedule:    "0 * * * *", // Every hour at :00
		Enabled:     true,
		Handler: func(ctx context.Context) error {
			logger.Info("Running expired session cleanup")
			deleted, err := repos.Session.DeleteExpired(ctx)
			if err != nil {
				logger.Error("Failed to clean expired sessions", "error", err)
				return err
			}
			logger.Info("Expired session cleanup completed", "deleted_count", deleted)
			return nil
		},
	})
	if err != nil {
		logger.Error("Failed to register session cleanup task", "error", err)
	} else {
		logger.Info("Registered session cleanup task with scheduler")
	}
}

// seedDevUser creates a development user if running in development mode and no users exist.
// This reduces friction during local development by providing ready-to-use credentials.
// Dev credentials: username "dev", password "devdev00"
func seedDevUser(
	ctx context.Context,
	cfg *appconfig.Config,
	repos *repositories.Repositories,
	svcs *services.Services,
	logger *slog.Logger,
) {
	// Only seed in development environment
	if cfg.Environment != "development" {
		return
	}

	// Skip if user repository not available
	if repos.User == nil {
		return
	}

	// Check if any users exist
	exists, err := repos.User.ExistsAny(ctx)
	if err != nil {
		logger.Warn("Failed to check for existing users during dev seed", "error", err)
		return
	}

	// Don't seed if users already exist
	if exists {
		logger.Debug("Dev user seeding skipped - users already exist")
		return
	}

	logger.Info("Seeding development user (no users exist)")

	// Create auth service for registration
	authService := auth.NewService(
		repos.User,
		repos.Session,
		svcs.PasswordHasher,
		svcs.TokenService,
		cfg.Auth.MaxSessionsPerUser,
	)

	// Register dev user (password must be 8+ characters)
	_, err = authService.Register(ctx, &auth.RegisterRequest{
		Username:    "dev",
		DisplayName: "Developer",
		Password:    "devdev00",
		IsAdmin:     true,
	})
	if err != nil {
		logger.Error("Failed to create dev user", "error", err)
		return
	}

	logger.Info("Development user created",
		"username", "dev",
		"password", "devdev00",
		"note", "Only in development mode when no users exist")
}

// loadExternalPlugins discovers, loads, and registers external plugins with the pipeline.
// External plugins are registered as disabled by default - users must explicitly enable them.
func loadExternalPlugins(ctx context.Context, svcs *services.Services, repos *repositories.Repositories, logger *slog.Logger) {
	pm := svcs.PluginManager
	pipeline := svcs.PipelineManager

	// Discover and load all plugins
	if err := pm.LoadAllPlugins(ctx); err != nil {
		logger.Error("Failed to load plugins", "error", err)
		return
	}

	// Get all loaded plugins and register enrichers with the pipeline
	for _, instance := range pm.GetEnrichers() {
		// Persist plugin to database (upsert to handle version updates)
		if repos.Plugin != nil {
			manifest := instance.Manifest
			categories := "[]"
			if len(manifest.Categories) > 0 {
				// Convert to JSON array format
				categories = `["` + manifest.Categories[0] + `"]`
				if len(manifest.Categories) > 1 {
					for _, c := range manifest.Categories[1:] {
						categories = categories[:len(categories)-1] + `","` + c + `"]`
					}
				}
			}

			if err := repos.Plugin.UpsertPlugin(ctx, appplugins.Plugin{
				ID:          instance.ID,
				Name:        manifest.Name,
				Version:     manifest.Version,
				Description: manifest.Description,
				Author:      manifest.Author,
				License:     manifest.License,
				Homepage:    manifest.Homepage,
				Categories:  categories,
				Path:        instance.Path,
			}); err != nil {
				logger.Error("Failed to persist plugin to database",
					"plugin", instance.ID,
					"error", err)
			}
		}

		// Wrap the plugin's enricher client as an application Enricher
		enricher, err := plugins.NewGRPCEnricher(
			instance.ID,
			instance.EnricherClient,
			logger.With("plugin", instance.ID),
		)
		if err != nil {
			logger.Error("Failed to create enricher wrapper",
				"plugin", instance.ID,
				"error", err)
			continue
		}

		// Register with the pipeline (creates disabled pipeline stages if not existing)
		if err := pipeline.RegisterExternalEnricher(ctx, enricher); err != nil {
			logger.Error("Failed to register external enricher",
				"plugin", instance.ID,
				"error", err)
			continue
		}

		logger.Info("Registered external enricher plugin",
			"plugin_id", instance.ID,
			"plugin_name", instance.Manifest.Name,
			"version", instance.Manifest.Version)
	}
}
