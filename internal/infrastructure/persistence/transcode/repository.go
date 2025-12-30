package transcode

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/mantonx/viewra/internal/domain/transcode"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Repository implements the domain transcode.Repository interface.
type Repository struct {
	db              *sql.DB
	dbType          string // "sqlite" or "postgres"
	sqliteQuerier   sqlc_sqlite.Querier
	postgresQuerier sqlc_postgres.Querier
}

// NewRepository creates a new transcode job repository.
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

// Create creates a new transcode job.
func (r *Repository) Create(ctx context.Context, job *transcode.TranscodeJob) error {
	if err := job.IsValid(); err != nil {
		return err
	}

	now := time.Now()
	job.CreatedAt = now

	if r.dbType == "sqlite" {
		result, err := r.sqliteQuerier.CreateTranscodeJob(ctx, sqlc_sqlite.CreateTranscodeJobParams{
			MediaID:   job.MediaID,
			Quality:   job.Quality,
			Status:    job.Status,
			Progress:  common.NullInt64(int64(job.Progress)),
			CreatedAt: common.NullTime(job.CreatedAt),
		})
		if err != nil {
			if common.IsUniqueConstraintError(err) {
				return transcode.ErrJobAlreadyExists
			}
			return err
		}

		job.ID = result.ID
		return nil
	}

	// PostgreSQL
	result, err := r.postgresQuerier.CreateTranscodeJob(ctx, sqlc_postgres.CreateTranscodeJobParams{
		MediaID:   job.MediaID,
		Quality:   job.Quality,
		Type:      job.Type,
		Status:    job.Status,
		Progress:  common.NullInt64(int64(job.Progress)),
		CreatedAt: common.NullTime(job.CreatedAt),
	})
	if err != nil {
		if common.IsUniqueConstraintError(err) {
			return transcode.ErrJobAlreadyExists
		}
		return err
	}

	job.ID = int64(result.ID)
	return nil
}

// Update updates an existing transcode job.
func (r *Repository) Update(ctx context.Context, job *transcode.TranscodeJob) error {
	if err := job.IsValid(); err != nil {
		return err
	}

	if r.dbType == "sqlite" {
		err := r.sqliteQuerier.UpdateTranscodeJob(ctx, sqlc_sqlite.UpdateTranscodeJobParams{
			ID:            job.ID,
			Status:        job.Status,
			Progress:      common.NullInt64(int64(job.Progress)),
			Error:         common.NullString(job.Error),
			StartedAt:     common.NullTime(job.StartedAt),
			CompletedAt:   common.NullTime(job.CompletedAt),
			FilePath:      common.NullString(job.FilePath),
			FileSizeBytes: common.NullInt64(job.FileSizeBytes),
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return transcode.ErrJobNotFound
			}
			return err
		}
		return nil
	}

	// PostgreSQL
	err := r.postgresQuerier.UpdateTranscodeJob(ctx, sqlc_postgres.UpdateTranscodeJobParams{
		ID:            job.ID,
		Status:        job.Status,
		Progress:      common.NullInt64(int64(job.Progress)),
		Error:         common.NullString(job.Error),
		StartedAt:     common.NullTime(job.StartedAt),
		CompletedAt:   common.NullTime(job.CompletedAt),
		FilePath:      common.NullString(job.FilePath),
		FileSizeBytes: common.NullInt64(job.FileSizeBytes),
		StartPosition: common.NullFloat64(float64(job.StartPosition)),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return transcode.ErrJobNotFound
		}
		return err
	}
	return nil
}

// GetByID retrieves a transcode job by its ID.
func (r *Repository) GetByID(ctx context.Context, id int64) (*transcode.TranscodeJob, error) {
	if r.dbType == "sqlite" {
		result, err := r.sqliteQuerier.GetTranscodeJobByID(ctx, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, transcode.ErrJobNotFound
			}
			return nil, err
		}
		return r.sqliteModelToDomain(result), nil
	}

	// PostgreSQL
	result, err := r.postgresQuerier.GetTranscodeJobByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, transcode.ErrJobNotFound
		}
		return nil, err
	}
	return r.postgresModelToDomain(result), nil
}

