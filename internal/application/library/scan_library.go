package library

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	domainCommon "github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/images"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/domain/scanner/parsers"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
	"github.com/mantonx/viewra/internal/infrastructure/metadata/music"
	"github.com/mantonx/viewra/internal/infrastructure/metadata/nfo"
	"github.com/mantonx/viewra/internal/infrastructure/system"
)

// ScanLibraryUseCase handles the business logic for scanning a library
type ScanLibraryUseCase struct {
	libraryRepo          library.Repository
	mediaRepo            media.Repository
	movieRepo            media.MovieRepository
	tvRepo               media.TVRepository
	musicRepo            media.MusicRepository
	scanJobRepo          scanner.ScanJobRepository
	checkpointRepo       scanner.CheckpointRepository
	scanStateRepo        scanner.ScanStateRepository
	incrementalScanner   *IncrementalScanner
	extractMovieImages   ExtractMovieImagesExecutor
	extractEpisodeImages ExtractTVEpisodeImagesExecutor
	extractShowImages    ExtractTVShowImagesExecutor
	extractSeasonImages  ExtractTVSeasonImagesExecutor
	extractMusicImages   ExtractMusicAlbumImagesExecutor
	extractArtistImages  ExtractMusicArtistImagesExecutor
	imageRepo            images.Repository
	imageCleanup         ImageCleanupExecutor
	scanTimeout          time.Duration
	systemProfile        *system.Profile
	logger               *slog.Logger

	// Artist deduplication tracking (per scan session)
	// Using sync.Map for lock-free concurrent access (fixes race condition)
	processedArtists sync.Map // string -> bool
}

// ExtractMovieImagesExecutor interface for movie image extraction
type ExtractMovieImagesExecutor interface {
	Execute(ctx context.Context, movieFilePath string, mediaType images.MediaType, entityID int, mediaID *int) error
}

// ExtractTVEpisodeImagesExecutor interface for TV episode image extraction
type ExtractTVEpisodeImagesExecutor interface {
	Execute(ctx context.Context, episodeFilePath string, mediaType images.MediaType, entityID int, mediaID *int) error
}

// ExtractTVShowImagesExecutor interface for TV show image extraction
type ExtractTVShowImagesExecutor interface {
	Execute(ctx context.Context, showDir string, mediaType images.MediaType, entityID int) error
}

// ExtractTVSeasonImagesExecutor interface for TV season image extraction
type ExtractTVSeasonImagesExecutor interface {
	Execute(ctx context.Context, showDir string, seasonNumber int, mediaType images.MediaType, entityID int) error
}

// ExtractMusicAlbumImagesExecutor interface for music album image extraction
type ExtractMusicAlbumImagesExecutor interface {
	Execute(ctx context.Context, albumDir string, mediaType images.MediaType, entityID int) error
}

// ExtractMusicArtistImagesExecutor interface for music artist image extraction
type ExtractMusicArtistImagesExecutor interface {
	Execute(ctx context.Context, artistDir string, mediaType images.MediaType, entityID int) error
}

// NewScanLibraryUseCase creates a new instance of ScanLibraryUseCase
func NewScanLibraryUseCase(
	libraryRepo library.Repository,
	mediaRepo media.Repository,
	movieRepo media.MovieRepository,
	tvRepo media.TVRepository,
	musicRepo media.MusicRepository,
	scanJobRepo scanner.ScanJobRepository,
	checkpointRepo scanner.CheckpointRepository,
	scanStateRepo scanner.ScanStateRepository,
	extractMovieImages ExtractMovieImagesExecutor,
	extractEpisodeImages ExtractTVEpisodeImagesExecutor,
	extractShowImages ExtractTVShowImagesExecutor,
	extractSeasonImages ExtractTVSeasonImagesExecutor,
	extractMusicImages ExtractMusicAlbumImagesExecutor,
	extractArtistImages ExtractMusicArtistImagesExecutor,
	imageRepo images.Repository,
	imageCleanup ImageCleanupExecutor,
	scanTimeout time.Duration,
	systemProfile *system.Profile,
	logger *slog.Logger,
) *ScanLibraryUseCase {
	// Create incremental scanner
	incrementalScanner := NewIncrementalScanner(scanStateRepo, logger)

	return &ScanLibraryUseCase{
		libraryRepo:          libraryRepo,
		mediaRepo:            mediaRepo,
		movieRepo:            movieRepo,
		tvRepo:               tvRepo,
		musicRepo:            musicRepo,
		scanJobRepo:          scanJobRepo,
		checkpointRepo:       checkpointRepo,
		scanStateRepo:        scanStateRepo,
		incrementalScanner:   incrementalScanner,
		extractMovieImages:   extractMovieImages,
		extractEpisodeImages: extractEpisodeImages,
		extractShowImages:    extractShowImages,
		extractSeasonImages:  extractSeasonImages,
		extractMusicImages:   extractMusicImages,
		extractArtistImages:  extractArtistImages,
		imageRepo:            imageRepo,
		imageCleanup:         imageCleanup,
		scanTimeout:          scanTimeout,
		systemProfile:        systemProfile,
		logger:               logger,
	}
}

