package repositories

import (
	"database/sql"

	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
	imageRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/image"
	libraryRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/library"
	mediaRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/media"
	movieRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/movie"
	musicRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/music"
	progressRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/progress"
	scanJobRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/scanjob"
	scanStateRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/scanstate"
	transcodeRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/transcode"
	tvRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/tvshow"
)

// Repositories holds all data access layer implementations.
// Groups all persistence repositories for dependency injection.
type Repositories struct {
	Library    *libraryRepo.Repository
	Media      *mediaRepo.Repository
	Progress   *progressRepo.Repository
	ScanJob    *scanJobRepo.Repository
	Checkpoint *scanJobRepo.CheckpointRepo
	ScanState  *scanStateRepo.Repository
	Transcode  *transcodeRepo.Repository
	Movie      *movieRepo.Repository
	TV         *tvRepo.Repository
	Music      *musicRepo.Repository
	Image      *imageRepo.Repository
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

	return &Repositories{
		Library:    libraryRepository,
		Media:      mediaRepository,
		Progress:   progressRepository,
		ScanJob:    scanJobRepository,
		Checkpoint: checkpointRepository,
		ScanState:  scanStateRepository,
		Transcode:  transcodeRepository,
		Movie:      movieRepository,
		TV:         tvRepository,
		Music:      musicRepository,
		Image:      imageRepository,
	}
}
