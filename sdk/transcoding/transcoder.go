package transcoding

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

	// Hardware acceleration detection (auto-detect available hardware)
	args = append(args, "-hwaccel", "auto")
	
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

	// Video filtering for quality enhancement
	videoFilters := t.getVideoFilters(req)
	if len(videoFilters) > 0 {
		args = append(args, "-vf", videoFilters)
	}

	// Audio settings optimized for source content
	audioArgs := t.getOptimalAudioSettings(req)
	args = append(args, audioArgs...)

	// Threading and performance optimization
	args = append(args, "-threads", "0") // Use all available CPU cores

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
		return "faster"     // Balance speed vs quality
	case SpeedPriorityQuality:
		return "slow"       // High quality
	default:
		return "medium"     // Balanced default
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
		args = append(args, "-profile:v", "high")
		args = append(args, "-level", "4.1")
		// Optimize for streaming with good compression
		args = append(args, "-x264-params", "ref=4:bframes=3:b-pyramid=normal:mixed-refs=1:8x8dct=1:trellis=1:fast-pskip=0:cabac=1")
	}
	
	return args
}

// getVideoFilters returns video filters for quality enhancement
func (t *Transcoder) getVideoFilters(req TranscodeRequest) string {
	// Scale filter for resolution optimization if needed
	// This will be expanded based on source content analysis
	
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
	
	// Enhanced audio settings for high-quality content
	if audioCodec == "aac" {
		// Higher bitrate for better quality with multichannel support
		args = append(args, "-b:a", "256k")      // Increased for better multichannel quality
		args = append(args, "-profile:a", "aac_low")
		args = append(args, "-ar", "48000")      // Standard sample rate
		
		// Enhanced channel handling - preserve up to 5.1 for better audio experience
		// This will be mixed down appropriately by FFmpeg if source has fewer channels
		args = append(args, "-ac", "6")          // Support up to 5.1 surround
		args = append(args, "-channel_layout", "5.1") // Explicit 5.1 layout
		
		// Audio filtering for better downmixing when needed
		args = append(args, "-af", "aresample=async=1000") // Better audio sync
	}
	
	return args
}

// getContainerSpecificArgs returns optimized settings for each container format
func (t *Transcoder) getContainerSpecificArgs(req TranscodeRequest, outputPath string) []string {
	var args []string
	
	switch req.Container {
	case "dash":
		// Low-latency DASH with adaptive segments and optimized settings
		segDuration := t.getAdaptiveSegmentDuration(req)
		args = append(args,
			"-f", "dash",
			"-seg_duration", segDuration,            // Adaptive segment duration
			"-frag_duration", "500000",              // 500ms fragments for low latency (microseconds)
			"-use_template", "1",
			"-use_timeline", "1",
			"-streaming", "1",                       // Enable streaming mode
			"-ldash", "1",                          // Low-latency DASH
			"-target_latency", "2000000",           // 2 second target latency (microseconds)
			"-min_seg_duration", "1000000",         // Min 1s segments (microseconds)
			"-max_seg_duration", "8000000",         // Max 8s segments (microseconds)
			"-init_seg_name", "init-$RepresentationID$.m4s",
			"-media_seg_name", "chunk-$RepresentationID$-$Number$.m4s",
			"-adaptation_sets", "id=0,streams=v id=1,streams=a",
			"-dash_segment_type", "mp4",
			"-single_file", "0",
			"-remove_at_exit", "1",                 // Cleanup segments on exit
			// Low-latency optimizations
			"-seg_duration_adaptive", "1",          // Adaptive segment duration
			"-utc_timing_url", "https://time.akamai.com/?iso", // UTC timing for sync
		)
	case "hls":
		outputDir := filepath.Dir(outputPath)
		segDuration := t.getAdaptiveSegmentDuration(req)
		args = append(args,
			"-f", "hls",
			"-hls_time", segDuration,               // Adaptive segment duration
			"-hls_playlist_type", "vod",
			"-hls_segment_type", "mpegts",
			"-hls_segment_filename", filepath.Join(outputDir, "segment_%03d.ts"),
			"-hls_flags", "independent_segments+program_date_time",
			// Low-latency HLS optimizations
			"-hls_list_size", "10",                 // Keep more segments in playlist
			"-hls_delete_threshold", "10",          // Cleanup old segments
			"-hls_start_number_source", "datetime", // Better segment numbering
			// Partial segment support for LL-HLS
			"-hls_partial_duration", "0.5",        // 500ms partial segments
			"-hls_segment_options", "movflags=+cmaf+dash+frag_every_frame",
		)
		
		// Add LL-HLS specific settings if seek position indicates need for responsiveness
		if req.Seek > 0 {
			args = append(args,
				"-hls_flags", "independent_segments+program_date_time+temp_file",
				"-master_pl_name", "master.m3u8",
				"-master_pl_publish_rate", "2",     // Update master playlist every 2 segments
			)
		}
		
	default: // MP4 with streaming optimizations
		args = append(args,
			"-f", "mp4",
			"-movflags", "+faststart+frag_keyframe+empty_moov+dash+cmaf",
			"-frag_duration", "1000000",            // 1s fragments for better seeking
			"-min_frag_duration", "500000",         // Min 500ms fragments
			"-brand", "mp42",                       // Better compatibility
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