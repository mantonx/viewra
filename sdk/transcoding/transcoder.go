package transcoding

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

// Transcoder provides a direct, working FFmpeg transcoder
// based on the old working implementation without unnecessary abstractions
type Transcoder struct {
	logger Logger
	
	// Configuration
	name        string
	description string
	version     string
	author      string
	priority    int
	
	// Session management
	sessions map[string]*Session
	mutex    sync.RWMutex
}

// Session represents an active transcoding session  
type Session struct {
	ID        string
	Handle    *TranscodeHandle
	Process   *exec.Cmd
	StartTime time.Time
	Request   TranscodeRequest
	Cancel    context.CancelFunc
	Progress  float64
}

// NewTranscoder creates a new direct transcoder  
func NewTranscoder(name, description, version, author string, priority int) *Transcoder {
	return &Transcoder{
		name:        name,
		description: description,
		version:     version,
		author:      author,
		priority:    priority,
		sessions:    make(map[string]*Session),
	}
}

// SetLogger sets the logger for this transcoder
func (t *Transcoder) SetLogger(logger Logger) {
	t.logger = logger
}

// GetInfo returns provider information
func (t *Transcoder) GetInfo() ProviderInfo {
	return ProviderInfo{
		ID:          t.name,
		Name:        t.name,
		Description: t.description,
		Version:     t.version,
		Author:      t.author,
		Priority:    t.priority,
	}
}

// GetSupportedFormats returns supported container formats
func (t *Transcoder) GetSupportedFormats() []ContainerFormat {
	return []ContainerFormat{
		{
			Format:      "mp4",
			MimeType:    "video/mp4",
			Extensions:  []string{".mp4"},
			Description: "MPEG-4 Container",
			Adaptive:    false,
		},
		{
			Format:      "dash",
			MimeType:    "application/dash+xml",
			Extensions:  []string{".mpd", ".m4s"},
			Description: "MPEG-DASH Adaptive Streaming",
			Adaptive:    true,
		},
		{
			Format:      "hls",
			MimeType:    "application/vnd.apple.mpegurl",
			Extensions:  []string{".m3u8", ".ts"},
			Description: "HLS Adaptive Streaming",
			Adaptive:    true,
		},
	}
}

// StartTranscode starts a new transcoding session
func (t *Transcoder) StartTranscode(ctx context.Context, req TranscodeRequest) (*TranscodeHandle, error) {
	// Generate session ID if not provided
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	// Use provided output directory or create default
	var outputDir string
	if req.OutputPath != "" {
		outputDir = req.OutputPath
	} else {
		baseDir := "/app/viewra-data/transcoding"
		if envDir := os.Getenv("VIEWRA_TRANSCODING_DIR"); envDir != "" {
			baseDir = envDir
		}
		outputDir = fmt.Sprintf("%s/%s_%s_%s", baseDir, req.Container, t.name, sessionID)
	}
	
	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Determine output file based on container
	var outputPath string
	switch req.Container {
	case "dash":
		outputPath = filepath.Join(outputDir, "manifest.mpd")
	case "hls":
		outputPath = filepath.Join(outputDir, "playlist.m3u8")
	default:
		outputPath = filepath.Join(outputDir, fmt.Sprintf("output.%s", req.Container))
	}

	// Create context with cancel for this session
	sessionCtx, cancelFunc := context.WithCancel(ctx)

	// Build FFmpeg arguments using simple approach
	args := t.buildSimpleFFmpegArgs(req, outputPath)

	// Create the command for background execution
	// Note: Don't use sessionCtx for the command to avoid premature cancellation
	cmd := exec.Command("ffmpeg", args...)
	
	// Set up process for proper background execution
	// Note: Setsid causes "operation not permitted" in containers
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,    // Create new process group (this is sufficient for background execution)
	}
	
	// Set working directory to output directory
	cmd.Dir = outputDir
	
	// Create log files for FFmpeg output
	var stdoutLogPath, stderrLogPath string
	stdoutLogPath = filepath.Join(outputDir, "ffmpeg-stdout.log")
	stderrLogPath = filepath.Join(outputDir, "ffmpeg-stderr.log")
	
	stdoutFile, err := os.Create(stdoutLogPath)
	if err != nil {
		if t.logger != nil {
			t.logger.Warn("failed to create stdout log file", "error", err)
		}
	} else {
		cmd.Stdout = stdoutFile
	}
	
	stderrFile, err := os.Create(stderrLogPath)
	if err != nil {
		if t.logger != nil {
			t.logger.Warn("failed to create stderr log file", "error", err)
		}
	} else {
		cmd.Stderr = stderrFile
	}

	if t.logger != nil {
		t.logger.Info("starting simple FFmpeg transcoding",
			"session_id", sessionID,
			"command", fmt.Sprintf("ffmpeg %v", args),
		)
	}

	// Create handle
	handle := &TranscodeHandle{
		SessionID:   sessionID,
		Provider:    t.name,
		StartTime:   time.Now(),
		Directory:   outputDir,
		Context:     sessionCtx,
		CancelFunc:  cancelFunc,
		PrivateData: sessionID,
	}

	// Create session
	session := &Session{
		ID:        sessionID,
		Handle:    handle,
		Process:   cmd,
		StartTime: time.Now(),
		Request:   req,
		Cancel:    cancelFunc,
		Progress:  0,
	}

	// Store session
	t.mutex.Lock()
	t.sessions[sessionID] = session
	t.mutex.Unlock()

	// Start FFmpeg process in background
	if err := cmd.Start(); err != nil {
		cancelFunc()
		t.mutex.Lock()
		delete(t.sessions, sessionID)
		t.mutex.Unlock()
		if t.logger != nil {
			t.logger.Error("failed to start FFmpeg process",
				"session_id", sessionID,
				"error", err,
			)
		}
		return nil, fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	// Log process info
	if t.logger != nil {
		t.logger.Info("FFmpeg process started in background",
			"session_id", sessionID,
			"pid", cmd.Process.Pid,
			"output_dir", outputDir,
			"stdout_log", stdoutLogPath,
			"stderr_log", stderrLogPath,
		)
	}

	// Monitor the process in a goroutine
	go t.monitorSession(session)

	return handle, nil
}

