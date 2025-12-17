package media

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mantonx/viewra/internal/application/library/scan/cleanup"
	domainevents "github.com/mantonx/viewra/internal/domain/events"
	"github.com/mantonx/viewra/internal/domain/images"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/pkg/logger"
)

// DeleteMediaUseCase handles deletion of media items with proper cleanup
type DeleteMediaUseCase struct {
	mediaRepo    media.Repository
	imageRepo    images.Repository
	imageCleanup cleanup.ImageCleanupExecutor
	logger       *slog.Logger
	publisher    domainevents.Publisher // Event publisher for media.removed events (optional)
}

// NewDeleteMediaUseCase creates a new delete media use case
func NewDeleteMediaUseCase(
	mediaRepo media.Repository,
	imageRepo images.Repository,
	imageCleanup cleanup.ImageCleanupExecutor,
	log *slog.Logger,
) *DeleteMediaUseCase {
	return &DeleteMediaUseCase{
		mediaRepo:    mediaRepo,
		imageRepo:    imageRepo,
		imageCleanup: imageCleanup,
		logger:       logger.DefaultIfNil(log),
	}
}

// SetPublisher sets the event publisher for media lifecycle events.
func (uc *DeleteMediaUseCase) SetPublisher(pub domainevents.Publisher) {
	uc.publisher = pub
}

// Execute deletes a media database record and cleans up associated image cache files
//
// IMPORTANT: This ONLY deletes the database record, NOT the actual media file on disk.
// Media files are user-managed and should never be deleted by the application.
// Only cached/generated files (like image thumbnails, transcodes) are cleaned up.
func (uc *DeleteMediaUseCase) Execute(ctx context.Context, mediaID int64) error {
	// Fetch media info before deletion for event publishing
	var mediaInfo *media.Media
	if uc.publisher != nil {
		m, err := uc.mediaRepo.GetByID(ctx, mediaID)
		if err == nil {
			mediaInfo = m
		}
		// If fetch fails, we still proceed with deletion but skip the event
	}

	// Collect image hashes before deletion for cache cleanup
	hashes := cleanup.CollectImageHashesForMedia(ctx, uc.imageRepo, mediaID)

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

	// Publish media.removed event
	if uc.publisher != nil && mediaInfo != nil {
		uc.publisher.Publish(domainevents.NewEvent(domainevents.EventMediaRemoved, "delete-media").
			WithMediaID(mediaID).
			WithLibraryID(mediaInfo.LibraryID).
			WithData("file_path", mediaInfo.FilePath).
			WithData("type", mediaInfo.Type).
			WithData("reason", "user_deleted").
			Build())
	}

	// Clean up image cache files for the collected hashes
	// This will only delete files if no other images reference the same hash
	if uc.imageCleanup != nil && len(hashes) > 0 {
		if err := uc.imageCleanup.CleanCacheForHashes(ctx, hashes); err != nil {
			// Log but don't fail - the scheduled cleanup will handle this later
			// We don't want to rollback the media deletion due to cleanup failures
			uc.logger.Warn("failed to clean image cache for media",
				"media_id", mediaID,
				"hash_count", len(hashes),
				"error", err)
		}
	}

	return nil
}
