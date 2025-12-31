package transcode

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/mantonx/viewra/internal/domain/transcode"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Repository implements the domain transcode.Repository interface.
type Repository struct {
	*common.BaseRepository
}

// NewRepository creates a new transcode job repository.
func NewRepository(db *common.BaseRepository) *Repository {
	return &Repository{
		BaseRepository: db,
	}
}

// Create creates a new transcode job.
func (r *Repository) Create(ctx context.Context, job *transcode.TranscodeJob) error {
	if err := job.IsValid(); err != nil {
		return err
	}

	now := time.Now()
	job.CreatedAt = now

	result, err := r.Q().CreateTranscodeJob(ctx, unified.CreateTranscodeJobParams{
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

	job.ID = result.ID
	return nil
}

// Update updates an existing transcode job.
func (r *Repository) Update(ctx context.Context, job *transcode.TranscodeJob) error {
	if err := job.IsValid(); err != nil {
		return err
	}

	err := r.Q().UpdateTranscodeJob(ctx, unified.UpdateTranscodeJobParams{
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
	result, err := r.Q().GetTranscodeJobByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, transcode.ErrJobNotFound
		}
		return nil, err
	}
	return modelToDomain(result), nil
}

// GetByMediaIDAndQuality retrieves a transcode job by media ID and quality.
func (r *Repository) GetByMediaIDAndQuality(ctx context.Context, mediaID int64, quality string) (*transcode.TranscodeJob, error) {
	result, err := r.Q().GetTranscodeJobByMediaIDAndQuality(ctx, unified.GetTranscodeJobByMediaIDAndQualityParams{
		MediaID: mediaID,
		Quality: quality,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, transcode.ErrJobNotFound
		}
		return nil, err
	}
	return modelToDomain(result), nil
}

// ListByStatus retrieves all transcode jobs with a specific status.
func (r *Repository) ListByStatus(ctx context.Context, status string) ([]*transcode.TranscodeJob, error) {
	results, err := r.Q().ListTranscodeJobsByStatus(ctx, status)
	if err != nil {
		return nil, err
	}
	return mapSlice(results, modelToDomain), nil
}

// ListQueuedJobs retrieves queued jobs up to a limit, ordered by creation time.
func (r *Repository) ListQueuedJobs(ctx context.Context, limit int) ([]*transcode.TranscodeJob, error) {
	results, err := r.Q().ListQueuedTranscodeJobs(ctx, int64(limit))
	if err != nil {
		return nil, err
	}
	return mapSlice(results, modelToDomain), nil
}

// ListProcessingJobs retrieves all currently processing jobs.
func (r *Repository) ListProcessingJobs(ctx context.Context) ([]*transcode.TranscodeJob, error) {
	results, err := r.Q().ListProcessingTranscodeJobs(ctx)
	if err != nil {
		return nil, err
	}
	return mapSlice(results, modelToDomain), nil
}

// CountByStatus counts jobs with a specific status.
func (r *Repository) CountByStatus(ctx context.Context, status string) (int64, error) {
	return r.Q().CountTranscodeJobsByStatus(ctx, status)
}

// Delete deletes a transcode job by ID.
func (r *Repository) Delete(ctx context.Context, id int64) error {
	return r.Q().DeleteTranscodeJob(ctx, id)
}

// DeleteByMediaID deletes all transcode jobs for a media item.
func (r *Repository) DeleteByMediaID(ctx context.Context, mediaID int64) error {
	return r.Q().DeleteTranscodeJobsByMediaID(ctx, mediaID)
}

// ListAll retrieves all transcode jobs (for cleanup operations).
func (r *Repository) ListAll(ctx context.Context) ([]*transcode.TranscodeJob, error) {
	results, err := r.Q().ListAllTranscodeJobs(ctx)
	if err != nil {
		return nil, err
	}
	return mapSlice(results, modelToDomain), nil
}

// UpdateAccess updates the last accessed time and increments access count.
func (r *Repository) UpdateAccess(ctx context.Context, mediaID int64, quality string) error {
	now := time.Now()
	return r.Q().UpdateTranscodeJobAccessByMediaAndQuality(ctx, unified.UpdateTranscodeJobAccessByMediaAndQualityParams{
		LastAccessedAt: common.NullTime(now),
		MediaID:        mediaID,
		Quality:        quality,
	})
}

// GetTotalSize returns the total size of all completed transcodes.
func (r *Repository) GetTotalSize(ctx context.Context) (int64, error) {
	totalSize, err := r.Q().GetTotalTranscodeSize(ctx)
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
	results, err := r.Q().ListTranscodeJobsByLRU(ctx, int64(limit))
	if err != nil {
		return nil, err
	}
	return mapSlice(results, modelToDomain), nil
}

// modelToDomain converts a unified model to a domain entity.
func modelToDomain(model unified.TranscodeJob) *transcode.TranscodeJob {
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

// mapSlice converts a slice of one type to another using the provided mapper function.
func mapSlice[TFrom, TTo any](from []TFrom, mapper func(TFrom) TTo) []TTo {
	result := make([]TTo, len(from))
	for i, v := range from {
		result[i] = mapper(v)
	}
	return result
}
