package repositories

import (
	"database/sql"

	"github.com/mantonx/viewra/internal/infrastructure/database"
	analyticsRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/analytics"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
	enrichmentRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/enrichment"
	imageRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/image"
	keywordsRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/keywords"
	libraryRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/library"
	mediaRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/media"
	movieRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/movie"
	musicRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/music"
	peopleRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/people"
	pluginRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/plugins"
	progressRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/progress"
	scanJobRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/scanjob"
	scanStateRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/scanstate"
	searchRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/search"
	settingsRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/settings"
	studiosRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/studios"
	transcodeRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/transcode"
	transcodeAnalyticsRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/transcode_analytics"
	tvRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/tvshow"
	userRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/user"
	"github.com/mantonx/viewra/internal/infrastructure/plugins"
)

// Repositories holds all data access layer implementations.
// Groups all persistence repositories for dependency injection.
type Repositories struct {
	Library             *libraryRepo.Repository
	Media               *mediaRepo.Repository
	Progress            *progressRepo.Repository
	PlaybackPreferences *database.PlaybackPreferencesRepository
	ScanJob             *scanJobRepo.Repository
	Checkpoint          *scanJobRepo.CheckpointRepo
	ScanState           *scanStateRepo.Repository
	Transcode           *transcodeRepo.Repository
	Movie               *movieRepo.Repository
	TV                  *tvRepo.Repository
	Music               *musicRepo.Repository
	Image               *imageRepo.Repository
	Analytics           *analyticsRepo.Repository
	User                *userRepo.UserRepository
	Session             *userRepo.SessionRepository
	SystemSettings      *settingsRepo.SystemRepository
	UserSettings        *settingsRepo.UserRepository

	// Enrichment repositories
	EnrichmentQueue          *enrichmentRepo.QueueRepository
	EnrichmentStatus         *enrichmentRepo.StatusRepository
	EnrichmentPipeline       *enrichmentRepo.PipelineRepository
	EnrichmentExternalID     *enrichmentRepo.ExternalIDRepository
	EnrichmentMetadataSource *enrichmentRepo.MetadataSourceRepository

	// People and credits
	People *peopleRepo.Repository

	// Studios (production companies)
	Studios *studiosRepo.Repository

	// Keywords (location-based and thematic tags)
	Keywords *keywordsRepo.Repository

	// Search repository (for fallback search)
	Search *searchRepo.Repository

	// Plugin repositories
	Plugin             *pluginRepo.Repository
	PluginMediaQuerier plugins.MediaQuerier

	// Transcode analytics repository
	TranscodeAnalytics *transcodeAnalyticsRepo.Repository
}

// BuildRepositories creates and wires all repository instances using the provided database connection.
// All repositories share a common base repository for dual-database (SQLite/PostgreSQL) support.
func BuildRepositories(db *sql.DB, driver string) *Repositories {
	// Create base repository for dual-database support
	baseRepo := common.NewBaseRepository(db, driver)

	// Create core repositories
	libraryRepository := libraryRepo.NewRepository(db, driver)
	mediaRepository := mediaRepo.NewRepository(db, driver)
	progressRepository := progressRepo.NewRepository(db, driver)
	scanJobRepository := scanJobRepo.NewRepository(db, driver)
	checkpointRepository := scanJobRepo.NewCheckpointRepo(db, driver)
	scanStateRepository := scanStateRepo.NewRepository(db, driver)
	transcodeRepository := transcodeRepo.NewRepository(db, driver)

	// Create media-type specific repositories (with dependencies)
	movieRepository := movieRepo.NewRepository(baseRepo, mediaRepository)
	tvRepository := tvRepo.NewRepository(baseRepo, mediaRepository)
	musicRepository := musicRepo.NewRepository(baseRepo, mediaRepository)

	// Create image repository
	imageRepository := imageRepo.NewRepository(baseRepo)

	// Create analytics repository
	analyticsRepository := analyticsRepo.NewRepository(db, driver)

	// Create user repositories
	userRepository := userRepo.NewUserRepository(db, driver)
	sessionRepository := userRepo.NewSessionRepository(db, driver)

	// Create settings repositories
	systemSettingsRepository := settingsRepo.NewSystemRepository(db, driver)
	userSettingsRepository := settingsRepo.NewUserRepository(db, driver)

	// Create enrichment repositories
	enrichmentQueueRepository := enrichmentRepo.NewQueueRepository(db, driver)
	enrichmentStatusRepository := enrichmentRepo.NewStatusRepository(db, driver)
	enrichmentPipelineRepository := enrichmentRepo.NewPipelineRepository(db, driver)
	enrichmentExternalIDRepository := enrichmentRepo.NewExternalIDRepository(db, driver)
	enrichmentMetadataSourceRepository := enrichmentRepo.NewMetadataSourceRepository(db, driver)

	// Create people/credits repository
	peopleRepository := peopleRepo.NewRepository(baseRepo)

	// Create studios repository
	studiosRepository := studiosRepo.NewRepository(baseRepo)

	// Create keywords repository
	keywordsRepository := keywordsRepo.NewRepository(baseRepo)

	// Create search repository
	searchRepository := searchRepo.NewRepository(baseRepo)

	// Create plugin repositories
	pluginRepository := pluginRepo.NewRepository(db, driver)
	pluginMediaQuerier := plugins.NewDBMediaQuerier(db, driver)

	// Create transcode analytics repository
	transcodeAnalyticsRepository := transcodeAnalyticsRepo.NewRepository(db, driver)

	// Create playback preferences repository
	playbackPreferencesRepository := database.NewPlaybackPreferencesRepository(db)

	return &Repositories{
		Library:                  libraryRepository,
		Media:                    mediaRepository,
		Progress:                 progressRepository,
		PlaybackPreferences:      playbackPreferencesRepository,
		ScanJob:                  scanJobRepository,
		Checkpoint:               checkpointRepository,
		ScanState:                scanStateRepository,
		Transcode:                transcodeRepository,
		Movie:                    movieRepository,
		TV:                       tvRepository,
		Music:                    musicRepository,
		Image:                    imageRepository,
		Analytics:                analyticsRepository,
		User:                     userRepository,
		Session:                  sessionRepository,
		SystemSettings:           systemSettingsRepository,
		UserSettings:             userSettingsRepository,
		EnrichmentQueue:          enrichmentQueueRepository,
		EnrichmentStatus:         enrichmentStatusRepository,
		EnrichmentPipeline:       enrichmentPipelineRepository,
		EnrichmentExternalID:     enrichmentExternalIDRepository,
		EnrichmentMetadataSource: enrichmentMetadataSourceRepository,
		People:                   peopleRepository,
		Studios:                  studiosRepository,
		Keywords:                 keywordsRepository,
		Search:                   searchRepository,
		Plugin:                   pluginRepository,
		PluginMediaQuerier:       pluginMediaQuerier,
		TranscodeAnalytics:       transcodeAnalyticsRepository,
	}
}
