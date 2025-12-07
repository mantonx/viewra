package ffmpeg

import (
	"bytes"
	"fmt"
	"os/exec"
)

// HardwareTestEncoder provides utilities for testing hardware encoder availability.
type HardwareTestEncoder struct{}

// NewHardwareTestEncoder creates a new hardware test encoder utility.
func NewHardwareTestEncoder() *HardwareTestEncoder {
	return &HardwareTestEncoder{}
}

// TestEncode performs a quick test encode to verify hardware is functional.
// Encodes 1 frame of a 1-second test pattern to verify the encoder works.
// Returns nil if the encoder is working, or an error with details if it fails.
func (h *HardwareTestEncoder) TestEncode(hwAccel HardwareAccel) error {
	encoder, preset := GetVideoCodecAndPreset(hwAccel)

	// Build test command - encode 1 frame of lavfi test pattern
	args := []string{
		"-f", "lavfi",
		"-i", "testsrc=duration=1:size=1280x720:rate=1", // 1 frame
		"-frames:v", "1",
		"-c:v", encoder,
	}

	// Add preset if applicable
	if preset != "" {
		args = append(args, "-preset", preset)
	}

	// Add hardware-specific initialization args
	hwArgs := h.getHardwareInitArgs(hwAccel)
	if len(hwArgs) > 0 {
		args = append(hwArgs, args...)
	}

	// Output to null
	args = append(args, "-f", "null", "-")

	cmd := exec.Command("ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, stderr.String())
	}

	return nil
}

// getHardwareInitArgs returns hardware-specific initialization arguments for test encoding.
// These args configure the hardware device before encoding begins.
func (h *HardwareTestEncoder) getHardwareInitArgs(hwAccel HardwareAccel) []string {
	return h.getHardwareInitArgsWithDevice(hwAccel, "/dev/dri/renderD128")
}

// getHardwareInitArgsWithDevice returns hardware-specific initialization arguments with a specific device.
func (h *HardwareTestEncoder) getHardwareInitArgsWithDevice(hwAccel HardwareAccel, device string) []string {
	switch hwAccel {
	case AccelVAAPI:
		// VAAPI with hardware upload and format conversion
		// Tests the full VAAPI pipeline
		return []string{
			"-init_hw_device", fmt.Sprintf("vaapi=va:%s", device),
			"-filter_hw_device", "va",
			"-vf", "format=nv12,hwupload",
		}
	case AccelQSV:
		return []string{
			"-init_hw_device", fmt.Sprintf("qsv=qsv:%s", device),
			"-filter_hw_device", "qsv",
			"-vf", "hwupload=extra_hw_frames=64,format=qsv",
		}
	case AccelNVENC:
		// NVENC with CUDA hardware acceleration
		// Enables full GPU pipeline for testing
		return []string{
			"-hwaccel", "cuda",
			"-hwaccel_output_format", "cuda",
		}
	case AccelVideoToolbox:
		// VideoToolbox doesn't need special init for test encode
		return []string{}
	default:
		return []string{}
	}
}

// CheckFFmpegEncoder verifies that FFmpeg has support for a specific encoder.
// Returns true if the encoder is available, false otherwise.
func CheckFFmpegEncoder(encoderName string) bool {
	cmd := exec.Command("ffmpeg", "-encoders")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Check if the encoder name appears in the output
	return bytes.Contains(output, []byte(encoderName))
}

// CheckFFmpegFilter verifies that FFmpeg has support for a specific filter.
// Returns true if the filter is available, false otherwise.
func CheckFFmpegFilter(filterName string) bool {
	cmd := exec.Command("ffmpeg", "-filters")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Check if the filter name appears in the output
	return bytes.Contains(output, []byte(filterName))
}
