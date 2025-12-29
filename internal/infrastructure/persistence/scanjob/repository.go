package scanjob

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// NewRepository creates a new scan job repository with the appropriate database driver.
// The driver parameter should be "sqlite", "sqlite3", "postgres", or "postgresql".
func NewRepository(db *sql.DB, driver string) *Repository {
	r := &Repository{
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

// Create creates a new scan job
func (r *Repository) Create(ctx context.Context, job *scanner.ScanJob) error {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.CreateScanJob(ctx, sqlc_postgres.CreateScanJobParams{
				LibraryID:      int32(job.LibraryID),
				Status:         string(job.Status),
				Progress:       sql.NullFloat64{Float64: job.Progress, Valid: true},
				FilesFound:     sql.NullInt64{Int64: job.FilesFound, Valid: true},
				FilesProcessed: sql.NullInt64{Int64: job.FilesProcessed, Valid: true},
				BytesProcessed: sql.NullInt64{Int64: job.BytesProcessed, Valid: true},
				ErrorCount:     sql.NullInt64{Int64: job.ErrorCount, Valid: true},
				StartedAt:      common.NullTime(job.StartedAt),
				Phase:          common.NullString(string(job.Phase)),
				EstimatedTotal: sql.NullInt64{Int64: job.EstimatedTotal, Valid: true},
				DiscoveryDone:  sql.NullBool{Bool: job.DiscoveryDone, Valid: true},
			})
		},
		func() (any, error) {
			return r.sqlite.CreateScanJob(ctx, sqlc_sqlite.CreateScanJobParams{
				LibraryID:      job.LibraryID,
				Status:         string(job.Status),
				Progress:       sql.NullFloat64{Float64: job.Progress, Valid: true},
				FilesFound:     sql.NullInt64{Int64: job.FilesFound, Valid: true},
				FilesProcessed: sql.NullInt64{Int64: job.FilesProcessed, Valid: true},
				BytesProcessed: sql.NullInt64{Int64: job.BytesProcessed, Valid: true},
				ErrorCount:     sql.NullInt64{Int64: job.ErrorCount, Valid: true},
				StartedAt:      common.NullTime(job.StartedAt),
				Phase:          common.NullString(string(job.Phase)),
				EstimatedTotal: sql.NullInt64{Int64: job.EstimatedTotal, Valid: true},
				DiscoveryDone:  sql.NullBool{Bool: job.DiscoveryDone, Valid: true},
			})
		},
	)
	if err != nil {
		return err
	}

	// Convert result to update job ID
	if r.router.IsPostgresDB() {
		pgResult := result.(sqlc_postgres.ScanJob)
		job.ID = int64(pgResult.ID)
		job.CreatedAt = common.ParseNullTime(pgResult.CreatedAt)
		job.UpdatedAt = common.ParseNullTime(pgResult.UpdatedAt)
	} else {
		sqResult := result.(sqlc_sqlite.ScanJob)
		job.ID = sqResult.ID
		job.CreatedAt = common.ParseNullTime(sqResult.CreatedAt)
		job.UpdatedAt = common.ParseNullTime(sqResult.UpdatedAt)
	}

	return nil
}

// GetByID retrieves a scan job by its ID
func (r *Repository) GetByID(ctx context.Context, id int64) (*scanner.ScanJob, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetScanJob(ctx, int32(id))
		},
		func() (any, error) {
			return r.sqlite.GetScanJob(ctx, id)
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, scanner.ErrNotFound
		}
		return nil, err
	}

	return r.convertToScanJob(result), nil
}

// GetLatestByLibrary retrieves the most recent scan job for a library
func (r *Repository) GetLatestByLibrary(ctx context.Context, libraryID int64) (*scanner.ScanJob, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetLatestScanJobByLibrary(ctx, int32(libraryID))
		},
		func() (any, error) {
			return r.sqlite.GetLatestScanJobByLibrary(ctx, libraryID)
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, scanner.ErrNotFound
		}
		return nil, err
	}

	return r.convertToScanJob(result), nil
}

