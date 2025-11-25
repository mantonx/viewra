package library

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
)

// mediaExtensions is a package-level lookup table for supported media file extensions.
// Allocated once at startup to avoid per-call allocation overhead.
var mediaExtensions = map[string]bool{
	// Video
	"mp4": true, "mkv": true, "avi": true, "mov": true, "wmv": true, "flv": true,
	"webm": true, "m4v": true, "mpg": true, "mpeg": true, "m2ts": true, "ts": true,
	"vob": true, "3gp": true, "3g2": true, "f4v": true, "rm": true, "rmvb": true,
	"divx": true, "asf": true, "qt": true, "mts": true, "ogv": true, "mxf": true,
	// Audio
	"mp3": true, "flac": true, "m4a": true, "aac": true, "ogg": true, "opus": true,
	"wav": true, "wma": true, "ape": true, "wv": true, "tta": true, "tak": true,
	"dsf": true, "dff": true, "alac": true, "aiff": true, "aif": true,
}

// audioExtensions is a package-level lookup table for audio-only file extensions.
// Used to skip audio files in video-only libraries (Movie/TV).
var audioExtensions = map[string]bool{
	"mp3": true, "flac": true, "m4a": true, "aac": true, "ogg": true, "opus": true,
	"wav": true, "wma": true, "ape": true, "wv": true, "tta": true, "tak": true,
	"dsf": true, "dff": true, "alac": true, "aiff": true, "aif": true,
}

// isMediaFile checks if a file extension is for a media file
func (uc *ScanLibraryUseCase) isMediaFile(ext string) bool {
	// Remove leading dot if present
	ext = strings.TrimPrefix(ext, ".")
	return mediaExtensions[strings.ToLower(ext)]
}

// calculateProcessingTimeout determines appropriate timeout for file processing
// based on file size and storage type to prevent worker deadlocks
func (uc *ScanLibraryUseCase) calculateProcessingTimeout(fileSize int64) time.Duration {
	// Base timeout from config (default: 30s local, 60s remote)
	baseTimeout := uc.config.BaseFileTimeout

	// For network storage, be more generous due to latency
	if uc.systemProfile != nil && uc.systemProfile.Storage.IsRemote {
		baseTimeout = uc.config.RemoteStorageTimeout
	}

	// Add extra time for large files (1 second per GB)
	// This handles 4K content and large remuxes that take longer to probe
	const bytesPerGB = 1024 * 1024 * 1024
	sizeGB := fileSize / bytesPerGB
	if sizeGB > 0 {
		// Add up to MaxExtraTimeout for very large files
		extraTime := time.Duration(sizeGB) * time.Second
		if extraTime > uc.config.MaxExtraTimeout {
			extraTime = uc.config.MaxExtraTimeout
		}
		baseTimeout += extraTime
	}

	return baseTimeout
}

// statWithTimeout wraps os.Stat with a timeout to prevent indefinite hangs on network storage
// os.Stat doesn't support context cancellation, so we run it in a goroutine with a timeout
func (uc *ScanLibraryUseCase) statWithTimeout(ctx context.Context, path string, timeout time.Duration) (os.FileInfo, error) {
	type result struct {
		info os.FileInfo
		err  error
	}

	resultChan := make(chan result, 1)

	// Run os.Stat in a goroutine
	go func() {
		info, err := os.Stat(path)
		resultChan <- result{info: info, err: err}
	}()

	// Wait for either the stat to complete or timeout
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(timeout):
		return nil, fmt.Errorf("stat timeout after %v: %s", timeout, path)
	case res := <-resultChan:
		return res.info, res.err
	}
}

// validateDiscovery checks discovery results for potential issues
// Returns a list of warning messages if problems are detected
func (uc *ScanLibraryUseCase) validateDiscovery(
	ctx context.Context,
	libraryID int64,
	filesDiscovered int64,
	stats *filesystem.WalkStats,
) []string {
	warnings := []string{}

	// Check 1: Discovery had errors
	if stats != nil && stats.HasErrors() {
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
		if stats.PermissionErrors > 10 {
			warnings = append(warnings, fmt.Sprintf(
				"Encountered %d permission errors. Check library path permissions.",
				stats.PermissionErrors))
		}
		if stats.NetworkErrors > 0 {
			warnings = append(warnings, fmt.Sprintf(
				"Encountered %d network/timeout errors. Check network storage connectivity.",
				stats.NetworkErrors))
		}
	}

	// Check 2: Compare against previous completed scan
	previousJobs, err := uc.scanRepos.ScanJob.ListByLibrary(ctx, libraryID, 5)
	if err == nil && len(previousJobs) > 1 {
		// Find the last completed scan (not the current one)
		for _, prevJob := range previousJobs {
			if prevJob.Status == scanner.ScanStatusCompleted && prevJob.FilesFound > 0 {
				// Calculate percentage drop
				percentDrop := float64(prevJob.FilesFound-filesDiscovered) / float64(prevJob.FilesFound) * 100

				// Warn if we found significantly fewer files
				if percentDrop > 10.0 {
					warnings = append(warnings, fmt.Sprintf(
						"Discovery found %.0f%% fewer files than last scan (%d vs %d). This may indicate incomplete discovery.",
						percentDrop, filesDiscovered, prevJob.FilesFound))
				}

				// Also check if previous scan had discovery errors
				if prevJob.DirsSkipped > 0 && stats != nil && stats.DirsSkipped > 0 {
					warnings = append(warnings, fmt.Sprintf(
						"Repeated discovery errors detected. Previous scan skipped %d dirs, current scan skipped %d dirs. This suggests a persistent issue.",
						prevJob.DirsSkipped, stats.DirsSkipped))
				}

				break // Only compare against the most recent completed scan
			}
		}
	}

	// Check 3: Estimated total vs discovered (if we had an estimate)
	// This is already logged by the scan orchestrator, so we don't duplicate here

	return warnings
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
