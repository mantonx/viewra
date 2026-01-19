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
	Marketplace      *handlers.MarketplaceHandler
	System           *handlers.SystemHandler

	// Home screen handler
	Home *handlers.HomeHandler

	// Trending data handler
	Trending *handlers.TrendingHandler

	// Ratings handler (user favorites, likes, dislikes)
	Ratings *handlers.RatingsHandler

	// PluginProxy proxies HTTP requests to plugin-defined routes.
	PluginProxy *plugins.HTTPProxy

	// Search handles /api/search. Plugins can override this via capability routing.
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

	// Health checks (public)
	s.router.GET("/health", h.Health.Check)
	s.router.GET("/health/live", h.Health.Live)
	s.router.GET("/health/ready", h.Health.Ready)

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
	routes.RegisterPluginRoutes(protected, h.Plugins, h.Marketplace, h.PluginProxy, h.AuthValidator)

	// Register search route (plugins can override via capability routing)
	routes.RegisterSearchRoutes(protected, h.Search)

	// Register home screen routes (protected)
	routes.RegisterHomeRoutes(protected, h.Home)

	// Register trending routes (protected)
	routes.RegisterTrendingRoutes(protected, h.Trending)

	// Register ratings routes (protected)
	routes.RegisterRatingsRoutes(protected, h.Ratings)

	// Register dynamic capability alias routes from plugins
	// Plugins can declare alias_path in their routes (e.g., "/api/chat" for chat capability)
	// Note: /api/search is handled above by SearchHandler for fallback support
	if h.PluginProxy != nil {
		h.PluginProxy.RegisterCapabilityRoutes(protected)
	}

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
