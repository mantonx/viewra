package enrichment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// NewQueueRepository creates a new enrichment queue repository with the appropriate database driver.
func NewQueueRepository(db *sql.DB, driver string) *QueueRepository {
	r := &QueueRepository{
		db:     db,
		dbType: driver,
		router: common.NewQueryRouter(driver),
	}

	if common.IsPostgres(driver) {
		r.postgres = sqlc_postgres.New(db)
	} else {
		r.sqlite = sqlc_sqlite.New(db)
	}

	return r
}

// Enqueue adds or updates a job in the queue.
// Uses upsert behavior: re-enqueues completed/skipped/failed jobs.
func (r *QueueRepository) Enqueue(ctx context.Context, job *enrichment.QueueJob) (*enrichment.QueueJob, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.EnqueueEnrichmentJob(ctx, sqlc_postgres.EnqueueEnrichmentJobParams{
				MediaID:     job.MediaID,
				LibraryID:   sql.NullInt64{Int64: job.LibraryID, Valid: job.LibraryID > 0},
				MediaType:   string(job.MediaType),
				Stage:       job.Stage,
				Priority:    sql.NullInt64{Int64: int64(job.Priority), Valid: true},
				MaxAttempts: sql.NullInt64{Int64: int64(job.MaxAttempts), Valid: job.MaxAttempts > 0},
			})
		},
		func() (any, error) {
			return r.sqlite.EnqueueEnrichmentJob(ctx, sqlc_sqlite.EnqueueEnrichmentJobParams{
				MediaID:     job.MediaID,
				LibraryID:   sql.NullInt64{Int64: job.LibraryID, Valid: job.LibraryID > 0},
				MediaType:   string(job.MediaType),
				Stage:       job.Stage,
				Priority:    sql.NullInt64{Int64: int64(job.Priority), Valid: true},
				MaxAttempts: sql.NullInt64{Int64: int64(job.MaxAttempts), Valid: job.MaxAttempts > 0},
			})
		},
	)
	if err != nil {
		return nil, err
	}

	return r.convertToQueueJob(result), nil
}

// ClaimBatch claims a batch of pending jobs for processing.
// Returns jobs locked to the specified worker.
func (r *QueueRepository) ClaimBatch(ctx context.Context, stage, workerID string, batchSize int) ([]*enrichment.QueueJob, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.ClaimEnrichmentJobs(ctx, sqlc_postgres.ClaimEnrichmentJobsParams{
				LockedBy: sql.NullString{String: workerID, Valid: true},
				Stage:    stage,
				Limit:    int32(batchSize),
			})
		},
		func() (any, error) {
			return r.sqlite.ClaimEnrichmentJobs(ctx, sqlc_sqlite.ClaimEnrichmentJobsParams{
				LockedBy: sql.NullString{String: workerID, Valid: true},
				Stage:    stage,
				Limit:    int64(batchSize),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	if r.router.IsPostgresDB() {
		pgJobs := result.([]sqlc_postgres.EnrichmentQueue)
		jobs := make([]*enrichment.QueueJob, len(pgJobs))
		for i, pgJob := range pgJobs {
			jobs[i] = r.convertToQueueJob(pgJob)
		}
		return jobs, nil
	}

	sqJobs := result.([]sqlc_sqlite.EnrichmentQueue)
	jobs := make([]*enrichment.QueueJob, len(sqJobs))
	for i, sqJob := range sqJobs {
		jobs[i] = r.convertToQueueJob(sqJob)
	}
	return jobs, nil
}

// Complete marks a job as successfully completed.
func (r *QueueRepository) Complete(ctx context.Context, jobID int64) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.CompleteEnrichmentJob(ctx, jobID)
		},
		func() error {
			return r.sqlite.CompleteEnrichmentJob(ctx, jobID)
		},
	)
}

