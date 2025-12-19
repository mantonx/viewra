// Package config provides transcoding configuration types and defaults.
package config

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/hls"
	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/paths"
)

// HardwareAccel is an alias for hls.HardwareAccel.
// This provides backwards compatibility for code using config.HardwareAccel.
type HardwareAccel = hls.HardwareAccel

// Hardware acceleration constants - re-exported from hls package for convenience.
const (
	AccelNone         = hls.AccelNone
	AccelVAAPI        = hls.AccelVAAPI
	AccelNVENC        = hls.AccelNVENC
	AccelQSV          = hls.AccelQSV
	AccelVideoToolbox = hls.AccelVideoToolbox
)

// TranscodeConfig holds transcoding configuration.
type TranscodeConfig struct {
	// FFmpegPaths contains FFmpeg/FFprobe binary paths and library path.
	// This is the single source of truth for FFmpeg configuration.
	FFmpegPaths *paths.Paths

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
	// Options: auto, vulkan, opencl, cuda, libplacebo, vaapi, cpu
	// - auto: OpenCL for NVENC (3s first run, ~1s cached), native for VAAPI/QSV
	// - vulkan: Vulkan + libplacebo (~1.9s consistent startup, best quality bt.2390)
	// - opencl: OpenCL tone mapping (default for NVENC)
	// - cuda: tonemap_cuda (fastest, but has Dolby Vision issues)
	// - libplacebo: CPU-based libplacebo (high quality, slower)
	// - vaapi: VAAPI native tone mapping (Intel/AMD)
	// - cpu: Software fallback (zscale + tonemap)
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

	// FFmpegLogEnabled enables persistent FFmpeg log capture for debugging
	// When true, FFmpeg stderr output is saved to files for later inspection
	// Environment variable: FFMPEG_LOG_ENABLED (default: true)
	FFmpegLogEnabled bool

	// FFmpegLogRetentionHours specifies how long to keep FFmpeg log files (in hours)
	// Logs older than this are automatically cleaned up
	// Environment variable: FFMPEG_LOG_RETENTION_HOURS (default: 48)
	FFmpegLogRetentionHours int
}

// EncoderChecker is a function that verifies if an FFmpeg encoder is available.
// This allows the config package to check encoders without creating circular dependencies.
type EncoderChecker func(encoderName string) bool

// Default returns sensible defaults for transcoding configuration.
func Default() *TranscodeConfig {
	return DefaultFromProfile("", nil)
}

// DefaultWithChecker returns sensible defaults with a custom encoder checker.
func DefaultWithChecker(encoderChecker EncoderChecker) *TranscodeConfig {
	return DefaultFromProfile("", encoderChecker)
}