// ListByLibrary retrieves scan jobs for a library, ordered by creation date (newest first)
func (r *Repository) ListByLibrary(ctx context.Context, libraryID int64, limit int32) ([]*scanner.ScanJob, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.ListScanJobsByLibrary(ctx, sqlc_postgres.ListScanJobsByLibraryParams{
				LibraryID: int32(libraryID),
				Limit:     limit,
			})
		},
		func() (any, error) {
			return r.sqlite.ListScanJobsByLibrary(ctx, sqlc_sqlite.ListScanJobsByLibraryParams{
				LibraryID: libraryID,
				Limit:     int64(limit),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	if r.router.IsPostgresDB() {
		pgJobs := result.([]sqlc_postgres.ScanJob)
		jobs := make([]*scanner.ScanJob, len(pgJobs))
		for i, pgJob := range pgJobs {
			jobs[i] = r.convertToScanJob(pgJob)
		}
		return jobs, nil
	}

	sqJobs := result.([]sqlc_sqlite.ScanJob)
	jobs := make([]*scanner.ScanJob, len(sqJobs))
	for i, sqJob := range sqJobs {
		jobs[i] = r.convertToScanJob(sqJob)
	}
	return jobs, nil
}

// ListRunning retrieves all currently running scan jobs
func (r *Repository) ListRunning(ctx context.Context) ([]*scanner.ScanJob, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.ListRunningScanJobs(ctx)
		},
		func() (any, error) {
			return r.sqlite.ListRunningScanJobs(ctx)
		},
	)
	if err != nil {
		return nil, err
	}

	if r.router.IsPostgresDB() {
		pgJobs := result.([]sqlc_postgres.ScanJob)
		jobs := make([]*scanner.ScanJob, len(pgJobs))
		for i, pgJob := range pgJobs {
			jobs[i] = r.convertToScanJob(pgJob)
		}
		return jobs, nil
	}

	sqJobs := result.([]sqlc_sqlite.ScanJob)
	jobs := make([]*scanner.ScanJob, len(sqJobs))
	for i, sqJob := range sqJobs {
		jobs[i] = r.convertToScanJob(sqJob)
	}
	return jobs, nil
}

// UpdateProgress updates the progress counters for a scan job
func (r *Repository) UpdateProgress(ctx context.Context, id int64, progress *scanner.Progress) error {
	_, err := r.router.Route(
		func() (any, error) {
			return nil, r.postgres.UpdateScanJobProgress(ctx, sqlc_postgres.UpdateScanJobProgressParams{
				Progress:       sql.NullFloat64{Float64: progress.GetPercentage(), Valid: true},
				FilesFound:     sql.NullInt64{Int64: progress.FilesFound, Valid: true},
				FilesProcessed: sql.NullInt64{Int64: progress.FilesProcessed, Valid: true},
				BytesProcessed: sql.NullInt64{Int64: progress.BytesProcessed, Valid: true},
				ErrorCount:     sql.NullInt64{Int64: progress.ErrorCount, Valid: true},
				WarningCount:   sql.NullInt32{Int32: int32(progress.WarningCount), Valid: true},
				Phase:          common.NullString(string(progress.Phase)),
				EstimatedTotal: sql.NullInt64{Int64: progress.EstimatedTotal, Valid: true},
				DiscoveryDone:  sql.NullBool{Bool: progress.DiscoveryDone, Valid: true},
				ID:             int32(id),
			})
		},
		func() (any, error) {
			return nil, r.sqlite.UpdateScanJobProgress(ctx, sqlc_sqlite.UpdateScanJobProgressParams{
				Progress:       sql.NullFloat64{Float64: progress.GetPercentage(), Valid: true},
				FilesFound:     sql.NullInt64{Int64: progress.FilesFound, Valid: true},
				FilesProcessed: sql.NullInt64{Int64: progress.FilesProcessed, Valid: true},
				BytesProcessed: sql.NullInt64{Int64: progress.BytesProcessed, Valid: true},
				ErrorCount:     sql.NullInt64{Int64: progress.ErrorCount, Valid: true},
				WarningCount:   sql.NullInt64{Int64: progress.WarningCount, Valid: true},
				Phase:          common.NullString(string(progress.Phase)),
				EstimatedTotal: sql.NullInt64{Int64: progress.EstimatedTotal, Valid: true},
				DiscoveryDone:  sql.NullBool{Bool: progress.DiscoveryDone, Valid: true},
				ID:             id,
			})
		},
	)
	return err
}