// StartScan initiates a new scan for a library
func (uc *ScanLibraryUseCase) StartScan(ctx context.Context, libraryID int64) (StartScanResponse, error) {
	// Verify library exists
	lib, err := uc.libraryRepo.GetByID(ctx, libraryID)
	if err != nil {
		return StartScanResponse{}, fmt.Errorf("failed to get library: %w", err)
	}

	// Check for existing running scan
	running, err := uc.scanJobRepo.ListRunning(ctx)
	if err != nil {
		return StartScanResponse{}, fmt.Errorf("failed to check running scans: %w", err)
	}
	for _, job := range running {
		if job.LibraryID == libraryID {
			return StartScanResponse{}, scanner.ErrAlreadyRunning
		}
	}

	// Create scan job
	job := &scanner.ScanJob{
		LibraryID:      libraryID,
		Status:         scanner.ScanStatusRunning,
		Progress:       0,
		FilesFound:     0,
		FilesProcessed: 0,
		BytesProcessed: 0,
		ErrorCount:     0,
		StartedAt:      time.Now(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := uc.scanJobRepo.Create(ctx, job); err != nil {
		return StartScanResponse{}, fmt.Errorf("failed to create scan job: %w", err)
	}

	// Start background scan with timeout
	// Use a new context with timeout (not derived from request context which may cancel early)
	scanCtx, cancel := context.WithTimeout(context.Background(), uc.scanTimeout)

	// Start the scan goroutine with proper cleanup and panic recovery
	go func() {
		defer cancel() // Always cancel to prevent context leak

		// Recover from panics to prevent crashing the entire application
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("PANIC: scan goroutine panicked: %v\nStack trace:\n%s\n", r, string(debug.Stack()))

				// Mark job as failed
				failedJob := &scanner.ScanJob{
					ID:           job.ID,
					Status:       scanner.ScanStatusFailed,
					ErrorMessage: fmt.Sprintf("scan panicked: %v", r),
					CompletedAt:  &[]time.Time{time.Now()}[0],
				}
				if err := uc.scanJobRepo.Complete(context.Background(), failedJob); err != nil {
					fmt.Printf("error: failed to mark panicked scan job as failed: %v\n", err)
				}
			}
		}()

		uc.runScan(scanCtx, job.ID, lib)
	}()

	// Note: We intentionally do NOT monitor the parent HTTP request context here.
	// The scan should continue in the background even after the HTTP response is sent.
	// The scan has its own timeout (scanCtx) to prevent it from running indefinitely.

	return ToStartScanResponse(job), nil
}

// ResumeStuckScans automatically resumes scans that were interrupted by server restart or crash.
// This is called during application startup to recover gracefully from unexpected shutdowns.
func (uc *ScanLibraryUseCase) ResumeStuckScans(ctx context.Context) error {
	// Get all running scans (these are stuck from previous session)
	runningScans, err := uc.scanJobRepo.ListRunning(ctx)
	if err != nil {
		return fmt.Errorf("failed to get running scans: %w", err)
	}

	if len(runningScans) == 0 {
		fmt.Printf("info: no stuck scans found during startup\n")
		return nil
	}

	fmt.Printf("info: found %d stuck scans, automatically resuming...\n", len(runningScans))

	// Resume each stuck scan
	for _, job := range runningScans {
		// Get the library for this scan
		lib, err := uc.libraryRepo.GetByID(ctx, job.LibraryID)
		if err != nil {
			fmt.Printf("error: failed to get library %d for stuck scan %d: %v\n", job.LibraryID, job.ID, err)
			// Mark this scan as failed since we can't resume it
			failedJob := &scanner.ScanJob{
				ID:           job.ID,
				Status:       scanner.ScanStatusFailed,
				ErrorMessage: fmt.Sprintf("library not found during resume: %v", err),
				CompletedAt:  &[]time.Time{time.Now()}[0],
			}
			_ = uc.scanJobRepo.Complete(ctx, failedJob)
			continue
		}

		// Check if there are checkpoints to resume from
		stats, err := uc.checkpointRepo.GetStats(ctx, job.ID)
		if err != nil || stats.PendingFiles == 0 {
			// No pending files, mark as completed
			fmt.Printf("info: stuck scan %d has no pending files, marking as completed\n", job.ID)
			completedJob := &scanner.ScanJob{
				ID:             job.ID,
				Status:         scanner.ScanStatusCompleted,
				FilesFound:     stats.TotalFiles,
				FilesProcessed: stats.CompletedFiles,
				ErrorCount:     stats.FailedFiles,
				Progress:       stats.GetProgress(),
				CompletedAt:    &[]time.Time{time.Now()}[0],
			}
			_ = uc.scanJobRepo.Complete(ctx, completedJob)
			continue
		}

		fmt.Printf("info: resuming stuck scan %d for library %d (pending=%d, completed=%d)\n",
			job.ID, lib.ID, stats.PendingFiles, stats.CompletedFiles)

		// Start background goroutine to resume the scan
		scanCtx, cancel := context.WithTimeout(context.Background(), uc.scanTimeout)
		go func(jobID int64, library *library.Library) {
			defer cancel()

			// Recover from panics
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("PANIC: resumed scan goroutine panicked: %v\nStack trace:\n%s\n", r, string(debug.Stack()))
					failedJob := &scanner.ScanJob{
						ID:           jobID,
						Status:       scanner.ScanStatusFailed,
						ErrorMessage: fmt.Sprintf("resumed scan panicked: %v", r),
						CompletedAt:  &[]time.Time{time.Now()}[0],
					}
					_ = uc.scanJobRepo.Complete(context.Background(), failedJob)
				}
			}()

			// Resume the scan using the existing runScan method
			uc.runScan(scanCtx, jobID, library)
		}(job.ID, lib)
	}

	return nil
}

// runScan executes the actual scan in the background with checkpoint-based resumability
func (uc *ScanLibraryUseCase) runScan(ctx context.Context, jobID int64, lib *library.Library) {
	// Initialize artist deduplication tracking for this scan session
	uc.processedArtists = sync.Map{}

	// Check if there are existing checkpoints to resume from
	stats, err := uc.checkpointRepo.GetStats(ctx, jobID)
	if err == nil && stats.PendingFiles > 0 {
		// Resume from existing checkpoints
		fmt.Printf("info: resuming scan from checkpoint (pending=%d, completed=%d)\n",
			stats.PendingFiles, stats.CompletedFiles)
		uc.resumeScanFromCheckpoints(ctx, jobID, lib)
		return
	}

	// Fresh scan - discover files and create checkpoints
	fmt.Printf("info: starting fresh scan for library %d\n", lib.ID)
	uc.runFreshScan(ctx, jobID, lib)
}

