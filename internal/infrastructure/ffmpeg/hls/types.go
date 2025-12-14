package hls

import (
	"github.com/mantonx/viewra/internal/domain/streaming"
	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/paths"
)

// Paths is an alias to the paths.Paths type for convenience.
// This allows consumers to use hls.Paths instead of importing both packages.
type Paths = paths.Paths

// Type aliases from domain/streaming - allows existing code to continue using hls.X

// VideoInfo contains video file metadata extracted via ffprobe.
type VideoInfo = streaming.VideoInfo

// VideoCodec represents supported video codecs for transcoding.
type VideoCodec = streaming.VideoCodec

// HardwareAccel represents hardware acceleration type.
type HardwareAccel = streaming.HardwareAccel

// Re-export codec constants from domain/streaming
const (
	CodecH264 = streaming.CodecH264
	CodecH265 = streaming.CodecH265
	CodecVP9  = streaming.CodecVP9
	CodecAV1  = streaming.CodecAV1
)

// Re-export hardware acceleration constants from domain/streaming
const (
	AccelNone         = streaming.AccelNone
	AccelVAAPI        = streaming.AccelVAAPI
	AccelNVENC        = streaming.AccelNVENC
	AccelQSV          = streaming.AccelQSV
	AccelVideoToolbox = streaming.AccelVideoToolbox
)

// Options contains options for the transcode operation.
type Options struct {
	InputPath                  string
	OutputDir                  string
	Profile                    *Profile
	ProgressHandler            func(progress int)
	AudioTrackIndex            int        // Specific audio track to use (for -map 0:a:N)
	UseSpecificAudioTrack      bool       // If true, use AudioTrackIndex; if false, use default (first)
	StartPosition              int        // Start position in seconds (for seek-based transcoding)
	UseStartPosition           bool       // If true, use StartPosition for seeking
	VideoInfo                  *VideoInfo // Video metadata including HDR info (optional)
	ToneMappingEnabled         bool       // Enable HDR to SDR tone mapping for HDR content
	ToneMappingAlgorithm       string     // Tone mapping algorithm: none, linear, gamma, clip, reinhard, hable, mobius, bt.2390, bt.2446a, spline
	ToneMappingBackend         string     // Tone mapping backend: auto, libplacebo, opencl, vaapi, cpu
	LibPlaceboPeakDetect       bool       // Enable dynamic peak detection for libplacebo (default: true)
	LibPlaceboContrastRecovery float64    // Contrast recovery for libplacebo (0.0-3.0, default: 0.3)
	VideoCodec                 VideoCodec // Target codec: h264, h265, vp9, av1 (default: h264)
}

// Profile defines granular bitrate-based quality profiles for adaptive streaming.
type Profile struct {
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

// Config holds transcoding configuration.
type Config struct {
	FFmpegPath       string // Path to FFmpeg binary
	FFmpegLibPath    string // LD_LIBRARY_PATH for custom builds
	HardwareAccel    HardwareAccel
	HardwareDevice   string
	ProcessGroupKill bool
	MaxMemoryMB      int
}
