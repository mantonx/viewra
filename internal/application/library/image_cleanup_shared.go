package library

import (
	"context"

	"github.com/viewra/viewra/internal/application/images"
	domainImages "github.com/viewra/viewra/internal/domain/images"
)

// ImageCleanupExecutor interface for cleaning up image cache files
// This is shared across library and media use cases
type ImageCleanupExecutor interface {
	CleanOrphanedImages(ctx context.Context) (*images.CleanupStats, error)
	CleanCacheForHashes(ctx context.Context, hashes []string) error
}

// CollectImageHashesForMedia retrieves all image hashes for a media item
// This is used before deleting media to enable cache cleanup
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
