package library

import (
	"context"
	"runtime/debug"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
)

// processFilesWithCheckpoints processes files in batches using checkpoints with parallel workers
// hashingDone channel signals when checkpoint creation is complete
// discoveryStats contains statistics from the discovery phase (may be nil)
func (uc *ScanLibraryUseCase) processFilesWithCheckpoints(ctx context.Context, jobID int64, lib *library.Library, hashingDone <-chan struct{}, discoveryStats *filesystem.WalkStats) {
	const batchSize = 50
	const maxRetries = 3
	updateTicker := time.NewTicker(2 * time.Second)
	defer updateTicker.Stop()

	// Determine number of concurrent workers from system profile
	numWorkers := 4 // Conservative default
	if uc.systemProfile != nil {
		settings := uc.systemProfile.Calculate()
		numWorkers = settings.ProcessingWorkers
		uc.logger.Info("using parallel checkpoint processing",
			"workers", numWorkers,
			"storage_type", uc.systemProfile.Storage.Type)
	}

	// Load all existing media file paths into memory (major optimization!)
	// This eliminates 58K+ individual SELECT queries (one per file)
	// Use sync.Map for thread-safe concurrent access by workers
	uc.logger.Info("loading existing media files into cache", "library_id", lib.ID)
	existingMediaMap, err := uc.mediaRepos.Media.GetFilePathCache(ctx, lib.ID)
	if err != nil {
		uc.logger.Warn("failed to load existing media cache, will fall back to per-file lookups",
			"library_id", lib.ID,
			"error", err)
		existingMediaMap = make(map[string]int64) // Empty cache if load fails
	} else {
		uc.logger.Info("loaded existing media cache",
			"library_id", lib.ID,
			"count", len(existingMediaMap))
	}

	// Convert to sync.Map for thread-safe concurrent access
	// Workers will read from this cache AND update it when creating new media
	var existingMediaCache sync.Map
	for path, id := range existingMediaMap {
		existingMediaCache.Store(path, id)
	}

	// Track found files (thread-safe with mutex)
	var foundFilesMu sync.Mutex
	foundFiles := make(map[string]bool)

	// Worker pool: checkpoints channel -> workers -> completion
	checkpointsChan := make(chan *scanner.ScanCheckpoint, batchSize*2)
	var workerWg sync.WaitGroup

	// Start worker goroutines
	for w := 0; w < numWorkers; w++ {
		workerWg.Add(1)
		go func(workerID int) {
			defer workerWg.Done()

			// Recover from panics to prevent leaving checkpoints in "processing" state
			defer func() {
				if r := recover(); r != nil {
					uc.logger.Error("PANIC: worker goroutine panicked",
						"worker_id", workerID,
						"job_id", jobID,
						"panic", r,
						"stack_trace", string(debug.Stack()))
				}
			}()

			for checkpoint := range checkpointsChan {
				uc.processCheckpointWorker(ctx, lib, checkpoint, maxRetries, &foundFilesMu, foundFiles, &existingMediaCache)
			}
		}(w)
	}

	// Main loop: fetch batches and feed to workers
processingLoop:
	for {
		// Check if scan has been paused
		currentJob, err := uc.scanRepos.ScanJob.GetByID(ctx, jobID)
		if err != nil {
			uc.logger.Error("failed to get scan job status",
				"job_id", jobID,
				"error", err)
			break
		}
		if currentJob.Status == scanner.ScanStatusPaused {
			uc.logger.Info("Scan paused by user, stopping workers",
				"job_id", jobID)
			break
		}

		// Get next batch of pending files
		batch, err := uc.scanRepos.Checkpoint.GetPendingBatch(ctx, jobID, batchSize)
		if err != nil {
			// If the scan job was deleted (library deleted during scan), silently exit
			if scanner.IsScanJobDeleted(err) {
				uc.logger.Info("Scan job deleted, stopping scan workers",
					"job_id", jobID)
				break
			}
			uc.logger.Error("failed to get pending checkpoint batch",
				"job_id", jobID,
				"error", err)
			break
		}

		if len(batch) == 0 {
			// Check if hashing is complete
			select {
			case <-hashingDone:
				// Hashing is done and no more checkpoints - we're finished
				uc.logger.Debug("All checkpoints processed, scan complete",
					"job_id", jobID)
				break processingLoop
			default:
				// Hashing still in progress, wait for more checkpoints
				time.Sleep(2 * time.Second)
				continue
			}
		}

		// Send checkpoints to worker pool
		for _, checkpoint := range batch {
			checkpointsChan <- checkpoint
		}

		// Periodic progress update
		select {
		case <-updateTicker.C:
			stats, _ := uc.scanRepos.Checkpoint.GetStats(ctx, jobID)
			// Get current job to preserve phase, discovery state, and FilesFound
			currentJob, err := uc.scanRepos.ScanJob.GetByID(ctx, jobID)
			if err == nil && currentJob != nil {
				progress := &scanner.Progress{
					FilesFound:     currentJob.FilesFound, // Preserve FilesFound from discovery
					FilesProcessed: stats.ProcessedFiles,
					ErrorCount:     stats.FailedFiles,
					WarningCount:   stats.WarningFiles,
					Phase:          currentJob.Phase,
					EstimatedTotal: currentJob.EstimatedTotal,
					DiscoveryDone:  currentJob.DiscoveryDone,
				}
				_ = uc.scanRepos.ScanJob.UpdateProgress(ctx, jobID, progress)
			}
		default:
		}
	}

	// Close checkpoint channel and wait for all workers to finish
	close(checkpointsChan)
	workerWg.Wait()

	// Get final stats from checkpoints
	stats, _ := uc.scanRepos.Checkpoint.GetStats(ctx, jobID)

	// Get current job to preserve FilesFound from discovery phase
	// CRITICAL: FilesFound is set during discovery and represents ALL discovered files,
	// while stats.TotalFiles only counts files that needed processing (new/modified).
	// We must ALWAYS use currentJob.FilesFound, never stats.TotalFiles!
	currentJob, err := uc.scanRepos.ScanJob.GetByID(ctx, jobID)
	if err != nil {
		uc.logger.Error("failed to get current job for completion",
			"job_id", jobID,
			"error", err)
		return
	}

	// Complete the job with discovery health metrics
	job := &scanner.ScanJob{
		ID:             jobID,
		FilesFound:     currentJob.FilesFound,   // MUST preserve from discovery (not stats.TotalFiles!)
		FilesProcessed: stats.ProcessedFiles,    // Total processed (completed + failed + warnings)
		ErrorCount:     stats.FailedFiles,
		WarningCount:   stats.WarningFiles,
		Progress:       stats.GetProgress(),
		CompletedAt:    &[]time.Time{time.Now()}[0],
		Phase:          scanner.ScanPhaseCompleted,
		DiscoveryDone:  true,
	}

	// Add discovery health metrics if available
	if discoveryStats != nil {
		// Validate discovery and count warnings
		// Use FilesFound (discovered files) not stats.TotalFiles (checkpoint count)
		discoveryWarnings := uc.validateDiscovery(ctx, lib.ID, currentJob.FilesFound, discoveryStats)

		job.DiscoveryErrors = discoveryStats.TotalErrors()
		job.DiscoveryWarnings = int64(len(discoveryWarnings))
		job.DirsScanned = discoveryStats.DirsScanned
		job.DirsSkipped = discoveryStats.DirsSkipped
		job.FilesSkipped = discoveryStats.FilesSkipped

		// Log warnings at completion time as well
		if len(discoveryWarnings) > 0 {
			uc.logger.Warn("Scan completed with discovery warnings",
				"job_id", jobID,
				"warning_count", len(discoveryWarnings),
				"warnings", discoveryWarnings)
		}
	}

	if stats.FailedFiles > 0 {
		job.Status = scanner.ScanStatusCompleted // Partial success
		uc.logger.Info("scan completed with errors",
			"job_id", jobID,
			"library_id", lib.ID,
			"files_found", currentJob.FilesFound,
			"files_processed", stats.TotalFiles,
			"completed_files", stats.CompletedFiles,
			"failed_files", stats.FailedFiles,
			"warning_files", stats.WarningFiles)
	} else if stats.WarningFiles > 0 {
		job.Status = scanner.ScanStatusCompleted
		uc.logger.Info("scan completed with warnings",
			"job_id", jobID,
			"library_id", lib.ID,
			"files_found", currentJob.FilesFound,
			"files_processed", stats.TotalFiles,
			"completed_files", stats.CompletedFiles,
			"warning_files", stats.WarningFiles)
	} else {
		job.Status = scanner.ScanStatusCompleted
		uc.logger.Info("scan completed successfully",
			"job_id", jobID,
			"library_id", lib.ID,
			"files_found", currentJob.FilesFound,
			"files_processed", stats.TotalFiles,
			"completed_files", stats.CompletedFiles)
	}

	if err := uc.scanRepos.ScanJob.Complete(ctx, job); err != nil {
		// If the scan job was deleted (library deleted during scan), silently return
		if scanner.IsScanJobDeleted(err) {
			uc.logger.Info("Scan job deleted before completion, exiting gracefully",
				"job_id", jobID)
			return
		}
		uc.logger.Error("failed to mark scan job as complete",
			"job_id", jobID,
			"error", err)
	}

	// Cleanup stale media only on fully successful scan
	if stats.FailedFiles == 0 && stats.CompletedFiles == stats.TotalFiles {
		if uc.imageRepo != nil && uc.imageCleanup != nil {
			uc.cleanupStaleMedia(ctx, lib.ID, foundFiles)
		}
	}

	// Clean up checkpoints after scan completion to prevent database bloat
	// This is safe to do even if the scan failed, as we can always re-scan from scratch
	if err := uc.scanRepos.Checkpoint.DeleteByJobID(ctx, jobID); err != nil {
		uc.logger.Warn("failed to cleanup checkpoints",
			"job_id", jobID,
			"error", err)
		// Non-fatal - just log the warning
	} else {
		uc.logger.Info("cleaned up checkpoints for completed scan",
			"job_id", jobID)
	}
}