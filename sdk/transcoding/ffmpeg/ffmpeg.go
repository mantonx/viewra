// Package ffmpeg provides FFmpeg command building and execution utilities.
// This package handles hardware acceleration detection, codec selection, and
// optimal encoding parameters for various video streaming scenarios.
package ffmpeg

import (
	"github.com/mantonx/viewra/sdk/transcoding/types"
)

// Common hardware acceleration types
const (
	HWAccelNone         = "none"
	HWAccelAuto         = "auto"
	HWAccelNVIDIA       = "cuda"
	HWAccelVAAPI        = "vaapi"
	HWAccelQSV          = "qsv"
	HWAccelVideoToolbox = "videotoolbox"
)

// GetVideoEncoder returns the appropriate video encoder for hardware type
func GetVideoEncoder(codec string, hwAccelType string) string {
	switch hwAccelType {
	case HWAccelNVIDIA:
		switch codec {
		case "h264":
			return "h264_nvenc"
		case "h265", "hevc":
			return "hevc_nvenc"
		default:
			return "h264_nvenc"
		}
	case HWAccelVAAPI:
		switch codec {
		case "h264":
			return "h264_vaapi"
		case "h265", "hevc":
			return "hevc_vaapi"
		default:
			return "h264_vaapi"
		}
	case HWAccelQSV:
		switch codec {
		case "h264":
			return "h264_qsv"
		case "h265", "hevc":
			return "hevc_qsv"
		default:
			return "h264_qsv"
		}
	default:
		// Software encoding
		switch codec {
		case "h264":
			return "libx264"
		case "h265", "hevc":
			return "libx265"
		case "vp9":
			return "libvpx-vp9"
		case "av1":
			return "libaom-av1"
		default:
			return "libx264"
		}
	}
}

// BuildCommand builds the FFmpeg command with proper arguments.
// This is a convenience function that creates an ArgsBuilder and builds
// the command in one step.
func BuildCommand(req types.TranscodeRequest, outputPath string, logger types.Logger) []string {
	builder := NewFFmpegArgsBuilder(logger)
	return builder.BuildArgs(req, outputPath)
}