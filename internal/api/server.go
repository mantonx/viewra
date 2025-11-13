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
	"github.com/viewra/viewra/internal/api/handlers"
	"github.com/viewra/viewra/internal/api/middleware"
	"github.com/viewra/viewra/internal/api/routes"
	"github.com/viewra/viewra/internal/application/library"
	"github.com/viewra/viewra/internal/application/media"
	"github.com/viewra/viewra/internal/infrastructure/streaming"
)

// Server represents the HTTP server
type Server struct {
	router          *gin.Engine
	healthHandler   *handlers.HealthHandler
	libraryHandler  *handlers.LibraryHandler
	mediaHandler    *handlers.MediaHandler
	streamHandler   *handlers.StreamHandler
	browserHandler  *handlers.BrowserHandler
	scanJobHandler  *handlers.ScanJobHandler
	progressHandler *handlers.ProgressHandler
	server          *http.Server
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	Browser         BrowserConfig
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

// NewServer creates a new HTTP server
func NewServer(
	config ServerConfig,
	logger *slog.Logger,
	healthHandler *handlers.HealthHandler,
	browserHandler *handlers.BrowserHandler,
	scanJobHandler *handlers.ScanJobHandler,
	progressHandler *handlers.ProgressHandler,
	// Library use cases
	createLibrary *library.CreateLibraryUseCase,
	updateLibrary *library.UpdateLibraryUseCase,
	deleteLibrary *library.DeleteLibraryUseCase,
	getLibrary *library.GetLibraryUseCase,
	listLibraries *library.ListLibrariesUseCase,
	scanLibrary *library.ScanLibraryUseCase,
	// Media use cases
	getMedia *media.GetMediaUseCase,
	listMedia *media.ListMediaUseCase,
) *Server {
	// Set Gin to release mode in production
	// gin.SetMode(gin.ReleaseMode)

	// Create router without default middleware
	router := gin.New()

	// Add recovery middleware (panic recovery)
	router.Use(gin.Recovery())

	// Add CORS middleware (for frontend development)
	router.Use(middleware.CORS())

	// Add our custom logging middleware
	router.Use(middleware.Logger(logger))

	// Create handlers with individual use cases
	libraryHandler := handlers.NewLibraryHandler(
		createLibrary,
		updateLibrary,
		deleteLibrary,
		getLibrary,
		listLibraries,
		scanLibrary,
	)
	mediaHandler := handlers.NewMediaHandler(
		getMedia,
		listMedia,
	)
	streamService := streaming.NewService()
	streamHandler := handlers.NewStreamHandler(getMedia, streamService)

	server := &Server{
		router:         router,
		healthHandler:  healthHandler,
		libraryHandler: libraryHandler,
		mediaHandler:   mediaHandler,
		streamHandler:  streamHandler,
		browserHandler: browserHandler,
		scanJobHandler: scanJobHandler,
		progressHandler: progressHandler,
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
	// Health check
	s.router.GET("/health", s.healthHandler.Check)

	// API v1 routes
	api := s.router.Group("/api")

	// Register route groups
	routes.RegisterLibraryRoutes(api, s.libraryHandler, s.scanJobHandler)
	routes.RegisterMediaRoutes(api, s.mediaHandler)
	routes.RegisterStreamRoutes(api, s.streamHandler)
	routes.RegisterBrowserRoutes(api, s.browserHandler)
	routes.RegisterProgressRoutes(api, s.progressHandler)
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
