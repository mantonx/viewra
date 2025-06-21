package ffmpeg

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	plugins "github.com/mantonx/viewra/sdk"
)

// CommandRunner interface for command execution (enables mocking in tests)
type CommandRunner interface {
	Run(ctx context.Context, cmd string, args ...string) ([]byte, error)
}

// DefaultCommandRunner implements CommandRunner using os/exec
type DefaultCommandRunner struct{}

// Run executes a command using os/exec
func (r *DefaultCommandRunner) Run(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, cmd, args...)
	return command.CombinedOutput()
}

// Runner manages FFmpeg transcoding operations
type Runner struct {
	logger     plugins.Logger
	execer     CommandRunner
	ffmpegPath string
	sessions   map[string]*RunningSession
	mutex      sync.RWMutex
}

// RunningSession represents an active FFmpeg process
type RunningSession struct {
	ID       string
	Process  *exec.Cmd
	Cancel   context.CancelFunc
	Progress chan ProgressUpdate
	Started  time.Time
	Request  *plugins.TranscodeRequest
}

// ProgressUpdate contains progress information from FFmpeg
type ProgressUpdate struct {
	Frame    int64
	FPS      float64
	Size     string
	Time     time.Duration
	Bitrate  string
	Speed    float64
	Progress float64
	Error    string
}

// NewRunner creates a new FFmpeg runner
func NewRunner(logger plugins.Logger) *Runner {
	return NewRunnerWithExecutor(logger, &DefaultCommandRunner{})
}

// NewRunnerWithExecutor creates a new FFmpeg runner with custom command executor (for testing)
func NewRunnerWithExecutor(logger plugins.Logger, execer CommandRunner) *Runner {
	ffmpegPath := "ffmpeg"
	if customPath := os.Getenv("FFMPEG_PATH"); customPath != "" {
		ffmpegPath = customPath
	}

	return &Runner{
		logger:     logger,
		execer:     execer,
		ffmpegPath: ffmpegPath,
		sessions:   make(map[string]*RunningSession),
	}
}

