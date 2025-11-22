package transcoding

import (
	"os"
	"os/exec"
	"strconv"
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

	// HardwareDevice specifies the device path for hardware acceleration
	// Linux VAAPI/QSV: /dev/dri/renderD128 (default), /dev/dri/renderD129, etc.
	// Windows: Not applicable (uses Direct3D/DXVA automatically)
	// macOS: Not applicable (uses VideoToolbox automatically)
	// Environment variable: HARDWARE_DEVICE
	HardwareDevice string

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

	// ToneMappingEnabled enables automatic HDR to SDR tone mapping for HDR content
	// When true, HDR10/HLG content will be tone mapped to SDR (bt709) for better compatibility
	// Environment variable: TONE_MAPPING_ENABLED (default: true)
	ToneMappingEnabled bool

	// ToneMappingAlgorithm specifies which tone mapping algorithm to use
	// Options: none, linear, gamma, clip, reinhard, hable, mobius, bt.2390, bt.2446a, spline
	// Environment variable: TONE_MAPPING_ALGORITHM (default: bt.2390)
	ToneMappingAlgorithm string

	// ToneMappingBackend specifies which tone mapping backend to use
	// Options: auto, libplacebo, opencl, vaapi, cpu
	// auto = libplacebo if available, else OpenCL/VAAPI/CPU based on hardware
	// Environment variable: TONE_MAPPING_BACKEND (default: auto)
	ToneMappingBackend string

	// LibPlaceboPeakDetect enables dynamic peak detection for adaptive HDR tone mapping
	// Only applies when using libplacebo backend
	// Environment variable: LIBPLACEBO_PEAK_DETECT (default: true)
	LibPlaceboPeakDetect bool

	// LibPlaceboContrastRecovery controls highlight detail preservation (0.0-3.0)
	// Higher values preserve more highlight detail at the cost of local contrast
	// Environment variable: LIBPLACEBO_CONTRAST_RECOVERY (default: 0.3)
	LibPlaceboContrastRecovery float64
}

// DefaultTranscodeConfig returns sensible defaults.
func DefaultTranscodeConfig() *TranscodeConfig {
	return DefaultTranscodeConfigFromProfile("")
}

// DefaultTranscodeConfigFromProfile returns transcode config based on system profile.
// If profile is nil, falls back to hardware detection.
func DefaultTranscodeConfigFromProfile(hwAccelType string) *TranscodeConfig {
	// Use provided hardware acceleration or detect
	var hwaccel HardwareAccel
	if hwAccelType != "" {
		hwaccel = HardwareAccel(hwAccelType)
	} else {
		hwaccel = detectHardwareAccel()
	}

	// Get hardware device from environment or use default
	hwDevice := os.Getenv("HARDWARE_DEVICE")
	if hwDevice == "" {
		hwDevice = "/dev/dri/renderD128" // Default Linux device
	}

	// Get tone mapping setting from environment (default: enabled)
	toneMappingEnabled := true
	if envToneMapping := os.Getenv("TONE_MAPPING_ENABLED"); envToneMapping != "" {
		toneMappingEnabled = envToneMapping == "true" || envToneMapping == "1"
	}

	// Get tone mapping algorithm from environment (default: bt.2390)
	toneMappingAlgorithm := os.Getenv("TONE_MAPPING_ALGORITHM")
	if toneMappingAlgorithm == "" {
		toneMappingAlgorithm = "bt.2390" // Default to ITU-R BT.2390 EETF (broadcast standard)
	}

	// Get tone mapping backend from environment (default: auto)
	toneMappingBackend := os.Getenv("TONE_MAPPING_BACKEND")
	if toneMappingBackend == "" {
		toneMappingBackend = "auto" // Auto-detect best backend
	}

	// Get libplacebo peak detection setting (default: true)
	libPlaceboPeakDetect := true
	if envPeakDetect := os.Getenv("LIBPLACEBO_PEAK_DETECT"); envPeakDetect != "" {
		libPlaceboPeakDetect = envPeakDetect == "true" || envPeakDetect == "1"
	}

	// Get libplacebo contrast recovery (default: 0.3)
	libPlaceboContrastRecovery := 0.3
	if envContrastRecov := os.Getenv("LIBPLACEBO_CONTRAST_RECOVERY"); envContrastRecov != "" {
		// Parse float, ignore errors (keep default)
		if val, err := strconv.ParseFloat(envContrastRecov, 64); err == nil {
			libPlaceboContrastRecovery = val
		}
	}

	return &TranscodeConfig{
		HardwareAccel:              hwaccel,
		HardwareDevice:             hwDevice,
		OutputBaseDir:              GetDefaultOutputDir(), // /data/dash or ./data/dash
		MinFreeDiskGB:              10,                    // Require 10GB free space
		MaxCPUPercent:              0,                     // Unlimited by default
		MaxMemoryMB:                0,                     // Unlimited by default
		ProcessGroupKill:           true,                  // Always kill process group
		ToneMappingEnabled:         toneMappingEnabled,    // Enable tone mapping by default
		ToneMappingAlgorithm:       toneMappingAlgorithm,  // Default: bt.2390 (ITU-R BT.2390)
		ToneMappingBackend:         toneMappingBackend,    // Default: auto
		LibPlaceboPeakDetect:       libPlaceboPeakDetect,  // Default: true
		LibPlaceboContrastRecovery: libPlaceboContrastRecovery, // Default: 0.3
	}
}

// detectHardwareAccel attempts to detect available hardware acceleration.
// Priority: NVENC > QSV > VAAPI > VideoToolbox > None
func detectHardwareAccel() HardwareAccel {
	// Check for environment variable override
	if accel := os.Getenv("HARDWARE_ACCEL"); accel != "" {
		switch HardwareAccel(accel) {
		case AccelNVENC, AccelVAAPI, AccelQSV, AccelVideoToolbox, AccelNone:
			return HardwareAccel(accel)
		}
	}

	// Detect NVIDIA GPU (nvenc)
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		// Verify FFmpeg has nvenc support
		if checkFFmpegEncoder("h264_nvenc") {
			return AccelNVENC
		}
	}

	// Detect Intel Quick Sync (qsv)
	if _, err := os.Stat("/dev/dri/renderD128"); err == nil {
		if checkFFmpegEncoder("h264_qsv") {
			return AccelQSV
		}
	}

	// Detect VAAPI (Intel/AMD on Linux)
	if _, err := os.Stat("/dev/dri/renderD128"); err == nil {
		if checkFFmpegEncoder("h264_vaapi") {
			return AccelVAAPI
		}
	}

	// Default to software encoding
	return AccelNone
}

// checkFFmpegEncoder checks if FFmpeg has a specific encoder available.
// Deprecated: Use CheckFFmpegEncoder from hardware_test_utils.go instead.
func checkFFmpegEncoder(encoder string) bool {
	return CheckFFmpegEncoder(encoder)
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
