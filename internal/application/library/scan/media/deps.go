package media

import (
	"context"
	"log/slog"
	"sync"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/application/library/scan/scanutil"
	"github.com/mantonx/viewra/internal/domain/enrichment"
	domainImages "github.com/mantonx/viewra/internal/domain/images"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
)

// EnrichmentEnqueuer enqueues media for enrichment after scanning.
// This is an optional dependency - if nil, enrichment is skipped.
type EnrichmentEnqueuer interface {
	EnqueueFirstStage(ctx context.Context, mediaID int64, mediaType enrichment.MediaType) error
}

// Deps bundles all dependencies needed by media processing functions.
// This allows functions to be standalone (not methods) while receiving
// all necessary dependencies for database access, image extraction, etc.
type Deps struct {
	// Repositories
	MediaRepos *scan.MediaRepositories
	ScanRepos  *scan.ScanRepositories
	ImageRepo  domainImages.Repository

	// Image extractors - interfaces for testability
	MovieExtractor   MovieImageExtractor
	EpisodeExtractor TVEpisodeImageExtractor
	ShowExtractor    TVShowImageExtractor
	SeasonExtractor  TVSeasonImageExtractor
	AlbumExtractor   MusicAlbumImageExtractor
	ArtistExtractor  MusicArtistImageExtractor
	TrackExtractor   MusicTrackImageExtractor

	// Deduplication tracking (shared across scan session)
	// These prevent redundant processing of artists and shows for enrichment
	ProcessedArtists *scanutil.AtomicDeduplicator
	ProcessedShows   *scanutil.AtomicDeduplicator

	// Infrastructure
	Coordinator *filesystem.Coordinator
	Logger      *slog.Logger

	// Enrichment - optional, if set, media is enqueued for enrichment after scanning
	EnrichmentEnqueuer EnrichmentEnqueuer
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