// GetProgress returns transcoding progress
func (t *Transcoder) GetProgress(handle *TranscodeHandle) (*TranscodingProgress, error) {
	if handle == nil {
		return nil, fmt.Errorf("invalid handle")
	}

	t.mutex.RLock()
	session, exists := t.sessions[handle.SessionID]
	t.mutex.RUnlock()

	if !exists {
		// Session not found - it may have failed or been terminated
		return nil, fmt.Errorf("session not found: %s", handle.SessionID)
	}

	// For now, return basic progress based on time and stored progress
	elapsed := time.Since(session.StartTime)
	progress := &TranscodingProgress{
		PercentComplete: session.Progress,
		TimeElapsed:     elapsed,
		CurrentSpeed:    1.0,
		AverageSpeed:    1.0,
	}

	return progress, nil
}

// StopTranscode stops a transcoding session with improved cleanup
func (t *Transcoder) StopTranscode(handle *TranscodeHandle) error {
	if handle == nil {
		return fmt.Errorf("invalid handle")
	}

	t.mutex.Lock()
	session, exists := t.sessions[handle.SessionID]
	if exists {
		delete(t.sessions, handle.SessionID)
	}
	t.mutex.Unlock()

	if !exists {
		return fmt.Errorf("session not found: %s", handle.SessionID)
	}

	if t.logger != nil {
		t.logger.Info("stopping transcoding session", "session_id", handle.SessionID)
	}

	// Cancel the context first
	session.Cancel()

	// Improved process termination with timeout
	if session.Process != nil && session.Process.Process != nil {
		pid := session.Process.Process.Pid
		
		// Try graceful termination first
		if err := t.terminateProcessGroup(pid); err != nil {
			if t.logger != nil {
				t.logger.Warn("graceful termination failed, forcing kill", "pid", pid, "error", err)
			}
			// Force kill if graceful termination fails
			t.forceKillProcess(pid)
		}
		
		// Wait for process to actually terminate (with timeout)
		go func() {
			timeout := time.NewTimer(10 * time.Second)
			defer timeout.Stop()
			
			done := make(chan error, 1)
			go func() {
				done <- session.Process.Wait()
			}()
			
			select {
			case <-done:
				if t.logger != nil {
					t.logger.Info("FFmpeg process terminated", "session_id", handle.SessionID, "pid", pid)
				}
			case <-timeout.C:
				if t.logger != nil {
					t.logger.Warn("FFmpeg process did not terminate within timeout", "session_id", handle.SessionID, "pid", pid)
				}
			}
		}()
	}

	return nil
}

