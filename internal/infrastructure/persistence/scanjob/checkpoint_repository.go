package scanjob

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// CheckpointRepo implements scanner.CheckpointRepository using sqlc.
// It supports both SQLite and PostgreSQL through database-specific queriers.
type CheckpointRepo struct {
	db       *sql.DB
	dbType   string
	postgres *sqlc_postgres.Queries
	sqlite   *sqlc_sqlite.Queries
	router   *common.QueryRouter
}

// NewCheckpointRepo creates a new checkpoint repository with the appropriate database driver.
// The driver parameter should be "sqlite", "sqlite3", "postgres", or "postgresql".
func NewCheckpointRepo(db *sql.DB, driver string) *CheckpointRepo {
	r := &CheckpointRepo{
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
	tx, err := r.db.BeginTx(ctx, nil)
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

	if r.router.IsPostgresDB() {
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
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetPendingScanCheckpoints(ctx, sqlc_postgres.GetPendingScanCheckpointsParams{
				ScanJobID: int32(jobID),
				Limit:     int32(limit),
			})
		},
		func() (any, error) {
			return r.sqlite.GetPendingScanCheckpoints(ctx, sqlc_sqlite.GetPendingScanCheckpointsParams{
				ScanJobID: jobID,
				Limit:     int64(limit),
			})
		},
	)
	if err != nil {
		return nil, err
	}

	return r.convertSlice(result), nil
}

// UpdateStatus updates the processing status of a checkpoint
func (r *CheckpointRepo) UpdateStatus(ctx context.Context, id int64, status scanner.CheckpointStatus, errorMsg string, errorCategory scanner.ErrorCategory) error {
	// Determine processed_at timestamp
	var processedAt sql.NullTime
	if status == scanner.CheckpointCompleted || status == scanner.CheckpointFailed {
		processedAt = sql.NullTime{Time: time.Now(), Valid: true}
	}

	_, err := r.router.Route(
		func() (any, error) {
			return nil, r.postgres.UpdateScanCheckpointStatus(ctx, sqlc_postgres.UpdateScanCheckpointStatusParams{
				Status:        string(status),
				ErrorMessage:  common.NullString(errorMsg),
				ErrorCategory: common.NullString(string(errorCategory)),
				ProcessedAt:   processedAt,
				ID:            int32(id),
			})
		},
		func() (any, error) {
			return nil, r.sqlite.UpdateScanCheckpointStatus(ctx, sqlc_sqlite.UpdateScanCheckpointStatusParams{
				Status:        string(status),
				ErrorMessage:  common.NullString(errorMsg),
				ErrorCategory: common.NullString(string(errorCategory)),
				ProcessedAt:   processedAt,
				ID:            id,
			})
		},
	)
	return err
}

// UpdateRetryCount increments the retry count for a checkpoint
func (r *CheckpointRepo) UpdateRetryCount(ctx context.Context, id int64, retryCount int) error {
	_, err := r.router.Route(
		func() (any, error) {
			return nil, r.postgres.UpdateScanCheckpointRetryCount(ctx, sqlc_postgres.UpdateScanCheckpointRetryCountParams{
				RetryCount: int32(retryCount),
				ID:         int32(id),
			})
		},
		func() (any, error) {
			return nil, r.sqlite.UpdateScanCheckpointRetryCount(ctx, sqlc_sqlite.UpdateScanCheckpointRetryCountParams{
				RetryCount: int64(retryCount),
				ID:         id,
			})
		},
	)
	return err
}

