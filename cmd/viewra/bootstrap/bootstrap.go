package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	_ "net/http/pprof" // Register pprof handlers
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/mantonx/viewra/internal/app"
	appconfig "github.com/mantonx/viewra/internal/app/config"
	"github.com/mantonx/viewra/internal/pkg/logger"
	"github.com/mantonx/viewra/web"
)

// Application holds all application dependencies and manages lifecycle.
// Config, Logger, Database handle runtime concerns.
// Container holds the dependency injection graph for business logic.
type Application struct {
	Config    *appconfig.Config
	Logger    *slog.Logger
	Database  *DatabaseConnection
	Container *app.Container
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

	// Run pre-container startup tasks (system profiling, migrations)
	ctx := context.Background()
	if err := RunPreContainerTasks(ctx, dbConn.DB, cfg, lgr); err != nil {
		dbConn.Close(lgr)
		return nil, err
	}

	// Initialize application container with all dependencies
	container := app.NewContainer(dbConn.DB, dbConn.Driver, cfg, lgr)

	// Run post-container startup tasks (reuses container's services)
	RunPostContainerTasks(ctx, container.UseCases.Library.Scan, lgr)

	// Add Swagger documentation endpoint
	container.Server.Router().GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Serve embedded frontend in production (if available)
	if web.IsEmbedded() {
		distFS, err := web.FS()
		if err != nil {
			lgr.Warn("Frontend embedded but failed to load", "error", err)
		} else {
			lgr.Info("Serving embedded frontend at http://localhost:8080/")
			// Serve frontend files - use NoRoute to catch all non-API routes
			httpFS := http.FS(distFS)
			container.Server.Router().NoRoute(func(c *gin.Context) {
				// Serve index.html for root or any path that doesn't look like an API call
				if c.Request.URL.Path == "/" || !isAPIPath(c.Request.URL.Path) {
					c.FileFromFS(c.Request.URL.Path, httpFS)
				}
			})
		}
	} else {
		lgr.Info("Frontend not embedded - development mode (use Vite on :5173)")
	}

	return &Application{
		Config:    cfg,
		Logger:    lgr,
		Database:  dbConn,
		Container: container,
	}, nil
}

// Run starts the HTTP server and handles graceful shutdown
func (a *Application) Run() error {
	// Ensure database is closed on exit
	defer a.Database.Close(a.Logger)

	// Start pprof server on a separate port for debugging
	go func() {
		// Add endpoint to force memory release to OS
		http.HandleFunc("/debug/freeOSMemory", func(w http.ResponseWriter, r *http.Request) {
			debug.FreeOSMemory()
			w.Write([]byte("Memory released to OS\n"))
		})
		a.Logger.Info("pprof server starting", "url", "http://localhost:6060/debug/pprof/")
		if err := http.ListenAndServe(":6060", nil); err != nil {
			a.Logger.Error("pprof server error", "error", err)
		}
	}()

	// Start transcode queue if available
	if a.Container.TranscodeQueue != nil {
		ctx := context.Background()
		if err := a.Container.TranscodeQueue.Start(ctx); err != nil {
			a.Logger.Error("Failed to start transcode queue", "error", err)
		} else {
			a.Logger.Info("Transcode queue started")
		}
	}

	// Start scheduler service if available
	// (includes transcode cleanup, image cleanup, session cleanup tasks)
	if a.Container.SchedulerService != nil {
		ctx := context.Background()
		go func() {
			if err := a.Container.SchedulerService.Start(ctx); err != nil {
				a.Logger.Error("Scheduler error", "error", err)
			}
		}()
	}

	// Start server in goroutine
	go func() {
		a.Logger.Info("HTTP server starting",
			"port", a.Config.Server.Port,
			"swagger", fmt.Sprintf("http://localhost:%d/swagger/index.html", a.Config.Server.Port))
		if err := a.Container.Server.Start(); err != nil {
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

// isAPIPath checks if a path is an API endpoint
func isAPIPath(path string) bool {
	return strings.HasPrefix(path, "/api/") ||
		strings.HasPrefix(path, "/swagger/") ||
		strings.HasPrefix(path, "/health")
}
