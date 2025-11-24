package library

import (
	"context"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
)

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
	if err := uc.scanRepos.Checkpoint.UpdateStatus(ctx, checkpoint.ID, scanner.CheckpointProcessing, "", ""); err != nil {
		// If the scan job was deleted (library deleted during scan), silently return
		if scanner.IsScanJobDeleted(err) {
			return
		}
		uc.logger.Error("failed to mark checkpoint as processing",
			"checkpoint_id", checkpoint.ID,
			"file_path", checkpoint.FilePath,
			"error", err)
		return
	}

	// Add overall timeout protection at worker level to catch any hangs
	// This includes os.Stat, database calls, and ProcessFile execution
	// Use 5 minutes as absolute maximum to prevent worker deadlocks
	workerCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Process the file with timeout protection
	hasWarning, err := uc.processFileWithCheckpoint(workerCtx, lib, checkpoint, existingMediaCache)

	if err != nil {
		// Categorize the error
		category := scanner.CategorizeError(err)

		// Check if error is transient and we haven't exceeded max retries
		if scanner.IsTransientError(err) && checkpoint.RetryCount < maxRetries {
			// Increment retry count
			checkpoint.RetryCount++
			if retryErr := uc.scanRepos.Checkpoint.UpdateRetryCount(ctx, checkpoint.ID, checkpoint.RetryCount); retryErr != nil {
				// If the scan job was deleted (library deleted during scan), silently return
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

			// Wait with exponential backoff
			time.Sleep(backoffDuration)

			// Reset to pending for retry
			if statusErr := uc.scanRepos.Checkpoint.UpdateStatus(ctx, checkpoint.ID, scanner.CheckpointPending, "", ""); statusErr != nil {
				// If the scan job was deleted (library deleted during scan), silently return
				if scanner.IsScanJobDeleted(statusErr) {
					return
				}
				uc.logger.Error("failed to reset checkpoint to pending for retry",
					"checkpoint_id", checkpoint.ID,
					"file_path", checkpoint.FilePath,
					"error", statusErr)
			}
		} else {
			// Either not transient or max retries exceeded - mark as failed
			if statusErr := uc.scanRepos.Checkpoint.UpdateStatus(ctx, checkpoint.ID, scanner.CheckpointFailed, err.Error(), category); statusErr != nil {
				// If the scan job was deleted (library deleted during scan), silently return
				if scanner.IsScanJobDeleted(statusErr) {
					return
				}
				uc.logger.Error("failed to mark checkpoint as failed",
					"checkpoint_id", checkpoint.ID,
					"file_path", checkpoint.FilePath,
					"error", statusErr)
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
	} else if hasWarning {
		// File processed successfully but with warnings (e.g., FFmpeg metadata extraction failed)
		// Mark as warning status so users can see which files had issues
		if statusErr := uc.scanRepos.Checkpoint.UpdateStatus(ctx, checkpoint.ID, scanner.CheckpointWarning, "metadata extraction incomplete", "ffmpeg"); statusErr != nil {
			// If the scan job was deleted (library deleted during scan), silently return
			if scanner.IsScanJobDeleted(statusErr) {
				return
			}
			uc.logger.Error("failed to mark checkpoint as warning",
				"checkpoint_id", checkpoint.ID,
				"file_path", checkpoint.FilePath,
				"error", statusErr)
		}
		// Thread-safe update of found files
		foundFilesMu.Lock()
		foundFiles[checkpoint.FilePath] = true
		foundFilesMu.Unlock()
	} else {
		// Mark as completed (no errors, no warnings)
		if statusErr := uc.scanRepos.Checkpoint.UpdateStatus(ctx, checkpoint.ID, scanner.CheckpointCompleted, "", ""); statusErr != nil {
			// If the scan job was deleted (library deleted during scan), silently return
			if scanner.IsScanJobDeleted(statusErr) {
				return
			}
			uc.logger.Error("failed to mark checkpoint as completed",
				"checkpoint_id", checkpoint.ID,
				"file_path", checkpoint.FilePath,
				"error", statusErr)
			// Continue anyway - file was processed successfully, just status update failed
		}
		// Thread-safe update of found files
		foundFilesMu.Lock()
		foundFiles[checkpoint.FilePath] = true
		foundFilesMu.Unlock()
	}
}