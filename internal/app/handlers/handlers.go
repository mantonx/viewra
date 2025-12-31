package handlers

import (
	"database/sql"
	"log/slog"

	"github.com/mantonx/viewra/internal/api"
	"github.com/mantonx/viewra/internal/api/handlers"
	"github.com/mantonx/viewra/internal/app/config"
	"github.com/mantonx/viewra/internal/app/lifecycle"
	"github.com/mantonx/viewra/internal/app/repositories"
	"github.com/mantonx/viewra/internal/app/services"
	"github.com/mantonx/viewra/internal/app/usecases"
	appauth "github.com/mantonx/viewra/internal/application/auth"
	appplugins "github.com/mantonx/viewra/internal/application/plugins"
	appscheduler "github.com/mantonx/viewra/internal/application/scheduler"
	infraPlugins "github.com/mantonx/viewra/internal/infrastructure/plugins"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/registry"
	"github.com/mantonx/viewra/internal/infrastructure/streaming"
)

// InfrastructureDeps holds infrastructure dependencies needed by some handlers.
// These are for handlers that interact directly with infrastructure (health checks,
// file serving, transcoding) rather than pure business logic.
type InfrastructureDeps struct {
	DB                 *sql.DB
	SchedulerService   *appscheduler.Service
	TranscodeOutputDir string
	Repos              *repositories.Repositories
	Config             *config.Config
	LifecycleMgr       *lifecycle.Manager
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
	healthHandler := handlers.NewHealthHandler(infra.DB, infra.SchedulerService, svcs.TranscodeQueue)
	browserHandler := handlers.NewBrowserHandler(svcs.PathBrowser)
	schedulerHandler := handlers.NewSchedulerHandler(infra.SchedulerService)
	analyticsHandler := handlers.NewAnalyticsHandler(cases.Analytics, cases.TranscodeAnalytics)

	// Library handlers
	libraryHandler := handlers.NewLibraryHandler(cases.Library.Service, cases.Library.Scan)
	scanJobHandler := handlers.NewScanJobHandler(cases.ScanJob, cases.Library.Scan, cases.Library.Scan, svcs.EventBus, logger)

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
	peopleHandler := handlers.NewPeopleHandler(
		cases.People.GetPerson,
		cases.People.GetCreditsForEntity,
		cases.People.GetCreditsForPerson,
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
	var locationSettingsHandler *handlers.LocationSettingsHandler
	if svcs.Settings != nil {
		settingsHandler = handlers.NewSettingsHandler(svcs.Settings, infra.Repos.User)
		// Add database info for system info endpoint
		if infra.Config != nil {
			settingsHandler.SetDatabaseInfo(&handlers.DatabaseInfo{
				Driver:   infra.Config.Database.Driver,
				Host:     infra.Config.Database.Host,
				Port:     infra.Config.Database.Port,
				DBName:   infra.Config.Database.DBName,
				SSLMode:  infra.Config.Database.SSLMode,
				PoolSize: infra.Config.Database.Pool.MaxOpenConns,
			})
		}
	}

	// Location settings handler (for weather context)
	if svcs.LocationRepo != nil && svcs.WeatherService != nil {
		locationSettingsHandler = handlers.NewLocationSettingsHandler(svcs.LocationRepo, svcs.WeatherService)
	}

	// Enrichment handler
	var enrichmentHandler *handlers.EnrichmentHandler
	if svcs.PipelineManager != nil && svcs.EventBus != nil && infra.Repos.EnrichmentStatus != nil && infra.Repos.EnrichmentQueue != nil {
		enrichmentHandler = handlers.NewEnrichmentHandler(
			svcs.PipelineManager,
			infra.Repos.EnrichmentStatus,
			infra.Repos.EnrichmentQueue,
			svcs.EventBus,
			logger.With("handler", "enrichment"),
		)
		// Wire up media repository for bulk enqueue functionality
		if infra.Repos.Media != nil {
			enrichmentHandler.SetMediaListByType(infra.Repos.Media.ListByType)
		}
	}

	// Plugin handler
	var pluginHandler *handlers.PluginHandler
	if infra.Repos.Plugin != nil && svcs.PluginManager != nil && svcs.EventBus != nil {
		pluginService := appplugins.NewService(
			infra.Repos.Plugin,
			svcs.PluginManager,
			svcs.EventBus,
			logger.With("service", "plugins"),
		)
		// Wire encryption for sensitive plugin settings
		if svcs.Encryptor != nil {
			pluginService.SetEncryptor(svcs.Encryptor)
		}
		pluginHandler = handlers.NewPluginHandler(pluginService)
	}

	// Get the plugin HTTP proxy if plugin manager is available
	var pluginProxy *infraPlugins.HTTPProxy
	var capabilityRegistry *registry.CapabilityRegistry
	if svcs.PluginManager != nil {
		if proxy := svcs.PluginManager.GetHTTPProxy(); proxy != nil {
			pluginProxy, _ = proxy.(*infraPlugins.HTTPProxy)
		}
		capabilityRegistry = svcs.PluginManager.GetCapabilityRegistry()
	}

	// Create search handler with fallback to text search
	var searchHandler *handlers.SearchHandler
	if svcs.Search != nil {
		searchHandler = handlers.NewSearchHandler(capabilityRegistry, pluginProxy, svcs.Search)
	}

	// System handler (requires lifecycle manager)
	var systemHandler *handlers.SystemHandler
	if infra.LifecycleMgr != nil {
		systemHandler = handlers.NewSystemHandler(infra.LifecycleMgr, infra.DB)
		// Wire maintenance manager to health handler for readiness checks
		healthHandler.SetMaintenanceManager(systemHandler.GetMaintenanceManager())
		// Wire migration service with database info and config saver
		if infra.DB != nil && infra.Config != nil {
			configSaver := createConfigSaver(infra.Config.DataDir)
			systemHandler.SetMigrationService(infra.DB, infra.Config.Database.Driver, configSaver)
		}
	}

	return &api.Handlers{
		Health:           healthHandler,
		Browser:          browserHandler,
		Scheduler:        schedulerHandler,
		Analytics:        analyticsHandler,
		Library:          libraryHandler,
		ScanJob:          scanJobHandler,
		Media:            mediaHandler,
		Stream:           streamHandler,
		Progress:         progressHandler,
		Subtitle:         subtitleHandler,
		Images:           imagesHandler,
		Transcode:        transcodeHandler,
		FFmpegLogs:       ffmpegLogsHandler,
		Movies:           moviesHandler,
		TV:               tvHandler,
		Music:            musicHandler,
		People:           peopleHandler,
		Auth:             authHandler,
		Users:            usersHandler,
		Settings:         settingsHandler,
		LocationSettings: locationSettingsHandler,
		Enrichment:       enrichmentHandler,
		Plugins:          pluginHandler,
		System:           systemHandler,
		PluginProxy:      pluginProxy,
		Search:           searchHandler,
		AuthValidator:    authService,
	}
}

// createConfigSaver creates a function that saves database configuration after migration.
func createConfigSaver(dataDir string) func(driver string, pgHost string, pgPort int, pgUser, pgDatabase, pgSSLMode string) error {
	return func(driver string, pgHost string, pgPort int, pgUser, pgDatabase, pgSSLMode string) error {
		var fileConfig *config.FileConfig
		switch driver {
		case "postgres", "postgresql":
			fileConfig = config.GenerateConfigForDatabase("postgres", "", pgHost, pgPort, pgUser, pgDatabase, pgSSLMode)
		case "sqlite", "sqlite3":
			// For SQLite, pgDatabase is actually the path
			fileConfig = config.GenerateConfigForDatabase("sqlite", pgDatabase, "", 0, "", "", "")
		default:
			return nil
		}
		return config.SaveConfigFile(dataDir, fileConfig)
	}
}
