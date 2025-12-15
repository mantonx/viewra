package status

import (
	"context"
	"log/slog"
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

// CompleteFromStats marks a scan job as completed using checkpoint statistics.
func CompleteFromStats(ctx context.Context, deps *Deps, jobID int64, filesFound int64, stats *scanner.CheckpointStats) {
	completedJob := &scanner.ScanJob{
		ID:             jobID,
		Status:         scanner.ScanStatusCompleted,
		FilesFound:     filesFound,
		FilesProcessed: stats.ProcessedFiles,
		ErrorCount:     stats.FailedFiles,
		WarningCount:   stats.WarningFiles,
		Progress:       stats.GetProgress(),
		CompletedAt:    ptrTime(time.Now()),
		Phase:          scanner.ScanPhaseCompleted,
		DiscoveryDone:  true,
	}
	CompleteSafely(ctx, deps, completedJob)
}

// CompleteWithError marks a scan job as failed with an error message.
func CompleteWithError(ctx context.Context, deps *Deps, jobID int64, err error) {
	job := &scanner.ScanJob{
		ID:           jobID,
		Status:       scanner.ScanStatusFailed,
		ErrorMessage: err.Error(),
		CompletedAt:  ptrTime(time.Now()),
	}
	CompleteSafely(ctx, deps, job)
}

// CompleteSafely persists the job completion, handling deleted jobs gracefully.
func CompleteSafely(ctx context.Context, deps *Deps, job *scanner.ScanJob) {
	if err := deps.ScanRepos.ScanJob.Complete(ctx, job); err != nil {
		if scanner.IsScanJobDeleted(err) {
			deps.Logger.Debug("scan job was deleted before completion",
				"job_id", job.ID,
				"status", job.Status)
			return
		}
		deps.Logger.Error("failed to complete scan job",
			"job_id", job.ID,
			"status", job.Status,
			"error", err)
	}
}

// MarkFailed marks a scan job as failed with a formatted error message.
func MarkFailed(ctx context.Context, deps *Deps, jobID int64, errMsg string, logger *slog.Logger, logAttrs ...any) {
	if logger != nil {
		logger.Error(errMsg, logAttrs...)
	}
	job := &scanner.ScanJob{
		ID:           jobID,
		Status:       scanner.ScanStatusFailed,
		ErrorMessage: errMsg,
		CompletedAt:  ptrTime(time.Now()),
	}
	CompleteSafely(ctx, deps, job)
}

// ptrTime returns a pointer to the given time value.
func ptrTime(t time.Time) *time.Time {
	return &t
}
