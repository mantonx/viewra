package progress

import "context"

// UpdateProgressExecutor defines the interface for updating watch progress
type UpdateProgressExecutor interface {
	Execute(ctx context.Context, req *UpdateProgressRequest) (*WatchProgressResponse, error)
}

// GetProgressExecutor defines the interface for getting watch progress
type GetProgressExecutor interface {
	Execute(ctx context.Context, mediaID, userID int64) (*WatchProgressResponse, error)
}

// GetBatchProgressExecutor defines the interface for getting batch progress
type GetBatchProgressExecutor interface {
	Execute(ctx context.Context, mediaIDs []int64, userID int64) (*BatchProgressResponse, error)
}

// ListProgressExecutor defines the interface for listing watch progress
type ListProgressExecutor interface {
	Execute(ctx context.Context, userID int64, limit, offset int) (*ListProgressResponse, error)
	ExecuteWatched(ctx context.Context, userID int64, limit, offset int) (*ListProgressResponse, error)
	ExecuteInProgress(ctx context.Context, userID int64, limit, offset int) (*ListProgressResponse, error)
}

// MarkWatchedExecutor defines the interface for marking items as watched/unwatched
type MarkWatchedExecutor interface {
	MarkWatched(ctx context.Context, req *MarkWatchedRequest) (*WatchProgressResponse, error)
	MarkUnwatched(ctx context.Context, req *MarkWatchedRequest) (*WatchProgressResponse, error)
}

// DeleteProgressExecutor defines the interface for deleting watch progress
type DeleteProgressExecutor interface {
	Execute(ctx context.Context, mediaID, userID int64) error
}