// Fail marks a job as failed with error details.
// Automatically schedules retry if attempts < maxAttempts.
func (r *QueueRepository) Fail(ctx context.Context, jobID int64, errMsg string, category enrichment.ErrorCategory, nextRetryAt *time.Time) error {
	return r.router.RouteVoid(
		func() error {
			var retryAt sql.NullTime
			if nextRetryAt != nil {
				retryAt = sql.NullTime{Time: *nextRetryAt, Valid: true}
			}
			return r.postgres.FailEnrichmentJob(ctx, sqlc_postgres.FailEnrichmentJobParams{
				ErrorMessage:  sql.NullString{String: errMsg, Valid: errMsg != ""},
				ErrorCategory: sql.NullString{String: string(category), Valid: category != ""},
				NextRetryAt:   retryAt,
				ID:            jobID,
			})
		},
		func() error {
			var retryAt sql.NullTime
			if nextRetryAt != nil {
				retryAt = sql.NullTime{Time: *nextRetryAt, Valid: true}
			}
			return r.sqlite.FailEnrichmentJob(ctx, sqlc_sqlite.FailEnrichmentJobParams{
				ErrorMessage:  sql.NullString{String: errMsg, Valid: errMsg != ""},
				ErrorCategory: sql.NullString{String: string(category), Valid: category != ""},
				NextRetryAt:   retryAt,
				ID:            jobID,
			})
		},
	)
}

// FailWithoutPenalty re-queues a job for retry without incrementing the attempt count.
// Used for transient infrastructure errors (plugin restarts, connection issues) that
// should not count against the retry limit.
func (r *QueueRepository) FailWithoutPenalty(ctx context.Context, jobID int64, errMsg string, category enrichment.ErrorCategory, nextRetryAt *time.Time) error {
	return r.router.RouteVoid(
		func() error {
			var retryAt sql.NullTime
			if nextRetryAt != nil {
				retryAt = sql.NullTime{Time: *nextRetryAt, Valid: true}
			}
			return r.postgres.RequeueEnrichmentJob(ctx, sqlc_postgres.RequeueEnrichmentJobParams{
				ErrorMessage:  sql.NullString{String: errMsg, Valid: errMsg != ""},
				ErrorCategory: sql.NullString{String: string(category), Valid: category != ""},
				NextRetryAt:   retryAt,
				ID:            jobID,
			})
		},
		func() error {
			var retryAt sql.NullTime
			if nextRetryAt != nil {
				retryAt = sql.NullTime{Time: *nextRetryAt, Valid: true}
			}
			return r.sqlite.RequeueEnrichmentJob(ctx, sqlc_sqlite.RequeueEnrichmentJobParams{
				ErrorMessage:  sql.NullString{String: errMsg, Valid: errMsg != ""},
				ErrorCategory: sql.NullString{String: string(category), Valid: category != ""},
				NextRetryAt:   retryAt,
				ID:            jobID,
			})
		},
	)
}

// Skip marks a job as skipped (no match found).
func (r *QueueRepository) Skip(ctx context.Context, jobID int64) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.SkipEnrichmentJob(ctx, jobID)
		},
		func() error {
			return r.sqlite.SkipEnrichmentJob(ctx, jobID)
		},
	)
}

// GetStats returns queue statistics for a stage.
func (r *QueueRepository) GetStats(ctx context.Context, stage string) (*enrichment.QueueStats, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetEnrichmentQueueStats(ctx, stage)
		},
		func() (any, error) {
			return r.sqlite.GetEnrichmentQueueStats(ctx, stage)
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &enrichment.QueueStats{Stage: stage}, nil
		}
		return nil, err
	}

	if r.router.IsPostgresDB() {
		pgStats := result.(sqlc_postgres.GetEnrichmentQueueStatsRow)
		return &enrichment.QueueStats{
			Stage:           pgStats.Stage,
			PendingCount:    pgStats.PendingCount,
			ProcessingCount: pgStats.ProcessingCount,
			CompletedCount:  pgStats.CompletedCount,
			FailedCount:     pgStats.FailedCount,
			SkippedCount:    pgStats.SkippedCount,
			TotalCount:      pgStats.TotalCount,
		}, nil
	}

	sqStats := result.(sqlc_sqlite.GetEnrichmentQueueStatsRow)
	return &enrichment.QueueStats{
		Stage:           sqStats.Stage,
		PendingCount:    int64(sqStats.PendingCount.Float64),
		ProcessingCount: int64(sqStats.ProcessingCount.Float64),
		CompletedCount:  int64(sqStats.CompletedCount.Float64),
		FailedCount:     int64(sqStats.FailedCount.Float64),
		SkippedCount:    int64(sqStats.SkippedCount.Float64),
		TotalCount:      sqStats.TotalCount,
	}, nil
}

