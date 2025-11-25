package library

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
)

// discoveryContext holds state shared across discovery phases
type discoveryContext struct {
	jobID          int64
	lib            *library.Library
	currentJob     *scanner.ScanJob
	walker         *filesystem.Walker
	discoveryStats *filesystem.WalkStats
}

// runFreshScan discovers all files and creates checkpoints before processing.
// Uses incremental scanning to only process new/modified/deleted files.
// Phases: count → walk → diff → delete → hash+process
func (uc *ScanLibraryUseCase) runFreshScan(ctx context.Context, jobID int64, lib *library.Library) {
	// Initialize discovery context
	dctx, err := uc.initDiscoveryContext(ctx, jobID, lib)
	if err != nil {
		return
	}

	// Phase 1: Count files for progress estimation
	uc.phaseCountFiles(ctx, dctx)

	// Phase 2: Walk directory and discover media files
	discoveredFiles, err := uc.phaseWalkDirectory(ctx, dctx)
	if err != nil {
		uc.completeJobWithError(ctx, jobID, err)
		return
	}

	// Phase 3: Determine changes (incremental diff)
	diff := uc.phaseDetermineChanges(ctx, dctx, discoveredFiles)
	if diff == nil {
		return // No changes, job already marked complete
	}

	// Phase 4: Handle deleted files
	uc.phaseHandleDeleted(ctx, dctx, diff)

	// Phase 5: Hash and process files concurrently
	uc.phaseHashAndProcess(ctx, dctx, diff)
}

// initDiscoveryContext sets up the discovery context and walker
func (uc *ScanLibraryUseCase) initDiscoveryContext(ctx context.Context, jobID int64, lib *library.Library) (*discoveryContext, error) {
	currentJob, err := uc.scanRepos.ScanJob.GetByID(ctx, jobID)
	if err != nil {
		uc.logger.Error("Failed to get job for progress callback",
			"job_id", jobID,
			"error", err)
		return nil, err
	}

	walker := uc.createWalker()

	return &discoveryContext{
		jobID:      jobID,
		lib:        lib,
		currentJob: currentJob,
		walker:     walker,
	}, nil
}

// createWalker creates a filesystem walker with optimal settings
func (uc *ScanLibraryUseCase) createWalker() *filesystem.Walker {
	var walkerOpts []filesystem.WalkerOption

	// Use profiler recommendations if available, otherwise use config
	if uc.systemProfile != nil {
		recommendations := uc.systemProfile.Calculate()
		if recommendations.ScanWalkers > 0 {
			walkerOpts = append(walkerOpts, filesystem.WithParallelWalking(recommendations.ScanWalkers))
			uc.logger.Info("using parallel directory walking",
				"workers", recommendations.ScanWalkers,
				"storage_type", uc.systemProfile.Storage.Type)
		}
	} else if uc.config.ParallelWalkers > 0 {
		walkerOpts = append(walkerOpts, filesystem.WithParallelWalking(uc.config.ParallelWalkers))
	}

	if uc.config.ProgressInterval > 0 {
		walkerOpts = append(walkerOpts, filesystem.WithProgressLogging(uc.config.ProgressInterval))
	}

	walkerOpts = append(walkerOpts, filesystem.WithLogger(uc.logger))

	return filesystem.NewWalker(walkerOpts...)
}

// phaseCountFiles performs a fast count pass to get accurate total for progress reporting
func (uc *ScanLibraryUseCase) phaseCountFiles(ctx context.Context, dctx *discoveryContext) {
	uc.logger.Info("Counting media files", "library_id", dctx.lib.ID, "path", dctx.lib.Path)

	totalFiles, err := dctx.walker.Count(ctx, dctx.lib.Path, func(fi scanner.FileInfo) bool {
		return !fi.IsDir && uc.isMediaFile(fi.Extension)
	})

	if err != nil {
		uc.logger.Warn("File count failed, proceeding with discovery",
			"library_id", dctx.lib.ID,
			"error", err)
		return
	}

	uc.logger.Info("File count completed",
		"library_id", dctx.lib.ID,
		"total_files", totalFiles)

	// Update job with estimated total
	countProgress := &scanner.Progress{
		FilesFound:     totalFiles,
		FilesProcessed: 0,
		LastUpdate:     time.Now(),
		Phase:          scanner.ScanPhaseDiscovering,
		EstimatedTotal: totalFiles,
		DiscoveryDone:  true,
	}
	if err := uc.scanRepos.ScanJob.UpdateProgress(ctx, dctx.jobID, countProgress); err != nil {
		uc.logger.Warn("Failed to update count progress",
			"job_id", dctx.jobID,
			"error", err)
	}
}

