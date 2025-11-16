package images

import (
	"context"
	"log/slog"

	"github.com/viewra/viewra/internal/domain/images"
	infraImages "github.com/viewra/viewra/internal/infrastructure/images"
)

// ProcessAndSaveImages is a shared helper that processes extracted images and saves them to the database
// This eliminates duplication across ExtractMovieImages, ExtractTVEpisodeImages, and ExtractMusicAlbumImages
func ProcessAndSaveImages(
	ctx context.Context,
	repo images.Repository,
	metadataExtractor *infraImages.MetadataExtractor,
	extractedImages *infraImages.ExtractedImages,
	mediaType images.MediaType,
	entityID int,
	mediaID *int,
) error {
	if extractedImages == nil || len(extractedImages.Images) == 0 {
		return nil
	}

	// Process each image
	for _, imgInfo := range extractedImages.Images {
		// Extract metadata
		metadata, err := metadataExtractor.ExtractMetadata(imgInfo.Path)
		if err != nil {
			slog.Warn("Failed to extract metadata for image",
				"path", imgInfo.Path,
				"error", err)
			continue
		}

		// Create domain image
		// NOTE: LocalCachePath is NOT populated in Phase 4.1
		// Current approach: Catalog images by reference only, serve from original paths
		// Future (Phase 4.3): Populate cache at data/cache/images/{hash}_original.{ext}
		// See docs/PHASE_4_1_GAP_ANALYSIS.md for details
		img := &images.Image{
			MediaID:       mediaID,
			MediaType:     mediaType,
			EntityID:      entityID,
			ImageType:     imgInfo.Type,
			SourceType:    images.SourceTypeLocal,
			FilePath:      imgInfo.Path, // Original file path in user's media directory
			// LocalCachePath: nil,       // Cache not populated yet (Phase 4.3)
			Width:         metadata.Width,
			Height:        metadata.Height,
			FileSizeBytes: metadata.FileSizeBytes,
			MimeType:      metadata.MimeType,
			FileHash:      metadata.FileHash, // SHA256 for future deduplication
			Language:      imgInfo.Language,
			Priority:      imgInfo.Priority,
		}

		// Validate
		if err := img.Validate(); err != nil {
			slog.Warn("Invalid image entity",
				"path", imgInfo.Path,
				"error", err)
			continue
		}

		// Save to database
		if err := repo.Create(ctx, img); err != nil {
			slog.Error("Failed to save image to database",
				"path", imgInfo.Path,
				"error", err)
			continue
		}

		slog.Debug("Image cataloged",
			"path", imgInfo.Path,
			"type", imgInfo.Type,
			"media_type", mediaType)
	}

	return nil
}