// runFreshScan discovers all files and creates checkpoints before processing
// Uses incremental scanning to only process new/modified/deleted files
func (uc *ScanLibraryUseCase) runFreshScan(ctx context.Context, jobID int64, lib *library.Library) {
	// Phase 1: File Discovery - walk directory to collect file paths with metadata
	walker := filesystem.NewWalker()

	var discoveredFiles []scanner.FileInfo
	var mu sync.Mutex

	// Collect all media file paths with size and mtime
	err := walker.Walk(ctx, lib.Path, func(fileInfo scanner.FileInfo) error {
		// Skip directories
		if fileInfo.IsDir {
			return nil
		}

		// Check if it's a media file based on extension
		if !uc.isMediaFile(fileInfo.Extension) {
			return nil
		}

		mu.Lock()
		discoveredFiles = append(discoveredFiles, fileInfo)
		mu.Unlock()

		return nil
	})

	if err != nil {
		fmt.Printf("error: file discovery failed: %v\n", err)
		uc.completeJobWithError(ctx, jobID, err)
		return
	}

	fmt.Printf("info: discovered %d media files\n", len(discoveredFiles))

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
		}
		if err := uc.scanJobRepo.Complete(ctx, job); err != nil {
			uc.logger.Error("failed to mark scan job as complete", "job_id", jobID, "error", err)
		}
		return
	}

	// Phase 3: Handle deleted files
	if len(diff.DeletedFiles) > 0 {
		fmt.Printf("info: removing %d deleted files from scan state\n", len(diff.DeletedFiles))
		if err := uc.scanStateRepo.DeleteByPaths(ctx, lib.ID, diff.DeletedFiles); err != nil {
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

// resumeScanFromCheckpoints resumes a scan from existing checkpoints
func (uc *ScanLibraryUseCase) resumeScanFromCheckpoints(ctx context.Context, jobID int64, lib *library.Library) {
	fmt.Printf("info: resuming scan from checkpoints for job %d\n", jobID)
	// When resuming, hashing is already done
	hashingDone := make(chan struct{})
	close(hashingDone)
	uc.processFilesWithCheckpoints(ctx, jobID, lib, hashingDone)
}

// processFilesWithCheckpoints processes files in batches using checkpoints
// hashingDone channel signals when checkpoint creation is complete
func (uc *ScanLibraryUseCase) processFilesWithCheckpoints(ctx context.Context, jobID int64, lib *library.Library, hashingDone <-chan struct{}) {
	const batchSize = 50
	const maxRetries = 3
	updateTicker := time.NewTicker(2 * time.Second)
	defer updateTicker.Stop()

	foundFiles := make(map[string]bool)

	for {
		// Get next batch of pending files
		batch, err := uc.checkpointRepo.GetPendingBatch(ctx, jobID, batchSize)
		if err != nil {
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
				uc.logger.Info("All checkpoints processed, scan complete",
					"job_id", jobID)
				break
			default:
				// Hashing still in progress, wait for more checkpoints
				time.Sleep(2 * time.Second)
				continue
			}
		}

		// Process each file in the batch
		for _, checkpoint := range batch {
			// Mark as processing
			if err := uc.checkpointRepo.UpdateStatus(ctx, checkpoint.ID, scanner.CheckpointProcessing, "", ""); err != nil {
				uc.logger.Error("failed to mark checkpoint as processing",
					"checkpoint_id", checkpoint.ID,
					"file_path", checkpoint.FilePath,
					"error", err)
				continue // Skip this file, will retry in next batch
			}

			// Process the file
			err := uc.processFileWithCheckpoint(ctx, lib, checkpoint)

			if err != nil {
				// Categorize the error
				category := scanner.CategorizeError(err)

				// Check if error is transient and we haven't exceeded max retries
				if scanner.IsTransientError(err) && checkpoint.RetryCount < maxRetries {
					// Increment retry count
					checkpoint.RetryCount++
					if retryErr := uc.checkpointRepo.UpdateRetryCount(ctx, checkpoint.ID, checkpoint.RetryCount); retryErr != nil {
						uc.logger.Error("failed to update retry count",
							"checkpoint_id", checkpoint.ID,
							"file_path", checkpoint.FilePath,
							"retry_count", checkpoint.RetryCount,
							"error", retryErr)
					}

					// Calculate exponential backoff: 2^retryCount seconds (1s, 2s, 4s)
					backoffDuration := time.Duration(math.Pow(2, float64(checkpoint.RetryCount))) * time.Second
					uc.logger.Info("retrying file after transient error",
						"file_path", checkpoint.FilePath,
						"retry_count", checkpoint.RetryCount,
						"max_retries", maxRetries,
						"backoff_duration", backoffDuration,
						"error", err)

					// Wait with exponential backoff
					time.Sleep(backoffDuration)

					// Reset to pending for retry
					if statusErr := uc.checkpointRepo.UpdateStatus(ctx, checkpoint.ID, scanner.CheckpointPending, "", ""); statusErr != nil {
						uc.logger.Error("failed to reset checkpoint to pending for retry",
							"checkpoint_id", checkpoint.ID,
							"file_path", checkpoint.FilePath,
							"error", statusErr)
					}
				} else {
					// Either not transient or max retries exceeded - mark as failed
					if statusErr := uc.checkpointRepo.UpdateStatus(ctx, checkpoint.ID, scanner.CheckpointFailed, err.Error(), category); statusErr != nil {
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
			} else {
				// Mark as completed
				if statusErr := uc.checkpointRepo.UpdateStatus(ctx, checkpoint.ID, scanner.CheckpointCompleted, "", ""); statusErr != nil {
					uc.logger.Error("failed to mark checkpoint as completed",
						"checkpoint_id", checkpoint.ID,
						"file_path", checkpoint.FilePath,
						"error", statusErr)
					// Continue anyway - file was processed successfully, just status update failed
				}
				foundFiles[checkpoint.FilePath] = true
			}
		}

		// Periodic progress update
		select {
		case <-updateTicker.C:
			stats, _ := uc.checkpointRepo.GetStats(ctx, jobID)
			progress := &scanner.Progress{
				FilesFound:     stats.TotalFiles,
				FilesProcessed: stats.ProcessedFiles,
				ErrorCount:     stats.FailedFiles,
			}
			_ = uc.scanJobRepo.UpdateProgress(ctx, jobID, progress)
		default:
		}
	}

	// Get final stats
	stats, _ := uc.checkpointRepo.GetStats(ctx, jobID)

	// Complete the job
	job := &scanner.ScanJob{
		ID:             jobID,
		FilesFound:     stats.TotalFiles,
		FilesProcessed: stats.CompletedFiles,
		ErrorCount:     stats.FailedFiles,
		Progress:       stats.GetProgress(),
		CompletedAt:    &[]time.Time{time.Now()}[0],
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

	if err := uc.scanJobRepo.Complete(ctx, job); err != nil {
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
	if err := uc.checkpointRepo.DeleteByJobID(ctx, jobID); err != nil {
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
func (uc *ScanLibraryUseCase) processFileWithCheckpoint(ctx context.Context, lib *library.Library, checkpoint *scanner.ScanCheckpoint) error {
	// Re-extract metadata for this file (checkpoint only stores the path)
	// We need full metadata to create the media entry
	fileInfo, err := os.Stat(checkpoint.FilePath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
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
	result := coordinator.ProcessFile(ctx, scanFileInfo)

	// Check if metadata extraction had an error
	if result.Error != nil {
		return result.Error
	}

	// Process based on library type (these methods don't return errors)
	switch lib.Type {
	case library.LibraryTypeMovies:
		uc.processMovie(ctx, lib.ID, &result, checkpoint)
	case library.LibraryTypeTV:
		uc.processTVEpisode(ctx, lib.ID, &result, checkpoint)
	case library.LibraryTypeMusic:
		uc.processMusicTrack(ctx, lib.ID, &result, checkpoint)
	default:
		return fmt.Errorf("unknown library type: %s", lib.Type)
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
		MediaID:       nil,                 // TODO: Link to created media record if needed
		LastScannedAt: time.Now(),
		ScanJobID:     checkpoint.ScanJobID,
	}

	if err := uc.scanStateRepo.Upsert(ctx, scanState); err != nil {
		// Log error but don't fail the scan - scan state is for optimization, not critical
		uc.logger.Warn("failed to update scan state",
			"file_path", checkpoint.FilePath,
			"error", err)
	}

	return nil
}

// completeJobWithError marks a job as failed
func (uc *ScanLibraryUseCase) completeJobWithError(ctx context.Context, jobID int64, err error) {
	job := &scanner.ScanJob{
		ID:           jobID,
		Status:       scanner.ScanStatusFailed,
		ErrorMessage: err.Error(),
		CompletedAt:  &[]time.Time{time.Now()}[0],
	}
	_ = uc.scanJobRepo.Complete(ctx, job)
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

// hashAndStreamCheckpoints hashes files in parallel and streams checkpoints to DB in batches.
// Optimizations:
// - Skips hashing for files unchanged since last scan (reuses existing hash from scan_state)
// - Uses xxHash instead of SHA256 (10-20x faster for new/modified files)
// - Memory efficient: holds max 10 checkpoints in memory vs all 33k+
// - Crash resilient: checkpoints saved incrementally, not lost if server crashes mid-hash
// - Uses system profile to auto-tune worker count based on CPU and storage type
func (uc *ScanLibraryUseCase) hashAndStreamCheckpoints(
	ctx context.Context,
	filesToProcess []scanner.FileInfo,
	jobID int64,
	libraryID int64,
) error {
	// Detect storage type for this library path (if we have files to process)
	if uc.systemProfile != nil && len(filesToProcess) > 0 {
		uc.systemProfile.UpdateForLibraryPath(ctx, filesToProcess[0].Path)
	}

	// Calculate optimal settings based on system profile
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
					existingState, err := uc.scanStateRepo.GetByPath(ctx, libraryID, job.fileInfo.Path)
					if err == nil && existingState != nil && existingState.FileHash != "" {
						// File exists in scan_state with a hash - reuse it (skip expensive hashing)
						fileHash = existingState.FileHash
					} else {
						// New file or hash missing - compute hash using xxHash (10-20x faster than SHA256)
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
			if err := uc.checkpointRepo.CreateBatch(ctx, batch); err != nil {
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

// processResults processes scan results and creates/updates media entries
func (uc *ScanLibraryUseCase) processResults(ctx context.Context, jobID int64, libraryID int64, libraryType library.LibraryType, resultChan <-chan scanner.ScanResult, foundFilePaths chan<- string) {
	updateTicker := time.NewTicker(2 * time.Second)
	defer updateTicker.Stop()
	defer close(foundFilePaths)

	progress := &scanner.Progress{
		StartTime:  time.Now(),
		LastUpdate: time.Now(),
	}

	for result := range resultChan {
		// Update progress counters
		progress.FilesProcessed++
		progress.BytesProcessed += result.BytesProcessed
		progress.LastUpdate = time.Now()

		if result.Error != nil {
			progress.ErrorCount++
			continue
		}

		// Track this file as found (use select to prevent deadlock if context is cancelled)
		select {
		case foundFilePaths <- result.FilePath:
			// Successfully sent
		case <-ctx.Done():
			// Context cancelled, stop processing
			return
		}

		// Create media entry based on library type
		// NOTE: This is legacy code path - checkpoint will be nil
		// New scans use processFileWithCheckpoint which has proper checkpoint with hash
		dummyCheckpoint := &scanner.ScanCheckpoint{
			FilePath: result.FilePath,
			FileHash: "", // No hash in legacy path
		}
		switch libraryType {
		case library.LibraryTypeMovies:
			uc.processMovie(ctx, libraryID, &result, dummyCheckpoint)
		case library.LibraryTypeTV:
			uc.processTVEpisode(ctx, libraryID, &result, dummyCheckpoint)
		case library.LibraryTypeMusic:
			uc.processMusicTrack(ctx, libraryID, &result, dummyCheckpoint)
		}

		// Periodically update job progress
		select {
		case <-updateTicker.C:
			if err := uc.scanJobRepo.UpdateProgress(ctx, jobID, progress); err != nil {
				fmt.Printf("failed to update scan progress: %v\n", err)
			}
		default:
		}
	}

	// Final progress update
	progress.LastUpdate = time.Now()
	if err := uc.scanJobRepo.UpdateProgress(ctx, jobID, progress); err != nil {
		fmt.Printf("failed to update final scan progress: %v\n", err)
	}
}

// processMovie creates or updates a movie entry
func (uc *ScanLibraryUseCase) processMovie(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint) {
	// Coordinator already parsed the filename - just use the results
	movie := &media.Movie{
		Media: media.Media{
			LibraryID:       libraryID,
			Title:           result.Title,
			FilePath:        result.FilePath,
			FileSize:        checkpoint.FileSize,
			FileHash:        checkpoint.FileHash,
			Duration:        int(result.Duration),
			IsExtra:         isExtra(result.FilePath),
			Width:           result.Width,
			Height:          result.Height,
			VideoCodec:      result.VideoCodec,
			AudioCodec:      result.AudioCodec,
			Bitrate:         result.Bitrate,
			FrameRate:       result.FrameRate,
			ContainerFormat: result.ContainerFormat,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
	}

	// Set year from scan result
	if result.Year != nil {
		movie.Year = *result.Year
	}

	// Try to enhance metadata from NFO file
	nfoPath, err := nfo.FindMovieNFO(result.FilePath)
	if err == nil && nfoPath != "" {
		nfoMetadata, err := nfo.ParseMovieNFO(nfoPath)
		if err == nil && nfoMetadata != nil {
			// Populate movie metadata from NFO
			if nfoMetadata.Title != "" {
				movie.Media.Title = nfoMetadata.Title
			}
			if nfoMetadata.Year > 0 {
				movie.Year = nfoMetadata.Year
			}
			movie.OriginalTitle = nfoMetadata.OriginalTitle
			movie.SortTitle = nfoMetadata.SortTitle
			movie.ReleaseDate = nfoMetadata.ReleaseDate
			movie.RuntimeMinutes = nfoMetadata.RuntimeMinutes
			movie.IMDbID = nfoMetadata.IMDbID
			movie.TMDbID = nfoMetadata.TMDbID
			movie.Director = nfoMetadata.Director
			movie.Cast = nfoMetadata.Cast
			movie.Genre = nfoMetadata.Genre
			movie.Plot = nfoMetadata.Plot
			movie.Tagline = nfoMetadata.Tagline
			movie.ContentRating = nfoMetadata.ContentRating
			movie.MaturityRating = nfoMetadata.MaturityRating
			movie.ContentAdvisories = nfoMetadata.ContentAdvisories
			movie.Budget = nfoMetadata.Budget
			movie.Revenue = nfoMetadata.Revenue
			movie.OriginalLanguage = nfoMetadata.OriginalLanguage
			movie.CountryOfOrigin = nfoMetadata.CountryOfOrigin
			movie.AwardsSummary = nfoMetadata.AwardsSummary
		}
	}

	// Ensure SortTitle is always set with normalized value
	if movie.SortTitle == "" {
		movie.SortTitle = domainCommon.NormalizeSortTitle(movie.Media.Title)
	}

	// Check if movie already exists
	existing, err := uc.mediaRepo.GetByFilePath(ctx, libraryID, result.FilePath)
	if err == nil && existing != nil {
		// Update existing entry
		movie.Media.ID = existing.ID
		movie.Media.Type = "movie"
		if err := uc.mediaRepo.Update(ctx, &movie.Media); err != nil {
			fmt.Printf("failed to update media %s: %v\n", result.FilePath, err)
		}
		if err := uc.movieRepo.UpdateMovie(ctx, movie); err != nil {
			fmt.Printf("failed to update movie metadata %s: %v\n", result.FilePath, err)
		}
		// Extract and catalog images (even for existing movies to populate cache)
		uc.extractImagesForMovie(ctx, movie, result.FilePath)
		return
	}

	// Create new entry - let movie repository handle both media and movie records
	movie.Media.Type = "movie"
	if err := uc.movieRepo.CreateMovie(ctx, movie); err != nil {
		fmt.Printf("failed to create movie %s: %v\n", result.FilePath, err)
		return
	}

	// Extract and catalog images for the movie
	uc.extractImagesForMovie(ctx, movie, result.FilePath)
}

// processTVEpisode creates or updates a TV episode entry
func (uc *ScanLibraryUseCase) processTVEpisode(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint) {
	// Coordinator already parsed season/episode/title, but we need show name which isn't in ScanResult
	parser := parsers.NewDefaultParser()
	tvInfo, err := parser.ParseTVEpisode(result.FilePath)
	if err != nil || tvInfo == nil {
		fmt.Printf("failed to parse TV episode filename %s: %v\n", result.FilePath, err)
		return // Can't create episode without show name
	}

	// Use coordinator's parsed data where available, parser for show name
	showTitle := tvInfo.ShowName
	season := 0
	episodeNumber := 0
	episodeTitle := result.Title

	// Prefer coordinator's parsed season/episode numbers
	if result.SeasonNumber != nil {
		season = *result.SeasonNumber
	} else {
		season = tvInfo.Season
	}

	if result.EpisodeNumber != nil {
		episodeNumber = *result.EpisodeNumber
	} else {
		episodeNumber = tvInfo.Episode
	}

	// Use parser's episode title if result title is empty
	if result.Title == "" && tvInfo != nil && tvInfo.EpisodeTitle != "" {
		episodeTitle = tvInfo.EpisodeTitle
	}

	episode := &media.TVEpisode{
		Media: media.Media{
			LibraryID:       libraryID,
			Title:           episodeTitle,
			FilePath:        result.FilePath,
			FileSize:        checkpoint.FileSize,
			FileHash:        checkpoint.FileHash,
			Duration:        int(result.Duration),
			IsExtra:         isExtra(result.FilePath),
			Width:           result.Width,
			Height:          result.Height,
			VideoCodec:      result.VideoCodec,
			AudioCodec:      result.AudioCodec,
			Bitrate:         result.Bitrate,
			FrameRate:       result.FrameRate,
			ContainerFormat: result.ContainerFormat,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
		ShowTitle:    showTitle,
		Season:       season,
		Episode:      episodeNumber,
		EpisodeTitle: episodeTitle,
	}

	// Try to enhance metadata from NFO file
	nfoPath, err := nfo.FindEpisodeNFO(result.FilePath)
	if err == nil && nfoPath != "" {
		nfoMetadata, err := nfo.ParseEpisodeNFO(nfoPath)
		if err == nil && nfoMetadata != nil {
			// Populate episode metadata from NFO
			if nfoMetadata.Title != "" {
				episode.EpisodeTitle = nfoMetadata.Title
				episode.Media.Title = nfoMetadata.Title
			}
			if nfoMetadata.ShowTitle != "" {
				episode.ShowTitle = nfoMetadata.ShowTitle
			}
			if nfoMetadata.Season > 0 {
				episode.Season = nfoMetadata.Season
			}
			if nfoMetadata.Episode > 0 {
				episode.Episode = nfoMetadata.Episode
			}
			if nfoMetadata.Plot != "" {
				episode.Description = nfoMetadata.Plot
			}
			if !nfoMetadata.AirDate.IsZero() {
				episode.AirDate = nfoMetadata.AirDate.Format("2006-01-02")
			}
			if nfoMetadata.RuntimeMinutes > 0 {
				episode.Media.Duration = nfoMetadata.RuntimeMinutes * 60
			}
			episode.IMDbID = nfoMetadata.IMDbID
			episode.TVDbID = nfoMetadata.TVDbID
		}
	}

	// Check if episode already exists
	existing, err := uc.mediaRepo.GetByFilePath(ctx, libraryID, result.FilePath)
	if err == nil && existing != nil {
		// Update existing entry
		episode.Media.ID = existing.ID
		episode.Media.Type = "tv_episode"
		if err := uc.mediaRepo.Update(ctx, &episode.Media); err != nil {
			fmt.Printf("failed to update media %s: %v\n", result.FilePath, err)
		}
		if err := uc.tvRepo.UpdateTVEpisode(ctx, episode); err != nil {
			fmt.Printf("failed to update TV episode metadata %s: %v\n", result.FilePath, err)
		}
		// Extract and catalog images (even for existing episodes to populate cache)
		uc.extractImagesForEpisode(ctx, episode, result.FilePath, libraryID)
		return
	}

	// Create new entry - let TV repository handle both media and episode records
	episode.Media.Type = "tv_episode"
	if err := uc.tvRepo.CreateTVEpisode(ctx, episode); err != nil {
		fmt.Printf("failed to create TV episode %s: %v\n", result.FilePath, err)
		return
	}

	// Extract and catalog images for the episode, show, and season
	uc.extractImagesForEpisode(ctx, episode, result.FilePath, libraryID)
}

// processMusicTrack creates or updates a music track entry
func (uc *ScanLibraryUseCase) processMusicTrack(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint) {
	// Use coordinator's parsed metadata
	title := result.Title
	artist := result.Artist
	album := result.Album
	trackNumber := 0
	year := 0
	albumArtist := ""
	discNumber := 0
	genre := ""

	if result.TrackNumber != nil {
		trackNumber = *result.TrackNumber
	}
	if result.Year != nil {
		year = *result.Year
	}

	// Try to extract additional metadata if coordinator's result is incomplete
	// This handles fields like album artist, disc number, and genre that aren't in ScanResult
	if artist == "" || album == "" || genre == "" {
		extractor := music.NewExtractor()
		musicInfo, err := extractor.ExtractMetadata(result.FilePath)
		if err == nil && musicInfo != nil {
			// Fill in missing fields from ID3 tags
			if title == "" {
				title = musicInfo.Title
			}
			if artist == "" {
				artist = musicInfo.Artist
			}
			if album == "" {
				album = musicInfo.Album
			}
			if albumArtist == "" {
				albumArtist = musicInfo.AlbumArtist
			}
			if genre == "" {
				genre = musicInfo.Genre
			}
			if trackNumber == 0 {
				trackNumber = musicInfo.TrackNumber
			}
			if discNumber == 0 {
				discNumber = musicInfo.DiscNumber
			}
			if year == 0 {
				year = musicInfo.Year
			}
		}
	}

	// Prepare extended metadata fields
	var (
		totalTracks         int
		totalDiscs          int
		releaseDate         string
		lyricist            string
		isrc                string
		releaseType         string
		compilation         bool
		originalTitle       string
		publisher           string
		musicBrainzTrackID  string
		musicBrainzAlbumID  string
		musicBrainzArtistID string
		composer            string
	)

	// Get extended metadata if we have the extractor result
	if extractor := music.NewExtractor(); extractor != nil {
		if musicInfo, err := extractor.ExtractMetadata(result.FilePath); err == nil && musicInfo != nil {
			totalTracks = musicInfo.TotalTracks
			totalDiscs = musicInfo.TotalDiscs
			releaseDate = musicInfo.ReleaseDate
			lyricist = musicInfo.Lyricist
			isrc = musicInfo.ISRC
			releaseType = musicInfo.ReleaseType
			compilation = musicInfo.Compilation
			originalTitle = musicInfo.OriginalTitle
			publisher = musicInfo.Publisher
			musicBrainzTrackID = musicInfo.MusicBrainzTrackID
			musicBrainzAlbumID = musicInfo.MusicBrainzAlbumID
			musicBrainzArtistID = musicInfo.MusicBrainzArtistID
			composer = musicInfo.Composer
		}
	}

	track := &media.MusicTrack{
		Media: media.Media{
			LibraryID:       libraryID,
			Title:           title,
			FilePath:        result.FilePath,
			FileSize:        checkpoint.FileSize,
			FileHash:        checkpoint.FileHash,
			Duration:        int(result.Duration),
			IsExtra:         isExtra(result.FilePath),
			AudioCodec:      result.AudioCodec,
			Bitrate:         result.Bitrate,
			ContainerFormat: result.ContainerFormat,
			Type:            "music_track",
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
		// Basic metadata
		Artist:      artist,
		Album:       album,
		AlbumArtist: albumArtist,
		TrackNumber: trackNumber,
		DiscNumber:  discNumber,
		Genre:       genre,
		Year:        year,
		Composer:    composer,
		// Extended metadata
		TotalTracks:         totalTracks,
		TotalDiscs:          totalDiscs,
		ReleaseDate:         releaseDate,
		Lyricist:            lyricist,
		ISRC:                isrc,
		ReleaseType:         releaseType,
		Compilation:         compilation,
		OriginalTitle:       originalTitle,
		Publisher:           publisher,
		MusicBrainzTrackID:  musicBrainzTrackID,
		MusicBrainzAlbumID:  musicBrainzAlbumID,
		MusicBrainzArtistID: musicBrainzArtistID,
	}

	// Check if track already exists
	existing, err := uc.mediaRepo.GetByFilePath(ctx, libraryID, result.FilePath)
	if err == nil && existing != nil {
		// Update existing entry
		track.Media.ID = existing.ID
		if err := uc.mediaRepo.Update(ctx, &track.Media); err != nil {
			fmt.Printf("failed to update media %s: %v\n", result.FilePath, err)
		}
		if err := uc.musicRepo.UpdateMusicTrack(ctx, track); err != nil {
			fmt.Printf("failed to update music track metadata %s: %v\n", result.FilePath, err)
		}
		// Extract album and artist images (even for existing tracks to populate cache)
		uc.extractImagesForTrack(ctx, track, result.FilePath)
		return
	}

	// Prepare artist entity if artist info is available
	var artistEntity *media.Artist
	if artist != "" {
		artistEntity = &media.Artist{
			LibraryID:           libraryID,
			Name:                artist,
			MusicBrainzArtistID: musicBrainzArtistID,
			Genre:               genre,
		}
	}

	// Prepare album entity if album info is available
	var albumEntity *media.Album
	if album != "" {
		// Use album artist if available, otherwise fall back to track artist
		effectiveAlbumArtist := albumArtist
		if effectiveAlbumArtist == "" {
			effectiveAlbumArtist = artist
		}

		albumEntity = &media.Album{
			LibraryID:          libraryID,
			Title:              album,
			AlbumArtist:        effectiveAlbumArtist,
			Artist:             artist,
			Year:               year,
			ReleaseDate:        releaseDate,
			Genre:              genre,
			TotalTracks:        totalTracks,
			TotalDiscs:         totalDiscs,
			RecordLabel:        publisher,
			ReleaseType:        releaseType,
			Compilation:        compilation,
			MusicBrainzAlbumID: musicBrainzAlbumID,
		}
	}

	// Create track with artist and album entities in a single transaction
	// This ensures all-or-nothing semantics - no orphaned records on failure
	if err := uc.musicRepo.CreateMusicTrackWithEntities(ctx, track, artistEntity, albumEntity); err != nil {
		fmt.Printf("failed to create music track %s: %v\n", result.FilePath, err)
		return
	}

	// Extract and catalog images for the album and artist
	uc.extractImagesForTrack(ctx, track, result.FilePath)
}

// GetProgress retrieves the current progress of a scan job
func (uc *ScanLibraryUseCase) GetProgress(ctx context.Context, jobID int64) (ScanProgressResponse, error) {
	job, err := uc.scanJobRepo.GetByID(ctx, jobID)
	if err != nil {
		return ScanProgressResponse{}, fmt.Errorf("failed to get scan job: %w", err)
	}

	return ToScanProgressResponse(job), nil
}

// GetLatestScan retrieves the most recent scan for a library
func (uc *ScanLibraryUseCase) GetLatestScan(ctx context.Context, libraryID int64) (ScanProgressResponse, error) {
	job, err := uc.scanJobRepo.GetLatestByLibrary(ctx, libraryID)
	if err != nil {
		return ScanProgressResponse{}, fmt.Errorf("failed to get latest scan: %w", err)
	}

	return ToScanProgressResponse(job), nil
}

// GetScanHistory retrieves scan history for a library
func (uc *ScanLibraryUseCase) GetScanHistory(ctx context.Context, libraryID int64, limit int32) (ScanHistoryResponse, error) {
	jobs, err := uc.scanJobRepo.ListByLibrary(ctx, libraryID, limit)
	if err != nil {
		return ScanHistoryResponse{}, fmt.Errorf("failed to get scan history: %w", err)
	}

	return ToScanHistoryResponse(jobs), nil
}

// cleanupStaleMedia removes media database records and associated images for files that no longer exist on disk
//
// IMPORTANT: This only deletes database records and image cache files, NOT actual media files.
// The media files were already removed by the user from disk - we're just cleaning up our catalog.
func (uc *ScanLibraryUseCase) cleanupStaleMedia(ctx context.Context, libraryID int64, foundFiles map[string]bool) {
	// Get all media items for this library
	allMedia, err := uc.mediaRepo.ListByLibrary(ctx, libraryID)
	if err != nil {
		fmt.Printf("warning: failed to list media for stale cleanup: %v\n", err)
		return
	}

	// Count how many files would be marked as stale
	staleCount := 0
	for _, m := range allMedia {
		if !foundFiles[m.FilePath] {
			staleCount++
		}
	}

	// SAFETY: Don't delete if >10% of library is "stale"
	// This likely indicates scan failure (permission error, network issue, etc.), not actual deletions
	// Better to leave stale records than accidentally delete valid media entries
	if len(allMedia) > 0 {
		stalePercent := float64(staleCount) / float64(len(allMedia)) * 100
		if stalePercent > 10.0 {
			fmt.Printf("error: refusing to cleanup - too many files marked stale (stale=%d, total=%d, percentage=%.1f%%). This likely indicates a scan failure, not actual file deletions.\n",
				staleCount, len(allMedia), stalePercent)
			return
		}
	}

	if staleCount == 0 {
		fmt.Printf("info: no stale media to cleanup\n")
		return
	}

	fmt.Printf("info: cleaning up %d stale media records (%.1f%% of library)\n",
		staleCount, float64(staleCount)/float64(len(allMedia))*100)

	// Track hashes for cleanup
	var hashesToClean []string

	// Find media that's in the database but not on disk
	for _, m := range allMedia {
		if !foundFiles[m.FilePath] {
			// This media file no longer exists on disk - delete it from database

			// Collect image hashes for this media before deletion
			mediaHashes := CollectImageHashesForMedia(ctx, uc.imageRepo, m.ID)
			hashesToClean = append(hashesToClean, mediaHashes...)

			// Delete the media database record (cascades to images, transcode jobs, etc.)
			if err := uc.mediaRepo.Delete(ctx, m.ID); err != nil {
				fmt.Printf("warning: failed to delete stale media %d (%s): %v\n", m.ID, m.FilePath, err)
			} else {
				fmt.Printf("info: removed stale media from library: %s\n", m.FilePath)
			}
		}
	}

	// Clean up image cache files for all the removed media
	if uc.imageCleanup != nil && len(hashesToClean) > 0 {
		if err := uc.imageCleanup.CleanCacheForHashes(ctx, hashesToClean); err != nil {
			fmt.Printf("warning: failed to clean image cache during library scan: %v\n", err)
		}
	}
}

// extractTVShowAndSeasonImages extracts images for a TV show and season
// This is a helper to avoid code duplication between create and update paths
func (uc *ScanLibraryUseCase) extractTVShowAndSeasonImages(ctx context.Context, showTitle string, libraryID int64, showDir string, seasonNumber int) {
	// Get show ID by title (show was created/ensured by CreateTVEpisode)
	show, err := uc.tvRepo.GetTVShowByTitle(ctx, libraryID, showTitle)
	if err != nil {
		fmt.Printf("failed to get TV show for image extraction: %v\n", err)
		return
	}

	// Extract show images
	if uc.extractShowImages != nil {
		if err := uc.extractShowImages.Execute(ctx, showDir, images.MediaTypeTVShow, int(show.ID)); err != nil {
			fmt.Printf("failed to extract images for show %s: %v\n", showTitle, err)
		}
	}

	// Get season ID
	season, err := uc.tvRepo.GetTVSeasonByShowAndNumber(ctx, show.ID, int64(seasonNumber))
	if err != nil {
		fmt.Printf("failed to get TV season for image extraction: %v\n", err)
		return
	}

	// Extract season images
	if uc.extractSeasonImages != nil {
		if err := uc.extractSeasonImages.Execute(ctx, showDir, seasonNumber, images.MediaTypeTVSeason, int(season.ID)); err != nil {
			fmt.Printf("failed to extract images for season %d: %v\n", seasonNumber, err)
		}
	}
}

// tryMarkArtistProcessed atomically checks if an artist has been processed and marks it as processed
// Returns true if this is the first time the artist is being processed (caller should extract images)
// Returns false if the artist was already processed (caller should skip extraction)
// This uses LoadOrStore for atomic check-and-set to prevent race conditions
func (uc *ScanLibraryUseCase) tryMarkArtistProcessed(artistName string) bool {
	_, alreadyProcessed := uc.processedArtists.LoadOrStore(artistName, true)
	return !alreadyProcessed // Return true if this is the first time
}

// extractImagesForMovie extracts and catalogs images for a movie
// Shared helper to eliminate duplication between create and update paths
func (uc *ScanLibraryUseCase) extractImagesForMovie(ctx context.Context, movie *media.Movie, filePath string) {
	if uc.extractMovieImages == nil {
		return
	}

	mediaID := int(movie.Media.ID)
	if err := uc.extractMovieImages.Execute(ctx, filePath, images.MediaTypeMovie, mediaID, &mediaID); err != nil {
		fmt.Printf("failed to extract images for movie %s: %v\n", filePath, err)
	}
}

// extractImagesForEpisode extracts images for a TV episode, its show, and season
// Shared helper to eliminate duplication between create and update paths
func (uc *ScanLibraryUseCase) extractImagesForEpisode(ctx context.Context, episode *media.TVEpisode, filePath string, libraryID int64) {
	// Extract episode images
	if uc.extractEpisodeImages != nil {
		mediaID := int(episode.Media.ID)
		if err := uc.extractEpisodeImages.Execute(ctx, filePath, images.MediaTypeTVEpisode, mediaID, &mediaID); err != nil {
			fmt.Printf("failed to extract images for episode %s: %v\n", filePath, err)
		}
	}

	// Extract show and season images
	showDir := filepath.Dir(filepath.Dir(filePath))
	uc.extractTVShowAndSeasonImages(ctx, episode.ShowTitle, libraryID, showDir, episode.Season)
}

// extractImagesForTrack extracts images for a music track (album and artist)
// Shared helper to eliminate duplication between create and update paths
func (uc *ScanLibraryUseCase) extractImagesForTrack(ctx context.Context, track *media.MusicTrack, filePath string) {
	// Extract album images
	if uc.extractMusicImages != nil && track.Album != "" {
		albumDir := filepath.Dir(filePath)
		entityID := int(track.Media.ID)
		if err := uc.extractMusicImages.Execute(ctx, albumDir, images.MediaTypeMusicAlbum, entityID); err != nil {
			fmt.Printf("failed to extract images for album %s: %v\n", track.Album, err)
		}
	}

	// Extract artist images (once per artist)
	if uc.extractArtistImages != nil && track.Artist != "" {
		artistDir := filepath.Dir(filepath.Dir(filePath)) // Parent of album dir
		entityID := int(track.Media.ID)

		// Atomically check and mark artist as processed (prevents race condition)
		if uc.tryMarkArtistProcessed(track.Artist) {
			if err := uc.extractArtistImages.Execute(ctx, artistDir, images.MediaTypeMusicArtist, entityID); err != nil {
				fmt.Printf("failed to extract artist images for %s: %v\n", track.Artist, err)
			}
		}
	}
}
