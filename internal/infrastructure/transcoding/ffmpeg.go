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
	ffmpegPath     string
	config         *TranscodeConfig
	processManager *ProcessManager
}

// newFFmpegExecutor creates a new FFmpeg executor.
// It verifies that FFmpeg is available in the system PATH.
func newFFmpegExecutor() (*ffmpegExecutor, error) {
	return newFFmpegExecutorWithConfig(DefaultTranscodeConfig())
}

// newFFmpegExecutorWithConfig creates a new FFmpeg executor with custom config.
// It checks for VIEWRA_FFMPEG_PATH environment variable first, then falls back to system PATH.
func newFFmpegExecutorWithConfig(config *TranscodeConfig) (*ffmpegExecutor, error) {
	// Check for custom FFmpeg path (e.g., patched viewra-ffmpeg)
	ffmpegPath := os.Getenv("VIEWRA_FFMPEG_PATH")
	if ffmpegPath == "" {
		var err error
		ffmpegPath, err = exec.LookPath("ffmpeg")
		if err != nil {
			return nil, fmt.Errorf("ffmpeg executable not found in system PATH: %w", err)
		}
	}

	if config == nil {
		config = DefaultTranscodeConfig()
	}

	return &ffmpegExecutor{
		ffmpegPath:     ffmpegPath,
		config:         config,
		processManager: NewProcessManager(config),
	}, nil
}

// TranscodeOptions contains options for the transcode operation.
type TranscodeOptions struct {
	InputPath                  string
	OutputDir                  string
	Profile                    *AdaptiveProfile
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

// TranscodeToHLS executes FFmpeg to transcode a video file to HLS format.
// It monitors progress and calls the progress handler with percentage updates.
func (e *ffmpegExecutor) TranscodeToHLS(ctx context.Context, opts TranscodeOptions) error {
	args := e.buildFFmpegArgs(opts)
	return e.executeFFmpeg(ctx, opts, args, "transcoding")
}

// RemuxToHLS remuxes a video to HLS format by copying streams without re-encoding.
// This is much faster than transcoding (2-5 minutes vs 20-60 minutes) and should be used
// when the video is already H.264 and audio is stereo, but the container format is incompatible.
func (e *ffmpegExecutor) RemuxToHLS(ctx context.Context, opts TranscodeOptions) error {
	args := e.buildRemuxArgs(opts)
	return e.executeFFmpeg(ctx, opts, args, "remuxing")
}

// RemuxWithAudioDownmixHLS remuxes video to HLS while copying video stream and downmixing multi-channel audio to stereo.
// This is used when video is H.264 compatible but audio has too many channels (5.1, 7.1) for browser playback.
// Processing time: 5-10 minutes (faster than full transcode since video is copied).
func (e *ffmpegExecutor) RemuxWithAudioDownmixHLS(ctx context.Context, opts TranscodeOptions) error {
	args := e.buildRemuxWithAudioDownmixArgs(opts)
	return e.executeFFmpeg(ctx, opts, args, "remux with audio downmix")
}

// executeFFmpeg is the unified execution method for all FFmpeg operations.
// It handles directory creation, command execution, progress monitoring, and output verification.
func (e *ffmpegExecutor) executeFFmpeg(ctx context.Context, opts TranscodeOptions, args []string, operationName string) error {
	// Ensure output directory exists
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create the command with proper process group management
	cmd := e.processManager.PrepareCommand(ctx, e.ffmpegPath, args)

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
		// but we can still proceed with the operation
		duration = 0
	}

	// Monitor progress in a goroutine
	progressDone := make(chan struct{})
	go e.monitorProgress(stderr, duration, opts.ProgressHandler, progressDone)

	// Wait for command to complete with proper cleanup on cancellation
	err = e.processManager.WaitWithCleanup(ctx, cmd)
	<-progressDone // Wait for progress monitoring to finish

	if err != nil {
		return fmt.Errorf("ffmpeg %s failed: %w", operationName, err)
	}

	// Verify output files were created
	playlistPath := filepath.Join(opts.OutputDir, "playlist.m3u8")
	if _, err := os.Stat(playlistPath); os.IsNotExist(err) {
		return fmt.Errorf("playlist file was not created: %s", playlistPath)
	}

	return nil
}

