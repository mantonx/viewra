package progress

import (
	"context"
	"time"

	"github.com/mantonx/viewra/internal/domain/progress"
)

// MarkWatchedRequest represents a request to mark media as watched or unwatched.
type MarkWatchedRequest struct {
	MediaID int64 `json:"media_id"`
	UserID  int64 `json:"user_id"`
}

// MarkWatched explicitly marks a media item as fully watched.
func MarkWatched(ctx context.Context, repo progress.Repository, req *MarkWatchedRequest) (*WatchProgressResponse, error) {
	if err := validateMediaID(req.MediaID); err != nil {
		return nil, err
	}

	// Try to get existing progress
	existing, err := repo.GetByMediaIDAndUserID(ctx, req.MediaID, req.UserID)
	if err != nil && err != progress.ErrProgressNotFound {
		return nil, err
	}

	if existing != nil {
		// Update existing progress to watched
		existing.MarkWatched()

		if err := repo.Update(ctx, existing); err != nil {
			return nil, err
		}

		return toResponse(existing), nil
	}

	// Create new progress marked as watched
	newProgress := &progress.WatchProgress{
		MediaID:         req.MediaID,
		UserID:          req.UserID,
		ProgressSeconds: 0, // Don't know duration yet
		DurationSeconds: 0,
		IsWatched:       true,
		LastWatchedAt:   time.Now(),
	}

	if err := repo.Create(ctx, newProgress); err != nil {
		return nil, err
	}

	return toResponse(newProgress), nil
}

// MarkUnwatched resets the watched status and progress for a media item.
func MarkUnwatched(ctx context.Context, repo progress.Repository, req *MarkWatchedRequest) (*WatchProgressResponse, error) {
	if err := validateMediaID(req.MediaID); err != nil {
		return nil, err
	}

	// Get existing progress
	existing, err := repo.GetByMediaIDAndUserID(ctx, req.MediaID, req.UserID)
	if err != nil {
		return nil, err
	}

	// Reset to unwatched
	existing.MarkUnwatched()

	if err := repo.Update(ctx, existing); err != nil {
		return nil, err
	}

	return toResponse(existing), nil
}

// DeleteProgress deletes watch progress for a media item.
func DeleteProgress(ctx context.Context, repo progress.Repository, mediaID, userID int64) error {
	if err := validateMediaID(mediaID); err != nil {
		return err
	}

	// Get the progress to find its ID
	prog, err := repo.GetByMediaIDAndUserID(ctx, mediaID, userID)
	if err != nil {
		return err
	}

	return repo.Delete(ctx, prog.ID)
}
