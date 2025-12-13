package library

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime/debug"
	"time"

	appImages "github.com/mantonx/viewra/internal/application/images"
	domainImages "github.com/mantonx/viewra/internal/domain/images"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
	"github.com/mantonx/viewra/internal/infrastructure/system"
)

// ScanLibraryUseCase handles the business logic for scanning a library
type ScanLibraryUseCase struct {
	mediaRepos         *MediaRepositories
	scanRepos          *ScanRepositories
	imageRepo          domainImages.Repository
	imageCleanup       ImageCleanupExecutor
	incrementalScanner *IncrementalScanner
	coordinator        *filesystem.Coordinator // Reused for all files (was created per file - major bottleneck!)
	config             ScanConfig
	systemProfile      *system.Profile
	logger             *slog.Logger

	// Image extraction use cases - called directly instead of through adapter
	movieImageExtractor   *appImages.ExtractMovieImagesUseCase
	episodeImageExtractor *appImages.ExtractTVEpisodeImagesUseCase
	showImageExtractor    *appImages.ExtractTVShowImagesUseCase
	seasonImageExtractor  *appImages.ExtractTVSeasonImagesUseCase
	albumImageExtractor   *appImages.ExtractMusicAlbumImagesUseCase
	artistImageExtractor  *appImages.ExtractMusicArtistImagesUseCase
	trackImageExtractor   *appImages.ExtractMusicTrackImagesUseCase

	// Artist deduplication tracking (per scan session)
	// Using AtomicDeduplicator for lock-free concurrent access (fixes race condition)
	processedArtists AtomicDeduplicator

	// TV show metadata enrichment tracking (per scan session)
	// Ensures we only parse tvshow.nfo and update show metadata once per show
	processedShows AtomicDeduplicator
}

// NewScanLibraryUseCase creates a new instance of ScanLibraryUseCase
func NewScanLibraryUseCase(
	mediaRepos *MediaRepositories,
	scanRepos *ScanRepositories,
	movieImageExtractor *appImages.ExtractMovieImagesUseCase,
	episodeImageExtractor *appImages.ExtractTVEpisodeImagesUseCase,
	showImageExtractor *appImages.ExtractTVShowImagesUseCase,
	seasonImageExtractor *appImages.ExtractTVSeasonImagesUseCase,
	albumImageExtractor *appImages.ExtractMusicAlbumImagesUseCase,
	artistImageExtractor *appImages.ExtractMusicArtistImagesUseCase,
	trackImageExtractor *appImages.ExtractMusicTrackImagesUseCase,
	imageRepo domainImages.Repository,
	imageCleanup ImageCleanupExecutor,
	config ScanConfig,
	systemProfile *system.Profile,
	logger *slog.Logger,
) *ScanLibraryUseCase {
	// Use a no-op logger if none provided (for tests)
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	// Apply defaults for any unset config values
	config = config.WithDefaults()

	// Create incremental scanner
	incrementalScanner := NewIncrementalScanner(scanRepos.ScanState, logger)

	// Create a single coordinator instance to reuse for all files (major performance optimization!)
	coordinatorConfig := filesystem.DefaultCoordinatorConfig()
	coordinatorConfig.Logger = logger
	coordinator := filesystem.NewCoordinator(coordinatorConfig)

	return &ScanLibraryUseCase{
		mediaRepos:            mediaRepos,
		scanRepos:             scanRepos,
		movieImageExtractor:   movieImageExtractor,
		episodeImageExtractor: episodeImageExtractor,
		showImageExtractor:    showImageExtractor,
		seasonImageExtractor:  seasonImageExtractor,
		albumImageExtractor:   albumImageExtractor,
		artistImageExtractor:  artistImageExtractor,
		trackImageExtractor:   trackImageExtractor,
		imageRepo:             imageRepo,
		imageCleanup:          imageCleanup,
		incrementalScanner:    incrementalScanner,
		coordinator:           coordinator,
		config:                config,
		systemProfile:         systemProfile,
		logger:                logger,
	}
}