// terminateProcessGroup gracefully terminates a process group
func (t *Transcoder) terminateProcessGroup(pid int) error {
	// Get process group ID
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return fmt.Errorf("failed to get process group: %w", err)
	}
	
	// Send SIGTERM to the entire process group
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM to process group: %w", err)
	}
	
	// Give it 5 seconds to terminate gracefully
	time.Sleep(5 * time.Second)
	
	// Check if the main process is still running
	if err := syscall.Kill(pid, 0); err == nil {
		// Process is still running, need to force kill
		return fmt.Errorf("process %d still running after SIGTERM", pid)
	}
	
	return nil
}

// forceKillProcess forcefully kills a process and its group
func (t *Transcoder) forceKillProcess(pid int) {
	if t.logger != nil {
		t.logger.Warn("force killing FFmpeg process", "pid", pid)
	}
	
	// Try to kill the process group first
	if pgid, err := syscall.Getpgid(pid); err == nil {
		syscall.Kill(-pgid, syscall.SIGKILL)
	}
	
	// Kill the main process
	syscall.Kill(pid, syscall.SIGKILL)
}

// buildSimpleFFmpegArgs builds optimized FFmpeg arguments for high-quality transcoding
func (t *Transcoder) buildSimpleFFmpegArgs(req TranscodeRequest, outputPath string) []string {
	var args []string

	// Always overwrite output files
	args = append(args, "-y")

	// Disable hardware acceleration for stability
	// Hardware acceleration can cause timing issues
	// args = append(args, "-hwaccel", "auto")
	
	// Seek to position if specified (input seeking for efficiency)
	if req.Seek > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", req.Seek.Seconds()))
	}

	// Input file
	args = append(args, "-i", req.InputPath)

	// Advanced video mapping and filtering
	args = append(args, "-map", "0:v:0") // Map first video stream
	args = append(args, "-map", "0:a:0") // Map first audio stream

	// Video codec with intelligent defaults
	videoCodec := t.getOptimalVideoCodec(req)
	args = append(args, "-c:v", videoCodec)

	// Preset optimization based on speed priority
	preset := t.getOptimalPreset(req.SpeedPriority, videoCodec)
	if preset != "" {
		args = append(args, "-preset", preset)
	}

	// Quality settings optimized for content
	qualityArgs := t.getOptimalQualitySettings(req, videoCodec)
	args = append(args, qualityArgs...)

	// Keyframe alignment for optimal seeking and segment boundaries
	keyframeArgs := t.getKeyframeAlignmentArgs(req)
	args = append(args, keyframeArgs...)

	// Video filtering for quality enhancement
	videoFilters := t.getVideoFilters(req)
	if len(videoFilters) > 0 {
		args = append(args, "-vf", videoFilters)
	}

	// Audio settings optimized for source content
	audioArgs := t.getOptimalAudioSettings(req)
	args = append(args, audioArgs...)

	// Conservative threading to prevent resource contention
	args = append(args, "-threads", "4") // Limit to 4 threads for stability

	// Container-specific settings with quality optimizations
	containerArgs := t.getContainerSpecificArgs(req, outputPath)
	args = append(args, containerArgs...)

	// Output file
	args = append(args, outputPath)

	return args
}

// getOptimalVideoCodec selects the best video codec based on request and available hardware
func (t *Transcoder) getOptimalVideoCodec(req TranscodeRequest) string {
	if req.VideoCodec != "" {
		return req.VideoCodec
	}
	
	// Default to H.264 for compatibility, hardware acceleration will auto-detect
	return "libx264"
}

// getOptimalPreset selects the best encoding preset for quality/speed balance
func (t *Transcoder) getOptimalPreset(speedPriority SpeedPriority, codec string) string {
	switch speedPriority {
	case SpeedPriorityFastest:
		return "fast"       // Balanced speed for responsiveness
	case SpeedPriorityQuality:
		return "slower"     // High quality but still reasonable speed
	default:
		return "medium"     // Better balance of speed vs quality
	}
}

