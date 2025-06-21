package transcoding

import (
	"fmt"
	"strconv"
)

// FFmpegArgsBuilder builds FFmpeg command line arguments from SDK requests
type FFmpegArgsBuilder struct {
	hwAccelType string
	preset      string
	extraArgs   []string
}

// NewFFmpegArgsBuilder creates a new FFmpeg argument builder
func NewFFmpegArgsBuilder(hwAccelType string) *FFmpegArgsBuilder {
	return &FFmpegArgsBuilder{
		hwAccelType: hwAccelType,
		preset:      "medium", // Default preset
	}
}

// WithPreset sets the encoding preset
func (b *FFmpegArgsBuilder) WithPreset(preset string) *FFmpegArgsBuilder {
	b.preset = preset
	return b
}

// WithExtraArgs adds extra FFmpeg arguments
func (b *FFmpegArgsBuilder) WithExtraArgs(args ...string) *FFmpegArgsBuilder {
	b.extraArgs = append(b.extraArgs, args...)
	return b
}

// BuildArgs builds FFmpeg arguments directly from SDK TranscodeRequest
func (b *FFmpegArgsBuilder) BuildArgs(req *TranscodeRequest, outputPath string) []string {
	var args []string

	// Always overwrite output files
	args = append(args, "-y")

	// Hardware acceleration (before input)
	if b.hwAccelType != "none" {
		args = append(args, "-hwaccel", b.hwAccelType)
	}

	// Seek to start position (before input for efficiency)
	if req.Seek > 0 {
		seekSeconds := req.Seek.Seconds()
		args = append(args, "-ss", fmt.Sprintf("%.3f", seekSeconds))
	}

	// Input file
	args = append(args, "-i", req.InputPath)

	// Duration limit (after input)
	if req.Duration > 0 {
		durationSeconds := req.Duration.Seconds()
		args = append(args, "-t", fmt.Sprintf("%.3f", durationSeconds))
	}

	// Video codec and settings
	args = append(args, b.buildVideoArgs(req)...)

	// Audio codec and settings
	args = append(args, b.buildAudioArgs(req)...)

	// Container-specific settings
	args = append(args, b.buildContainerArgs(req)...)

	// Extra arguments
	args = append(args, b.extraArgs...)

	// Output path (always last)
	args = append(args, outputPath)

	return args
}

// buildVideoArgs builds video encoding arguments
func (b *FFmpegArgsBuilder) buildVideoArgs(req *TranscodeRequest) []string {
	var args []string

	// Video codec
	videoCodec := req.VideoCodec
	if videoCodec == "" {
		videoCodec = "h264" // Default
	}
	args = append(args, "-c:v", videoCodec)

	// Encoding preset
	args = append(args, "-preset", b.preset)

	// Quality settings
	if req.Quality > 0 {
		crf := b.qualityToCRF(req.Quality, videoCodec)
		args = append(args, "-crf", strconv.Itoa(crf))
	}

	// Resolution
	if req.Resolution != nil {
		resolution := fmt.Sprintf("%dx%d", req.Resolution.Width, req.Resolution.Height)
		args = append(args, "-s", resolution)
	}

	// Frame rate
	if req.FrameRate != nil {
		args = append(args, "-r", fmt.Sprintf("%.2f", *req.FrameRate))
	}

	// Video stream mapping
	args = append(args, "-map", "0:v:0")

	return args
}

// buildAudioArgs builds audio encoding arguments
func (b *FFmpegArgsBuilder) buildAudioArgs(req *TranscodeRequest) []string {
	var args []string

	// Audio codec
	audioCodec := req.AudioCodec
	if audioCodec == "" {
		audioCodec = "aac" // Default
	}
	args = append(args, "-c:a", audioCodec)

	// Audio bitrate
	args = append(args, "-b:a", "128k") // Default

	// Force stereo for better compatibility
	if audioCodec == "aac" {
		args = append(args,
			"-ac", "2", // Force stereo output
			"-af", "aformat=channel_layouts=stereo", // Ensure stereo channel layout
		)
	}

	// Audio stream mapping
	args = append(args, "-map", "0:a:0")

	return args
}

// buildContainerArgs builds container-specific arguments
func (b *FFmpegArgsBuilder) buildContainerArgs(req *TranscodeRequest) []string {
	var args []string

	switch req.Container {
	case "dash":
		args = append(args,
			"-f", "dash",
			"-seg_duration", "4",
			"-use_template", "1",
			"-use_timeline", "1",
			"-init_seg_name", "init-$RepresentationID$.m4s",
			"-media_seg_name", "chunk-$RepresentationID$-$Number$.m4s",
			"-adaptation_sets", "id=0,streams=v id=1,streams=a",
		)

	case "hls":
		args = append(args,
			"-f", "hls",
			"-hls_time", "4",
			"-hls_playlist_type", "vod",
		)

	case "mp4":
		args = append(args,
			"-f", "mp4",
			"-movflags", "+faststart",
		)

	default:
		if req.Container != "" {
			args = append(args, "-f", req.Container)
		}
	}

	return args
}

// qualityToCRF converts quality percentage to CRF value
func (b *FFmpegArgsBuilder) qualityToCRF(quality int, codec string) int {
	// Quality 0-100 mapped to CRF ranges by codec
	switch codec {
	case "h264":
		// CRF 18-28 for h264 (lower = better quality)
		return 28 - (quality * 10 / 100)
	case "h265", "hevc":
		// CRF 22-32 for h265
		return 32 - (quality * 10 / 100)
	default:
		// Default h264 range
		return 28 - (quality * 10 / 100)
	}
}

// Common hardware acceleration types
const (
	HWAccelNone     = "none"
	HWAccelAuto     = "auto"
	HWAccelNVIDIA   = "cuda"
	HWAccelVAAPI    = "vaapi"
	HWAccelQSV      = "qsv"
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