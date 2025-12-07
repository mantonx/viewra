package ffmpeg

import (
	infraffmpeg "github.com/mantonx/viewra/internal/infrastructure/ffmpeg"
)

// types.go - Type definitions used by the ffmpeg package and its consumers.
// These types are defined here to avoid import cycles with the parent transcoding package.

// AdaptiveProfile defines granular bitrate-based quality profiles for adaptive streaming.
type AdaptiveProfile struct {
	// Identity
	ID          string
	DisplayName string

	// Resolution
	Width  int
	Height int

	// Bitrate
	VideoBitrate int
	VideoMaxRate int
	VideoBufSize int

	// Audio
	AudioBitrate     int
	AudioChannels    int
	AudioSampleRate  int
	PreserveMultiCh  bool
	AudioCodec       string
	MaxAudioChannels int

	// Codec preferences
	PreferredCodec string
	FallbackCodecs []string

	// Encoding parameters
	Preset          string
	CRF             int
	EnableHWAccel   bool
	EnableFastStart bool

	// HLS segments
	SegmentDuration int
	GOPSize         int

	// Frame rate and aspect ratio
	FrameRate   float64
	AspectRatio string
	Is3D        bool
	StereoMode  string
}

// VideoInfo contains video file metadata extracted via ffprobe.
type VideoInfo struct {
	Codec           string
	Width           int
	Height          int
	Bitrate         int64
	Duration        float64
	AudioCodec      string
	AudioBitrate    int64
	AudioChannels   int
	ContainerFormat string

	// HDR and color space metadata
	PixelFormat    string
	ColorSpace     string
	ColorPrimaries string
	ColorTransfer  string
	BitDepth       int
	IsHDR          bool
}

// TranscodeConfig holds transcoding configuration.
// Note: FFmpegPaths uses the infrastructure/ffmpeg.Paths type.
type TranscodeConfig struct {
	FFmpegPaths      *Paths // From infrastructure/ffmpeg package
	HardwareAccel    HardwareAccel
	HardwareDevice   string
	ProcessGroupKill bool
	MaxMemoryMB      int
}

// Paths is an alias to the infrastructure/ffmpeg.Paths type.
// Re-exported here to avoid import cycles in the session package.
type Paths = infraffmpeg.Paths

const (
	// SegmentFilenameFormat is the format string for segment filenames
	SegmentFilenameFormat = "seg_%06d.ts"
)
