package enrichment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// NewQueueRepository creates a new enrichment queue repository with the appropriate database driver.
func NewQueueRepository(db *common.BaseRepository) *QueueRepository {
	return &QueueRepository{
		BaseRepository: db,
	}
}

// Enqueue adds or updates a job in the queue.
// Uses upsert behavior: re-enqueues completed/skipped/failed jobs.
func (r *QueueRepository) Enqueue(ctx context.Context, job *enrichment.QueueJob) (*enrichment.QueueJob, error) {
	row, err := r.Q().EnqueueEnrichmentJob(ctx, unified.EnqueueEnrichmentJobParams{
		MediaID:     job.MediaID,
		LibraryID:   sql.NullInt64{Int64: job.LibraryID, Valid: job.LibraryID > 0},
		MediaType:   string(job.MediaType),
		Stage:       job.Stage,
		Priority:    sql.NullInt64{Int64: int64(job.Priority), Valid: true},
		MaxAttempts: sql.NullInt64{Int64: int64(job.MaxAttempts), Valid: job.MaxAttempts > 0},
	})
	if err != nil {
		return nil, err
	}
	return convertQueueJob(row), nil
}

// ClaimBatch claims a batch of pending jobs for processing.
// Returns jobs locked to the specified worker.
func (r *QueueRepository) ClaimBatch(ctx context.Context, stage, workerID string, batchSize int) ([]*enrichment.QueueJob, error) {
	rows, err := r.Q().ClaimEnrichmentJobs(ctx, unified.ClaimEnrichmentJobsParams{
		LockedBy: sql.NullString{String: workerID, Valid: true},
		Stage:    stage,
		Limit:    int64(batchSize),
	})
	if err != nil {
		return nil, err
	}
	return mapSlice(rows, convertQueueJob), nil
}

// Complete marks a job as successfully completed.
func (r *QueueRepository) Complete(ctx context.Context, jobID int64) error {
	return r.Q().CompleteEnrichmentJob(ctx, jobID)
}

// Fail marks a job as failed with error details.
// Automatically schedules retry if attempts < maxAttempts.
func (r *QueueRepository) Fail(ctx context.Context, jobID int64, errMsg string, category enrichment.ErrorCategory, nextRetryAt *time.Time) error {
	var retryAt sql.NullTime
	if nextRetryAt != nil {
		retryAt = sql.NullTime{Time: *nextRetryAt, Valid: true}
	}
	return r.Q().FailEnrichmentJob(ctx, unified.FailEnrichmentJobParams{
		ErrorMessage:  sql.NullString{String: errMsg, Valid: errMsg != ""},
		ErrorCategory: sql.NullString{String: string(category), Valid: category != ""},
		NextRetryAt:   retryAt,
		ID:            jobID,
	})
}

// FailWithoutPenalty re-queues a job for retry without incrementing the attempt count.
// Used for transient infrastructure errors (plugin restarts, connection issues) that
// should not count against the retry limit.
func (r *QueueRepository) FailWithoutPenalty(ctx context.Context, jobID int64, errMsg string, category enrichment.ErrorCategory, nextRetryAt *time.Time) error {
	var retryAt sql.NullTime
	if nextRetryAt != nil {
		retryAt = sql.NullTime{Time: *nextRetryAt, Valid: true}
	}
	return r.Q().RequeueEnrichmentJob(ctx, unified.RequeueEnrichmentJobParams{
		ErrorMessage:  sql.NullString{String: errMsg, Valid: errMsg != ""},
		ErrorCategory: sql.NullString{String: string(category), Valid: category != ""},
		NextRetryAt:   retryAt,
		ID:            jobID,
	})
}

// Skip marks a job as skipped (no match found).
func (r *QueueRepository) Skip(ctx context.Context, jobID int64) error {
	return r.Q().SkipEnrichmentJob(ctx, jobID)
}

// GetStats returns queue statistics for a stage.
func (r *QueueRepository) GetStats(ctx context.Context, stage string) (*enrichment.QueueStats, error) {
	row, err := r.Q().GetEnrichmentQueueStats(ctx, stage)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &enrichment.QueueStats{Stage: stage}, nil
		}
		return nil, err
	}
	return convertQueueStats(row), nil
}

