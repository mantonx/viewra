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
	"github.com/mantonx/viewra/internal/infrastructure/persistence/adapters"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// NewQueueRepository creates a new enrichment queue repository with the appropriate database driver.
func NewQueueRepository(db *sql.DB, driver string) *QueueRepository {
	r := &QueueRepository{
		db:      db,
		dbType:  driver,
		adapter: adapters.NewTypeAdapter(),
		router:  common.NewQueryRouter(driver),
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
				MediaID:     int32(job.MediaID),
				LibraryID:   sql.NullInt32{Int32: int32(job.LibraryID), Valid: job.LibraryID > 0},
				MediaType:   string(job.MediaType),
				Stage:       job.Stage,
				Priority:    sql.NullInt32{Int32: int32(job.Priority), Valid: true},
				MaxAttempts: sql.NullInt32{Int32: int32(job.MaxAttempts), Valid: job.MaxAttempts > 0},
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
			return r.postgres.CompleteEnrichmentJob(ctx, int32(jobID))
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
				ID:            int32(jobID),
			})
		},
		func() error {
			var retryAtStr sql.NullString
			if nextRetryAt != nil {
				retryAtStr = sql.NullString{String: nextRetryAt.Format(time.RFC3339), Valid: true}
			}
			return r.sqlite.FailEnrichmentJob(ctx, sqlc_sqlite.FailEnrichmentJobParams{
				ErrorMessage:  sql.NullString{String: errMsg, Valid: errMsg != ""},
				ErrorCategory: sql.NullString{String: string(category), Valid: category != ""},
				NextRetryAt:   retryAtStr,
				ID:            jobID,
			})
		},
	)
}

// Skip marks a job as skipped (no match found).
func (r *QueueRepository) Skip(ctx context.Context, jobID int64) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.SkipEnrichmentJob(ctx, int32(jobID))
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
				MediaID:   int32(mediaID),
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
			return r.postgres.GetEnrichmentJob(ctx, int32(jobID))
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
				MediaID:   int32(mediaID),
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
			ID:            int64(pgJob.ID),
			MediaID:       int64(pgJob.MediaID),
			LibraryID:     int64(pgJob.LibraryID.Int32),
			MediaType:     enrichment.MediaType(pgJob.MediaType),
			Stage:         pgJob.Stage,
			Priority:      int(pgJob.Priority.Int32),
			Status:        enrichment.JobStatus(common.ParseNullString(pgJob.Status)),
			Attempts:      int(pgJob.Attempts.Int32),
			MaxAttempts:   int(pgJob.MaxAttempts.Int32),
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
		NextRetryAt:   parseNullTimeString(sqJob.NextRetryAt),
		LockedBy:      common.ParseNullString(sqJob.LockedBy),
		LockedAt:      parseNullTimeString(sqJob.LockedAt),
		CreatedAt:     parseTimeString(sqJob.CreatedAt),
		UpdatedAt:     parseTimeString(sqJob.UpdatedAt),
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
			return r.postgres.GetCurrentEnrichmentItem(ctx, sql.NullInt32{Int32: int32(libraryID), Valid: true})
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
			ID:        int64(row.ID),
			MediaID:   int64(row.MediaID),
			LibraryID: int64(row.LibraryID.Int32),
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
				MediaID:        int64(row.MediaID),
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
