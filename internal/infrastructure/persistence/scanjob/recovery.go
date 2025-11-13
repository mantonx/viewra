package scanjob

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/viewra/viewra/internal/domain/scanner"
	"github.com/viewra/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/viewra/viewra/internal/infrastructure/database/sqlc_sqlite"
)

// RecoverStuckScans checks for scans stuck in "running" status and marks them as failed.
// This should be called on application startup to clean up any scans that were interrupted
// by a server restart or crash.
func (r *Repository) RecoverStuckScans(ctx context.Context, logger *slog.Logger) error {
	// Find all scans in "running" status
	runningScans, err := r.GetRunningScans(ctx)
	if err != nil {
		return fmt.Errorf("failed to get running scans: %w", err)
	}

	if len(runningScans) == 0 {
		logger.Info("No stuck scans found during recovery")
		return nil
	}

	logger.Info("Found stuck scans during recovery", "count", len(runningScans))

	// Mark each stuck scan as failed
	for _, scan := range runningScans {
		logger.Warn("Recovering stuck scan",
			"scan_id", scan.ID,
			"library_id", scan.LibraryID,
			"started_at", scan.StartedAt,
		)

		// Update scan status to failed
		now := time.Now()
		scan.Status = scanner.ScanStatusFailed
		scan.CompletedAt = &now
		scan.ErrorMessage = "Scan interrupted - server was restarted or crashed. Please retry the scan."

		if err := r.Complete(ctx, scan); err != nil {
			logger.Error("Failed to recover stuck scan",
				"scan_id", scan.ID,
				"error", err,
			)
			continue
		}

		logger.Info("Successfully recovered stuck scan",
			"scan_id", scan.ID,
			"library_id", scan.LibraryID,
		)
	}

	return nil
}

// GetRunningScans returns all scans with "running" status
func (r *Repository) GetRunningScans(ctx context.Context) ([]*scanner.ScanJob, error) {
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

	// Convert results to domain scan jobs
	if r.router.IsPostgresDB() {
		pgJobs := result.([]sqlc_postgres.ScanJob)
		return r.convertManyToScanJobs(pgJobs), nil
	}

	sqJobs := result.([]sqlc_sqlite.ScanJob)
	return r.convertManyToScanJobs(sqJobs), nil
}

// convertManyToScanJobs converts a slice of database scan jobs to domain scan jobs
func (r *Repository) convertManyToScanJobs(jobs any) []*scanner.ScanJob {
	switch v := jobs.(type) {
	case []sqlc_postgres.ScanJob:
		result := make([]*scanner.ScanJob, 0, len(v))
		for i := range v {
			result = append(result, r.convertToScanJob(v[i]))
		}
		return result
	case []sqlc_sqlite.ScanJob:
		result := make([]*scanner.ScanJob, 0, len(v))
		for i := range v {
			result = append(result, r.convertToScanJob(v[i]))
		}
		return result
	default:
		return nil
	}
}
