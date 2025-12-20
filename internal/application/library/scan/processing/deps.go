package processing

import (
	"context"
	"log/slog"
	"sync"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/events"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
	"github.com/mantonx/viewra/internal/infrastructure/system"
)

// MediaProcessor defines the interface for processing media files based on library type.
// This allows the processing package to process files without creating circular imports.
type MediaProcessor interface {
	// ProcessMovie processes a movie file and returns the media ID.
	ProcessMovie(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error)

	// ProcessTVEpisode processes a TV episode file and returns the media ID.
	ProcessTVEpisode(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error)

	// ProcessMusicTrack processes a music track file and returns the media ID.
	ProcessMusicTrack(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error)
}

// Deps bundles all dependencies needed by the processing package.
// This allows functions to receive dependencies explicitly rather than accessing
// them through a use case struct, enabling easier testing and cleaner code.
type Deps struct {
	// Repositories
	ScanRepos  *scan.ScanRepositories
	MediaRepos *scan.MediaRepositories

	// Processing components
	MediaProcessor MediaProcessor
	Coordinator    *filesystem.Coordinator

	// Configuration
	Config        *scan.Config
	SystemProfile *system.Profile

	// Event publishing (optional - for SSE progress updates)
	EventBus *events.Bus

	// Logging
	Logger *slog.Logger
}

// CheckpointContext holds shared state for checkpoint processing within a scan job.
// This is passed to workers and batch processing functions.
type CheckpointContext struct {
	JobID              int64
	Lib                *library.Library
	BatchSize          int
	MaxRetries         int
	ExistingMediaCache *sync.Map
	FoundFilesMu       *sync.Mutex
	FoundFiles         map[string]bool
	CheckpointsChan    chan *scanner.ScanCheckpoint
	WorkerWg           *sync.WaitGroup
}

// NewCheckpointContext creates a new checkpoint processing context.
func NewCheckpointContext(
	jobID int64,
	lib *library.Library,
	batchSize int,
	maxRetries int,
	bufferSize int,
	existingMediaCache *sync.Map,
) *CheckpointContext {
	var foundFilesMu sync.Mutex

	return &CheckpointContext{
		JobID:              jobID,
		Lib:                lib,
		BatchSize:          batchSize,
		MaxRetries:         maxRetries,
		ExistingMediaCache: existingMediaCache,
		FoundFilesMu:       &foundFilesMu,
		FoundFiles:         make(map[string]bool),
		CheckpointsChan:    make(chan *scanner.ScanCheckpoint, bufferSize),
		WorkerWg:           &sync.WaitGroup{},
	}
}

// GetNumWorkers returns the number of processing workers based on system profile.
func GetNumWorkers(profile *system.Profile, defaultWorkers int, logger *slog.Logger) int {
	numWorkers := defaultWorkers
	if profile != nil {
		settings := profile.Calculate()
		numWorkers = settings.ProcessingWorkers
		logger.Info("using parallel checkpoint processing",
			"workers", numWorkers,
			"storage_type", profile.Storage.Type)
	}
	return numWorkers
}
