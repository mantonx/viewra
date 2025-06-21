package transcoding

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

// BaseTranscoder provides common transcoding functionality that can be embedded in plugins
type BaseTranscoder struct {
	logger Logger
	
	// Configuration
	name        string
	description string
	version     string
	author      string
	priority    int
	
	// FFmpeg configuration
	argsBuilder *FFmpegArgsBuilder
	ffmpegPath  string
	
	// Session management
	sessions map[string]*TranscodeSession
	mutex    sync.RWMutex
}

// TranscodeSession represents an active transcoding session
type TranscodeSession struct {
	ID        string
	Handle    *TranscodeHandle
	Process   *exec.Cmd
	StartTime time.Time
	Request   TranscodeRequest
	Cancel    context.CancelFunc
}

// NewBaseTranscoder creates a new base transcoder
func NewBaseTranscoder(name, description, version, author string, priority int, hwAccelType string) *BaseTranscoder {
	ffmpegPath := "ffmpeg"
	if customPath := os.Getenv("FFMPEG_PATH"); customPath != "" {
		ffmpegPath = customPath
	}

	return &BaseTranscoder{
		name:        name,
		description: description,
		version:     version,
		author:      author,
		priority:    priority,
		argsBuilder: NewFFmpegArgsBuilder(hwAccelType),
		ffmpegPath:  ffmpegPath,
		sessions:    make(map[string]*TranscodeSession),
	}
}

// SetLogger sets the logger for this transcoder
func (bt *BaseTranscoder) SetLogger(logger Logger) {
	bt.logger = logger
}

// GetInfo returns provider information
func (bt *BaseTranscoder) GetInfo() ProviderInfo {
	return ProviderInfo{
		ID:          bt.name,
		Name:        bt.name,
		Description: bt.description,
		Version:     bt.version,
		Author:      bt.author,
		Priority:    bt.priority,
	}
}

