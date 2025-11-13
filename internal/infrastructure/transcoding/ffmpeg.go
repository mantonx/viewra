package transcoding

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ffmpegExecutor handles FFmpeg command execution with progress tracking.
type ffmpegExecutor struct {
	ffmpegPath string
}

// newFFmpegExecutor creates a new FFmpeg executor.
// It verifies that FFmpeg is available in the system PATH.
func newFFmpegExecutor() (*ffmpegExecutor, error) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg executable not found in system PATH: %w", err)
	}

	return &ffmpegExecutor{
		ffmpegPath: ffmpegPath,
	}, nil
}

// TranscodeOptions contains options for the transcode operation.
type TranscodeOptions struct {
	InputPath       string
	OutputDir       string
	Profile         *QualityProfile
	ProgressHandler func(progress int)
}

// Transcode executes FFmpeg to transcode a video file to DASH format.
// It monitors progress and calls the progress handler with percentage updates.
func (e *ffmpegExecutor) Transcode(ctx context.Context, opts TranscodeOptions) error {
	// Ensure output directory exists
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Build FFmpeg command arguments
	args := e.buildFFmpegArgs(opts)

	// Create the command
	cmd := exec.CommandContext(ctx, e.ffmpegPath, args...)

	// Get stderr pipe for progress monitoring
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Get video duration for progress calculation
	duration, err := e.getVideoDuration(opts.InputPath)
	if err != nil {
		// If we can't get duration, we can't track progress accurately
		// but we can still proceed with the transcode
		duration = 0
	}

	// Monitor progress in a goroutine
	progressDone := make(chan struct{})
	go e.monitorProgress(stderr, duration, opts.ProgressHandler, progressDone)

	// Wait for command to complete
	err = cmd.Wait()
	<-progressDone // Wait for progress monitoring to finish

	if err != nil {
		// Check if context was cancelled
		if ctx.Err() != nil {
			return fmt.Errorf("transcode cancelled: %w", ctx.Err())
		}
		return fmt.Errorf("ffmpeg transcoding failed: %w", err)
	}

	// Verify output files were created
	manifestPath := filepath.Join(opts.OutputDir, "manifest.mpd")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return fmt.Errorf("manifest file was not created: %s", manifestPath)
	}

	return nil
}

// buildFFmpegArgs constructs the FFmpeg command line arguments for DASH transcoding.
func (e *ffmpegExecutor) buildFFmpegArgs(opts TranscodeOptions) []string {
	p := opts.Profile
	outputManifest := filepath.Join(opts.OutputDir, "manifest.mpd")

	args := []string{
		// Input file
		"-i", opts.InputPath,

		// Video encoding settings
		"-c:v", "libx264",           // H.264 codec
		"-preset", "medium",         // Encoding speed/quality tradeoff
		"-profile:v", "high",        // H.264 profile
		"-level", "4.1",             // H.264 level (compatible with most devices)
		"-pix_fmt", "yuv420p",       // Pixel format (widely compatible)

		// Video bitrate control
		"-b:v", p.VideoBitrate,
		"-maxrate", p.VideoMaxRate,
		"-bufsize", p.VideoBufSize,

		// Video resolution
		"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2",
			p.Width, p.Height, p.Width, p.Height),

		// GOP structure for DASH
		"-g", strconv.Itoa(p.GOPSize),           // GOP size (keyframe interval)
		"-keyint_min", strconv.Itoa(p.GOPSize),  // Minimum GOP size
		"-sc_threshold", "0",                     // Disable scene change detection

		// Audio encoding settings
		"-c:a", "aac",                           // AAC audio codec
		"-b:a", p.AudioBitrate,
		"-ac", strconv.Itoa(p.AudioChannels),
		"-ar", strconv.Itoa(p.AudioSampleRate),

		// DASH-specific settings
		"-f", "dash",                                    // DASH format
		"-seg_duration", strconv.Itoa(p.SegmentDuration), // Segment duration
		"-use_template", "1",                            // Use template-based segments
		"-use_timeline", "1",                            // Use timeline in manifest
		"-init_seg_name", "init_$RepresentationID$.m4s", // Init segment naming
		"-media_seg_name", "segment_$RepresentationID$_$Number$.m4s", // Media segment naming
		"-adaptation_sets", "id=0,streams=v id=1,streams=a", // Separate video and audio adaptation sets

		// Progress reporting
		"-progress", "pipe:2", // Output progress to stderr
		"-stats",              // Show encoding statistics

		// Overwrite output files without asking
		"-y",

		// Output manifest
		outputManifest,
	}

	return args
}

// getVideoDuration extracts the duration of the video file using ffprobe.
// Returns 0 if duration cannot be determined.
func (e *ffmpegExecutor) getVideoDuration(inputPath string) (time.Duration, error) {
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		return 0, fmt.Errorf("ffprobe not found: %w", err)
	}

	cmd := exec.Command(ffprobePath,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inputPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	durationStr := strings.TrimSpace(string(output))
	durationSec, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	return time.Duration(durationSec * float64(time.Second)), nil
}

// monitorProgress monitors FFmpeg's stderr output and extracts progress information.
// It parses the time= field from FFmpeg's progress output and calculates percentage.
func (e *ffmpegExecutor) monitorProgress(stderr io.Reader, totalDuration time.Duration, handler func(int), done chan<- struct{}) {
	defer close(done)

	if handler == nil {
		// No handler provided, just drain stderr
		io.Copy(io.Discard, stderr)
		return
	}

	scanner := bufio.NewScanner(stderr)
	timeRegex := regexp.MustCompile(`time=(\d+):(\d+):(\d+\.\d+)`)

	lastProgress := -1

	for scanner.Scan() {
		line := scanner.Text()

		// Look for time= in the progress output
		matches := timeRegex.FindStringSubmatch(line)
		if len(matches) != 4 {
			continue
		}

		// Parse time components
		hours, _ := strconv.Atoi(matches[1])
		minutes, _ := strconv.Atoi(matches[2])
		seconds, _ := strconv.ParseFloat(matches[3], 64)

		// Calculate current position
		currentTime := time.Duration(hours)*time.Hour +
			time.Duration(minutes)*time.Minute +
			time.Duration(seconds*float64(time.Second))

		// Calculate progress percentage
		var progress int
		if totalDuration > 0 {
			progress = int((float64(currentTime) / float64(totalDuration)) * 100)
			if progress > 100 {
				progress = 100
			}
		} else {
			// If we don't know the duration, we can't calculate accurate progress
			// Just report that we're working (at 50%)
			progress = 50
		}

		// Only report if progress changed
		if progress != lastProgress {
			handler(progress)
			lastProgress = progress
		}
	}

	// Ensure we report 100% on successful completion
	if lastProgress < 100 && lastProgress >= 0 {
		handler(100)
	}
}

// ValidateInputFile checks if the input file exists and is accessible.
func ValidateInputFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("input file does not exist: %s", path)
		}
		return fmt.Errorf("cannot access input file: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("input path is a directory, not a file: %s", path)
	}

	if info.Size() == 0 {
		return fmt.Errorf("input file is empty: %s", path)
	}

	return nil
}

// CleanupOutputDir removes all files in the output directory.
// Used for cleaning up after failed transcode attempts.
func CleanupOutputDir(outputDir string) error {
	// Check if directory exists
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		return nil // Nothing to clean up
	}

	// Remove the entire directory and its contents
	return os.RemoveAll(outputDir)
}