// ReleaseStuck releases jobs that have been locked too long.
func (r *QueueRepository) ReleaseStuck(ctx context.Context, maxLockSeconds int) error {
	return r.router.RouteVoid(
		func() error {
			interval := fmt.Sprintf("%d seconds", -maxLockSeconds)
			return r.postgres.ReleaseStuckEnrichmentJobs(ctx, sql.NullString{String: interval, Valid: true})
		},
		func() error {
			modifier := fmt.Sprintf("-%d", maxLockSeconds)
			return r.sqlite.ReleaseStuckEnrichmentJobs(ctx, sql.NullString{String: modifier, Valid: true})
		},
	)
}

// DeleteByMedia removes all queue jobs for a media item of a specific type.
func (r *QueueRepository) DeleteByMedia(ctx context.Context, mediaID int64, mediaType enrichment.MediaType) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.DeleteEnrichmentJobsByMedia(ctx, sqlc_postgres.DeleteEnrichmentJobsByMediaParams{
				MediaID:   mediaID,
				MediaType: string(mediaType),
			})
		},
		func() error {
			return r.sqlite.DeleteEnrichmentJobsByMedia(ctx, sqlc_sqlite.DeleteEnrichmentJobsByMediaParams{
				MediaID:   mediaID,
				MediaType: string(mediaType),
			})
		},
	)
}

// GetByID retrieves a queue job by ID.
func (r *QueueRepository) GetByID(ctx context.Context, jobID int64) (*enrichment.QueueJob, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetEnrichmentJob(ctx, jobID)
		},
		func() (any, error) {
			return r.sqlite.GetEnrichmentJob(ctx, jobID)
		},
	)
	if err != nil {
		return nil, err
	}

	return r.convertToQueueJob(result), nil
}

// GetByMediaAndStage retrieves a queue job by media ID, type, and stage.
func (r *QueueRepository) GetByMediaAndStage(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, stage string) (*enrichment.QueueJob, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetEnrichmentJobByMediaAndStage(ctx, sqlc_postgres.GetEnrichmentJobByMediaAndStageParams{
				MediaID:   mediaID,
				MediaType: string(mediaType),
				Stage:     stage,
			})
		},
		func() (any, error) {
			return r.sqlite.GetEnrichmentJobByMediaAndStage(ctx, sqlc_sqlite.GetEnrichmentJobByMediaAndStageParams{
				MediaID:   mediaID,
				MediaType: string(mediaType),
				Stage:     stage,
			})
		},
	)
	if err != nil {
		return nil, err
	}

	return r.convertToQueueJob(result), nil
}

// convertToQueueJob converts sqlc result to domain QueueJob.
func (r *QueueRepository) convertToQueueJob(result any) *enrichment.QueueJob {
	if r.router.IsPostgresDB() {
		pgJob := result.(sqlc_postgres.EnrichmentQueue)
		return &enrichment.QueueJob{
			ID:            pgJob.ID,
			MediaID:       pgJob.MediaID,
			LibraryID:     pgJob.LibraryID.Int64,
			MediaType:     enrichment.MediaType(pgJob.MediaType),
			Stage:         pgJob.Stage,
			Priority:      int(pgJob.Priority.Int64),
			Status:        enrichment.JobStatus(common.ParseNullString(pgJob.Status)),
			Attempts:      int(pgJob.Attempts.Int64),
			MaxAttempts:   int(pgJob.MaxAttempts.Int64),
			ErrorMessage:  common.ParseNullString(pgJob.ErrorMessage),
			ErrorCategory: enrichment.ErrorCategory(common.ParseNullString(pgJob.ErrorCategory)),
			NextRetryAt:   common.ParseNullTimePtr(pgJob.NextRetryAt),
			LockedBy:      common.ParseNullString(pgJob.LockedBy),
			LockedAt:      common.ParseNullTimePtr(pgJob.LockedAt),
			CreatedAt:     common.ParseNullTime(pgJob.CreatedAt),
			UpdatedAt:     common.ParseNullTime(pgJob.UpdatedAt),
		}
	}

	sqJob := result.(sqlc_sqlite.EnrichmentQueue)
	return &enrichment.QueueJob{
		ID:            sqJob.ID,
		MediaID:       sqJob.MediaID,
		LibraryID:     sqJob.LibraryID.Int64,
		MediaType:     enrichment.MediaType(sqJob.MediaType),
		Stage:         sqJob.Stage,
		Priority:      int(sqJob.Priority.Int64),
		Status:        enrichment.JobStatus(common.ParseNullString(sqJob.Status)),
		Attempts:      int(sqJob.Attempts.Int64),
		MaxAttempts:   int(sqJob.MaxAttempts.Int64),
		ErrorMessage:  common.ParseNullString(sqJob.ErrorMessage),
		ErrorCategory: enrichment.ErrorCategory(common.ParseNullString(sqJob.ErrorCategory)),
		NextRetryAt:   common.ParseNullTimePtr(sqJob.NextRetryAt),
		LockedBy:      common.ParseNullString(sqJob.LockedBy),
		LockedAt:      common.ParseNullTimePtr(sqJob.LockedAt),
		CreatedAt:     common.ParseNullTime(sqJob.CreatedAt),
		UpdatedAt:     common.ParseNullTime(sqJob.UpdatedAt),
	}
}