// GetSupportedFormats returns supported container formats
func (bt *BaseTranscoder) GetSupportedFormats() []ContainerFormat {
	return []ContainerFormat{
		{
			Format:      "mp4",
			MimeType:    "video/mp4",
			Extensions:  []string{".mp4"},
			Description: "MPEG-4 Container",
			Adaptive:    false,
		},
		{
			Format:      "webm",
			MimeType:    "video/webm",
			Extensions:  []string{".webm"},
			Description: "WebM Container",
			Adaptive:    false,
		},
		{
			Format:      "mkv",
			MimeType:    "video/x-matroska",
			Extensions:  []string{".mkv"},
			Description: "Matroska Container",
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
func (bt *BaseTranscoder) StartTranscode(ctx context.Context, req TranscodeRequest) (*TranscodeHandle, error) {
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
		outputDir = fmt.Sprintf("%s/%s_%s_%s", baseDir, req.Container, bt.name, sessionID)
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

	// Build FFmpeg arguments using SDK builder
	args := bt.argsBuilder.BuildArgs(&req, outputPath)

	// Create the command directly without shell wrapper
	cmd := exec.CommandContext(sessionCtx, bt.ffmpegPath, args...)
	
	// Set process group to prevent FFmpeg from being killed with parent
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group for FFmpeg
	}
	
	// Add debugging for process attributes
	if bt.logger != nil {
		bt.logger.Info("configuring FFmpeg process",
			"session_id", sessionID,
			"ffmpeg_path", bt.ffmpegPath,
			"setpgid", true,
		)
	}
	
	// Enable debug mode if requested
	if os.Getenv("FFMPEG_DEBUG") == "true" {
		// Create debug directory
		debugDir := filepath.Join(filepath.Dir(outputPath), "debug")
		os.MkdirAll(debugDir, 0755)
		
		// Create debug log
		debugLog, err := os.Create(filepath.Join(debugDir, fmt.Sprintf("ffmpeg_%s.log", sessionID)))
		if err == nil {
			debugLog.WriteString(fmt.Sprintf("=== FFmpeg Debug Log ===\n"))
			debugLog.WriteString(fmt.Sprintf("Session: %s\n", sessionID))
			debugLog.WriteString(fmt.Sprintf("Time: %s\n", time.Now().Format(time.RFC3339)))
			debugLog.WriteString(fmt.Sprintf("Command: %s %v\n", bt.ffmpegPath, args))
			debugLog.WriteString(fmt.Sprintf("Input: %s\n", req.InputPath))
			debugLog.WriteString(fmt.Sprintf("Output: %s\n", outputPath))
			debugLog.WriteString(fmt.Sprintf("\n=== Environment ===\n"))
			for _, env := range os.Environ() {
				if strings.Contains(env, "FFMPEG") || strings.Contains(env, "PATH") {
					debugLog.WriteString(fmt.Sprintf("%s\n", env))
				}
			}
			debugLog.WriteString(fmt.Sprintf("\n=== FFmpeg Output ===\n"))
			
			// Redirect both stdout and stderr to debug log
			cmd.Stdout = debugLog
			cmd.Stderr = debugLog
			
			defer debugLog.Close()
		}
	}
	
	// Capture stderr for debugging (only if not already redirected to debug log)
	if os.Getenv("FFMPEG_DEBUG") != "true" {
		cmd.Stderr = &logWriter{logger: bt.logger, prefix: "[ffmpeg stderr] "}
		cmd.Stdout = &logWriter{logger: bt.logger, prefix: "[ffmpeg stdout] "}
	}

	// Log the command for debugging
	if bt.logger != nil {
		bt.logger.Info("starting FFmpeg transcoding",
			"session_id", sessionID,
			"command", fmt.Sprintf("ffmpeg %v", args),
		)
	}

	// Create handle
	handle := &TranscodeHandle{
		SessionID:   sessionID,
		Provider:    bt.name,
		StartTime:   time.Now(),
		Directory:   outputDir,
		Context:     sessionCtx,
		CancelFunc:  cancelFunc,
		PrivateData: sessionID,
	}

	// Create session
	session := &TranscodeSession{
		ID:        sessionID,
		Handle:    handle,
		Process:   cmd,
		StartTime: time.Now(),
		Request:   req,
		Cancel:    cancelFunc,
	}

	// Store session
	bt.mutex.Lock()
	bt.sessions[sessionID] = session
	bt.mutex.Unlock()

	// Start FFmpeg process
	if err := cmd.Start(); err != nil {
		cancelFunc()
		bt.mutex.Lock()
		delete(bt.sessions, sessionID)
		bt.mutex.Unlock()
		if bt.logger != nil {
			bt.logger.Error("failed to start FFmpeg process",
				"session_id", sessionID,
				"error", err,
				"ffmpeg_path", bt.ffmpegPath,
			)
		}
		return nil, fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	// Log process info
	if bt.logger != nil {
		bt.logger.Info("FFmpeg process started",
			"session_id", sessionID,
			"pid", cmd.Process.Pid,
		)
	}

	// Monitor the process in a goroutine
	go bt.monitorSession(session)

	return handle, nil
}

// GetProgress returns transcoding progress
func (bt *BaseTranscoder) GetProgress(handle *TranscodeHandle) (*TranscodingProgress, error) {
	if handle == nil {
		return nil, fmt.Errorf("invalid handle")
	}

	bt.mutex.RLock()
	session, exists := bt.sessions[handle.SessionID]
	bt.mutex.RUnlock()

	if !exists {
		// Session not found - it may have failed or been terminated
		return nil, fmt.Errorf("session not found: %s", handle.SessionID)
	}

	// For now, return basic progress based on time
	// TODO: Parse FFmpeg output for real progress
	elapsed := time.Since(session.StartTime)
	progress := &TranscodingProgress{
		PercentComplete: 0, // Would need FFmpeg output parsing
		TimeElapsed:     elapsed,
		CurrentSpeed:    1.0,
		AverageSpeed:    1.0,
	}

	return progress, nil
}

// StopTranscode stops a transcoding session
func (bt *BaseTranscoder) StopTranscode(handle *TranscodeHandle) error {
	if handle == nil {
		return fmt.Errorf("invalid handle")
	}

	bt.mutex.Lock()
	session, exists := bt.sessions[handle.SessionID]
	if exists {
		delete(bt.sessions, handle.SessionID)
	}
	bt.mutex.Unlock()

	if !exists {
		return fmt.Errorf("session not found: %s", handle.SessionID)
	}

	// Cancel the context (this should stop FFmpeg)
	session.Cancel()

	// Kill the process group if it's still running
	if session.Process != nil && session.Process.Process != nil {
		pid := session.Process.Process.Pid
		// Try to kill the entire process group
		if pgid, err := syscall.Getpgid(pid); err == nil {
			// Send SIGTERM to the process group (negative pgid)
			if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
				// Fallback to killing just the process
				if err := session.Process.Process.Kill(); err != nil {
					if bt.logger != nil {
						bt.logger.Warn("failed to kill FFmpeg process", "error", err)
					}
				}
			}
		} else {
			// Fallback to killing just the process
			if err := session.Process.Process.Kill(); err != nil {
				if bt.logger != nil {
					bt.logger.Warn("failed to kill FFmpeg process", "error", err)
				}
			}
		}
	}

	return nil
}

// monitorSession monitors a transcoding session
func (bt *BaseTranscoder) monitorSession(session *TranscodeSession) {
	// Give process time to start
	time.Sleep(500 * time.Millisecond)
	
	// Log process state before waiting
	if bt.logger != nil && session.Process.Process != nil {
		// Check if process is still alive
		if session.Process.ProcessState == nil {
			// Process hasn't exited yet
			bt.logger.Info("monitoring FFmpeg process",
				"session_id", session.ID,
				"pid", session.Process.Process.Pid,
				"process_state", "running")
		} else {
			// Process already exited
			bt.logger.Warn("FFmpeg process already exited",
				"session_id", session.ID,
				"pid", session.Process.Process.Pid,
				"exit_code", session.Process.ProcessState.ExitCode())
		}
	}
	
	// Wait for the process to complete
	err := session.Process.Wait()

	// Get exit information before removing session
	var exitCode int
	var wasKilled bool
	if session.Process.ProcessState != nil {
		exitCode = session.Process.ProcessState.ExitCode()
		wasKilled = !session.Process.ProcessState.Success() && exitCode == -1
	}

	// Remove from sessions
	bt.mutex.Lock()
	delete(bt.sessions, session.ID)
	bt.mutex.Unlock()

	if err != nil && bt.logger != nil {
		bt.logger.Error("FFmpeg process failed", 
			"session_id", session.ID, 
			"error", err,
			"exit_code", exitCode,
			"was_killed", wasKilled,
			"system_time", time.Now().Format(time.RFC3339Nano))
	} else if bt.logger != nil {
		bt.logger.Info("FFmpeg process completed", 
			"session_id", session.ID,
			"exit_code", exitCode)
	}
}

// logWriter is a simple writer that logs output
type logWriter struct {
	logger Logger
	prefix string
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	if w.logger != nil {
		w.logger.Debug(w.prefix + string(p))
	}
	return len(p), nil
}

// StreamSession represents an active streaming session
type StreamSession struct {
	ID          string
	Handle      *StreamHandle
	Process     *exec.Cmd
	StartTime   time.Time
	Request     TranscodeRequest
	Cancel      context.CancelFunc
	OutputPath  string
	IsAdaptive  bool
}

// Streaming methods with DASH/HLS support
func (bt *BaseTranscoder) StartStream(ctx context.Context, req TranscodeRequest) (*StreamHandle, error) {
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
		baseDir := "/app/viewra-data/streaming"
		if envDir := os.Getenv("VIEWRA_STREAMING_DIR"); envDir != "" {
			baseDir = envDir
		}
		outputDir = fmt.Sprintf("%s/%s_%s_%s", baseDir, req.Container, bt.name, sessionID)
	}
	
	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create streaming directory: %w", err)
	}

	// Determine output configuration based on container
	var outputPath string
	var isAdaptive bool
	
	switch req.Container {
	case "dash":
		outputPath = filepath.Join(outputDir, "manifest.mpd")
		isAdaptive = true
	case "hls":
		outputPath = filepath.Join(outputDir, "playlist.m3u8")
		isAdaptive = true
	default:
		return nil, fmt.Errorf("unsupported streaming container: %s", req.Container)
	}

	// Create context with cancel for this session
	streamCtx, cancelFunc := context.WithCancel(ctx)

	// Build FFmpeg arguments for streaming
	args := bt.buildStreamingArgs(&req, outputPath)

	// Create the streaming command
	cmd := exec.CommandContext(streamCtx, bt.ffmpegPath, args...)
	
	// Set process group to prevent FFmpeg from being killed with parent
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group for FFmpeg
	}

	// Log the streaming command
	if bt.logger != nil {
		bt.logger.Info("starting FFmpeg streaming",
			"session_id", sessionID,
			"container", req.Container,
			"output", outputPath,
			"command", fmt.Sprintf("ffmpeg %v", args),
		)
	}

	// Create stream handle
	handle := &StreamHandle{
		SessionID:   sessionID,
		Status:      TranscodeStatusStarting,
		Context:     streamCtx,
		CancelFunc:  cancelFunc,
		PrivateData: sessionID,
	}

	// Create stream session
	streamSession := &StreamSession{
		ID:         sessionID,
		Handle:     handle,
		Process:    cmd,
		StartTime:  time.Now(),
		Request:    req,
		Cancel:     cancelFunc,
		OutputPath: outputPath,
		IsAdaptive: isAdaptive,
	}

	// Store stream session (we'll use the same sessions map for simplicity)
	bt.mutex.Lock()
	// Convert to TranscodeSession for storage compatibility
	session := &TranscodeSession{
		ID:        sessionID,
		Handle:    nil, // No transcode handle for streaming
		Process:   cmd,
		StartTime: time.Now(),
		Request:   req,
		Cancel:    cancelFunc,
	}
	bt.sessions[sessionID] = session
	bt.mutex.Unlock()

	// Start FFmpeg streaming process
	if err := cmd.Start(); err != nil {
		cancelFunc()
		bt.mutex.Lock()
		delete(bt.sessions, sessionID)
		bt.mutex.Unlock()
		return nil, fmt.Errorf("failed to start FFmpeg streaming: %w", err)
	}

	// Update status
	handle.Status = TranscodeStatusRunning

	// Monitor the streaming session
	go bt.monitorStreamSession(streamSession)

	return handle, nil
}

