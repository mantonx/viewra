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
func newFFmpegExecutorWithConfig(config *TranscodeConfig) (*ffmpegExecutor, error) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg executable not found in system PATH: %w", err)
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
	InputPath       string
	OutputDir       string
	Profile         *QualityProfile
	ProgressHandler func(progress int)
}

// TranscodeToHLS executes FFmpeg to transcode a video file to HLS format.
// It monitors progress and calls the progress handler with percentage updates.
func (e *ffmpegExecutor) TranscodeToHLS(ctx context.Context, opts TranscodeOptions) error {
	// Ensure output directory exists
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Build FFmpeg command arguments
	args := e.buildFFmpegArgs(opts)

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
		// but we can still proceed with the transcode
		duration = 0
	}

	// Monitor progress in a goroutine
	progressDone := make(chan struct{})
	go e.monitorProgress(stderr, duration, opts.ProgressHandler, progressDone)

	// Wait for command to complete with proper cleanup on cancellation
	err = e.processManager.WaitWithCleanup(ctx, cmd)
	<-progressDone // Wait for progress monitoring to finish

	if err != nil {
		return fmt.Errorf("ffmpeg transcoding failed: %w", err)
	}

	// Verify output files were created
	playlistPath := filepath.Join(opts.OutputDir, "playlist.m3u8")
	if _, err := os.Stat(playlistPath); os.IsNotExist(err) {
		return fmt.Errorf("playlist file was not created: %s", playlistPath)
	}

	return nil
}

// RemuxToHLS remuxes a video to HLS format by copying streams without re-encoding.
// This is much faster than transcoding (2-5 minutes vs 20-60 minutes) and should be used
// when the video is already H.264 and audio is stereo, but the container format is incompatible.
func (e *ffmpegExecutor) RemuxToHLS(ctx context.Context, opts TranscodeOptions) error {
	// Ensure output directory exists
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Build FFmpeg command arguments for remuxing
	args := e.buildRemuxArgs(opts)

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
		duration = 0
	}

	// Monitor progress in a goroutine
	progressDone := make(chan struct{})
	go e.monitorProgress(stderr, duration, opts.ProgressHandler, progressDone)

	// Wait for command to complete with proper cleanup on cancellation
	err = e.processManager.WaitWithCleanup(ctx, cmd)
	<-progressDone // Wait for progress monitoring to finish

	if err != nil {
		return fmt.Errorf("ffmpeg remuxing failed: %w", err)
	}

	// Verify output files were created
	playlistPath := filepath.Join(opts.OutputDir, "playlist.m3u8")
	if _, err := os.Stat(playlistPath); os.IsNotExist(err) {
		return fmt.Errorf("playlist file was not created: %s", playlistPath)
	}

	return nil
}