// BuildFFmpegArgs constructs FFmpeg command arguments based on the transcode request
func (r *Runner) BuildFFmpegArgs(req plugins.TranscodeRequest) []string {
	var args []string

	// Add hardware acceleration BEFORE input file - default to "auto" for optimal performance
	hwaccelSet := false
	if req.CodecOpts != nil && len(req.CodecOpts.Extra) > 0 {
		// Check if hwaccel is specified in extra args
		for i, arg := range req.CodecOpts.Extra {
			if arg == "-hwaccel" && i+1 < len(req.CodecOpts.Extra) {
				args = append(args, "-hwaccel", req.CodecOpts.Extra[i+1])
				hwaccelSet = true
				break
			}
		}
	}

	// If no hardware acceleration specified, default to "auto"
	if !hwaccelSet {
		args = append(args, "-hwaccel", "auto")
	}

	// NOW add input file after hwaccel
	args = append(args, "-i", req.InputPath)

	// Video codec handling with auto hardware acceleration
	if req.CodecOpts != nil && req.CodecOpts.Video != "" {
		videoEncoder := r.getVideoEncoder(req.CodecOpts.Video)
		args = append(args, "-c:v", videoEncoder)
	} else {
		// Default to h264 with auto hardware acceleration
		args = append(args, "-c:v", "h264")
	}

	// Video quality settings
	if req.CodecOpts != nil && req.CodecOpts.Quality > 0 {
		args = append(args, "-crf", fmt.Sprintf("%d", req.CodecOpts.Quality))
	}

	// Video bitrate (if specified instead of CRF)
	if req.CodecOpts != nil && req.CodecOpts.Bitrate != "" && req.CodecOpts.Quality == 0 {
		args = append(args, "-b:v", req.CodecOpts.Bitrate)
	}

	// Video preset
	if req.CodecOpts != nil && req.CodecOpts.Preset != "" {
		args = append(args, "-preset", req.CodecOpts.Preset)
	}

	// Audio codec handling
	if req.CodecOpts != nil && req.CodecOpts.Audio != "" {
		audioEncoder := getAudioEncoder(req.CodecOpts.Audio)
		args = append(args, "-c:a", audioEncoder)

		// Force stereo downmix for better compatibility with surround sound sources
		// This fixes issues with EAC3 5.1/7.1 sources that don't play properly
		if audioEncoder == "aac" {
			args = append(args,
				"-ac", "2", // Force stereo output
				"-af", "aformat=channel_layouts=stereo", // Ensure stereo channel layout
			)
		}
	}

	// Audio bitrate
	if req.CodecOpts != nil && req.CodecOpts.Bitrate != "" {
		// For audio bitrate, extract or default
		audioBitrate := "128k"
		if req.Environment != nil {
			if ab := req.Environment["audio_bitrate"]; ab != "" {
				audioBitrate = ab
			}
		}
		args = append(args, "-b:a", audioBitrate)
	}

	// Container-specific settings
	if req.CodecOpts != nil {
		switch req.CodecOpts.Container {
		case "dash":
			args = append(args, "-f", "dash")
			args = append(args, "-seg_duration", "4")
			args = append(args, "-use_template", "1")
			args = append(args, "-use_timeline", "1")
			args = append(args, "-map", "0:v:0?", "-map", "0:a:0?")
		case "hls":
			args = append(args, "-f", "hls")
			args = append(args, "-hls_time", "4")
			args = append(args, "-hls_playlist_type", "vod")
			args = append(args, "-map", "0:v:0?", "-map", "0:a:0?")
		case "mp4":
			args = append(args, "-f", "mp4")
			args = append(args, "-movflags", "+faststart")
		default:
			if req.CodecOpts.Container != "" {
				args = append(args, "-f", req.CodecOpts.Container)
			}
		}
	}

	// Add any extra arguments specified
	if req.CodecOpts != nil && len(req.CodecOpts.Extra) > 0 {
		// Filter out already-processed arguments like -hwaccel
		for i := 0; i < len(req.CodecOpts.Extra); i++ {
			arg := req.CodecOpts.Extra[i]
			if arg == "-hwaccel" {
				i++ // Skip the value too
				continue
			}
			if arg == "-crf" || arg == "-preset" {
				i++ // Skip values we've already handled
				continue
			}
			args = append(args, arg)
		}
	}

	// Output file (always comes last)
	args = append(args, "-y", req.OutputPath)

	return args
}

// getVideoEncoder returns the appropriate video encoder with hardware acceleration support
func (r *Runner) getVideoEncoder(codec string) string {
	switch codec {
	case "auto", "h264":
		// Try hardware encoders first, fallback to software
		return r.selectBestH264Encoder()
	case "hevc", "h265":
		return r.selectBestHEVCEncoder()
	case "vp8":
		return "libvpx"
	case "vp9":
		return "libvpx-vp9"
	case "av1":
		return r.selectBestAV1Encoder()
	default:
		// If it's already a specific encoder name, use it
		return codec
	}
}

// selectBestH264Encoder chooses the best available H.264 encoder
func (r *Runner) selectBestH264Encoder() string {
	// Temporarily prioritize software encoding due to VAAPI issues in Docker
	encoders := []string{
		"libx264",           // Software (prioritized for stability)
		"h264_nvenc",        // NVIDIA hardware
		"h264_vaapi",        // Intel/AMD hardware on Linux (problematic in Docker)
		"h264_qsv",          // Intel Quick Sync
		"h264_videotoolbox", // macOS hardware
	}

	return r.selectFirstAvailableEncoder(encoders)
}

// selectBestHEVCEncoder chooses the best available HEVC encoder
func (r *Runner) selectBestHEVCEncoder() string {
	encoders := []string{
		"hevc_nvenc",
		"hevc_vaapi",
		"hevc_qsv",
		"hevc_videotoolbox",
		"libx265", // Software fallback
	}

	return r.selectFirstAvailableEncoder(encoders)
}

// selectBestAV1Encoder chooses the best available AV1 encoder
func (r *Runner) selectBestAV1Encoder() string {
	encoders := []string{
		"av1_nvenc",  // NVIDIA hardware (newer GPUs)
		"libaom-av1", // Software
	}

	return r.selectFirstAvailableEncoder(encoders)
}