// getOptimalQualitySettings returns quality parameters optimized for content
func (t *Transcoder) getOptimalQualitySettings(req TranscodeRequest, codec string) []string {
	var args []string
	
	// CRF calculation with improved mapping for better quality
	// Map 0-100 quality to CRF 28-16 for better visual quality
	crf := 28 - (req.Quality * 12 / 100)
	if crf < 16 {
		crf = 16 // Maximum quality
	}
	if crf > 28 {
		crf = 28 // Minimum quality for streaming
	}
	
	args = append(args, "-crf", strconv.Itoa(crf))
	
	// Additional quality settings for H.264
	if codec == "libx264" {
		// Use baseline profile for low bitrate stream, high for others
		if req.Quality < 30 {
			args = append(args, "-profile:v", "baseline")
			args = append(args, "-level", "3.0")
		} else {
			args = append(args, "-profile:v", "high")
			args = append(args, "-level", "4.1")
		}
		// Conservative x264 params for stability
		args = append(args, "-x264-params", "ref=2:bframes=2:me=hex:subme=6:rc-lookahead=40")
	} else if codec == "libx265" {
		args = append(args, "-preset", "medium")
		args = append(args, "-x265-params", "keyint=48:min-keyint=24:no-open-gop=1")
	} else if codec == "libvpx-vp9" {
		args = append(args, "-b:v", "0") // Use constant quality mode
		args = append(args, "-deadline", "good")
		args = append(args, "-cpu-used", "2")
		args = append(args, "-row-mt", "1")
		args = append(args, "-tile-columns", "2")
		args = append(args, "-tile-rows", "1")
		args = append(args, "-g", "48") // Keyframe interval
	}
	
	return args
}

// getVideoFilters returns video filters for quality enhancement
func (t *Transcoder) getVideoFilters(req TranscodeRequest) string {
	var filters []string
	
	// Resolution scaling if specified
	if req.Resolution != nil && req.Resolution.Width > 0 && req.Resolution.Height > 0 {
		// Use lanczos for high quality downscaling
		scaleFilter := fmt.Sprintf("scale=%d:%d:flags=lanczos", req.Resolution.Width, req.Resolution.Height)
		filters = append(filters, scaleFilter)
	}
	
	// Deinterlacing if needed (detect interlaced content)
	filters = append(filters, "yadif=mode=send_field:deint=interlaced")
	
	// Pixel format conversion for compatibility
	filters = append(filters, "format=yuv420p")
	
	if len(filters) > 0 {
		return strings.Join(filters, ",")
	}
	
	return ""
}

// getOptimalAudioSettings returns optimized audio encoding settings
func (t *Transcoder) getOptimalAudioSettings(req TranscodeRequest) []string {
	var args []string
	
	audioCodec := req.AudioCodec
	if audioCodec == "" {
		audioCodec = "aac"
	}
	args = append(args, "-c:a", audioCodec)
	
	// Enhanced audio settings to prevent pops and artifacts
	if audioCodec == "aac" {
		// High quality bitrate for clean audio
		args = append(args, "-b:a", "192k")      // Increased from 128k for better quality
		args = append(args, "-profile:a", "aac_low")
		args = append(args, "-ar", "48000")      // Standard sample rate
		
		// Critical: Audio sync and timing correction to prevent pops
		args = append(args, "-async", "1")       // Audio sync correction
		args = append(args, "-fps_mode", "cfr")  // Constant frame rate for sync (replaces deprecated -vsync)
		
		// Simplified audio filtering for stability (test with basic filters first)
		var audioFilters []string
		
		// Basic high-quality resampling
		audioFilters = append(audioFilters, "aresample=48000")
		
		// Simple channel downmixing for compatibility
		audioFilters = append(audioFilters, "pan=stereo|FL=FL+0.707*FC|FR=FR+0.707*FC")
		
		// Apply simplified audio filters
		if len(audioFilters) > 0 {
			args = append(args, "-af", strings.Join(audioFilters, ","))
		}
		
		// Force stereo output after proper processing
		args = append(args, "-ac", "2")          // Stereo output
		
		// Basic AAC encoding parameters for stability
		args = append(args, "-cutoff", "15000")            // Anti-aliasing filter
		
		// Audio delay correction for sync issues
		args = append(args, "-max_muxing_queue_size", "1024") // Prevent buffer overflows
	}
	
	return args
}

