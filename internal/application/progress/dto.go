package progress

import (
	"time"

	"github.com/mantonx/viewra/internal/domain/progress"
)

// UpdateProgressRequest represents a request to update watch progress.
type UpdateProgressRequest struct {
	MediaID         int64   `json:"media_id"`
	UserID          int64   `json:"user_id"`
	ProgressSeconds float64 `json:"progress_seconds"`
	DurationSeconds float64 `json:"duration_seconds"`

	// Device profile for device-specific preferences (e.g., "chrome-h264-sdr", "firefox-h264h265-hdr")
	// If provided, preferences are stored per-device; otherwise stored globally
	DeviceProfile string `json:"device_profile,omitempty"`

	// Playback preferences (omitted or null means don't update)
	SelectedQuality       *string `json:"selected_quality,omitempty"`
	SelectedAudioTrack    *int    `json:"selected_audio_track,omitempty"`
	SelectedSubtitleTrack *int    `json:"selected_subtitle_track,omitempty"`
}

// WatchProgressResponse represents watch progress information in API responses.
type WatchProgressResponse struct {
	ID                 int64     `json:"id"`
	MediaID            int64     `json:"media_id"`
	UserID             int64     `json:"user_id"`
	ProgressSeconds    float64   `json:"progress_seconds"`
	DurationSeconds    float64   `json:"duration_seconds"`
	ProgressPercentage float64   `json:"progress_percentage"`
	IsWatched          bool      `json:"is_watched"`
	LastWatchedAt      time.Time `json:"last_watched_at"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`

	// Playback preferences (null means no saved preference / use defaults)
	// Note: No omitempty - we need to distinguish "never set" from "explicitly null"
	SelectedQuality       *string `json:"selected_quality"`
	SelectedAudioTrack    *int    `json:"selected_audio_track"`
	SelectedSubtitleTrack *int    `json:"selected_subtitle_track"`
}

// ListProgressResponse represents a list of watch progress records.
type ListProgressResponse struct {
	Progress []*WatchProgressResponse `json:"progress"`
	Total    int                      `json:"total"`
}

// BatchProgressResponse represents batch progress for multiple media items.
type BatchProgressResponse struct {
	Progress map[int64]*WatchProgressResponse `json:"progress"` // map[media_id]progress
}

// toResponse converts a domain entity to a response DTO.
func toResponse(prog *progress.WatchProgress) *WatchProgressResponse {
	return &WatchProgressResponse{
		ID:                    prog.ID,
		MediaID:               prog.MediaID,
		UserID:                prog.UserID,
		ProgressSeconds:       prog.ProgressSeconds,
		DurationSeconds:       prog.DurationSeconds,
		ProgressPercentage:    prog.GetProgressPercentage(),
		IsWatched:             prog.IsWatched,
		LastWatchedAt:         prog.LastWatchedAt,
		CreatedAt:             prog.CreatedAt,
		UpdatedAt:             prog.UpdatedAt,
		SelectedQuality:       prog.SelectedQuality,
		SelectedAudioTrack:    prog.SelectedAudioTrack,
		SelectedSubtitleTrack: prog.SelectedSubtitleTrack,
	}
}

// toListResponse converts a slice of domain entities to a list response DTO.
func toListResponse(progresses []*progress.WatchProgress) *ListProgressResponse {
	responses := make([]*WatchProgressResponse, len(progresses))
	for i, prog := range progresses {
		responses[i] = toResponse(prog)
	}

	return &ListProgressResponse{
		Progress: responses,
		Total:    len(responses),
	}
}

// validateMediaID validates that a media ID is positive.
func validateMediaID(mediaID int64) error {
	if mediaID <= 0 {
		return progress.ErrInvalidMediaID
	}
	return nil
}
