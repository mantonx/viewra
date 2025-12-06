package ffmpeg

import "time"

// Client provides methods for interacting with FFmpeg and FFprobe.
type Client struct {
	ffmpegPath  string
	ffprobePath string
	libPath     string // Custom library path for patched FFmpeg (LD_LIBRARY_PATH)
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
