package library

import (
	"context"
	"fmt"

	domainImages "github.com/viewra/viewra/internal/domain/images"
	"github.com/viewra/viewra/internal/domain/library"
)

// DeleteLibraryUseCase handles the business logic for deleting a library
type DeleteLibraryUseCase struct {
	repo         library.Repository
	imageRepo    domainImages.Repository
	imageCleanup ImageCleanupExecutor
}

// NewDeleteLibraryUseCase creates a new instance of DeleteLibraryUseCase
func NewDeleteLibraryUseCase(
	repo library.Repository,
	imageRepo domainImages.Repository,
	imageCleanup ImageCleanupExecutor,
) *DeleteLibraryUseCase {
	return &DeleteLibraryUseCase{
		repo:         repo,
		imageRepo:    imageRepo,
		imageCleanup: imageCleanup,
	}
}

// Execute deletes a library by its ID
//
// IMPORTANT: This only removes database records, NOT actual media files on disk.
// Media files are user-managed. Only cached/generated files are cleaned up.
//
// What gets deleted:
// - Library database record
// - All media database records (CASCADE)
// - All image database records (CASCADE)
// - Image cache files (thumbnails, posters, etc.)
// - Transcode job records (CASCADE)
// - Watch progress records (CASCADE)
//
// What is preserved:
// - Original media files on disk (NEVER deleted)
func (uc *DeleteLibraryUseCase) Execute(ctx context.Context, id int64) error {
	// Verify library exists
	_, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get library: %w", err)
	}

	// Delete the library
	// This cascades to media table, which cascades to media_images table (ON DELETE CASCADE)
	// Database records will be deleted, but cache files will remain until cleanup runs
	if err := uc.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete library: %w", err)
	}

	// Trigger image cache cleanup to remove orphaned cache files immediately
	// This is a "best effort" cleanup - failures won't prevent library deletion
	if uc.imageCleanup != nil {
		if _, err := uc.imageCleanup.CleanOrphanedImages(ctx); err != nil {
			// Log but don't fail - the scheduled cleanup will handle this later
			fmt.Printf("warning: failed to clean image cache after deleting library %d: %v\n", id, err)
		}
	}

	return nil
}