func (bt *BaseTranscoder) GetStream(handle *StreamHandle) (io.ReadCloser, error) {
	if handle == nil {
		return nil, fmt.Errorf("invalid stream handle")
	}

	bt.mutex.RLock()
	session, exists := bt.sessions[handle.SessionID]
	bt.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("stream session not found: %s", handle.SessionID)
	}

	// For DASH/HLS, we return the manifest/playlist file
	baseDir := "/app/viewra-data/streaming"
	if envDir := os.Getenv("VIEWRA_STREAMING_DIR"); envDir != "" {
		baseDir = envDir
	}
	
	var manifestPath string
	switch session.Request.Container {
	case "dash":
		manifestPath = filepath.Join(fmt.Sprintf("%s/%s_%s_%s", 
			baseDir, session.Request.Container, bt.name, handle.SessionID), "manifest.mpd")
	case "hls":
		manifestPath = filepath.Join(fmt.Sprintf("%s/%s_%s_%s", 
			baseDir, session.Request.Container, bt.name, handle.SessionID), "playlist.m3u8")
	default:
		return nil, fmt.Errorf("unsupported streaming container: %s", session.Request.Container)
	}

	// Wait briefly for the manifest to be created
	for i := 0; i < 30; i++ { // Wait up to 3 seconds
		if _, err := os.Stat(manifestPath); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Open and return the manifest file
	file, err := os.Open(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open streaming manifest: %w", err)
	}

	return file, nil
}