// buildFFmpegArgs constructs the FFmpeg command line arguments for HLS transcoding.
func (e *ffmpegExecutor) buildFFmpegArgs(opts TranscodeOptions) []string {
	// Get codec from profile's preferred codec (e.g., h265 for 4K profiles)
	targetCodec := CodecH264 // Default fallback
	if opts.Profile != nil && opts.Profile.PreferredCodec != "" {
		targetCodec = VideoCodec(opts.Profile.PreferredCodec)
	}

	// Set the target codec in opts so builder's getVideoCodec() returns correct codec
	opts.VideoCodec = targetCodec

	videoEncoder, videoPreset := GetVideoCodecAndPresetForCodec(e.config.HardwareAccel, targetCodec)
	builder := NewFFmpegArgsBuilder(opts).
		AddLogLevel("error").
		AddHardwareAccel(GetHardwareAccelArgsWithDevice(e.config.HardwareAccel, e.config.HardwareDevice)).
		AddFastInputOptions()

	// Add memory safety options for ALL transcoding operations to prevent OOM crashes
	// Memory-intensive operations include: PGS subtitle burn-in, HDR tone mapping (especially
	// libplacebo with peak_detect), 4K scaling, and complex filter chains
	builder.AddMemorySafetyOptions(e.config.MaxMemoryMB)

	builder.AddSeekPosition().
		AddInput().
		AddTimestampReset().
		AddStreamMapping().
		AddVideoCodec(videoEncoder, videoPreset)

	// Use hardware or software encoding based on configuration
	if e.config.HardwareAccel != AccelNone {
		builder.AddHardwareVideoEncoding(e.config.HardwareAccel)
	} else {
		builder.AddVideoEncoding()
	}

	// Note: No force_key_frames needed - GOP settings ensure frame 0 is a keyframe
	return builder.
		AddAudioEncoding().
		AddHLSOutput().
		AddProgressReporting().
		AddOverwriteOutput().
		AddOutputFile().
		Build()
}

// buildRemuxArgs constructs FFmpeg arguments for remuxing to HLS (copying streams without re-encoding).
// This is used when video is already H.264 and audio is stereo, but container format needs conversion.
// Note: force_key_frames doesn't apply to copy mode since we're not re-encoding.
func (e *ffmpegExecutor) buildRemuxArgs(opts TranscodeOptions) []string {
	return NewFFmpegArgsBuilder(opts).
		AddLogLevel("error").
		AddFastInputOptions().
		AddSeekPosition().
		AddInput().
		AddTimestampReset().
		AddStreamMapping().
		AddVideoCodec("copy", "").
		AddAudioCodec("copy").
		AddHLSOutput().
		AddProgressReporting().
		AddOverwriteOutput().
		AddOutputFile().
		Build()
}

// buildRemuxWithAudioDownmixArgs constructs FFmpeg arguments for remuxing with audio downmix.
// This copies the video stream but re-encodes multi-channel audio to stereo for browser compatibility.
// Note: force_key_frames doesn't apply to copy mode since we're not re-encoding video.
func (e *ffmpegExecutor) buildRemuxWithAudioDownmixArgs(opts TranscodeOptions) []string {
	return NewFFmpegArgsBuilder(opts).
		AddLogLevel("error").
		AddFastInputOptions().
		AddSeekPosition().
		AddInput().
		AddTimestampReset().
		AddStreamMapping().
		AddVideoCodec("copy", "").
		AddAudioDownmix().
		AddHLSOutput().
		AddProgressReporting().
		AddOverwriteOutput().
		AddOutputFile().
		Build()
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
