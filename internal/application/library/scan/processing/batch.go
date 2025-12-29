package processing

import (
	"context"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/events"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
)

// LoadMediaCache loads all existing media file paths into a sync.Map for fast lookup.
func LoadMediaCache(ctx context.Context, deps *Deps, libraryID int64) *sync.Map {
	deps.Logger.Info("loading existing media files into cache", "library_id", libraryID)

	existingMediaMap, err := deps.MediaRepos.Media.GetFilePathCache(ctx, libraryID)
	if err != nil {
		deps.Logger.Warn("failed to load existing media cache, will fall back to per-file lookups",
			"library_id", libraryID,
			"error", err)
		existingMediaMap = make(map[string]int64)
	} else {
		deps.Logger.Info("loaded existing media cache",
			"library_id", libraryID,
			"count", len(existingMediaMap))
	}

	// Convert to sync.Map for thread-safe concurrent access
	var cache sync.Map
	for path, id := range existingMediaMap {
		cache.Store(path, id)
	}

	return &cache
}

// InitCheckpointProcessing initializes the processing context and loads the media cache.
func InitCheckpointProcessing(ctx context.Context, deps *Deps, jobID int64, lib *library.Library) *CheckpointContext {
	// Load existing media cache
	existingMediaCache := LoadMediaCache(ctx, deps, lib.ID)

	return NewCheckpointContext(
		jobID,
		lib,
		deps.Config.CheckpointBatchSize,
		deps.Config.MaxRetries,
		deps.Config.CheckpointBufferSize,
		existingMediaCache,
	)
}

// StartCheckpointWorkers starts the worker pool for processing checkpoints.
func StartCheckpointWorkers(ctx context.Context, deps *Deps, pctx *CheckpointContext) {
	numWorkers := GetNumWorkers(deps.SystemProfile, scan.DefaultProcessingWorkers, deps.Logger)

	pool := &WorkerPool[*scanner.ScanCheckpoint, struct{}]{
		NumWorkers: numWorkers,
		Input:      pctx.CheckpointsChan,
		Process: func(workerID int, checkpoint *scanner.ScanCheckpoint) {
			ProcessCheckpointWorker(ctx, deps, pctx.Lib, checkpoint, pctx.MaxRetries, pctx.FoundFilesMu, pctx.FoundFiles, pctx.ExistingMediaCache)
		},
		OnPanic: func(workerID int, checkpoint *scanner.ScanCheckpoint, recovered any) struct{} {
			info := NewPanicInfo(workerID, recovered)
			deps.Logger.Error("panic in checkpoint worker",
				"job_id", pctx.JobID,
				"worker_id", info.WorkerID,
				"file_path", checkpoint.FilePath,
				"panic", info.Recovered,
				"stack", info.Stack)
			return struct{}{}
		},
	}

	// Run in goroutine so we can wait on it via WaitGroup
	pctx.WorkerWg.Add(1)
	go func() {
		defer pctx.WorkerWg.Done()
		pool.Run()
	}()
}

// CheckScanStatus checks if the scan has been paused or encountered an error.
// Returns (shouldBreak, shouldContinue).
func CheckScanStatus(ctx context.Context, deps *Deps, jobID int64) (shouldBreak, shouldContinue bool) {
	currentJob, err := deps.ScanRepos.ScanJob.GetByID(ctx, jobID)
	if err != nil {
		deps.Logger.Error("failed to get scan job status", "job_id", jobID, "error", err)
		return true, false
	}
	if currentJob.Status == scanner.ScanStatusPaused {
		deps.Logger.Info("Scan paused by user, stopping workers", "job_id", jobID)
		return true, false
	}
	return false, false
}

