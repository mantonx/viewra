package library

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/domain/images"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
	"github.com/mantonx/viewra/internal/infrastructure/system"
)

// MediaRepositories bundles media-related repositories
type MediaRepositories struct {
	Library library.Repository
	Media   media.Repository
	Movie   media.MovieRepository
	TV      media.TVRepository
	Music   media.MusicRepository
}

// ScanRepositories bundles scan-related repositories
type ScanRepositories struct {
	ScanJob    scanner.ScanJobRepository
	Checkpoint scanner.CheckpointRepository
	ScanState  scanner.ScanStateRepository
}

// ScanConfig bundles scan configuration parameters
type ScanConfig struct {
	Timeout          time.Duration
	ParallelWalkers  int // Number of concurrent directory walkers (0 = sequential)
	ProgressInterval int // Log progress every N files (0 = disabled)
}

// ScanLibraryUseCase handles the business logic for scanning a library
type ScanLibraryUseCase struct {
	mediaRepos         *MediaRepositories
	scanRepos          *ScanRepositories
	imageRepo          images.Repository
	imageExtractor     ImageExtractor // Unified image extractor (replaces 6 separate executors)
	imageCleanup       ImageCleanupExecutor
	incrementalScanner *IncrementalScanner
	coordinator        *filesystem.Coordinator // Reused for all files (was created per file - major bottleneck!)
	config             ScanConfig
	systemProfile      *system.Profile
	logger             *slog.Logger

	// Artist deduplication tracking (per scan session)
	// Using sync.Map for lock-free concurrent access (fixes race condition)
	processedArtists sync.Map // string -> bool
}

