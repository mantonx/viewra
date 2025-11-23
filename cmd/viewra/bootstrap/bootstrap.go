package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/mantonx/viewra/internal/api"
	"github.com/mantonx/viewra/internal/app"
	appconfig "github.com/mantonx/viewra/internal/app/config"
	"github.com/mantonx/viewra/internal/pkg/logger"
	"github.com/mantonx/viewra/web"
)

// Application holds all application dependencies
type Application struct {
	Config           *appconfig.Config
	Logger           *slog.Logger
	Database         *DatabaseConnection
	Server           *api.Server
	Container        *app.Container
}

// Initialize sets up the application with all dependencies
func Initialize() (*Application, error) {
	// Load configuration from environment
	cfg, err := appconfig.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize logger
	lgr := logger.New(cfg.Environment)
	lgr.Info("Starting ViewRA Media Server", "version", "0.0.1", "environment", cfg.Environment)

	// Initialize database
	dbConn, err := InitializeDatabaseFromConfig(&cfg.Database, lgr)
	if err != nil {
		return nil, err
	}

	// Run startup tasks
	ctx := context.Background()
	if err := RunStartupTasksFromConfig(ctx, dbConn.DB, dbConn.Driver, cfg, lgr); err != nil {
		dbConn.Close(lgr)
		return nil, err
	}

	// Initialize application container with all dependencies
	container := app.NewContainer(dbConn.DB, dbConn.Driver, cfg, lgr)

	// Add Swagger documentation endpoint
	container.Server.Router().GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Serve embedded frontend in production (if available)
	if web.IsEmbedded() {
		distFS, err := web.FS()
		if err != nil {
			lgr.Warn("Frontend embedded but failed to load", "error", err)
		} else {
			lgr.Info("Serving embedded frontend at http://localhost:8080/")
			container.Server.Router().StaticFS("/", http.FS(distFS))
		}
	} else {
		lgr.Info("Frontend not embedded - development mode (use Vite on :5173)")
	}

	return &Application{
		Config:    cfg,
		Logger:    lgr,
		Database:  dbConn,
		Server:    container.Server,
		Container: container,
	}, nil
}

// Run starts the HTTP server and handles graceful shutdown
func (a *Application) Run() error {
	// Ensure database is closed on exit
	defer a.Database.Close(a.Logger)

	// Start transcode queue if available
	if a.Container.TranscodeQueue != nil {
		ctx := context.Background()
		if err := a.Container.TranscodeQueue.Start(ctx); err != nil {
			a.Logger.Error("Failed to start transcode queue", "error", err)
		} else {
			a.Logger.Info("Transcode queue started")
		}
	}

	// Start unified task scheduler if available
	// (includes transcode cleanup and image cleanup tasks)
	if a.Container.Scheduler != nil {
		ctx := context.Background()
		go func() {
			if err := a.Container.Scheduler.Start(ctx); err != nil {
				a.Logger.Error("Scheduler error", "error", err)
			}
		}()
		a.Logger.Info("Unified task scheduler started")
	}

	// Start server in goroutine
	go func() {
		a.Logger.Info("HTTP server starting",
			"port", a.Config.Server.Port,
			"swagger", fmt.Sprintf("http://localhost:%d/swagger/index.html", a.Config.Server.Port))
		if err := a.Server.Start(); err != nil {
			a.Logger.Error("Server error", "error", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	a.Logger.Info("Shutdown signal received", "signal", sig.String())

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.Container.Shutdown(ctx); err != nil {
		a.Logger.Error("Container forced shutdown", "error", err)
		return err
	}

	a.Logger.Info("Application stopped gracefully")
	return nil
}