// UpdateStatus updates the status of a scan job
func (r *Repository) UpdateStatus(ctx context.Context, id int64, status scanner.ScanStatus) error {
	_, err := r.router.Route(
		func() (any, error) {
			return nil, r.postgres.UpdateScanJobStatus(ctx, sqlc_postgres.UpdateScanJobStatusParams{
				Status: string(status),
				ID:     int32(id),
			})
		},
		func() (any, error) {
			return nil, r.sqlite.UpdateScanJobStatus(ctx, sqlc_sqlite.UpdateScanJobStatusParams{
				Status: string(status),
				ID:     id,
			})
		},
	)
	return err
}

// Complete marks a scan job as completed or failed
func (r *Repository) Complete(ctx context.Context, job *scanner.ScanJob) error {
	_, err := r.router.Route(
		func() (any, error) {
			return nil, r.postgres.CompleteScanJob(ctx, sqlc_postgres.CompleteScanJobParams{
				Status:         string(job.Status),
				Progress:       sql.NullFloat64{Float64: job.Progress, Valid: true},
				FilesFound:     sql.NullInt64{Int64: job.FilesFound, Valid: true},
				FilesProcessed: sql.NullInt64{Int64: job.FilesProcessed, Valid: true},
				BytesProcessed: sql.NullInt64{Int64: job.BytesProcessed, Valid: true},
				ErrorCount:     sql.NullInt64{Int64: job.ErrorCount, Valid: true},
				WarningCount:   sql.NullInt32{Int32: int32(job.WarningCount), Valid: true},
				CompletedAt:    common.NullTimePtr(job.CompletedAt),
				ErrorMessage:   common.NullString(job.ErrorMessage),
				Phase:          common.NullString(string(job.Phase)),
				DiscoveryDone:  sql.NullBool{Bool: job.DiscoveryDone, Valid: true},
				ID:             int32(job.ID),
			})
		},
		func() (any, error) {
			return nil, r.sqlite.CompleteScanJob(ctx, sqlc_sqlite.CompleteScanJobParams{
				Status:         string(job.Status),
				Progress:       sql.NullFloat64{Float64: job.Progress, Valid: true},
				FilesFound:     sql.NullInt64{Int64: job.FilesFound, Valid: true},
				FilesProcessed: sql.NullInt64{Int64: job.FilesProcessed, Valid: true},
				BytesProcessed: sql.NullInt64{Int64: job.BytesProcessed, Valid: true},
				ErrorCount:     sql.NullInt64{Int64: job.ErrorCount, Valid: true},
				WarningCount:   sql.NullInt64{Int64: job.WarningCount, Valid: true},
				CompletedAt:    common.NullTimePtr(job.CompletedAt),
				ErrorMessage:   common.NullString(job.ErrorMessage),
				Phase:          common.NullString(string(job.Phase)),
				DiscoveryDone:  sql.NullBool{Bool: job.DiscoveryDone, Valid: true},
				ID:             job.ID,
			})
		},
	)
	return err
}

// Delete deletes a scan job
func (r *Repository) Delete(ctx context.Context, id int64) error {
	_, err := r.router.Route(
		func() (any, error) {
			return nil, r.postgres.DeleteScanJob(ctx, int32(id))
		},
		func() (any, error) {
			return nil, r.sqlite.DeleteScanJob(ctx, id)
		},
	)
	return err
}