// GetStats retrieves aggregate statistics for a scan job's checkpoints
func (r *CheckpointRepo) GetStats(ctx context.Context, jobID int64) (*scanner.CheckpointStats, error) {
	// Get basic stats
	statsResult, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetScanCheckpointStats(ctx, int32(jobID))
		},
		func() (any, error) {
			return r.sqlite.GetScanCheckpointStats(ctx, jobID)
		},
	)
	if err != nil {
		return nil, err
	}

	// Get error breakdown by category
	errorResult, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetScanCheckpointErrorsByCategory(ctx, int32(jobID))
		},
		func() (any, error) {
			return r.sqlite.GetScanCheckpointErrorsByCategory(ctx, jobID)
		},
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	stats := &scanner.CheckpointStats{
		ErrorsByCategory: make(map[scanner.ErrorCategory]int64),
	}

	if r.router.IsPostgresDB() {
		pgStats := statsResult.(sqlc_postgres.GetScanCheckpointStatsRow)
		stats.TotalFiles = pgStats.TotalFiles
		stats.PendingFiles = int64(pgStats.PendingFiles)
		stats.CompletedFiles = int64(pgStats.CompletedFiles)
		stats.FailedFiles = int64(pgStats.FailedFiles)
		stats.WarningFiles = int64(pgStats.WarningFiles)
		stats.ProcessedFiles = int64(pgStats.ProcessedFiles)
		stats.FirstProcessedAt = parseInterfaceTime(pgStats.FirstProcessedAt)

		if errorResult != nil {
			pgErrors := errorResult.([]sqlc_postgres.GetScanCheckpointErrorsByCategoryRow)
			for _, e := range pgErrors {
				if e.ErrorCategory.Valid {
					stats.ErrorsByCategory[scanner.ErrorCategory(e.ErrorCategory.String)] = e.ErrorCount
				}
			}
		}
	} else {
		sqStats := statsResult.(sqlc_sqlite.GetScanCheckpointStatsRow)
		stats.TotalFiles = sqStats.TotalFiles
		stats.PendingFiles = int64(sqStats.PendingFiles.Float64)
		stats.CompletedFiles = int64(sqStats.CompletedFiles.Float64)
		stats.FailedFiles = int64(sqStats.FailedFiles.Float64)
		stats.WarningFiles = int64(sqStats.WarningFiles.Float64)
		stats.ProcessedFiles = int64(sqStats.ProcessedFiles.Float64)
		stats.FirstProcessedAt = parseInterfaceTime(sqStats.FirstProcessedAt)

		if errorResult != nil {
			sqErrors := errorResult.([]sqlc_sqlite.GetScanCheckpointErrorsByCategoryRow)
			for _, e := range sqErrors {
				if e.ErrorCategory.Valid {
					stats.ErrorsByCategory[scanner.ErrorCategory(e.ErrorCategory.String)] = e.ErrorCount
				}
			}
		}
	}

	return stats, nil
}

// ListFailed retrieves all failed checkpoints for error reporting
func (r *CheckpointRepo) ListFailed(ctx context.Context, jobID int64, limit int) ([]*scanner.ScanCheckpoint, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.ListFailedScanCheckpoints(ctx, sqlc_postgres.ListFailedScanCheckpointsParams{
				ScanJobID: int32(jobID),
				Limit:     int32(limit),
			})
		},
		func() (any, error) {
			return r.sqlite.ListFailedScanCheckpoints(ctx, sqlc_sqlite.ListFailedScanCheckpointsParams{
				ScanJobID: jobID,
				Limit:     int64(limit),
			})
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []*scanner.ScanCheckpoint{}, nil
		}
		return nil, err
	}

	return r.convertSlice(result), nil
}

// GetByPath retrieves a checkpoint for a specific file path
func (r *CheckpointRepo) GetByPath(ctx context.Context, jobID int64, filePath string) (*scanner.ScanCheckpoint, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetScanCheckpointByPath(ctx, sqlc_postgres.GetScanCheckpointByPathParams{
				ScanJobID: int32(jobID),
				FilePath:  filePath,
			})
		},
		func() (any, error) {
			return r.sqlite.GetScanCheckpointByPath(ctx, sqlc_sqlite.GetScanCheckpointByPathParams{
				ScanJobID: jobID,
				FilePath:  filePath,
			})
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, scanner.ErrNotFound
		}
		return nil, err
	}

	return r.convertToCheckpoint(result), nil
}

