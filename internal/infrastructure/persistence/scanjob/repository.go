package scanjob

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// NewRepository creates a new scan job repository with the unified querier pattern.
func NewRepository(db *common.BaseRepository) *Repository {
	return &Repository{
		BaseRepository: db,
	}
}

// Create creates a new scan job
func (r *Repository) Create(ctx context.Context, job *scanner.ScanJob) error {
	// Encode target paths as JSON if present
	var targetPathsJSON sql.NullString
	if len(job.TargetPaths) > 0 {
		data, err := json.Marshal(job.TargetPaths)
		if err != nil {
			return fmt.Errorf("failed to marshal target paths: %w", err)
		}
		targetPathsJSON = sql.NullString{String: string(data), Valid: true}
	}

	result, err := r.Q().CreateScanJob(ctx, unified.CreateScanJobParams{
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
		DiscoveryDone:  common.NullInt64FromBool(job.DiscoveryDone),
		TargetPaths:    targetPathsJSON,
	})
	if err != nil {
		return err
	}

	job.ID = result.ID
	job.CreatedAt = common.ParseNullTime(result.CreatedAt)
	job.UpdatedAt = common.ParseNullTime(result.UpdatedAt)

	return nil
}

// GetByID retrieves a scan job by its ID
func (r *Repository) GetByID(ctx context.Context, id int64) (*scanner.ScanJob, error) {
	result, err := r.Q().GetScanJob(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, scanner.ErrNotFound
		}
		return nil, err
	}

	return convertToScanJob(result), nil
}

// GetLatestByLibrary retrieves the most recent scan job for a library
func (r *Repository) GetLatestByLibrary(ctx context.Context, libraryID int64) (*scanner.ScanJob, error) {
	result, err := r.Q().GetLatestScanJobByLibrary(ctx, libraryID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, scanner.ErrNotFound
		}
		return nil, err
	}

	return convertToScanJob(result), nil
}

// ListByLibrary retrieves scan jobs for a library, ordered by creation date (newest first)
func (r *Repository) ListByLibrary(ctx context.Context, libraryID int64, limit int32) ([]*scanner.ScanJob, error) {
	rows, err := r.Q().ListScanJobsByLibrary(ctx, unified.ListScanJobsByLibraryParams{
		LibraryID: libraryID,
		Limit:     int64(limit),
	})
	if err != nil {
		return nil, err
	}

	return mapSlice(rows, convertToScanJob), nil
}

// ListRunning retrieves all currently running scan jobs
func (r *Repository) ListRunning(ctx context.Context) ([]*scanner.ScanJob, error) {
	rows, err := r.Q().ListRunningScanJobs(ctx)
	if err != nil {
		return nil, err
	}

	return mapSlice(rows, convertToScanJob), nil
}

// UpdateProgress updates the progress counters for a scan job
func (r *Repository) UpdateProgress(ctx context.Context, id int64, progress *scanner.Progress) error {
	return r.Q().UpdateScanJobProgress(ctx, unified.UpdateScanJobProgressParams{
		Progress:       sql.NullFloat64{Float64: progress.GetPercentage(), Valid: true},
		FilesFound:     sql.NullInt64{Int64: progress.FilesFound, Valid: true},
		FilesProcessed: sql.NullInt64{Int64: progress.FilesProcessed, Valid: true},
		BytesProcessed: sql.NullInt64{Int64: progress.BytesProcessed, Valid: true},
		ErrorCount:     sql.NullInt64{Int64: progress.ErrorCount, Valid: true},
		WarningCount:   sql.NullInt64{Int64: progress.WarningCount, Valid: true},
		Phase:          common.NullString(string(progress.Phase)),
		EstimatedTotal: sql.NullInt64{Int64: progress.EstimatedTotal, Valid: true},
		DiscoveryDone:  common.NullInt64FromBool(progress.DiscoveryDone),
		ID:             id,
	})
}

// UpdateStatus updates the status of a scan job
func (r *Repository) UpdateStatus(ctx context.Context, id int64, status scanner.ScanStatus) error {
	return r.Q().UpdateScanJobStatus(ctx, unified.UpdateScanJobStatusParams{
		Status: string(status),
		ID:     id,
	})
}

// Complete marks a scan job as completed or failed
func (r *Repository) Complete(ctx context.Context, job *scanner.ScanJob) error {
	return r.Q().CompleteScanJob(ctx, unified.CompleteScanJobParams{
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
		DiscoveryDone:  common.NullInt64FromBool(job.DiscoveryDone),
		ID:             job.ID,
	})
}

// Delete deletes a scan job
func (r *Repository) Delete(ctx context.Context, id int64) error {
	return r.Q().DeleteScanJob(ctx, id)
}

// DeleteOld deletes old completed/failed scan jobs for a library
func (r *Repository) DeleteOld(ctx context.Context, libraryID int64, retentionMinutes int) error {
	// The unified querier uses SQLite modifier format: '-N minutes'
	// For PostgreSQL, the generated code handles the conversion internally
	modifier := fmt.Sprintf("-%d minutes", retentionMinutes)
	return r.Q().DeleteOldScanJobs(ctx, unified.DeleteOldScanJobsParams{
		LibraryID:     libraryID,
		RetentionDays: modifier,
	})
}

// convertToScanJob converts a unified ScanJob to domain ScanJob
func convertToScanJob(row unified.ScanJob) *scanner.ScanJob {
	// Decode target paths from JSON if present
	var targetPaths []string
	if row.TargetPaths.Valid && row.TargetPaths.String != "" {
		if err := json.Unmarshal([]byte(row.TargetPaths.String), &targetPaths); err != nil {
			// Log error but don't fail - just leave targetPaths empty
			// In production, we might want to log this with a proper logger
			targetPaths = nil
		}
	}

	return &scanner.ScanJob{
		ID:             row.ID,
		LibraryID:      row.LibraryID,
		Status:         scanner.ScanStatus(row.Status),
		Progress:       row.Progress.Float64,
		FilesFound:     row.FilesFound.Int64,
		FilesProcessed: row.FilesProcessed.Int64,
		BytesProcessed: row.BytesProcessed.Int64,
		ErrorCount:     row.ErrorCount.Int64,
		WarningCount:   row.WarningCount.Int64,
		StartedAt:      common.ParseNullTime(row.StartedAt),
		CompletedAt:    common.ParseNullTimePtr(row.CompletedAt),
		ErrorMessage:   common.ParseNullString(row.ErrorMessage),
		CreatedAt:      common.ParseNullTime(row.CreatedAt),
		UpdatedAt:      common.ParseNullTime(row.UpdatedAt),
		Phase:          scanner.ScanPhase(common.ParseNullString(row.Phase)),
		EstimatedTotal: row.EstimatedTotal.Int64,
		DiscoveryDone:  row.DiscoveryDone.Int64 != 0,
		TargetPaths:    targetPaths,
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
