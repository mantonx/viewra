package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/handlers"
	"github.com/mantonx/viewra/internal/api/middleware"
	"github.com/mantonx/viewra/internal/api/routes"
)

// Server represents the HTTP server
type Server struct {
	router   *gin.Engine
	logger   *slog.Logger
	handlers *Handlers
	server   *http.Server
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port                 int
	ReadTimeout          time.Duration
	WriteTimeout         time.Duration
	ShutdownTimeout      time.Duration
	Browser              BrowserConfig
	CORSAllowedOrigins   []string
	CORSAllowCredentials bool
}

// BrowserConfig holds filesystem browser configuration
type BrowserConfig struct {
	AllowedBasePaths []string
	DefaultBasePath  string
}

// DefaultServerConfig returns sensible defaults
func DefaultServerConfig() ServerConfig {
	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		homeDir = "/home"
	}

	return ServerConfig{
		Port:            8080,
		ReadTimeout:     10 * time.Second,
		WriteTimeout:    10 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		Browser: BrowserConfig{
			AllowedBasePaths: []string{
				homeDir, // Allow home directory as starting point
				"/media",
				"/mnt",
				"/cifs",
				filepath.Join(homeDir, "Videos"),
				filepath.Join(homeDir, "Movies"),
				filepath.Join(homeDir, "Music"),
			},
			DefaultBasePath: homeDir,
		},
	}
}

// Handlers holds all HTTP handlers. This type alias allows the api package
// to accept handlers without importing the app/handlers package.
type Handlers struct {
	Health    *handlers.HealthHandler
	Browser   *handlers.BrowserHandler
	Scheduler *handlers.SchedulerHandler
	Analytics *handlers.AnalyticsHandler
	Library   *handlers.LibraryHandler
	ScanJob   *handlers.ScanJobHandler
	Media     *handlers.MediaHandler
	Stream    *handlers.StreamHandler
	Progress  *handlers.ProgressHandler
	Images    *handlers.ImagesHandler
	Transcode *handlers.TranscodeHandler
	Movies    *handlers.MoviesHandler
	TV        *handlers.TVHandler
	Music     *handlers.MusicHandler
}

// NewServer creates a new HTTP server with the provided configuration and handlers.
func NewServer(config ServerConfig, logger *slog.Logger, h *Handlers) *Server {
	// Create router without default middleware
	router := gin.New()

	// Add recovery middleware (panic recovery)
	router.Use(gin.Recovery())

	// Add gzip compression middleware
	router.Use(middleware.Gzip())

	// Add request ID middleware (must be before logger to include ID in logs)
	router.Use(middleware.RequestID(logger))

	// Add CORS middleware with configuration
	router.Use(middleware.CORS(middleware.CORSConfig{
		AllowedOrigins:   config.CORSAllowedOrigins,
		AllowCredentials: config.CORSAllowCredentials,
	}))

	// Add our custom logging middleware
	router.Use(middleware.Logger(logger))

	server := &Server{
		router:   router,
		logger:   logger,
		handlers: h,
	}

	// Setup routes
	server.setupRoutes()

	// Create HTTP server
	server.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", config.Port),
		Handler:      router,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	}

	return server
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	h := s.handlers

	// Health check
	s.router.GET("/health", h.Health.Check)

	// API v1 routes
	api := s.router.Group("/api")

	// Register route groups
	routes.RegisterLibraryRoutes(api, h.Library, h.ScanJob)
	routes.RegisterMediaRoutes(api, h.Media)
	routes.RegisterStreamRoutes(api, h.Stream)
	routes.RegisterBrowserRoutes(api, h.Browser)
	routes.RegisterProgressRoutes(api, h.Progress)
	routes.RegisterTranscodeRoutes(api, h.Transcode)

	// Register media-type specific routes
	routes.RegisterMoviesRoutes(api, h.Movies)
	routes.RegisterTVRoutes(api, h.TV)
	routes.RegisterMusicRoutes(api, h.Music)

	// Register image routes
	routes.RegisterImageRoutes(s.router, h.Images)

	// Register adaptive quality routes
	routes.RegisterAdaptiveQualityRoutes(s.router, s.logger)

	// Register analytics routes
	routes.RegisterAnalyticsRoutes(api, h.Analytics)

	// Register admin routes
	admin := api.Group("/admin")
	routes.RegisterSchedulerRoutes(admin, h.Scheduler)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// Router returns the Gin router (for testing)
func (s *Server) Router() *gin.Engine {
	return s.router
}