// RemuxWithAudioDownmixHLS remuxes video to HLS while copying video stream and downmixing multi-channel audio to stereo.
// This is used when video is H.264 compatible but audio has too many channels (5.1, 7.1) for browser playback.
// Processing time: 5-10 minutes (faster than full transcode since video is copied).
func (e *ffmpegExecutor) RemuxWithAudioDownmixHLS(ctx context.Context, opts TranscodeOptions) error {
	// Ensure output directory exists
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Build FFmpeg command arguments for remux with audio downmix
	args := e.buildRemuxWithAudioDownmixArgs(opts)

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
		duration = 0
	}

	// Monitor progress in a goroutine
	progressDone := make(chan struct{})
	go e.monitorProgress(stderr, duration, opts.ProgressHandler, progressDone)

	// Wait for command to complete with proper cleanup on cancellation
	err = e.processManager.WaitWithCleanup(ctx, cmd)
	<-progressDone // Wait for progress monitoring to finish

	if err != nil {
		return fmt.Errorf("ffmpeg remux with audio downmix failed: %w", err)
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
	p := opts.Profile
	outputPlaylist := filepath.Join(opts.OutputDir, "playlist.m3u8")
	segmentPath := filepath.Join(opts.OutputDir, "segment_%03d.ts")

	args := []string{}

	// Add hardware acceleration flags BEFORE input
	args = append(args, e.getHardwareAccelArgs()...)

	// Input file
	args = append(args, "-i", opts.InputPath)

	// Video encoding settings (codec varies by hardware acceleration)
	videoCodec, videoPreset := e.getVideoCodecAndPreset()
	args = append(args,
		"-c:v", videoCodec,
	)

	// Add preset if using software encoding
	if videoPreset != "" {
		args = append(args, "-preset", videoPreset)
	}

	// Common video settings
	args = append(args,
		"-profile:v", "high",        // H.264 profile
		"-level", "4.1",             // H.264 level (compatible with most devices)
		"-pix_fmt", "yuv420p",       // Pixel format (widely compatible)
	)

	// Video bitrate control
	args = append(args,
		"-b:v", p.VideoBitrate,
		"-maxrate", p.VideoMaxRate,
		"-bufsize", p.VideoBufSize,
	)

	// Video resolution with pixel format conversion for 10-bit sources
	// format=yuv420p ensures 10-bit HDR sources are converted to 8-bit before NVENC encoding
	args = append(args,
		"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,format=yuv420p",
			p.Width, p.Height, p.Width, p.Height),
	)

	// GOP structure for HLS
	args = append(args,
		"-g", strconv.Itoa(p.GOPSize),           // GOP size (keyframe interval)
		"-keyint_min", strconv.Itoa(p.GOPSize),  // Minimum GOP size
		"-sc_threshold", "0",                     // Disable scene change detection
	)

	// Audio encoding settings
	args = append(args,
		"-c:a", "aac",                           // AAC audio codec
		"-b:a", p.AudioBitrate,
		"-ac", strconv.Itoa(p.AudioChannels),
		"-ar", strconv.Itoa(p.AudioSampleRate),
	)

	// HLS-specific settings
	args = append(args,
		"-f", "hls",                                      // HLS format
		"-hls_time", strconv.Itoa(p.SegmentDuration),    // Segment duration (4 seconds)
		"-hls_playlist_type", "event",                   // Event type - updates playlist as segments are created
		"-hls_segment_filename", segmentPath,            // Segment filename pattern
		"-hls_segment_type", "mpegts",                   // MPEG-TS segments
		"-hls_flags", "independent_segments",            // Each segment can be decoded independently
		"-start_number", "0",                            // Start segment numbering at 0
	)

	// Progress reporting
	args = append(args,
		"-progress", "pipe:2", // Output progress to stderr
		"-stats",              // Show encoding statistics
	)

	// Overwrite output files without asking
	args = append(args, "-y")

	// Output playlist
	args = append(args, outputPlaylist)

	return args
}

// buildRemuxArgs constructs FFmpeg arguments for remuxing to HLS (copying streams without re-encoding).
// This is used when video is already H.264 and audio is stereo, but container format needs conversion.
func (e *ffmpegExecutor) buildRemuxArgs(opts TranscodeOptions) []string {
	p := opts.Profile
	outputPlaylist := filepath.Join(opts.OutputDir, "playlist.m3u8")
	segmentPath := filepath.Join(opts.OutputDir, "segment_%03d.ts")

	args := []string{}

	// Input file
	args = append(args, "-i", opts.InputPath)

	// Copy video and audio streams without re-encoding
	args = append(args,
		"-c:v", "copy", // Copy video stream
		"-c:a", "copy", // Copy audio stream
	)

	// HLS-specific settings
	args = append(args,
		"-f", "hls",                                      // HLS format
		"-hls_time", strconv.Itoa(p.SegmentDuration),    // Segment duration (4 seconds)
		"-hls_playlist_type", "event",                   // Event type - updates playlist as segments are created
		"-hls_segment_filename", segmentPath,            // Segment filename pattern
		"-hls_segment_type", "mpegts",                   // MPEG-TS segments
		"-hls_flags", "independent_segments",            // Each segment can be decoded independently
		"-start_number", "0",                            // Start segment numbering at 0
	)

	// Progress reporting
	args = append(args,
		"-progress", "pipe:2", // Output progress to stderr
		"-stats",              // Show encoding statistics
	)

	// Overwrite output files without asking
	args = append(args, "-y")

	// Output playlist
	args = append(args, outputPlaylist)

	return args
}

