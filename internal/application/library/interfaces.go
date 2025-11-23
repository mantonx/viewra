package library

import (
	"context"

	"github.com/mantonx/viewra/internal/domain/images"
)

// LibraryService defines the interface for library CRUD operations
type LibraryServiceInterface interface {
	Create(ctx context.Context, req CreateLibraryRequest) (LibraryResponse, error)
	Get(ctx context.Context, id int64) (LibraryResponse, error)
	List(ctx context.Context) (ListLibrariesResponse, error)
	Update(ctx context.Context, id int64, req UpdateLibraryRequest) (LibraryResponse, error)
	Delete(ctx context.Context, id int64) error
}

// ScanLibraryExecutor defines the interface for library scanning operations
type ScanLibraryExecutor interface {
	StartScan(ctx context.Context, libraryID int64) (StartScanResponse, error)
	GetProgress(ctx context.Context, jobID int64) (ScanProgressResponse, error)
	GetLatestScan(ctx context.Context, libraryID int64) (ScanProgressResponse, error)
	GetScanHistory(ctx context.Context, libraryID int64, limit int32) (ScanHistoryResponse, error)
}

// ImageExtractor defines a unified interface for extracting images from media files
// Replaces the 6 separate executor interfaces (Movie, Episode, Show, Season, Album, Artist)
type ImageExtractor interface {
	Extract(ctx context.Context, opts ImageExtractionOptions) error
}

// ImageExtractionOptions contains all parameters needed for image extraction
type ImageExtractionOptions struct {
	FilePath  string           // Path to the media file or directory
	MediaType images.MediaType // Type of media (movie, episode, show, etc.)
	EntityID  int              // ID of the entity in the database
	MediaID   *int             // Optional: media ID (used for episodes and tracks)
	SeasonNum int              // Optional: season number (only for TV seasons)
}
