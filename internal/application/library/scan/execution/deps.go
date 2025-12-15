package execution

import (
	"context"
	"log/slog"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/application/library/scan/cleanup"
	"github.com/mantonx/viewra/internal/application/library/scan/discovery"
	"github.com/mantonx/viewra/internal/application/library/scan/processing"
	"github.com/mantonx/viewra/internal/application/library/scan/status"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
	"github.com/mantonx/viewra/internal/infrastructure/system"
)

// ProgressUpdater creates progress update builders.
type ProgressUpdater interface {
	NewProgressUpdate(jobID int64) *status.ProgressUpdate
}

// SessionInitializer handles scan session initialization.
type SessionInitializer interface {
	InitializeScanSession(ctx context.Context, lib *library.Library)
}

// Deps bundles dependencies needed by the execution package.
type Deps struct {
	ScanRepos     *scan.ScanRepositories
	MediaRepos    *scan.MediaRepositories
	Config        *scan.Config
	SystemProfile *system.Profile
	Logger        *slog.Logger

	// Sub-package deps factories
	DiscoveryDeps  func() *discovery.Deps
	ProcessingDeps func() *processing.Deps
	StatusDeps     func() *status.Deps
	CleanupDeps    func() *cleanup.Deps

	// Progress and session callbacks
	ProgressUpdater    ProgressUpdater
	SessionInitializer SessionInitializer

	// Recovery callbacks
	RecoverFromPanic          func(jobID, libraryID int64, description string)
	RecoverFromPanicWithError func(jobID, libraryID int64, description string, errChan chan<- error)

	// Image cleanup check
	HasImageCleanup func() bool
}

// RunScanParams holds parameters for running a scan.
type RunScanParams struct {
	JobID int64
	Lib   *library.Library
}

// CanResumeFromCheckpoints checks if a scan can be resumed from existing checkpoints.
func CanResumeFromCheckpoints(ctx context.Context, deps *Deps, params *RunScanParams, currentJob *scanner.ScanJob) bool {
	stats, err := deps.ScanRepos.Checkpoint.GetStats(ctx, params.JobID)
	if err != nil {
		return false
	}

	if stats.TotalFiles == 0 {
		return false
	}

	if stats.PendingFiles == 0 {
		deps.Logger.Info("scan already complete from checkpoints",
			"job_id", params.JobID,
			"files_found", currentJob.FilesFound,
			"processed", stats.ProcessedFiles)
		status.CompleteFromStats(ctx, deps.StatusDeps(), params.JobID, stats.TotalFiles, stats)
		return true
	}

	if validateCheckpointCompleteness(ctx, deps, params.JobID, currentJob, stats) {
		deps.Logger.Info("resuming scan from checkpoints",
			"files_found", currentJob.FilesFound,
			"total_checkpoints", stats.TotalFiles,
			"pending", stats.PendingFiles,
			"completed", stats.CompletedFiles)
		ResumeFromCheckpoints(ctx, deps, params.JobID, params.Lib)
		return true
	}

	return false
}

func validateCheckpointCompleteness(ctx context.Context, deps *Deps, jobID int64, currentJob *scanner.ScanJob, stats *scanner.CheckpointStats) bool {
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

	deps.Logger.Warn("incomplete checkpoint creation detected, cleaning up and starting fresh",
		"job_id", jobID,
		"files_found", filesFound,
		"checkpoints_created", totalCheckpoints,
		"expected_minimum", minExpected)

	if deleteErr := deps.ScanRepos.Checkpoint.DeleteByJobID(ctx, jobID); deleteErr != nil {
		deps.Logger.Error("failed to delete partial checkpoints", "error", deleteErr)
	}

	return false
}

// ResumeFromCheckpoints resumes a scan from existing checkpoints.
func ResumeFromCheckpoints(ctx context.Context, deps *Deps, jobID int64, lib *library.Library) {
	stats, err := deps.ScanRepos.Checkpoint.GetStats(ctx, jobID)
	if err != nil {
		deps.Logger.Error("Failed to get checkpoint stats for resume", "job_id", jobID, "error", err)
		return
	}

	currentJob, err := deps.ScanRepos.ScanJob.GetByID(ctx, jobID)
	if err != nil {
		deps.Logger.Error("Failed to get scan job for resume", "job_id", jobID, "error", err)
		return
	}

	totalCheckpoints := stats.PendingFiles + stats.CompletedFiles + stats.FailedFiles
	deps.Logger.Info("Resuming scan from checkpoints",
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
	if err := deps.ScanRepos.ScanJob.UpdateProgress(ctx, jobID, progress); err != nil {
		deps.Logger.Warn("Failed to initialize resume progress", "job_id", jobID, "error", err)
	}

	hashingDone := make(chan struct{})
	close(hashingDone)
	ProcessFilesWithCheckpoints(ctx, deps, jobID, lib, hashingDone, nil)
}

// ProcessFilesWithCheckpoints processes files using checkpoint-based batch processing.
func ProcessFilesWithCheckpoints(ctx context.Context, deps *Deps, jobID int64, lib *library.Library, hashingDone <-chan struct{}, discoveryStats *filesystem.WalkStats) {
	pDeps := deps.ProcessingDeps()
	pctx := processing.InitCheckpointProcessing(ctx, pDeps, jobID, lib)

	processing.StartCheckpointWorkers(ctx, pDeps, pctx)
	processing.RunCheckpointProcessingLoop(ctx, pDeps, pctx, hashingDone)
	close(pctx.CheckpointsChan) // Close channel to signal workers to exit
	pctx.WorkerWg.Wait()

	var cleanupFn func()
	if deps.HasImageCleanup() {
		cleanupFn = func() {
			cleanup.CleanupStaleMedia(ctx, deps.CleanupDeps(), lib.ID, pctx.FoundFiles)
		}
	}
	processing.CompleteScan(ctx, pDeps, pctx, discoveryStats, cleanupFn)
}
