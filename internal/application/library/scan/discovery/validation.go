package discovery

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
)

// Validation thresholds - exported for use by calling code
const (
	// FileDropWarningThresholdPercent triggers a warning when a scan discovers
	// significantly fewer files than the previous completed scan. This may indicate
	// incomplete discovery due to network issues or permission changes.
	FileDropWarningThresholdPercent = 10.0

	// PermissionErrorWarningThreshold is the minimum number of permission errors
	// during discovery before a warning is logged. A few permission errors are
	// normal, but many suggest a systemic permissions problem.
	PermissionErrorWarningThreshold = 10
)

// CheckWalkStatsErrors examines filesystem walk statistics for issues.
// Returns warnings for skipped directories/files, permission errors, and network errors.
func CheckWalkStatsErrors(stats *filesystem.WalkStats) []string {
	if stats == nil || !stats.HasErrors() {
		return nil
	}

	var warnings []string

	if stats.DirsSkipped > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"Failed to read %d directories during discovery. Some files may be missing. Check permissions and network connectivity.",
			stats.DirsSkipped))
	}

	if stats.FilesSkipped > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"Failed to stat %d files during discovery. These files were skipped.",
			stats.FilesSkipped))
	}

	if stats.PermissionErrors > PermissionErrorWarningThreshold {
		warnings = append(warnings, fmt.Sprintf(
			"Encountered %d permission errors. Check library path permissions.",
			stats.PermissionErrors))
	}

	if stats.NetworkErrors > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"Encountered %d network/timeout errors. Check network storage connectivity.",
			stats.NetworkErrors))
	}

	return warnings
}

// DetectFileDrop checks if the current scan found significantly fewer files than previous.
// Returns a warning message if the drop exceeds FileDropWarningThresholdPercent.
func DetectFileDrop(currentCount, previousCount int64) string {
	if previousCount == 0 {
		return ""
	}

	percentDrop := float64(previousCount-currentCount) / float64(previousCount) * 100

	if percentDrop > FileDropWarningThresholdPercent {
		return fmt.Sprintf(
			"Discovery found %.0f%% fewer files than last scan (%d vs %d). This may indicate incomplete discovery.",
			percentDrop, currentCount, previousCount)
	}

	return ""
}

// DetectRepeatedErrors checks if both current and previous scans had directory skip errors.
// Returns a warning message if this pattern suggests a persistent issue.
func DetectRepeatedErrors(stats *filesystem.WalkStats, prevJob *scanner.ScanJob) string {
	if stats == nil || stats.DirsSkipped == 0 || prevJob.DirsSkipped == 0 {
		return ""
	}

	return fmt.Sprintf(
		"Repeated discovery errors detected. Previous scan skipped %d dirs, current scan skipped %d dirs. This suggests a persistent issue.",
		prevJob.DirsSkipped, stats.DirsSkipped)
}

// ValidateDiscovery checks discovery results for issues and compares against previous scans.
// Returns a list of warning messages for any problems detected.
func ValidateDiscovery(ctx context.Context, deps *Deps, libraryID int64, filesDiscovered int64, stats *filesystem.WalkStats) []string {
	var warnings []string
	warnings = append(warnings, CheckWalkStatsErrors(stats)...)
	warnings = append(warnings, checkAgainstPreviousScan(ctx, deps, libraryID, filesDiscovered, stats)...)
	return warnings
}

// checkAgainstPreviousScan compares the current discovery against previous completed scans.
func checkAgainstPreviousScan(ctx context.Context, deps *Deps, libraryID int64, filesDiscovered int64, stats *filesystem.WalkStats) []string {
	previousJobs, err := deps.ScanRepos.ScanJob.ListByLibrary(ctx, libraryID, scan.PreviousJobsToCompare)
	if err != nil || len(previousJobs) <= 1 {
		return nil
	}

	for _, prevJob := range previousJobs {
		if prevJob.Status != scanner.ScanStatusCompleted || prevJob.FilesFound == 0 {
			continue
		}

		var warnings []string
		if warning := DetectFileDrop(filesDiscovered, prevJob.FilesFound); warning != "" {
			warnings = append(warnings, warning)
		}
		if warning := DetectRepeatedErrors(stats, prevJob); warning != "" {
			warnings = append(warnings, warning)
		}
		return warnings
	}
	return nil
}