// phaseWalkDirectory walks the directory tree and collects media files
func (uc *ScanLibraryUseCase) phaseWalkDirectory(ctx context.Context, dctx *discoveryContext) ([]scanner.FileInfo, error) {
	var mediaFilesDiscovered atomic.Int64

	// Progress callback for real-time updates
	progressCallback := func(filesDiscovered int64) {
		progress := &scanner.Progress{
			FilesFound:     filesDiscovered,
			FilesProcessed: 0,
			LastUpdate:     time.Now(),
			Phase:          scanner.ScanPhaseDiscovering,
			EstimatedTotal: dctx.currentJob.EstimatedTotal,
			DiscoveryDone:  false,
		}
		go func() {
			if err := uc.scanRepos.ScanJob.UpdateProgress(ctx, dctx.jobID, progress); err != nil {
				uc.logger.Warn("Failed to update discovery progress",
					"job_id", dctx.jobID,
					"files_discovered", filesDiscovered,
					"error", err)
			}
		}()
	}

	// Collect files via channel
	filesChan := make(chan scanner.FileInfo, uc.config.DiscoveryBufferSize)
	var discoveryWg sync.WaitGroup
	discoveryWg.Add(1)

	var discoveredFiles []scanner.FileInfo
	go func() {
		defer discoveryWg.Done()
		for fileInfo := range filesChan {
			discoveredFiles = append(discoveredFiles, fileInfo)
		}
	}()

	// Walk directory tree
	err := dctx.walker.Walk(ctx, dctx.lib.Path, func(fileInfo scanner.FileInfo) error {
		if fileInfo.IsDir || !uc.isMediaFile(fileInfo.Extension) {
			return nil
		}

		filesChan <- fileInfo

		count := mediaFilesDiscovered.Add(1)
		if count%int64(uc.config.DiscoveryLogEvery) == 0 {
			progressCallback(count)
		}

		return nil
	})

	close(filesChan)
	discoveryWg.Wait()

	// Report final count
	if finalCount := mediaFilesDiscovered.Load(); finalCount > 0 {
		progressCallback(finalCount)
	}

	// Capture and log discovery statistics
	dctx.discoveryStats = dctx.walker.GetStats()
	uc.logDiscoveryStats(dctx.jobID, dctx.discoveryStats)

	if err != nil {
		uc.logger.Error("File discovery failed",
			"job_id", dctx.jobID,
			"library_id", dctx.lib.ID,
			"error", err)
		return nil, err
	}

	uc.logger.Info("File discovery completed",
		"job_id", dctx.jobID,
		"media_files_discovered", len(discoveredFiles))

	// Validate and log warnings
	warnings := uc.validateDiscovery(ctx, dctx.lib.ID, int64(len(discoveredFiles)), dctx.discoveryStats)
	for _, warning := range warnings {
		uc.logger.Warn("Discovery validation warning", "job_id", dctx.jobID, "warning", warning)
	}

	// Mark discovery complete
	discoveryProgress := &scanner.Progress{
		FilesFound:     int64(len(discoveredFiles)),
		FilesProcessed: 0,
		LastUpdate:     time.Now(),
		Phase:          scanner.ScanPhaseProcessing,
		EstimatedTotal: dctx.currentJob.EstimatedTotal,
		DiscoveryDone:  true,
	}
	if err := uc.scanRepos.ScanJob.UpdateProgress(ctx, dctx.jobID, discoveryProgress); err != nil {
		uc.logger.Warn("Failed to update discovery completion status",
			"job_id", dctx.jobID,
			"error", err)
	}

	return discoveredFiles, nil
}

// logDiscoveryStats logs statistics from the directory walk
func (uc *ScanLibraryUseCase) logDiscoveryStats(jobID int64, stats *filesystem.WalkStats) {
	if stats == nil {
		return
	}

	uc.logger.Info("Discovery statistics",
		"job_id", jobID,
		"files_discovered", stats.FilesDiscovered,
		"dirs_scanned", stats.DirsScanned,
		"dirs_skipped", stats.DirsSkipped,
		"files_skipped", stats.FilesSkipped,
		"permission_errors", stats.PermissionErrors,
		"network_errors", stats.NetworkErrors,
		"other_errors", stats.OtherErrors)

	if stats.HasErrors() {
		uc.logger.Warn("Discovery completed with errors - some files may be missing",
			"job_id", jobID,
			"total_errors", stats.TotalErrors(),
			"dirs_skipped", stats.DirsSkipped,
			"files_skipped", stats.FilesSkipped,
			"sample_paths", stats.SkippedPaths)
	}
}

