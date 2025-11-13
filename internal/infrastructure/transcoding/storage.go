package transcoding

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// StorageInfo contains storage space information.
type StorageInfo struct {
	TotalBytes     uint64
	FreeBytes      uint64
	AvailableBytes uint64
	UsedBytes      uint64
	UsedPercent    float64
}

// GetStorageInfo returns storage information for the given path.
func GetStorageInfo(path string) (*StorageInfo, error) {
	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Ensure directory exists or get parent directory
	dir := absPath
	if info, err := os.Stat(dir); err != nil {
		// If path doesn't exist, use parent directory
		dir = filepath.Dir(dir)
	} else if !info.IsDir() {
		// If path is a file, use parent directory
		dir = filepath.Dir(dir)
	}

	// Get filesystem stats
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err != nil {
		return nil, fmt.Errorf("failed to get filesystem stats for %s: %w", dir, err)
	}

	// Calculate sizes (block size * block count)
	totalBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bfree * uint64(stat.Bsize)
	availableBytes := stat.Bavail * uint64(stat.Bsize) // Available to non-root users
	usedBytes := totalBytes - freeBytes
	usedPercent := float64(usedBytes) / float64(totalBytes) * 100

	return &StorageInfo{
		TotalBytes:     totalBytes,
		FreeBytes:      freeBytes,
		AvailableBytes: availableBytes,
		UsedBytes:      usedBytes,
		UsedPercent:    usedPercent,
	}, nil
}

// CheckDiskSpace verifies there is sufficient disk space available.
// Returns an error if available space is less than minGB.
func CheckDiskSpace(path string, minGB int64) error {
	info, err := GetStorageInfo(path)
	if err != nil {
		return fmt.Errorf("failed to check disk space: %w", err)
	}

	minBytes := uint64(minGB) * 1024 * 1024 * 1024
	if info.AvailableBytes < minBytes {
		return fmt.Errorf(
			"insufficient disk space: %.2f GB available, %.2f GB required",
			float64(info.AvailableBytes)/(1024*1024*1024),
			float64(minBytes)/(1024*1024*1024),
		)
	}

	return nil
}

// EstimateOutputSize estimates the output size for a transcode based on bitrate and duration.
// Returns estimated size in bytes.
func EstimateOutputSize(videoBitrate string, audioBitrate string, durationSeconds float64) (uint64, error) {
	// Parse video bitrate (e.g., "2500k" -> 2500000)
	videoBitsPerSecond, err := parseBitrate(videoBitrate)
	if err != nil {
		return 0, fmt.Errorf("invalid video bitrate: %w", err)
	}

	// Parse audio bitrate
	audioBitsPerSecond, err := parseBitrate(audioBitrate)
	if err != nil {
		return 0, fmt.Errorf("invalid audio bitrate: %w", err)
	}

	// Total bits = (video + audio) bitrate * duration
	totalBits := (videoBitsPerSecond + audioBitsPerSecond) * uint64(durationSeconds)

	// Convert to bytes and add 20% overhead for container, metadata, etc.
	estimatedBytes := (totalBits / 8) * 12 / 10

	return estimatedBytes, nil
}

// parseBitrate parses a bitrate string like "2500k" or "5000k" to bits per second.
func parseBitrate(bitrateStr string) (uint64, error) {
	if len(bitrateStr) == 0 {
		return 0, fmt.Errorf("empty bitrate string")
	}

	var multiplier uint64 = 1
	numStr := bitrateStr

	// Check for suffix
	lastChar := bitrateStr[len(bitrateStr)-1]
	if lastChar == 'k' || lastChar == 'K' {
		multiplier = 1000
		numStr = bitrateStr[:len(bitrateStr)-1]
	} else if lastChar == 'm' || lastChar == 'M' {
		multiplier = 1000000
		numStr = bitrateStr[:len(bitrateStr)-1]
	}

	// Parse number
	var num uint64
	_, err := fmt.Sscanf(numStr, "%d", &num)
	if err != nil {
		return 0, fmt.Errorf("failed to parse bitrate number: %w", err)
	}

	return num * multiplier, nil
}

// FormatBytes formats bytes as a human-readable string.
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.2f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
