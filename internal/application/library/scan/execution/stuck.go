package execution

import (
	"context"
	"fmt"
	"time"

	"github.com/mantonx/viewra/internal/application/library/scan/status"
	"github.com/mantonx/viewra/internal/domain/scanner"
)

// BackgroundStarter is the interface for starting scans in the background.
type BackgroundStarter interface {
	StartScanBackground(jobID int64, libraryID int64, panicContext string)
}

// HandleStuckScan handles a stuck scan job (one that was running when the server restarted).
func HandleStuckScan(ctx context.Context, deps *Deps, job *scanner.ScanJob, backgroundStarter BackgroundStarter) {
	lib, err := deps.MediaRepos.Library.GetByID(ctx, job.LibraryID)
	if err != nil {
		deps.Logger.Error("failed to get library for stuck scan",
			"library_id", job.LibraryID,
			"scan_id", job.ID,
			"error", err)
		failedJob := &scanner.ScanJob{
			ID:           job.ID,
			Status:       scanner.ScanStatusFailed,
			ErrorMessage: fmt.Sprintf("library not found during resume: %v", err),
			CompletedAt:  &[]time.Time{time.Now()}[0],
		}
		status.CompleteSafely(ctx, deps.StatusDeps(), failedJob)
		return
	}

	stats, err := deps.ScanRepos.Checkpoint.GetStats(ctx, job.ID)
	if err != nil {
		deps.Logger.Warn("failed to get checkpoint stats for stuck scan, resuming anyway",
			"scan_id", job.ID,
			"error", err)
		backgroundStarter.StartScanBackground(job.ID, lib.ID, "resumed stuck scan goroutine panicked")
		return
	}

	actuallyComplete := stats.PendingFiles == 0 && job.FilesFound > 0 && stats.ProcessedFiles >= job.FilesFound
	if actuallyComplete {
		deps.Logger.Info("stuck scan has no pending files, marking as completed",
			"scan_id", job.ID,
			"files_found", job.FilesFound)
		status.CompleteFromStats(ctx, deps.StatusDeps(), job.ID, job.FilesFound, stats)
		return
	}

	deps.Logger.Info("resuming stuck scan",
		"scan_id", job.ID,
		"library_id", lib.ID,
		"files_found", job.FilesFound,
		"pending_checkpoints", stats.PendingFiles,
		"processed_checkpoints", stats.ProcessedFiles)

	backgroundStarter.StartScanBackground(job.ID, lib.ID, "resumed stuck scan goroutine panicked")
}
