package progress

import (
	"context"

	"github.com/viewra/viewra/internal/domain/progress"
)

// GetProgressByMediaID retrieves watch progress for a specific media item.
func GetProgressByMediaID(ctx context.Context, repo progress.Repository, mediaID int64) (*WatchProgressResponse, error) {
	prog, err := repo.GetByMediaID(ctx, mediaID)
	if err != nil {
		return nil, err
	}

	return toResponse(prog), nil
}

// GetProgressByMediaIDAndUserID retrieves watch progress for a specific media item and user.
func GetProgressByMediaIDAndUserID(ctx context.Context, repo progress.Repository, mediaID, userID int64) (*WatchProgressResponse, error) {
	prog, err := repo.GetByMediaIDAndUserID(ctx, mediaID, userID)
	if err != nil {
		return nil, err
	}

	return toResponse(prog), nil
}

// ListProgressByUserID retrieves all watch progress records for a user.
func ListProgressByUserID(ctx context.Context, repo progress.Repository, userID int64, limit, offset int) (*ListProgressResponse, error) {
	progresses, err := repo.ListByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}

	return toListResponse(progresses), nil
}

// ListWatchedByUserID retrieves all watched items for a user.
func ListWatchedByUserID(ctx context.Context, repo progress.Repository, userID int64, limit, offset int) (*ListProgressResponse, error) {
	progresses, err := repo.ListWatchedByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}

	return toListResponse(progresses), nil
}

// ListInProgressByUserID retrieves all in-progress items for a user.
func ListInProgressByUserID(ctx context.Context, repo progress.Repository, userID int64, limit, offset int) (*ListProgressResponse, error) {
	progresses, err := repo.ListInProgressByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}

	return toListResponse(progresses), nil
}
