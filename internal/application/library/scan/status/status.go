package status

import (
	"context"
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

// Result represents the enriched status of a library scan.
// This combines data from scan jobs, checkpoints, and scan state.
type Result struct {
	// Core job info
	JobID        int64
	LibraryID    int64
	Status       scanner.ScanStatus
	Phase        scanner.ScanPhase
	StartedAt    time.Time
	CompletedAt  *time.Time
	ErrorMessage string

	// Progress metrics
	FilesFound     int64
	FilesProcessed int64   // From scan_state (library-wide), not just current job
	BytesProcessed int64
	Progress       float64 // Calculated: filesProcessed / filesFound * 100
	ETASeconds     *int64  // Estimated seconds remaining

	// Issue counts (from scan_state - library-wide persistent issues)
	ErrorCount   int64
	WarningCount int64

	// Discovery metrics
	EstimatedTotal    int64
	DiscoveryDone     bool
	DiscoveryErrors   int64
	DiscoveryWarnings int64
	DirsScanned       int64
	DirsSkipped       int64
	FilesSkipped      int64
}

// GetScanStatus retrieves the enriched scan status for a library.
// This aggregates data from scan jobs, checkpoints, and scan state.
func GetScanStatus(ctx context.Context, deps *Deps, libraryID int64) (*Result, error) {
	// Get latest scan job
	job, err := deps.ScanRepos.ScanJob.GetLatestByLibrary(ctx, libraryID)
	if err != nil {
		return nil, err
	}

	status := &Result{
		JobID:        job.ID,
		LibraryID:    job.LibraryID,
		Status:       job.Status,
		Phase:        job.Phase,
		StartedAt:    job.StartedAt,
		CompletedAt:  job.CompletedAt,
		ErrorMessage: job.ErrorMessage,

		FilesFound:     job.FilesFound,
		FilesProcessed: job.FilesProcessed, // Default, may be overridden
		BytesProcessed: job.BytesProcessed,
		Progress:       job.Progress, // Default, may be overridden

		ErrorCount:   job.ErrorCount,   // Default, may be overridden
		WarningCount: job.WarningCount, // Default, may be overridden

		EstimatedTotal:    job.EstimatedTotal,
		DiscoveryDone:     job.DiscoveryDone,
		DiscoveryErrors:   job.DiscoveryErrors,
		DiscoveryWarnings: job.DiscoveryWarnings,
		DirsScanned:       job.DirsScanned,
		DirsSkipped:       job.DirsSkipped,
		FilesSkipped:      job.FilesSkipped,
	}

	// Enrich with library-wide counts from scan_state
	EnrichWithScanState(ctx, deps, libraryID, job, status)

	// Calculate ETA for running scans
	if job.Status == scanner.ScanStatusRunning {
		EnrichWithETA(ctx, deps, job.ID, job.FilesFound, status)
	}

	return status, nil
}

// EnrichWithScanState adds library-wide metrics from scan_state.
func EnrichWithScanState(ctx context.Context, deps *Deps, libraryID int64, job *scanner.ScanJob, status *Result) {
	// Get library-level persistent error/warning counts
	if counts, err := deps.ScanRepos.ScanState.CountLibraryIssues(ctx, libraryID); err == nil && counts != nil {
		status.ErrorCount = counts.ErrorCount
		status.WarningCount = counts.WarningCount
	} else {
		deps.Logger.Warn("failed to get library issue counts, using job counts",
			"library_id", libraryID,
			"error", err)
	}

	// Get total files processed (library-wide, not just current job)
	if totalProcessed, err := deps.ScanRepos.ScanState.CountByLibrary(ctx, libraryID); err == nil {
		status.FilesProcessed = totalProcessed

		// Recalculate progress based on library-wide processed count
		if job.FilesFound > 0 {
			status.Progress = float64(totalProcessed) / float64(job.FilesFound) * 100.0
		}
	} else {
		deps.Logger.Warn("failed to get total processed count, using job count",
			"library_id", libraryID,
			"error", err)
	}
}

// EnrichWithETA calculates and adds ETA for running scans.
func EnrichWithETA(ctx context.Context, deps *Deps, jobID int64, filesFound int64, status *Result) {
	if stats, err := deps.ScanRepos.Checkpoint.GetStats(ctx, jobID); err == nil && stats != nil {
		status.ETASeconds = stats.EstimateRemainingSeconds(filesFound)
	}
}
