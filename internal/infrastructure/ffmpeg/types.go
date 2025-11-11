package ffmpeg

import "time"

// Client provides methods for interacting with FFmpeg and FFprobe.
type Client struct {
	ffmpegPath  string
	ffprobePath string
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
