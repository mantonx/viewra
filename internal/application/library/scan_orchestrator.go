package library

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/application/library/scan/cleanup"
	"github.com/mantonx/viewra/internal/application/library/scan/discovery"
	"github.com/mantonx/viewra/internal/application/library/scan/execution"
	scanmedia "github.com/mantonx/viewra/internal/application/library/scan/media"
	"github.com/mantonx/viewra/internal/application/library/scan/processing"
	"github.com/mantonx/viewra/internal/application/library/scan/recovery"
	"github.com/mantonx/viewra/internal/application/library/scan/scanutil"
	"github.com/mantonx/viewra/internal/application/library/scan/status"
	domainImages "github.com/mantonx/viewra/internal/domain/images"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
	"github.com/mantonx/viewra/internal/infrastructure/system"
)

// =============================================================================
// ScanLibraryUseCase - Main Struct
// =============================================================================

// ScanLibraryUseCase handles the business logic for scanning a library
type ScanLibraryUseCase struct {
	mediaRepos         *scan.MediaRepositories
	scanRepos          *scan.ScanRepositories
	imageRepo          domainImages.Repository
	imageCleanup       cleanup.ImageCleanupExecutor
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

	// Enrichment - optional, if set, media is enqueued for enrichment after scanning
	enrichmentEnqueuer scanmedia.EnrichmentEnqueuer

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
	imageCleanup cleanup.ImageCleanupExecutor,
	enrichmentEnqueuer scanmedia.EnrichmentEnqueuer,
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
		enrichmentEnqueuer:    enrichmentEnqueuer,
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
		execution.HandleStuckScan(ctx, uc.executionDeps(), job, uc)
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
		MediaRepos:         uc.mediaRepos,
		ScanRepos:          uc.scanRepos,
		ImageRepo:          uc.imageRepo,
		EnrichmentEnqueuer: uc.enrichmentEnqueuer,
		MovieExtractor:     uc.movieImageExtractor,
		EpisodeExtractor:   uc.episodeImageExtractor,
		ShowExtractor:      uc.showImageExtractor,
		SeasonExtractor:    uc.seasonImageExtractor,
		AlbumExtractor:     uc.albumImageExtractor,
		ArtistExtractor:    uc.artistImageExtractor,
		TrackExtractor:     uc.trackImageExtractor,
		ProcessedArtists:   &uc.processedArtists,
		ProcessedShows:     &uc.processedShows,
		Coordinator:        uc.coordinator,
		Logger:             uc.logger,
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
		IsMediaFile:   scanutil.IsMediaFile,
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

func (uc *ScanLibraryUseCase) executionDeps() *execution.Deps {
	return &execution.Deps{
		ScanRepos:              uc.scanRepos,
		MediaRepos:             uc.mediaRepos,
		Config:                 &uc.config,
		SystemProfile:          uc.systemProfile,
		Logger:                 uc.logger,
		DiscoveryDeps:          uc.discoveryDeps,
		ProcessingDeps:         uc.processingDeps,
		StatusDeps:             uc.statusDeps,
		CleanupDeps:            uc.cleanupDeps,
		ProgressUpdater:        uc,
		SessionInitializer:     uc,
		RecoverFromPanic:       uc.recoverFromPanic,
		RecoverFromPanicWithError: uc.recoverFromPanicWithError,
		HasImageCleanup:        func() bool { return uc.imageRepo != nil && uc.imageCleanup != nil },
	}
}

// =============================================================================
// Interface Implementations
// =============================================================================

var _ processing.MediaProcessor = (*ScanLibraryUseCase)(nil)
var _ execution.ProgressUpdater = (*ScanLibraryUseCase)(nil)
var _ execution.SessionInitializer = (*ScanLibraryUseCase)(nil)
var _ execution.BackgroundStarter = (*ScanLibraryUseCase)(nil)

func (uc *ScanLibraryUseCase) ProcessMovie(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error) {
	return scanmedia.ProcessMovie(ctx, uc.mediaDeps(), libraryID, result, checkpoint, cache)
}

func (uc *ScanLibraryUseCase) ProcessTVEpisode(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error) {
	return scanmedia.ProcessTVEpisode(ctx, uc.mediaDeps(), libraryID, result, checkpoint, cache)
}

func (uc *ScanLibraryUseCase) ProcessMusicTrack(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error) {
	return scanmedia.ProcessMusicTrack(ctx, uc.mediaDeps(), libraryID, result, checkpoint, cache)
}

func (uc *ScanLibraryUseCase) NewProgressUpdate(jobID int64) *status.ProgressUpdate {
	return status.NewProgressUpdate(uc.scanRepos.ScanJob, uc.logger, jobID)
}

func (uc *ScanLibraryUseCase) InitializeScanSession(ctx context.Context, lib *library.Library) {
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

func (uc *ScanLibraryUseCase) StartScanBackground(jobID int64, libraryID int64, panicContext string) {
	lib, err := uc.mediaRepos.Library.GetByID(context.Background(), libraryID)
	if err != nil {
		uc.logger.Error("failed to get library for background scan", "library_id", libraryID, "error", err)
		return
	}
	uc.startScanBackground(jobID, lib, panicContext)
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
	uc.InitializeScanSession(ctx, lib)

	currentJob, err := uc.scanRepos.ScanJob.GetByID(ctx, jobID)
	if err != nil {
		uc.logger.Error("failed to get scan job", "job_id", jobID, "error", err)
		status.CompleteWithError(ctx, uc.statusDeps(), jobID, err)
		return
	}

	params := &execution.RunScanParams{JobID: jobID, Lib: lib}
	if execution.CanResumeFromCheckpoints(ctx, uc.executionDeps(), params, currentJob) {
		return
	}

	uc.logger.Info("starting fresh scan", "library_id", lib.ID)
	execution.RunFreshScan(ctx, uc.executionDeps(), jobID, lib)
}

// =============================================================================
// Panic Recovery
// =============================================================================

func (uc *ScanLibraryUseCase) recoverFromPanic(jobID, libraryID int64, description string) {
	recovery.RecoverFromPanic(uc.logger, uc.scanRepos.ScanJob, jobID, libraryID, description)
}

func (uc *ScanLibraryUseCase) recoverFromPanicWithError(jobID, libraryID int64, description string, errChan chan<- error) {
	recovery.RecoverFromPanicWithError(uc.logger, jobID, libraryID, description, errChan)
}