// getContainerSpecificArgs returns optimized settings for each container format
func (t *Transcoder) getContainerSpecificArgs(req TranscodeRequest, outputPath string) []string {
	var args []string
	
	switch req.Container {
	case "dash":
		// Check if ABR ladder is requested
		if req.EnableABR {
			return t.getDashABRArgs(req, outputPath)
		}
		
		// Enhanced DASH settings with stability fixes (using valid FFmpeg options only)
		args = append(args,
			"-f", "dash",
			
			// Fixed timing to prevent 2-minute failures
			"-seg_duration", "2",                    // Shorter, more reliable segments
			// Note: min_seg_duration and max_seg_duration are not valid in this FFmpeg version
			
			// Core DASH settings
			"-use_template", "1",
			"-use_timeline", "1",
			"-streaming", "1",                       // Enable streaming mode
			"-single_file", "0",
			
			// Timestamp fixes for problematic sources
			"-avoid_negative_ts", "make_zero",       // Fix timestamp issues
			"-copyts",                               // Preserve original timestamps
			
			// Segment naming and structure
			"-init_seg_name", "init-$RepresentationID$.m4s",
			"-media_seg_name", "chunk-$RepresentationID$-$Number$.m4s",
			"-adaptation_sets", "id=0,streams=v id=1,streams=a",
			"-dash_segment_type", "mp4",
			
			// Better seeking and streaming optimization
			"-write_prft", "1",                      // Producer Reference Time for sync
			"-global_sidx", "1",                     // Global SIDX for better seeking
			"-frag_type", "duration",                // Use duration-based fragmentation
			"-frag_duration", "1",                   // 1 second fragment duration
			
			// Disable problematic low-latency features for stability
			"-ldash", "0",                           // Disable low-latency DASH
			
			// UTC timing for better sync (optional)
			"-utc_timing_url", "https://time.akamai.com/?iso",
		)
	case "hls":
		// Check if ABR ladder is requested
		if req.EnableABR {
			return t.getHLSABRArgs(req, outputPath)
		}
		
		// Single bitrate HLS
		outputDir := filepath.Dir(outputPath)
		segDuration := t.getAdaptiveSegmentDuration(req)
		
		// Use fMP4 segments for better seeking with byte-range support
		args = append(args,
			"-f", "hls",
			"-hls_time", segDuration,               // Adaptive segment duration
			"-hls_playlist_type", "vod",
			"-hls_segment_type", "fmp4",            // Use fMP4 for byte-range support
			"-hls_fmp4_init_filename", "init.mp4",  // Single init segment
			"-hls_segment_filename", filepath.Join(outputDir, "segment_%03d.m4s"),
			"-hls_flags", "independent_segments+program_date_time+single_file",
			// Enable byte-range support for efficient seeking
			"-hls_segment_options", "movflags=+cmaf+dash+delay_moov+global_sidx+write_colr+write_gama",
			// Low-latency HLS optimizations
			"-hls_list_size", "0",                  // Keep all segments in playlist
			"-hls_start_number_source", "datetime", // Better segment numbering
			// Partial segment support for LL-HLS
			"-hls_partial_duration", "0.5",        // 500ms partial segments
		)
		
		// Add LL-HLS specific settings if seek position indicates need for responsiveness
		if req.Seek > 0 {
			args = append(args,
				"-hls_flags", "independent_segments+program_date_time+single_file+temp_file",
				"-master_pl_name", "master.m3u8",
				"-master_pl_publish_rate", "2",     // Update master playlist every 2 segments
			)
		}
		
	default: // MP4 with streaming optimizations
		args = append(args,
			"-f", "mp4",
			"-movflags", "+faststart+frag_keyframe+empty_moov+dash+cmaf+global_sidx+write_colr",
			"-frag_duration", "1",                  // 1s fragments for better seeking
			"-min_frag_duration", "0.5",            // Min 500ms fragments
			"-brand", "mp42",                       // Better compatibility
			// Seek optimization
			"-write_tmcd", "0",                     // Disable timecode track
			"-strict", "experimental",              // Enable experimental features
		)
	}
	
	return args
}

// getAdaptiveSegmentDuration returns segment duration based on content and seek position
func (t *Transcoder) getAdaptiveSegmentDuration(req TranscodeRequest) string {
	// For seek-ahead requests, use shorter segments for better responsiveness
	if req.Seek > 0 {
		return "2" // 2 second segments for seek-ahead
	}
	
	// For regular playback, optimize based on content characteristics
	// Start with shorter segments, can be adapted during transcoding
	
	// Default to 3 seconds for good balance of startup time vs efficiency
	// FFmpeg will adapt this based on keyframe intervals and content
	return "3"
}