// parseNullTimeString parses a nullable time string (SQLite stores times as strings).
func parseNullTimeString(s sql.NullString) *time.Time {
	if !s.Valid || s.String == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s.String)
	if err != nil {
		// Try other common formats
		t, err = time.Parse("2006-01-02 15:04:05", s.String)
		if err != nil {
			return nil
		}
	}
	return &t
}

// parseTimeString parses a time string.
func parseTimeString(s sql.NullString) time.Time {
	if !s.Valid || s.String == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s.String)
	if err != nil {
		t, err = time.Parse("2006-01-02 15:04:05", s.String)
		if err != nil {
			return time.Time{}
		}
	}
	return t
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
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetCurrentEnrichmentItem(ctx, sql.NullInt64{Int64: libraryID, Valid: true})
		},
		func() (any, error) {
			return r.sqlite.GetCurrentEnrichmentItem(ctx, sql.NullInt64{Int64: libraryID, Valid: true})
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if r.router.IsPostgresDB() {
		row := result.(sqlc_postgres.GetCurrentEnrichmentItemRow)
		return &CurrentEnrichmentItem{
			ID:        row.ID,
			MediaID:   row.MediaID,
			LibraryID: row.LibraryID.Int64,
			MediaType: enrichment.MediaType(row.MediaType),
			Stage:     row.Stage,
			Title:     row.Title,
		}, nil
	}

	row := result.(sqlc_sqlite.GetCurrentEnrichmentItemRow)
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
	return r.router.RouteVoid(
		func() error {
			// Postgres reuses $1 for both priority comparisons
			return r.postgres.UpdatePriorityByMedia(ctx, sqlc_postgres.UpdatePriorityByMediaParams{
				Priority:  sql.NullInt64{Int64: int64(priority), Valid: true},
				MediaID:   mediaID,
				MediaType: string(mediaType),
			})
		},
		func() error {
			// SQLite uses positional params, so we need to pass priority twice
			return r.sqlite.UpdatePriorityByMedia(ctx, sqlc_sqlite.UpdatePriorityByMediaParams{
				Priority:   sql.NullInt64{Int64: int64(priority), Valid: true},
				MediaID:    mediaID,
				MediaType:  string(mediaType),
				Priority_2: sql.NullInt64{Int64: int64(priority), Valid: true},
			})
		},
	)
}

