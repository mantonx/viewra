package progress

import (
	"context"
	"time"

	"github.com/mantonx/viewra/internal/domain/progress"
)

// UpdateProgress updates or creates watch progress for a media item.
// Automatically marks as watched if progress exceeds 90%.
// Also updates playback preferences if provided in the request.
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

	// Build progress entity with preferences
	prog := &progress.WatchProgress{
		MediaID:               req.MediaID,
		UserID:                req.UserID,
		ProgressSeconds:       req.ProgressSeconds,
		DurationSeconds:       req.DurationSeconds,
		LastWatchedAt:         time.Now(),
		SelectedQuality:       req.SelectedQuality,
		SelectedAudioTrack:    req.SelectedAudioTrack,
		SelectedSubtitleTrack: req.SelectedSubtitleTrack,
	}

	// Auto-mark as watched if threshold met
	if prog.ShouldMarkWatched() {
		prog.IsWatched = true
	}

	// Upsert handles both create and update, and preserves preferences via COALESCE
	if err := repo.Upsert(ctx, prog); err != nil {
		return nil, err
	}

	return toResponse(prog), nil
}
