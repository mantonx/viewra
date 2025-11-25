package images

import (
	"context"
	"log/slog"

	"github.com/mantonx/viewra/internal/domain/images"
	infraImages "github.com/mantonx/viewra/internal/infrastructure/images"
	"github.com/mantonx/viewra/internal/pkg/logger"
)

// ProcessAndSaveImages is a shared helper that processes extracted images and saves them to the database
// This eliminates duplication across ExtractMovieImages, ExtractTVEpisodeImages, and ExtractMusicAlbumImages
func ProcessAndSaveImages(
	ctx context.Context,
	log *slog.Logger,
	repo images.Repository,
	metadataExtractor *infraImages.MetadataExtractor,
	cacheService *infraImages.CacheService,
	transformer *infraImages.Transformer,
	extractedImages *infraImages.ExtractedImages,
	mediaType images.MediaType,
	entityID int,
	mediaID *int,
) error {
	// Use default logger if none provided
	log = logger.DefaultIfNil(log)

	if extractedImages == nil || len(extractedImages.Images) == 0 {
		log.Info("no images extracted", "media_type", mediaType, "entity_id", entityID)
		return nil
	}

	log.Info("processing extracted images",
		"media_type", mediaType,
		"entity_id", entityID,
		"image_count", len(extractedImages.Images))

	// Process each image
	for _, imgInfo := range extractedImages.Images {
		// Extract metadata (calculate hash to check for duplicates)
		metadata, err := metadataExtractor.ExtractMetadata(imgInfo.Path)
		if err != nil {
			log.Warn("failed to extract metadata for image",
				"path", imgInfo.Path,
				"error", err)
			continue
		}

		// Check if an image with this hash already exists
		// This enables smart deduplication and cache reuse
		var existingImage *images.Image
		if metadata.FileHash != nil {
			existingImages, err := repo.GetByHash(ctx, *metadata.FileHash)
			if err != nil {
				log.Warn("failed to check for existing image by hash",
					"hash", *metadata.FileHash,
					"error", err)
			} else if len(existingImages) > 0 {
				// Found existing image(s) with same hash
				// Check if any match our current media_id/entity_id and image_type
				for _, existing := range existingImages {
					matchesMedia := (mediaID != nil && existing.MediaID != nil && *mediaID == *existing.MediaID) ||
						(mediaID == nil && existing.MediaID == nil)
					matchesEntity := existing.EntityID == entityID && existing.MediaType == mediaType
					matchesType := existing.ImageType == imgInfo.Type

					if matchesMedia && matchesEntity && matchesType {
						existingImage = existing
						break
					}
				}

				// If we found a matching image, check if we can reuse it
				if existingImage != nil {
					// Image already exists with same hash - skip processing
					log.Debug("reusing existing image with matching hash",
						"path", imgInfo.Path,
						"hash", *metadata.FileHash,
						"existing_id", existingImage.ID,
						"cache_path", existingImage.LocalCachePath)
					continue
				}

				// If hash exists but for different media/entity, we can still reuse the cache
				// by copying the cache path from the existing image
				log.Debug("found existing image with same hash for different media",
					"hash", *metadata.FileHash,
					"existing_count", len(existingImages))
			}
		}

		// Generate all preset size variants during scan (Phase 4.1+ implementation)
		// Cache format: data/cache/images/{first2}/{next2}/{hash}_{preset_name}.webp
		// Example presets: thumb, medium, large, xlarge (based on image type)
		var localCachePath *string
		cachedMimeType := metadata.MimeType // Default to original if caching fails

		// Check if we can reuse cache from existing image with same hash
		if existingImage == nil && metadata.FileHash != nil {
			// Check if any existing image has this hash and valid cache
			existingImages, err := repo.GetByHash(ctx, *metadata.FileHash)
			if err == nil && len(existingImages) > 0 {
				for _, existing := range existingImages {
					if existing.LocalCachePath != nil && *existing.LocalCachePath != "" {
						// Reuse the cache path from existing image
						localCachePath = existing.LocalCachePath
						if existing.MimeType != nil {
							cachedMimeType = existing.MimeType
						}
						log.Debug("reusing cache from existing image with same hash",
							"hash", *metadata.FileHash,
							"cache_path", *localCachePath)
						break
					}
				}
			}
		}

		// Only generate new cache if we don't have one from an existing image
		if localCachePath == nil && transformer != nil && metadata.FileHash != nil {
			// Generate all preset sizes for this image type
			presetPaths, err := transformer.TransformAllPresets(imgInfo.Path, *metadata.FileHash, imgInfo.Type)
			if err != nil {
				log.Warn("failed to generate image presets",
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
				log.Debug("image presets generated",
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
			FileHash:       metadata.FileHash, // XXH3-128 for deduplication
			Language:       imgInfo.Language,
			Priority:       imgInfo.Priority,
		}

		// Validate
		if err := img.Validate(); err != nil {
			log.Warn("invalid image entity",
				"path", imgInfo.Path,
				"error", err)
			continue
		}

		// Save to database
		if err := repo.Create(ctx, img); err != nil {
			log.Error("failed to save image to database",
				"path", imgInfo.Path,
				"error", err)
			continue
		}

		log.Debug("image cataloged",
			"path", imgInfo.Path,
			"type", imgInfo.Type,
			"media_type", mediaType)
	}

	return nil
}
