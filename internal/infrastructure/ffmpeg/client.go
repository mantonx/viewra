package ffmpeg

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// NewClient creates a new FFmpeg client.
// It checks for the presence of ffmpeg and ffprobe executables in the system PATH.
func NewClient() (*Client, error) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, ErrFFmpegNotFound
	}

	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		return nil, ErrFFprobeNotFound
	}

	return &Client{
		ffmpegPath:  ffmpegPath,
		ffprobePath: ffprobePath,
	}, nil
}

// ffprobeOutput represents the JSON structure returned by ffprobe.
type ffprobeOutput struct {
	Format struct {
		Duration   string            `json:"duration"`
		Size       string            `json:"size"`
		BitRate    string            `json:"bit_rate"`
		FormatName string            `json:"format_name"`
		Tags       map[string]string `json:"tags"`
	} `json:"format"`
	Streams []struct {
		CodecType  string `json:"codec_type"`
		CodecName  string `json:"codec_name"`
		Width      int    `json:"width"`
		Height     int    `json:"height"`
		RFrameRate string `json:"r_frame_rate"`
		BitRate    string `json:"bit_rate"`
	} `json:"streams"`
}

// ExtractMetadata extracts metadata from a video file using ffprobe.
func (c *Client) ExtractMetadata(ctx context.Context, filePath string) (*VideoMetadata, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, ErrInvalidFile
	}

	// Run ffprobe with JSON output
	cmd := exec.CommandContext(ctx, c.ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMetadataExtraction, err)
	}

	// Parse JSON output
	var probe ffprobeOutput
	if err := json.Unmarshal(output, &probe); err != nil {
		return nil, fmt.Errorf("%w: failed to parse ffprobe output: %v", ErrMetadataExtraction, err)
	}

	metadata := &VideoMetadata{}

	// Extract duration
	if probe.Format.Duration != "" {
		durationSec, err := strconv.ParseFloat(probe.Format.Duration, 64)
		if err == nil {
			metadata.Duration = time.Duration(durationSec * float64(time.Second))
		}
	}

	// Extract file size
	if probe.Format.Size != "" {
		fileSize, err := strconv.ParseInt(probe.Format.Size, 10, 64)
		if err == nil {
			metadata.FileSize = fileSize
		}
	}

	// Extract bitrate
	if probe.Format.BitRate != "" {
		bitrate, err := strconv.ParseInt(probe.Format.BitRate, 10, 64)
		if err == nil {
			metadata.Bitrate = bitrate
		}
	}

	// Extract video and audio stream information
	for _, stream := range probe.Streams {
		switch stream.CodecType {
		case "video":
			metadata.VideoCodec = stream.CodecName
			metadata.Width = stream.Width
			metadata.Height = stream.Height

			// Parse frame rate (format: "num/den")
			if stream.RFrameRate != "" {
				parts := strings.Split(stream.RFrameRate, "/")
				if len(parts) == 2 {
					num, err1 := strconv.ParseFloat(parts[0], 64)
					den, err2 := strconv.ParseFloat(parts[1], 64)
					if err1 == nil && err2 == nil && den != 0 {
						metadata.FrameRate = num / den
					}
				}
			}

		case "audio":
			if metadata.AudioCodec == "" { // Take first audio stream
				metadata.AudioCodec = stream.CodecName
			}
		}
	}

	return metadata, nil
}

// GenerateThumbnail generates a thumbnail image from a video file.
// The output path should include the desired image extension (e.g., .jpg, .png).
func (c *Client) GenerateThumbnail(ctx context.Context, videoPath, outputPath string, opts ThumbnailOptions) error {
	// Check if file exists
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return ErrInvalidFile
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("%w: failed to create output directory: %v", ErrThumbnailGeneration, err)
	}

	// Set default quality if not specified
	quality := opts.Quality
	if quality == 0 {
		quality = 2
	}

	// Build ffmpeg command
	args := []string{
		"-ss", formatDuration(opts.Timestamp),
		"-i", videoPath,
		"-vframes", "1",
		"-q:v", strconv.Itoa(quality),
	}

	// Add scaling if width or height specified
	if opts.Width > 0 || opts.Height > 0 {
		width := opts.Width
		height := opts.Height
		if width == 0 {
			width = -1 // Auto-scale width
		}
		if height == 0 {
			height = -1 // Auto-scale height
		}
		args = append(args, "-vf", fmt.Sprintf("scale=%d:%d", width, height))
	}

	args = append(args, "-y", outputPath)

	cmd := exec.CommandContext(ctx, c.ffmpegPath, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %v: %s", ErrThumbnailGeneration, err, string(output))
	}

	// Verify output file was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return fmt.Errorf("%w: output file was not created", ErrThumbnailGeneration)
	}

	return nil
}

// formatDuration formats a time.Duration into a string suitable for ffmpeg (HH:MM:SS.mmm).
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	milliseconds := int(d.Milliseconds()) % 1000

	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, milliseconds)
}