// StartScan initiates a new scan for a library
func (uc *ScanLibraryUseCase) StartScan(ctx context.Context, libraryID int64) (StartScanResponse, error) {
	// Verify library exists
	lib, err := uc.mediaRepos.Library.GetByID(ctx, libraryID)
	if err != nil {
		return StartScanResponse{}, fmt.Errorf("failed to get library: %w", err)
	}

	// Check for existing running scan
	running, err := uc.scanRepos.ScanJob.ListRunning(ctx)
	if err != nil {
		return StartScanResponse{}, fmt.Errorf("failed to check running scans: %w", err)
	}
	for _, job := range running {
		if job.LibraryID == libraryID {
			return StartScanResponse{}, scanner.ErrAlreadyRunning
		}
	}

	// Ensure logger is initialized (for tests that bypass constructor)
	if uc.logger == nil {
		uc.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	// Get estimated total from previous completed scan for progress calculation
	var estimatedTotal int64
	previousScan, err := uc.scanRepos.ScanJob.GetLatestByLibrary(ctx, libraryID)
	if err == nil && previousScan != nil && previousScan.Status == scanner.ScanStatusCompleted {
		estimatedTotal = previousScan.FilesFound
		uc.logger.Info("Using previous scan for progress estimation",
			"library_id", libraryID,
			"previous_files_found", estimatedTotal)
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
		Phase:          scanner.ScanPhaseDiscovering,
		EstimatedTotal: estimatedTotal,
		DiscoveryDone:  false,
	}

	if err := uc.scanRepos.ScanJob.Create(ctx, job); err != nil {
		return StartScanResponse{}, fmt.Errorf("failed to create scan job: %w", err)
	}

	// Start scan in background (not tied to HTTP request context)
	uc.startScanBackground(job.ID, lib, "scan goroutine panicked")

	return ToStartScanResponse(job), nil
}

// ResumeStuckScans automatically resumes scans that were interrupted by server restart or crash.
// This is called during application startup to recover gracefully from unexpected shutdowns.
func (uc *ScanLibraryUseCase) ResumeStuckScans(ctx context.Context) error {
	runningScans, err := uc.scanRepos.ScanJob.ListRunning(ctx)
	if err != nil {
		return fmt.Errorf("failed to get running scans: %w", err)
	}

	if len(runningScans) == 0 {
		uc.logger.Debug("no stuck scans found during startup")
		return nil
	}

	uc.logger.Info("found stuck scans, automatically resuming", "count", len(runningScans))

	for _, job := range runningScans {
		uc.handleStuckScan(ctx, job)
	}

	return nil
}

// handleStuckScan processes a single stuck scan job
func (uc *ScanLibraryUseCase) handleStuckScan(ctx context.Context, job *scanner.ScanJob) {
	lib, err := uc.mediaRepos.Library.GetByID(ctx, job.LibraryID)
	if err != nil {
		uc.markStuckScanFailed(ctx, job, err)
		return
	}

	stats, err := uc.scanRepos.Checkpoint.GetStats(ctx, job.ID)
	if err != nil {
		// Can't get stats - resume the scan to figure out state
		uc.logger.Warn("failed to get checkpoint stats for stuck scan, resuming anyway",
			"scan_id", job.ID,
			"error", err)
		uc.startScanBackground(job.ID, lib, "resumed stuck scan goroutine panicked")
		return
	}

	// Check if scan is truly complete by comparing processed files to files found
	// Note: PendingFiles == 0 is not sufficient because checkpoints are created in batches,
	// so there may be files that haven't been batched yet
	actuallyComplete := stats.PendingFiles == 0 && job.FilesFound > 0 && stats.ProcessedFiles >= job.FilesFound

	if actuallyComplete {
		uc.markStuckScanCompleted(ctx, job, stats)
		return
	}

	uc.logger.Info("resuming stuck scan",
		"scan_id", job.ID,
		"library_id", lib.ID,
		"files_found", job.FilesFound,
		"pending_checkpoints", stats.PendingFiles,
		"processed_checkpoints", stats.ProcessedFiles)

	uc.startScanBackground(job.ID, lib, "resumed stuck scan goroutine panicked")
}

// markStuckScanFailed marks a stuck scan as failed when library cannot be found
func (uc *ScanLibraryUseCase) markStuckScanFailed(ctx context.Context, job *scanner.ScanJob, err error) {
	uc.logger.Error("failed to get library for stuck scan",
		"library_id", job.LibraryID,
		"scan_id", job.ID,
		"error", err)

	failedJob := &scanner.ScanJob{
		ID:           job.ID,
		Status:       scanner.ScanStatusFailed,
		ErrorMessage: fmt.Sprintf("library not found during resume: %v", err),
		CompletedAt:  &[]time.Time{time.Now()}[0],
	}
	_ = uc.scanRepos.ScanJob.Complete(ctx, failedJob)
}

// markStuckScanCompleted marks a stuck scan as completed when no pending work remains
func (uc *ScanLibraryUseCase) markStuckScanCompleted(ctx context.Context, job *scanner.ScanJob, stats *scanner.CheckpointStats) {
	uc.logger.Info("stuck scan has no pending files, marking as completed",
		"scan_id", job.ID,
		"files_found", job.FilesFound)
	uc.completeJobFromStats(ctx, job.ID, job.FilesFound, stats)
}

// markScanCompleted marks a scan as completed using checkpoint stats
func (uc *ScanLibraryUseCase) markScanCompleted(ctx context.Context, jobID int64, stats *scanner.CheckpointStats) {
	uc.completeJobFromStats(ctx, jobID, stats.TotalFiles, stats)
}

// completeJobFromStats marks a scan job as completed using checkpoint statistics
func (uc *ScanLibraryUseCase) completeJobFromStats(ctx context.Context, jobID int64, filesFound int64, stats *scanner.CheckpointStats) {
	completedJob := &scanner.ScanJob{
		ID:             jobID,
		Status:         scanner.ScanStatusCompleted,
		FilesFound:     filesFound,
		FilesProcessed: stats.ProcessedFiles,
		ErrorCount:     stats.FailedFiles,
		WarningCount:   stats.WarningFiles,
		Progress:       stats.GetProgress(),
		CompletedAt:    &[]time.Time{time.Now()}[0],
		Phase:          scanner.ScanPhaseCompleted,
		DiscoveryDone:  true,
	}
	_ = uc.scanRepos.ScanJob.Complete(ctx, completedJob)
}

// startScanBackground starts a scan goroutine with timeout and panic recovery
func (uc *ScanLibraryUseCase) startScanBackground(jobID int64, lib *library.Library, panicContext string) {
	scanCtx, cancel := context.WithTimeout(context.Background(), uc.config.Timeout)
	go func() {
		defer cancel()
		defer uc.recoverFromPanic(jobID, lib.ID, panicContext)
		uc.runScan(scanCtx, jobID, lib)
	}()
}

// ResumeScan resumes a paused scan job by starting the scan processing goroutine
func (uc *ScanLibraryUseCase) ResumeScan(ctx context.Context, jobID int64) error {
	job, err := uc.scanRepos.ScanJob.GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get scan job: %w", err)
	}

	if job.Status != scanner.ScanStatusPaused {
		return fmt.Errorf("scan job is not paused (current status: %s)", job.Status)
	}

	lib, err := uc.mediaRepos.Library.GetByID(ctx, job.LibraryID)
	if err != nil {
		return fmt.Errorf("failed to get library: %w", err)
	}

	if err := uc.scanRepos.ScanJob.UpdateStatus(ctx, jobID, scanner.ScanStatusRunning); err != nil {
		return fmt.Errorf("failed to update scan status: %w", err)
	}

	uc.logger.Info("resuming scan from user request",
		"job_id", jobID,
		"library_id", lib.ID,
		"files_processed", job.FilesProcessed,
		"files_found", job.FilesFound)

	uc.startScanBackground(job.ID, lib, "resumed scan goroutine panicked")
	return nil
}

