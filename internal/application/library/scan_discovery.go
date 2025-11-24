package library

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
)

// runFreshScan discovers all files and creates checkpoints before processing
// Uses incremental scanning to only process new/modified/deleted files
func (uc *ScanLibraryUseCase) runFreshScan(ctx context.Context, jobID int64, lib *library.Library) {
	// Phase 1: File Discovery - walk directory to collect file paths with metadata
	// Configure walker with optimal settings for this system
	var walkerOpts []filesystem.WalkerOption

	// Use profiler recommendations if available, otherwise use config
	if uc.systemProfile != nil {
		recommendations := uc.systemProfile.Calculate()

		// Enable parallel walking if recommended
		if recommendations.ScanWalkers > 0 {
			walkerOpts = append(walkerOpts, filesystem.WithParallelWalking(recommendations.ScanWalkers))
			uc.logger.Info("using parallel directory walking",
				"workers", recommendations.ScanWalkers,
				"storage_type", uc.systemProfile.Storage.Type)
		}
	} else if uc.config.ParallelWalkers > 0 {
		// Fallback to manual configuration
		walkerOpts = append(walkerOpts, filesystem.WithParallelWalking(uc.config.ParallelWalkers))
	}

	// Enable progress logging if configured
	if uc.config.ProgressInterval > 0 {
		walkerOpts = append(walkerOpts, filesystem.WithProgressLogging(uc.config.ProgressInterval))
	}

	// Get current job to access estimated total for progress updates
	currentJob, jobErr := uc.scanRepos.ScanJob.GetByID(ctx, jobID)
	if jobErr != nil {
		uc.logger.Error("Failed to get job for progress callback",
			"job_id", jobID,
			"error", jobErr)
		return
	}

	// Track media files discovered (not all files)
	// We'll manually increment this counter as media files are discovered
	var mediaFilesDiscovered atomic.Int64

	// Add progress callback to update scan job progress in real-time
	// This callback is called by the scan_file_processor when a media file is discovered
	progressCallback := func(filesDiscovered int64) {
		progress := &scanner.Progress{
			FilesFound:     filesDiscovered,
			FilesProcessed: 0,
			LastUpdate:     time.Now(),
			Phase:          scanner.ScanPhaseDiscovering,
			EstimatedTotal: currentJob.EstimatedTotal,
			DiscoveryDone:  false,
		}
		// Update progress asynchronously to avoid blocking file discovery
		go func() {
			if err := uc.scanRepos.ScanJob.UpdateProgress(ctx, jobID, progress); err != nil {
				uc.logger.Warn("Failed to update discovery progress",
					"job_id", jobID,
					"files_discovered", filesDiscovered,
					"error", err)
			}
		}()
	}

	// Pass logger to walker for consistent structured logging
	walkerOpts = append(walkerOpts, filesystem.WithLogger(uc.logger))

	walker := filesystem.NewWalker(walkerOpts...)

	// Phase 1: Fast count pass to get accurate total for progress reporting
	uc.logger.Info("Counting media files", "library_id", lib.ID, "path", lib.Path)
	totalFiles, err := walker.Count(ctx, lib.Path, func(fi scanner.FileInfo) bool {
		return !fi.IsDir && uc.isMediaFile(fi.Extension)
	})
	if err != nil {
		uc.logger.Warn("File count failed, proceeding with discovery",
			"library_id", lib.ID,
			"error", err)
		totalFiles = 0 // Continue with discovery even if count fails
	} else {
		uc.logger.Info("File count completed",
			"library_id", lib.ID,
			"total_files", totalFiles)

		// Update job with estimated total and mark discovery as done
		countProgress := &scanner.Progress{
			FilesFound:     totalFiles, // Set to actual count after count phase
			FilesProcessed: 0,
			LastUpdate:     time.Now(),
			Phase:          scanner.ScanPhaseDiscovering,
			EstimatedTotal: totalFiles,
			DiscoveryDone:  true, // We know the total now
		}
		if err := uc.scanRepos.ScanJob.UpdateProgress(ctx, jobID, countProgress); err != nil {
			uc.logger.Warn("Failed to update count progress",
				"job_id", jobID,
				"error", err)
		}
	}

	// Use buffered channel to collect files with less lock contention
	// Buffer size = 100000 allows discovery to proceed without blocking even for large libraries
	filesChan := make(chan scanner.FileInfo, 100000)
	var discoveryWg sync.WaitGroup
	discoveryWg.Add(1)

	// Collector goroutine - drains channel into slice
	var discoveredFiles []scanner.FileInfo
	go func() {
		defer discoveryWg.Done()
		for fileInfo := range filesChan {
			discoveredFiles = append(discoveredFiles, fileInfo)
		}
	}()

	// Walk directory tree and send media files to channel
	err = walker.Walk(ctx, lib.Path, func(fileInfo scanner.FileInfo) error {
		// Skip directories
		if fileInfo.IsDir {
			return nil
		}

		// Check if it's a media file based on extension
		if !uc.isMediaFile(fileInfo.Extension) {
			return nil
		}

		// Send to channel (non-blocking due to buffer)
		filesChan <- fileInfo

		// Increment media files counter and report progress periodically
		count := mediaFilesDiscovered.Add(1)
		if count%1000 == 0 { // Report every 1000 files to avoid too many updates
			progressCallback(count)
		}

		return nil
	})

	// Close channel and wait for collector to finish
	close(filesChan)
	discoveryWg.Wait()

	// Report final count (in case total is less than 1000 or not a multiple of 1000)
	finalCount := mediaFilesDiscovered.Load()
	if finalCount > 0 {
		progressCallback(finalCount)
	}

	// Get discovery statistics from walker
	discoveryStats := walker.GetStats()
	if discoveryStats != nil {
		uc.logger.Info("Discovery statistics",
			"job_id", jobID,
			"files_discovered", discoveryStats.FilesDiscovered,
			"dirs_scanned", discoveryStats.DirsScanned,
			"dirs_skipped", discoveryStats.DirsSkipped,
			"files_skipped", discoveryStats.FilesSkipped,
			"permission_errors", discoveryStats.PermissionErrors,
			"network_errors", discoveryStats.NetworkErrors,
			"other_errors", discoveryStats.OtherErrors)

		// Warn if discovery had errors
		if discoveryStats.HasErrors() {
			uc.logger.Warn("Discovery completed with errors - some files may be missing",
				"job_id", jobID,
				"total_errors", discoveryStats.TotalErrors(),
				"dirs_skipped", discoveryStats.DirsSkipped,
				"files_skipped", discoveryStats.FilesSkipped,
				"sample_paths", discoveryStats.SkippedPaths)
		}
	}

	if err != nil {
		uc.logger.Error("File discovery failed",
			"job_id", jobID,
			"library_id", lib.ID,
			"error", err)
		uc.completeJobWithError(ctx, jobID, err)
		return
	}

	uc.logger.Info("File discovery completed",
		"job_id", jobID,
		"media_files_discovered", len(discoveredFiles))

	// Validate discovery results against previous scans
	discoveryWarnings := uc.validateDiscovery(ctx, lib.ID, int64(len(discoveredFiles)), discoveryStats)
	if len(discoveryWarnings) > 0 {
		for _, warning := range discoveryWarnings {
			uc.logger.Warn("Discovery validation warning", "job_id", jobID, "warning", warning)
		}
	}

	// Mark discovery as complete and transition to processing phase
	discoveryProgress := &scanner.Progress{
		FilesFound:     int64(len(discoveredFiles)),
		FilesProcessed: 0,
		LastUpdate:     time.Now(),
		Phase:          scanner.ScanPhaseProcessing,
		EstimatedTotal: currentJob.EstimatedTotal,
		DiscoveryDone:  true,
	}
	if err := uc.scanRepos.ScanJob.UpdateProgress(ctx, jobID, discoveryProgress); err != nil {
		uc.logger.Warn("Failed to update discovery completion status",
			"job_id", jobID,
			"error", err)
	}

	// Phase 2: Incremental Scan - determine what changed since last scan
	diff, err := uc.incrementalScanner.DetermineChanges(ctx, lib.ID, discoveredFiles)
	if err != nil {
		// If incremental scan fails, fall back to full scan
		uc.logger.Warn("incremental scan failed, falling back to full scan",
			"library_id", lib.ID,
			"error", err)
		diff = &scanner.ScanDiff{
			NewFiles:       discoveredFiles, // Treat all files as new
			ModifiedFiles:  []scanner.FileInfo{},
			DeletedFiles:   []string{},
			UnchangedFiles: []string{},
		}
	} else {
		uc.logger.Info("incremental scan completed",
			"library_id", lib.ID,
			"diff", diff.Summary())
	}

	// Check if there are any changes to process
	if !diff.NeedsProcessing() {
		uc.logger.Info("no changes detected, scan complete",
			"files_found", len(discoveredFiles),
			"unchanged_files", len(diff.UnchangedFiles))
		// Mark job as completed successfully with no files processed
		// FilesFound = total discovered files (not just unchanged files!)
		job := &scanner.ScanJob{
			ID:             jobID,
			Status:         scanner.ScanStatusCompleted,
			FilesFound:     int64(len(discoveredFiles)), // Total discovered, not just unchanged
			FilesProcessed: 0,
			ErrorCount:     0,
			Progress:       100.0,
			CompletedAt:    &[]time.Time{time.Now()}[0],
			Phase:          scanner.ScanPhaseCompleted,
			DiscoveryDone:  true,
		}
		if err := uc.scanRepos.ScanJob.Complete(ctx, job); err != nil {
			uc.logger.Error("failed to mark scan job as complete", "job_id", jobID, "error", err)
		}
		return
	}

	// Phase 3: Handle deleted files
	if len(diff.DeletedFiles) > 0 {
		uc.logger.Info("removing deleted files from scan state",
			"count", len(diff.DeletedFiles))
		if err := uc.scanRepos.ScanState.DeleteByPaths(ctx, lib.ID, diff.DeletedFiles); err != nil {
			uc.logger.Error("failed to delete scan state for removed files",
				"library_id", lib.ID,
				"count", len(diff.DeletedFiles),
				"error", err)
		}
		// TODO: Optionally delete media records for deleted files
		// For now, we just remove from scan_state tracking
	}

	// Phase 4 & 5: Hash files and process them concurrently
	// Hash files upfront so hashes survive crashes and we don't duplicate work on resume
	// Uses streaming approach: hash in parallel → insert in batches → process immediately
	// This doubles efficiency: hashing and processing happen in parallel instead of sequentially
	filesToProcess := append(diff.NewFiles, diff.ModifiedFiles...)

	uc.logger.Info("starting concurrent hashing and processing",
		"file_count", len(filesToProcess),
		"hash_workers", 8)

	startTime := time.Now()

	uc.logger.Info("processing files",
		"total", len(filesToProcess),
		"new", len(diff.NewFiles),
		"modified", len(diff.ModifiedFiles),
		"skipped", len(diff.UnchangedFiles))

	// Start processing goroutine immediately - it will consume checkpoints as they're created
	var processingWg sync.WaitGroup
	processingWg.Add(1)
	processingErrChan := make(chan error, 1)
	hashingDone := make(chan struct{}) // Signal when hashing completes

	go func() {
		defer processingWg.Done()
		defer func() {
			if r := recover(); r != nil {
				uc.logger.Error("panic in processing goroutine",
					"panic", r,
					"stack", string(debug.Stack()))
				processingErrChan <- fmt.Errorf("processing panicked: %v", r)
			}
		}()

		// Process files as checkpoints become available, passing discovery stats
		uc.processFilesWithCheckpoints(ctx, jobID, lib, hashingDone, discoveryStats)
	}()

	// Hash files and stream checkpoints in parallel (processing consumes them concurrently)
	if err := uc.hashAndStreamCheckpoints(ctx, filesToProcess, jobID, lib.ID); err != nil {
		uc.logger.Error("failed to create checkpoints", "error", err)
		close(hashingDone) // Signal completion even on error
		uc.completeJobWithError(ctx, jobID, err)
		return
	}

	// Signal that hashing is complete
	close(hashingDone)

	// Wait for processing to complete
	processingWg.Wait()

	// Check for processing errors
	select {
	case err := <-processingErrChan:
		if err != nil {
			uc.logger.Error("processing failed", "error", err)
			uc.completeJobWithError(ctx, jobID, err)
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