package media

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/application/enrichment/pipeline"
	"github.com/mantonx/viewra/internal/application/library/scan/scanutil"
	domainCommon "github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
)

// ProcessMovie creates or updates a movie entry.
func ProcessMovie(
	ctx context.Context,
	deps *Deps,
	libraryID int64,
	result *scanner.ScanResult,
	checkpoint *scanner.ScanCheckpoint,
	existingMediaCache *sync.Map,
) (*int64, error) {
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
	if scanutil.IsAudioFile(ext) {
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
			IsExtra:         scanutil.IsExtra(result.FilePath),
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

	// NOTE: NFO metadata enrichment is now handled asynchronously by the enrichment pipeline.
	// The scanner saves minimal records; the NFO enricher populates rich metadata.

	// Ensure SortTitle is always set with normalized value
	if movie.SortTitle == "" {
		movie.SortTitle = domainCommon.NormalizeSortTitle(movie.Media.Title)
	}

	// Use shared cache-based upsert pattern with race condition handling
	movie.Media.Type = "movie"
	return ProcessMediaWithCache(ctx, deps, libraryID, result.FilePath, existingMediaCache, UpsertCallbacks{
		GetMediaID: func() int64 { return movie.Media.ID },
		SetMediaID: func(id int64) { movie.Media.ID = id },
		Update: func(ctx context.Context) error {
			if err := deps.MediaRepos.Media.Update(ctx, &movie.Media); err != nil {
				return fmt.Errorf("failed to update base media record: %w", err)
			}
			if err := deps.MediaRepos.Movie.UpdateMovie(ctx, movie); err != nil {
				return fmt.Errorf("failed to update movie metadata: %w", err)
			}
			return nil
		},
		Create: func(ctx context.Context) error {
			if err := deps.MediaRepos.Movie.CreateMovie(ctx, movie); err != nil {
				return fmt.Errorf("failed to create movie: %w", err)
			}
			return nil
		},
		PostSave: func(ctx context.Context) {
			PersistMediaTracks(ctx, deps, movie.Media.ID, result)
			// Calculate enrichment priority from release date estimate
			priority := pipeline.CalculatePriorityFromMetadata(result.Year, time.Now())
			// Enqueue for enrichment - images are now extracted via the enrichment pipeline
			enqueueForEnrichment(ctx, deps, movie.Media.ID, libraryID, enrichment.MediaTypeMovie, priority)
		},
		EventMeta: &EventMetadata{Type: "movie", Title: movie.Media.Title},
	})
}
