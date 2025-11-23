package library

import (
	"context"
	"fmt"
)

// cleanupStaleMedia removes media database records and associated images for files that no longer exist on disk
//
// IMPORTANT: This only deletes database records and image cache files, NOT actual media files.
// The media files were already removed by the user from disk - we're just cleaning up our catalog.
func (uc *ScanLibraryUseCase) cleanupStaleMedia(ctx context.Context, libraryID int64, foundFiles map[string]bool) {
	// Get all media items for this library
	allMedia, err := uc.mediaRepos.Media.ListByLibrary(ctx, libraryID)
	if err != nil {
		fmt.Printf("warning: failed to list media for stale cleanup: %v\n", err)
		return
	}

	// Count how many files would be marked as stale
	staleCount := 0
	for _, m := range allMedia {
		if !foundFiles[m.FilePath] {
			staleCount++
		}
	}

	// SAFETY: Don't delete if >10% of library is "stale"
	// This likely indicates scan failure (permission error, network issue, etc.), not actual deletions
	// Better to leave stale records than accidentally delete valid media entries
	if len(allMedia) > 0 {
		stalePercent := float64(staleCount) / float64(len(allMedia)) * 100
		if stalePercent > 10.0 {
			fmt.Printf("error: refusing to cleanup - too many files marked stale (stale=%d, total=%d, percentage=%.1f%%). This likely indicates a scan failure, not actual file deletions.\n",
				staleCount, len(allMedia), stalePercent)
			return
		}
	}

	if staleCount == 0 {
		fmt.Printf("info: no stale media to cleanup\n")
		return
	}

	fmt.Printf("info: cleaning up %d stale media records (%.1f%% of library)\n",
		staleCount, float64(staleCount)/float64(len(allMedia))*100)

	// Track hashes for cleanup
	var hashesToClean []string

	// Find media that's in the database but not on disk
	for _, m := range allMedia {
		if !foundFiles[m.FilePath] {
			// This media file no longer exists on disk - delete it from database

			// Collect image hashes for this media before deletion
			mediaHashes := CollectImageHashesForMedia(ctx, uc.imageRepo, m.ID)
			hashesToClean = append(hashesToClean, mediaHashes...)

			// Delete the media database record (cascades to images, transcode jobs, etc.)
			if err := uc.mediaRepos.Media.Delete(ctx, m.ID); err != nil {
				fmt.Printf("warning: failed to delete stale media %d (%s): %v\n", m.ID, m.FilePath, err)
			} else {
				fmt.Printf("info: removed stale media from library: %s\n", m.FilePath)
			}
		}
	}

	// Clean up image cache files for all the removed media
	if uc.imageCleanup != nil && len(hashesToClean) > 0 {
		if err := uc.imageCleanup.CleanCacheForHashes(ctx, hashesToClean); err != nil {
			fmt.Printf("warning: failed to clean image cache during library scan: %v\n", err)
		}
	}
}
