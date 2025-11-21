package transcoding

// hardware.go - Unified hardware acceleration configuration
// Consolidates hardware logic previously duplicated across ffmpeg.go, session.go, and hardware_fallback.go

// GetVideoCodecAndPreset returns the appropriate video codec and preset for a hardware acceleration type.
// Returns (codec, preset). Preset is empty for hardware encoders.
func GetVideoCodecAndPreset(hwAccel HardwareAccel) (codec string, preset string) {
	switch hwAccel {
	case AccelVAAPI:
		return "h264_vaapi", ""
	case AccelNVENC:
		return "h264_nvenc", ""
	case AccelQSV:
		return "h264_qsv", ""
	case AccelVideoToolbox:
		return "h264_videotoolbox", ""
	default:
		// Software encoding with preset
		return "libx264", "medium"
	}
}

// GetHardwareAccelArgs returns FFmpeg arguments for hardware acceleration.
// These args must come BEFORE the input file (-i).
// Uses the device path from config for VAAPI/QSV (defaults to /dev/dri/renderD128).
func GetHardwareAccelArgs(hwAccel HardwareAccel) []string {
	return GetHardwareAccelArgsWithDevice(hwAccel, "/dev/dri/renderD128")
}

// GetHardwareAccelArgsWithDevice returns FFmpeg arguments for hardware acceleration with a specific device.
// These args must come BEFORE the input file (-i).
func GetHardwareAccelArgsWithDevice(hwAccel HardwareAccel, device string) []string {
	switch hwAccel {
	case AccelVAAPI:
		return []string{
			"-hwaccel", "vaapi",
			"-hwaccel_device", device,
			"-hwaccel_output_format", "vaapi",
		}
	case AccelNVENC:
		// Enable CUDA hardware decoding for full GPU pipeline
		// Decoding (NVDEC) → Scaling (scale_cuda) → Encoding (NVENC)
		// This keeps all frames in GPU memory, avoiding PCIe bottlenecks
		// Device selection happens automatically via CUDA
		return []string{
			"-hwaccel", "cuda",
			"-hwaccel_output_format", "cuda",
		}
	case AccelQSV:
		return []string{
			"-hwaccel", "qsv",
			"-hwaccel_output_format", "qsv",
		}
	case AccelVideoToolbox:
		return []string{
			"-hwaccel", "videotoolbox",
		}
	default:
		// No hardware acceleration
		return []string{}
	}
}
