package scanjob

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// CheckpointRepo implements scanner.CheckpointRepository using sqlc.
// It embeds BaseRepository for dual-database support.
type CheckpointRepo struct {
	*common.BaseRepository
}

// NewCheckpointRepo creates a new checkpoint repository with the unified querier pattern.
func NewCheckpointRepo(db *common.BaseRepository) *CheckpointRepo {
	return &CheckpointRepo{
		BaseRepository: db,
	}
}

// CreateBatch creates multiple checkpoints in a single multi-row INSERT operation.
// This is significantly faster than individual INSERTs, especially for network storage
// where reducing round-trips is critical (e.g., 100 files: 1 round-trip vs 100 round-trips).
//
// Performance: ~10-50x faster for large batches depending on batch size and network latency.
func (r *CheckpointRepo) CreateBatch(ctx context.Context, checkpoints []*scanner.ScanCheckpoint) error {
	if len(checkpoints) == 0 {
		return nil
	}

	// Use a transaction for atomic batch insert
	tx, err := r.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Build multi-row INSERT statement
	// SQLite: INSERT INTO ... VALUES (?, ?, ...), (?, ?, ...), ...
	// Postgres: INSERT INTO ... VALUES ($1, $2, ...), ($3, $4, ...), ...

	const baseQuery = `INSERT INTO scan_checkpoints (scan_job_id, file_path, status, file_size, file_hash, created_at) VALUES `

	// Build placeholders for multi-row insert
	var placeholders string
	args := make([]interface{}, 0, len(checkpoints)*6)

	if common.IsPostgres(r.DBType()) {
		// PostgreSQL uses $1, $2, $3, ... for placeholders
		for i, cp := range checkpoints {
			if i > 0 {
				placeholders += ", "
			}
			base := i * 6
			placeholders += fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, CURRENT_TIMESTAMP)",
				base+1, base+2, base+3, base+4, base+5)

			args = append(args,
				int32(cp.ScanJobID),
				cp.FilePath,
				string(cp.Status),
				common.NullInt64(cp.FileSize),
				common.NullString(cp.FileHash))
		}
	} else {
		// SQLite uses ? for placeholders
		for i, cp := range checkpoints {
			if i > 0 {
				placeholders += ", "
			}
			placeholders += "(?, ?, ?, ?, ?, CURRENT_TIMESTAMP)"

			args = append(args,
				cp.ScanJobID,
				cp.FilePath,
				string(cp.Status),
				common.NullInt64(cp.FileSize),
				common.NullString(cp.FileHash))
		}
	}

	// Execute the multi-row insert
	query := baseQuery + placeholders
	_, err = tx.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetPendingBatch retrieves a batch of pending files to process
func (r *CheckpointRepo) GetPendingBatch(ctx context.Context, jobID int64, limit int) ([]*scanner.ScanCheckpoint, error) {
	rows, err := r.Q().GetPendingScanCheckpoints(ctx, unified.GetPendingScanCheckpointsParams{
		ScanJobID: jobID,
		Limit:     int64(limit),
	})
	if err != nil {
		return nil, err
	}

	return mapSlice(rows, convertToCheckpoint), nil
}

// UpdateStatus updates the processing status of a checkpoint
func (r *CheckpointRepo) UpdateStatus(ctx context.Context, id int64, status scanner.CheckpointStatus, errorMsg string, errorCategory scanner.ErrorCategory) error {
	// Determine processed_at timestamp
	var processedAt sql.NullTime
	if status == scanner.CheckpointCompleted || status == scanner.CheckpointFailed {
		processedAt = sql.NullTime{Time: time.Now(), Valid: true}
	}

	return r.Q().UpdateScanCheckpointStatus(ctx, unified.UpdateScanCheckpointStatusParams{
		Status:        string(status),
		ErrorMessage:  common.NullString(errorMsg),
		ErrorCategory: common.NullString(string(errorCategory)),
		ProcessedAt:   processedAt,
		ID:            id,
	})
}

// UpdateRetryCount increments the retry count for a checkpoint
func (r *CheckpointRepo) UpdateRetryCount(ctx context.Context, id int64, retryCount int) error {
	return r.Q().UpdateScanCheckpointRetryCount(ctx, unified.UpdateScanCheckpointRetryCountParams{
		RetryCount: int64(retryCount),
		ID:         id,
	})
}

