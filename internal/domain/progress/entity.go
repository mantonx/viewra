package progress

import (
	"time"
)

// WatchProgress represents the viewing progress for a media item.
// Tracks how much of a video has been watched and completion status.
type WatchProgress struct {
	ID              int64
	UserID          int64   // Foreign key to users table
	MediaID         int64   // Foreign key to media table
	ProgressSeconds float64 // Current playback position in seconds (float for sub-second precision)
	DurationSeconds float64 // Total duration at time of viewing (cached for percentage calculation)
	IsWatched       bool    // True if user explicitly marked as watched or >90% complete
	LastWatchedAt   time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time

	// Playback preferences (nil means use defaults)
	SelectedQuality       *string // Quality profile ID (e.g., "1080p-10m", "original")
	SelectedAudioTrack    *int    // FFmpeg audio stream index
	SelectedSubtitleTrack *int    // Subtitle track ID (-1 for off, nil for auto)
}

// IsValid validates the watch progress entity fields.
func (wp *WatchProgress) IsValid() error {
	if wp.MediaID <= 0 {
		return ErrInvalidMediaID
	}

	if wp.ProgressSeconds < 0 {
		return ErrInvalidProgress
	}

	if wp.DurationSeconds < 0 {
		return ErrInvalidDuration
	}

	if wp.ProgressSeconds > wp.DurationSeconds {
		return ErrProgressExceedsDuration
	}

	return nil
}

// GetProgressPercentage returns the viewing progress as a percentage (0-100).
func (wp *WatchProgress) GetProgressPercentage() float64 {
	if wp.DurationSeconds == 0 {
		return 0.0
	}

	percentage := (wp.ProgressSeconds / wp.DurationSeconds) * 100.0
	if percentage > 100.0 {
		return 100.0
	}

	return percentage
}

// ShouldMarkWatched returns true if the progress is >90% and should be auto-marked as watched.
func (wp *WatchProgress) ShouldMarkWatched() bool {
	return wp.GetProgressPercentage() >= 90.0
}

// UpdateProgress updates the progress position and auto-marks as watched if threshold met.
// If progress drops below 90%, the watched flag is automatically cleared.
func (wp *WatchProgress) UpdateProgress(progressSeconds float64) {
	wp.ProgressSeconds = progressSeconds
	wp.LastWatchedAt = time.Now()
	wp.UpdatedAt = time.Now()

	// Auto-mark as watched if >90% complete, or unmark if below threshold
	if wp.ShouldMarkWatched() {
		wp.IsWatched = true
	} else {
		// If progress drops below threshold, clear the watched flag
		wp.IsWatched = false
	}
}

// MarkWatched explicitly marks the media as fully watched.
func (wp *WatchProgress) MarkWatched() {
	wp.IsWatched = true
	wp.ProgressSeconds = wp.DurationSeconds
	wp.LastWatchedAt = time.Now()
	wp.UpdatedAt = time.Now()
}

// MarkUnwatched resets the watched status and progress.
func (wp *WatchProgress) MarkUnwatched() {
	wp.IsWatched = false
	wp.ProgressSeconds = 0
	wp.UpdatedAt = time.Now()
}
