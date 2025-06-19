package services

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/mantonx/viewra/data/plugins/ffmpeg_transcoder/internal/types"
	"github.com/mantonx/viewra/pkg/plugins"
)

// hardwareDetector detects available hardware acceleration
type hardwareDetector struct {
	logger     plugins.Logger
	hwInfo     *types.HardwareInfo
	lastDetect time.Time
}

// NewHardwareDetector creates a new hardware detector
func NewHardwareDetector(logger plugins.Logger) HardwareDetector {
	return &hardwareDetector{
		logger: logger,
	}
}

// DetectHardware detects available hardware acceleration
func (d *hardwareDetector) DetectHardware() (*types.HardwareInfo, error) {
	// Cache hardware info for 5 minutes
	if d.hwInfo != nil && time.Since(d.lastDetect) < 5*time.Minute {
		return d.hwInfo, nil
	}

	d.logger.Info("detecting hardware acceleration capabilities")

	hwInfo := &types.HardwareInfo{
		Available: false,
		Type:      "none",
		Encoders:  make(map[string][]string),
	}

	// Check for NVIDIA GPU
	if d.hasNVIDIA() {
		hwInfo.Available = true
		hwInfo.Type = "nvidia"
		hwInfo.Encoders["h264"] = []string{"h264_nvenc"}
		hwInfo.Encoders["hevc"] = []string{"hevc_nvenc"}
		hwInfo.Encoders["av1"] = []string{"av1_nvenc"}
		d.logger.Info("NVIDIA hardware acceleration detected")
	}

	// Check for VAAPI (Intel/AMD on Linux)
	if d.hasVAAPI() {
		hwInfo.Available = true
		if hwInfo.Type == "none" {
			hwInfo.Type = "vaapi"
		}
		hwInfo.Encoders["h264"] = append(hwInfo.Encoders["h264"], "h264_vaapi")
		hwInfo.Encoders["hevc"] = append(hwInfo.Encoders["hevc"], "hevc_vaapi")
		d.logger.Info("VAAPI hardware acceleration detected")
	}

	// Check for QSV (Intel)
	if d.hasQSV() {
		hwInfo.Available = true
		if hwInfo.Type == "none" {
			hwInfo.Type = "qsv"
		}
		hwInfo.Encoders["h264"] = append(hwInfo.Encoders["h264"], "h264_qsv")
		hwInfo.Encoders["hevc"] = append(hwInfo.Encoders["hevc"], "hevc_qsv")
		d.logger.Info("Intel QSV hardware acceleration detected")
	}

	// Check for VideoToolbox (macOS)
	if d.hasVideoToolbox() {
		hwInfo.Available = true
		hwInfo.Type = "videotoolbox"
		hwInfo.Encoders["h264"] = []string{"h264_videotoolbox"}
		hwInfo.Encoders["hevc"] = []string{"hevc_videotoolbox"}
		d.logger.Info("VideoToolbox hardware acceleration detected")
	}

	d.hwInfo = hwInfo
	d.lastDetect = time.Now()

	return hwInfo, nil
}

// GetBestEncoder returns the best encoder for given codec
func (d *hardwareDetector) GetBestEncoder(codec string) string {
	hwInfo, err := d.DetectHardware()
	if err != nil || !hwInfo.Available {
		return d.getSoftwareEncoder(codec)
	}

	// Check hardware encoders
	if encoders, ok := hwInfo.Encoders[codec]; ok && len(encoders) > 0 {
		// Test which encoder is actually available
		for _, encoder := range encoders {
			if d.IsEncoderAvailable(encoder) {
				return encoder
			}
		}
	}

	// Fallback to software encoder
	return d.getSoftwareEncoder(codec)
}

// IsEncoderAvailable checks if an encoder is available
func (d *hardwareDetector) IsEncoderAvailable(encoder string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-encoders")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.Contains(string(output), encoder)
}

// Hardware detection methods
func (d *hardwareDetector) hasNVIDIA() bool {
	// Check if nvidia-smi is available
	cmd := exec.Command("nvidia-smi", "--query-gpu=name", "--format=csv,noheader")
	if err := cmd.Run(); err == nil {
		return true
	}
	return false
}

func (d *hardwareDetector) hasVAAPI() bool {
	// Check if VAAPI device exists
	cmd := exec.Command("ls", "/dev/dri/renderD128")
	if err := cmd.Run(); err == nil {
		return true
	}
	return false
}

func (d *hardwareDetector) hasQSV() bool {
	// Check if Intel GPU is present
	cmd := exec.Command("lspci")
	output, err := cmd.Output()
	if err == nil && strings.Contains(strings.ToLower(string(output)), "intel") {
		// Further check if QSV is available in FFmpeg
		return d.IsEncoderAvailable("h264_qsv")
	}
	return false
}

func (d *hardwareDetector) hasVideoToolbox() bool {
	// Check if running on macOS
	cmd := exec.Command("uname", "-s")
	output, err := cmd.Output()
	if err == nil && strings.TrimSpace(string(output)) == "Darwin" {
		return d.IsEncoderAvailable("h264_videotoolbox")
	}
	return false
}

// getSoftwareEncoder returns the appropriate software encoder for a codec
func (d *hardwareDetector) getSoftwareEncoder(codec string) string {
	switch codec {
	case "h264":
		return "libx264"
	case "h265", "hevc":
		return "libx265"
	case "vp8":
		return "libvpx"
	case "vp9":
		return "libvpx-vp9"
	case "av1":
		return "libaom-av1"
	default:
		return codec // Return as-is, FFmpeg will handle or error
	}
}