// ResetFailed resets all failed checkpoints to pending for retry
func (r *CheckpointRepo) ResetFailed(ctx context.Context, jobID int64) (int64, error) {
	// First count the failed checkpoints
	countResult, err := r.router.Route(
		func() (any, error) {
			return r.postgres.CountFailedScanCheckpoints(ctx, int32(jobID))
		},
		func() (any, error) {
			return r.sqlite.CountFailedScanCheckpoints(ctx, jobID)
		},
	)
	if err != nil {
		return 0, err
	}

	var count int64
	if r.router.IsPostgresDB() {
		count = countResult.(int64)
	} else {
		count = countResult.(int64)
	}

	if count == 0 {
		return 0, nil
	}

	// Reset the failed checkpoints
	_, err = r.router.Route(
		func() (any, error) {
			return nil, r.postgres.ResetFailedScanCheckpoints(ctx, int32(jobID))
		},
		func() (any, error) {
			return nil, r.sqlite.ResetFailedScanCheckpoints(ctx, jobID)
		},
	)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// DeleteByJobID deletes all checkpoints for a scan job
func (r *CheckpointRepo) DeleteByJobID(ctx context.Context, jobID int64) error {
	_, err := r.router.Route(
		func() (any, error) {
			return nil, r.postgres.DeleteScanCheckpointsByJobID(ctx, int32(jobID))
		},
		func() (any, error) {
			return nil, r.sqlite.DeleteScanCheckpointsByJobID(ctx, jobID)
		},
	)
	return err
}

// convertSlice converts a slice of sqlc checkpoints to domain checkpoints
func (r *CheckpointRepo) convertSlice(result any) []*scanner.ScanCheckpoint {
	if r.router.IsPostgresDB() {
		pgSlice := result.([]sqlc_postgres.ScanCheckpoint)
		checkpoints := make([]*scanner.ScanCheckpoint, len(pgSlice))
		for i, pg := range pgSlice {
			checkpoints[i] = r.convertToCheckpoint(pg)
		}
		return checkpoints
	}

	sqSlice := result.([]sqlc_sqlite.ScanCheckpoint)
	checkpoints := make([]*scanner.ScanCheckpoint, len(sqSlice))
	for i, sq := range sqSlice {
		checkpoints[i] = r.convertToCheckpoint(sq)
	}
	return checkpoints
}

// convertToCheckpoint converts sqlc result to domain ScanCheckpoint
func (r *CheckpointRepo) convertToCheckpoint(result any) *scanner.ScanCheckpoint {
	if r.router.IsPostgresDB() {
		pg := result.(sqlc_postgres.ScanCheckpoint)
		return &scanner.ScanCheckpoint{
			ID:            int64(pg.ID),
			ScanJobID:     int64(pg.ScanJobID),
			FilePath:      pg.FilePath,
			Status:        scanner.CheckpointStatus(pg.Status),
			FileSize:      common.ParseNullInt64(pg.FileSize),
			FileHash:      common.ParseNullString(pg.FileHash),
			ErrorMessage:  common.ParseNullString(pg.ErrorMessage),
			ErrorCategory: scanner.ErrorCategory(common.ParseNullString(pg.ErrorCategory)),
			RetryCount:    int(pg.RetryCount),
			ProcessedAt:   common.ParseNullTimePtr(pg.ProcessedAt),
			CreatedAt:     common.ParseNullTime(pg.CreatedAt),
		}
	}

	sq := result.(sqlc_sqlite.ScanCheckpoint)
	return &scanner.ScanCheckpoint{
		ID:            sq.ID,
		ScanJobID:     sq.ScanJobID,
		FilePath:      sq.FilePath,
		Status:        scanner.CheckpointStatus(sq.Status),
		FileSize:      common.ParseNullInt64(sq.FileSize),
		FileHash:      common.ParseNullString(sq.FileHash),
		ErrorMessage:  common.ParseNullString(sq.ErrorMessage),
		ErrorCategory: scanner.ErrorCategory(common.ParseNullString(sq.ErrorCategory)),
		RetryCount:    int(sq.RetryCount),
		ProcessedAt:   common.ParseNullTimePtr(sq.ProcessedAt),
		CreatedAt:     common.ParseNullTime(sq.CreatedAt),
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
