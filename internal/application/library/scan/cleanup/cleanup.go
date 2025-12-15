package cleanup

import (
	"context"

	"github.com/mantonx/viewra/internal/application/library/scan"
	domainImages "github.com/mantonx/viewra/internal/domain/images"
)

// CleanupStaleMedia removes media database records and associated images for files that no longer exist on disk.
//
// IMPORTANT: This only deletes database records and image cache files, NOT actual media files.
// The media files were already removed by the user from disk - we're just cleaning up our catalog.
func CleanupStaleMedia(ctx context.Context, deps *Deps, libraryID int64, foundFiles map[string]bool) {
	// Get all media items for this library
	allMedia, err := deps.MediaRepos.Media.ListByLibrary(ctx, libraryID)
	if err != nil {
		deps.Logger.Warn("failed to list media for stale cleanup",
			"library_id", libraryID,
			"error", err)
		return
	}

	// Count how many files would be marked as stale
	staleCount := 0
	for _, m := range allMedia {
		if !foundFiles[m.FilePath] {
			staleCount++
		}
	}

	// SAFETY: Don't delete if too many files are "stale"
	// This likely indicates scan failure (permission error, network issue, etc.), not actual deletions
	// Better to leave stale records than accidentally delete valid media entries
	if len(allMedia) > 0 {
		stalePercent := float64(staleCount) / float64(len(allMedia)) * 100
		if stalePercent > scan.StaleMediaThresholdPercent {
			deps.Logger.Error("refusing to cleanup - too many files marked stale",
				"library_id", libraryID,
				"stale_count", staleCount,
				"total_count", len(allMedia),
				"stale_percent", stalePercent,
				"reason", "likely indicates scan failure, not actual file deletions")
			return
		}
	}

	if staleCount == 0 {
		deps.Logger.Info("no stale media to cleanup", "library_id", libraryID)
		return
	}

	deps.Logger.Info("cleaning up stale media records",
		"library_id", libraryID,
		"stale_count", staleCount,
		"stale_percent", float64(staleCount)/float64(len(allMedia))*100)

	// Track hashes for cleanup
	var hashesToClean []string

	// Find media that's in the database but not on disk
	for _, m := range allMedia {
		if !foundFiles[m.FilePath] {
			// This media file no longer exists on disk - delete it from database

			// Collect image hashes for this media before deletion
			mediaHashes := CollectImageHashesForMedia(ctx, deps.ImageRepo, m.ID)
			hashesToClean = append(hashesToClean, mediaHashes...)

			// Delete the media database record (cascades to images, transcode jobs, etc.)
			if err := deps.MediaRepos.Media.Delete(ctx, m.ID); err != nil {
				deps.Logger.Warn("failed to delete stale media",
					"library_id", libraryID,
					"media_id", m.ID,
					"file_path", m.FilePath,
					"error", err)
			} else {
				deps.Logger.Info("removed stale media from library",
					"library_id", libraryID,
					"file_path", m.FilePath)
			}
		}
	}

	// Clean up image cache files for all the removed media
	if deps.ImageCleanup != nil && len(hashesToClean) > 0 {
		if err := deps.ImageCleanup.CleanCacheForHashes(ctx, hashesToClean); err != nil {
			deps.Logger.Warn("failed to clean image cache during library scan",
				"library_id", libraryID,
				"hash_count", len(hashesToClean),
				"error", err)
		}
	}
}

// CollectImageHashesForMedia retrieves all image hashes for a media item.
// This is used before deleting media to enable cache cleanup.
func CollectImageHashesForMedia(ctx context.Context, imageRepo domainImages.Repository, mediaID int64) []string {
	if imageRepo == nil {
		return nil
	}

	mediaImages, err := imageRepo.GetByMediaID(ctx, int(mediaID))
	if err != nil {
		// Not critical - caller will handle
		return nil
	}

	// Collect unique hashes
	hashSet := make(map[string]bool)
	for _, img := range mediaImages {
		if img.FileHash != nil && *img.FileHash != "" {
			hashSet[*img.FileHash] = true
		}
	}

	// Convert to slice
	hashes := make([]string, 0, len(hashSet))
	for hash := range hashSet {
		hashes = append(hashes, hash)
	}

	return hashes
}
