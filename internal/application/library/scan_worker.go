package library

import (
	"context"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
)

// updateCheckpointStatus updates checkpoint status with scan job deletion handling.
// Returns true if the operation should abort (job was deleted), false otherwise.
func (uc *ScanLibraryUseCase) updateCheckpointStatus(
	ctx context.Context,
	checkpoint *scanner.ScanCheckpoint,
	status scanner.CheckpointStatus,
	message string,
	category scanner.ErrorCategory,
	action string,
) (shouldAbort bool) {
	err := uc.scanRepos.Checkpoint.UpdateStatus(ctx, checkpoint.ID, status, message, category)
	if err == nil {
		return false
	}

	if scanner.IsScanJobDeleted(err) {
		return true
	}

	uc.logger.Error("failed to "+action,
		"checkpoint_id", checkpoint.ID,
		"file_path", checkpoint.FilePath,
		"error", err)
	return false
}

// processCheckpointWorker handles individual checkpoint processing in a worker goroutine
// This allows parallel FFprobe execution to overcome I/O latency on network storage
func (uc *ScanLibraryUseCase) processCheckpointWorker(
	ctx context.Context,
	lib *library.Library,
	checkpoint *scanner.ScanCheckpoint,
	maxRetries int,
	foundFilesMu *sync.Mutex,
	foundFiles map[string]bool,
	existingMediaCache *sync.Map,
) {
	// Mark as processing
	if uc.updateCheckpointStatus(ctx, checkpoint, scanner.CheckpointProcessing, "", "", "mark checkpoint as processing") {
		return
	}

	// Add overall timeout protection at worker level to catch any hangs
	// This includes os.Stat, database calls, and ProcessFile execution
	// Use configurable timeout as absolute maximum to prevent worker deadlocks
	workerCtx, cancel := context.WithTimeout(ctx, uc.config.WorkerTimeout)
	defer cancel()

	// Process the file with timeout protection
	hasWarning, err := uc.processFileWithCheckpoint(workerCtx, lib, checkpoint, existingMediaCache)

	if err != nil {
		uc.handleCheckpointError(ctx, lib, checkpoint, err, maxRetries)
		return
	}

	if hasWarning {
		uc.handleCheckpointWarning(ctx, checkpoint, foundFilesMu, foundFiles)
		return
	}

	uc.handleCheckpointSuccess(ctx, checkpoint, foundFilesMu, foundFiles)
}

// handleCheckpointError processes a checkpoint that failed with an error
func (uc *ScanLibraryUseCase) handleCheckpointError(
	ctx context.Context,
	lib *library.Library,
	checkpoint *scanner.ScanCheckpoint,
	err error,
	maxRetries int,
) {
	category := scanner.CategorizeError(err)

	// Check if error is transient and we haven't exceeded max retries
	if scanner.IsTransientError(err) && checkpoint.RetryCount < maxRetries {
		uc.retryCheckpoint(ctx, checkpoint, err, maxRetries)
		return
	}

	// Either not transient or max retries exceeded - mark as failed
	if uc.updateCheckpointStatus(ctx, checkpoint, scanner.CheckpointFailed, err.Error(), category, "mark checkpoint as failed") {
		return
	}

	// Persist error to scan_state for library-level tracking
	if setErr := uc.scanRepos.ScanState.SetError(ctx, lib.ID, checkpoint.FilePath, err.Error(), string(category)); setErr != nil {
		uc.logger.Warn("failed to set error in scan_state",
			"file_path", checkpoint.FilePath,
			"error", setErr)
	}

	if checkpoint.RetryCount >= maxRetries {
		uc.logger.Error("max retries exceeded",
			"file_path", checkpoint.FilePath,
			"error_category", category,
			"retry_count", checkpoint.RetryCount,
			"error", err)
	} else {
		uc.logger.Error("failed to process file",
			"file_path", checkpoint.FilePath,
			"error_category", category,
			"error", err)
	}
}

// retryCheckpoint handles retry logic for transient errors
func (uc *ScanLibraryUseCase) retryCheckpoint(
	ctx context.Context,
	checkpoint *scanner.ScanCheckpoint,
	err error,
	maxRetries int,
) {
	checkpoint.RetryCount++

	if retryErr := uc.scanRepos.Checkpoint.UpdateRetryCount(ctx, checkpoint.ID, checkpoint.RetryCount); retryErr != nil {
		if scanner.IsScanJobDeleted(retryErr) {
			return
		}
		uc.logger.Error("failed to update retry count",
			"checkpoint_id", checkpoint.ID,
			"file_path", checkpoint.FilePath,
			"retry_count", checkpoint.RetryCount,
			"error", retryErr)
	}

	// Calculate exponential backoff: 2^retryCount seconds (1s, 2s, 4s)
	backoffDuration := time.Duration(1<<checkpoint.RetryCount) * time.Second
	uc.logger.Info("retrying file after transient error",
		"file_path", checkpoint.FilePath,
		"retry_count", checkpoint.RetryCount,
		"max_retries", maxRetries,
		"backoff_duration", backoffDuration,
		"error", err)

	time.Sleep(backoffDuration)

	// Reset to pending for retry
	uc.updateCheckpointStatus(ctx, checkpoint, scanner.CheckpointPending, "", "", "reset checkpoint to pending for retry")
}

// handleCheckpointWarning processes a checkpoint that completed with warnings
func (uc *ScanLibraryUseCase) handleCheckpointWarning(
	ctx context.Context,
	checkpoint *scanner.ScanCheckpoint,
	foundFilesMu *sync.Mutex,
	foundFiles map[string]bool,
) {
	if uc.updateCheckpointStatus(ctx, checkpoint, scanner.CheckpointWarning, "metadata extraction incomplete", "ffmpeg", "mark checkpoint as warning") {
		return
	}

	foundFilesMu.Lock()
	foundFiles[checkpoint.FilePath] = true
	foundFilesMu.Unlock()
}

// handleCheckpointSuccess processes a checkpoint that completed successfully
func (uc *ScanLibraryUseCase) handleCheckpointSuccess(
	ctx context.Context,
	checkpoint *scanner.ScanCheckpoint,
	foundFilesMu *sync.Mutex,
	foundFiles map[string]bool,
) {
	// Update status - continue even if it fails since the file was processed
	uc.updateCheckpointStatus(ctx, checkpoint, scanner.CheckpointCompleted, "", "", "mark checkpoint as completed")

	foundFilesMu.Lock()
	foundFiles[checkpoint.FilePath] = true
	foundFilesMu.Unlock()
}
