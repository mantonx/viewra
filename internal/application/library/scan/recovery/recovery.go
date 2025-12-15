package recovery

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

// ScanJobCompleter is the interface for completing scan jobs.
type ScanJobCompleter interface {
	Complete(ctx context.Context, job *scanner.ScanJob) error
}

// LogPanic logs a panic with stack trace.
func LogPanic(logger *slog.Logger, r any, description string, fields ...any) {
	allFields := append([]any{
		"panic", r,
		"stack_trace", string(debug.Stack()),
	}, fields...)
	logger.Error("PANIC: "+description, allFields...)
}

// RecoverFromPanic recovers from a panic and marks the scan job as failed.
func RecoverFromPanic(logger *slog.Logger, completer ScanJobCompleter, jobID, libraryID int64, description string) {
	if r := recover(); r != nil {
		LogPanic(logger, r, description, "job_id", jobID, "library_id", libraryID)

		failedJob := &scanner.ScanJob{
			ID:           jobID,
			Status:       scanner.ScanStatusFailed,
			ErrorMessage: fmt.Sprintf("scan panicked: %v", r),
			CompletedAt:  &[]time.Time{time.Now()}[0],
		}
		if err := completer.Complete(context.Background(), failedJob); err != nil {
			logger.Error("failed to mark panicked scan job as failed", "job_id", jobID, "error", err)
		}
	}
}

// RecoverFromPanicWithError recovers from a panic and sends an error to the channel.
func RecoverFromPanicWithError(logger *slog.Logger, jobID, libraryID int64, description string, errChan chan<- error) {
	if r := recover(); r != nil {
		LogPanic(logger, r, description, "job_id", jobID, "library_id", libraryID)

		err := fmt.Errorf("%s: %v", description, r)
		select {
		case errChan <- err:
		default:
		}
	}
}

// RecoverWorkerPanic recovers from a panic in a worker goroutine.
func RecoverWorkerPanic(logger *slog.Logger, jobID int64, workerID int) {
	if r := recover(); r != nil {
		LogPanic(logger, r, "worker goroutine panicked", "worker_id", workerID, "job_id", jobID)
	}
}
