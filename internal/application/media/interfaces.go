package media

import "context"

// GetMediaExecutor defines the interface for getting media
type GetMediaExecutor interface {
	Execute(ctx context.Context, id int64) (MediaResponse, error)
}

// ListMediaExecutor defines the interface for listing media
type ListMediaExecutor interface {
	ExecuteAll(ctx context.Context) (ListMediaResponse, error)
	Execute(ctx context.Context, libraryID int64) (ListMediaResponse, error)
}
