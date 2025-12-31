package scanjob

import (
	"context"
	"log/slog"
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

// RecoverStuckScans checks for scans stuck in "running" status and marks them as failed.
// This should be called on application startup to clean up any scans that were interrupted
// by a server restart or crash.
func (r *Repository) RecoverStuckScans(ctx context.Context, logger *slog.Logger) error {
	// Find all scans in "running" status
	runningScans, err := r.GetRunningScans(ctx)
	if err != nil {
		return err
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
	rows, err := r.Q().ListRunningScanJobs(ctx)
	if err != nil {
		return nil, err
	}

	return mapSlice(rows, convertToScanJob), nil
}
