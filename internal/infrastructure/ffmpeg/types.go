package ffmpeg

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Paths holds FFmpeg and FFprobe binary paths with optional library path.
// This is the single source of truth for FFmpeg configuration across the application.
// Create using NewPaths() which handles environment variables and PATH lookup.
type Paths struct {
	FFmpeg  string // Path to FFmpeg binary
	FFprobe string // Path to FFprobe binary
	LibPath string // LD_LIBRARY_PATH for custom builds (empty for system FFmpeg)
}

// NewPaths resolves FFmpeg/FFprobe paths from environment or system PATH.
// Priority: VIEWRA_FFMPEG_PATH > system PATH
// Returns error if binaries cannot be found.
func NewPaths() (*Paths, error) {
	p := &Paths{}

	// FFmpeg path
	p.FFmpeg = os.Getenv("VIEWRA_FFMPEG_PATH")
	if p.FFmpeg == "" {
		path, err := exec.LookPath("ffmpeg")
		if err != nil {
			return nil, fmt.Errorf("ffmpeg not found in PATH and VIEWRA_FFMPEG_PATH not set: %w", err)
		}
		p.FFmpeg = path
	}

	// FFprobe path
	p.FFprobe = os.Getenv("VIEWRA_FFPROBE_PATH")
	if p.FFprobe == "" {
		path, err := exec.LookPath("ffprobe")
		if err != nil {
			return nil, fmt.Errorf("ffprobe not found in PATH and VIEWRA_FFPROBE_PATH not set: %w", err)
		}
		p.FFprobe = path
	}

	// Library path (optional - only needed for custom builds)
	p.LibPath = os.Getenv("VIEWRA_FFMPEG_LIB_PATH")

	return p, nil
}

// IsCustomBuild returns true if using a custom FFmpeg build (has lib path).
func (p *Paths) IsCustomBuild() bool {
	return p.LibPath != ""
}

// Version returns the FFmpeg version string.
func (p *Paths) Version() string {
	cmd := exec.Command(p.FFmpeg, "-version")
	if p.LibPath != "" {
		cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+p.LibPath)
	}

	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 && strings.HasPrefix(lines[0], "ffmpeg version ") {
		parts := strings.Fields(lines[0])
		if len(parts) >= 3 {
			return parts[2]
		}
	}
	return "unknown"
}

// PrepareCommand creates an exec.Cmd with LD_LIBRARY_PATH set if needed.
func (p *Paths) PrepareCommand(name string, args ...string) *exec.Cmd {
	var binary string
	switch name {
	case "ffmpeg":
		binary = p.FFmpeg
	case "ffprobe":
		binary = p.FFprobe
	default:
		binary = name
	}

	cmd := exec.Command(binary, args...)
	if p.LibPath != "" {
		cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+p.LibPath)
	}
	return cmd
}

// Client provides methods for interacting with FFmpeg and FFprobe.
type Client struct {
	paths *Paths
}

// VideoMetadata contains extracted metadata from a video file.
type VideoMetadata struct {
	// Duration is the total duration of the video
	Duration time.Duration

	// Width is the video width in pixels
	Width int

	// Height is the video height in pixels
	Height int

	// VideoCodec is the codec used for video encoding (e.g., "h264", "hevc")
	VideoCodec string

	// AudioCodec is the codec used for audio encoding (e.g., "aac", "mp3")
	AudioCodec string

	// Bitrate is the overall bitrate in bits per second
	Bitrate int64

	// FrameRate is the frames per second
	FrameRate float64

	// FileSize is the size of the file in bytes
	FileSize int64

	// CodecProfile is the codec profile (e.g., "High", "Main", "Baseline")
	CodecProfile string

	// ScanType indicates if the video is progressive or interlaced
	ScanType string

	// HDRFormat indicates the HDR format (e.g., "HDR10", "Dolby Vision", "HLG")
	HDRFormat string

	// ColorSpace is the color space (e.g., "bt709", "bt2020nc")
	ColorSpace string

	// ColorPrimaries defines the color primaries (e.g., "bt709", "bt2020")
	ColorPrimaries string
}

// ThumbnailOptions configures thumbnail generation.
type ThumbnailOptions struct {
	// Timestamp is the time position to extract the thumbnail from
	Timestamp time.Duration

	// Width is the desired thumbnail width in pixels (0 for auto)
	Width int

	// Height is the desired thumbnail height in pixels (0 for auto)
	Height int

	// Quality is the output quality (1-31, lower is better, default 2)
	Quality int
}

// AudioTrackInfo contains metadata for an audio stream.
type AudioTrackInfo struct {
	StreamIndex   int
	Codec         string
	CodecProfile  string
	Channels      int
	ChannelLayout string
	SampleRate    int
	BitRate       int
	Language      string
	Title         string
	IsDefault     bool
	IsCommentary  bool
	IsDescriptive bool
}

// SubtitleTrackInfo contains metadata for a subtitle stream.
type SubtitleTrackInfo struct {
	StreamIndex  int
	Codec        string
	Language     string
	Title        string
	IsDefault    bool
	IsForced     bool
	IsSDH        bool
	IsCommentary bool
	IsBitmap     bool
}

// MediaTracksInfo contains all audio and subtitle tracks from a media file.
type MediaTracksInfo struct {
	AudioTracks    []AudioTrackInfo
	SubtitleTracks []SubtitleTrackInfo
}