// DeleteOld deletes old completed/failed scan jobs for a library
func (r *Repository) DeleteOld(ctx context.Context, libraryID int64, retentionMinutes int) error {
	_, err := r.router.Route(
		func() (any, error) {
			// PostgreSQL: convert minutes to days for the interval
			// Using float division to support sub-day retention
			retentionDays := float64(retentionMinutes) / (24.0 * 60.0)
			return nil, r.postgres.DeleteOldScanJobs(ctx, sqlc_postgres.DeleteOldScanJobsParams{
				LibraryID:     int32(libraryID),
				RetentionDays: fmt.Sprintf("%.4f", retentionDays),
			})
		},
		func() (any, error) {
			// SQLite uses modifier syntax: '-30 minutes'
			modifier := fmt.Sprintf("-%d minutes", retentionMinutes)
			return nil, r.sqlite.DeleteOldScanJobs(ctx, sqlc_sqlite.DeleteOldScanJobsParams{
				LibraryID: libraryID,
				Datetime:  modifier,
			})
		},
	)
	return err
}

// convertToScanJob converts sqlc result to domain ScanJob
func (r *Repository) convertToScanJob(result any) *scanner.ScanJob {
	if r.router.IsPostgresDB() {
		pgJob := result.(sqlc_postgres.ScanJob)
		return &scanner.ScanJob{
			ID:             int64(pgJob.ID),
			LibraryID:      int64(pgJob.LibraryID),
			Status:         scanner.ScanStatus(pgJob.Status),
			Progress:       pgJob.Progress.Float64,
			FilesFound:     pgJob.FilesFound.Int64,
			FilesProcessed: pgJob.FilesProcessed.Int64,
			BytesProcessed: pgJob.BytesProcessed.Int64,
			ErrorCount:     pgJob.ErrorCount.Int64,
			WarningCount:   int64(pgJob.WarningCount.Int32),
			StartedAt:      common.ParseNullTime(pgJob.StartedAt),
			CompletedAt:    common.ParseNullTimePtr(pgJob.CompletedAt),
			ErrorMessage:   common.ParseNullString(pgJob.ErrorMessage),
			CreatedAt:      common.ParseNullTime(pgJob.CreatedAt),
			UpdatedAt:      common.ParseNullTime(pgJob.UpdatedAt),
			Phase:          scanner.ScanPhase(common.ParseNullString(pgJob.Phase)),
			EstimatedTotal: pgJob.EstimatedTotal.Int64,
			DiscoveryDone:  pgJob.DiscoveryDone.Bool,
		}
	}

	sqJob := result.(sqlc_sqlite.ScanJob)
	return &scanner.ScanJob{
		ID:             sqJob.ID,
		LibraryID:      sqJob.LibraryID,
		Status:         scanner.ScanStatus(sqJob.Status),
		Progress:       sqJob.Progress.Float64,
		FilesFound:     sqJob.FilesFound.Int64,
		FilesProcessed: sqJob.FilesProcessed.Int64,
		BytesProcessed: sqJob.BytesProcessed.Int64,
		ErrorCount:     sqJob.ErrorCount.Int64,
		WarningCount:   sqJob.WarningCount.Int64,
		StartedAt:      common.ParseNullTime(sqJob.StartedAt),
		CompletedAt:    common.ParseNullTimePtr(sqJob.CompletedAt),
		ErrorMessage:   common.ParseNullString(sqJob.ErrorMessage),
		CreatedAt:      common.ParseNullTime(sqJob.CreatedAt),
		UpdatedAt:      common.ParseNullTime(sqJob.UpdatedAt),
		Phase:          scanner.ScanPhase(common.ParseNullString(sqJob.Phase)),
		EstimatedTotal: sqJob.EstimatedTotal.Int64,
		DiscoveryDone:  sqJob.DiscoveryDone.Bool,
	}
}
