package app

import (
	"database/sql"
	"log/slog"

	"github.com/viewra/viewra/internal/api"
	"github.com/viewra/viewra/internal/api/handlers"
	"github.com/viewra/viewra/internal/app/noop"
	"github.com/viewra/viewra/internal/application/library"
	"github.com/viewra/viewra/internal/application/media"
	libraryRepo "github.com/viewra/viewra/internal/infrastructure/persistence/library"
	mediaRepo "github.com/viewra/viewra/internal/infrastructure/persistence/media"
	scanJobRepo "github.com/viewra/viewra/internal/infrastructure/persistence/scanjob"
)

// Container holds all application dependencies
type Container struct {
	// HTTP Server
	Server *api.Server
}

// NewContainer creates and wires up all application dependencies
func NewContainer(db *sql.DB, dbDriver string, config api.ServerConfig, logger *slog.Logger) *Container {
	// Initialize repositories
	libraryRepository := libraryRepo.NewRepository(db, dbDriver)
	mediaRepository := mediaRepo.NewRepository(db, dbDriver)
	scanJobRepository := scanJobRepo.NewRepository(db, dbDriver)

	// Initialize library use cases (they use repositories directly)
	createLibrary := library.NewCreateLibraryUseCase(libraryRepository)
	updateLibrary := library.NewUpdateLibraryUseCase(libraryRepository)
	deleteLibrary := library.NewDeleteLibraryUseCase(libraryRepository)
	getLibrary := library.NewGetLibraryUseCase(libraryRepository)
	listLibraries := library.NewListLibrariesUseCase(libraryRepository)

	// Initialize scan library use case
	// NOTE: Movie/TV/Music-specific repositories (Phase 3) not yet implemented.
	// Using no-op implementations to prevent nil pointer panics during scanning.
	scanLibrary := library.NewScanLibraryUseCase(
		libraryRepository,
		mediaRepository,
		noop.NewMovieRepository(), // Phase 3 - no-op for now
		noop.NewTVRepository(),    // Phase 3 - no-op for now
		noop.NewMusicRepository(), // Phase 3 - no-op for now
		scanJobRepository,
	)

	// Initialize media use cases
	getMedia := media.NewGetMediaUseCase(mediaRepository)
	listMedia := media.NewListMediaUseCase(mediaRepository)

	// Create handlers
	healthHandler := handlers.NewHealthHandler(db)

	// Create HTTP server
	server := api.NewServer(
		config,
		logger,
		healthHandler,
		createLibrary,
		updateLibrary,
		deleteLibrary,
		getLibrary,
		listLibraries,
		scanLibrary,
		getMedia,
		listMedia,
	)

	return &Container{
		Server: server,
	}
}
