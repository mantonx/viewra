package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/viewra/viewra/internal/api"
	"github.com/viewra/viewra/internal/app"
)

// Application holds all application dependencies
type Application struct {
	Config           *Config
	Logger           *slog.Logger
	Database         *DatabaseConnection
	Server           *api.Server
	Container        *app.Container
}

// Initialize sets up the application with all dependencies
func Initialize() (*Application, error) {
	// Load configuration
	cfg := LoadConfig()

	// Initialize logger
	logger := NewLogger(cfg.Environment)
	logger.Info("Starting ViewRA Media Server", "version", "0.0.1", "environment", cfg.Environment)

	// Initialize database
	dbConn, err := InitializeDatabase(cfg.Database, logger)
	if err != nil {
		return nil, err
	}

	// Run startup tasks
	ctx := context.Background()
	if err := RunStartupTasks(ctx, dbConn.DB, dbConn.Driver, cfg, logger); err != nil {
		dbConn.Close(logger)
		return nil, err
	}

	// Initialize application container with all dependencies
	container := app.NewContainer(dbConn.DB, dbConn.Driver, cfg.Server, logger)

	// Add Swagger documentation endpoint
	container.Server.Router().GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return &Application{
		Config:    cfg,
		Logger:    logger,
		Database:  dbConn,
		Server:    container.Server,
		Container: container,
	}, nil
}

// Run starts the HTTP server and handles graceful shutdown
func (a *Application) Run() error {
	// Ensure database is closed on exit
	defer a.Database.Close(a.Logger)

	// Start cleanup scheduler if available
	if a.Container.CleanupScheduler != nil {
		ctx := context.Background()
		a.Container.CleanupScheduler.Start(ctx)
		defer a.Container.CleanupScheduler.Stop()
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

	if err := a.Server.Shutdown(ctx); err != nil {
		a.Logger.Error("Server forced shutdown", "error", err)
		return err
	}

	a.Logger.Info("Server stopped gracefully")
	return nil
}
