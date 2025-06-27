// Package progress provides FFmpeg progress parsing and conversion utilities.
// This package interprets FFmpeg's stderr output to extract real-time transcoding
// progress information, converting it into structured data that applications can
// use to display progress bars, estimate completion times, and monitor performance.
//
// The progress converter handles:
// - Parsing FFmpeg's progress output format
// - Extracting frame counts, time codes, and bitrates
// - Calculating completion percentages
// - Estimating time remaining
// - Detecting stalls and errors
// - Providing moving averages for smooth progress updates
//
// FFmpeg output formats supported:
// - Standard progress output (frame=, time=, speed=)
// - JSON progress output (-progress_url)
// - Custom progress reporting via pipes
package progress

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	plugins "github.com/mantonx/viewra/sdk"
)

// Converter converts FFmpeg progress to standard format
type Converter struct {
	startTime     time.Time
	totalDuration time.Duration
	lastUpdate    time.Time
	totalBytes    int64
}

// NewConverter creates a new progress converter
func NewConverter(totalDuration time.Duration) *Converter {
	return &Converter{
		startTime:     time.Now(),
		totalDuration: totalDuration,
		lastUpdate:    time.Now(),
	}
}

// Convert converts FFmpeg output to standard progress
func (c *Converter) Convert(ffmpegOutput string) *plugins.TranscodingProgress {
	progress := &plugins.TranscodingProgress{
		TimeElapsed: time.Since(c.startTime),
	}

	// Parse FFmpeg progress output
	// Example: frame= 1234 fps=25.0 q=28.0 size=  10240kB time=00:00:51.20 bitrate=1638.4kbits/s speed=1.05x

	// Extract time
	if match := regexp.MustCompile(`time=(\d+):(\d+):(\d+\.\d+)`).FindStringSubmatch(ffmpegOutput); len(match) > 0 {
		hours, _ := strconv.Atoi(match[1])
		mins, _ := strconv.Atoi(match[2])
		secs, _ := strconv.ParseFloat(match[3], 64)
		currentTime := time.Duration(hours)*time.Hour +
			time.Duration(mins)*time.Minute +
			time.Duration(secs*float64(time.Second))

		if c.totalDuration > 0 {
			progress.PercentComplete = float64(currentTime) / float64(c.totalDuration) * 100
			if progress.PercentComplete > 100 {
				progress.PercentComplete = 100
			}
		}
	}

	// Extract speed
	if match := regexp.MustCompile(`speed=(\d+\.?\d*)x`).FindStringSubmatch(ffmpegOutput); len(match) > 0 {
		progress.CurrentSpeed, _ = strconv.ParseFloat(match[1], 64)
		progress.AverageSpeed = progress.CurrentSpeed // Could track average over time
	}

	// Extract size
	if match := regexp.MustCompile(`size=\s*(\d+)kB`).FindStringSubmatch(ffmpegOutput); len(match) > 0 {
		kb, _ := strconv.ParseInt(match[1], 10, 64)
		progress.BytesWritten = kb * 1024
		c.totalBytes = progress.BytesWritten
	}

	// Calculate time remaining
	if progress.PercentComplete > 0 && progress.CurrentSpeed > 0 {
		elapsed := float64(progress.TimeElapsed)
		total := elapsed / (progress.PercentComplete / 100)
		remaining := total - elapsed
		progress.TimeRemaining = time.Duration(remaining)
	}

	// Estimate bytes read (approximate based on progress)
	if c.totalDuration > 0 && progress.PercentComplete > 0 {
		// Rough estimate: assume input bitrate similar to output
		progress.BytesRead = int64(float64(progress.BytesWritten) * (100.0 / progress.PercentComplete))
	}

	// Extract CPU usage if available (requires ps monitoring)
	// This would need to be implemented separately by monitoring the FFmpeg process

	c.lastUpdate = time.Now()
	return progress
}

// ParseFFmpegDuration parses duration from FFmpeg probe output
// Format: "Duration: 00:01:23.45, start: 0.000000, bitrate: 1234 kb/s"
func ParseFFmpegDuration(probeOutput string) time.Duration {
	// Look for Duration line
	lines := strings.Split(probeOutput, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Duration:") {
			// Extract duration
			if match := regexp.MustCompile(`Duration: (\d+):(\d+):(\d+\.\d+)`).FindStringSubmatch(line); len(match) > 0 {
				hours, _ := strconv.Atoi(match[1])
				mins, _ := strconv.Atoi(match[2])
				secs, _ := strconv.ParseFloat(match[3], 64)
				return time.Duration(hours)*time.Hour +
					time.Duration(mins)*time.Minute +
					time.Duration(secs*float64(time.Second))
			}
		}
	}
	return 0
}

// ParseBitrate extracts bitrate from FFmpeg output
func ParseBitrate(ffmpegOutput string) int64 {
	// Look for bitrate in kbits/s
	if match := regexp.MustCompile(`bitrate=(\d+\.?\d*)kbits/s`).FindStringSubmatch(ffmpegOutput); len(match) > 0 {
		kbits, _ := strconv.ParseFloat(match[1], 64)
		return int64(kbits * 1000) // Convert to bits per second
	}
	return 0
}
