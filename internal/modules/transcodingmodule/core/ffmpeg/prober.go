// Package ffmpeg provides utilities for FFmpeg-based video transcoding.
// This file handles media probing using FFprobe.
package ffmpeg

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/hashicorp/go-hclog"
)

// MediaProber uses FFprobe to extract media information
type MediaProber struct {
	logger hclog.Logger
}

// ProbeResult contains media information from FFprobe
type ProbeResult struct {
	Format struct {
		Duration string `json:"duration"`
		BitRate  string `json:"bit_rate"`
		Size     string `json:"size"`
	} `json:"format"`
	Streams []struct {
		CodecType string `json:"codec_type"`
		CodecName string `json:"codec_name"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		BitRate   string `json:"bit_rate"`
	} `json:"streams"`
}

// NewMediaProber creates a new media prober
func NewMediaProber(logger hclog.Logger) *MediaProber {
	return &MediaProber{logger: logger}
}

// GetDuration extracts the duration of a media file
func (mp *MediaProber) GetDuration(inputPath string) (time.Duration, error) {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		inputPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	var result ProbeResult
	if err := json.Unmarshal(output, &result); err != nil {
		return 0, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	// Parse duration
	if result.Format.Duration != "" {
		seconds, err := time.ParseDuration(result.Format.Duration + "s")
		if err != nil {
			return 0, fmt.Errorf("failed to parse duration: %w", err)
		}
		return seconds, nil
	}

	return 0, fmt.Errorf("no duration found in media file")
}
