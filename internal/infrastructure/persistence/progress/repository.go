package progress

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/mantonx/viewra/internal/domain/progress"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Repository implements the domain progress.Repository interface.
type Repository struct {
	db              *sql.DB
	dbType          string // "sqlite" or "postgres"
	sqliteQuerier   sqlc_sqlite.Querier
	postgresQuerier sqlc_postgres.Querier
}

// NewRepository creates a new watch progress repository.
func NewRepository(db *sql.DB, dbType string) *Repository {
	r := &Repository{
		db:     db,
		dbType: dbType,
	}

	if common.IsPostgres(dbType) {
		r.postgresQuerier = sqlc_postgres.New(db)
	} else {
		r.sqliteQuerier = sqlc_sqlite.New(db)
	}

	return r
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

	if r.dbType == "sqlite" {
		result, err := r.sqliteQuerier.CreateWatchProgress(ctx, sqlc_sqlite.CreateWatchProgressParams{
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

	// PostgreSQL
	result, err := r.postgresQuerier.CreateWatchProgress(ctx, sqlc_postgres.CreateWatchProgressParams{
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

	if r.dbType == "sqlite" {
		_, err := r.sqliteQuerier.UpdateWatchProgress(ctx, sqlc_sqlite.UpdateWatchProgressParams{
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

	// PostgreSQL
	_, err := r.postgresQuerier.UpdateWatchProgress(ctx, sqlc_postgres.UpdateWatchProgressParams{
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
	if r.dbType == "sqlite" {
		result, err := r.sqliteQuerier.GetWatchProgressByMediaID(ctx, mediaID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, progress.ErrProgressNotFound
			}
			return nil, err
		}

		return sqliteRowToProgress(result), nil
	}

	// PostgreSQL
	result, err := r.postgresQuerier.GetWatchProgressByMediaID(ctx, mediaID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, progress.ErrProgressNotFound
		}
		return nil, err
	}

	return postgresRowToProgress(result), nil
}

// GetByMediaIDAndUserID retrieves watch progress for a specific media item and user.
func (r *Repository) GetByMediaIDAndUserID(ctx context.Context, mediaID, userID int64) (*progress.WatchProgress, error) {
	if r.dbType == "sqlite" {
		result, err := r.sqliteQuerier.GetWatchProgressByMediaIDAndUserID(ctx, sqlc_sqlite.GetWatchProgressByMediaIDAndUserIDParams{
			MediaID: mediaID,
			UserID:  common.NullInt64(userID),
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, progress.ErrProgressNotFound
			}
			return nil, err
		}

		return sqliteRowToProgress(result), nil
	}

	// PostgreSQL
	result, err := r.postgresQuerier.GetWatchProgressByMediaIDAndUserID(ctx, sqlc_postgres.GetWatchProgressByMediaIDAndUserIDParams{
		MediaID: mediaID,
		UserID:  common.NullInt64(userID),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, progress.ErrProgressNotFound
		}
		return nil, err
	}

	return postgresRowToProgress(result), nil
}

// GetBatchByMediaIDs retrieves watch progress for multiple media items.
func (r *Repository) GetBatchByMediaIDs(ctx context.Context, mediaIDs []int64, userID int64) (map[int64]*progress.WatchProgress, error) {
	result := make(map[int64]*progress.WatchProgress)

	if len(mediaIDs) == 0 {
		return result, nil
	}

	if r.dbType == "sqlite" {
		rows, err := r.sqliteQuerier.GetBatchWatchProgressByMediaIDs(ctx, sqlc_sqlite.GetBatchWatchProgressByMediaIDsParams{
			MediaIds: mediaIDs,
			UserID:   common.NullInt64(userID),
		})
		if err != nil {
			return nil, err
		}

		for _, row := range rows {
			prog := sqliteRowToProgress(row)
			result[prog.MediaID] = prog
		}
		return result, nil
	}

	// PostgreSQL
	rows, err := r.postgresQuerier.GetBatchWatchProgressByMediaIDs(ctx, sqlc_postgres.GetBatchWatchProgressByMediaIDsParams{
		Column1: mediaIDs,
		UserID:  common.NullInt64(userID),
	})
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		prog := postgresRowToProgress(row)
		result[prog.MediaID] = prog
	}
	return result, nil
}

// ListByUserID retrieves all watch progress records for a user.
func (r *Repository) ListByUserID(ctx context.Context, userID int64, limit, offset int) ([]*progress.WatchProgress, error) {
	if r.dbType == "sqlite" {
		results, err := r.sqliteQuerier.ListWatchProgressByUserID(ctx, sqlc_sqlite.ListWatchProgressByUserIDParams{
			UserID: common.NullInt64(userID),
			Limit:  int64(limit),
			Offset: int64(offset),
		})
		if err != nil {
			return nil, err
		}

		progresses := make([]*progress.WatchProgress, len(results))
		for i, result := range results {
			progresses[i] = sqliteRowToProgress(result)
		}
		return progresses, nil
	}

	// PostgreSQL
	results, err := r.postgresQuerier.ListWatchProgressByUserID(ctx, sqlc_postgres.ListWatchProgressByUserIDParams{
		UserID: common.NullInt64(userID),
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}

	progresses := make([]*progress.WatchProgress, len(results))
	for i, result := range results {
		progresses[i] = postgresRowToProgress(result)
	}
	return progresses, nil
}

// ListWatchedByUserID retrieves all watched items for a user.
func (r *Repository) ListWatchedByUserID(ctx context.Context, userID int64, limit, offset int) ([]*progress.WatchProgress, error) {
	if r.dbType == "sqlite" {
		results, err := r.sqliteQuerier.ListWatchedByUserID(ctx, sqlc_sqlite.ListWatchedByUserIDParams{
			UserID: common.NullInt64(userID),
			Limit:  int64(limit),
			Offset: int64(offset),
		})
		if err != nil {
			return nil, err
		}

		progresses := make([]*progress.WatchProgress, len(results))
		for i, result := range results {
			progresses[i] = sqliteRowToProgress(result)
		}
		return progresses, nil
	}

	// PostgreSQL
	results, err := r.postgresQuerier.ListWatchedByUserID(ctx, sqlc_postgres.ListWatchedByUserIDParams{
		UserID: common.NullInt64(userID),
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}

	progresses := make([]*progress.WatchProgress, len(results))
	for i, result := range results {
		progresses[i] = postgresRowToProgress(result)
	}
	return progresses, nil
}

// ListInProgressByUserID retrieves all in-progress items for a user.
func (r *Repository) ListInProgressByUserID(ctx context.Context, userID int64, limit, offset int) ([]*progress.WatchProgress, error) {
	if r.dbType == "sqlite" {
		results, err := r.sqliteQuerier.ListInProgressByUserID(ctx, sqlc_sqlite.ListInProgressByUserIDParams{
			UserID: common.NullInt64(userID),
			Limit:  int64(limit),
			Offset: int64(offset),
		})
		if err != nil {
			return nil, err
		}

		progresses := make([]*progress.WatchProgress, len(results))
		for i, result := range results {
			progresses[i] = sqliteRowToProgress(result)
		}
		return progresses, nil
	}

	// PostgreSQL
	results, err := r.postgresQuerier.ListInProgressByUserID(ctx, sqlc_postgres.ListInProgressByUserIDParams{
		UserID: common.NullInt64(userID),
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}

	progresses := make([]*progress.WatchProgress, len(results))
	for i, result := range results {
		progresses[i] = postgresRowToProgress(result)
	}
	return progresses, nil
}

// Delete deletes a watch progress record.
func (r *Repository) Delete(ctx context.Context, id int64) error {
	if r.dbType == "sqlite" {
		err := r.sqliteQuerier.DeleteWatchProgress(ctx, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return progress.ErrProgressNotFound
			}
			return err
		}
		return nil
	}

	// PostgreSQL
	err := r.postgresQuerier.DeleteWatchProgress(ctx, id)
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
	if r.dbType == "sqlite" {
		err := r.sqliteQuerier.DeleteWatchProgressByMediaID(ctx, mediaID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return progress.ErrProgressNotFound
			}
			return err
		}
		return nil
	}

	// PostgreSQL
	err := r.postgresQuerier.DeleteWatchProgressByMediaID(ctx, mediaID)
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

	if r.dbType == "sqlite" {
		result, err := r.sqliteQuerier.UpsertWatchProgress(ctx, sqlc_sqlite.UpsertWatchProgressParams{
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

	// PostgreSQL
	result, err := r.postgresQuerier.UpsertWatchProgress(ctx, sqlc_postgres.UpsertWatchProgressParams{
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

// Helper functions to convert database rows to domain entities

func sqliteRowToProgress(row sqlc_sqlite.WatchProgress) *progress.WatchProgress {
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

func postgresRowToProgress(row sqlc_postgres.WatchProgress) *progress.WatchProgress {
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