// runScan executes the actual scan in the background with checkpoint-based resumability
func (uc *ScanLibraryUseCase) runScan(ctx context.Context, jobID int64, lib *library.Library) {
	uc.initializeScanSession(ctx, lib)

	currentJob, err := uc.scanRepos.ScanJob.GetByID(ctx, jobID)
	if err != nil {
		uc.logger.Error("failed to get scan job", "job_id", jobID, "error", err)
		uc.completeJobWithError(ctx, jobID, err)
		return
	}

	// Check if we can resume from existing checkpoints
	if uc.canResumeFromCheckpoints(ctx, jobID, currentJob, lib) {
		return
	}

	// Fresh scan - discover files and create checkpoints
	uc.logger.Info("starting fresh scan", "library_id", lib.ID)
	uc.runFreshScan(ctx, jobID, lib)
}

// initializeScanSession prepares for a scan by resetting tracking state and detecting storage
func (uc *ScanLibraryUseCase) initializeScanSession(ctx context.Context, lib *library.Library) {
	// Reset deduplication tracking for this scan session
	uc.processedArtists.Reset()
	uc.processedShows.Reset()

	// Update system profile with library-specific storage detection
	if uc.systemProfile != nil {
		uc.systemProfile.UpdateForLibraryPath(ctx, lib.Path)
		uc.logger.Info("detected storage for library",
			"library_path", lib.Path,
			"storage_type", uc.systemProfile.Storage.Type,
			"is_remote", uc.systemProfile.Storage.IsRemote)
	}
}

