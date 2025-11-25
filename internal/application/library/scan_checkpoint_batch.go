package library

import (
	"context"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
)

// checkpointProcessingContext holds shared state for checkpoint processing
type checkpointProcessingContext struct {
	jobID              int64
	lib                *library.Library
	batchSize          int
	maxRetries         int
	existingMediaCache *sync.Map
	foundFilesMu       *sync.Mutex
	foundFiles         map[string]bool
	checkpointsChan    chan *scanner.ScanCheckpoint
	workerWg           *sync.WaitGroup
}

// processFilesWithCheckpoints processes files in batches using checkpoints with parallel workers
// hashingDone channel signals when checkpoint creation is complete
// discoveryStats contains statistics from the discovery phase (may be nil)
func (uc *ScanLibraryUseCase) processFilesWithCheckpoints(ctx context.Context, jobID int64, lib *library.Library, hashingDone <-chan struct{}, discoveryStats *filesystem.WalkStats) {
	pctx := uc.initCheckpointProcessing(ctx, jobID, lib)
	defer close(pctx.checkpointsChan)

	// Start worker pool
	uc.startCheckpointWorkers(ctx, pctx)

	// Run the main processing loop
	uc.runCheckpointProcessingLoop(ctx, pctx, hashingDone)

	// Wait for workers to finish
	pctx.workerWg.Wait()

	// Complete the scan
	uc.completeScan(ctx, pctx, discoveryStats)
}

// initCheckpointProcessing initializes the processing context and loads the media cache
func (uc *ScanLibraryUseCase) initCheckpointProcessing(ctx context.Context, jobID int64, lib *library.Library) *checkpointProcessingContext {
	// Load existing media cache
	existingMediaCache := uc.loadMediaCache(ctx, lib.ID)

	// Track found files
	var foundFilesMu sync.Mutex
	foundFiles := make(map[string]bool)

	// Create checkpoint channel
	checkpointsChan := make(chan *scanner.ScanCheckpoint, uc.config.CheckpointBufferSize)

	return &checkpointProcessingContext{
		jobID:              jobID,
		lib:                lib,
		batchSize:          uc.config.CheckpointBatchSize,
		maxRetries:         uc.config.MaxRetries,
		existingMediaCache: existingMediaCache,
		foundFilesMu:       &foundFilesMu,
		foundFiles:         foundFiles,
		checkpointsChan:    checkpointsChan,
		workerWg:           &sync.WaitGroup{},
	}
}

