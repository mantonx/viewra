package library

import (
	"context"

	"github.com/mantonx/viewra/internal/application/library/scan"
	scanmedia "github.com/mantonx/viewra/internal/application/library/scan/media"
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
	StartScan(ctx context.Context, libraryID int64) (scan.StartScanResponse, error)
	ResumeScan(ctx context.Context, jobID int64) error
	GetProgress(ctx context.Context, jobID int64) (scan.ScanProgressResponse, error)
	GetLatestScan(ctx context.Context, libraryID int64) (scan.ScanProgressResponse, error)
	GetScanHistory(ctx context.Context, libraryID int64, limit int32) (scan.ScanHistoryResponse, error)
}

// Image extractor interfaces - re-exported from scan/media for backwards compatibility.
// These allow mocking image extraction in unit tests without FFmpeg dependencies.
type (
	// MovieImageExtractor extracts images for movies
	MovieImageExtractor = scanmedia.MovieImageExtractor

	// TVEpisodeImageExtractor extracts images for TV episodes
	TVEpisodeImageExtractor = scanmedia.TVEpisodeImageExtractor

	// TVShowImageExtractor extracts images for TV shows
	TVShowImageExtractor = scanmedia.TVShowImageExtractor

	// TVSeasonImageExtractor extracts images for TV seasons
	TVSeasonImageExtractor = scanmedia.TVSeasonImageExtractor

	// MusicAlbumImageExtractor extracts images for music albums
	MusicAlbumImageExtractor = scanmedia.MusicAlbumImageExtractor

	// MusicArtistImageExtractor extracts images for music artists
	MusicArtistImageExtractor = scanmedia.MusicArtistImageExtractor

	// MusicTrackImageExtractor extracts embedded images from music track files
	MusicTrackImageExtractor = scanmedia.MusicTrackImageExtractor
)
