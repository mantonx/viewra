package progress

import (
	"time"

	"github.com/viewra/viewra/internal/domain/progress"
)

// UpdateProgressRequest represents a request to update watch progress.
type UpdateProgressRequest struct {
	MediaID         int64 `json:"media_id"`
	UserID          int64 `json:"user_id"`
	ProgressSeconds int   `json:"progress_seconds"`
	DurationSeconds int   `json:"duration_seconds"`
}

// WatchProgressResponse represents watch progress information in API responses.
type WatchProgressResponse struct {
	ID                 int64     `json:"id"`
	MediaID            int64     `json:"media_id"`
	UserID             int64     `json:"user_id"`
	ProgressSeconds    int       `json:"progress_seconds"`
	DurationSeconds    int       `json:"duration_seconds"`
	ProgressPercentage float64   `json:"progress_percentage"`
	IsWatched          bool      `json:"is_watched"`
	LastWatchedAt      time.Time `json:"last_watched_at"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// ListProgressResponse represents a list of watch progress records.
type ListProgressResponse struct {
	Progress []*WatchProgressResponse `json:"progress"`
	Total    int                      `json:"total"`
}

// toResponse converts a domain entity to a response DTO.
func toResponse(prog *progress.WatchProgress) *WatchProgressResponse {
	return &WatchProgressResponse{
		ID:                 prog.ID,
		MediaID:            prog.MediaID,
		UserID:             prog.UserID,
		ProgressSeconds:    prog.ProgressSeconds,
		DurationSeconds:    prog.DurationSeconds,
		ProgressPercentage: prog.GetProgressPercentage(),
		IsWatched:          prog.IsWatched,
		LastWatchedAt:      prog.LastWatchedAt,
		CreatedAt:          prog.CreatedAt,
		UpdatedAt:          prog.UpdatedAt,
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
