package progress

import (
	"context"
	"time"

	"github.com/mantonx/viewra/internal/domain/progress"
)

// UpdateProgress updates or creates watch progress for a media item.
// Automatically marks as watched if progress exceeds 90%.
func UpdateProgress(ctx context.Context, repo progress.Repository, req *UpdateProgressRequest) (*WatchProgressResponse, error) {
	// Validate request
	if err := validateMediaID(req.MediaID); err != nil {
		return nil, err
	}

	if req.ProgressSeconds < 0 {
		return nil, progress.ErrInvalidProgress
	}

	if req.DurationSeconds < 0 {
		return nil, progress.ErrInvalidDuration
	}

	if req.ProgressSeconds > req.DurationSeconds {
		return nil, progress.ErrProgressExceedsDuration
	}

	// Try to get existing progress
	existing, err := repo.GetByMediaIDAndUserID(ctx, req.MediaID, req.UserID)
	if err != nil && err != progress.ErrProgressNotFound {
		return nil, err
	}

	if existing != nil {
		// Update existing progress
		existing.UpdateProgress(req.ProgressSeconds)
		existing.DurationSeconds = req.DurationSeconds

		if err := repo.Update(ctx, existing); err != nil {
			return nil, err
		}

		return toResponse(existing), nil
	}

	// Create new progress
	newProgress := &progress.WatchProgress{
		MediaID:         req.MediaID,
		UserID:          req.UserID,
		ProgressSeconds: req.ProgressSeconds,
		DurationSeconds: req.DurationSeconds,
		LastWatchedAt:   time.Now(),
	}

	// Auto-mark as watched if threshold met
	if newProgress.ShouldMarkWatched() {
		newProgress.IsWatched = true
	}

	if err := repo.Create(ctx, newProgress); err != nil {
		return nil, err
	}

	return toResponse(newProgress), nil
}
