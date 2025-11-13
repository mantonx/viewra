package transcoding

import (
	"os"
)

// HardwareAccel represents hardware acceleration type.
type HardwareAccel string

const (
	// AccelNone uses software encoding (slowest, most compatible)
	AccelNone HardwareAccel = "none"

	// AccelVAAPI uses Intel/AMD GPU via VAAPI (Linux)
	AccelVAAPI HardwareAccel = "vaapi"

	// AccelNVENC uses NVIDIA GPU (Linux/Windows)
	AccelNVENC HardwareAccel = "nvenc"

	// AccelQSV uses Intel Quick Sync Video (Linux/Windows)
	AccelQSV HardwareAccel = "qsv"

	// AccelVideoToolbox uses Apple VideoToolbox (macOS)
	AccelVideoToolbox HardwareAccel = "videotoolbox"
)

// TranscodeConfig holds transcoding configuration.
type TranscodeConfig struct {
	// HardwareAccel specifies which hardware acceleration to use
	HardwareAccel HardwareAccel

	// OutputBaseDir is the base directory for DASH outputs
	// Environment variable: TRANSCODE_OUTPUT_DIR
	// Default: /data/dash (or ./data/dash if /data doesn't exist)
	OutputBaseDir string

	// MinFreeDiskGB is the minimum free disk space required to start a transcode (in GB)
	MinFreeDiskGB int64

	// MaxCPUPercent limits CPU usage (0 = unlimited, 100 = 1 core, 200 = 2 cores)
	// Uses nice/cpulimit on Linux
	MaxCPUPercent int

	// MaxMemoryMB limits memory usage in megabytes (0 = unlimited)
	// Uses cgroup limits on Linux if available
	MaxMemoryMB int

	// ProcessGroupKill ensures all child processes are killed on cancellation
	ProcessGroupKill bool
}

// DefaultTranscodeConfig returns sensible defaults.
func DefaultTranscodeConfig() *TranscodeConfig {
	return &TranscodeConfig{
		HardwareAccel:    AccelNone,             // Safe default, detect and configure in production
		OutputBaseDir:    GetDefaultOutputDir(), // /data/dash or ./data/dash
		MinFreeDiskGB:    10,                    // Require 10GB free space
		MaxCPUPercent:    0,                     // Unlimited by default
		MaxMemoryMB:      0,                     // Unlimited by default
		ProcessGroupKill: true,                  // Always kill process group
	}
}

// GetDefaultOutputDir returns the default transcode output directory.
// Prefers /data/dash (absolute), falls back to ./data/dash (relative) if /data doesn't exist.
func GetDefaultOutputDir() string {
	// Check environment variable first
	if dir := os.Getenv("TRANSCODE_OUTPUT_DIR"); dir != "" {
		return dir
	}

	// Fall back to relative ./data/dash (development)
	return "./data/dash"
}