func (bt *BaseTranscoder) StopStream(handle *StreamHandle) error {
	if handle == nil {
		return fmt.Errorf("invalid stream handle")
	}

	bt.mutex.Lock()
	session, exists := bt.sessions[handle.SessionID]
	if exists {
		delete(bt.sessions, handle.SessionID)
	}
	bt.mutex.Unlock()

	if !exists {
		return fmt.Errorf("stream session not found: %s", handle.SessionID)
	}

	// Cancel the context (this should stop FFmpeg)
	session.Cancel()

	// Kill the process if it's still running
	if session.Process != nil && session.Process.Process != nil {
		if err := session.Process.Process.Kill(); err != nil {
			if bt.logger != nil {
				bt.logger.Warn("failed to kill FFmpeg streaming process", "error", err)
			}
		}
	}

	return nil
}

// buildStreamingArgs builds FFmpeg arguments for streaming
func (bt *BaseTranscoder) buildStreamingArgs(req *TranscodeRequest, outputPath string) []string {
	// Start with basic streaming-specific args
	args := []string{"-y"} // Overwrite output

	// Hardware acceleration (before input)
	if bt.argsBuilder.hwAccelType != "none" {
		args = append(args, "-hwaccel", bt.argsBuilder.hwAccelType)
	}

	// Input
	args = append(args, "-i", req.InputPath)

	// Video codec
	args = append(args, "-c:v", req.VideoCodec)
	
	// Audio codec
	args = append(args, "-c:a", req.AudioCodec)

	// Quality settings based on quality percentage
	if req.Quality > 0 {
		switch req.VideoCodec {
		case "h264", "libx264":
			// CRF scale (lower = better quality)
			crf := 51 - (req.Quality * 28 / 100) // Maps 0-100 to CRF 51-23
			if crf < 18 { crf = 18 } // Minimum CRF
			args = append(args, "-crf", fmt.Sprintf("%d", crf))
		case "h265", "libx265":
			crf := 51 - (req.Quality * 23 / 100) // Maps 0-100 to CRF 51-28
			if crf < 20 { crf = 20 }
			args = append(args, "-crf", fmt.Sprintf("%d", crf))
		}
	}

	// Speed/preset settings
	switch req.SpeedPriority {
	case SpeedPriorityFastest:
		args = append(args, "-preset", "ultrafast")
	case SpeedPriorityBalanced:
		args = append(args, "-preset", "medium")
	case SpeedPriorityQuality:
		args = append(args, "-preset", "slow")
	}

	// Container-specific streaming arguments
	switch req.Container {
	case "dash":
		args = append(args,
			"-f", "dash",
			"-seg_duration", "4",
			"-adaptation_sets", "id=0,streams=v id=1,streams=a",
			"-use_template", "1",
			"-use_timeline", "1",
			"-init_seg_name", "init-$RepresentationID$.m4s",
			"-media_seg_name", "chunk-$RepresentationID$-$Number$.m4s",
		)
	case "hls":
		args = append(args,
			"-f", "hls",
			"-hls_time", "4",
			"-hls_playlist_type", "vod",
			"-hls_segment_filename", filepath.Join(filepath.Dir(outputPath), "segment_%03d.ts"),
		)
	}

	// Add output path
	args = append(args, outputPath)

	return args
}