// monitorSession monitors a transcoding session
func (t *Transcoder) monitorSession(session *Session) {
	if t.logger != nil {
		t.logger.Info("monitorSession started", "session_id", session.ID, "pid", session.Process.Process.Pid)
	}

	// Wait for the process to complete
	err := session.Process.Wait()

	// Get exit information before removing session
	var exitCode int
	if session.Process.ProcessState != nil {
		exitCode = session.Process.ProcessState.ExitCode()
	}

	// Update progress based on completion
	if err == nil {
		session.Progress = 100.0
	}

	// Remove from sessions
	t.mutex.Lock()
	delete(t.sessions, session.ID)
	t.mutex.Unlock()

	if err != nil && t.logger != nil {
		t.logger.Error("FFmpeg process failed", 
			"session_id", session.ID, 
			"error", err,
			"exit_code", exitCode,
			"pid", session.Process.Process.Pid)
	} else if t.logger != nil {
		t.logger.Info("FFmpeg process completed", 
			"session_id", session.ID,
			"exit_code", exitCode,
			"pid", session.Process.Process.Pid)
	}
}

// Streaming methods (simple stubs for now)
func (t *Transcoder) StartStream(ctx context.Context, req TranscodeRequest) (*StreamHandle, error) {
	return nil, fmt.Errorf("streaming not implemented in simple transcoder")
}

func (t *Transcoder) GetStream(handle *StreamHandle) (io.ReadCloser, error) {
	return nil, fmt.Errorf("streaming not implemented in simple transcoder")
}

func (t *Transcoder) StopStream(handle *StreamHandle) error {
	return fmt.Errorf("streaming not implemented in simple transcoder")
}

// getKeyframeAlignmentArgs returns FFmpeg arguments for keyframe alignment
func (t *Transcoder) getKeyframeAlignmentArgs(req TranscodeRequest) []string {
	var args []string
	
	// Determine segment duration for adaptive streaming
	segmentDuration := 2.0 // Default 2 seconds
	if req.Container == "dash" || req.Container == "hls" {
		segmentDuration = t.getSegmentDurationFloat(req)
	}
	
	// Enhanced keyframe alignment to prevent 2-minute failures
	// Force keyframes at consistent intervals matching segment boundaries
	keyframeExpr := fmt.Sprintf("expr:gte(t,n_forced*%.1f)", segmentDuration)
	args = append(args, "-force_key_frames", keyframeExpr)
	
	// Improved GOP settings for stability and seeking
	gopSize := int(segmentDuration * 30) // 60 frames at 30fps for 2s segments
	minGopSize := int(segmentDuration * 24) // 48 frames at 24fps minimum
	
	args = append(args, "-g", strconv.Itoa(gopSize))           // Max GOP size
	args = append(args, "-keyint_min", strconv.Itoa(minGopSize)) // Min GOP size
	
	// Ensure closed GOPs for better seeking and segment alignment
	args = append(args, "-flags", "+cgop")
	args = append(args, "-bf", "3")                            // 3 B-frames for quality
	args = append(args, "-b_strategy", "2")                    // Optimal B-frame strategy
	
	// Scene change detection threshold - prevent excessive keyframes
	args = append(args, "-sc_threshold", "40")
	
	// Additional stability settings
	args = append(args, "-refs", "2")                          // Reference frames
	args = append(args, "-rc_lookahead", "40")                 // Rate control lookahead
	
	return args
}

// getSegmentDurationFloat returns segment duration as float for calculations
func (t *Transcoder) getSegmentDurationFloat(req TranscodeRequest) float64 {
	// Use consistent segment duration for stability
	return 4.0 // 4 seconds for all cases
}