// GetStats retrieves aggregate statistics for a scan job's checkpoints
func (r *CheckpointRepo) GetStats(ctx context.Context, jobID int64) (*scanner.CheckpointStats, error) {
	// Get basic stats
	statsRow, err := r.Q().GetScanCheckpointStats(ctx, jobID)
	if err != nil {
		return nil, err
	}

	// Get error breakdown by category
	errorRows, err := r.Q().GetScanCheckpointErrorsByCategory(ctx, jobID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	stats := &scanner.CheckpointStats{
		TotalFiles:       statsRow.TotalFiles,
		PendingFiles:     statsRow.PendingFiles,
		CompletedFiles:   statsRow.CompletedFiles,
		FailedFiles:      statsRow.FailedFiles,
		WarningFiles:     statsRow.WarningFiles,
		ProcessedFiles:   statsRow.ProcessedFiles,
		FirstProcessedAt: parseInterfaceTime(statsRow.FirstProcessedAt),
		ErrorsByCategory: make(map[scanner.ErrorCategory]int64),
	}

	for _, e := range errorRows {
		if e.ErrorCategory.Valid {
			stats.ErrorsByCategory[scanner.ErrorCategory(e.ErrorCategory.String)] = e.ErrorCount
		}
	}

	return stats, nil
}

// ListFailed retrieves all failed checkpoints for error reporting
func (r *CheckpointRepo) ListFailed(ctx context.Context, jobID int64, limit int) ([]*scanner.ScanCheckpoint, error) {
	rows, err := r.Q().ListFailedScanCheckpoints(ctx, unified.ListFailedScanCheckpointsParams{
		ScanJobID: jobID,
		Limit:     int64(limit),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []*scanner.ScanCheckpoint{}, nil
		}
		return nil, err
	}

	return mapSlice(rows, convertToCheckpoint), nil
}

// GetByPath retrieves a checkpoint for a specific file path
func (r *CheckpointRepo) GetByPath(ctx context.Context, jobID int64, filePath string) (*scanner.ScanCheckpoint, error) {
	row, err := r.Q().GetScanCheckpointByPath(ctx, unified.GetScanCheckpointByPathParams{
		ScanJobID: jobID,
		FilePath:  filePath,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, scanner.ErrNotFound
		}
		return nil, err
	}

	return convertToCheckpoint(row), nil
}

// ResetFailed resets all failed checkpoints to pending for retry
func (r *CheckpointRepo) ResetFailed(ctx context.Context, jobID int64) (int64, error) {
	// First count the failed checkpoints
	count, err := r.Q().CountFailedScanCheckpoints(ctx, jobID)
	if err != nil {
		return 0, err
	}

	if count == 0 {
		return 0, nil
	}

	// Reset the failed checkpoints
	if err := r.Q().ResetFailedScanCheckpoints(ctx, jobID); err != nil {
		return 0, err
	}

	return count, nil
}

// DeleteByJobID deletes all checkpoints for a scan job
func (r *CheckpointRepo) DeleteByJobID(ctx context.Context, jobID int64) error {
	return r.Q().DeleteScanCheckpointsByJobID(ctx, jobID)
}

// convertToCheckpoint converts a unified ScanCheckpoint to domain ScanCheckpoint
func convertToCheckpoint(row unified.ScanCheckpoint) *scanner.ScanCheckpoint {
	return &scanner.ScanCheckpoint{
		ID:            row.ID,
		ScanJobID:     row.ScanJobID,
		FilePath:      row.FilePath,
		Status:        scanner.CheckpointStatus(row.Status),
		FileSize:      common.ParseNullInt64(row.FileSize),
		FileHash:      common.ParseNullString(row.FileHash),
		ErrorMessage:  common.ParseNullString(row.ErrorMessage),
		ErrorCategory: scanner.ErrorCategory(common.ParseNullString(row.ErrorCategory)),
		RetryCount:    int(row.RetryCount),
		ProcessedAt:   common.ParseNullTimePtr(row.ProcessedAt),
		CreatedAt:     common.ParseNullTime(row.CreatedAt),
	}
}

// parseInterfaceTime converts interface{} from sqlc to *time.Time
// This handles the MIN() aggregate which returns interface{} for nullable timestamps
func parseInterfaceTime(v interface{}) *time.Time {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case time.Time:
		return &t
	case *time.Time:
		return t
	case string:
		// SQLite returns timestamps as strings in various formats
		// Try formats from most specific to least specific
		formats := []string{
			"2006-01-02 15:04:05.999999999-07:00", // With nanoseconds and timezone
			"2006-01-02 15:04:05.999999999",       // With nanoseconds, no timezone
			"2006-01-02 15:04:05-07:00",           // With timezone, no subseconds
			"2006-01-02 15:04:05",                 // Basic format
			time.RFC3339Nano,                      // ISO 8601 with T separator
			time.RFC3339,                          // ISO 8601 basic
		}
		for _, format := range formats {
			if parsed, err := time.Parse(format, t); err == nil {
				return &parsed
			}
		}
		return nil
	default:
		return nil
	}
}
