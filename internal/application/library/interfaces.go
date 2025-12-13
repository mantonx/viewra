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
	ResumeScan(ctx context.Context, jobID int64) error
	GetProgress(ctx context.Context, jobID int64) (ScanProgressResponse, error)
	GetLatestScan(ctx context.Context, libraryID int64) (ScanProgressResponse, error)
	GetScanHistory(ctx context.Context, libraryID int64, limit int32) (ScanHistoryResponse, error)
}

// Image extractor interfaces for testability
// These allow mocking image extraction in unit tests without FFmpeg dependencies

// MovieImageExtractor extracts images for movies
type MovieImageExtractor interface {
	Execute(ctx context.Context, movieFilePath string, mediaType images.MediaType, entityID int, mediaID *int) error
}

// TVEpisodeImageExtractor extracts images for TV episodes
type TVEpisodeImageExtractor interface {
	Execute(ctx context.Context, episodeFilePath string, mediaType images.MediaType, entityID int, mediaID *int) error
}

// TVShowImageExtractor extracts images for TV shows
type TVShowImageExtractor interface {
	Execute(ctx context.Context, showDir string, mediaType images.MediaType, entityID int) error
}

// TVSeasonImageExtractor extracts images for TV seasons
type TVSeasonImageExtractor interface {
	Execute(ctx context.Context, showDir string, seasonNumber int, mediaType images.MediaType, entityID int) error
}

// MusicAlbumImageExtractor extracts images for music albums
type MusicAlbumImageExtractor interface {
	Execute(ctx context.Context, albumDir string, mediaType images.MediaType, entityID int) error
}

// MusicArtistImageExtractor extracts images for music artists
type MusicArtistImageExtractor interface {
	Execute(ctx context.Context, artistDir string, mediaType images.MediaType, entityID int) error
}

// MusicTrackImageExtractor extracts embedded images from music track files
type MusicTrackImageExtractor interface {
	Execute(ctx context.Context, trackFile string, mediaType images.MediaType, entityID int, mediaID *int) error
}
