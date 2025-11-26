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
	"github.com/mantonx/viewra/internal/application/library"
	"github.com/mantonx/viewra/internal/application/media"
	"github.com/mantonx/viewra/internal/application/movies"
	"github.com/mantonx/viewra/internal/application/music"
	"github.com/mantonx/viewra/internal/application/tv"
	"github.com/mantonx/viewra/internal/infrastructure/streaming"
)

// Server represents the HTTP server
type Server struct {
	router            *gin.Engine
	logger            *slog.Logger
	healthHandler     *handlers.HealthHandler
	libraryHandler    *handlers.LibraryHandler
	mediaHandler      *handlers.MediaHandler
	streamHandler     *handlers.StreamHandler
	browserHandler    *handlers.BrowserHandler
	scanJobHandler    *handlers.ScanJobHandler
	progressHandler   *handlers.ProgressHandler
	transcodeHandler  *handlers.TranscodeHandler
	moviesHandler     *handlers.MoviesHandler
	tvHandler         *handlers.TVHandler
	musicHandler      *handlers.MusicHandler
	imagesHandler     *handlers.ImagesHandler
	schedulerHandler  *handlers.SchedulerHandler
	analyticsHandler  *handlers.AnalyticsHandler
	server            *http.Server
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

// NewServer creates a new HTTP server
func NewServer(
	config ServerConfig,
	logger *slog.Logger,
	healthHandler *handlers.HealthHandler,
	browserHandler *handlers.BrowserHandler,
	scanJobHandler *handlers.ScanJobHandler,
	progressHandler *handlers.ProgressHandler,
	transcodeHandler *handlers.TranscodeHandler,
	imagesHandler *handlers.ImagesHandler,
	schedulerHandler *handlers.SchedulerHandler,
	analyticsHandler *handlers.AnalyticsHandler,
	// Library use cases
	libraryService *library.LibraryService,
	scanLibrary *library.ScanLibraryUseCase,
	// Media use cases
	getMedia *media.GetMediaUseCase,
	listMedia *media.ListMediaUseCase,
	// Movie use cases
	listMovies *movies.ListMoviesUseCase,
	getMovie *movies.GetMovieUseCase,
	searchMovies *movies.SearchMoviesUseCase,
	listMovieIDs *movies.ListMovieIDsUseCase,
	// TV use cases
	listTVShows *tv.ListTVShowsUseCase,
	getTVShow *tv.GetTVShowUseCase,
	listTVEpisodes *tv.ListTVEpisodesUseCase,
	getTVEpisode *tv.GetTVEpisodeUseCase,
	searchTVEpisodes *tv.SearchTVEpisodesUseCase,
	listTVShowIDs *tv.ListTVShowIDsUseCase,
	getNextEpisode *tv.GetNextEpisodeUseCase,
	// Music use cases
	listArtists *music.ListArtistsUseCase,
	listAlbumsByArtistID *music.ListAlbumsByArtistIDUseCase,
	listTracksByAlbumID *music.ListTracksByAlbumIDUseCase,
	getTrack *music.GetTrackUseCase,
	searchTracks *music.SearchTracksUseCase,
	listArtistIDs *music.ListArtistIDsUseCase,
) *Server {
	// Set Gin to release mode in production
	// gin.SetMode(gin.ReleaseMode)

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

	// Create handlers with services
	libraryHandler := handlers.NewLibraryHandler(
		libraryService,
		scanLibrary,
	)
	mediaHandler := handlers.NewMediaHandler(
		getMedia,
		listMedia,
	)
	streamService := streaming.NewService()
	streamHandler := handlers.NewStreamHandler(getMedia, streamService, logger)

	// Create media-type specific handlers
	moviesHandler := handlers.NewMoviesHandler(
		listMovies,
		getMovie,
		searchMovies,
		listMovieIDs,
	)
	tvHandler := handlers.NewTVHandler(
		listTVShows,
		getTVShow,
		listTVEpisodes,
		getTVEpisode,
		searchTVEpisodes,
		listTVShowIDs,
		getNextEpisode,
	)
	musicHandler := handlers.NewMusicHandler(
		listArtists,
		listAlbumsByArtistID,
		listTracksByAlbumID,
		getTrack,
		searchTracks,
		listArtistIDs,
	)

	server := &Server{
		router:            router,
		logger:            logger,
		healthHandler:     healthHandler,
		libraryHandler:    libraryHandler,
		mediaHandler:      mediaHandler,
		streamHandler:     streamHandler,
		browserHandler:    browserHandler,
		scanJobHandler:    scanJobHandler,
		progressHandler:   progressHandler,
		transcodeHandler:  transcodeHandler,
		moviesHandler:     moviesHandler,
		tvHandler:         tvHandler,
		musicHandler:      musicHandler,
		imagesHandler:     imagesHandler,
		schedulerHandler:  schedulerHandler,
		analyticsHandler:  analyticsHandler,
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
	routes.RegisterTranscodeRoutes(api, s.transcodeHandler)

	// Register media-type specific routes
	routes.RegisterMoviesRoutes(api, s.moviesHandler)
	routes.RegisterTVRoutes(api, s.tvHandler)
	routes.RegisterMusicRoutes(api, s.musicHandler)

	// Register image routes
	routes.RegisterImageRoutes(s.router, s.imagesHandler)

	// Register adaptive quality routes
	routes.RegisterAdaptiveQualityRoutes(s.router, s.logger)

	// Register analytics routes
	routes.RegisterAnalyticsRoutes(api, s.analyticsHandler)

	// Register admin routes
	admin := api.Group("/admin")
	routes.RegisterSchedulerRoutes(admin, s.schedulerHandler)
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
