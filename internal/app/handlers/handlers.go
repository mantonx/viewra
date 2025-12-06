package handlers

import (
	"database/sql"
	"log/slog"

	"github.com/mantonx/viewra/internal/api"
	"github.com/mantonx/viewra/internal/api/handlers"
	"github.com/mantonx/viewra/internal/app/config"
	"github.com/mantonx/viewra/internal/app/repositories"
	"github.com/mantonx/viewra/internal/app/services"
	"github.com/mantonx/viewra/internal/app/usecases"
	appauth "github.com/mantonx/viewra/internal/application/auth"
	"github.com/mantonx/viewra/internal/infrastructure/scheduler"
	"github.com/mantonx/viewra/internal/infrastructure/streaming"
)

// InfrastructureDeps holds infrastructure dependencies needed by some handlers.
// These are for handlers that interact directly with infrastructure (health checks,
// file serving, transcoding) rather than pure business logic.
type InfrastructureDeps struct {
	DB                 *sql.DB
	Scheduler          *scheduler.Scheduler
	TranscodeOutputDir string
	Repos              *repositories.Repositories
	Config             *config.Config
}

// BuildHandlers creates all HTTP handler instances.
// This is the single place where all handlers are instantiated.
// Returns *api.Handlers directly to avoid duplicate struct definitions.
func BuildHandlers(
	infra *InfrastructureDeps,
	svcs *services.Services,
	cases *usecases.UseCases,
	logger *slog.Logger,
) *api.Handlers {
	// Infrastructure handlers
	healthHandler := handlers.NewHealthHandler(infra.DB, infra.Scheduler, svcs.TranscodeQueue)
	browserHandler := handlers.NewBrowserHandler(svcs.PathBrowser)
	schedulerHandler := handlers.NewSchedulerHandler(infra.Scheduler)
	analyticsHandler := handlers.NewAnalyticsHandler(cases.Analytics)

	// Library handlers
	libraryHandler := handlers.NewLibraryHandler(cases.Library.Service, cases.Library.Scan)
	scanJobHandler := handlers.NewScanJobHandler(cases.ScanJob, cases.Library.Scan, cases.Library.Scan, logger)

	// Media handlers
	mediaHandler := handlers.NewMediaHandler(cases.Media.Get, cases.Media.List, cases.Media.StreamInfo, cases.Media.GetTracks)
	streamService := streaming.NewService()
	streamHandler := handlers.NewStreamHandler(cases.Media.Get, streamService, logger)
	progressHandler := handlers.NewProgressHandler(cases.Progress)
	subtitleHandler := handlers.NewSubtitleHandler(cases.Media.Get, cases.Media.GetTracks, svcs.SubtitleConverter)
	imagesHandler := handlers.NewImagesHandler(
		cases.Images.Get,
		cases.Images.GetMedia,
		cases.Images.GetEntity,
		cases.Images.GetBatch,
		svcs.ImageTransformer,
		svcs.ImageCache,
	)

	// Transcode handler (if transcode is enabled)
	var transcodeHandler *handlers.TranscodeHandler
	var ffmpegLogsHandler *handlers.FFmpegLogsHandler
	if svcs.TranscodeQueue != nil && cases.Transcode.CreateJob != nil {
		transcodeHandler = handlers.NewTranscodeHandler(
			cases.Transcode.CreateJob,
			cases.Transcode.GetStatus,
			cases.Transcode.ServeManifest,
			cases.Transcode.ServeMasterPlaylist,
			cases.Media.Get,
			cases.Media.GetTracks,
			svcs.TranscodeQueue,
			svcs.CleanupService,
			svcs.SessionManager,
			infra.TranscodeOutputDir,
			svcs.SubtitleConverter,
		)

		// FFmpeg logs handler (requires session manager)
		ffmpegLogsHandler = handlers.NewFFmpegLogsHandler(svcs.SessionManager)
	}

	// Media-type specific handlers
	moviesHandler := handlers.NewMoviesHandler(
		cases.Movies.List,
		cases.Movies.Get,
		cases.Movies.Search,
		cases.Movies.ListIDs,
	)
	tvHandler := handlers.NewTVHandler(
		cases.TV.ListShows,
		cases.TV.GetShow,
		cases.TV.ListEpisodes,
		cases.TV.GetEpisode,
		cases.TV.SearchEpisodes,
		cases.TV.ListShowIDs,
		cases.TV.GetNextEpisode,
	)
	musicHandler := handlers.NewMusicHandler(
		cases.Music.ListArtists,
		cases.Music.ListAlbumsByArtist,
		cases.Music.ListTracksByAlbum,
		cases.Music.GetTrack,
		cases.Music.SearchTracks,
		cases.Music.ListArtistIDs,
	)

	// Auth handlers
	var authHandler *handlers.AuthHandler
	var usersHandler *handlers.UsersHandler
	var authService *appauth.Service

	if infra.Repos != nil && infra.Repos.User != nil && svcs.TokenService != nil {
		// Create auth service
		authService = appauth.NewService(
			infra.Repos.User,
			infra.Repos.Session,
			svcs.PasswordHasher,
			svcs.TokenService,
			infra.Config.Auth.MaxSessionsPerUser,
		)

		// Create admin service
		adminService := appauth.NewAdminService(
			infra.Repos.User,
			infra.Repos.Session,
			svcs.PasswordHasher,
		)

		authHandler = handlers.NewAuthHandler(authService)
		usersHandler = handlers.NewUsersHandler(adminService)
	}

	// Settings handler
	var settingsHandler *handlers.SettingsHandler
	if svcs.Settings != nil {
		settingsHandler = handlers.NewSettingsHandler(svcs.Settings)
	}

	return &api.Handlers{
		Health:        healthHandler,
		Browser:       browserHandler,
		Scheduler:     schedulerHandler,
		Analytics:     analyticsHandler,
		Library:       libraryHandler,
		ScanJob:       scanJobHandler,
		Media:         mediaHandler,
		Stream:        streamHandler,
		Progress:      progressHandler,
		Subtitle:      subtitleHandler,
		Images:        imagesHandler,
		Transcode:     transcodeHandler,
		FFmpegLogs:    ffmpegLogsHandler,
		Movies:        moviesHandler,
		TV:            tvHandler,
		Music:         musicHandler,
		Auth:          authHandler,
		Users:         usersHandler,
		Settings:      settingsHandler,
		AuthValidator: authService,
	}
}
