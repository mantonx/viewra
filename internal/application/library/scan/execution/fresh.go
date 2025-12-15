package execution

import (
	"context"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/application/library/scan/discovery"
	"github.com/mantonx/viewra/internal/application/library/scan/processing"
	"github.com/mantonx/viewra/internal/application/library/scan/status"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
)

// RunFreshScan runs a fresh scan for a library (not resuming from checkpoints).
func RunFreshScan(ctx context.Context, deps *Deps, jobID int64, lib *library.Library) {
	currentJob, err := deps.ScanRepos.ScanJob.GetByID(ctx, jobID)
	if err != nil {
		deps.Logger.Error("failed to get job for discovery", "job_id", jobID, "error", err)
		return
	}
	dDeps := deps.DiscoveryDeps()
	walker := discovery.CreateWalker(dDeps)
	dctx := discovery.NewContext(jobID, lib, currentJob, walker)

	discovery.PhaseCountFiles(ctx, dctx, dDeps)

	discoveredFiles, err := phaseWalkDirectory(ctx, deps, dctx)
	if err != nil {
		status.CompleteWithError(ctx, deps.StatusDeps(), jobID, err)
		return
	}

	diff := phaseDetermineChanges(ctx, deps, dctx, discoveredFiles)
	if diff == nil {
		return
	}

	discovery.PhaseHandleDeleted(ctx, dctx, dDeps, diff)
	phaseHashAndProcess(ctx, deps, dctx, diff)
}

func phaseWalkDirectory(ctx context.Context, deps *Deps, dctx *discovery.Context) ([]scanner.FileInfo, error) {
	dDeps := deps.DiscoveryDeps()
	progressCallback := func(filesDiscovered int64) {
		deps.ProgressUpdater.NewProgressUpdate(dctx.JobID).
			Phase(scanner.ScanPhaseDiscovering).
			FilesFound(filesDiscovered).
			EstimatedTotal(dctx.CurrentJob.EstimatedTotal).
			UpdateAsync(ctx)
	}

	discoveredFiles, err := discovery.PhaseWalkDirectory(ctx, dctx, dDeps, progressCallback)
	if err != nil {
		return nil, err
	}

	warnings := discovery.ValidateDiscovery(ctx, dDeps, dctx.Lib.ID, int64(len(discoveredFiles)), dctx.DiscoveryStats)
	for _, warning := range warnings {
		deps.Logger.Warn("Discovery validation warning", "job_id", dctx.JobID, "warning", warning)
	}

	if err := deps.ProgressUpdater.NewProgressUpdate(dctx.JobID).
		Phase(scanner.ScanPhaseProcessing).
		FilesFound(int64(len(discoveredFiles))).
		EstimatedTotal(dctx.CurrentJob.EstimatedTotal).
		DiscoveryDone().
		Update(ctx); err != nil {
		deps.Logger.Warn("Failed to update discovery completion status", "job_id", dctx.JobID, "error", err)
	}

	return discoveredFiles, nil
}

func phaseDetermineChanges(ctx context.Context, deps *Deps, dctx *discovery.Context, discoveredFiles []scanner.FileInfo) *scanner.ScanDiff {
	diff := discovery.PhaseDetermineChanges(ctx, dctx, deps.DiscoveryDeps(), discoveredFiles)
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
		status.CompleteSafely(ctx, deps.StatusDeps(), job)
		return nil
	}
	return diff
}

func phaseHashAndProcess(ctx context.Context, deps *Deps, dctx *discovery.Context, diff *scanner.ScanDiff) {
	filesToProcess := append(diff.NewFiles, diff.ModifiedFiles...)

	deps.Logger.Info("processing files",
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
		defer deps.RecoverFromPanicWithError(dctx.JobID, dctx.Lib.ID, "processing goroutine panicked", processingErrChan)
		ProcessFilesWithCheckpoints(ctx, deps, dctx.JobID, dctx.Lib, hashingDone, dctx.DiscoveryStats)
	}()

	if err := processing.HashAndStreamCheckpoints(ctx, deps.ProcessingDeps(), filesToProcess, dctx.JobID, dctx.Lib.ID); err != nil {
		deps.Logger.Error("failed to create checkpoints", "error", err)
		close(hashingDone)
		status.CompleteWithError(ctx, deps.StatusDeps(), dctx.JobID, err)
		return
	}

	close(hashingDone)
	processingWg.Wait()

	select {
	case err := <-processingErrChan:
		if err != nil {
			deps.Logger.Error("processing failed", "error", err)
			status.CompleteWithError(ctx, deps.StatusDeps(), dctx.JobID, err)
			return
		}
	default:
	}

	duration := time.Since(startTime)
	avgMs := float64(duration.Milliseconds()) / float64(len(filesToProcess))
	deps.Logger.Info("concurrent hashing and processing completed",
		"file_count", len(filesToProcess),
		"duration", duration,
		"avg_ms_per_file", avgMs)
}
