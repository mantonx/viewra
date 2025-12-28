package media

import (
	"context"
	"log/slog"
	"sync"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/application/library/scan/scanutil"
	"github.com/mantonx/viewra/internal/domain/enrichment"
	domainevents "github.com/mantonx/viewra/internal/domain/events"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
)

// EnrichmentEnqueuer enqueues media for enrichment after scanning.
// This is an optional dependency - if nil, enrichment is skipped.
type EnrichmentEnqueuer interface {
	// EnqueueFirstStage enqueues a media item for the first enrichment stage.
	// priority is calculated from estimated release date (higher = processed sooner).
	// Use pipeline.CalculatePriorityFromMetadata() to compute priority.
	EnqueueFirstStage(ctx context.Context, mediaID int64, libraryID int64, mediaType enrichment.MediaType, priority int) error
}

// Deps bundles all dependencies needed by media processing functions.
// This allows functions to be standalone (not methods) while receiving
// all necessary dependencies for database access and enrichment enqueueing.
type Deps struct {
	// Repositories
	MediaRepos *scan.MediaRepositories
	ScanRepos  *scan.ScanRepositories

	// Deduplication tracking (shared across scan session)
	// These prevent redundant processing of artists and shows for enrichment
	ProcessedArtists *scanutil.AtomicDeduplicator
	ProcessedShows   *scanutil.AtomicDeduplicator

	// Infrastructure
	Coordinator *filesystem.Coordinator
	Logger      *slog.Logger

	// Enrichment - optional, if set, media is enqueued for enrichment after scanning
	EnrichmentEnqueuer EnrichmentEnqueuer

	// Event publisher - optional, if set, media lifecycle events are published
	Publisher domainevents.Publisher
}

// ProcessMediaResult holds the result of processing a single media file.
type ProcessMediaResult struct {
	MediaID *int64
	Err     error
}

// MediaProcessor defines the interface for processing different media types.
// This allows the processing package to call back to media processing
// without creating a circular import.
type MediaProcessor interface {
	ProcessMovie(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error)
	ProcessTVEpisode(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error)
	ProcessMusicTrack(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error)
}
