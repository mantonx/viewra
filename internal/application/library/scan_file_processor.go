package library

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
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

	// Add progress callback to update scan job progress in real-time
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
	walkerOpts = append(walkerOpts, filesystem.WithProgressCallback(progressCallback))

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
			FilesFound:     0,
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
		return nil
	})

	// Close channel and wait for collector to finish
	close(filesChan)
	discoveryWg.Wait()

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
		fmt.Printf("info: no changes detected, scan complete (unchanged=%d)\n", len(diff.UnchangedFiles))
		// Mark job as completed successfully with no files processed
		job := &scanner.ScanJob{
			ID:             jobID,
			Status:         scanner.ScanStatusCompleted,
			FilesFound:     int64(len(diff.UnchangedFiles)),
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
		fmt.Printf("info: removing %d deleted files from scan state\n", len(diff.DeletedFiles))
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

	fmt.Printf("info: processing %d files (new=%d, modified=%d, skipped=%d)\n",
		len(filesToProcess), len(diff.NewFiles), len(diff.ModifiedFiles), len(diff.UnchangedFiles))

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

		// Process files as checkpoints become available
		uc.processFilesWithCheckpoints(ctx, jobID, lib, hashingDone)
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

// processFilesWithCheckpoints processes files in batches using checkpoints with parallel workers
// hashingDone channel signals when checkpoint creation is complete
func (uc *ScanLibraryUseCase) processFilesWithCheckpoints(ctx context.Context, jobID int64, lib *library.Library, hashingDone <-chan struct{}) {
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
			for checkpoint := range checkpointsChan {
				uc.processCheckpointWorker(ctx, lib, checkpoint, maxRetries, &foundFilesMu, foundFiles)
			}
		}(w)
	}

	// Main loop: fetch batches and feed to workers
	for {
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
				break
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
			progress := &scanner.Progress{
				FilesFound:     stats.TotalFiles,
				FilesProcessed: stats.ProcessedFiles,
				ErrorCount:     stats.FailedFiles,
				WarningCount:   stats.WarningFiles,
			}
			_ = uc.scanRepos.ScanJob.UpdateProgress(ctx, jobID, progress)
		default:
		}
	}

	// Close checkpoint channel and wait for all workers to finish
	close(checkpointsChan)
	workerWg.Wait()

	// Get final stats
	stats, _ := uc.scanRepos.Checkpoint.GetStats(ctx, jobID)

	// Complete the job
	job := &scanner.ScanJob{
		ID:             jobID,
		FilesFound:     stats.TotalFiles,
		FilesProcessed: stats.CompletedFiles,
		ErrorCount:     stats.FailedFiles,
		WarningCount:   stats.WarningFiles,
		Progress:       stats.GetProgress(),
		CompletedAt:    &[]time.Time{time.Now()}[0],
		Phase:          scanner.ScanPhaseCompleted,
		DiscoveryDone:  true,
	}

	if stats.FailedFiles > 0 {
		job.Status = scanner.ScanStatusCompleted // Partial success
		uc.logger.Info("scan completed with errors",
			"job_id", jobID,
			"library_id", lib.ID,
			"total_files", stats.TotalFiles,
			"completed_files", stats.CompletedFiles,
			"failed_files", stats.FailedFiles)
	} else {
		job.Status = scanner.ScanStatusCompleted
		uc.logger.Info("scan completed successfully",
			"job_id", jobID,
			"library_id", lib.ID,
			"total_files", stats.TotalFiles,
			"completed_files", stats.CompletedFiles)
	}

	if err := uc.scanRepos.ScanJob.Complete(ctx, job); err != nil {
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

// processFileWithCheckpoint processes a single file based on library type
// Returns (hasWarning bool, error)
func (uc *ScanLibraryUseCase) processFileWithCheckpoint(ctx context.Context, lib *library.Library, checkpoint *scanner.ScanCheckpoint) (bool, error) {
	// Re-extract metadata for this file (checkpoint only stores the path)
	// We need full metadata to create the media entry
	fileInfo, err := os.Stat(checkpoint.FilePath)
	if err != nil {
		return false, fmt.Errorf("failed to stat file: %w", err)
	}

	// Create a scanner.FileInfo for the coordinator to process
	scanFileInfo := scanner.FileInfo{
		Path:      checkpoint.FilePath,
		Size:      fileInfo.Size(),
		ModTime:   fileInfo.ModTime(),
		IsDir:     false,
		Extension: strings.ToLower(filepath.Ext(checkpoint.FilePath)), // Keep the dot for GetMediaType
	}

	// Use coordinator to extract metadata (this will parse filename and run ffprobe)
	// Note: Coordinator no longer handles hashing - done during checkpoint creation
	config := filesystem.DefaultCoordinatorConfig()
	config.Logger = uc.logger // Pass the use case logger to coordinator
	coordinator := filesystem.NewCoordinator(config)

	// Add timeout protection to prevent worker deadlocks on slow network storage
	// FFprobe can hang indefinitely on large files over CIFS/NFS
	// Timeout varies based on storage type and file size
	timeout := uc.calculateProcessingTimeout(fileInfo.Size())
	processCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result := coordinator.ProcessFile(processCtx, scanFileInfo)

	// Check if metadata extraction had an error
	if result.Error != nil {
		// Check if it was a timeout
		if processCtx.Err() == context.DeadlineExceeded {
			uc.logger.Warn("file processing timeout exceeded",
				"file_path", checkpoint.FilePath,
				"file_size", fileInfo.Size(),
				"timeout", timeout)
			return false, fmt.Errorf("processing timeout after %v: %w", timeout, processCtx.Err())
		}
		return false, result.Error
	}

	// Process based on library type and capture the returned media ID
	var mediaID *int64
	switch lib.Type {
	case library.LibraryTypeMovies:
		mediaID = uc.processMovie(ctx, lib.ID, &result, checkpoint)
	case library.LibraryTypeTV:
		mediaID = uc.processTVEpisode(ctx, lib.ID, &result, checkpoint)
	case library.LibraryTypeMusic:
		mediaID = uc.processMusicTrack(ctx, lib.ID, &result, checkpoint)
	default:
		return false, fmt.Errorf("unknown library type: %s", lib.Type)
	}

	// Update scan state after successful processing
	// This enables incremental scanning on the next scan
	// Use checkpoint hash (computed upfront during checkpoint creation)
	scanState := &scanner.ScanState{
		LibraryID:     lib.ID,
		FilePath:      checkpoint.FilePath,
		FileSize:      fileInfo.Size(),
		FileMTime:     fileInfo.ModTime(),
		FileHash:      checkpoint.FileHash, // Already computed during checkpoint creation
		MediaID:       mediaID,              // Link to created/updated media record
		LastScannedAt: time.Now(),
		ScanJobID:     checkpoint.ScanJobID,
	}

	if err := uc.scanRepos.ScanState.Upsert(ctx, scanState); err != nil {
		// Log error but don't fail the scan - scan state is for optimization, not critical
		uc.logger.Warn("failed to update scan state",
			"file_path", checkpoint.FilePath,
			"error", err)
	}

	// Check if file processing had warnings (e.g., FFmpeg metadata extraction failure)
	hasWarning := result.Warning != nil
	return hasWarning, nil
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
	hasWarning, err := uc.processFileWithCheckpoint(workerCtx, lib, checkpoint)

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

// hashAndStreamCheckpoints hashes files in parallel and streams checkpoints to DB in batches.
// Optimizations:
// - Skips hashing for files unchanged since last scan (reuses existing hash from scan_state)
// - Uses XXH3-128 instead of SHA256 (50-100x faster for new/modified files, zero collision risk)
// - Memory efficient: holds max 10 checkpoints in memory vs all 33k+
// - Crash resilient: checkpoints saved incrementally, not lost if server crashes mid-hash
// - Uses system profile to auto-tune worker count based on CPU and storage type
func (uc *ScanLibraryUseCase) hashAndStreamCheckpoints(
	ctx context.Context,
	filesToProcess []scanner.FileInfo,
	jobID int64,
	libraryID int64,
) error {
	// Calculate optimal settings based on system profile
	// Storage type detection is done earlier in runScan()
	var numWorkers, batchSize int
	if uc.systemProfile != nil {
		settings := uc.systemProfile.Calculate()
		numWorkers = settings.HashWorkers
		batchSize = settings.CheckpointBatchSize
		uc.logger.Info("Using profile-based scan settings",
			"hash_workers", numWorkers,
			"batch_size", batchSize,
			"storage_type", uc.systemProfile.Storage.Type)
	} else {
		// Fallback to conservative defaults if no profile
		numWorkers = 8
		batchSize = 10
		uc.logger.Warn("No system profile available, using default settings",
			"hash_workers", numWorkers,
			"batch_size", batchSize)
	}

	const batchTimeout = 500 * time.Millisecond // Insert even partial batches after 500ms

	type hashJob struct {
		fileInfo scanner.FileInfo
	}

	// Create channels for work distribution
	jobs := make(chan hashJob, 100)
	checkpoints := make(chan *scanner.ScanCheckpoint, 100)
	errors := make(chan error, 1) // Buffered to prevent goroutine leak

	// Start hash workers
	var hashWg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		hashWg.Add(1)
		go func() {
			defer hashWg.Done()
			hasher := filesystem.NewHasher()

			for job := range jobs {
				func() {
					defer func() {
						if r := recover(); r != nil {
							// Panic during hashing - send error result
							uc.logger.Error("panic during file hashing",
								"file_path", job.fileInfo.Path,
								"panic", r,
								"stack", string(debug.Stack()))
							// Send checkpoint with empty hash
							checkpoints <- &scanner.ScanCheckpoint{
								ScanJobID: jobID,
								FilePath:  job.fileInfo.Path,
								Status:    scanner.CheckpointPending,
								FileSize:  job.fileInfo.Size,
								FileHash:  "", // Empty due to panic
								CreatedAt: time.Now(),
							}
						}
					}()

					// Try to get existing hash from scan_state (optimization for unchanged files)
					var fileHash string
					existingState, err := uc.scanRepos.ScanState.GetByPath(ctx, libraryID, job.fileInfo.Path)
					if err == nil && existingState != nil && existingState.FileHash != "" {
						// File exists in scan_state with a hash - reuse it (skip expensive hashing)
						fileHash = existingState.FileHash
					} else {
						// New file or hash missing - compute hash using XXH3-128 (50-100x faster than SHA256)
						hash, err := hasher.Hash(job.fileInfo.Path)
						fileHash = hash
						if err != nil {
							uc.logger.Warn("failed to hash file, will use mtime+size fallback for change detection",
								"file_path", job.fileInfo.Path,
								"error", err)
							fileHash = "" // Leave empty - scan_state will use mtime+size fallback
						}
					}

					checkpoints <- &scanner.ScanCheckpoint{
						ScanJobID: jobID,
						FilePath:  job.fileInfo.Path,
						Status:    scanner.CheckpointPending,
						FileSize:  job.fileInfo.Size,
						FileHash:  fileHash,
						CreatedAt: time.Now(),
					}
				}()
			}
		}()
	}

	// Start DB writer goroutine - inserts checkpoints in small batches with timeout
	var writerWg sync.WaitGroup
	writerWg.Add(1)
	go func() {
		defer writerWg.Done()

		batch := make([]*scanner.ScanCheckpoint, 0, batchSize)
		processed := 0
		timer := time.NewTimer(batchTimeout)
		defer timer.Stop()

		flushBatch := func() {
			if len(batch) == 0 {
				return
			}
			if err := uc.scanRepos.Checkpoint.CreateBatch(ctx, batch); err != nil {
				uc.logger.Error("failed to insert checkpoint batch", "error", err)
				select {
				case errors <- err:
				default:
				}
				return
			}
			batch = batch[:0] // Clear batch
			timer.Reset(batchTimeout)
		}

		for {
			select {
			case checkpoint, ok := <-checkpoints:
				if !ok {
					// Channel closed, flush remaining batch
					flushBatch()
					return
				}

				batch = append(batch, checkpoint)
				processed++

				// Log progress every 5000 files
				if processed%5000 == 0 {
					uc.logger.Info("hashing and checkpoint creation progress",
						"processed", processed,
						"total", len(filesToProcess),
						"percent", int(float64(processed)/float64(len(filesToProcess))*100))
				}

				// Insert batch when it reaches batch size
				if len(batch) >= batchSize {
					flushBatch()
				}

			case <-timer.C:
				// Timeout - flush partial batch
				flushBatch()
			}
		}
	}()

	// Send all jobs to workers
	go func() {
		for _, fileInfo := range filesToProcess {
			jobs <- hashJob{
				fileInfo: fileInfo,
			}
		}
		close(jobs)
	}()

	// Wait for all hash workers to finish, then close checkpoints channel
	go func() {
		hashWg.Wait()
		close(checkpoints)
	}()

	// Wait for DB writer to finish
	writerWg.Wait()

	// Check for errors
	select {
	case err := <-errors:
		return err
	default:
		return nil
	}
}

// isMediaFile checks if a file extension is for a media file
func (uc *ScanLibraryUseCase) isMediaFile(ext string) bool {
	// Remove leading dot if present
	ext = strings.TrimPrefix(ext, ".")

	mediaExtensions := map[string]bool{
		// Video
		"mp4": true, "mkv": true, "avi": true, "mov": true, "wmv": true, "flv": true,
		"webm": true, "m4v": true, "mpg": true, "mpeg": true, "m2ts": true, "ts": true,
		// Audio
		"mp3": true, "flac": true, "m4a": true, "aac": true, "ogg": true, "opus": true,
		"wav": true, "wma": true, "ape": true, "wv": true, "tta": true, "tak": true,
	}
	return mediaExtensions[strings.ToLower(ext)]
}

// calculateProcessingTimeout determines appropriate timeout for file processing
// based on file size and storage type to prevent worker deadlocks
func (uc *ScanLibraryUseCase) calculateProcessingTimeout(fileSize int64) time.Duration {
	// Base timeout: 30 seconds (sufficient for most files on local storage)
	baseTimeout := 30 * time.Second

	// For network storage, be more generous due to latency
	if uc.systemProfile != nil && uc.systemProfile.Storage.IsRemote {
		baseTimeout = 60 * time.Second
	}

	// Add extra time for large files (1 second per GB)
	// This handles 4K content and large remuxes that take longer to probe
	const bytesPerGB = 1024 * 1024 * 1024
	sizeGB := fileSize / bytesPerGB
	if sizeGB > 0 {
		// Add up to 2 minutes extra for very large files
		extraTime := time.Duration(sizeGB) * time.Second
		if extraTime > 120*time.Second {
			extraTime = 120 * time.Second
		}
		baseTimeout += extraTime
	}

	return baseTimeout
}

// isExtra determines if a file is an extra (trailer, deleted scene, featurette, etc.)
// based on common filename patterns
func isExtra(filepath string) bool {
	lower := strings.ToLower(filepath)
	extraPatterns := []string{
		"-trailer.",
		"_trailer.",
		".trailer.",
		"-deleted",
		"_deleted",
		".deleted",
		"-featurette",
		"_featurette",
		".featurette",
		"-extra.",
		"_extra.",
		".extra.",
		"-bonus.",
		"_bonus.",
		".bonus.",
	}

	for _, pattern := range extraPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