// getDashABRArgs returns DASH arguments for adaptive bitrate streaming
func (t *Transcoder) getDashABRArgs(req TranscodeRequest, outputPath string) []string {
	var args []string
	
	// Get source dimensions (simplified - in real implementation would probe the file)
	sourceWidth := 1920
	sourceHeight := 1080
	if req.Resolution != nil {
		sourceWidth = req.Resolution.Width
		sourceHeight = req.Resolution.Height
	}
	
	// Generate bitrate ladder
	ladder := t.generateBitrateLadder(sourceWidth, sourceHeight, req.Quality)
	
	// Map streams for each quality level
	var maps []string
	var adaptationSets []string
	
	for i, rung := range ladder {
		// Create a named output for each quality
		maps = append(maps,
			"-map", "0:v:0",
			"-map", "0:a:0",
		)
		
		// Video encoding settings for this rung
		streamIndex := i * 2
		args = append(args,
			fmt.Sprintf("-c:v:%d", streamIndex), "libx264",
			fmt.Sprintf("-b:v:%d", streamIndex), fmt.Sprintf("%dk", rung.videoBitrate),
			fmt.Sprintf("-maxrate:%d", streamIndex), fmt.Sprintf("%dk", int(float64(rung.videoBitrate)*1.5)),
			fmt.Sprintf("-bufsize:%d", streamIndex), fmt.Sprintf("%dk", rung.videoBitrate*2),
			fmt.Sprintf("-vf:%d", streamIndex), fmt.Sprintf("scale=%d:%d:flags=lanczos", rung.width, rung.height),
			fmt.Sprintf("-profile:v:%d", streamIndex), rung.profile,
			fmt.Sprintf("-level:%d", streamIndex), rung.level,
			fmt.Sprintf("-crf:%d", streamIndex), strconv.Itoa(rung.crf),
		)
		
		// Audio encoding settings for this rung
		audioIndex := streamIndex + 1
		args = append(args,
			fmt.Sprintf("-c:a:%d", audioIndex), "aac",
			fmt.Sprintf("-b:a:%d", audioIndex), fmt.Sprintf("%dk", rung.audioBitrate),
			fmt.Sprintf("-ar:%d", audioIndex), "48000",
			fmt.Sprintf("-profile:a:%d", audioIndex), "aac_low",
			// Let FFmpeg handle channels automatically
		)
		
		// Build adaptation set mapping
		adaptationSets = append(adaptationSets, fmt.Sprintf("id=%d,streams=%d", i, streamIndex))
		adaptationSets = append(adaptationSets, fmt.Sprintf("id=%d,streams=%d", len(ladder)+i, audioIndex))
	}
	
	// Apply all maps first
	args = append(maps, args...)
	
	// DASH muxer settings with seek optimization
	segDuration := t.getAdaptiveSegmentDuration(req)
	args = append(args,
		"-f", "dash",
		"-seg_duration", segDuration,
		"-use_template", "1",
		"-use_timeline", "1",
		"-single_file", "0",
		"-adaptation_sets", strings.Join(adaptationSets, " "),
		"-media_seg_name", "chunk-$RepresentationID$-$Number$.m4s",
		"-init_seg_name", "init-$RepresentationID$.m4s",
		// Seek optimization features
		"-write_prft", "1",                      // Producer Reference Time
		"-global_sidx", "1",                    // Global SIDX for all segments
		"-profile", "urn:mpeg:dash:profile:isoff-on-demand:2011", // On-demand profile
	)
	
	return args
}

// getHLSABRArgs returns HLS arguments for adaptive bitrate streaming  
func (t *Transcoder) getHLSABRArgs(req TranscodeRequest, outputPath string) []string {
	var args []string
	
	// Get source dimensions
	sourceWidth := 1920
	sourceHeight := 1080
	if req.Resolution != nil {
		sourceWidth = req.Resolution.Width
		sourceHeight = req.Resolution.Height
	}
	
	// Generate bitrate ladder
	ladder := t.generateBitrateLadder(sourceWidth, sourceHeight, req.Quality)
	outputDir := filepath.Dir(outputPath)
	
	// Create variant streams
	var variantStreams []string
	
	for i, rung := range ladder {
		// Map video and audio
		args = append(args,
			"-map", "0:v:0",
			"-map", "0:a:0",
		)
		
		// Video encoding settings
		args = append(args,
			fmt.Sprintf("-c:v:%d", i), "libx264",
			fmt.Sprintf("-b:v:%d", i), fmt.Sprintf("%dk", rung.videoBitrate),
			fmt.Sprintf("-maxrate:%d", i), fmt.Sprintf("%dk", int(float64(rung.videoBitrate)*1.5)),
			fmt.Sprintf("-bufsize:%d", i), fmt.Sprintf("%dk", rung.videoBitrate*2),
			fmt.Sprintf("-vf:%d", i), fmt.Sprintf("scale=%d:%d:flags=lanczos", rung.width, rung.height),
			fmt.Sprintf("-profile:v:%d", i), rung.profile,
			fmt.Sprintf("-level:%d", i), rung.level,
		)
		
		// Audio encoding settings
		args = append(args,
			fmt.Sprintf("-c:a:%d", i), "aac",
			fmt.Sprintf("-b:a:%d", i), fmt.Sprintf("%dk", rung.audioBitrate),
			fmt.Sprintf("-ar:%d", i), "48000",
			fmt.Sprintf("-profile:a:%d", i), "aac_low",
		)
		
		// Variant playlist info
		variantStreams = append(variantStreams,
			fmt.Sprintf("v:%d,a:%d,name:%s", i, i, rung.label),
		)
	}
	
	// HLS muxer settings
	segDuration := t.getAdaptiveSegmentDuration(req)
	args = append(args,
		"-f", "hls",
		"-hls_time", segDuration,
		"-hls_playlist_type", "vod",
		"-hls_segment_type", "mpegts",
		"-hls_flags", "independent_segments",
		"-master_pl_name", "playlist.m3u8",
		"-hls_segment_filename", filepath.Join(outputDir, "stream_%v/segment_%03d.ts"),
		"-var_stream_map", strings.Join(variantStreams, " "),
	)
	
	return args
}

