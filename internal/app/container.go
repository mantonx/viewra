package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	"github.com/mantonx/viewra/internal/api"
	appconfig "github.com/mantonx/viewra/internal/app/config"
	apphandlers "github.com/mantonx/viewra/internal/app/handlers"
	"github.com/mantonx/viewra/internal/app/lifecycle"
	"github.com/mantonx/viewra/internal/app/repositories"
	"github.com/mantonx/viewra/internal/app/services"
	"github.com/mantonx/viewra/internal/app/usecases"
	"github.com/mantonx/viewra/internal/application/auth"
	"github.com/mantonx/viewra/internal/application/common"
	appplugins "github.com/mantonx/viewra/internal/application/plugins"
	appscheduler "github.com/mantonx/viewra/internal/application/scheduler"
	"github.com/mantonx/viewra/internal/application/transcode"
	"github.com/mantonx/viewra/internal/infrastructure/plugins"
)

// Container holds all application dependencies
type Container struct {
	// HTTP Server
	Server *api.Server

	// Background services
	SchedulerService *appscheduler.Service
	TranscodeQueue   *transcode.Queue
	Services         *services.Services

	// Use cases (exposed for startup tasks)
	UseCases *usecases.UseCases

	// Lifecycle manager for server restart
	LifecycleMgr *lifecycle.Manager
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

	// Create scheduler service
	var schedulerService *appscheduler.Service
	schedulerService, err = InitScheduler(SchedulerDeps{
		DB:       db,
		DBDriver: dbDriver,
		Config:   cfg,
		Cases:    cases,
		Svcs:     svcs,
		Repos:    repos,
		Logger:   logger,
	})
	if err != nil {
		logger.Error("Failed to create scheduler service", "error", err)
	}

	// Create lifecycle manager for server restart coordination
	lifecycleMgr := lifecycle.NewManager(logger, svcs.EventBus)

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
		SchedulerService:   schedulerService,
		TranscodeOutputDir: cfg.Media.TranscodeOutputDir,
		Repos:              repos,
		Config:             cfg,
		LifecycleMgr:       lifecycleMgr,
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

	// Start the enqueue buffer (batches enrichment requests during scans)
	if svcs.EnqueueBuffer != nil {
		svcs.EnqueueBuffer.Start(context.Background())
		logger.Info("Enrichment enqueue buffer started")
	}

	// Start transcode analytics service (event bus subscription)
	if cases.TranscodeAnalytics != nil {
		cases.TranscodeAnalytics.Start()
		logger.Info("Transcode analytics service started")
	}

	// Start file system monitor (real-time library monitoring)
	if svcs.FileMonitor != nil {
		if err := svcs.FileMonitor.Start(context.Background()); err != nil {
			logger.Error("Failed to start file monitor service", "error", err)
		} else {
			logger.Info("File monitor service started")
		}
	}

	return &Container{
		Server:           server,
		SchedulerService: schedulerService,
		TranscodeQueue:   svcs.TranscodeQueue,
		Services:         svcs,
		UseCases:         cases,
		LifecycleMgr:     lifecycleMgr,
	}
}

// Shutdown gracefully shuts down all container services
func (c *Container) Shutdown(ctx context.Context) error {
	var firstErr error

	// Stop file monitor first (before enrichment pipeline)
	if c.Services != nil && c.Services.FileMonitor != nil {
		c.Services.FileMonitor.Stop()
	}

	// Stop transcode queue first
	if c.TranscodeQueue != nil {
		c.TranscodeQueue.Stop()
	}

	// Stop transcode analytics service
	if c.UseCases != nil && c.UseCases.TranscodeAnalytics != nil {
		c.UseCases.TranscodeAnalytics.Stop()
	}

	// Stop enqueue buffer first (flushes remaining jobs to pipeline)
	if c.Services != nil && c.Services.EnqueueBuffer != nil {
		c.Services.EnqueueBuffer.Stop()
	}

	// Stop enrichment pipeline manager
	if c.Services != nil && c.Services.PipelineManager != nil {
		c.Services.PipelineManager.Stop()
	}

	// Stop scheduler
	if c.SchedulerService != nil {
		c.SchedulerService.Stop()
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
	registry := svcs.EnricherRegistry

	// Discover and load all plugins
	if err := pm.LoadAllPlugins(ctx); err != nil {
		logger.Error("Failed to load plugins", "error", err)
		return
	}

	// Persist ALL plugins to database (including provider plugins)
	if repos.Plugin != nil {
		for _, instance := range pm.GetAllPlugins() {
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
	}

	// Register enrichers with the pipeline
	for _, instance := range pm.GetEnrichers() {
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

		// Register with the unified registry
		if err := registry.RegisterExternal(enricher, instance.Manifest.Name, instance.Manifest.Version, instance.BuildTime); err != nil {
			logger.Warn("Failed to register plugin in registry",
				"plugin", instance.ID,
				"error", err)
		}
	}

	// Print plugin summary table (shows ALL plugins, not just enrichers)
	allPlugins := pm.GetAllPlugins()
	if len(allPlugins) > 0 {
		pm.PrintTable(os.Stderr, fmt.Sprintf("Plugins (%d loaded)", len(allPlugins)))
	}
}