// monitorStreamSession monitors a streaming session
func (bt *BaseTranscoder) monitorStreamSession(streamSession *StreamSession) {
	// Wait for the process to complete
	err := streamSession.Process.Wait()

	// Remove from sessions
	bt.mutex.Lock()
	delete(bt.sessions, streamSession.ID)
	bt.mutex.Unlock()

	// Update handle status
	if err != nil {
		streamSession.Handle.Status = TranscodeStatusFailed
		streamSession.Handle.Error = err.Error()
		if bt.logger != nil {
			bt.logger.Error("FFmpeg streaming process failed", "session_id", streamSession.ID, "error", err)
		}
	} else {
		streamSession.Handle.Status = TranscodeStatusCompleted
		if bt.logger != nil {
			bt.logger.Info("FFmpeg streaming process completed", "session_id", streamSession.ID)
		}
	}
}

// Dashboard methods (basic implementations)
func (bt *BaseTranscoder) GetDashboardSections() []DashboardSection {
	return []DashboardSection{
		{
			ID:    "overview",
			Title: "Overview",
			Type:  "stats",
		},
	}
}

func (bt *BaseTranscoder) GetDashboardData(sectionID string) (interface{}, error) {
	switch sectionID {
	case "overview":
		bt.mutex.RLock()
		sessionCount := len(bt.sessions)
		bt.mutex.RUnlock()

		return map[string]interface{}{
			"active_sessions": sessionCount,
			"provider":        bt.name,
		}, nil
	default:
		return nil, fmt.Errorf("unknown section: %s", sectionID)
	}
}

func (bt *BaseTranscoder) ExecuteDashboardAction(actionID string, params map[string]interface{}) error {
	return fmt.Errorf("action not supported: %s", actionID)
}