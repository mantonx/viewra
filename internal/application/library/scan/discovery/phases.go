package discovery

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
)

// ProgressUpdater interface allows discovery to update job progress without circular imports.
type ProgressUpdater interface {
	UpdateDiscoveryProgress(ctx context.Context, jobID int64, phase scanner.ScanPhase, filesFound int64, estimatedTotal int64, discoveryDone bool) error
	UpdateDiscoveryProgressAsync(ctx context.Context, jobID int64, phase scanner.ScanPhase, filesFound int64, estimatedTotal int64)
}

// PhaseCountFiles performs a fast count pass to get accurate total for progress reporting.
// This is Phase 1 of discovery.
func PhaseCountFiles(ctx context.Context, dctx *Context, deps *Deps) int64 {
	deps.Logger.Info("Counting media files", "library_id", dctx.Lib.ID, "path", dctx.Lib.Path)

	totalFiles, err := dctx.Walker.Count(ctx, dctx.Lib.Path, func(fi scanner.FileInfo) bool {
		return !fi.IsDir && deps.IsMediaFile(fi.Extension)
	})

	if err != nil {
		deps.Logger.Warn("File count failed, proceeding with discovery",
			"library_id", dctx.Lib.ID,
			"error", err)
		return 0
	}

	deps.Logger.Info("File count completed",
		"library_id", dctx.Lib.ID,
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
	if err := deps.ScanRepos.ScanJob.UpdateProgress(ctx, dctx.JobID, countProgress); err != nil {
		deps.Logger.Warn("Failed to update count progress",
			"job_id", dctx.JobID,
			"error", err)
	}

	return totalFiles
}

// PhaseWalkDirectory walks the directory tree and collects media files.
// This is Phase 2 of discovery.
func PhaseWalkDirectory(
	ctx context.Context,
	dctx *Context,
	deps *Deps,
	progressCallback func(filesDiscovered int64),
) ([]scanner.FileInfo, error) {
	var mediaFilesDiscovered atomic.Int64

	// Collect files via channel
	filesChan := make(chan scanner.FileInfo, deps.Config.DiscoveryBufferSize)
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
	err := dctx.Walker.Walk(ctx, dctx.Lib.Path, func(fileInfo scanner.FileInfo) error {
		if fileInfo.IsDir || !deps.IsMediaFile(fileInfo.Extension) {
			return nil
		}

		filesChan <- fileInfo

		count := mediaFilesDiscovered.Add(1)
		if count%int64(deps.Config.DiscoveryLogEvery) == 0 && progressCallback != nil {
			progressCallback(count)
		}

		return nil
	})

	close(filesChan)
	discoveryWg.Wait()

	// Report final count
	if finalCount := mediaFilesDiscovered.Load(); finalCount > 0 && progressCallback != nil {
		progressCallback(finalCount)
	}

	// Capture discovery statistics
	dctx.DiscoveryStats = dctx.Walker.GetStats()
	LogDiscoveryStats(deps.Logger, dctx.JobID, dctx.DiscoveryStats)

	if err != nil {
		deps.Logger.Error("File discovery failed",
			"job_id", dctx.JobID,
			"library_id", dctx.Lib.ID,
			"error", err)
		return nil, err
	}

	deps.Logger.Info("File discovery completed",
		"job_id", dctx.JobID,
		"media_files_discovered", len(discoveredFiles))

	return discoveredFiles, nil
}

// LogDiscoveryStats logs statistics from the directory walk.
func LogDiscoveryStats(logger interface{ Info(msg string, args ...any); Warn(msg string, args ...any) }, jobID int64, stats *filesystem.WalkStats) {
	if stats == nil {
		return
	}

	logger.Info("Discovery statistics",
		"job_id", jobID,
		"files_discovered", stats.FilesDiscovered,
		"dirs_scanned", stats.DirsScanned,
		"dirs_skipped", stats.DirsSkipped,
		"files_skipped", stats.FilesSkipped,
		"permission_errors", stats.PermissionErrors,
		"network_errors", stats.NetworkErrors,
		"other_errors", stats.OtherErrors)

	if stats.HasErrors() {
		logger.Warn("Discovery completed with errors - some files may be missing",
			"job_id", jobID,
			"total_errors", stats.TotalErrors(),
			"dirs_skipped", stats.DirsSkipped,
			"files_skipped", stats.FilesSkipped,
			"sample_paths", stats.SkippedPaths)
	}
}

// PhaseDetermineChanges computes the diff between current and previous scan.
// Returns nil if no changes need processing (indicating scan should be marked complete).
// This is Phase 3 of discovery.
func PhaseDetermineChanges(
	ctx context.Context,
	dctx *Context,
	deps *Deps,
	discoveredFiles []scanner.FileInfo,
) *scanner.ScanDiff {
	diff, err := deps.IncrScanner.DetermineChanges(ctx, dctx.Lib.ID, discoveredFiles)
	if err != nil {
		deps.Logger.Warn("incremental scan failed, falling back to full scan",
			"library_id", dctx.Lib.ID,
			"error", err)
		diff = &scanner.ScanDiff{
			NewFiles:       discoveredFiles,
			ModifiedFiles:  []scanner.FileInfo{},
			DeletedFiles:   []string{},
			UnchangedFiles: []string{},
		}
	} else {
		deps.Logger.Info("incremental scan completed",
			"library_id", dctx.Lib.ID,
			"diff", diff.Summary())
	}

	// No changes - return nil to signal scan complete
	if !diff.NeedsProcessing() {
		deps.Logger.Info("no changes detected, scan complete",
			"files_found", len(discoveredFiles),
			"unchanged_files", len(diff.UnchangedFiles))
		return nil
	}

	return diff
}

// PhaseHandleDeleted removes scan state for files that no longer exist.
// This is Phase 4 of discovery.
func PhaseHandleDeleted(ctx context.Context, dctx *Context, deps *Deps, diff *scanner.ScanDiff) {
	if len(diff.DeletedFiles) == 0 {
		return
	}

	deps.Logger.Info("removing deleted files from scan state",
		"count", len(diff.DeletedFiles))

	if err := deps.ScanRepos.ScanState.DeleteByPaths(ctx, dctx.Lib.ID, diff.DeletedFiles); err != nil {
		deps.Logger.Error("failed to delete scan state for removed files",
			"library_id", dctx.Lib.ID,
			"count", len(diff.DeletedFiles),
			"error", err)
	}
}

// CreateWalker creates a filesystem walker with optimal settings based on system profile.
func CreateWalker(deps *Deps) *filesystem.Walker {
	var walkerOpts []filesystem.WalkerOption

	// Use profiler recommendations if available, otherwise use config
	if deps.SystemProfile != nil {
		recommendations := deps.SystemProfile.Calculate()
		if recommendations.ScanWalkers > 0 {
			walkerOpts = append(walkerOpts, filesystem.WithParallelWalking(recommendations.ScanWalkers))
			deps.Logger.Info("using parallel directory walking",
				"workers", recommendations.ScanWalkers,
				"storage_type", deps.SystemProfile.Storage.Type)
		}
	} else if deps.Config.ParallelWalkers > 0 {
		walkerOpts = append(walkerOpts, filesystem.WithParallelWalking(deps.Config.ParallelWalkers))
	}

	if deps.Config.ProgressInterval > 0 {
		walkerOpts = append(walkerOpts, filesystem.WithProgressLogging(deps.Config.ProgressInterval))
	}

	walkerOpts = append(walkerOpts, filesystem.WithLogger(deps.Logger))

	return filesystem.NewWalker(walkerOpts...)
}
