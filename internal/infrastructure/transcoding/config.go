package transcoding

import (
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/config"
)

// HardwareAccel represents hardware acceleration type.
// Re-exported from config subpackage for backward compatibility.
type HardwareAccel = config.HardwareAccel

const (
	// AccelNone uses software encoding (slowest, most compatible)
	AccelNone = config.AccelNone

	// AccelVAAPI uses Intel/AMD GPU via VAAPI (Linux)
	AccelVAAPI = config.AccelVAAPI

	// AccelNVENC uses NVIDIA GPU (Linux/Windows)
	AccelNVENC = config.AccelNVENC

	// AccelQSV uses Intel Quick Sync Video (Linux/Windows)
	AccelQSV = config.AccelQSV

	// AccelVideoToolbox uses Apple VideoToolbox (macOS)
	AccelVideoToolbox = config.AccelVideoToolbox
)

// TranscodeConfig holds transcoding configuration.
// Re-exported from config subpackage for backward compatibility.
type TranscodeConfig = config.TranscodeConfig

// DefaultTranscodeConfig returns sensible defaults.
// Re-exported from config subpackage for backward compatibility.
func DefaultTranscodeConfig() *TranscodeConfig {
	return config.DefaultWithChecker(CheckFFmpegEncoder)
}

// DefaultTranscodeConfigFromProfile returns transcode config based on system profile.
// If profile is nil, falls back to hardware detection.
// Re-exported from config subpackage for backward compatibility.
func DefaultTranscodeConfigFromProfile(hwAccelType string) *TranscodeConfig {
	return config.DefaultFromProfile(hwAccelType, CheckFFmpegEncoder)
}

// detectHardwareAccel attempts to detect available hardware acceleration.
// Priority: NVENC > QSV > VAAPI > VideoToolbox > None
// Re-exported from config subpackage for backward compatibility.
func detectHardwareAccel() HardwareAccel {
	return config.DetectHardwareAccel(CheckFFmpegEncoder)
}

// GetDefaultOutputDir returns the default transcode output directory.
// Re-exported from config subpackage for backward compatibility.
func GetDefaultOutputDir() string {
	return config.GetDefaultOutputDir()
}
