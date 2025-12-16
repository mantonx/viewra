package status

import (
	"context"
	"log/slog"
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/events"
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
// Also publishes scan.completed or scan.failed events if EventBus is configured.
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
		return
	}

	// Publish completion event if event bus is configured
	publishCompletionEvent(deps, job)
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

// publishCompletionEvent publishes scan.completed or scan.failed events.
func publishCompletionEvent(deps *Deps, job *scanner.ScanJob) {
	if deps.EventBus == nil {
		return
	}

	// Determine event type based on status
	var eventType events.EventType
	switch job.Status {
	case scanner.ScanStatusCompleted:
		eventType = events.EventScanCompleted
	case scanner.ScanStatusFailed:
		eventType = events.EventScanFailed
	default:
		return // Don't publish for other statuses
	}

	event := events.NewEvent(eventType, "scanner").
		WithLibraryID(job.LibraryID).
		WithData("job_id", job.ID).
		WithData("status", string(job.Status)).
		WithData("progress", job.Progress).
		WithData("files_found", job.FilesFound).
		WithData("files_processed", job.FilesProcessed).
		WithData("error_count", job.ErrorCount).
		WithData("warning_count", job.WarningCount).
		WithData("phase", string(job.Phase)).
		WithData("discovery_done", job.DiscoveryDone)

	if job.ErrorMessage != "" {
		event = event.WithData("error_message", job.ErrorMessage)
	}

	deps.EventBus.Publish(event.Build())
}

// ptrTime returns a pointer to the given time value.
func ptrTime(t time.Time) *time.Time {
	return &t
}