// generateBitrateLadder creates an optimized set of encoding profiles
func (t *Transcoder) generateBitrateLadder(sourceWidth, sourceHeight, quality int) []bitrateLadderRung {
	var ladder []bitrateLadderRung
	
	// Calculate aspect ratio
	aspectRatio := float64(sourceWidth) / float64(sourceHeight)
	
	// Define standard ladder rungs
	standardRungs := []struct {
		height       int
		videoBitrate int // kbps
		audioBitrate int // kbps  
		profile      string
		level        string
		crf          int
		label        string
	}{
		{240, 300, 64, "baseline", "3.0", 28, "240p"},
		{360, 600, 96, "baseline", "3.0", 26, "360p"},
		{480, 1000, 128, "main", "3.1", 24, "480p"},
		{720, 2500, 192, "high", "4.0", 23, "720p"},
		{1080, 5000, 256, "high", "4.1", 22, "1080p"},
	}
	
	// Only include rungs up to source resolution
	for _, rung := range standardRungs {
		if rung.height > sourceHeight {
			break
		}
		
		width := int(float64(rung.height) * aspectRatio)
		if width%2 != 0 {
			width++
		}
		
		// Adjust bitrate based on quality setting
		adjustedBitrate := rung.videoBitrate * quality / 65
		
		ladder = append(ladder, bitrateLadderRung{
			width:        width,
			height:       rung.height,
			videoBitrate: adjustedBitrate,
			audioBitrate: rung.audioBitrate,
			profile:      rung.profile,
			level:        rung.level,
			crf:          rung.crf,
			label:        rung.label,
		})
	}
	
	// Always include at least the lowest rung
	if len(ladder) == 0 {
		width := int(float64(240) * aspectRatio)
		if width%2 != 0 {
			width++
		}
		ladder = append(ladder, bitrateLadderRung{
			width:        width,
			height:       240,
			videoBitrate: 300,
			audioBitrate: 64,
			profile:      "baseline",
			level:        "3.0",
			crf:          28,
			label:        "240p",
		})
	}
	
	return ladder
}

// bitrateLadderRung represents a single quality level in the ABR ladder
type bitrateLadderRung struct {
	width        int
	height       int
	videoBitrate int    // kbps
	audioBitrate int    // kbps
	profile      string // H.264 profile
	level        string // H.264 level
	crf          int    // Constant Rate Factor
	label        string // Human-readable label
}

// Dashboard methods (basic implementations)
func (t *Transcoder) GetDashboardSections() []DashboardSection {
	return []DashboardSection{
		{
			ID:    "overview",
			Title: "Overview",
			Type:  "stats",
		},
	}
}

func (t *Transcoder) GetDashboardData(sectionID string) (interface{}, error) {
	switch sectionID {
	case "overview":
		t.mutex.RLock()
		sessionCount := len(t.sessions)
		t.mutex.RUnlock()

		return map[string]interface{}{
			"active_sessions": sessionCount,
			"provider":        t.name,
		}, nil
	default:
		return nil, fmt.Errorf("unknown section: %s", sectionID)
	}
}

func (t *Transcoder) ExecuteDashboardAction(actionID string, params map[string]interface{}) error {
	return fmt.Errorf("action not supported: %s", actionID)
}