// RunCheckpointProcessingLoop fetches checkpoint batches and feeds them to workers.
func RunCheckpointProcessingLoop(ctx context.Context, deps *Deps, pctx *CheckpointContext, hashingDone <-chan struct{}) {
	updateTicker := time.NewTicker(deps.Config.ProgressUpdateTick)
	defer updateTicker.Stop()

	for {
		// Check if scan has been paused or deleted
		shouldBreak, shouldContinue := CheckScanStatus(ctx, deps, pctx.JobID)
		if shouldBreak {
			return
		}
		if shouldContinue {
			continue
		}

		// Get next batch of pending files
		batch, err := deps.ScanRepos.Checkpoint.GetPendingBatch(ctx, pctx.JobID, pctx.BatchSize)
		if err != nil {
			if scanner.IsScanJobDeleted(err) {
				deps.Logger.Info("Scan job deleted, stopping scan workers", "job_id", pctx.JobID)
			} else {
				deps.Logger.Error("failed to get pending checkpoint batch", "job_id", pctx.JobID, "error", err)
			}
			return
		}

		// Check if we're done
		if len(batch) == 0 {
			select {
			case <-hashingDone:
				// Hashing is done, but we need to verify all checkpoints are actually processed.
				// Workers may still be processing claimed checkpoints that aren't "pending" anymore.
				// A scan is complete when: total == processed (completed + failed + warning)
				// OR when there were no files to process at all (TotalFiles == 0 or stats nil)
				stats, _ := deps.ScanRepos.Checkpoint.GetStats(ctx, pctx.JobID)
				if stats == nil || stats.TotalFiles == 0 || stats.ProcessedFiles >= stats.TotalFiles {
					var total, processed int64
					if stats != nil {
						total, processed = stats.TotalFiles, stats.ProcessedFiles
					}
					deps.Logger.Debug("All checkpoints processed, scan complete",
						"job_id", pctx.JobID,
						"total", total,
						"processed", processed)
					return
				}
				// Still have checkpoints being processed, wait for workers to finish
				time.Sleep(deps.Config.ProgressUpdateTick)
				continue
			default:
				time.Sleep(deps.Config.ProgressUpdateTick)
				continue
			}
		}

		// Send checkpoints to worker pool
		for _, checkpoint := range batch {
			pctx.CheckpointsChan <- checkpoint
		}

		// Periodic progress update
		updateProgressIfDue(ctx, deps, pctx.JobID, pctx.Lib.ID, updateTicker)
	}
}

// updateProgressIfDue updates scan progress if the ticker has fired.
func updateProgressIfDue(ctx context.Context, deps *Deps, jobID int64, libraryID int64, ticker *time.Ticker) {
	select {
	case <-ticker.C:
		stats, _ := deps.ScanRepos.Checkpoint.GetStats(ctx, jobID)
		currentJob, err := deps.ScanRepos.ScanJob.GetByID(ctx, jobID)
		if err == nil && currentJob != nil {
			progress := &scanner.Progress{
				FilesFound:     currentJob.FilesFound,
				FilesProcessed: stats.ProcessedFiles,
				ErrorCount:     stats.FailedFiles,
				WarningCount:   stats.WarningFiles,
				Phase:          scanner.ScanPhaseProcessing,
				EstimatedTotal: currentJob.EstimatedTotal,
				DiscoveryDone:  currentJob.DiscoveryDone,
			}
			_ = deps.ScanRepos.ScanJob.UpdateProgress(ctx, jobID, progress)

			// Publish SSE event for real-time UI updates
			publishProgressEvent(deps, jobID, libraryID, currentJob, stats)
		}
	default:
	}
}

// publishProgressEvent publishes a scan.progress event for SSE clients.
func publishProgressEvent(deps *Deps, jobID int64, libraryID int64, job *scanner.ScanJob, stats *scanner.CheckpointStats) {
	if deps.EventBus == nil {
		return
	}

	var progressPercent float64
	if job.FilesFound > 0 {
		progressPercent = float64(stats.ProcessedFiles) / float64(job.FilesFound) * 100
	}

	deps.EventBus.Publish(events.NewEvent(events.EventScanProgress, "scanner").
		WithLibraryID(libraryID).
		WithData("job_id", jobID).
		WithData("status", "running").
		WithData("phase", string(scanner.ScanPhaseProcessing)).
		WithData("progress", progressPercent).
		WithData("files_found", job.FilesFound).
		WithData("files_processed", stats.ProcessedFiles).
		WithData("error_count", stats.FailedFiles).
		WithData("warning_count", stats.WarningFiles).
		WithData("estimated_total", job.EstimatedTotal).
		WithData("discovery_done", job.DiscoveryDone).
		Build())
}