// buildRemuxWithAudioDownmixArgs constructs FFmpeg arguments for remuxing with audio downmix.
// This copies the video stream but re-encodes multi-channel audio to stereo for browser compatibility.
func (e *ffmpegExecutor) buildRemuxWithAudioDownmixArgs(opts TranscodeOptions) []string {
	p := opts.Profile
	outputPlaylist := filepath.Join(opts.OutputDir, "playlist.m3u8")
	segmentPath := filepath.Join(opts.OutputDir, "segment_%03d.ts")

	args := []string{}

	// Input file
	args = append(args, "-i", opts.InputPath)

	// Copy video stream without re-encoding
	args = append(args, "-c:v", "copy")

	// Audio encoding with downmix to stereo
	// FFmpeg will automatically downmix from any multi-channel layout (TrueHD, DTS, etc.) to stereo
	args = append(args,
		"-c:a", "aac",      // AAC audio codec (web-compatible)
		"-b:a", p.AudioBitrate,
		"-ac", "2",         // Force stereo output (2 channels) - FFmpeg auto-downmixes
		"-ar", strconv.Itoa(p.AudioSampleRate),
	)

	// HLS-specific settings
	args = append(args,
		"-f", "hls",                                      // HLS format
		"-hls_time", strconv.Itoa(p.SegmentDuration),    // Segment duration (4 seconds)
		"-hls_playlist_type", "event",                   // Event type - updates playlist as segments are created
		"-hls_segment_filename", segmentPath,            // Segment filename pattern
		"-hls_segment_type", "mpegts",                   // MPEG-TS segments
		"-hls_flags", "independent_segments",            // Each segment can be decoded independently
		"-start_number", "0",                            // Start segment numbering at 0
	)

	// Progress reporting
	args = append(args,
		"-progress", "pipe:2", // Output progress to stderr
		"-stats",              // Show encoding statistics
	)

	// Overwrite output files without asking
	args = append(args, "-y")

	// Output playlist
	args = append(args, outputPlaylist)

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

// getHardwareAccelArgs returns FFmpeg arguments for hardware acceleration.
// These args must come BEFORE the input file (-i).
func (e *ffmpegExecutor) getHardwareAccelArgs() []string {
	switch e.config.HardwareAccel {
	case AccelVAAPI:
		return []string{
			"-hwaccel", "vaapi",
			"-hwaccel_device", "/dev/dri/renderD128",
			"-hwaccel_output_format", "vaapi",
		}
	case AccelNVENC:
		// For NVENC, we don't need to hardware accelerate the input decoding
		// Just use the GPU encoder. This is more reliable.
		return []string{}
	case AccelQSV:
		return []string{
			"-hwaccel", "qsv",
			"-hwaccel_output_format", "qsv",
		}
	case AccelVideoToolbox:
		return []string{
			"-hwaccel", "videotoolbox",
		}
	default:
		// No hardware acceleration
		return []string{}
	}
}

// getVideoCodecAndPreset returns the appropriate video codec and preset based on hardware acceleration.
// Returns (codec, preset). Preset is empty for hardware encoders.
func (e *ffmpegExecutor) getVideoCodecAndPreset() (string, string) {
	switch e.config.HardwareAccel {
	case AccelVAAPI:
		return "h264_vaapi", ""
	case AccelNVENC:
		return "h264_nvenc", ""
	case AccelQSV:
		return "h264_qsv", ""
	case AccelVideoToolbox:
		return "h264_videotoolbox", ""
	default:
		// Software encoding with preset
		return "libx264", "medium"
	}
}
