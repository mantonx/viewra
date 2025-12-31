package progress

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/mantonx/viewra/internal/domain/progress"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Repository implements the domain progress.Repository interface.
type Repository struct {
	*common.BaseRepository
}

// NewRepository creates a new watch progress repository.
func NewRepository(db *common.BaseRepository) *Repository {
	return &Repository{
		BaseRepository: db,
	}
}

// Create creates a new watch progress record.
func (r *Repository) Create(ctx context.Context, prog *progress.WatchProgress) error {
	if err := prog.IsValid(); err != nil {
		return err
	}

	now := time.Now()
	prog.CreatedAt = now
	prog.UpdatedAt = now

	if prog.LastWatchedAt.IsZero() {
		prog.LastWatchedAt = now
	}

	result, err := r.Q().CreateWatchProgress(ctx, unified.CreateWatchProgressParams{
		MediaID:     prog.MediaID,
		UserID:      common.NullInt64(prog.UserID),
		Position:    prog.ProgressSeconds,
		Duration:    common.NullFloat64(prog.DurationSeconds),
		Watched:     common.NullInt64FromBool(prog.IsWatched),
		LastWatched: common.NullTime(prog.LastWatchedAt),
		CreatedAt:   common.NullTime(prog.CreatedAt),
		UpdatedAt:   common.NullTime(prog.UpdatedAt),
	})
	if err != nil {
		if common.IsUniqueConstraintError(err) {
			return progress.ErrProgressAlreadyExists
		}
		return err
	}

	prog.ID = result.ID
	return nil
}

// Update updates an existing watch progress record.
func (r *Repository) Update(ctx context.Context, prog *progress.WatchProgress) error {
	if err := prog.IsValid(); err != nil {
		return err
	}

	prog.UpdatedAt = time.Now()

	_, err := r.Q().UpdateWatchProgress(ctx, unified.UpdateWatchProgressParams{
		Position:    prog.ProgressSeconds,
		Duration:    common.NullFloat64(prog.DurationSeconds),
		Watched:     common.NullInt64FromBool(prog.IsWatched),
		LastWatched: common.NullTime(prog.LastWatchedAt),
		UpdatedAt:   common.NullTime(prog.UpdatedAt),
		ID:          prog.ID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return progress.ErrProgressNotFound
		}
		return err
	}

	return nil
}

// GetByMediaID retrieves watch progress for a specific media item.
func (r *Repository) GetByMediaID(ctx context.Context, mediaID int64) (*progress.WatchProgress, error) {
	result, err := r.Q().GetWatchProgressByMediaID(ctx, mediaID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, progress.ErrProgressNotFound
		}
		return nil, err
	}

	return rowToProgress(result), nil
}

// GetByMediaIDAndUserID retrieves watch progress for a specific media item and user.
func (r *Repository) GetByMediaIDAndUserID(ctx context.Context, mediaID, userID int64) (*progress.WatchProgress, error) {
	result, err := r.Q().GetWatchProgressByMediaIDAndUserID(ctx, unified.GetWatchProgressByMediaIDAndUserIDParams{
		MediaID: mediaID,
		UserID:  common.NullInt64(userID),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, progress.ErrProgressNotFound
		}
		return nil, err
	}

	return rowToProgress(result), nil
}

// GetBatchByMediaIDs retrieves watch progress for multiple media items.
func (r *Repository) GetBatchByMediaIDs(ctx context.Context, mediaIDs []int64, userID int64) (map[int64]*progress.WatchProgress, error) {
	result := make(map[int64]*progress.WatchProgress)

	if len(mediaIDs) == 0 {
		return result, nil
	}

	rows, err := r.Q().GetBatchWatchProgressByMediaIDs(ctx, unified.GetBatchWatchProgressByMediaIDsParams{
		MediaIds: mediaIDs,
		UserID:   common.NullInt64(userID),
	})
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		prog := rowToProgress(row)
		result[prog.MediaID] = prog
	}

	return result, nil
}