// loadMediaCache loads all existing media file paths into a sync.Map for fast lookup
func (uc *ScanLibraryUseCase) loadMediaCache(ctx context.Context, libraryID int64) *sync.Map {
	uc.logger.Info("loading existing media files into cache", "library_id", libraryID)

	existingMediaMap, err := uc.mediaRepos.Media.GetFilePathCache(ctx, libraryID)
	if err != nil {
		uc.logger.Warn("failed to load existing media cache, will fall back to per-file lookups",
			"library_id", libraryID,
			"error", err)
		existingMediaMap = make(map[string]int64)
	} else {
		uc.logger.Info("loaded existing media cache",
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

// startCheckpointWorkers starts the worker pool for processing checkpoints
func (uc *ScanLibraryUseCase) startCheckpointWorkers(ctx context.Context, pctx *checkpointProcessingContext) {
	numWorkers := uc.getNumWorkers()

	for w := 0; w < numWorkers; w++ {
		pctx.workerWg.Add(1)
		go func(workerID int) {
			defer pctx.workerWg.Done()
			defer uc.recoverWorkerPanic(pctx.jobID, workerID)

			for checkpoint := range pctx.checkpointsChan {
				uc.processCheckpointWorker(ctx, pctx.lib, checkpoint, pctx.maxRetries, pctx.foundFilesMu, pctx.foundFiles, pctx.existingMediaCache)
			}
		}(w)
	}
}

// getNumWorkers returns the number of processing workers based on system profile
func (uc *ScanLibraryUseCase) getNumWorkers() int {
	numWorkers := 4 // Conservative default
	if uc.systemProfile != nil {
		settings := uc.systemProfile.Calculate()
		numWorkers = settings.ProcessingWorkers
		uc.logger.Info("using parallel checkpoint processing",
			"workers", numWorkers,
			"storage_type", uc.systemProfile.Storage.Type)
	}
	return numWorkers
}

// runCheckpointProcessingLoop fetches checkpoint batches and feeds them to workers
func (uc *ScanLibraryUseCase) runCheckpointProcessingLoop(ctx context.Context, pctx *checkpointProcessingContext, hashingDone <-chan struct{}) {
	updateTicker := time.NewTicker(uc.config.ProgressUpdateTick)
	defer updateTicker.Stop()

	for {
		// Check if scan has been paused or deleted
		shouldBreak, shouldContinue := uc.checkScanStatus(ctx, pctx.jobID)
		if shouldBreak {
			return
		}
		if shouldContinue {
			continue
		}

		// Get next batch of pending files
		batch, err := uc.scanRepos.Checkpoint.GetPendingBatch(ctx, pctx.jobID, pctx.batchSize)
		if err != nil {
			if scanner.IsScanJobDeleted(err) {
				uc.logger.Info("Scan job deleted, stopping scan workers", "job_id", pctx.jobID)
			} else {
				uc.logger.Error("failed to get pending checkpoint batch", "job_id", pctx.jobID, "error", err)
			}
			return
		}

		// Check if we're done
		if len(batch) == 0 {
			select {
			case <-hashingDone:
				uc.logger.Debug("All checkpoints processed, scan complete", "job_id", pctx.jobID)
				return
			default:
				time.Sleep(uc.config.ProgressUpdateTick)
				continue
			}
		}

		// Send checkpoints to worker pool
		for _, checkpoint := range batch {
			pctx.checkpointsChan <- checkpoint
		}

		// Periodic progress update
		uc.updateProgressIfDue(ctx, pctx.jobID, updateTicker)
	}
}

// checkScanStatus checks if the scan has been paused or encountered an error
// Returns (shouldBreak, shouldContinue)
func (uc *ScanLibraryUseCase) checkScanStatus(ctx context.Context, jobID int64) (shouldBreak, shouldContinue bool) {
	currentJob, err := uc.scanRepos.ScanJob.GetByID(ctx, jobID)
	if err != nil {
		uc.logger.Error("failed to get scan job status", "job_id", jobID, "error", err)
		return true, false
	}
	if currentJob.Status == scanner.ScanStatusPaused {
		uc.logger.Info("Scan paused by user, stopping workers", "job_id", jobID)
		return true, false
	}
	return false, false
}

// updateProgressIfDue updates scan progress if the ticker has fired
func (uc *ScanLibraryUseCase) updateProgressIfDue(ctx context.Context, jobID int64, ticker *time.Ticker) {
	select {
	case <-ticker.C:
		stats, _ := uc.scanRepos.Checkpoint.GetStats(ctx, jobID)
		currentJob, err := uc.scanRepos.ScanJob.GetByID(ctx, jobID)
		if err == nil && currentJob != nil {
			progress := &scanner.Progress{
				FilesFound:     currentJob.FilesFound,
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

// completeScan finalizes the scan job with stats and cleanup
func (uc *ScanLibraryUseCase) completeScan(ctx context.Context, pctx *checkpointProcessingContext, discoveryStats *filesystem.WalkStats) {
	stats, _ := uc.scanRepos.Checkpoint.GetStats(ctx, pctx.jobID)

	currentJob, err := uc.scanRepos.ScanJob.GetByID(ctx, pctx.jobID)
	if err != nil {
		uc.logger.Error("failed to get current job for completion", "job_id", pctx.jobID, "error", err)
		return
	}

	job := uc.buildCompletedJob(pctx.jobID, currentJob, stats, discoveryStats)
	uc.logScanCompletion(pctx.jobID, pctx.lib.ID, currentJob.FilesFound, stats)

	if err := uc.scanRepos.ScanJob.Complete(ctx, job); err != nil {
		if scanner.IsScanJobDeleted(err) {
			uc.logger.Info("Scan job deleted before completion, exiting gracefully", "job_id", pctx.jobID)
			return
		}
		uc.logger.Error("failed to mark scan job as complete", "job_id", pctx.jobID, "error", err)
	}

	// Cleanup stale media only on fully successful scan
	if stats.FailedFiles == 0 && stats.CompletedFiles == stats.TotalFiles {
		if uc.imageRepo != nil && uc.imageCleanup != nil {
			uc.cleanupStaleMedia(ctx, pctx.lib.ID, pctx.foundFiles)
		}
	}

	// Clean up checkpoints
	uc.cleanupCheckpoints(ctx, pctx.jobID)
}

// buildCompletedJob creates the completed job record with all stats
func (uc *ScanLibraryUseCase) buildCompletedJob(jobID int64, currentJob *scanner.ScanJob, stats *scanner.CheckpointStats, discoveryStats *filesystem.WalkStats) *scanner.ScanJob {
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
		discoveryWarnings := uc.validateDiscovery(context.Background(), currentJob.LibraryID, currentJob.FilesFound, discoveryStats)

		job.DiscoveryErrors = discoveryStats.TotalErrors()
		job.DiscoveryWarnings = int64(len(discoveryWarnings))
		job.DirsScanned = discoveryStats.DirsScanned
		job.DirsSkipped = discoveryStats.DirsSkipped
		job.FilesSkipped = discoveryStats.FilesSkipped

		if len(discoveryWarnings) > 0 {
			uc.logger.Warn("Scan completed with discovery warnings",
				"job_id", jobID,
				"warning_count", len(discoveryWarnings),
				"warnings", discoveryWarnings)
		}
	}

	return job
}

// logScanCompletion logs the appropriate completion message based on results
func (uc *ScanLibraryUseCase) logScanCompletion(jobID, libraryID, filesFound int64, stats *scanner.CheckpointStats) {
	if stats.FailedFiles > 0 {
		uc.logger.Info("scan completed with errors",
			"job_id", jobID,
			"library_id", libraryID,
			"files_found", filesFound,
			"files_processed", stats.TotalFiles,
			"completed_files", stats.CompletedFiles,
			"failed_files", stats.FailedFiles,
			"warning_files", stats.WarningFiles)
	} else if stats.WarningFiles > 0 {
		uc.logger.Info("scan completed with warnings",
			"job_id", jobID,
			"library_id", libraryID,
			"files_found", filesFound,
			"files_processed", stats.TotalFiles,
			"completed_files", stats.CompletedFiles,
			"warning_files", stats.WarningFiles)
	} else {
		uc.logger.Info("scan completed successfully",
			"job_id", jobID,
			"library_id", libraryID,
			"files_found", filesFound,
			"files_processed", stats.TotalFiles,
			"completed_files", stats.CompletedFiles)
	}
}

// cleanupCheckpoints removes checkpoints after scan completion
func (uc *ScanLibraryUseCase) cleanupCheckpoints(ctx context.Context, jobID int64) {
	if err := uc.scanRepos.Checkpoint.DeleteByJobID(ctx, jobID); err != nil {
		uc.logger.Warn("failed to cleanup checkpoints", "job_id", jobID, "error", err)
	} else {
		uc.logger.Info("cleaned up checkpoints for completed scan", "job_id", jobID)
	}
}