// phaseDetermineChanges computes the diff between current and previous scan.
// Returns nil if no changes need processing (job marked complete).
func (uc *ScanLibraryUseCase) phaseDetermineChanges(ctx context.Context, dctx *discoveryContext, discoveredFiles []scanner.FileInfo) *scanner.ScanDiff {
	diff, err := uc.incrementalScanner.DetermineChanges(ctx, dctx.lib.ID, discoveredFiles)
	if err != nil {
		uc.logger.Warn("incremental scan failed, falling back to full scan",
			"library_id", dctx.lib.ID,
			"error", err)
		diff = &scanner.ScanDiff{
			NewFiles:       discoveredFiles,
			ModifiedFiles:  []scanner.FileInfo{},
			DeletedFiles:   []string{},
			UnchangedFiles: []string{},
		}
	} else {
		uc.logger.Info("incremental scan completed",
			"library_id", dctx.lib.ID,
			"diff", diff.Summary())
	}

	// No changes - mark complete and return nil
	if !diff.NeedsProcessing() {
		uc.logger.Info("no changes detected, scan complete",
			"files_found", len(discoveredFiles),
			"unchanged_files", len(diff.UnchangedFiles))

		job := &scanner.ScanJob{
			ID:             dctx.jobID,
			Status:         scanner.ScanStatusCompleted,
			FilesFound:     int64(len(discoveredFiles)),
			FilesProcessed: 0,
			ErrorCount:     0,
			Progress:       100.0,
			CompletedAt:    &[]time.Time{time.Now()}[0],
			Phase:          scanner.ScanPhaseCompleted,
			DiscoveryDone:  true,
		}
		if err := uc.scanRepos.ScanJob.Complete(ctx, job); err != nil {
			uc.logger.Error("failed to mark scan job as complete", "job_id", dctx.jobID, "error", err)
		}
		return nil
	}

	return diff
}

// phaseHandleDeleted removes scan state for files that no longer exist
func (uc *ScanLibraryUseCase) phaseHandleDeleted(ctx context.Context, dctx *discoveryContext, diff *scanner.ScanDiff) {
	if len(diff.DeletedFiles) == 0 {
		return
	}

	uc.logger.Info("removing deleted files from scan state",
		"count", len(diff.DeletedFiles))

	if err := uc.scanRepos.ScanState.DeleteByPaths(ctx, dctx.lib.ID, diff.DeletedFiles); err != nil {
		uc.logger.Error("failed to delete scan state for removed files",
			"library_id", dctx.lib.ID,
			"count", len(diff.DeletedFiles),
			"error", err)
	}
}

// phaseHashAndProcess hashes files and processes them concurrently
func (uc *ScanLibraryUseCase) phaseHashAndProcess(ctx context.Context, dctx *discoveryContext, diff *scanner.ScanDiff) {
	filesToProcess := append(diff.NewFiles, diff.ModifiedFiles...)

	uc.logger.Info("processing files",
		"total", len(filesToProcess),
		"new", len(diff.NewFiles),
		"modified", len(diff.ModifiedFiles),
		"skipped", len(diff.UnchangedFiles))

	startTime := time.Now()

	// Start processing goroutine - consumes checkpoints as they're created
	var processingWg sync.WaitGroup
	processingWg.Add(1)
	processingErrChan := make(chan error, 1)
	hashingDone := make(chan struct{})

	go func() {
		defer processingWg.Done()
		defer uc.recoverFromPanicWithError(dctx.jobID, dctx.lib.ID, "processing goroutine panicked", processingErrChan)
		uc.processFilesWithCheckpoints(ctx, dctx.jobID, dctx.lib, hashingDone, dctx.discoveryStats)
	}()

	// Hash files and stream checkpoints
	if err := uc.hashAndStreamCheckpoints(ctx, filesToProcess, dctx.jobID, dctx.lib.ID); err != nil {
		uc.logger.Error("failed to create checkpoints", "error", err)
		close(hashingDone)
		uc.completeJobWithError(ctx, dctx.jobID, err)
		return
	}

	close(hashingDone)
	processingWg.Wait()

	// Check for processing errors
	select {
	case err := <-processingErrChan:
		if err != nil {
			uc.logger.Error("processing failed", "error", err)
			uc.completeJobWithError(ctx, dctx.jobID, err)
			return
		}
	default:
	}

	duration := time.Since(startTime)
	avgMs := float64(duration.Milliseconds()) / float64(len(filesToProcess))
	uc.logger.Info("concurrent hashing and processing completed",
		"file_count", len(filesToProcess),
		"duration", duration,
		"avg_ms_per_file", avgMs)
}