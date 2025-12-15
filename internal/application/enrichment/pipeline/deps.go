package pipeline

import (
	"context"
	"log/slog"

	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/domain/images"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/events"
)

// Deps contains dependencies for the pipeline manager.
type Deps struct {
	// QueueRepo handles persistent job queue operations.
	QueueRepo enrichment.QueueRepository

	// StatusRepo tracks per-media enrichment status.
	StatusRepo enrichment.StatusRepository

	// PipelineRepo provides pipeline configuration.
	PipelineRepo enrichment.PipelineRepository

	// ExternalIDRepo stores external IDs discovered during enrichment.
	ExternalIDRepo enrichment.ExternalIDRepository

	// MetadataSourceRepo tracks which plugin provided metadata fields.
	MetadataSourceRepo enrichment.MetadataSourceRepository

	// MediaRepo provides media data for building EnrichRequests.
	MediaRepo MediaRepository

	// ImageRepo stores discovered artwork/images.
	ImageRepo ImageRepository

	// EventBus publishes enrichment events for observability.
	EventBus events.Publisher

	// Logger for structured logging.
	Logger *slog.Logger

	// Optional: Image processing dependencies (nil = disabled)
	// MetadataExtractor extracts metadata from image files.
	MetadataExtractor MetadataExtractor

	// Transformer generates WebP presets from source images.
	Transformer ImageTransformer

	// Downloader downloads remote images to local cache.
	Downloader ImageDownloader
}

// MediaRepository is the subset of media repository methods needed by the pipeline.
// This avoids importing the full media repository and keeps dependencies minimal.
type MediaRepository interface {
	GetByID(ctx context.Context, id int64) (*media.Media, error)
}

// MovieRepository provides movie-specific data for enrichment.
type MovieRepository interface {
	GetMovieByID(ctx context.Context, id int64) (*media.Movie, error)
	UpdateMovie(ctx context.Context, movie *media.Movie) error
}

// TVRepository provides TV-specific data for enrichment.
type TVRepository interface {
	GetTVEpisodeByID(ctx context.Context, id int64) (*media.TVEpisode, error)
	GetTVShowByID(ctx context.Context, id int64) (media.TVShow, error)
	GetTVSeasonByID(ctx context.Context, id int64) (media.TVSeason, error)
	UpdateTVEpisode(ctx context.Context, episode *media.TVEpisode) error
	UpdateTVShow(ctx context.Context, show media.TVShow) error
}

// MusicRepository provides music-specific data for enrichment.
type MusicRepository interface {
	GetMusicTrackByID(ctx context.Context, id int64) (*media.MusicTrack, error)
	UpdateMusicTrack(ctx context.Context, track *media.MusicTrack) error
}

// ImageRepository provides image storage for enrichment.
type ImageRepository interface {
	Create(ctx context.Context, image *images.Image) error
	// GetByHash retrieves all images with a specific file hash (for deduplication)
	GetByHash(ctx context.Context, hash string) ([]*images.Image, error)
	// GetByExternalURL retrieves an image by its external URL and media ID (for remote dedup)
	GetByExternalURL(ctx context.Context, externalURL string, mediaID int64) (*images.Image, error)
}

// MetadataExtractor extracts metadata from image files.
type MetadataExtractor interface {
	// ExtractMetadata extracts dimensions, hash, MIME type from an image file.
	ExtractMetadata(imagePath string) (*ImageMetadata, error)
}

// ImageMetadata contains extracted image file metadata.
type ImageMetadata struct {
	Width         *int
	Height        *int
	FileSizeBytes *int64
	MimeType      *string
	FileHash      *string
}

// ImageTransformer generates WebP presets from source images.
type ImageTransformer interface {
	// TransformAllPresets generates all preset sizes for a given image type.
	// Returns a map of preset names to cache paths.
	TransformAllPresets(sourcePath, fileHash string, imageType images.ImageType) (map[string]string, error)
}

// ImageDownloader downloads remote images to local cache.
type ImageDownloader interface {
	// Download fetches a remote image and stores it locally.
	// Returns the local file path.
	Download(ctx context.Context, url string) (string, error)
}

// TypedMediaRepos provides type-specific repositories for building EnrichRequests.
type TypedMediaRepos struct {
	Movie MovieRepository
	TV    TVRepository
	Music MusicRepository
}

// StageWorkerConfig configures a worker pool for a specific stage.
type StageWorkerConfig struct {
	// Stage name this config applies to.
	Stage string

	// Concurrency is the number of concurrent workers.
	Concurrency int

	// RateLimit is the maximum requests per second (0 = unlimited).
	RateLimit float64

	// Timeout is the maximum duration for processing a single job.
	Timeout int // seconds

	// BatchSize is how many jobs to claim at once.
	BatchSize int
}

// DefaultLocalStageConfig returns config for local (CPU-bound) stages.
// High concurrency, no rate limiting.
func DefaultLocalStageConfig(stage string) StageWorkerConfig {
	return StageWorkerConfig{
		Stage:       stage,
		Concurrency: 4, // Can be tuned based on CPU count
		RateLimit:   0, // No rate limit for local operations
		Timeout:     60,
		BatchSize:   10,
	}
}

// DefaultRemoteStageConfig returns config for remote (API) stages.
// Lower concurrency, rate limited to respect API limits.
func DefaultRemoteStageConfig(stage string) StageWorkerConfig {
	return StageWorkerConfig{
		Stage:       stage,
		Concurrency: 2,
		RateLimit:   5, // 5 requests per second
		Timeout:     30,
		BatchSize:   5,
	}
}
