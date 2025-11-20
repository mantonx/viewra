# ADR 010: Container Refactoring Strategy

**Status**: Proposed
**Date**: 2025-11-17
**Author**: ViewRA Team
**Supersedes**: N/A
**Related**: Phase 4.5 Architectural Refactoring

## Context

### Current Architecture Issues

After architectural review by specialized agents (complexity-critic and go-backend-architect), several critical scalability issues were identified:

1. **Container God Object** (`internal/app/container.go`): 420-line `NewContainer()` function that manually wires every dependency
2. **Server Parameter Explosion** (`internal/api/server.go`): `NewServer()` takes 28 parameters
3. **Scattered Configuration**: Environment variable parsing spread across multiple files
4. **Use Case Boilerplate**: 30+ individual use case instantiations with repetitive patterns
5. **Repository Initialization Duplication**: Same `(db, dbDriver)` pattern repeated 9+ times

### Architecture Goals

For a media server application at our scale (15K+ LOC, 3 media types, growing feature set), we need:

- **DRY (Don't Repeat Yourself)**: Eliminate repetitive initialization patterns
- **Organized**: Clear separation of concerns with focused, single-responsibility components
- **Scalable**: Easy to add new features without modifying multiple files
- **Testable**: Components can be tested in isolation
- **Maintainable**: Code is easy to understand and modify
- **No Over-Engineering**: Avoid enterprise patterns that don't provide value at our scale

## Decision

We will refactor the dependency injection architecture using **manual DI with focused builder functions**. This approach provides the benefits of DI frameworks without external dependencies.

### Architecture Pattern: Builder Functions + Struct Grouping

Instead of one massive constructor, we'll use:

1. **Grouped Structs**: Organize related dependencies into focused structs
2. **Builder Functions**: Each builder creates one group of dependencies
3. **Clear Dependency Flow**: Builders take only what they need, return grouped structs
4. **Single Responsibility**: Each builder has one job

## Architecture Design

### 1. Configuration Layer

**File**: `internal/config/config.go`

Centralize ALL configuration with validation:

```go
package config

import (
    "fmt"
    "os"
    "strconv"
    "time"
)

// Config holds all application configuration
type Config struct {
    Environment string
    Server      ServerConfig
    Database    DatabaseConfig
    Transcode   TranscodeConfig
    Images      ImagesConfig
    Scheduler   SchedulerConfig
    Browser     BrowserConfig
}

type ServerConfig struct {
    Port            int
    Host            string
    ReadTimeout     time.Duration
    WriteTimeout    time.Duration
    ShutdownTimeout time.Duration
}

type DatabaseConfig struct {
    Driver string
    DSN    string
}

type TranscodeConfig struct {
    WorkerCount      int
    OutputDir        string
    PollInterval     time.Duration
    IdleTimeout      time.Duration
    CleanupEnabled   bool
    DiskThreshold    int
    DiskWarning      int
    MinFreeSpaceGB   int64
    MaxStorageGB     int64
    MaxAgeHours      int
    MaxIdleHours     int
    CleanupBatchSize int
    KeepFailedHours  int
}

type ImagesConfig struct {
    CacheDir string
}

type SchedulerConfig struct {
    Enabled bool
}

type BrowserConfig struct {
    AllowedBasePaths []string
    DefaultBasePath  string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
    cfg := &Config{
        Environment: getEnvOrDefault("ENVIRONMENT", "development"),
        Server:      loadServerConfig(),
        Database:    loadDatabaseConfig(),
        Transcode:   loadTranscodeConfig(),
        Images:      loadImagesConfig(),
        Scheduler:   loadSchedulerConfig(),
        Browser:     loadBrowserConfig(),
    }

    if err := cfg.Validate(); err != nil {
        return nil, fmt.Errorf("config validation failed: %w", err)
    }

    return cfg, nil
}

// Validate ensures configuration is valid
func (c *Config) Validate() error {
    if c.Server.Port < 1 || c.Server.Port > 65535 {
        return fmt.Errorf("invalid port: %d", c.Server.Port)
    }
    if c.Transcode.WorkerCount < 1 {
        return fmt.Errorf("transcode worker count must be >= 1")
    }
    if c.Database.Driver != "sqlite" && c.Database.Driver != "postgres" {
        return fmt.Errorf("unsupported database driver: %s", c.Database.Driver)
    }
    return nil
}

// Helper functions for consistent parsing
func getEnvOrDefault(key, defaultVal string) string {
    if val := os.Getenv(key); val != "" {
        return val
    }
    return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
    if val := os.Getenv(key); val != "" {
        if i, err := strconv.Atoi(val); err == nil {
            return i
        }
    }
    return defaultVal
}

func getEnvAsInt64(key string, defaultVal int64) int64 {
    if val := os.Getenv(key); val != "" {
        if i, err := strconv.ParseInt(val, 10, 64); err == nil {
            return i
        }
    }
    return defaultVal
}

func getEnvAsBool(key string, defaultVal bool) bool {
    if val := os.Getenv(key); val != "" {
        return val == "true" || val == "1"
    }
    return defaultVal
}

func getEnvAsDuration(key string, defaultVal time.Duration) time.Duration {
    if val := os.Getenv(key); val != "" {
        if d, err := time.ParseDuration(val); err == nil {
            return d
        }
    }
    return defaultVal
}

func loadServerConfig() ServerConfig {
    return ServerConfig{
        Port:            getEnvAsInt("PORT", 8080),
        Host:            getEnvOrDefault("HOST", "0.0.0.0"),
        ReadTimeout:     getEnvAsDuration("SERVER_READ_TIMEOUT", 10*time.Second),
        WriteTimeout:    getEnvAsDuration("SERVER_WRITE_TIMEOUT", 10*time.Second),
        ShutdownTimeout: getEnvAsDuration("SERVER_SHUTDOWN_TIMEOUT", 10*time.Second),
    }
}

func loadDatabaseConfig() DatabaseConfig {
    return DatabaseConfig{
        Driver: getEnvOrDefault("DB_DRIVER", "sqlite"),
        DSN:    getEnvOrDefault("DB_DSN", "data/viewra.db"),
    }
}

func loadTranscodeConfig() TranscodeConfig {
    return TranscodeConfig{
        WorkerCount:      getEnvAsInt("TRANSCODE_WORKER_COUNT", 2),
        OutputDir:        getEnvOrDefault("TRANSCODE_OUTPUT_DIR", "./data/cache/transcodes"),
        PollInterval:     getEnvAsDuration("TRANSCODE_POLL_INTERVAL", 10*time.Second),
        IdleTimeout:      getEnvAsDuration("TRANSCODE_IDLE_TIMEOUT", 5*time.Minute),
        CleanupEnabled:   getEnvAsBool("TRANSCODE_CLEANUP_ENABLED", true),
        DiskThreshold:    getEnvAsInt("TRANSCODE_CLEANUP_DISK_THRESHOLD", 85),
        DiskWarning:      getEnvAsInt("TRANSCODE_CLEANUP_DISK_WARNING", 80),
        MinFreeSpaceGB:   getEnvAsInt64("TRANSCODE_MIN_FREE_SPACE_GB", 10),
        MaxStorageGB:     getEnvAsInt64("TRANSCODE_MAX_STORAGE_GB", 0),
        MaxAgeHours:      getEnvAsInt("TRANSCODE_MAX_AGE_DAYS", 30) * 24,
        MaxIdleHours:     getEnvAsInt("TRANSCODE_MAX_IDLE_DAYS", 7) * 24,
        CleanupBatchSize: getEnvAsInt("TRANSCODE_CLEANUP_BATCH_SIZE", 10),
        KeepFailedHours:  getEnvAsInt("TRANSCODE_KEEP_FAILED_HOURS", 24),
    }
}

func loadImagesConfig() ImagesConfig {
    return ImagesConfig{
        CacheDir: getEnvOrDefault("IMAGE_CACHE_DIR", "./data/cache/images"),
    }
}

func loadSchedulerConfig() SchedulerConfig {
    return SchedulerConfig{
        Enabled: getEnvAsBool("SCHEDULER_ENABLED", true),
    }
}

func loadBrowserConfig() BrowserConfig {
    // Parse comma-separated list
    basePaths := getEnvOrDefault("BROWSER_ALLOWED_BASE_PATHS", "/home,/mnt,/media")
    var paths []string
    if basePaths != "" {
        paths = strings.Split(basePaths, ",")
    }

    return BrowserConfig{
        AllowedBasePaths: paths,
        DefaultBasePath:  getEnvOrDefault("BROWSER_DEFAULT_BASE_PATH", "/home"),
    }
}
```

### 2. Repository Layer

**File**: `internal/app/repositories.go`

Group all repositories into a single struct:

```go
package app

import (
    "database/sql"

    "github.com/viewra/viewra/internal/infrastructure/persistence/common"
    libraryRepo "github.com/viewra/viewra/internal/infrastructure/persistence/library"
    mediaRepo "github.com/viewra/viewra/internal/infrastructure/persistence/media"
    movieRepo "github.com/viewra/viewra/internal/infrastructure/persistence/movies"
    musicRepo "github.com/viewra/viewra/internal/infrastructure/persistence/music"
    tvRepo "github.com/viewra/viewra/internal/infrastructure/persistence/tv"
    imageRepo "github.com/viewra/viewra/internal/infrastructure/persistence/images"
    progressRepo "github.com/viewra/viewra/internal/infrastructure/persistence/progress"
    scanJobRepo "github.com/viewra/viewra/internal/infrastructure/persistence/scanjob"
    transcodeRepo "github.com/viewra/viewra/internal/infrastructure/persistence/transcode"
)

// Repositories holds all data access layer dependencies
type Repositories struct {
    Library   *libraryRepo.Repository
    Media     *mediaRepo.Repository
    Movie     *movieRepo.Repository
    TV        *tvRepo.Repository
    Music     *musicRepo.Repository
    Image     *imageRepo.Repository
    Progress  *progressRepo.Repository
    ScanJob   *scanJobRepo.Repository
    Transcode *transcodeRepo.Repository
}

// BuildRepositories creates all repository instances
func BuildRepositories(db *sql.DB, dbDriver string) *Repositories {
    baseRepo := common.NewBaseRepository(db, dbDriver)
    media := mediaRepo.NewRepository(db, dbDriver)

    return &Repositories{
        Library:   libraryRepo.NewRepository(db, dbDriver),
        Media:     media,
        Movie:     movieRepo.NewRepository(baseRepo, media),
        TV:        tvRepo.NewRepository(baseRepo, media),
        Music:     musicRepo.NewRepository(baseRepo, media),
        Image:     imageRepo.NewRepository(baseRepo),
        Progress:  progressRepo.NewRepository(db, dbDriver),
        ScanJob:   scanJobRepo.NewRepository(db, dbDriver),
        Transcode: transcodeRepo.NewRepository(db, dbDriver),
    }
}
```

### 3. Handler Layer

**File**: `internal/app/handlers.go`

Group handlers with a builder function:

```go
package app

import (
    "database/sql"
    "log/slog"

    "github.com/viewra/viewra/internal/api/handlers"
    "github.com/viewra/viewra/internal/application/library"
    "github.com/viewra/viewra/internal/application/media"
    "github.com/viewra/viewra/internal/application/movies"
    "github.com/viewra/viewra/internal/application/music"
    "github.com/viewra/viewra/internal/application/tv"
    "github.com/viewra/viewra/internal/infrastructure/scheduler"
)

// Handlers holds all HTTP request handlers
type Handlers struct {
    Health    *handlers.HealthHandler
    Browser   *handlers.BrowserHandler
    ScanJob   *handlers.ScanJobHandler
    Progress  *handlers.ProgressHandler
    Transcode *handlers.TranscodeHandler
    Images    *handlers.ImagesHandler
    Scheduler *handlers.SchedulerHandler
    Library   *handlers.LibraryHandler
    Media     *handlers.MediaHandler
    Movies    *handlers.MoviesHandler
    TV        *handlers.TVHandler
    Music     *handlers.MusicHandler
}

// HandlerDependencies groups dependencies needed to build handlers
type HandlerDependencies struct {
    DB            *sql.DB
    Repos         *Repositories
    LibraryUseCases LibraryUseCases
    MediaUseCases   MediaUseCases
    MoviesUseCases  MoviesUseCases
    TVUseCases      TVUseCases
    MusicUseCases   MusicUseCases
    Services      *Services
    Scheduler     *scheduler.Scheduler
}

// BuildHandlers creates all HTTP handlers
func BuildHandlers(deps *HandlerDependencies) *Handlers {
    return &Handlers{
        Health:  handlers.NewHealthHandler(deps.DB),
        Browser: handlers.NewBrowserHandler(deps.Services.PathBrowser),
        ScanJob: handlers.NewScanJobHandler(
            deps.Repos.ScanJob,
            deps.Services.StreamingService,
        ),
        Progress: handlers.NewProgressHandler(
            deps.Repos.Progress,
            deps.Repos.Media,
        ),
        Transcode: handlers.NewTranscodeHandler(
            deps.Services.TranscodeCreateJob,
            deps.Services.TranscodeGetStatus,
            deps.Services.TranscodeServeManifest,
            deps.Services.TranscodeQueue,
            deps.Services.TranscodeCleanup,
            deps.Services.TranscodeOutputDir,
        ),
        Images: handlers.NewImagesHandler(
            deps.Services.GetImage,
            deps.Services.GetMediaImages,
            deps.Services.GetEntityImages,
            deps.Services.ImageCacheDir,
        ),
        Scheduler: handlers.NewSchedulerHandler(deps.Scheduler),
        Library: handlers.NewLibraryHandler(
            deps.LibraryUseCases.Create,
            deps.LibraryUseCases.Update,
            deps.LibraryUseCases.Delete,
            deps.LibraryUseCases.Get,
            deps.LibraryUseCases.List,
            deps.LibraryUseCases.Scan,
        ),
        Media: handlers.NewMediaHandler(
            deps.MediaUseCases.Get,
            deps.MediaUseCases.List,
        ),
        Movies: handlers.NewMoviesHandler(
            deps.MoviesUseCases.List,
            deps.MoviesUseCases.Get,
            deps.MoviesUseCases.Search,
        ),
        TV: handlers.NewTVHandler(
            deps.TVUseCases.ListShows,
            deps.TVUseCases.GetShow,
            deps.TVUseCases.ListEpisodes,
            deps.TVUseCases.GetEpisode,
            deps.TVUseCases.SearchEpisodes,
        ),
        Music: handlers.NewMusicHandler(
            deps.MusicUseCases.ListArtists,
            deps.MusicUseCases.ListAlbums,
            deps.MusicUseCases.ListTracks,
            deps.MusicUseCases.GetTrack,
            deps.MusicUseCases.SearchTracks,
        ),
    }
}
```

### 4. Use Case Layer (Grouped)

**File**: `internal/app/usecases.go`

```go
package app

import (
    "github.com/viewra/viewra/internal/application/library"
    "github.com/viewra/viewra/internal/application/media"
    "github.com/viewra/viewra/internal/application/movies"
    "github.com/viewra/viewra/internal/application/music"
    "github.com/viewra/viewra/internal/application/tv"
    "github.com/viewra/viewra/internal/application/images"
)

// LibraryUseCases groups library-related use cases
type LibraryUseCases struct {
    Create library.CreateLibraryExecutor
    Update library.UpdateLibraryExecutor
    Delete library.DeleteLibraryExecutor
    Get    library.GetLibraryExecutor
    List   library.ListLibrariesExecutor
    Scan   library.ScanLibraryExecutor
}

// MediaUseCases groups media-related use cases
type MediaUseCases struct {
    Get  media.GetMediaExecutor
    List media.ListMediaExecutor
}

// MoviesUseCases groups movie-related use cases
type MoviesUseCases struct {
    List   movies.ListMoviesExecutor
    Get    movies.GetMovieExecutor
    Search movies.SearchMoviesExecutor
}

// TVUseCases groups TV-related use cases
type TVUseCases struct {
    ListShows      tv.ListTVShowsExecutor
    GetShow        tv.GetTVShowExecutor
    ListEpisodes   tv.ListTVEpisodesExecutor
    GetEpisode     tv.GetTVEpisodeExecutor
    SearchEpisodes tv.SearchTVEpisodesExecutor
}

// MusicUseCases groups music-related use cases
type MusicUseCases struct {
    ListArtists music.ListArtistsExecutor
    ListAlbums  music.ListAlbumsByArtistIDExecutor
    ListTracks  music.ListTracksByAlbumIDExecutor
    GetTrack    music.GetTrackExecutor
    SearchTracks music.SearchTracksExecutor
}

// BuildUseCases creates all application use cases
func BuildUseCases(repos *Repositories, services *Services) *UseCases {
    return &UseCases{
        Library: buildLibraryUseCases(repos, services),
        Media:   buildMediaUseCases(repos),
        Movies:  buildMoviesUseCases(repos),
        TV:      buildTVUseCases(repos),
        Music:   buildMusicUseCases(repos),
    }
}

type UseCases struct {
    Library LibraryUseCases
    Media   MediaUseCases
    Movies  MoviesUseCases
    TV      TVUseCases
    Music   MusicUseCases
}

func buildLibraryUseCases(repos *Repositories, services *Services) LibraryUseCases {
    return LibraryUseCases{
        Create: library.NewCreateLibraryUseCase(repos.Library),
        Update: library.NewUpdateLibraryUseCase(repos.Library),
        Delete: library.NewDeleteLibraryUseCase(
            repos.Library,
            repos.Image,
            services.ImageCleanup,
        ),
        Get:  library.NewGetLibraryUseCase(repos.Library),
        List: library.NewListLibrariesUseCase(repos.Library),
        Scan: library.NewScanLibraryUseCase(
            repos.Library,
            repos.Media,
            repos.Movie,
            repos.TV,
            repos.Music,
            repos.ScanJob,
            services.ExtractMovieImages,
            services.ExtractEpisodeImages,
            services.ExtractShowImages,
            services.ExtractSeasonImages,
            services.ExtractMusicImages,
            repos.Image,
            services.ImageCleanup,
        ),
    }
}

func buildMediaUseCases(repos *Repositories) MediaUseCases {
    return MediaUseCases{
        Get:  media.NewGetMediaUseCase(repos.Media),
        List: media.NewListMediaUseCase(repos.Media),
    }
}

func buildMoviesUseCases(repos *Repositories) MoviesUseCases {
    return MoviesUseCases{
        List:   movies.NewListMoviesUseCase(repos.Movie),
        Get:    movies.NewGetMovieUseCase(repos.Movie),
        Search: movies.NewSearchMoviesUseCase(repos.Movie),
    }
}

func buildTVUseCases(repos *Repositories) TVUseCases {
    return TVUseCases{
        ListShows:      tv.NewListTVShowsUseCase(repos.TV),
        GetShow:        tv.NewGetTVShowUseCase(repos.TV),
        ListEpisodes:   tv.NewListTVEpisodesUseCase(repos.TV),
        GetEpisode:     tv.NewGetTVEpisodeUseCase(repos.TV),
        SearchEpisodes: tv.NewSearchTVEpisodesUseCase(repos.TV),
    }
}

func buildMusicUseCases(repos *Repositories) MusicUseCases {
    return MusicUseCases{
        ListArtists:  music.NewListArtistsUseCase(repos.Music),
        ListAlbums:   music.NewListAlbumsByArtistIDUseCase(repos.Music),
        ListTracks:   music.NewListTracksByAlbumIDUseCase(repos.Music),
        GetTrack:     music.NewGetTrackUseCase(repos.Music),
        SearchTracks: music.NewSearchTracksUseCase(repos.Music),
    }
}
```

### 5. Services Layer

**File**: `internal/app/services.go`

Group infrastructure services:

```go
package app

import (
    "database/sql"
    "log/slog"

    "github.com/viewra/viewra/internal/application/images"
    "github.com/viewra/viewra/internal/application/transcode"
    "github.com/viewra/viewra/internal/infrastructure/pathbrowser"
    "github.com/viewra/viewra/internal/infrastructure/streaming"
    "github.com/viewra/viewra/internal/infrastructure/transcoding"
    infraimages "github.com/viewra/viewra/internal/infrastructure/images"
    "github.com/viewra/viewra/internal/config"
)

// Services holds infrastructure-level services
type Services struct {
    // Image services
    ImageCacheService    *infraimages.CacheService
    ImageTransformer     *infraimages.Transformer
    ImageExtractor       *infraimages.Extractor
    GetImage             *images.GetImageUseCase
    GetMediaImages       *images.GetMediaImagesUseCase
    GetEntityImages      *images.GetEntityImagesUseCase
    ExtractMovieImages   *images.ExtractMovieImagesUseCase
    ExtractEpisodeImages *images.ExtractEpisodeImagesUseCase
    ExtractShowImages    *images.ExtractShowImagesUseCase
    ExtractSeasonImages  *images.ExtractSeasonImagesUseCase
    ExtractMusicImages   *images.ExtractMusicAlbumImagesUseCase
    ImageCleanup         *images.CleanupUseCase
    ImageCacheDir        string

    // Transcode services
    TranscodeService       *transcoding.Service
    TranscodeQueue         *transcoding.Queue
    TranscodeCreateJob     *transcode.CreateJobUseCase
    TranscodeGetStatus     *transcode.GetJobStatusUseCase
    TranscodeServeManifest *transcode.ServeManifestUseCase
    TranscodeCleanup       *transcode.CleanupService
    TranscodeOutputDir     string

    // Other services
    PathBrowser      *pathbrowser.Service
    StreamingService *streaming.Service
}

// BuildServices creates all infrastructure services
func BuildServices(cfg *config.Config, repos *Repositories, logger *slog.Logger) (*Services, error) {
    // Image services
    imageCacheService := infraimages.NewCacheService(cfg.Images.CacheDir)
    imageTransformer := infraimages.NewTransformer(imageCacheService)
    imageExtractor := infraimages.NewExtractor(imageCacheService, imageTransformer)

    getImage := images.NewGetImageUseCase(repos.Image)
    getMediaImages := images.NewGetMediaImagesUseCase(repos.Image)
    getEntityImages := images.NewGetEntityImagesUseCase(repos.Image)
    imageCleanup := images.NewCleanupUseCase(repos.Image, cfg.Images.CacheDir, logger)

    extractMovieImages := images.NewExtractMovieImagesUseCase(repos.Image, imageExtractor, logger)
    extractEpisodeImages := images.NewExtractEpisodeImagesUseCase(repos.Image, imageExtractor, logger)
    extractShowImages := images.NewExtractShowImagesUseCase(repos.Image, imageExtractor, logger)
    extractSeasonImages := images.NewExtractSeasonImagesUseCase(repos.Image, imageExtractor, logger)
    extractMusicImages := images.NewExtractMusicAlbumImagesUseCase(repos.Image, imageExtractor, logger)

    // Transcode services
    transcodeService, err := transcoding.NewService(repos.Transcode, logger)
    if err != nil {
        logger.Warn("Failed to initialize transcode service", "error", err)
        transcodeService = nil
    }

    var transcodeQueue *transcoding.Queue
    var transcodeCreateJob *transcode.CreateJobUseCase
    var transcodeGetStatus *transcode.GetJobStatusUseCase
    var transcodeServeManifest *transcode.ServeManifestUseCase
    var transcodeCleanup *transcode.CleanupService

    if transcodeService != nil {
        transcodeQueue = transcoding.NewQueue(
            repos.Transcode,
            transcodeService,
            cfg.Transcode.WorkerCount,
            cfg.Transcode.PollInterval,
            cfg.Transcode.IdleTimeout,
            logger,
        )
        transcodeCreateJob = transcode.NewCreateJobUseCase(repos.Transcode, transcodeQueue)
        transcodeGetStatus = transcode.NewGetJobStatusUseCase(repos.Transcode)
        transcodeServeManifest = transcode.NewServeManifestUseCase(repos.Transcode, cfg.Transcode.OutputDir)
        transcodeCleanup = transcode.NewCleanupService(repos.Transcode, cfg.Transcode.OutputDir)
    }

    // Other services
    pathBrowser := pathbrowser.NewService(
        cfg.Browser.AllowedBasePaths,
        cfg.Browser.DefaultBasePath,
    )

    streamingService := streaming.NewService(repos.Media, logger)

    return &Services{
        ImageCacheService:    imageCacheService,
        ImageTransformer:     imageTransformer,
        ImageExtractor:       imageExtractor,
        GetImage:             getImage,
        GetMediaImages:       getMediaImages,
        GetEntityImages:      getEntityImages,
        ExtractMovieImages:   extractMovieImages,
        ExtractEpisodeImages: extractEpisodeImages,
        ExtractShowImages:    extractShowImages,
        ExtractSeasonImages:  extractSeasonImages,
        ExtractMusicImages:   extractMusicImages,
        ImageCleanup:         imageCleanup,
        ImageCacheDir:        cfg.Images.CacheDir,

        TranscodeService:       transcodeService,
        TranscodeQueue:         transcodeQueue,
        TranscodeCreateJob:     transcodeCreateJob,
        TranscodeGetStatus:     transcodeGetStatus,
        TranscodeServeManifest: transcodeServeManifest,
        TranscodeCleanup:       transcodeCleanup,
        TranscodeOutputDir:     cfg.Transcode.OutputDir,

        PathBrowser:      pathBrowser,
        StreamingService: streamingService,
    }, nil
}
```

### 6. Refactored Container (MUCH SMALLER!)

**File**: `internal/app/container.go`

```go
package app

import (
    "database/sql"
    "log/slog"

    "github.com/viewra/viewra/internal/api"
    "github.com/viewra/viewra/internal/config"
    "github.com/viewra/viewra/internal/infrastructure/scheduler"
)

// Container holds the application's top-level dependencies
type Container struct {
    Server    *api.Server
    Scheduler *scheduler.Scheduler
}

// NewContainer creates and wires up all application dependencies
// This function is now much smaller (~50 lines instead of 420!)
func NewContainer(
    db *sql.DB,
    dbDriver string,
    cfg *config.Config,
    logger *slog.Logger,
) (*Container, error) {
    // Build layers bottom-up
    repos := BuildRepositories(db, dbDriver)

    services, err := BuildServices(cfg, repos, logger)
    if err != nil {
        return nil, err
    }

    useCases := BuildUseCases(repos, services)

    taskScheduler, err := buildScheduler(db, logger, services, cfg)
    if err != nil {
        logger.Error("Failed to create task scheduler", "error", err)
    }

    handlers := BuildHandlers(&HandlerDependencies{
        DB:              db,
        Repos:           repos,
        LibraryUseCases: useCases.Library,
        MediaUseCases:   useCases.Media,
        MoviesUseCases:  useCases.Movies,
        TVUseCases:      useCases.TV,
        MusicUseCases:   useCases.Music,
        Services:        services,
        Scheduler:       taskScheduler,
    })

    server := api.NewServer(cfg.Server, logger, handlers)

    return &Container{
        Server:    server,
        Scheduler: taskScheduler,
    }, nil
}

func buildScheduler(
    db *sql.DB,
    logger *slog.Logger,
    services *Services,
    cfg *config.Config,
) (*scheduler.Scheduler, error) {
    execLogger := scheduler.NewDBExecutionLogger(db, logger)
    taskScheduler, err := scheduler.New(
        scheduler.DefaultConfig(),
        logger,
        execLogger,
    )
    if err != nil {
        return nil, err
    }

    // Register tasks
    registerScheduledTasks(taskScheduler, services, cfg, logger)

    return taskScheduler, nil
}

func registerScheduledTasks(
    scheduler *scheduler.Scheduler,
    services *Services,
    cfg *config.Config,
    logger *slog.Logger,
) {
    // Image cleanup task
    scheduler.RegisterTask(scheduler.Task{
        ID:          "image-cache-cleanup",
        Name:        "Image Cache Cleanup",
        Description: "Remove orphaned image cache files",
        Schedule:    "0 3 * * *",
        Enabled:     true,
        Handler: func(ctx context.Context) error {
            _, err := services.ImageCleanup.CleanOrphanedImages(ctx)
            return err
        },
    })

    // Transcode cleanup tasks
    if services.TranscodeCleanup != nil {
        // Policy cleanup
        scheduler.RegisterTask(scheduler.Task{
            ID:          "transcode-cleanup-policy",
            Name:        "Transcode Policy Cleanup",
            Description: "Clean failed/old/idle/orphaned transcodes",
            Schedule:    "0 */6 * * *",
            Enabled:     cfg.Transcode.CleanupEnabled,
            Handler: func(ctx context.Context) error {
                return transcode.PerformPolicyCleanup(ctx, services.TranscodeCleanup, &transcode.CleanupSchedulerConfig{
                    Enabled:              cfg.Transcode.CleanupEnabled,
                    DiskThresholdPercent: cfg.Transcode.DiskThreshold,
                    DiskWarningPercent:   cfg.Transcode.DiskWarning,
                    MinFreeSpaceGB:       cfg.Transcode.MinFreeSpaceGB,
                    MaxAgeHours:          cfg.Transcode.MaxAgeHours,
                    MaxIdleHours:         cfg.Transcode.MaxIdleHours,
                    MaxStorageGB:         cfg.Transcode.MaxStorageGB,
                    CleanupBatchSize:     cfg.Transcode.CleanupBatchSize,
                    KeepFailedHours:      cfg.Transcode.KeepFailedHours,
                })
            },
        })

        // Disk monitoring
        scheduler.RegisterTask(scheduler.Task{
            ID:          "transcode-cleanup-disk-check",
            Name:        "Transcode Disk Monitor",
            Description: "Monitor disk usage and perform LRU cleanup",
            Schedule:    "*/30 * * * *",
            Enabled:     cfg.Transcode.CleanupEnabled,
            Handler: func(ctx context.Context) error {
                return transcode.PerformDiskMonitoring(
                    ctx,
                    services.TranscodeCleanup,
                    repos.Transcode,
                    &transcode.CleanupSchedulerConfig{ /* same config */ },
                    cfg.Transcode.OutputDir,
                )
            },
        })
    }
}
```

### 7. Refactored Server

**File**: `internal/api/server.go`

```go
func NewServer(
    config ServerConfig,
    logger *slog.Logger,
    handlers *app.Handlers,
) *Server {
    router := gin.New()
    router.Use(gin.Recovery())
    router.Use(middleware.RequestID())
    router.Use(middleware.CORS())
    router.Use(middleware.Logger(logger))

    server := &Server{
        router:   router,
        handlers: handlers,
    }

    server.setupRoutes()

    server.server = &http.Server{
        Addr:         fmt.Sprintf(":%d", config.Port),
        Handler:      router,
        ReadTimeout:  config.ReadTimeout,
        WriteTimeout: config.WriteTimeout,
    }

    return server
}
```

## Benefits

### DRY Principles

1. **Configuration**: Single source of truth in `config.go`
2. **Repository Init**: No repeated `(db, dbDriver)` patterns
3. **Handler Creation**: Grouped into focused builders
4. **Use Cases**: Clear organization by domain

### Organization

1. **Clear File Structure**: Each builder in its own file
2. **Single Responsibility**: Each builder does one thing
3. **Focused Concerns**: Repositories, services, handlers, use cases separated
4. **Easy Navigation**: Know exactly where to find initialization logic

### Scalability

1. **Adding New Repository**: Add one line to `Repositories` struct
2. **Adding New Handler**: Add to `Handlers` struct and builder
3. **Adding New Use Case**: Add to domain-specific use case group
4. **Adding New Config**: Extend appropriate config struct

### Example: Adding a New Feature

**Before** (current architecture):
- Modify 420-line `NewContainer()` function
- Add parameter to 28-parameter `NewServer()` function
- Update 3+ function signatures
- Risk breaking existing code

**After** (refactored architecture):
```go
// 1. Add to repository struct (1 line)
type Repositories struct {
    // ... existing
    Podcast *podcastRepo.Repository  // NEW
}

// 2. Initialize in builder (1 line)
func BuildRepositories(db *sql.DB, dbDriver string) *Repositories {
    return &Repositories{
        // ... existing
        Podcast: podcastRepo.NewRepository(db, dbDriver),  // NEW
    }
}

// 3. Add use cases group
type PodcastUseCases struct {
    List podcast.ListPodcastsExecutor
    Get  podcast.GetPodcastExecutor
}

// 4. Add handler
type Handlers struct {
    // ... existing
    Podcast *handlers.PodcastHandler  // NEW
}
```

## Migration Strategy

### Phase 1: Configuration (2 hours)
1. Create `internal/config/config.go`
2. Move all env var parsing to config package
3. Update bootstrap to use new config

### Phase 2: Repositories (1 hour)
1. Create `internal/app/repositories.go`
2. Create `BuildRepositories()` function
3. Update container to use it

### Phase 3: Services (2 hours)
1. Create `internal/app/services.go`
2. Create `BuildServices()` function
3. Update container

### Phase 4: Use Cases (2 hours)
1. Create `internal/app/usecases.go`
2. Group use cases by domain
3. Update container

### Phase 5: Handlers (2 hours)
1. Create `internal/app/handlers.go`
2. Create `BuildHandlers()` function
3. Refactor `NewServer()` to take `*Handlers`

### Phase 6: Container Cleanup (1 hour)
1. Simplify `NewContainer()` to ~50 lines
2. Delete old helper functions
3. Update tests

### Phase 7: Middleware Enhancements (2 hours)
1. Add `RequestID` middleware
2. Add rate limiting (optional)
3. Add timeout middleware (optional)

## Consequences

### Positive

✅ **420-line god function → ~50 lines**: Massive maintainability improvement
✅ **28 parameters → 1 struct**: Clean, extensible API
✅ **Centralized config**: Single source of truth, easy validation
✅ **Clear dependencies**: Explicit dependency graph
✅ **Easy testing**: Mock one struct instead of 28 parameters
✅ **Scalable**: Adding features requires minimal changes
✅ **No external dependencies**: Pure Go, no frameworks

### Negative

⚠️ **Migration effort**: 10-12 hours total work
⚠️ **More files**: 5 new files (repositories, services, usecases, handlers, config)
⚠️ **Learning curve**: Team needs to learn new structure

### Neutral

🔹 **More boilerplate upfront**: But pays dividends as project grows
🔹 **Indirection**: One more level of abstraction

## Success Criteria

1. **Container < 100 lines**: `NewContainer()` is concise and readable
2. **Server < 5 parameters**: `NewServer()` takes structured dependencies
3. **Config centralized**: All env vars in one place with validation
4. **Add feature in < 10 minutes**: New features require minimal file changes
5. **Tests simplified**: Mock structs instead of many parameters

## References

- Architectural Review by complexity-critic agent (2025-11-17)
- Architectural Review by go-backend-architect agent (2025-11-17)
- Current implementation: `internal/app/container.go`
- Current implementation: `internal/api/server.go`

## Notes

This refactoring aligns with Go best practices:
- Explicit over implicit
- Simple over complex
- Composition over inheritance
- Clear dependency injection without magic

The result is a codebase that scales gracefully from 15K to 100K+ lines of code.