// canResumeFromCheckpoints checks if checkpoints exist and are valid for resuming.
// Returns true if scan was resumed from checkpoints or already complete, false if fresh scan needed.
func (uc *ScanLibraryUseCase) canResumeFromCheckpoints(ctx context.Context, jobID int64, currentJob *scanner.ScanJob, lib *library.Library) bool {
	stats, err := uc.scanRepos.Checkpoint.GetStats(ctx, jobID)
	if err != nil {
		return false
	}

	// No checkpoints at all - need fresh scan
	if stats.TotalFiles == 0 {
		return false
	}

	// All checkpoints processed - scan is complete, mark it and return
	if stats.PendingFiles == 0 {
		uc.logger.Info("scan already complete from checkpoints",
			"job_id", jobID,
			"files_found", currentJob.FilesFound,
			"processed", stats.ProcessedFiles)
		uc.markScanCompleted(ctx, jobID, stats)
		return true
	}

	if uc.validateCheckpointCompleteness(ctx, jobID, currentJob, stats) {
		uc.logger.Info("resuming scan from checkpoints",
			"files_found", currentJob.FilesFound,
			"total_checkpoints", stats.TotalFiles,
			"pending", stats.PendingFiles,
			"completed", stats.CompletedFiles)
		uc.resumeScanFromCheckpoints(ctx, jobID, lib)
		return true
	}

	return false
}

// validateCheckpointCompleteness verifies checkpoints represent enough discovered files.
// If incomplete, cleans up partial checkpoints.
func (uc *ScanLibraryUseCase) validateCheckpointCompleteness(ctx context.Context, jobID int64, currentJob *scanner.ScanJob, stats *scanner.CheckpointStats) bool {
	totalCheckpoints := stats.TotalFiles
	filesFound := currentJob.FilesFound

	// Calculate minimum expected checkpoints (at least 1% of files, or 1 minimum)
	minExpected := int64(1)
	if filesFound > 0 {
		minExpected = filesFound / 100
		if minExpected < 1 {
			minExpected = 1
		}
	}

	// Checkpoint count seems reasonable, or we're early in discovery
	if totalCheckpoints >= minExpected || filesFound == 0 {
		return true
	}

	// Too few checkpoints - checkpoint creation was interrupted
	uc.logger.Warn("incomplete checkpoint creation detected, cleaning up and starting fresh",
		"job_id", jobID,
		"files_found", filesFound,
		"checkpoints_created", totalCheckpoints,
		"expected_minimum", minExpected)

	if deleteErr := uc.scanRepos.Checkpoint.DeleteByJobID(ctx, jobID); deleteErr != nil {
		uc.logger.Error("failed to delete partial checkpoints", "error", deleteErr)
	}

	return false
}

// resumeScanFromCheckpoints resumes a scan from existing checkpoints
func (uc *ScanLibraryUseCase) resumeScanFromCheckpoints(ctx context.Context, jobID int64, lib *library.Library) {
	// Get checkpoint stats to initialize progress correctly
	stats, err := uc.scanRepos.Checkpoint.GetStats(ctx, jobID)
	if err != nil {
		uc.logger.Error("Failed to get checkpoint stats for resume", "job_id", jobID, "error", err)
		return
	}

	// Get the current job to preserve FilesFound from original discovery
	currentJob, err := uc.scanRepos.ScanJob.GetByID(ctx, jobID)
	if err != nil {
		uc.logger.Error("Failed to get scan job for resume", "job_id", jobID, "error", err)
		return
	}

	totalCheckpoints := stats.PendingFiles + stats.CompletedFiles + stats.FailedFiles
	uc.logger.Info("Resuming scan from checkpoints",
		"job_id", jobID,
		"files_found", currentJob.FilesFound,
		"checkpoints", totalCheckpoints,
		"completed", stats.CompletedFiles,
		"pending", stats.PendingFiles)

	// Initialize progress with existing checkpoint counts so progress shows correctly
	// When resuming, discovery is already complete
	// CRITICAL: Preserve FilesFound from original discovery, don't overwrite with checkpoint count
	progress := &scanner.Progress{
		FilesFound:     currentJob.FilesFound, // Preserve from discovery, not checkpoint count
		FilesProcessed: stats.ProcessedFiles,  // Total processed (completed + failed + warnings)
		ErrorCount:     stats.FailedFiles,
		WarningCount:   stats.WarningFiles,
		Phase:          scanner.ScanPhaseProcessing,
		EstimatedTotal: currentJob.EstimatedTotal,
		DiscoveryDone:  true,
	}
	if err := uc.scanRepos.ScanJob.UpdateProgress(ctx, jobID, progress); err != nil {
		uc.logger.Warn("Failed to initialize resume progress", "job_id", jobID, "error", err)
	}

	// When resuming, hashing is already done
	hashingDone := make(chan struct{})
	close(hashingDone)
	// When resuming, we don't have fresh discovery stats, pass nil
	uc.processFilesWithCheckpoints(ctx, jobID, lib, hashingDone, nil)
}

