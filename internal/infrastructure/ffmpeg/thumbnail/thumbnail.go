// Package thumbnail provides video thumbnail generation using FFmpeg.
//
// This package handles:
//   - Extracting single frames from video files
//   - Scaling thumbnails to specified dimensions
//   - Quality configuration for output images
package thumbnail

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg/paths"
)

// Client provides methods for generating thumbnails using FFmpeg.
type Client struct {
	paths *paths.Paths
}

// NewClient creates a new thumbnail client.
// If paths is nil, creates new Paths from environment/PATH.
func NewClient(p *paths.Paths) (*Client, error) {
	var err error
	if p == nil {
		p, err = paths.New()
		if err != nil {
			return nil, err
		}
	}
	return &Client{paths: p}, nil
}

// Options configures thumbnail generation.
type Options struct {
	// Timestamp is the time position to extract the thumbnail from
	Timestamp time.Duration

	// Width is the desired thumbnail width in pixels (0 for auto)
	Width int

	// Height is the desired thumbnail height in pixels (0 for auto)
	Height int

	// Quality is the output quality (1-31, lower is better, default 2)
	Quality int
}

// Generate generates a thumbnail image from a video file.
// The output path should include the desired image extension (e.g., .jpg, .png).
func (c *Client) Generate(ctx context.Context, videoPath, outputPath string, opts Options) error {
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

	cmd := c.paths.PrepareCommand("ffmpeg", args...)
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
