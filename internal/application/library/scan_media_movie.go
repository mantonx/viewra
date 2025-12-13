package library

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	domainCommon "github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/metadata/nfo"
)

// processMovie creates or updates a movie entry
func (uc *ScanLibraryUseCase) processMovie(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, existingMediaCache *sync.Map) (*int64, error) {
	// Handle nil checkpoint (legacy code path or tests)
	if checkpoint == nil {
		checkpoint = &scanner.ScanCheckpoint{
			FilePath: result.FilePath,
			FileHash: "",
			FileSize: 0,
		}
	}

	// Skip audio files in Movie libraries - they can't be movies (e.g., soundtrack files)
	// Audio files should only be processed in Music libraries
	ext := strings.ToLower(strings.TrimPrefix(result.FilePath[strings.LastIndex(result.FilePath, "."):], "."))
	if audioExtensions[ext] { // Uses package-level var from scan_utils.go
		// Return nil media ID but no error - this file is intentionally skipped
		return nil, nil
	}

	// Coordinator already parsed the filename - just use the results
	movie := &media.Movie{
		Media: media.Media{
			LibraryID:       libraryID,
			Title:           result.Title,
			FilePath:        result.FilePath,
			FileSize:        checkpoint.FileSize,
			FileHash:        checkpoint.FileHash,
			Duration:        int(result.Duration),
			IsExtra:         isExtra(result.FilePath),
			Width:           result.Width,
			Height:          result.Height,
			VideoCodec:      result.VideoCodec,
			AudioCodec:      result.AudioCodec,
			Bitrate:         result.Bitrate,
			FrameRate:       result.FrameRate,
			ContainerFormat: result.ContainerFormat,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
	}

	// Set year from scan result
	if result.Year != nil {
		movie.Year = *result.Year
	}

	// Try to enhance metadata from NFO file
	nfoPath, err := nfo.FindMovieNFO(result.FilePath)
	if err == nil && nfoPath != "" {
		nfoMetadata, err := nfo.ParseMovieNFO(nfoPath)
		if err == nil && nfoMetadata != nil {
			// Populate movie metadata from NFO
			if nfoMetadata.Title != "" {
				movie.Media.Title = nfoMetadata.Title
			}
			if nfoMetadata.Year > 0 {
				movie.Year = nfoMetadata.Year
			}
			movie.OriginalTitle = nfoMetadata.OriginalTitle
			movie.SortTitle = nfoMetadata.SortTitle
			movie.ReleaseDate = nfoMetadata.ReleaseDate
			movie.RuntimeMinutes = nfoMetadata.RuntimeMinutes
			movie.IMDbID = nfoMetadata.IMDbID
			movie.TMDbID = nfoMetadata.TMDbID
			movie.Director = nfoMetadata.Director
			movie.Cast = nfoMetadata.Cast
			movie.Genre = nfoMetadata.Genre
			movie.Plot = nfoMetadata.Plot
			movie.Tagline = nfoMetadata.Tagline
			movie.ContentRating = nfoMetadata.ContentRating
			movie.MaturityRating = nfoMetadata.MaturityRating
			movie.ContentAdvisories = nfoMetadata.ContentAdvisories
			movie.Budget = nfoMetadata.Budget
			movie.Revenue = nfoMetadata.Revenue
			movie.OriginalLanguage = nfoMetadata.OriginalLanguage
			movie.CountryOfOrigin = nfoMetadata.CountryOfOrigin
			movie.AwardsSummary = nfoMetadata.AwardsSummary
		}
	}

	// Ensure SortTitle is always set with normalized value
	if movie.SortTitle == "" {
		movie.SortTitle = domainCommon.NormalizeSortTitle(movie.Media.Title)
	}

	// Use shared cache-based upsert pattern with race condition handling
	movie.Media.Type = "movie"
	return uc.processMediaWithCache(ctx, libraryID, result.FilePath, existingMediaCache, MediaUpsertCallbacks{
		GetMediaID: func() int64 { return movie.Media.ID },
		SetMediaID: func(id int64) { movie.Media.ID = id },
		Update: func(ctx context.Context) error {
			if err := uc.mediaRepos.Media.Update(ctx, &movie.Media); err != nil {
				return fmt.Errorf("failed to update base media record: %w", err)
			}
			if err := uc.mediaRepos.Movie.UpdateMovie(ctx, movie); err != nil {
				return fmt.Errorf("failed to update movie metadata: %w", err)
			}
			return nil
		},
		Create: func(ctx context.Context) error {
			if err := uc.mediaRepos.Movie.CreateMovie(ctx, movie); err != nil {
				return fmt.Errorf("failed to create movie: %w", err)
			}
			return nil
		},
		PostSave: func(ctx context.Context) {
			uc.extractImagesForMovie(ctx, movie, result.FilePath)
			uc.persistMediaTracks(ctx, movie.Media.ID, result)
		},
	})
}