// GetProgress retrieves the current progress of a scan job
func (uc *ScanLibraryUseCase) GetProgress(ctx context.Context, jobID int64) (ScanProgressResponse, error) {
	job, err := uc.scanRepos.ScanJob.GetByID(ctx, jobID)
	if err != nil {
		return ScanProgressResponse{}, fmt.Errorf("failed to get scan job: %w", err)
	}

	return ToScanProgressResponse(job), nil
}

// GetLatestScan retrieves the most recent scan for a library
func (uc *ScanLibraryUseCase) GetLatestScan(ctx context.Context, libraryID int64) (ScanProgressResponse, error) {
	job, err := uc.scanRepos.ScanJob.GetLatestByLibrary(ctx, libraryID)
	if err != nil {
		return ScanProgressResponse{}, fmt.Errorf("failed to get latest scan: %w", err)
	}

	return ToScanProgressResponse(job), nil
}

// GetScanHistory retrieves scan history for a library
func (uc *ScanLibraryUseCase) GetScanHistory(ctx context.Context, libraryID int64, limit int32) (ScanHistoryResponse, error) {
	jobs, err := uc.scanRepos.ScanJob.ListByLibrary(ctx, libraryID, limit)
	if err != nil {
		return ScanHistoryResponse{}, fmt.Errorf("failed to get scan history: %w", err)
	}

	return ToScanHistoryResponse(jobs), nil
}

// completeJobWithError marks a job as failed
func (uc *ScanLibraryUseCase) completeJobWithError(ctx context.Context, jobID int64, err error) {
	job := &scanner.ScanJob{
		ID:           jobID,
		Status:       scanner.ScanStatusFailed,
		ErrorMessage: err.Error(),
		CompletedAt:  &[]time.Time{time.Now()}[0],
	}
	_ = uc.scanRepos.ScanJob.Complete(ctx, job)
}

// logPanic logs a panic with full stack trace. This is the shared logging
// implementation used by all panic recovery functions.
func (uc *ScanLibraryUseCase) logPanic(r any, description string, fields ...any) {
	allFields := append([]any{
		"panic", r,
		"stack_trace", string(debug.Stack()),
	}, fields...)
	uc.logger.Error("PANIC: "+description, allFields...)
}

// recoverFromPanic handles panic recovery for scan goroutines.
// It logs the panic with stack trace and marks the job as failed.
// Usage: defer uc.recoverFromPanic(jobID, libraryID, "context description")
func (uc *ScanLibraryUseCase) recoverFromPanic(jobID, libraryID int64, description string) {
	if r := recover(); r != nil {
		uc.logPanic(r, description, "job_id", jobID, "library_id", libraryID)

		failedJob := &scanner.ScanJob{
			ID:           jobID,
			Status:       scanner.ScanStatusFailed,
			ErrorMessage: fmt.Sprintf("scan panicked: %v", r),
			CompletedAt:  &[]time.Time{time.Now()}[0],
		}
		if err := uc.scanRepos.ScanJob.Complete(context.Background(), failedJob); err != nil {
			uc.logger.Error("failed to mark panicked scan job as failed",
				"job_id", jobID,
				"error", err)
		}
	}
}

// recoverFromPanicWithError handles panic recovery and sends error to a channel.
// Use this variant when the caller needs to know about the panic.
// Usage: defer uc.recoverFromPanicWithError(jobID, libraryID, "context", errChan)
func (uc *ScanLibraryUseCase) recoverFromPanicWithError(jobID, libraryID int64, description string, errChan chan<- error) {
	if r := recover(); r != nil {
		uc.logPanic(r, description, "job_id", jobID, "library_id", libraryID)

		err := fmt.Errorf("%s: %v", description, r)
		select {
		case errChan <- err:
		default:
		}
	}
}

// recoverWorkerPanic handles panic recovery for worker goroutines.
// Unlike recoverFromPanic, this doesn't mark jobs as failed since other workers continue.
// Usage: defer uc.recoverWorkerPanic(jobID, workerID)
func (uc *ScanLibraryUseCase) recoverWorkerPanic(jobID int64, workerID int) {
	if r := recover(); r != nil {
		uc.logPanic(r, "worker goroutine panicked", "worker_id", workerID, "job_id", jobID)
	}
}
