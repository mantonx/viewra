package ffmpeg

// VideoCodec represents supported video codecs for transcoding
type VideoCodec string

const (
	CodecH264 VideoCodec = "h264"
	CodecH265 VideoCodec = "h265"
	CodecVP9  VideoCodec = "vp9"
	CodecAV1  VideoCodec = "av1"
)

// HardwareAccel represents hardware acceleration type.
// This is duplicated from parent package to avoid import cycle.
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

// GetVideoCodecAndPreset returns the appropriate video codec and preset for a hardware acceleration type.
// Returns (codec, preset). Preset is empty for hardware encoders.
// Defaults to H.264 for maximum compatibility.
func GetVideoCodecAndPreset(hwAccel HardwareAccel) (codec string, preset string) {
	return GetVideoCodecAndPresetForCodec(hwAccel, CodecH264)
}

// GetVideoCodecAndPresetForCodec returns the FFmpeg encoder name and preset for a given codec and hardware acceleration.
// Returns (encoderName, preset). Preset is empty for hardware encoders.
func GetVideoCodecAndPresetForCodec(hwAccel HardwareAccel, videoCodec VideoCodec) (encoder string, preset string) {
	switch videoCodec {
	case CodecH265:
		return getH265Encoder(hwAccel)
	case CodecVP9:
		return getVP9Encoder(hwAccel)
	case CodecAV1:
		return getAV1Encoder(hwAccel)
	default: // CodecH264
		return getH264Encoder(hwAccel)
	}
}

// getH264Encoder returns the H.264 encoder for the given hardware acceleration
func getH264Encoder(hwAccel HardwareAccel) (encoder string, preset string) {
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
		return "libx264", "medium"
	}
}

// getH265Encoder returns the H.265/HEVC encoder for the given hardware acceleration
func getH265Encoder(hwAccel HardwareAccel) (encoder string, preset string) {
	switch hwAccel {
	case AccelVAAPI:
		return "hevc_vaapi", ""
	case AccelNVENC:
		return "hevc_nvenc", ""
	case AccelQSV:
		return "hevc_qsv", ""
	case AccelVideoToolbox:
		return "hevc_videotoolbox", ""
	default:
		return "libx265", "medium"
	}
}

// getVP9Encoder returns the VP9 encoder for the given hardware acceleration
// Note: VP9 hardware encoding support is limited
func getVP9Encoder(hwAccel HardwareAccel) (encoder string, preset string) {
	switch hwAccel {
	case AccelVAAPI:
		return "vp9_vaapi", ""
	case AccelQSV:
		return "vp9_qsv", ""
	default:
		// NVENC and VideoToolbox don't support VP9 encoding
		// Fall back to software encoder (libvpx-vp9)
		return "libvpx-vp9", ""
	}
}

// getAV1Encoder returns the AV1 encoder for the given hardware acceleration
// Note: AV1 hardware encoding is still limited in availability
func getAV1Encoder(hwAccel HardwareAccel) (encoder string, preset string) {
	switch hwAccel {
	case AccelVAAPI:
		return "av1_vaapi", ""
	case AccelNVENC:
		// NVENC AV1 requires RTX 40 series or newer
		return "av1_nvenc", ""
	case AccelQSV:
		// QSV AV1 requires Intel Arc or 12th gen+ with AV1 support
		return "av1_qsv", ""
	default:
		// Software AV1 encoding is very slow but widely compatible
		// SVT-AV1 is faster than libaom
		return "libsvtav1", ""
	}
}

// IsCodecSupported checks if a codec is supported for the given hardware acceleration
func IsCodecSupported(hwAccel HardwareAccel, videoCodec VideoCodec) bool {
	// H.264 is universally supported
	if videoCodec == CodecH264 {
		return true
	}

	// H.265 is widely supported
	if videoCodec == CodecH265 {
		return true
	}

	// VP9 has limited hardware support
	if videoCodec == CodecVP9 {
		switch hwAccel {
		case AccelVAAPI, AccelQSV, AccelNone:
			return true
		default:
			// NVENC and VideoToolbox don't support VP9
			return false
		}
	}

	// AV1 has newer hardware support
	if videoCodec == CodecAV1 {
		// All accelerators have AV1 support (though may require newer hardware)
		return true
	}

	return false
}

// GetBestCodecForHardware returns the best codec for bandwidth savings given hardware support
// Prioritizes: AV1 > H.265/VP9 > H.264
func GetBestCodecForHardware(hwAccel HardwareAccel, clientCodecs []string) VideoCodec {
	// Build a set of client-supported codecs
	supported := make(map[string]bool)
	for _, c := range clientCodecs {
		supported[c] = true
	}

	// Try AV1 first (best compression)
	if supported["av1"] && IsCodecSupported(hwAccel, CodecAV1) {
		return CodecAV1
	}

	// Try H.265 (good compression, good browser support)
	if supported["h265"] && IsCodecSupported(hwAccel, CodecH265) {
		return CodecH265
	}

	// Try VP9 (good compression, Chrome/Firefox support)
	if supported["vp9"] && IsCodecSupported(hwAccel, CodecVP9) {
		return CodecVP9
	}

	// Fall back to H.264 (universal compatibility)
	return CodecH264
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
