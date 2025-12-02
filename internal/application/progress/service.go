package progress

import (
	"context"

	"github.com/mantonx/viewra/internal/domain/progress"
)

// Service provides all progress-related use cases.
// Implements UpdateProgressExecutor, GetProgressExecutor, GetBatchProgressExecutor,
// ListProgressExecutor, MarkWatchedExecutor, and DeleteProgressExecutor.
type Service struct {
	repo progress.Repository
}

// NewService creates a new progress service
func NewService(repo progress.Repository) *Service {
	return &Service{repo: repo}
}

// Execute updates or creates watch progress for a media item (UpdateProgressExecutor)
func (s *Service) Execute(ctx context.Context, req *UpdateProgressRequest) (*WatchProgressResponse, error) {
	return UpdateProgress(ctx, s.repo, req)
}

// GetProgress retrieves watch progress for a specific media item (GetProgressExecutor)
func (s *Service) GetProgress(ctx context.Context, mediaID, userID int64) (*WatchProgressResponse, error) {
	return GetProgressByMediaIDAndUserID(ctx, s.repo, mediaID, userID)
}

// GetBatchProgress retrieves watch progress for multiple media items (GetBatchProgressExecutor)
func (s *Service) GetBatchProgress(ctx context.Context, mediaIDs []int64, userID int64) (*BatchProgressResponse, error) {
	return GetBatchProgressByMediaIDs(ctx, s.repo, mediaIDs, userID)
}

// ListProgress retrieves all watch progress records for a user (ListProgressExecutor)
func (s *Service) ListProgress(ctx context.Context, userID int64, limit, offset int) (*ListProgressResponse, error) {
	return ListProgressByUserID(ctx, s.repo, userID, limit, offset)
}

// ListWatched retrieves all watched items for a user (ListProgressExecutor)
func (s *Service) ListWatched(ctx context.Context, userID int64, limit, offset int) (*ListProgressResponse, error) {
	return ListWatchedByUserID(ctx, s.repo, userID, limit, offset)
}

// ListInProgress retrieves all in-progress items for a user (ListProgressExecutor)
func (s *Service) ListInProgress(ctx context.Context, userID int64, limit, offset int) (*ListProgressResponse, error) {
	return ListInProgressByUserID(ctx, s.repo, userID, limit, offset)
}

// MarkWatched marks a media item as fully watched (MarkWatchedExecutor)
func (s *Service) MarkWatched(ctx context.Context, req *MarkWatchedRequest) (*WatchProgressResponse, error) {
	return MarkWatched(ctx, s.repo, req)
}

// MarkUnwatched resets the watched status for a media item (MarkWatchedExecutor)
func (s *Service) MarkUnwatched(ctx context.Context, req *MarkWatchedRequest) (*WatchProgressResponse, error) {
	return MarkUnwatched(ctx, s.repo, req)
}

// DeleteProgress deletes watch progress for a media item (DeleteProgressExecutor)
func (s *Service) DeleteProgress(ctx context.Context, mediaID, userID int64) error {
	return DeleteProgress(ctx, s.repo, mediaID, userID)
}