// ReleaseStuck releases jobs that have been locked too long.
func (r *QueueRepository) ReleaseStuck(ctx context.Context, maxLockSeconds int) error {
	// Use appropriate interval format based on database type
	var interval string
	if common.IsPostgres(r.DBType()) {
		interval = fmt.Sprintf("%d seconds", -maxLockSeconds)
	} else {
		interval = fmt.Sprintf("-%d", maxLockSeconds)
	}
	return r.Q().ReleaseStuckEnrichmentJobs(ctx, sql.NullString{String: interval, Valid: true})
}

// DeleteByMedia removes all queue jobs for a media item of a specific type.
func (r *QueueRepository) DeleteByMedia(ctx context.Context, mediaID int64, mediaType enrichment.MediaType) error {
	return r.Q().DeleteEnrichmentJobsByMedia(ctx, unified.DeleteEnrichmentJobsByMediaParams{
		MediaID:   mediaID,
		MediaType: string(mediaType),
	})
}

// GetByID retrieves a queue job by ID.
func (r *QueueRepository) GetByID(ctx context.Context, jobID int64) (*enrichment.QueueJob, error) {
	row, err := r.Q().GetEnrichmentJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	return convertQueueJob(row), nil
}

// GetByMediaAndStage retrieves a queue job by media ID, type, and stage.
func (r *QueueRepository) GetByMediaAndStage(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, stage string) (*enrichment.QueueJob, error) {
	row, err := r.Q().GetEnrichmentJobByMediaAndStage(ctx, unified.GetEnrichmentJobByMediaAndStageParams{
		MediaID:   mediaID,
		MediaType: string(mediaType),
		Stage:     stage,
	})
	if err != nil {
		return nil, err
	}
	return convertQueueJob(row), nil
}

// CurrentEnrichmentItem represents the currently processing enrichment item.
type CurrentEnrichmentItem struct {
	ID        int64
	MediaID   int64
	LibraryID int64
	MediaType enrichment.MediaType
	Stage     string
	Title     string
}

// GetCurrentItem returns the currently processing enrichment item for a library.
// Returns nil if no item is currently being processed.
func (r *QueueRepository) GetCurrentItem(ctx context.Context, libraryID int64) (*CurrentEnrichmentItem, error) {
	row, err := r.Q().GetCurrentEnrichmentItem(ctx, sql.NullInt64{Int64: libraryID, Valid: true})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &CurrentEnrichmentItem{
		ID:        row.ID,
		MediaID:   row.MediaID,
		LibraryID: row.LibraryID.Int64,
		MediaType: enrichment.MediaType(row.MediaType),
		Stage:     row.Stage,
		Title:     row.Title,
	}, nil
}

// UpdatePriorityByMedia updates the priority for all pending/processing jobs
// for a specific media item. Only upgrades priority, never downgrades - this
// preserves the "added recently" boost even after TMDB returns an old release date.
func (r *QueueRepository) UpdatePriorityByMedia(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, priority int) error {
	return r.Q().UpdatePriorityByMedia(ctx, unified.UpdatePriorityByMediaParams{
		Priority:  sql.NullInt64{Int64: int64(priority), Valid: true},
		MediaID:   mediaID,
		MediaType: string(mediaType),
	})
}

