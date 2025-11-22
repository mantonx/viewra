package media

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/application/library"
	"github.com/mantonx/viewra/internal/domain/images"
	"github.com/mantonx/viewra/internal/domain/media"
)

// DeleteMediaUseCase handles deletion of media items with proper cleanup
type DeleteMediaUseCase struct {
	mediaRepo    media.Repository
	imageRepo    images.Repository
	imageCleanup library.ImageCleanupExecutor
}

// NewDeleteMediaUseCase creates a new delete media use case
func NewDeleteMediaUseCase(
	mediaRepo media.Repository,
	imageRepo images.Repository,
	imageCleanup library.ImageCleanupExecutor,
) *DeleteMediaUseCase {
	return &DeleteMediaUseCase{
		mediaRepo:    mediaRepo,
		imageRepo:    imageRepo,
		imageCleanup: imageCleanup,
	}
}

// Execute deletes a media database record and cleans up associated image cache files
//
// IMPORTANT: This ONLY deletes the database record, NOT the actual media file on disk.
// Media files are user-managed and should never be deleted by the application.
// Only cached/generated files (like image thumbnails, transcodes) are cleaned up.
func (uc *DeleteMediaUseCase) Execute(ctx context.Context, mediaID int64) error {
	// Collect image hashes before deletion for cache cleanup
	hashes := library.CollectImageHashesForMedia(ctx, uc.imageRepo, mediaID)

	// Delete the media DATABASE RECORD (NOT the actual media file)
	// This will CASCADE delete database records:
	// - media_images records (via foreign key)
	// - transcode_jobs records (via foreign key)
	// - watch_progress records (via foreign key)
	//
	// The actual media file on disk is NEVER touched.
	if err := uc.mediaRepo.Delete(ctx, mediaID); err != nil {
		return fmt.Errorf("failed to delete media: %w", err)
	}

	// Clean up image cache files for the collected hashes
	// This will only delete files if no other images reference the same hash
	if uc.imageCleanup != nil && len(hashes) > 0 {
		if err := uc.imageCleanup.CleanCacheForHashes(ctx, hashes); err != nil {
			// Log but don't fail - the scheduled cleanup will handle this later
			// We don't want to rollback the media deletion due to cleanup failures
			fmt.Printf("warning: failed to clean image cache for media %d: %v\n", mediaID, err)
		}
	}

	return nil
}
