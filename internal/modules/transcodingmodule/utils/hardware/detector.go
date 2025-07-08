// Package hardware provides hardware acceleration detection and management.
// This package automatically detects available hardware encoders and decoders,
// including NVIDIA NVENC, Intel Quick Sync, AMD VCE, and Apple VideoToolbox.
package hardware

import (
	"context"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/logger"
	plugins "github.com/mantonx/viewra/sdk"
)

// HardwareInfo contains information about available hardware acceleration
type HardwareInfo struct {
	Available bool
	Type      string
	Encoders  map[string][]string
}

// Detector detects available hardware acceleration
type Detector struct {
	hwInfo     *HardwareInfo
	lastDetect time.Time
	mu         sync.RWMutex
}

// NewDetector creates a new hardware detector
func NewDetector() *Detector {
	return &Detector{}
}

// Detect detects available hardware acceleration
func (d *Detector) Detect() (*HardwareInfo, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Cache hardware info for 5 minutes
	if d.hwInfo != nil && time.Since(d.lastDetect) < 5*time.Minute {
		return d.hwInfo, nil
	}

	logger.Info("detecting hardware acceleration capabilities")

	hwInfo := &HardwareInfo{
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
		logger.Info("NVIDIA hardware acceleration detected")
	}

	// Check for VAAPI (Intel/AMD on Linux)
	if d.hasVAAPI() {
		hwInfo.Available = true
		if hwInfo.Type == "none" {
			hwInfo.Type = "vaapi"
		}
		hwInfo.Encoders["h264"] = append(hwInfo.Encoders["h264"], "h264_vaapi")
		hwInfo.Encoders["hevc"] = append(hwInfo.Encoders["hevc"], "hevc_vaapi")
		logger.Info("VAAPI hardware acceleration detected")
	}

	// Check for QSV (Intel)
	if d.hasQSV() {
		hwInfo.Available = true
		if hwInfo.Type == "none" {
			hwInfo.Type = "qsv"
		}
		hwInfo.Encoders["h264"] = append(hwInfo.Encoders["h264"], "h264_qsv")
		hwInfo.Encoders["hevc"] = append(hwInfo.Encoders["hevc"], "hevc_qsv")
		logger.Info("Intel QSV hardware acceleration detected")
	}

	// Check for VideoToolbox (macOS)
	if d.hasVideoToolbox() {
		hwInfo.Available = true
		hwInfo.Type = "videotoolbox"
		hwInfo.Encoders["h264"] = []string{"h264_videotoolbox"}
		hwInfo.Encoders["hevc"] = []string{"hevc_videotoolbox"}
		logger.Info("VideoToolbox hardware acceleration detected")
	}

	d.hwInfo = hwInfo
	d.lastDetect = time.Now()

	return hwInfo, nil
}

// GetBestEncoder returns the best encoder for given codec
func (d *Detector) GetBestEncoder(codec string) string {
	hwInfo, err := d.Detect()
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
func (d *Detector) IsEncoderAvailable(encoder string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-encoders")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.Contains(string(output), encoder)
}

// GetHardwareAccelerators returns available hardware accelerators
func (d *Detector) GetHardwareAccelerators() []plugins.HardwareAccelerator {
	hwInfo, err := d.Detect()
	if err != nil || !hwInfo.Available {
		return []plugins.HardwareAccelerator{
			{
				Type:        "software",
				ID:          "cpu",
				Name:        "Software CPU Encoding",
				Available:   true,
				DeviceCount: 1,
			},
		}
	}

	var accelerators []plugins.HardwareAccelerator

	switch hwInfo.Type {
	case "nvidia":
		accelerators = append(accelerators, plugins.HardwareAccelerator{
			Type:        "nvidia",
			ID:          "nvenc",
			Name:        "NVIDIA NVENC",
			Available:   true,
			DeviceCount: 1, // TODO: Detect actual GPU count
		})
	case "vaapi":
		accelerators = append(accelerators, plugins.HardwareAccelerator{
			Type:        "vaapi",
			ID:          "vaapi",
			Name:        "VA-API Hardware Acceleration",
			Available:   true,
			DeviceCount: 1,
		})
	case "qsv":
		accelerators = append(accelerators, plugins.HardwareAccelerator{
			Type:        "intel",
			ID:          "qsv",
			Name:        "Intel Quick Sync Video",
			Available:   true,
			DeviceCount: 1,
		})
	case "videotoolbox":
		accelerators = append(accelerators, plugins.HardwareAccelerator{
			Type:        "apple",
			ID:          "videotoolbox",
			Name:        "Apple VideoToolbox",
			Available:   true,
			DeviceCount: 1,
		})
	}

	// Always include CPU as fallback
	accelerators = append(accelerators, plugins.HardwareAccelerator{
		Type:        "software",
		ID:          "cpu",
		Name:        "Software CPU Encoding",
		Available:   true,
		DeviceCount: 1,
	})

	return accelerators
}

// Hardware detection methods
func (d *Detector) hasNVIDIA() bool {
	// Check if nvidia-smi is available
	cmd := exec.Command("nvidia-smi", "--query-gpu=name", "--format=csv,noheader")
	if err := cmd.Run(); err == nil {
		return true
	}
	return false
}

func (d *Detector) hasVAAPI() bool {
	// Check if VAAPI device exists
	cmd := exec.Command("ls", "/dev/dri/renderD128")
	if err := cmd.Run(); err == nil {
		return true
	}
	return false
}

func (d *Detector) hasQSV() bool {
	// Check if Intel GPU is present
	cmd := exec.Command("lspci")
	output, err := cmd.Output()
	if err == nil && strings.Contains(strings.ToLower(string(output)), "intel") {
		// Further check if QSV is available in FFmpeg
		return d.IsEncoderAvailable("h264_qsv")
	}
	return false
}

func (d *Detector) hasVideoToolbox() bool {
	// Check if running on macOS
	cmd := exec.Command("uname", "-s")
	output, err := cmd.Output()
	if err == nil && strings.TrimSpace(string(output)) == "Darwin" {
		return d.IsEncoderAvailable("h264_videotoolbox")
	}
	return false
}

// getSoftwareEncoder returns the appropriate software encoder for a codec
func (d *Detector) getSoftwareEncoder(codec string) string {
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