// BoostPriority updates the priority for all pending/processing jobs for a media item
// and returns true if any jobs were updated. This is used for interactive priority boost.
func (r *QueueRepository) BoostPriority(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, priority int) (bool, error) {
	var rowsAffected int64
	var err error

	if r.router.IsPostgresDB() {
		rowsAffected, err = r.postgres.BoostPriority(ctx, sqlc_postgres.BoostPriorityParams{
			Priority:  sql.NullInt64{Int64: int64(priority), Valid: true},
			MediaID:   mediaID,
			MediaType: string(mediaType),
		})
	} else {
		rowsAffected, err = r.sqlite.BoostPriority(ctx, sqlc_sqlite.BoostPriorityParams{
			Priority:  sql.NullInt64{Int64: int64(priority), Valid: true},
			MediaID:   mediaID,
			MediaType: string(mediaType),
		})
	}

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

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	var successCount int

	if r.router.IsPostgresDB() {
		q := r.postgres.WithTx(tx)
		for _, job := range jobs {
			_, err := q.EnqueueEnrichmentJob(ctx, sqlc_postgres.EnqueueEnrichmentJobParams{
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
	} else {
		q := r.sqlite.WithTx(tx)
		for _, job := range jobs {
			_, err := q.EnqueueEnrichmentJob(ctx, sqlc_sqlite.EnqueueEnrichmentJobParams{
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
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetLibraryEnrichmentFailures(ctx, sqlc_postgres.GetLibraryEnrichmentFailuresParams{
				LibraryID: sql.NullInt64{Int64: libraryID, Valid: true},
				Limit:     int32(limit),
				Offset:    int32(offset),
			})
		},
		func() (any, error) {
			return r.sqlite.GetLibraryEnrichmentFailures(ctx, sqlc_sqlite.GetLibraryEnrichmentFailuresParams{
				LibraryID: sql.NullInt64{Int64: libraryID, Valid: true},
				Limit:     int64(limit),
				Offset:    int64(offset),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	if r.router.IsPostgresDB() {
		pgRows := result.([]sqlc_postgres.GetLibraryEnrichmentFailuresRow)
		failures := make([]*EnrichmentFailure, len(pgRows))
		for i, row := range pgRows {
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

	sqRows := result.([]sqlc_sqlite.GetLibraryEnrichmentFailuresRow)
	failures := make([]*EnrichmentFailure, len(sqRows))
	for i, row := range sqRows {
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
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.CountLibraryEnrichmentFailures(ctx, sql.NullInt64{Int64: libraryID, Valid: true})
		},
		func() (any, error) {
			return r.sqlite.CountLibraryEnrichmentFailures(ctx, sql.NullInt64{Int64: libraryID, Valid: true})
		},
	)
	if err != nil {
		return 0, err
	}

	if r.router.IsPostgresDB() {
		return result.(int64), nil
	}
	return result.(int64), nil
}

// RetryLibraryFailures resets all failed jobs for a library to pending.
// Returns the number of jobs that were reset.
func (r *QueueRepository) RetryLibraryFailures(ctx context.Context, libraryID int64) (int64, error) {
	if r.router.IsPostgresDB() {
		return r.postgres.RetryEnrichmentJobsByLibrary(ctx, sql.NullInt64{Int64: libraryID, Valid: true})
	}
	return r.sqlite.RetryEnrichmentJobsByLibrary(ctx, sql.NullInt64{Int64: libraryID, Valid: true})
}

// RetryJob resets a single failed job to pending.
func (r *QueueRepository) RetryJob(ctx context.Context, jobID int64) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.RetryEnrichmentJob(ctx, jobID)
		},
		func() error {
			return r.sqlite.RetryEnrichmentJob(ctx, jobID)
		},
	)
}

// GetOrphanedPipelineStates finds media items where a stage completed but
// the next stage was never enqueued. This happens when the server crashes
// between marking a stage complete and enqueuing the next.
func (r *QueueRepository) GetOrphanedPipelineStates(ctx context.Context) ([]*enrichment.OrphanedPipelineState, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetOrphanedPipelineStates(ctx)
		},
		func() (any, error) {
			return r.sqlite.GetOrphanedPipelineStates(ctx)
		},
	)
	if err != nil {
		return nil, err
	}

	if r.router.IsPostgresDB() {
		pgRows := result.([]sqlc_postgres.GetOrphanedPipelineStatesRow)
		states := make([]*enrichment.OrphanedPipelineState, len(pgRows))
		for i, row := range pgRows {
			states[i] = &enrichment.OrphanedPipelineState{
				MediaType:      enrichment.MediaType(row.MediaType),
				MediaID:        row.MediaID,
				CompletedStage: row.CompletedStage,
				NextStage:      row.NextStage,
			}
		}
		return states, nil
	}

	sqRows := result.([]sqlc_sqlite.GetOrphanedPipelineStatesRow)
	states := make([]*enrichment.OrphanedPipelineState, len(sqRows))
	for i, row := range sqRows {
		states[i] = &enrichment.OrphanedPipelineState{
			MediaType:      enrichment.MediaType(row.MediaType),
			MediaID:        row.MediaID,
			CompletedStage: row.CompletedStage,
			NextStage:      row.NextStage,
		}
	}
	return states, nil
}