// BuildCompletedJob creates the completed job record with all stats.
func BuildCompletedJob(
	jobID int64,
	currentJob *scanner.ScanJob,
	stats *scanner.CheckpointStats,
	discoveryStats *filesystem.WalkStats,
) *scanner.ScanJob {
	job := &scanner.ScanJob{
		ID:             jobID,
		Status:         scanner.ScanStatusCompleted,
		FilesFound:     currentJob.FilesFound,
		FilesProcessed: stats.ProcessedFiles,
		ErrorCount:     stats.FailedFiles,
		WarningCount:   stats.WarningFiles,
		Progress:       stats.GetProgress(),
		CompletedAt:    &[]time.Time{time.Now()}[0],
		Phase:          scanner.ScanPhaseCompleted,
		DiscoveryDone:  true,
	}

	if discoveryStats != nil {
		job.DiscoveryErrors = discoveryStats.TotalErrors()
		job.DirsScanned = discoveryStats.DirsScanned
		job.DirsSkipped = discoveryStats.DirsSkipped
		job.FilesSkipped = discoveryStats.FilesSkipped
	}

	return job
}

// LogScanCompletion logs the appropriate completion message based on results.
func LogScanCompletion(deps *Deps, jobID, libraryID, filesFound int64, stats *scanner.CheckpointStats) {
	if stats.FailedFiles > 0 {
		deps.Logger.Info("scan completed with errors",
			"job_id", jobID,
			"library_id", libraryID,
			"files_found", filesFound,
			"files_processed", stats.TotalFiles,
			"completed_files", stats.CompletedFiles,
			"failed_files", stats.FailedFiles,
			"warning_files", stats.WarningFiles)
	} else if stats.WarningFiles > 0 {
		deps.Logger.Info("scan completed with warnings",
			"job_id", jobID,
			"library_id", libraryID,
			"files_found", filesFound,
			"files_processed", stats.TotalFiles,
			"completed_files", stats.CompletedFiles,
			"warning_files", stats.WarningFiles)
	} else {
		deps.Logger.Info("scan completed successfully",
			"job_id", jobID,
			"library_id", libraryID,
			"files_found", filesFound,
			"files_processed", stats.TotalFiles,
			"completed_files", stats.CompletedFiles)
	}
}

// CleanupCheckpoints removes checkpoints after scan completion.
func CleanupCheckpoints(ctx context.Context, deps *Deps, jobID int64) {
	if err := deps.ScanRepos.Checkpoint.DeleteByJobID(ctx, jobID); err != nil {
		deps.Logger.Warn("failed to cleanup checkpoints", "job_id", jobID, "error", err)
	} else {
		deps.Logger.Info("cleaned up checkpoints for completed scan", "job_id", jobID)
	}
}

// CompleteScan finalizes a scan job after all processing is done.
// The cleanupFn is called if the scan completed without errors and all files were processed.
func CompleteScan(ctx context.Context, deps *Deps, pctx *CheckpointContext, discoveryStats *filesystem.WalkStats, cleanupFn func()) {
	stats, _ := deps.ScanRepos.Checkpoint.GetStats(ctx, pctx.JobID)

	currentJob, err := deps.ScanRepos.ScanJob.GetByID(ctx, pctx.JobID)
	if err != nil {
		deps.Logger.Error("failed to get current job for completion", "job_id", pctx.JobID, "error", err)
		return
	}

	job := BuildCompletedJob(pctx.JobID, currentJob, stats, discoveryStats)
	LogScanCompletion(deps, pctx.JobID, pctx.Lib.ID, currentJob.FilesFound, stats)

	if err := deps.ScanRepos.ScanJob.Complete(ctx, job); err != nil {
		if scanner.IsScanJobDeleted(err) {
			deps.Logger.Info("Scan job deleted before completion, exiting gracefully", "job_id", pctx.JobID)
			return
		}
		deps.Logger.Error("failed to mark scan job as complete", "job_id", pctx.JobID, "error", err)
	}

	if stats.FailedFiles == 0 && stats.CompletedFiles == stats.TotalFiles && cleanupFn != nil {
		cleanupFn()
	}

	CleanupCheckpoints(ctx, deps, pctx.JobID)
}
