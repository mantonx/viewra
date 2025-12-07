package transcoding

import (
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/storage"
)

// StorageInfo is re-exported from storage subpackage for backward compatibility.
type StorageInfo = storage.Info

// GetStorageInfo returns storage information for the given path.
// Re-exported from storage subpackage for backward compatibility.
func GetStorageInfo(path string) (*StorageInfo, error) {
	return storage.GetInfo(path)
}

// CheckDiskSpace verifies there is sufficient disk space available.
// Re-exported from storage subpackage for backward compatibility.
func CheckDiskSpace(path string, minGB int64) error {
	return storage.CheckDiskSpace(path, minGB)
}

// EstimateOutputSize estimates the output size for a transcode based on bitrate and duration.
// Re-exported from storage subpackage for backward compatibility.
func EstimateOutputSize(videoBitrate string, audioBitrate string, durationSeconds float64) (uint64, error) {
	return storage.EstimateOutputSize(videoBitrate, audioBitrate, durationSeconds)
}

// FormatBytes formats bytes as a human-readable string.
// Re-exported from storage subpackage for backward compatibility.
func FormatBytes(bytes uint64) string {
	return storage.FormatBytes(bytes)
}