// GetByMediaIDAndQuality retrieves a transcode job by media ID and quality.
func (r *Repository) GetByMediaIDAndQuality(ctx context.Context, mediaID int64, quality string) (*transcode.TranscodeJob, error) {
	if r.dbType == "sqlite" {
		result, err := r.sqliteQuerier.GetTranscodeJobByMediaIDAndQuality(ctx,
			sqlc_sqlite.GetTranscodeJobByMediaIDAndQualityParams{
				MediaID: mediaID,
				Quality: quality,
			})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, transcode.ErrJobNotFound
			}
			return nil, err
		}
		return r.sqliteModelToDomain(result), nil
	}

	// PostgreSQL
	result, err := r.postgresQuerier.GetTranscodeJobByMediaIDAndQuality(ctx,
		sqlc_postgres.GetTranscodeJobByMediaIDAndQualityParams{
			MediaID: mediaID,
			Quality: quality,
		})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, transcode.ErrJobNotFound
		}
		return nil, err
	}
	return r.postgresModelToDomain(result), nil
}

// ListByStatus retrieves all transcode jobs with a specific status.
func (r *Repository) ListByStatus(ctx context.Context, status string) ([]*transcode.TranscodeJob, error) {
	if r.dbType == "sqlite" {
		results, err := r.sqliteQuerier.ListTranscodeJobsByStatus(ctx, status)
		if err != nil {
			return nil, err
		}
		jobs := make([]*transcode.TranscodeJob, len(results))
		for i, result := range results {
			jobs[i] = r.sqliteModelToDomain(result)
		}
		return jobs, nil
	}

	// PostgreSQL
	results, err := r.postgresQuerier.ListTranscodeJobsByStatus(ctx, status)
	if err != nil {
		return nil, err
	}
	jobs := make([]*transcode.TranscodeJob, len(results))
	for i, result := range results {
		jobs[i] = r.postgresModelToDomain(result)
	}
	return jobs, nil
}

