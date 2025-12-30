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
	"github.com/mantonx/viewra/internal/infrastructure/plugins"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/registry"
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
	Health           *handlers.HealthHandler
	Browser          *handlers.BrowserHandler
	Scheduler        *handlers.SchedulerHandler
	Analytics        *handlers.AnalyticsHandler
	Library          *handlers.LibraryHandler
	ScanJob          *handlers.ScanJobHandler
	Media            *handlers.MediaHandler
	Stream           *handlers.StreamHandler
	Progress         *handlers.ProgressHandler
	Images           *handlers.ImagesHandler
	Transcode        *handlers.TranscodeHandler
	FFmpegLogs       *handlers.FFmpegLogsHandler
	Subtitle         *handlers.SubtitleHandler
	Movies           *handlers.MoviesHandler
	TV               *handlers.TVHandler
	Music            *handlers.MusicHandler
	People           *handlers.PeopleHandler
	Auth             *handlers.AuthHandler
	Users            *handlers.UsersHandler
	Settings         *handlers.SettingsHandler
	LocationSettings *handlers.LocationSettingsHandler
	Enrichment       *handlers.EnrichmentHandler
	Plugins          *handlers.PluginHandler
	System           *handlers.SystemHandler

	// PluginProxy proxies HTTP requests to plugin-defined routes.
	// This replaces the hardcoded AISearchHandler.
	PluginProxy *plugins.HTTPProxy

	// Search handles /api/search with fallback to text search when no plugin provides semantic search.
	Search *handlers.SearchHandler

	// AuthValidator is used by routes to set up auth middleware
	AuthValidator middleware.AuthValidator

	// AuthRateLimiters provides rate limiting for auth endpoints
	AuthRateLimiters *middleware.AuthRateLimiters
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

	// Health check (public)
	s.router.GET("/health", h.Health.Check)

	// API v1 routes
	api := s.router.Group("/api")

	// Register auth routes first (public endpoints for login/setup)
	if h.Auth != nil && h.AuthValidator != nil {
		rateLimiters := h.AuthRateLimiters
		if rateLimiters == nil {
			rateLimiters = middleware.NewAuthRateLimiters()
		}
		routes.RegisterAuthRoutes(api, h.Auth, h.Users, h.AuthValidator, rateLimiters)
	}

	// Create protected route group (requires authentication)
	var protected *gin.RouterGroup
	if h.AuthValidator != nil {
		protected = api.Group("")
		protected.Use(middleware.RequireAuth(h.AuthValidator))
	} else {
		// If auth is not configured, routes are public
		protected = api
	}

	// Register protected route groups
	routes.RegisterLibraryRoutes(protected, h.Library, h.ScanJob, h.Enrichment)
	routes.RegisterMediaRoutes(protected, h.Media)
	routes.RegisterSubtitleRoutes(protected, h.Subtitle)
	routes.RegisterStreamRoutes(protected, h.Stream)
	routes.RegisterBrowserRoutes(protected, h.Browser)
	routes.RegisterProgressRoutes(protected, h.Progress)
	routes.RegisterTranscodeRoutes(protected, h.Transcode)
	routes.RegisterFFmpegLogRoutes(protected, h.FFmpegLogs)

	// Register media-type specific routes (protected)
	routes.RegisterMoviesRoutes(protected, h.Movies)
	routes.RegisterTVRoutes(protected, h.TV)
	routes.RegisterMusicRoutes(protected, h.Music)
	routes.RegisterPeopleRoutes(protected, h.People)

	// Register analytics routes (protected)
	routes.RegisterAnalyticsRoutes(protected, h.Analytics)

	// Register settings routes (protected, with admin requirement for system settings)
	routes.RegisterSettingsRoutesWithLocation(protected, h.Settings, h.LocationSettings, h.AuthValidator)

	// Register enrichment routes (protected)
	routes.RegisterEnrichmentRoutes(protected, h.Enrichment)

	// Register plugin routes (protected, with admin requirement for mutations)
	// This also registers plugin custom routes via the HTTP proxy
	routes.RegisterPluginRoutes(protected, h.Plugins, h.PluginProxy, h.AuthValidator)

	// Register search route with fallback support
	// This is handled separately from capability aliases because SearchHandler
	// provides fallback to text search when no semantic search plugin is available
	routes.RegisterSearchRoutes(protected, h.Search)

	// Register capability alias routes for other plugins (excluding semantic_search)
	s.registerCapabilityAliases(protected)

	// Register image routes (protected via api group)
	routes.RegisterImageRoutes(s.router, h.Images)

	// Register adaptive quality routes
	routes.RegisterAdaptiveQualityRoutes(s.router, s.logger)

	// Register admin routes (requires admin)
	if h.AuthValidator != nil {
		admin := api.Group("/admin")
		admin.Use(middleware.RequireAdmin(h.AuthValidator))
		routes.RegisterSchedulerRoutes(admin, h.Scheduler)
		routes.RegisterSystemRoutes(admin, h.System, h.AuthValidator)
	} else {
		admin := api.Group("/admin")
		routes.RegisterSchedulerRoutes(admin, h.Scheduler)
		routes.RegisterSystemRoutes(admin, h.System, nil)
	}
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

// registerCapabilityAliases sets up capability alias routes for plugins.
// Note: semantic_search is handled separately by RegisterSearchRoutes which provides fallback.
func (s *Server) registerCapabilityAliases(protected *gin.RouterGroup) {
	proxy := s.handlers.PluginProxy
	if proxy == nil {
		s.logger.Debug("plugin proxy not configured, skipping capability alias registration")
		return
	}

	// Register capability alias routes (excluding semantic_search which has fallback support)
	// These go on the parent protected group to get stable URLs
	for capability, aliasPath := range registry.CapabilityAliases {
		// Skip semantic_search - it's handled by SearchHandler with fallback support
		if capability == "semantic_search" {
			continue
		}

		// Handle both the exact path and with trailing wildcard for sub-paths
		cap := capability // capture for closure

		// Register the base path
		basePath := aliasPath[4:] // strip "/api" prefix since we're in protected group
		if basePath == "" {
			basePath = "/"
		}

		handler := func(c *gin.Context) {
			proxy.HandleCapabilityRoute(cap)(c)
		}

		// Register for all HTTP methods
		protected.Any(basePath, handler)
		protected.Any(basePath+"/*subpath", handler)

		s.logger.Debug("registered capability alias",
			"capability", capability,
			"path", aliasPath)
	}
}