// DefaultFromProfile returns transcode config based on system profile.
// If hwAccelType is empty, falls back to hardware detection.
// If encoderChecker is nil, uses a simple exec-based check.
func DefaultFromProfile(hwAccelType string, encoderChecker EncoderChecker) *TranscodeConfig {
	// Get FFmpeg paths using the shared Paths type (single source of truth)
	ffmpegPaths, err := paths.New()
	if err != nil {
		// Fall back to placeholder paths that will fail with clear error at runtime
		ffmpegPaths = &paths.Paths{
			FFmpeg:  "ffmpeg",
			FFprobe: "ffprobe",
		}
	}

	// Use provided hardware acceleration or detect
	var hwaccel HardwareAccel
	if hwAccelType != "" {
		hwaccel = HardwareAccel(hwAccelType)
	} else {
		hwaccel = DetectHardwareAccel(encoderChecker)
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

	// Get FFmpeg log settings
	ffmpegLogEnabled := true
	if envLogEnabled := os.Getenv("FFMPEG_LOG_ENABLED"); envLogEnabled != "" {
		ffmpegLogEnabled = envLogEnabled == "true" || envLogEnabled == "1"
	}

	ffmpegLogRetentionHours := 48 // Default: 2 days
	if envRetention := os.Getenv("FFMPEG_LOG_RETENTION_HOURS"); envRetention != "" {
		if val, err := strconv.Atoi(envRetention); err == nil && val > 0 {
			ffmpegLogRetentionHours = val
		}
	}

	return &TranscodeConfig{
		FFmpegPaths:                ffmpegPaths,
		HardwareAccel:              hwaccel,
		HardwareDevice:             hwDevice,
		OutputBaseDir:              GetDefaultOutputDir(),   // /data/dash or ./data/dash
		MinFreeDiskGB:              10,                      // Require 10GB free space
		MaxCPUPercent:              0,                       // Unlimited by default
		MaxMemoryMB:                getDefaultMaxMemoryMB(), // Based on system RAM
		ProcessGroupKill:           true,                    // Always kill process group
		ToneMappingEnabled:         toneMappingEnabled,      // Enable tone mapping by default
		ToneMappingAlgorithm:       toneMappingAlgorithm,    // Default: bt.2390 (ITU-R BT.2390)
		ToneMappingBackend:         toneMappingBackend,      // Default: auto
		LibPlaceboPeakDetect:       libPlaceboPeakDetect,    // Default: true
		LibPlaceboContrastRecovery: libPlaceboContrastRecovery, // Default: 0.3
		FFmpegLogEnabled:           ffmpegLogEnabled,        // Default: true
		FFmpegLogRetentionHours:    ffmpegLogRetentionHours, // Default: 48 hours
	}
}

// DetectHardwareAccel attempts to detect available hardware acceleration.
// Priority: NVENC > QSV > VAAPI > VideoToolbox > None
// If encoderChecker is nil, uses a simple exec-based check.
func DetectHardwareAccel(encoderChecker EncoderChecker) HardwareAccel {
	// Check for environment variable override
	if accel := os.Getenv("HARDWARE_ACCEL"); accel != "" {
		switch HardwareAccel(accel) {
		case AccelNVENC, AccelVAAPI, AccelQSV, AccelVideoToolbox, AccelNone:
			return HardwareAccel(accel)
		}
	}

	// Use provided checker or fall back to simple check
	checkEncoder := encoderChecker
	if checkEncoder == nil {
		checkEncoder = simpleEncoderCheck
	}

	// Detect NVIDIA GPU (nvenc)
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		// Verify FFmpeg has nvenc support
		if checkEncoder("h264_nvenc") {
			return AccelNVENC
		}
	}

	// Detect Intel Quick Sync (qsv)
	if _, err := os.Stat("/dev/dri/renderD128"); err == nil {
		if checkEncoder("h264_qsv") {
			return AccelQSV
		}
	}

	// Detect VAAPI (Intel/AMD on Linux)
	if _, err := os.Stat("/dev/dri/renderD128"); err == nil {
		if checkEncoder("h264_vaapi") {
			return AccelVAAPI
		}
	}

	// Default to software encoding
	return AccelNone
}

// simpleEncoderCheck is a basic encoder check using ffmpeg -encoders.
// Uses paths.Paths to ensure consistent FFmpeg binary and library path configuration.
func simpleEncoderCheck(encoderName string) bool {
	// Get FFmpeg paths (same resolution logic as DefaultFromProfile)
	p, err := paths.New()
	if err != nil {
		return false
	}

	// Check if encoder is listed in -encoders output
	cmd := p.PrepareCommand("ffmpeg", "-encoders")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Verify encoder is actually usable with -h encoder=name
	helpCmd := p.PrepareCommand("ffmpeg", "-h", "encoder="+encoderName)
	return len(output) > 0 && helpCmd.Run() == nil
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

// getDefaultMaxMemoryMB calculates a sensible memory limit based on system RAM.
// This limits per-process FFmpeg memory to prevent OOM crashes during complex
// operations like HDR tone mapping with libplacebo or PGS subtitle burn-in.
//
// Strategy: Assume up to 4 concurrent transcodes, reserve 25% for system.
// Per-process limit = (totalRAM * 0.75) / 4, capped at 4GB.
func getDefaultMaxMemoryMB() int {
	var sysinfo syscall.Sysinfo_t
	if err := syscall.Sysinfo(&sysinfo); err != nil {
		// Can't detect system memory, use conservative 2GB default
		return 2048
	}

	totalRAM := sysinfo.Totalram * uint64(sysinfo.Unit)
	totalMB := totalRAM / (1024 * 1024)

	// Reserve 25% for system, divide rest among 4 potential transcodes
	availableMB := (totalMB * 75) / 100
	maxMemMB := int(availableMB / 4)

	// Clamp between 1GB and 4GB per process
	// 4GB is plenty for any single FFmpeg operation
	if maxMemMB < 1024 {
		maxMemMB = 1024 // Minimum 1GB
	}
	if maxMemMB > 4096 {
		maxMemMB = 4096 // Maximum 4GB per process
	}

	return maxMemMB
}