// selectFirstAvailableEncoder tests encoders and returns the first available one
func (r *Runner) selectFirstAvailableEncoder(encoders []string) string {
	for _, encoder := range encoders {
		if r.isEncoderAvailable(encoder) {
			if r.logger != nil {
				r.logger.Info("selected encoder", "encoder", encoder)
			}
			return encoder
		}
	}

	// Fallback to the last one (should be software)
	fallback := encoders[len(encoders)-1]
	if r.logger != nil {
		r.logger.Warn("using fallback encoder", "encoder", fallback)
	}
	return fallback
}

// isEncoderAvailable checks if an encoder is available in the current FFmpeg build
func (r *Runner) isEncoderAvailable(encoder string) bool {
	// Create a simple test command to check encoder availability
	cmd := exec.Command("ffmpeg", "-hide_banner", "-encoders")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Check if the encoder is listed in the output
	return strings.Contains(string(output), encoder)
}

// RunFFmpeg executes FFmpeg with the given arguments, with automatic software fallback
func (r *Runner) RunFFmpeg(ctx context.Context, args []string) error {
	if r.logger != nil {
		r.logger.Info("executing FFmpeg command", "command", r.ffmpegPath, "args", strings.Join(args, " "))
	}

	// Try the original command first
	err := r.runFFmpegCommand(ctx, args)

	// If it failed and we were using hardware acceleration, try software fallback
	if err != nil && r.isHardwareAccelError(err) {
		if r.logger != nil {
			r.logger.Warn("hardware acceleration failed, falling back to software", "error", err)
		}

		// Replace hardware accelerated encoders with software ones
		fallbackArgs := r.convertToSoftwareFallback(args)
		if r.logger != nil {
			r.logger.Info("retrying with software fallback", "args", strings.Join(fallbackArgs, " "))
		}

		// Try again with software encoding
		return r.runFFmpegCommand(ctx, fallbackArgs)
	}

	return err
}

// runFFmpegCommand executes a single FFmpeg command
func (r *Runner) runFFmpegCommand(ctx context.Context, args []string) error {
	// Create context with timeout
	cmd := exec.CommandContext(ctx, r.ffmpegPath, args...)

	// Set up pipes for progress monitoring
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	// Monitor progress in a goroutine
	go r.monitorProgress(stderr)

	// Wait for completion
	if err := cmd.Wait(); err != nil {
		// Check if it was canceled due to context
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("FFmpeg process failed: %w", err)
	}

	return nil
}

// isHardwareAccelError checks if the error is related to hardware acceleration failure
func (r *Runner) isHardwareAccelError(err error) bool {
	errorStr := strings.ToLower(err.Error())

	// Common hardware acceleration error patterns
	hardwareErrors := []string{
		"function not implemented",
		"no device available",
		"failed to initialize",
		"not supported",
		"device creation failed",
		"vaapi",
		"nvenc",
		"qsv",
		"videotoolbox",
	}

	for _, pattern := range hardwareErrors {
		if strings.Contains(errorStr, pattern) {
			return true
		}
	}

	return false
}

// convertToSoftwareFallback converts hardware-accelerated arguments to software equivalents
func (r *Runner) convertToSoftwareFallback(args []string) []string {
	fallbackArgs := make([]string, len(args))
	copy(fallbackArgs, args)

	// Replace hardware video encoders with software equivalents
	for i, arg := range fallbackArgs {
		switch arg {
		case "h264_nvenc", "h264_vaapi", "h264_qsv", "h264_videotoolbox":
			fallbackArgs[i] = "libx264"
			if r.logger != nil {
				r.logger.Info("replaced hardware encoder with software", "from", arg, "to", "libx264")
			}
		case "hevc_nvenc", "hevc_vaapi", "hevc_qsv", "hevc_videotoolbox":
			fallbackArgs[i] = "libx265"
			if r.logger != nil {
				r.logger.Info("replaced hardware encoder with software", "from", arg, "to", "libx265")
			}
		case "av1_nvenc":
			fallbackArgs[i] = "libaom-av1"
			if r.logger != nil {
				r.logger.Info("replaced hardware encoder with software", "from", arg, "to", "libaom-av1")
			}
		}
	}

	// Remove hardware acceleration flags that might cause issues
	var cleanedArgs []string
	for i := 0; i < len(fallbackArgs); i++ {
		if fallbackArgs[i] == "-hwaccel" {
			// Skip -hwaccel and its value
			i++ // Skip the next argument too (the hwaccel method)
			continue
		}
		cleanedArgs = append(cleanedArgs, fallbackArgs[i])
	}

	return cleanedArgs
}

