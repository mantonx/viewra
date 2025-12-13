package library

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/application/library/scan/cleanup"
	"github.com/mantonx/viewra/internal/application/library/scan/discovery"
	scanmedia "github.com/mantonx/viewra/internal/application/library/scan/media"
	"github.com/mantonx/viewra/internal/application/library/scan/processing"
	"github.com/mantonx/viewra/internal/application/library/scan/scanutil"
	"github.com/mantonx/viewra/internal/application/library/scan/status"
	domainImages "github.com/mantonx/viewra/internal/domain/images"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
	"github.com/mantonx/viewra/internal/infrastructure/system"
)

// Internal type aliases for sub-package types used in this file
type (
	discoveryContext            = discovery.Context
	checkpointProcessingContext = processing.CheckpointContext
)

// =============================================================================
// ScanLibraryUseCase - Main Struct
// =============================================================================

// ScanLibraryUseCase handles the business logic for scanning a library
type ScanLibraryUseCase struct {
	mediaRepos         *scan.MediaRepositories
	scanRepos          *scan.ScanRepositories
	imageRepo          domainImages.Repository
	imageCleanup       ImageCleanupExecutor
	incrementalScanner *discovery.IncrementalScanner
	coordinator        *filesystem.Coordinator
	config             scan.Config
	systemProfile      *system.Profile
	logger             *slog.Logger

	// Image extractors
	movieImageExtractor   MovieImageExtractor
	episodeImageExtractor TVEpisodeImageExtractor
	showImageExtractor    TVShowImageExtractor
	seasonImageExtractor  TVSeasonImageExtractor
	albumImageExtractor   MusicAlbumImageExtractor
	artistImageExtractor  MusicArtistImageExtractor
	trackImageExtractor   MusicTrackImageExtractor

	// Per-session deduplication
	processedArtists scanutil.AtomicDeduplicator
	processedShows   scanutil.AtomicDeduplicator
}