// ListQueuedJobs retrieves queued jobs up to a limit, ordered by creation time.
func (r *Repository) ListQueuedJobs(ctx context.Context, limit int) ([]*transcode.TranscodeJob, error) {
	if r.dbType == "sqlite" {
		results, err := r.sqliteQuerier.ListQueuedTranscodeJobs(ctx, int64(limit))
		if err != nil {
			return nil, err
		}
		jobs := make([]*transcode.TranscodeJob, len(results))
		for i, result := range results {
			jobs[i] = r.sqliteModelToDomain(result)
		}
		return jobs, nil
	}

	// PostgreSQL
	results, err := r.postgresQuerier.ListQueuedTranscodeJobs(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	jobs := make([]*transcode.TranscodeJob, len(results))
	for i, result := range results {
		jobs[i] = r.postgresModelToDomain(result)
	}
	return jobs, nil
}

// ListProcessingJobs retrieves all currently processing jobs.
func (r *Repository) ListProcessingJobs(ctx context.Context) ([]*transcode.TranscodeJob, error) {
	if r.dbType == "sqlite" {
		results, err := r.sqliteQuerier.ListProcessingTranscodeJobs(ctx)
		if err != nil {
			return nil, err
		}
		jobs := make([]*transcode.TranscodeJob, len(results))
		for i, result := range results {
			jobs[i] = r.sqliteModelToDomain(result)
		}
		return jobs, nil
	}

	// PostgreSQL
	results, err := r.postgresQuerier.ListProcessingTranscodeJobs(ctx)
	if err != nil {
		return nil, err
	}
	jobs := make([]*transcode.TranscodeJob, len(results))
	for i, result := range results {
		jobs[i] = r.postgresModelToDomain(result)
	}
	return jobs, nil
}

// CountByStatus counts jobs with a specific status.
func (r *Repository) CountByStatus(ctx context.Context, status string) (int64, error) {
	if r.dbType == "sqlite" {
		count, err := r.sqliteQuerier.CountTranscodeJobsByStatus(ctx, status)
		if err != nil {
			return 0, err
		}
		return count, nil
	}

	// PostgreSQL
	count, err := r.postgresQuerier.CountTranscodeJobsByStatus(ctx, status)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// Delete deletes a transcode job by ID.
func (r *Repository) Delete(ctx context.Context, id int64) error {
	if r.dbType == "sqlite" {
		err := r.sqliteQuerier.DeleteTranscodeJob(ctx, id)
		if err != nil {
			return err
		}
		return nil
	}

	// PostgreSQL
	err := r.postgresQuerier.DeleteTranscodeJob(ctx, id)
	if err != nil {
		return err
	}
	return nil
}

// DeleteByMediaID deletes all transcode jobs for a media item.
func (r *Repository) DeleteByMediaID(ctx context.Context, mediaID int64) error {
	if r.dbType == "sqlite" {
		err := r.sqliteQuerier.DeleteTranscodeJobsByMediaID(ctx, mediaID)
		if err != nil {
			return err
		}
		return nil
	}

	// PostgreSQL
	err := r.postgresQuerier.DeleteTranscodeJobsByMediaID(ctx, mediaID)
	if err != nil {
		return err
	}
	return nil
}

// ListAll retrieves all transcode jobs (for cleanup operations).
func (r *Repository) ListAll(ctx context.Context) ([]*transcode.TranscodeJob, error) {
	// SQLite only for now
	results, err := r.sqliteQuerier.ListAllTranscodeJobs(ctx)
	if err != nil {
		return nil, err
	}
	jobs := make([]*transcode.TranscodeJob, len(results))
	for i, result := range results {
		jobs[i] = r.sqliteModelToDomain(result)
	}
	return jobs, nil
}

// UpdateAccess updates the last accessed time and increments access count.
func (r *Repository) UpdateAccess(ctx context.Context, mediaID int64, quality string) error {
	now := time.Now()
	err := r.sqliteQuerier.UpdateTranscodeJobAccessByMediaAndQuality(ctx, sqlc_sqlite.UpdateTranscodeJobAccessByMediaAndQualityParams{
		LastAccessedAt: common.NullTime(now),
		MediaID:        mediaID,
		Quality:        quality,
	})
	return err
}

// GetTotalSize returns the total size of all completed transcodes.
func (r *Repository) GetTotalSize(ctx context.Context) (int64, error) {
	totalSize, err := r.sqliteQuerier.GetTotalTranscodeSize(ctx)
	if err != nil {
		return 0, err
	}
	// totalSize is an interface{}, need to convert to int64
	if size, ok := totalSize.(int64); ok {
		return size, nil
	}
	return 0, nil
}

// ListByLRU lists transcode jobs ordered by least recently used.
func (r *Repository) ListByLRU(ctx context.Context, limit int) ([]*transcode.TranscodeJob, error) {
	results, err := r.sqliteQuerier.ListTranscodeJobsByLRU(ctx, int64(limit))
	if err != nil {
		return nil, err
	}
	jobs := make([]*transcode.TranscodeJob, len(results))
	for i, result := range results {
		jobs[i] = r.sqliteModelToDomain(result)
	}
	return jobs, nil
}

// sqliteModelToDomain converts a SQLite model to a domain entity.
func (r *Repository) sqliteModelToDomain(model sqlc_sqlite.TranscodeJob) *transcode.TranscodeJob {
	return &transcode.TranscodeJob{
		ID:             model.ID,
		MediaID:        model.MediaID,
		Quality:        model.Quality,
		Type:           model.Type,
		Status:         model.Status,
		Progress:       int(common.ParseNullInt64(model.Progress)),
		Error:          common.ParseNullString(model.Error),
		StartedAt:      common.ParseNullTime(model.StartedAt),
		CompletedAt:    common.ParseNullTime(model.CompletedAt),
		CreatedAt:      common.ParseNullTime(model.CreatedAt),
		FilePath:       common.ParseNullString(model.FilePath),
		FileSizeBytes:  common.ParseNullInt64(model.FileSizeBytes),
		LastAccessedAt: common.ParseNullTime(model.LastAccessedAt),
		AccessCount:    int(common.ParseNullInt64(model.AccessCount)),
	}
}

// postgresModelToDomain converts a PostgreSQL model to a domain entity.
func (r *Repository) postgresModelToDomain(model sqlc_postgres.TranscodeJob) *transcode.TranscodeJob {
	return &transcode.TranscodeJob{
		ID:             model.ID,
		MediaID:        model.MediaID,
		Quality:        model.Quality,
		Type:           model.Type,
		Status:         model.Status,
		Progress:       int(common.ParseNullInt64(model.Progress)),
		Error:          common.ParseNullString(model.Error),
		StartedAt:      common.ParseNullTime(model.StartedAt),
		CompletedAt:    common.ParseNullTime(model.CompletedAt),
		CreatedAt:      common.ParseNullTime(model.CreatedAt),
		FilePath:       common.ParseNullString(model.FilePath),
		FileSizeBytes:  common.ParseNullInt64(model.FileSizeBytes),
		LastAccessedAt: common.ParseNullTime(model.LastAccessedAt),
		AccessCount:    int(common.ParseNullInt64(model.AccessCount)),
		StartPosition:  int(common.ParseNullFloat64(model.StartPosition)),
	}
}
