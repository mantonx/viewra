package progress

import "context"

// Repository defines the interface for persisting and retrieving watch progress.
type Repository interface {
	// Create creates a new watch progress record.
	// Returns ErrProgressAlreadyExists if progress already exists for this media.
	Create(ctx context.Context, progress *WatchProgress) error

	// Update updates an existing watch progress record.
	// Returns ErrProgressNotFound if the progress doesn't exist.
	Update(ctx context.Context, progress *WatchProgress) error

	// GetByMediaID retrieves watch progress for a specific media item.
	// Returns ErrProgressNotFound if no progress exists.
	GetByMediaID(ctx context.Context, mediaID int64) (*WatchProgress, error)

	// GetByMediaIDAndUserID retrieves watch progress for a specific media item and user.
	// Returns ErrProgressNotFound if no progress exists.
	GetByMediaIDAndUserID(ctx context.Context, mediaID, userID int64) (*WatchProgress, error)

	// GetBatchByMediaIDs retrieves watch progress for multiple media items.
	// Returns a map of media_id -> WatchProgress. Missing media IDs won't be in the map.
	GetBatchByMediaIDs(ctx context.Context, mediaIDs []int64, userID int64) (map[int64]*WatchProgress, error)

	// ListByUserID retrieves all watch progress records for a user.
	// Returns empty slice if no progress exists.
	ListByUserID(ctx context.Context, userID int64, limit, offset int) ([]*WatchProgress, error)

	// ListWatchedByUserID retrieves all watched items for a user.
	// Returns empty slice if no watched items exist.
	ListWatchedByUserID(ctx context.Context, userID int64, limit, offset int) ([]*WatchProgress, error)

	// ListInProgressByUserID retrieves all in-progress items (not completed, not at 0%) for a user.
	// Returns empty slice if no in-progress items exist.
	ListInProgressByUserID(ctx context.Context, userID int64, limit, offset int) ([]*WatchProgress, error)

	// Delete deletes a watch progress record.
	// Returns ErrProgressNotFound if the progress doesn't exist.
	Delete(ctx context.Context, id int64) error

	// DeleteByMediaID deletes watch progress for a specific media item.
	// Returns ErrProgressNotFound if no progress exists.
	DeleteByMediaID(ctx context.Context, mediaID int64) error

	// Upsert creates or updates watch progress.
	// If progress exists, it updates; otherwise, it creates.
	Upsert(ctx context.Context, progress *WatchProgress) error
}