// BoostPriority updates the priority for all pending/processing jobs for a media item
// and returns true if any jobs were updated. This is used for interactive priority boost.
func (r *QueueRepository) BoostPriority(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, priority int) (bool, error) {
	rowsAffected, err := r.Q().BoostPriority(ctx, unified.BoostPriorityParams{
		Priority:  sql.NullInt64{Int64: int64(priority), Valid: true},
		MediaID:   mediaID,
		MediaType: string(mediaType),
	})
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

// EnqueueBatch adds multiple jobs to the queue in a single transaction.
// This is much faster than individual Enqueue calls for bulk operations.
// Jobs that fail to enqueue are skipped (logged), and the operation continues.
func (r *QueueRepository) EnqueueBatch(ctx context.Context, jobs []*enrichment.QueueJob) (int, error) {
	if len(jobs) == 0 {
		return 0, nil
	}

	tx, err := r.DB().BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	q := r.QWithTx(tx)
	var successCount int
	for _, job := range jobs {
		_, err := q.EnqueueEnrichmentJob(ctx, unified.EnqueueEnrichmentJobParams{
			MediaID:     job.MediaID,
			LibraryID:   sql.NullInt64{Int64: job.LibraryID, Valid: job.LibraryID > 0},
			MediaType:   string(job.MediaType),
			Stage:       job.Stage,
			Priority:    sql.NullInt64{Int64: int64(job.Priority), Valid: true},
			MaxAttempts: sql.NullInt64{Int64: int64(job.MaxAttempts), Valid: job.MaxAttempts > 0},
		})
		if err == nil {
			successCount++
		}
		// Skip failed inserts - don't abort the whole batch
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit transaction: %w", err)
	}

	return successCount, nil
}

// EnrichmentFailure represents a failed enrichment job with its title for display.
type EnrichmentFailure struct {
	ID            int64
	MediaID       int64
	LibraryID     int64
	MediaType     enrichment.MediaType
	Title         string
	Stage         string
	Attempts      int
	MaxAttempts   int
	ErrorMessage  string
	ErrorCategory enrichment.ErrorCategory
	LastAttemptAt time.Time
}

// GetLibraryFailures returns failed enrichment jobs for a library.
func (r *QueueRepository) GetLibraryFailures(ctx context.Context, libraryID int64, limit, offset int) ([]*EnrichmentFailure, error) {
	rows, err := r.Q().GetLibraryEnrichmentFailures(ctx, unified.GetLibraryEnrichmentFailuresParams{
		LibraryID: sql.NullInt64{Int64: libraryID, Valid: true},
		Limit:     int64(limit),
		Offset:    int64(offset),
	})
	if err != nil {
		return nil, err
	}

	failures := make([]*EnrichmentFailure, len(rows))
	for i, row := range rows {
		failures[i] = &EnrichmentFailure{
			ID:            row.ID,
			MediaID:       row.MediaID,
			LibraryID:     row.LibraryID.Int64,
			MediaType:     enrichment.MediaType(row.MediaType),
			Title:         row.Title,
			Stage:         row.Stage,
			Attempts:      int(row.Attempts.Int64),
			MaxAttempts:   int(row.MaxAttempts.Int64),
			ErrorMessage:  common.ParseNullString(row.ErrorMessage),
			ErrorCategory: enrichment.ErrorCategory(common.ParseNullString(row.ErrorCategory)),
			LastAttemptAt: common.ParseNullTime(row.LastAttemptAt),
		}
	}
	return failures, nil
}

// CountLibraryFailures returns the total count of failed enrichment jobs for a library.
func (r *QueueRepository) CountLibraryFailures(ctx context.Context, libraryID int64) (int64, error) {
	return r.Q().CountLibraryEnrichmentFailures(ctx, sql.NullInt64{Int64: libraryID, Valid: true})
}

// RetryLibraryFailures resets all failed jobs for a library to pending.
// Returns the number of jobs that were reset.
func (r *QueueRepository) RetryLibraryFailures(ctx context.Context, libraryID int64) (int64, error) {
	return r.Q().RetryEnrichmentJobsByLibrary(ctx, sql.NullInt64{Int64: libraryID, Valid: true})
}

// RetryJob resets a single failed job to pending.
func (r *QueueRepository) RetryJob(ctx context.Context, jobID int64) error {
	return r.Q().RetryEnrichmentJob(ctx, jobID)
}

// GetOrphanedPipelineStates finds media items where a stage completed but
// the next stage was never enqueued. This happens when the server crashes
// between marking a stage complete and enqueuing the next.
func (r *QueueRepository) GetOrphanedPipelineStates(ctx context.Context) ([]*enrichment.OrphanedPipelineState, error) {
	rows, err := r.Q().GetOrphanedPipelineStates(ctx)
	if err != nil {
		return nil, err
	}

	states := make([]*enrichment.OrphanedPipelineState, len(rows))
	for i, row := range rows {
		states[i] = &enrichment.OrphanedPipelineState{
			MediaType:      enrichment.MediaType(row.MediaType),
			MediaID:        row.MediaID,
			CompletedStage: row.CompletedStage,
			NextStage:      row.NextStage,
		}
	}
	return states, nil
}
