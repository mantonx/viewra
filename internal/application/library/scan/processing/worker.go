package processing

import (
	"context"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
)

// UpdateCheckpointStatus updates checkpoint status with scan job deletion handling.
// Returns true if the operation should abort (job was deleted), false otherwise.
func UpdateCheckpointStatus(
	ctx context.Context,
	deps *Deps,
	checkpoint *scanner.ScanCheckpoint,
	status scanner.CheckpointStatus,
	message string,
	category scanner.ErrorCategory,
	action string,
) (shouldAbort bool) {
	err := deps.ScanRepos.Checkpoint.UpdateStatus(ctx, checkpoint.ID, status, message, category)
	if err == nil {
		return false
	}

	if scanner.IsScanJobDeleted(err) {
		return true
	}

	deps.Logger.Error("failed to "+action,
		"checkpoint_id", checkpoint.ID,
		"file_path", checkpoint.FilePath,
		"error", err)
	return false
}

// ProcessCheckpointWorker handles individual checkpoint processing in a worker goroutine.
// This allows parallel FFprobe execution to overcome I/O latency on network storage.
func ProcessCheckpointWorker(
	ctx context.Context,
	deps *Deps,
	lib *library.Library,
	checkpoint *scanner.ScanCheckpoint,
	maxRetries int,
	foundFilesMu *sync.Mutex,
	foundFiles map[string]bool,
	existingMediaCache *sync.Map,
) {
	// Mark as processing
	if UpdateCheckpointStatus(ctx, deps, checkpoint, scanner.CheckpointProcessing, "", "", "mark checkpoint as processing") {
		return
	}

	// Add overall timeout protection at worker level to catch any hangs
	// This includes os.Stat, database calls, and ProcessFile execution
	// Use configurable timeout as absolute maximum to prevent worker deadlocks
	workerCtx, cancel := context.WithTimeout(ctx, deps.Config.WorkerTimeout)
	defer cancel()

	// Process the file with timeout protection
	hasWarning, err := ProcessFileWithCheckpoint(workerCtx, deps, lib, checkpoint, existingMediaCache)

	if err != nil {
		HandleCheckpointError(ctx, deps, lib, checkpoint, err, maxRetries)
		return
	}

	if hasWarning {
		HandleCheckpointWarning(ctx, deps, checkpoint, foundFilesMu, foundFiles)
		return
	}

	HandleCheckpointSuccess(ctx, deps, checkpoint, foundFilesMu, foundFiles)
}

// HandleCheckpointError processes a checkpoint that failed with an error.
func HandleCheckpointError(
	ctx context.Context,
	deps *Deps,
	lib *library.Library,
	checkpoint *scanner.ScanCheckpoint,
	err error,
	maxRetries int,
) {
	category := scanner.CategorizeError(err)

	// Check if error is transient and we haven't exceeded max retries
	if scanner.IsTransientError(err) && checkpoint.RetryCount < maxRetries {
		RetryCheckpoint(ctx, deps, checkpoint, err, maxRetries)
		return
	}

	// Either not transient or max retries exceeded - mark as failed
	if UpdateCheckpointStatus(ctx, deps, checkpoint, scanner.CheckpointFailed, err.Error(), category, "mark checkpoint as failed") {
		return
	}

	// Persist error to scan_state for library-level tracking
	if setErr := deps.ScanRepos.ScanState.SetError(ctx, lib.ID, checkpoint.FilePath, err.Error(), string(category)); setErr != nil {
		deps.Logger.Warn("failed to set error in scan_state",
			"file_path", checkpoint.FilePath,
			"error", setErr)
	}

	if checkpoint.RetryCount >= maxRetries {
		deps.Logger.Error("max retries exceeded",
			"file_path", checkpoint.FilePath,
			"error_category", category,
			"retry_count", checkpoint.RetryCount,
			"error", err)
	} else {
		deps.Logger.Error("failed to process file",
			"file_path", checkpoint.FilePath,
			"error_category", category,
			"error", err)
	}
}

// RetryCheckpoint handles retry logic for transient errors.
func RetryCheckpoint(
	ctx context.Context,
	deps *Deps,
	checkpoint *scanner.ScanCheckpoint,
	err error,
	maxRetries int,
) {
	checkpoint.RetryCount++

	if retryErr := deps.ScanRepos.Checkpoint.UpdateRetryCount(ctx, checkpoint.ID, checkpoint.RetryCount); retryErr != nil {
		if scanner.IsScanJobDeleted(retryErr) {
			return
		}
		deps.Logger.Error("failed to update retry count",
			"checkpoint_id", checkpoint.ID,
			"file_path", checkpoint.FilePath,
			"retry_count", checkpoint.RetryCount,
			"error", retryErr)
	}

	// Calculate exponential backoff: base * 2^retryCount (default: 1s, 2s, 4s)
	backoffBase := deps.Config.RetryBackoffBase
	if backoffBase == 0 {
		backoffBase = time.Second
	}
	backoffDuration := time.Duration(1<<checkpoint.RetryCount) * backoffBase
	deps.Logger.Info("retrying file after transient error",
		"file_path", checkpoint.FilePath,
		"retry_count", checkpoint.RetryCount,
		"max_retries", maxRetries,
		"backoff_duration", backoffDuration,
		"error", err)

	time.Sleep(backoffDuration)

	// Reset to pending for retry
	UpdateCheckpointStatus(ctx, deps, checkpoint, scanner.CheckpointPending, "", "", "reset checkpoint to pending for retry")
}

// HandleCheckpointWarning processes a checkpoint that completed with warnings.
func HandleCheckpointWarning(
	ctx context.Context,
	deps *Deps,
	checkpoint *scanner.ScanCheckpoint,
	foundFilesMu *sync.Mutex,
	foundFiles map[string]bool,
) {
	if UpdateCheckpointStatus(ctx, deps, checkpoint, scanner.CheckpointWarning, "metadata extraction incomplete", "ffmpeg", "mark checkpoint as warning") {
		return
	}

	foundFilesMu.Lock()
	foundFiles[checkpoint.FilePath] = true
	foundFilesMu.Unlock()
}

// HandleCheckpointSuccess processes a checkpoint that completed successfully.
func HandleCheckpointSuccess(
	ctx context.Context,
	deps *Deps,
	checkpoint *scanner.ScanCheckpoint,
	foundFilesMu *sync.Mutex,
	foundFiles map[string]bool,
) {
	// Update status - continue even if it fails since the file was processed
	UpdateCheckpointStatus(ctx, deps, checkpoint, scanner.CheckpointCompleted, "", "", "mark checkpoint as completed")

	foundFilesMu.Lock()
	foundFiles[checkpoint.FilePath] = true
	foundFilesMu.Unlock()
}