// NewScanLibraryUseCase creates a new instance of ScanLibraryUseCase
func NewScanLibraryUseCase(
	mediaRepos *scan.MediaRepositories,
	scanRepos *scan.ScanRepositories,
	movieImageExtractor MovieImageExtractor,
	episodeImageExtractor TVEpisodeImageExtractor,
	showImageExtractor TVShowImageExtractor,
	seasonImageExtractor TVSeasonImageExtractor,
	albumImageExtractor MusicAlbumImageExtractor,
	artistImageExtractor MusicArtistImageExtractor,
	trackImageExtractor MusicTrackImageExtractor,
	imageRepo domainImages.Repository,
	imageCleanup ImageCleanupExecutor,
	config scan.Config,
	systemProfile *system.Profile,
	logger *slog.Logger,
) *ScanLibraryUseCase {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	config = config.WithDefaults()

	incrementalScanner := discovery.NewIncrementalScanner(scanRepos.ScanState, logger)

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

// =============================================================================
// Public API Methods
// =============================================================================

// StartScan initiates a new scan for a library
func (uc *ScanLibraryUseCase) StartScan(ctx context.Context, libraryID int64) (scan.StartScanResponse, error) {
	lib, err := uc.mediaRepos.Library.GetByID(ctx, libraryID)
	if err != nil {
		return scan.StartScanResponse{}, fmt.Errorf("failed to get library: %w", err)
	}

	running, err := uc.scanRepos.ScanJob.ListRunning(ctx)
	if err != nil {
		return scan.StartScanResponse{}, fmt.Errorf("failed to check running scans: %w", err)
	}
	for _, job := range running {
		if job.LibraryID == libraryID {
			return scan.StartScanResponse{}, scanner.ErrAlreadyRunning
		}
	}

	if uc.logger == nil {
		uc.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	var estimatedTotal int64
	previousScan, err := uc.scanRepos.ScanJob.GetLatestByLibrary(ctx, libraryID)
	if err == nil && previousScan != nil && previousScan.Status == scanner.ScanStatusCompleted {
		estimatedTotal = previousScan.FilesFound
		uc.logger.Info("Using previous scan for progress estimation",
			"library_id", libraryID,
			"previous_files_found", estimatedTotal)
	}

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
		return scan.StartScanResponse{}, fmt.Errorf("failed to create scan job: %w", err)
	}

	uc.startScanBackground(job.ID, lib, "scan goroutine panicked")
	return scan.ToStartScanResponse(job), nil
}

// ResumeScan resumes a paused scan job
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

// ResumeStuckScans automatically resumes scans interrupted by server restart
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

// GetProgress retrieves the current progress of a scan job
func (uc *ScanLibraryUseCase) GetProgress(ctx context.Context, jobID int64) (scan.ScanProgressResponse, error) {
	job, err := uc.scanRepos.ScanJob.GetByID(ctx, jobID)
	if err != nil {
		return scan.ScanProgressResponse{}, fmt.Errorf("failed to get scan job: %w", err)
	}
	return scan.ToScanProgressResponse(job), nil
}

// GetLatestScan retrieves the most recent scan for a library
func (uc *ScanLibraryUseCase) GetLatestScan(ctx context.Context, libraryID int64) (scan.ScanProgressResponse, error) {
	job, err := uc.scanRepos.ScanJob.GetLatestByLibrary(ctx, libraryID)
	if err != nil {
		return scan.ScanProgressResponse{}, fmt.Errorf("failed to get latest scan: %w", err)
	}
	return scan.ToScanProgressResponse(job), nil
}

// GetScanHistory retrieves scan history for a library
func (uc *ScanLibraryUseCase) GetScanHistory(ctx context.Context, libraryID int64, limit int32) (scan.ScanHistoryResponse, error) {
	jobs, err := uc.scanRepos.ScanJob.ListByLibrary(ctx, libraryID, limit)
	if err != nil {
		return scan.ScanHistoryResponse{}, fmt.Errorf("failed to get scan history: %w", err)
	}
	return scan.ToScanHistoryResponse(jobs), nil
}

// GetScanStatus retrieves the enriched scan status for a library
func (uc *ScanLibraryUseCase) GetScanStatus(ctx context.Context, libraryID int64) (*status.Result, error) {
	return status.GetScanStatus(ctx, uc.statusDeps(), libraryID)
}

// =============================================================================
// Dependency Bundles
// =============================================================================

func (uc *ScanLibraryUseCase) mediaDeps() *scanmedia.Deps {
	return &scanmedia.Deps{
		MediaRepos:       uc.mediaRepos,
		ScanRepos:        uc.scanRepos,
		ImageRepo:        uc.imageRepo,
		MovieExtractor:   uc.movieImageExtractor,
		EpisodeExtractor: uc.episodeImageExtractor,
		ShowExtractor:    uc.showImageExtractor,
		SeasonExtractor:  uc.seasonImageExtractor,
		AlbumExtractor:   uc.albumImageExtractor,
		ArtistExtractor:  uc.artistImageExtractor,
		TrackExtractor:   uc.trackImageExtractor,
		ProcessedArtists: &uc.processedArtists,
		ProcessedShows:   &uc.processedShows,
		Coordinator:      uc.coordinator,
		Logger:           uc.logger,
	}
}

func (uc *ScanLibraryUseCase) processingDeps() *processing.Deps {
	return &processing.Deps{
		ScanRepos:      uc.scanRepos,
		MediaRepos:     uc.mediaRepos,
		MediaProcessor: uc,
		Coordinator:    uc.coordinator,
		Config:         &uc.config,
		SystemProfile:  uc.systemProfile,
		Logger:         uc.logger,
	}
}

func (uc *ScanLibraryUseCase) discoveryDeps() *discovery.Deps {
	return &discovery.Deps{
		ScanRepos:     uc.scanRepos,
		Config:        &uc.config,
		SystemProfile: uc.systemProfile,
		Coordinator:   uc.coordinator,
		Logger:        uc.logger,
		IsMediaFile:   uc.isMediaFile,
		IncrScanner:   uc.incrementalScanner,
	}
}

func (uc *ScanLibraryUseCase) statusDeps() *status.Deps {
	return &status.Deps{
		ScanRepos: uc.scanRepos,
		Logger:    uc.logger,
	}
}

func (uc *ScanLibraryUseCase) cleanupDeps() *cleanup.Deps {
	return &cleanup.Deps{
		MediaRepos:   uc.mediaRepos,
		ImageRepo:    uc.imageRepo,
		ImageCleanup: uc.imageCleanup,
		Logger:       uc.logger,
	}
}

// =============================================================================
// MediaProcessor Interface Implementation
// =============================================================================

var _ processing.MediaProcessor = (*ScanLibraryUseCase)(nil)

func (uc *ScanLibraryUseCase) ProcessMovie(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error) {
	return uc.processMovie(ctx, libraryID, result, checkpoint, cache)
}

func (uc *ScanLibraryUseCase) ProcessTVEpisode(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error) {
	return uc.processTVEpisode(ctx, libraryID, result, checkpoint, cache)
}

func (uc *ScanLibraryUseCase) ProcessMusicTrack(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error) {
	return uc.processMusicTrack(ctx, libraryID, result, checkpoint, cache)
}

// =============================================================================
// Internal Scan Logic
// =============================================================================

func (uc *ScanLibraryUseCase) startScanBackground(jobID int64, lib *library.Library, panicContext string) {
	scanCtx, cancel := context.WithTimeout(context.Background(), uc.config.Timeout)
	go func() {
		defer cancel()
		defer uc.recoverFromPanic(jobID, lib.ID, panicContext)
		uc.runScan(scanCtx, jobID, lib)
	}()
}

func (uc *ScanLibraryUseCase) runScan(ctx context.Context, jobID int64, lib *library.Library) {
	uc.initializeScanSession(ctx, lib)

	currentJob, err := uc.scanRepos.ScanJob.GetByID(ctx, jobID)
	if err != nil {
		uc.logger.Error("failed to get scan job", "job_id", jobID, "error", err)
		uc.completeJobWithError(ctx, jobID, err)
		return
	}

	if uc.canResumeFromCheckpoints(ctx, jobID, currentJob, lib) {
		return
	}

	uc.logger.Info("starting fresh scan", "library_id", lib.ID)
	uc.runFreshScan(ctx, jobID, lib)
}

func (uc *ScanLibraryUseCase) initializeScanSession(ctx context.Context, lib *library.Library) {
	uc.processedArtists.Reset()
	uc.processedShows.Reset()

	if uc.systemProfile != nil {
		uc.systemProfile.UpdateForLibraryPath(ctx, lib.Path)
		uc.logger.Info("detected storage for library",
			"library_path", lib.Path,
			"storage_type", uc.systemProfile.Storage.Type,
			"is_remote", uc.systemProfile.Storage.IsRemote)
	}
}

func (uc *ScanLibraryUseCase) canResumeFromCheckpoints(ctx context.Context, jobID int64, currentJob *scanner.ScanJob, lib *library.Library) bool {
	stats, err := uc.scanRepos.Checkpoint.GetStats(ctx, jobID)
	if err != nil {
		return false
	}

	if stats.TotalFiles == 0 {
		return false
	}

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

func (uc *ScanLibraryUseCase) validateCheckpointCompleteness(ctx context.Context, jobID int64, currentJob *scanner.ScanJob, stats *scanner.CheckpointStats) bool {
	totalCheckpoints := stats.TotalFiles
	filesFound := currentJob.FilesFound

	minExpected := int64(1)
	if filesFound > 0 {
		minExpected = filesFound / 100
		if minExpected < 1 {
			minExpected = 1
		}
	}

	if totalCheckpoints >= minExpected || filesFound == 0 {
		return true
	}

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

func (uc *ScanLibraryUseCase) resumeScanFromCheckpoints(ctx context.Context, jobID int64, lib *library.Library) {
	stats, err := uc.scanRepos.Checkpoint.GetStats(ctx, jobID)
	if err != nil {
		uc.logger.Error("Failed to get checkpoint stats for resume", "job_id", jobID, "error", err)
		return
	}

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

	progress := &scanner.Progress{
		FilesFound:     currentJob.FilesFound,
		FilesProcessed: stats.ProcessedFiles,
		ErrorCount:     stats.FailedFiles,
		WarningCount:   stats.WarningFiles,
		Phase:          scanner.ScanPhaseProcessing,
		EstimatedTotal: currentJob.EstimatedTotal,
		DiscoveryDone:  true,
	}
	if err := uc.scanRepos.ScanJob.UpdateProgress(ctx, jobID, progress); err != nil {
		uc.logger.Warn("Failed to initialize resume progress", "job_id", jobID, "error", err)
	}

	hashingDone := make(chan struct{})
	close(hashingDone)
	uc.processFilesWithCheckpoints(ctx, jobID, lib, hashingDone, nil)
}

func (uc *ScanLibraryUseCase) handleStuckScan(ctx context.Context, job *scanner.ScanJob) {
	lib, err := uc.mediaRepos.Library.GetByID(ctx, job.LibraryID)
	if err != nil {
		uc.markStuckScanFailed(ctx, job, err)
		return
	}

	stats, err := uc.scanRepos.Checkpoint.GetStats(ctx, job.ID)
	if err != nil {
		uc.logger.Warn("failed to get checkpoint stats for stuck scan, resuming anyway",
			"scan_id", job.ID,
			"error", err)
		uc.startScanBackground(job.ID, lib, "resumed stuck scan goroutine panicked")
		return
	}

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
	uc.completeJobSafely(ctx, failedJob)
}

func (uc *ScanLibraryUseCase) markStuckScanCompleted(ctx context.Context, job *scanner.ScanJob, stats *scanner.CheckpointStats) {
	uc.logger.Info("stuck scan has no pending files, marking as completed",
		"scan_id", job.ID,
		"files_found", job.FilesFound)
	uc.completeJobFromStats(ctx, job.ID, job.FilesFound, stats)
}

func (uc *ScanLibraryUseCase) markScanCompleted(ctx context.Context, jobID int64, stats *scanner.CheckpointStats) {
	uc.completeJobFromStats(ctx, jobID, stats.TotalFiles, stats)
}

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
	uc.completeJobSafely(ctx, completedJob)
}

func (uc *ScanLibraryUseCase) completeJobWithError(ctx context.Context, jobID int64, err error) {
	job := &scanner.ScanJob{
		ID:           jobID,
		Status:       scanner.ScanStatusFailed,
		ErrorMessage: err.Error(),
		CompletedAt:  &[]time.Time{time.Now()}[0],
	}
	uc.completeJobSafely(ctx, job)
}

func (uc *ScanLibraryUseCase) completeJobSafely(ctx context.Context, job *scanner.ScanJob) {
	if err := uc.scanRepos.ScanJob.Complete(ctx, job); err != nil {
		if scanner.IsScanJobDeleted(err) {
			uc.logger.Debug("scan job was deleted before completion",
				"job_id", job.ID,
				"status", job.Status)
			return
		}
		uc.logger.Error("failed to complete scan job",
			"job_id", job.ID,
			"status", job.Status,
			"error", err)
	}
}

// =============================================================================
// Discovery Delegates
// =============================================================================

func (uc *ScanLibraryUseCase) runFreshScan(ctx context.Context, jobID int64, lib *library.Library) {
	dctx, err := uc.initDiscoveryContext(ctx, jobID, lib)
	if err != nil {
		return
	}

	uc.phaseCountFiles(ctx, dctx)

	discoveredFiles, err := uc.phaseWalkDirectory(ctx, dctx)
	if err != nil {
		uc.completeJobWithError(ctx, jobID, err)
		return
	}

	diff := uc.phaseDetermineChanges(ctx, dctx, discoveredFiles)
	if diff == nil {
		return
	}

	uc.phaseHandleDeleted(ctx, dctx, diff)
	uc.phaseHashAndProcess(ctx, dctx, diff)
}

func (uc *ScanLibraryUseCase) initDiscoveryContext(ctx context.Context, jobID int64, lib *library.Library) (*discoveryContext, error) {
	currentJob, err := uc.scanRepos.ScanJob.GetByID(ctx, jobID)
	if err != nil {
		uc.logger.Error("Failed to get job for progress callback", "job_id", jobID, "error", err)
		return nil, err
	}
	walker := discovery.CreateWalker(uc.discoveryDeps())
	return discovery.NewContext(jobID, lib, currentJob, walker), nil
}

func (uc *ScanLibraryUseCase) phaseCountFiles(ctx context.Context, dctx *discoveryContext) {
	discovery.PhaseCountFiles(ctx, dctx, uc.discoveryDeps())
}

func (uc *ScanLibraryUseCase) phaseWalkDirectory(ctx context.Context, dctx *discoveryContext) ([]scanner.FileInfo, error) {
	deps := uc.discoveryDeps()
	progressCallback := func(filesDiscovered int64) {
		uc.NewProgressUpdate(dctx.JobID).
			Phase(scanner.ScanPhaseDiscovering).
			FilesFound(filesDiscovered).
			EstimatedTotal(dctx.CurrentJob.EstimatedTotal).
			UpdateAsync(ctx)
	}

	discoveredFiles, err := discovery.PhaseWalkDirectory(ctx, dctx, deps, progressCallback)
	if err != nil {
		return nil, err
	}

	warnings := uc.validateDiscovery(ctx, dctx.Lib.ID, int64(len(discoveredFiles)), dctx.DiscoveryStats)
	for _, warning := range warnings {
		uc.logger.Warn("Discovery validation warning", "job_id", dctx.JobID, "warning", warning)
	}

	if err := uc.NewProgressUpdate(dctx.JobID).
		Phase(scanner.ScanPhaseProcessing).
		FilesFound(int64(len(discoveredFiles))).
		EstimatedTotal(dctx.CurrentJob.EstimatedTotal).
		DiscoveryDone().
		Update(ctx); err != nil {
		uc.logger.Warn("Failed to update discovery completion status", "job_id", dctx.JobID, "error", err)
	}

	return discoveredFiles, nil
}

func (uc *ScanLibraryUseCase) phaseDetermineChanges(ctx context.Context, dctx *discoveryContext, discoveredFiles []scanner.FileInfo) *scanner.ScanDiff {
	diff := discovery.PhaseDetermineChanges(ctx, dctx, uc.discoveryDeps(), discoveredFiles)
	if diff == nil {
		job := &scanner.ScanJob{
			ID:             dctx.JobID,
			Status:         scanner.ScanStatusCompleted,
			FilesFound:     int64(len(discoveredFiles)),
			FilesProcessed: 0,
			ErrorCount:     0,
			Progress:       100.0,
			CompletedAt:    &[]time.Time{time.Now()}[0],
			Phase:          scanner.ScanPhaseCompleted,
			DiscoveryDone:  true,
		}
		uc.completeJobSafely(ctx, job)
		return nil
	}
	return diff
}

func (uc *ScanLibraryUseCase) phaseHandleDeleted(ctx context.Context, dctx *discoveryContext, diff *scanner.ScanDiff) {
	discovery.PhaseHandleDeleted(ctx, dctx, uc.discoveryDeps(), diff)
}

func (uc *ScanLibraryUseCase) phaseHashAndProcess(ctx context.Context, dctx *discoveryContext, diff *scanner.ScanDiff) {
	filesToProcess := append(diff.NewFiles, diff.ModifiedFiles...)

	uc.logger.Info("processing files",
		"total", len(filesToProcess),
		"new", len(diff.NewFiles),
		"modified", len(diff.ModifiedFiles),
		"skipped", len(diff.UnchangedFiles))

	startTime := time.Now()

	var processingWg sync.WaitGroup
	processingWg.Add(1)
	processingErrChan := make(chan error, 1)
	hashingDone := make(chan struct{})

	go func() {
		defer processingWg.Done()
		defer uc.recoverFromPanicWithError(dctx.JobID, dctx.Lib.ID, "processing goroutine panicked", processingErrChan)
		uc.processFilesWithCheckpoints(ctx, dctx.JobID, dctx.Lib, hashingDone, dctx.DiscoveryStats)
	}()

	if err := uc.hashAndStreamCheckpoints(ctx, filesToProcess, dctx.JobID, dctx.Lib.ID); err != nil {
		uc.logger.Error("failed to create checkpoints", "error", err)
		close(hashingDone)
		uc.completeJobWithError(ctx, dctx.JobID, err)
		return
	}

	close(hashingDone)
	processingWg.Wait()

	select {
	case err := <-processingErrChan:
		if err != nil {
			uc.logger.Error("processing failed", "error", err)
			uc.completeJobWithError(ctx, dctx.JobID, err)
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

func (uc *ScanLibraryUseCase) validateDiscovery(ctx context.Context, libraryID int64, filesDiscovered int64, stats *filesystem.WalkStats) []string {
	var warnings []string
	warnings = append(warnings, discovery.CheckWalkStatsErrors(stats)...)
	warnings = append(warnings, uc.checkAgainstPreviousScan(ctx, libraryID, filesDiscovered, stats)...)
	return warnings
}

func (uc *ScanLibraryUseCase) checkAgainstPreviousScan(ctx context.Context, libraryID int64, filesDiscovered int64, stats *filesystem.WalkStats) []string {
	previousJobs, err := uc.scanRepos.ScanJob.ListByLibrary(ctx, libraryID, scan.PreviousJobsToCompare)
	if err != nil || len(previousJobs) <= 1 {
		return nil
	}

	for _, prevJob := range previousJobs {
		if prevJob.Status != scanner.ScanStatusCompleted || prevJob.FilesFound == 0 {
			continue
		}

		var warnings []string
		if warning := discovery.DetectFileDrop(filesDiscovered, prevJob.FilesFound); warning != "" {
			warnings = append(warnings, warning)
		}
		if warning := discovery.DetectRepeatedErrors(stats, prevJob); warning != "" {
			warnings = append(warnings, warning)
		}
		return warnings
	}
	return nil
}

// =============================================================================
// Processing Delegates
// =============================================================================

func (uc *ScanLibraryUseCase) processFilesWithCheckpoints(ctx context.Context, jobID int64, lib *library.Library, hashingDone <-chan struct{}, discoveryStats *filesystem.WalkStats) {
	deps := uc.processingDeps()
	pctx := processing.InitCheckpointProcessing(ctx, deps, jobID, lib)
	defer close(pctx.CheckpointsChan)

	processing.StartCheckpointWorkers(ctx, deps, pctx)
	processing.RunCheckpointProcessingLoop(ctx, deps, pctx, hashingDone)
	pctx.WorkerWg.Wait()
	uc.completeScan(ctx, pctx, discoveryStats)
}

func (uc *ScanLibraryUseCase) completeScan(ctx context.Context, pctx *processing.CheckpointContext, discoveryStats *filesystem.WalkStats) {
	deps := uc.processingDeps()
	stats, _ := deps.ScanRepos.Checkpoint.GetStats(ctx, pctx.JobID)

	currentJob, err := deps.ScanRepos.ScanJob.GetByID(ctx, pctx.JobID)
	if err != nil {
		uc.logger.Error("failed to get current job for completion", "job_id", pctx.JobID, "error", err)
		return
	}

	job := uc.buildCompletedJob(pctx.JobID, currentJob, stats, discoveryStats)
	uc.logScanCompletion(pctx.JobID, pctx.Lib.ID, currentJob.FilesFound, stats)

	if err := deps.ScanRepos.ScanJob.Complete(ctx, job); err != nil {
		if scanner.IsScanJobDeleted(err) {
			uc.logger.Info("Scan job deleted before completion, exiting gracefully", "job_id", pctx.JobID)
			return
		}
		uc.logger.Error("failed to mark scan job as complete", "job_id", pctx.JobID, "error", err)
	}

	if stats.FailedFiles == 0 && stats.CompletedFiles == stats.TotalFiles {
		if uc.imageRepo != nil && uc.imageCleanup != nil {
			uc.cleanupStaleMedia(ctx, pctx.Lib.ID, pctx.FoundFiles)
		}
	}

	uc.cleanupCheckpoints(ctx, pctx.JobID)
}

func (uc *ScanLibraryUseCase) hashAndStreamCheckpoints(ctx context.Context, filesToProcess []scanner.FileInfo, jobID int64, libraryID int64) error {
	return processing.HashAndStreamCheckpoints(ctx, uc.processingDeps(), filesToProcess, jobID, libraryID)
}

func (uc *ScanLibraryUseCase) buildCompletedJob(jobID int64, currentJob *scanner.ScanJob, stats *scanner.CheckpointStats, discoveryStats *filesystem.WalkStats) *scanner.ScanJob {
	return processing.BuildCompletedJob(jobID, currentJob, stats, discoveryStats, uc.validateDiscovery)
}

func (uc *ScanLibraryUseCase) logScanCompletion(jobID, libraryID, filesFound int64, stats *scanner.CheckpointStats) {
	processing.LogScanCompletion(uc.processingDeps(), jobID, libraryID, filesFound, stats)
}

func (uc *ScanLibraryUseCase) cleanupCheckpoints(ctx context.Context, jobID int64) {
	processing.CleanupCheckpoints(ctx, uc.processingDeps(), jobID)
}

// =============================================================================
// Media Processing Delegates
// =============================================================================

func (uc *ScanLibraryUseCase) processMovie(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error) {
	return scanmedia.ProcessMovie(ctx, uc.mediaDeps(), libraryID, result, checkpoint, cache)
}

func (uc *ScanLibraryUseCase) processTVEpisode(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error) {
	return scanmedia.ProcessTVEpisode(ctx, uc.mediaDeps(), libraryID, result, checkpoint, cache)
}

func (uc *ScanLibraryUseCase) processMusicTrack(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error) {
	return scanmedia.ProcessMusicTrack(ctx, uc.mediaDeps(), libraryID, result, checkpoint, cache)
}

func (uc *ScanLibraryUseCase) processMediaWithCache(ctx context.Context, libraryID int64, filePath string, cache *sync.Map, callbacks scanmedia.UpsertCallbacks) (*int64, error) {
	return scanmedia.ProcessMediaWithCache(ctx, uc.mediaDeps(), libraryID, filePath, cache, callbacks)
}

func (uc *ScanLibraryUseCase) persistMediaTracks(ctx context.Context, mediaID int64, result *scanner.ScanResult) {
	scanmedia.PersistMediaTracks(ctx, uc.mediaDeps(), mediaID, result)
}

func (uc *ScanLibraryUseCase) extractImagesForMovie(ctx context.Context, movie *media.Movie, filePath string) {
	scanmedia.ExtractImagesForMovie(ctx, uc.mediaDeps(), movie, filePath)
}

func (uc *ScanLibraryUseCase) extractImagesForEpisode(ctx context.Context, episode *media.TVEpisode, filePath string, libraryID int64) {
	scanmedia.ExtractImagesForEpisode(ctx, uc.mediaDeps(), episode, filePath, libraryID)
}

func (uc *ScanLibraryUseCase) extractImagesForTrack(ctx context.Context, track *media.MusicTrack, filePath string) {
	scanmedia.ExtractImagesForTrack(ctx, uc.mediaDeps(), track, filePath)
}

func (uc *ScanLibraryUseCase) cleanupStaleMedia(ctx context.Context, libraryID int64, foundFiles map[string]bool) {
	cleanup.CleanupStaleMedia(ctx, uc.cleanupDeps(), libraryID, foundFiles)
}

// =============================================================================
// Utility Methods
// =============================================================================

func (uc *ScanLibraryUseCase) isMediaFile(ext string) bool {
	return scanutil.IsMediaFile(ext)
}

func (uc *ScanLibraryUseCase) calculateProcessingTimeout(fileSize int64) time.Duration {
	isRemote := uc.systemProfile != nil && uc.systemProfile.Storage.IsRemote
	return scanutil.CalculateProcessingTimeout(fileSize, scanutil.TimeoutConfig{
		BaseFileTimeout:      uc.config.BaseFileTimeout,
		RemoteStorageTimeout: uc.config.RemoteStorageTimeout,
		MaxExtraTimeout:      uc.config.MaxExtraTimeout,
		IsRemoteStorage:      isRemote,
	})
}

func (uc *ScanLibraryUseCase) statWithTimeout(ctx context.Context, path string, timeout time.Duration) (os.FileInfo, error) {
	return scanutil.StatWithTimeout(ctx, path, timeout)
}

func (uc *ScanLibraryUseCase) NewProgressUpdate(jobID int64) *status.ProgressUpdate {
	return status.NewProgressUpdate(uc.scanRepos.ScanJob, uc.logger, jobID)
}

// =============================================================================
// Panic Recovery
// =============================================================================

func (uc *ScanLibraryUseCase) logPanic(r any, description string, fields ...any) {
	allFields := append([]any{
		"panic", r,
		"stack_trace", string(debug.Stack()),
	}, fields...)
	uc.logger.Error("PANIC: "+description, allFields...)
}

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
			uc.logger.Error("failed to mark panicked scan job as failed", "job_id", jobID, "error", err)
		}
	}
}

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

func (uc *ScanLibraryUseCase) recoverWorkerPanic(jobID int64, workerID int) {
	if r := recover(); r != nil {
		uc.logPanic(r, "worker goroutine panicked", "worker_id", workerID, "job_id", jobID)
	}
}

// =============================================================================
// Test Helpers - Wrapper methods for testing internal functionality
// =============================================================================

func (uc *ScanLibraryUseCase) initCheckpointProcessing(ctx context.Context, jobID int64, lib *library.Library) *checkpointProcessingContext {
	return processing.InitCheckpointProcessing(ctx, uc.processingDeps(), jobID, lib)
}

func (uc *ScanLibraryUseCase) startCheckpointWorkers(ctx context.Context, pctx *checkpointProcessingContext) {
	processing.StartCheckpointWorkers(ctx, uc.processingDeps(), pctx)
}

func (uc *ScanLibraryUseCase) runCheckpointProcessingLoop(ctx context.Context, pctx *checkpointProcessingContext, hashingDone <-chan struct{}) {
	processing.RunCheckpointProcessingLoop(ctx, uc.processingDeps(), pctx, hashingDone)
}

func (uc *ScanLibraryUseCase) checkScanStatus(ctx context.Context, jobID int64) (shouldBreak, shouldContinue bool) {
	return processing.CheckScanStatus(ctx, uc.processingDeps(), jobID)
}

func (uc *ScanLibraryUseCase) getNumWorkers() int {
	return processing.GetNumWorkers(uc.systemProfile, scan.DefaultProcessingWorkers, uc.logger)
}

func (uc *ScanLibraryUseCase) loadMediaCache(ctx context.Context, libraryID int64) *sync.Map {
	return processing.LoadMediaCache(ctx, uc.processingDeps(), libraryID)
}

func (uc *ScanLibraryUseCase) updateCheckpointStatus(ctx context.Context, checkpoint *scanner.ScanCheckpoint, status scanner.CheckpointStatus, message string, category scanner.ErrorCategory, action string) (shouldAbort bool) {
	return processing.UpdateCheckpointStatus(ctx, uc.processingDeps(), checkpoint, status, message, category, action)
}

func (uc *ScanLibraryUseCase) processCheckpointWorker(ctx context.Context, lib *library.Library, checkpoint *scanner.ScanCheckpoint, maxRetries int, foundFilesMu *sync.Mutex, foundFiles map[string]bool, existingMediaCache *sync.Map) {
	processing.ProcessCheckpointWorker(ctx, uc.processingDeps(), lib, checkpoint, maxRetries, foundFilesMu, foundFiles, existingMediaCache)
}

func (uc *ScanLibraryUseCase) handleCheckpointError(ctx context.Context, lib *library.Library, checkpoint *scanner.ScanCheckpoint, err error, maxRetries int) {
	processing.HandleCheckpointError(ctx, uc.processingDeps(), lib, checkpoint, err, maxRetries)
}

func (uc *ScanLibraryUseCase) retryCheckpoint(ctx context.Context, checkpoint *scanner.ScanCheckpoint, err error, maxRetries int) {
	processing.RetryCheckpoint(ctx, uc.processingDeps(), checkpoint, err, maxRetries)
}

func (uc *ScanLibraryUseCase) handleCheckpointWarning(ctx context.Context, checkpoint *scanner.ScanCheckpoint, foundFilesMu *sync.Mutex, foundFiles map[string]bool) {
	processing.HandleCheckpointWarning(ctx, uc.processingDeps(), checkpoint, foundFilesMu, foundFiles)
}

func (uc *ScanLibraryUseCase) handleCheckpointSuccess(ctx context.Context, checkpoint *scanner.ScanCheckpoint, foundFilesMu *sync.Mutex, foundFiles map[string]bool) {
	processing.HandleCheckpointSuccess(ctx, uc.processingDeps(), checkpoint, foundFilesMu, foundFiles)
}

func (uc *ScanLibraryUseCase) processFileWithCheckpoint(ctx context.Context, lib *library.Library, checkpoint *scanner.ScanCheckpoint, existingMediaCache *sync.Map) (bool, error) {
	return processing.ProcessFileWithCheckpoint(ctx, uc.processingDeps(), lib, checkpoint, existingMediaCache)
}

func (uc *ScanLibraryUseCase) updateProgressIfDue(ctx context.Context, jobID int64, ticker *time.Ticker) {
	select {
	case <-ticker.C:
		deps := uc.processingDeps()
		stats, _ := deps.ScanRepos.Checkpoint.GetStats(ctx, jobID)
		currentJob, err := deps.ScanRepos.ScanJob.GetByID(ctx, jobID)
		if err == nil && currentJob != nil {
			_ = uc.NewProgressUpdate(jobID).
				FromJob(currentJob).
				FromCheckpointStats(stats).
				Update(ctx)
		}
	default:
	}
}

func (uc *ScanLibraryUseCase) enrichWithScanState(ctx context.Context, libraryID int64, job *scanner.ScanJob, result *status.Result) {
	status.EnrichWithScanState(ctx, uc.statusDeps(), libraryID, job, result)
}

func (uc *ScanLibraryUseCase) enrichWithETA(ctx context.Context, jobID int64, filesFound int64, result *status.Result) {
	status.EnrichWithETA(ctx, uc.statusDeps(), jobID, filesFound, result)
}

func (uc *ScanLibraryUseCase) discoverExternalSubtitles(ctx context.Context, mediaID int64, videoPath string) {
	scanmedia.DiscoverExternalSubtitles(ctx, uc.mediaDeps(), mediaID, videoPath)
}

func (uc *ScanLibraryUseCase) enrichTVShowMetadataFromNFO(ctx context.Context, showID int64, episodeFilePath string) {
	scanmedia.EnrichTVShowMetadataFromNFO(ctx, uc.mediaDeps(), showID, episodeFilePath)
}

func (uc *ScanLibraryUseCase) processMultiEpisodeFile(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, existingMediaCache *sync.Map, showTitle string, season int, episodeStart int, episodeEnd int, episodeTitle string) (*int64, error) {
	return scanmedia.ProcessMultiEpisodeFile(ctx, uc.mediaDeps(), libraryID, result, checkpoint, existingMediaCache, showTitle, season, episodeStart, episodeEnd, episodeTitle)
}

func (uc *ScanLibraryUseCase) extractTVShowAndSeasonImages(ctx context.Context, showTitle string, libraryID int64, showDir string, seasonNumber int) {
	show, err := uc.mediaRepos.TV.GetTVShowByTitle(ctx, libraryID, showTitle)
	if err != nil {
		uc.logger.Warn("failed to get TV show for image extraction", "show_title", showTitle, "error", err)
		return
	}

	if uc.tryMarkShowMetadataProcessed(showTitle) {
		episodeFilePath := showDir + "/dummy.mkv"
		scanmedia.EnrichTVShowMetadataFromNFO(ctx, uc.mediaDeps(), show.ID, episodeFilePath)
	}

	if uc.showImageExtractor != nil {
		if err := uc.showImageExtractor.Execute(ctx, showDir, domainImages.MediaTypeTVShow, int(show.ID)); err != nil {
			uc.logger.Warn("failed to extract images for show", "show_title", showTitle, "show_dir", showDir, "error", err)
		}
	}

	season, err := uc.mediaRepos.TV.GetTVSeasonByShowAndNumber(ctx, show.ID, int64(seasonNumber))
	if err != nil {
		uc.logger.Warn("failed to get TV season for image extraction", "show_title", showTitle, "season", seasonNumber, "error", err)
		return
	}

	if uc.seasonImageExtractor != nil {
		if err := uc.seasonImageExtractor.Execute(ctx, showDir, seasonNumber, domainImages.MediaTypeTVSeason, int(season.ID)); err != nil {
			uc.logger.Warn("failed to extract images for season", "show_title", showTitle, "season", seasonNumber, "error", err)
		}
	}
}

func (uc *ScanLibraryUseCase) tryMarkArtistProcessed(artistName string) bool {
	return uc.processedArtists.TryMark(artistName)
}

func (uc *ScanLibraryUseCase) tryMarkShowMetadataProcessed(showTitle string) bool {
	return uc.processedShows.TryMark(showTitle)
}

func (uc *ScanLibraryUseCase) recordImageWarning(ctx context.Context, libraryID int64, filePath string, err error) {
	if setErr := uc.scanRepos.ScanState.SetWarning(ctx, libraryID, filePath, err.Error(), "image_extraction"); setErr != nil {
		uc.logger.Warn("failed to set image extraction warning in scan_state",
			"library_id", libraryID,
			"file_path", filePath,
			"original_error", err.Error(),
			"set_warning_error", setErr.Error())
	}
}

func (uc *ScanLibraryUseCase) createWalker() *filesystem.Walker {
	return discovery.CreateWalker(uc.discoveryDeps())
}

func (uc *ScanLibraryUseCase) logDiscoveryStats(jobID int64, stats *filesystem.WalkStats) {
	discovery.LogDiscoveryStats(uc.logger, jobID, stats)
}