// getAudioEncoder maps generic codec names to specific encoders
func getAudioEncoder(codec string) string {
	switch codec {
	case "aac":
		return "aac"
	case "mp3":
		return "libmp3lame"
	case "opus":
		return "libopus"
	case "vorbis":
		return "libvorbis"
	case "ac3":
		return "ac3"
	default:
		return codec // Use as-is if not recognized
	}
}

// StopSession stops a running transcoding session
func (r *Runner) StopSession(sessionID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	session, exists := r.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	// Cancel the context
	if session.Cancel != nil {
		session.Cancel()
	}

	// Kill the process if it's still running
	if session.Process != nil && session.Process.Process != nil {
		if err := session.Process.Process.Kill(); err != nil {
			r.logger.Warn("failed to kill FFmpeg process", "session_id", sessionID, "error", err)
		}
	}

	// Close progress channel
	if session.Progress != nil {
		close(session.Progress)
	}

	// Remove from sessions map
	delete(r.sessions, sessionID)

	if r.logger != nil {
		r.logger.Info("stopped transcoding session", "session_id", sessionID)
	}

	return nil
}

// GetSession retrieves a running session by ID
func (r *Runner) GetSession(sessionID string) (*RunningSession, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	session, exists := r.sessions[sessionID]
	return session, exists
}

// ListActiveSessions returns all currently active sessions
func (r *Runner) ListActiveSessions() []*RunningSession {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	sessions := make([]*RunningSession, 0, len(r.sessions))
	for _, session := range r.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// monitorProgress parses FFmpeg stderr output for progress information
func (r *Runner) monitorProgress(stderr io.ReadCloser) {
	defer stderr.Close()

	scanner := bufio.NewScanner(stderr)
	frameRegex := regexp.MustCompile(`frame=\s*(\d+)`)
	fpsRegex := regexp.MustCompile(`fps=\s*([\d.]+)`)
	sizeRegex := regexp.MustCompile(`size=\s*(\w+)`)
	timeRegex := regexp.MustCompile(`time=(\d{2}):(\d{2}):(\d{2}\.\d{2})`)
	bitrateRegex := regexp.MustCompile(`bitrate=\s*([\d.]+\w*\/s)`)
	speedRegex := regexp.MustCompile(`speed=\s*([\d.]+)x`)

	for scanner.Scan() {
		line := scanner.Text()

		update := ProgressUpdate{}

		if matches := frameRegex.FindStringSubmatch(line); matches != nil {
			if frame, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
				update.Frame = frame
			}
		}

		if matches := fpsRegex.FindStringSubmatch(line); matches != nil {
			if fps, err := strconv.ParseFloat(matches[1], 64); err == nil {
				update.FPS = fps
			}
		}

		if matches := sizeRegex.FindStringSubmatch(line); matches != nil {
			update.Size = matches[1]
		}

		if matches := timeRegex.FindStringSubmatch(line); matches != nil {
			hours, _ := strconv.Atoi(matches[1])
			minutes, _ := strconv.Atoi(matches[2])
			seconds, _ := strconv.ParseFloat(matches[3], 64)
			update.Time = time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds*float64(time.Second))
		}

		if matches := bitrateRegex.FindStringSubmatch(line); matches != nil {
			update.Bitrate = matches[1]
		}

		if matches := speedRegex.FindStringSubmatch(line); matches != nil {
			if speed, err := strconv.ParseFloat(matches[1], 64); err == nil {
				update.Speed = speed
			}
		}

		// Log progress if logger is available
		if r.logger != nil && update.Frame > 0 {
			r.logger.Debug("transcoding progress", "frame", update.Frame, "fps", update.FPS, "time", update.Time, "speed", update.Speed)
		}
	}

	if err := scanner.Err(); err != nil && r.logger != nil {
		r.logger.Error("error reading FFmpeg output", "error", err)
	}
}
