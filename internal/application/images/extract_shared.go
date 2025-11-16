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
	cacheService *infraImages.CacheService,
	transformer *infraImages.Transformer,
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

		// Generate all preset size variants during scan (Phase 4.1+ implementation)
		// Cache format: data/cache/images/{first2}/{next2}/{hash}_{preset_name}.webp
		// Example presets: thumb, medium, large, xlarge (based on image type)
		var localCachePath *string
		cachedMimeType := metadata.MimeType // Default to original if caching fails
		if transformer != nil && metadata.FileHash != nil {
			// Generate all preset sizes for this image type
			presetPaths, err := transformer.TransformAllPresets(imgInfo.Path, *metadata.FileHash, imgInfo.Type)
			if err != nil {
				slog.Warn("Failed to generate image presets",
					"path", imgInfo.Path,
					"image_type", imgInfo.Type,
					"error", err)
				// Continue without cache - we'll serve from original path
			} else if len(presetPaths) > 0 {
				// Store the "medium" preset path as the default cache path
				// API can construct other preset paths using the file hash
				mediumPath, hasMedium := presetPaths["medium"]
				if hasMedium {
					localCachePath = &mediumPath
				} else {
					// Fallback to any available preset
					for _, path := range presetPaths {
						localCachePath = &path
						break
					}
				}
				webpMime := "image/webp"
				cachedMimeType = &webpMime // All presets are WebP
				slog.Debug("Image presets generated",
					"path", imgInfo.Path,
					"image_type", imgInfo.Type,
					"preset_count", len(presetPaths))
			}
		}

		// Create domain image
		img := &images.Image{
			MediaID:        mediaID,
			MediaType:      mediaType,
			EntityID:       entityID,
			ImageType:      imgInfo.Type,
			SourceType:     images.SourceTypeLocal,
			FilePath:       imgInfo.Path,      // Original file path in user's media directory
			LocalCachePath: localCachePath,    // Cache path populated during scan
			Width:          metadata.Width,
			Height:         metadata.Height,
			FileSizeBytes:  metadata.FileSizeBytes,
			MimeType:       cachedMimeType,    // WebP if cached, original if not
			FileHash:       metadata.FileHash, // SHA256 for deduplication
			Language:       imgInfo.Language,
			Priority:       imgInfo.Priority,
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