// NewScanLibraryUseCase creates a new instance of ScanLibraryUseCase
func NewScanLibraryUseCase(
	mediaRepos *MediaRepositories,
	scanRepos *ScanRepositories,
	imageExtractor ImageExtractor,
	imageRepo images.Repository,
	imageCleanup ImageCleanupExecutor,
	config ScanConfig,
	systemProfile *system.Profile,
	logger *slog.Logger,
) *ScanLibraryUseCase {
	// Use a no-op logger if none provided (for tests)
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	// Create incremental scanner
	incrementalScanner := NewIncrementalScanner(scanRepos.ScanState, logger)

	// Create a single coordinator instance to reuse for all files (major performance optimization!)
	coordinatorConfig := filesystem.DefaultCoordinatorConfig()
	coordinatorConfig.Logger = logger
	coordinator := filesystem.NewCoordinator(coordinatorConfig)

	return &ScanLibraryUseCase{
		mediaRepos:         mediaRepos,
		scanRepos:          scanRepos,
		imageExtractor:     imageExtractor,
		imageRepo:          imageRepo,
		imageCleanup:       imageCleanup,
		incrementalScanner: incrementalScanner,
		coordinator:        coordinator,
		config:             config,
		systemProfile:      systemProfile,
		logger:             logger,
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

	// Start background scan with timeout
	// Use a new context with timeout (not derived from request context which may cancel early)
	scanCtx, cancel := context.WithTimeout(context.Background(), uc.config.Timeout)

	// Start the scan goroutine with proper cleanup and panic recovery
	go func() {
		defer cancel() // Always cancel to prevent context leak

		// Recover from panics to prevent crashing the entire application
		defer func() {
			if r := recover(); r != nil {
				uc.logger.Error("PANIC: scan goroutine panicked",
					"job_id", job.ID,
					"library_id", lib.ID,
					"panic", r,
					"stack_trace", string(debug.Stack()))

				// Mark job as failed
				failedJob := &scanner.ScanJob{
					ID:           job.ID,
					Status:       scanner.ScanStatusFailed,
					ErrorMessage: fmt.Sprintf("scan panicked: %v", r),
					CompletedAt:  &[]time.Time{time.Now()}[0],
				}
				if err := uc.scanRepos.ScanJob.Complete(context.Background(), failedJob); err != nil {
					uc.logger.Error("failed to mark panicked scan job as failed",
						"job_id", job.ID,
						"error", err)
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
	runningScans, err := uc.scanRepos.ScanJob.ListRunning(ctx)
	if err != nil {
		return fmt.Errorf("failed to get running scans: %w", err)
	}

	if len(runningScans) == 0 {
		uc.logger.Debug("no stuck scans found during startup")
		return nil
	}

	uc.logger.Info("found stuck scans, automatically resuming",
		"count", len(runningScans))

	// Resume each stuck scan
	for _, job := range runningScans {
		// Get the library for this scan
		lib, err := uc.mediaRepos.Library.GetByID(ctx, job.LibraryID)
		if err != nil {
			uc.logger.Error("failed to get library for stuck scan",
				"library_id", job.LibraryID,
				"scan_id", job.ID,
				"error", err)
			// Mark this scan as failed since we can't resume it
			failedJob := &scanner.ScanJob{
				ID:           job.ID,
				Status:       scanner.ScanStatusFailed,
				ErrorMessage: fmt.Sprintf("library not found during resume: %v", err),
				CompletedAt:  &[]time.Time{time.Now()}[0],
			}
			_ = uc.scanRepos.ScanJob.Complete(ctx, failedJob)
			continue
		}

		// Check if there are checkpoints to resume from
		stats, err := uc.scanRepos.Checkpoint.GetStats(ctx, job.ID)
		if err != nil || stats.PendingFiles == 0 {
			// No pending files, mark as completed
			// Preserve FilesFound from original job (not stats.TotalFiles which is checkpoint count)
			uc.logger.Info("stuck scan has no pending files, marking as completed",
				"scan_id", job.ID,
				"files_found", job.FilesFound)
			completedJob := &scanner.ScanJob{
				ID:             job.ID,
				Status:         scanner.ScanStatusCompleted,
				FilesFound:     job.FilesFound,         // Preserve from discovery, not checkpoint count
				FilesProcessed: stats.ProcessedFiles,   // Total processed (completed + failed + warnings)
				ErrorCount:     stats.FailedFiles,
				WarningCount:   stats.WarningFiles,     // Include warning count for consistency
				Progress:       stats.GetProgress(),
				CompletedAt:    &[]time.Time{time.Now()}[0],
				Phase:          scanner.ScanPhaseCompleted,
				DiscoveryDone:  true,
			}
			_ = uc.scanRepos.ScanJob.Complete(ctx, completedJob)
			continue
		}

		uc.logger.Info("resuming stuck scan",
			"scan_id", job.ID,
			"library_id", lib.ID,
			"pending", stats.PendingFiles,
			"completed", stats.CompletedFiles)

		// Start background goroutine to resume the scan
		scanCtx, cancel := context.WithTimeout(context.Background(), uc.config.Timeout)
		go func(jobID int64, library *library.Library) {
			defer cancel()

			// Recover from panics
			defer func() {
				if r := recover(); r != nil {
					uc.logger.Error("PANIC: resumed scan goroutine panicked",
						"job_id", jobID,
						"library_id", library.ID,
						"panic", r,
						"stack_trace", string(debug.Stack()))
					failedJob := &scanner.ScanJob{
						ID:           jobID,
						Status:       scanner.ScanStatusFailed,
						ErrorMessage: fmt.Sprintf("resumed scan panicked: %v", r),
						CompletedAt:  &[]time.Time{time.Now()}[0],
					}
					_ = uc.scanRepos.ScanJob.Complete(context.Background(), failedJob)
				}
			}()

			// Resume the scan using the existing runScan method
			uc.runScan(scanCtx, jobID, library)
		}(job.ID, lib)
	}

	return nil
}

// ResumeScan resumes a paused scan job by starting the scan processing goroutine
func (uc *ScanLibraryUseCase) ResumeScan(ctx context.Context, jobID int64) error {
	// Get the scan job
	job, err := uc.scanRepos.ScanJob.GetByID(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get scan job: %w", err)
	}

	// Verify the job is paused
	if job.Status != scanner.ScanStatusPaused {
		return fmt.Errorf("scan job is not paused (current status: %s)", job.Status)
	}

	// Get the library for this scan
	lib, err := uc.mediaRepos.Library.GetByID(ctx, job.LibraryID)
	if err != nil {
		return fmt.Errorf("failed to get library: %w", err)
	}

	// Update status to running
	if err := uc.scanRepos.ScanJob.UpdateStatus(ctx, jobID, scanner.ScanStatusRunning); err != nil {
		return fmt.Errorf("failed to update scan status: %w", err)
	}

	uc.logger.Info("resuming scan from user request",
		"job_id", jobID,
		"library_id", lib.ID,
		"files_processed", job.FilesProcessed,
		"files_found", job.FilesFound)

	// Start background goroutine to resume the scan
	scanCtx, cancel := context.WithTimeout(context.Background(), uc.config.Timeout)
	go func(jobID int64, library *library.Library) {
		defer cancel()

		// Recover from panics
		defer func() {
			if r := recover(); r != nil {
				uc.logger.Error("PANIC: resumed scan goroutine panicked",
					"job_id", jobID,
					"library_id", library.ID,
					"panic", r,
					"stack_trace", string(debug.Stack()))
				failedJob := &scanner.ScanJob{
					ID:           jobID,
					Status:       scanner.ScanStatusFailed,
					ErrorMessage: fmt.Sprintf("resumed scan panicked: %v", r),
					CompletedAt:  &[]time.Time{time.Now()}[0],
				}
				_ = uc.scanRepos.ScanJob.Complete(context.Background(), failedJob)
			}
		}()

		// Resume the scan using the existing runScan method
		uc.runScan(scanCtx, jobID, library)
	}(job.ID, lib)

	return nil
}

// runScan executes the actual scan in the background with checkpoint-based resumability
func (uc *ScanLibraryUseCase) runScan(ctx context.Context, jobID int64, lib *library.Library) {
	// Initialize artist deduplication tracking for this scan session
	uc.processedArtists = sync.Map{}

	// Update system profile with library-specific storage detection
	// This ensures we get correct worker counts for network vs local storage
	if uc.systemProfile != nil {
		uc.systemProfile.UpdateForLibraryPath(ctx, lib.Path)
		uc.logger.Info("detected storage for library",
			"library_path", lib.Path,
			"storage_type", uc.systemProfile.Storage.Type,
			"is_remote", uc.systemProfile.Storage.IsRemote)
	}

	// Check if there are existing checkpoints to resume from
	// IMPORTANT: Only resume if ALL checkpoints were created for discovered files
	// If checkpoint creation was interrupted, we need to start fresh
	currentJob, err := uc.scanRepos.ScanJob.GetByID(ctx, jobID)
	if err != nil {
		uc.logger.Error("failed to get scan job", "job_id", jobID, "error", err)
		uc.completeJobWithError(ctx, jobID, err)
		return
	}

	stats, err := uc.scanRepos.Checkpoint.GetStats(ctx, jobID)
	if err == nil && stats.PendingFiles > 0 {
		// Checkpoints exist - verify they represent ALL discovered files before resuming
		// Calculate expected checkpoint count (files needing processing)
		// Note: We can't know exact count without re-running incremental scan,
		// but we can detect if checkpoint creation was obviously interrupted:
		// If we have FilesFound but very few checkpoints, creation was interrupted
		totalCheckpoints := stats.TotalFiles // pending + completed + failed
		filesFound := currentJob.FilesFound

		// If checkpoint count seems reasonable (at least 1% of discovered files for incremental,
		// or we're early in discovery so FilesFound isn't set yet), resume
		minExpected := int64(1) // At minimum we expect some checkpoints
		if filesFound > 0 {
			minExpected = filesFound / 100 // At least 1% of files should have checkpoints
			if minExpected < 1 {
				minExpected = 1
			}
		}

		if totalCheckpoints >= minExpected || filesFound == 0 {
			// Checkpoint count seems reasonable - safe to resume
			uc.logger.Info("resuming scan from checkpoints",
				"files_found", filesFound,
				"total_checkpoints", totalCheckpoints,
				"pending", stats.PendingFiles,
				"completed", stats.CompletedFiles)
			uc.resumeScanFromCheckpoints(ctx, jobID, lib)
			return
		} else {
			// Too few checkpoints compared to discovered files - checkpoint creation was interrupted
			uc.logger.Warn("incomplete checkpoint creation detected, cleaning up and starting fresh",
				"job_id", jobID,
				"files_found", filesFound,
				"checkpoints_created", totalCheckpoints,
				"expected_minimum", minExpected)
			if deleteErr := uc.scanRepos.Checkpoint.DeleteByJobID(ctx, jobID); deleteErr != nil {
				uc.logger.Error("failed to delete partial checkpoints", "error", deleteErr)
			}
		}
	}

	// Fresh scan - discover files and create checkpoints
	uc.logger.Info("starting fresh scan", "library_id", lib.ID)
	uc.runFreshScan(ctx, jobID, lib)
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