// ListByUserID retrieves all watch progress records for a user.
func (r *Repository) ListByUserID(ctx context.Context, userID int64, limit, offset int) ([]*progress.WatchProgress, error) {
	results, err := r.Q().ListWatchProgressByUserID(ctx, unified.ListWatchProgressByUserIDParams{
		UserID: common.NullInt64(userID),
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, err
	}

	return mapSlice(results, rowToProgress), nil
}

// ListWatchedByUserID retrieves all watched items for a user.
func (r *Repository) ListWatchedByUserID(ctx context.Context, userID int64, limit, offset int) ([]*progress.WatchProgress, error) {
	results, err := r.Q().ListWatchedByUserID(ctx, unified.ListWatchedByUserIDParams{
		UserID: common.NullInt64(userID),
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, err
	}

	return mapSlice(results, rowToProgress), nil
}

// ListInProgressByUserID retrieves all in-progress items for a user.
func (r *Repository) ListInProgressByUserID(ctx context.Context, userID int64, limit, offset int) ([]*progress.WatchProgress, error) {
	results, err := r.Q().ListInProgressByUserID(ctx, unified.ListInProgressByUserIDParams{
		UserID: common.NullInt64(userID),
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, err
	}

	return mapSlice(results, rowToProgress), nil
}

// Delete deletes a watch progress record.
func (r *Repository) Delete(ctx context.Context, id int64) error {
	err := r.Q().DeleteWatchProgress(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return progress.ErrProgressNotFound
		}
		return err
	}

	return nil
}

// DeleteByMediaID deletes watch progress for a specific media item.
func (r *Repository) DeleteByMediaID(ctx context.Context, mediaID int64) error {
	err := r.Q().DeleteWatchProgressByMediaID(ctx, mediaID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return progress.ErrProgressNotFound
		}
		return err
	}

	return nil
}

// Upsert creates or updates watch progress.
func (r *Repository) Upsert(ctx context.Context, prog *progress.WatchProgress) error {
	if err := prog.IsValid(); err != nil {
		return err
	}

	now := time.Now()
	if prog.CreatedAt.IsZero() {
		prog.CreatedAt = now
	}
	prog.UpdatedAt = now

	if prog.LastWatchedAt.IsZero() {
		prog.LastWatchedAt = now
	}

	result, err := r.Q().UpsertWatchProgress(ctx, unified.UpsertWatchProgressParams{
		MediaID:               prog.MediaID,
		UserID:                common.NullInt64(prog.UserID),
		Position:              prog.ProgressSeconds,
		Duration:              common.NullFloat64(prog.DurationSeconds),
		Watched:               common.NullInt64FromBool(prog.IsWatched),
		LastWatched:           common.NullTime(prog.LastWatchedAt),
		CreatedAt:             common.NullTime(prog.CreatedAt),
		UpdatedAt:             common.NullTime(prog.UpdatedAt),
		SelectedQuality:       common.NullStringPtr(prog.SelectedQuality),
		SelectedAudioTrack:    common.NullInt64FromIntPtr(prog.SelectedAudioTrack),
		SelectedSubtitleTrack: common.NullInt64FromIntPtr(prog.SelectedSubtitleTrack),
	})
	if err != nil {
		return err
	}

	prog.ID = result.ID
	return nil
}

// rowToProgress converts a database row to a domain entity.
func rowToProgress(row unified.WatchProgress) *progress.WatchProgress {
	return &progress.WatchProgress{
		ID:                    row.ID,
		UserID:                common.ParseNullInt64(row.UserID),
		MediaID:               row.MediaID,
		ProgressSeconds:       row.Position,
		DurationSeconds:       common.ParseNullFloat64(row.Duration),
		IsWatched:             common.NullInt64ToBool(row.Watched),
		LastWatchedAt:         common.ParseNullTime(row.LastWatched),
		CreatedAt:             common.ParseNullTime(row.CreatedAt),
		UpdatedAt:             common.ParseNullTime(row.UpdatedAt),
		SelectedQuality:       common.ParseNullStringPtr(row.SelectedQuality),
		SelectedAudioTrack:    common.ParseNullInt64ToIntPtr(row.SelectedAudioTrack),
		SelectedSubtitleTrack: common.ParseNullInt64ToIntPtr(row.SelectedSubtitleTrack),
	}
}

// mapSlice converts a slice of one type to another using the provided mapper function.
func mapSlice[TFrom, TTo any](from []TFrom, mapper func(TFrom) TTo) []TTo {
	result := make([]TTo, len(from))
	for i, v := range from {
		result[i] = mapper(v)
	}
	return result
}
