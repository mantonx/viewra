// Package streaming provides live streaming capabilities for video transcoding.
// This package handles DASH and HLS streaming protocols, managing streaming sessions,
// and providing real-time manifest generation. It ensures proper segment creation,
// manifest updates, and client-side playback compatibility.
//
// The streaming system supports:
// - DASH (Dynamic Adaptive Streaming over HTTP) with MPD manifests
// - HLS (HTTP Live Streaming) with M3U8 playlists
// - Live and VOD streaming modes
// - Adaptive bitrate streaming
// - Low-latency streaming optimizations
//
// Key features:
// - Session-based streaming management
// - Automatic manifest generation and updates
// - Segment file management
// - Process lifecycle handling
// - Resource cleanup on stream termination
//
// Example usage:
//   streamer := streaming.NewStreamer(logger, ffmpegPath)
//   handle, err := streamer.StartStream(ctx, request)
//   if err != nil {
//       log.Fatal(err)
//   }
//   defer streamer.StopStream(handle)
//   
//   // Get manifest for client playback
//   manifest, err := streamer.GetManifest(handle)
package streaming

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/mantonx/viewra/sdk/transcoding/ffmpeg"
	"github.com/mantonx/viewra/sdk/transcoding/process"
	"github.com/mantonx/viewra/sdk/transcoding/types"
)

// Streamer handles video streaming operations
type Streamer struct {
	logger          types.Logger
	ffmpegPath      string
	sessions        map[string]*StreamSession
	mutex           sync.RWMutex
	processRegistry *process.ProcessRegistry
	argsBuilder     *ffmpeg.FFmpegArgsBuilder
}

// StreamSession represents an active streaming session
type StreamSession struct {
	ID          string
	Handle      *types.StreamHandle
	Process     *exec.Cmd
	StartTime   time.Time
	Request     types.TranscodeRequest
	Cancel      context.CancelFunc
	OutputPath  string
	OutputDir   string
	IsAdaptive  bool
	Container   string
}

// NewStreamer creates a new streaming handler
func NewStreamer(logger types.Logger, ffmpegPath string, processRegistry *process.ProcessRegistry) *Streamer {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
		if customPath := os.Getenv("FFMPEG_PATH"); customPath != "" {
			ffmpegPath = customPath
		}
	}

	return &Streamer{
		logger:          logger,
		ffmpegPath:      ffmpegPath,
		sessions:        make(map[string]*StreamSession),
		processRegistry: processRegistry,
		argsBuilder:     ffmpeg.NewFFmpegArgsBuilder(logger),
	}
}

// StartStream starts a new streaming session
func (s *Streamer) StartStream(ctx context.Context, req types.TranscodeRequest) (*types.StreamHandle, error) {
	// Validate streaming container
	if req.Container != "dash" && req.Container != "hls" {
		return nil, fmt.Errorf("unsupported streaming container: %s (only dash and hls supported)", req.Container)
	}

	// Generate session ID if not provided
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	// Prepare output directory
	outputDir, outputPath, err := s.prepareOutputPaths(req, sessionID)
	if err != nil {
		return nil, err
	}

	// Create context with cancel
	streamCtx, cancelFunc := context.WithCancel(ctx)

	// Build FFmpeg arguments using the centralized builder
	// The FFmpegArgsBuilder already handles DASH/HLS with ABR support
	args := s.argsBuilder.BuildArgs(req, outputPath)

	// Create command
	cmd := exec.CommandContext(streamCtx, s.ffmpegPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group
	}

	// Set up logging
	if err := s.setupProcessLogging(cmd, outputDir, sessionID); err != nil {
		if s.logger != nil {
			s.logger.Warn("failed to setup process logging", "error", err)
		}
	}

	// Log the command
	if s.logger != nil {
		s.logger.Info("starting FFmpeg streaming",
			"session_id", sessionID,
			"container", req.Container,
			"output", outputPath,
			"command", fmt.Sprintf("ffmpeg %v", args),
		)
	}

	// Create handle
	handle := &types.StreamHandle{
		SessionID:   sessionID,
		Provider:    "streamer",
		StartTime:   time.Now(),
		Status:      types.TranscodeStatusStarting,
		Context:     streamCtx,
		CancelFunc:  cancelFunc,
		PrivateData: sessionID,
	}

	// Create session
	session := &StreamSession{
		ID:         sessionID,
		Handle:     handle,
		Process:    cmd,
		StartTime:  time.Now(),
		Request:    req,
		Cancel:     cancelFunc,
		OutputPath: outputPath,
		OutputDir:  outputDir,
		IsAdaptive: true,
		Container:  req.Container,
	}

	// Store session
	s.mutex.Lock()
	s.sessions[sessionID] = session
	s.mutex.Unlock()

	// Start process
	if err := cmd.Start(); err != nil {
		s.handleStartError(sessionID, err)
		return nil, fmt.Errorf("failed to start FFmpeg streaming: %w", err)
	}

	// Register process
	if s.processRegistry != nil {
		s.processRegistry.Register(cmd.Process.Pid, sessionID, "streamer")
	}

	// Update status
	handle.Status = types.TranscodeStatusRunning

	// Monitor the session
	go s.monitorSession(session)

	return handle, nil
}

// GetManifest returns the streaming manifest (MPD or M3U8)
func (s *Streamer) GetManifest(handle *types.StreamHandle) (io.ReadCloser, error) {
	if handle == nil {
		return nil, fmt.Errorf("invalid stream handle")
	}

	s.mutex.RLock()
	session, exists := s.sessions[handle.SessionID]
	s.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("stream session not found: %s", handle.SessionID)
	}

	// Wait briefly for manifest creation
	manifestPath := session.OutputPath
	for i := 0; i < 30; i++ { // Wait up to 3 seconds
		if _, err := os.Stat(manifestPath); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Open and return the manifest
	file, err := os.Open(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open streaming manifest: %w", err)
	}

	return file, nil
}

// StopStream stops a streaming session
func (s *Streamer) StopStream(handle *types.StreamHandle) error {
	if handle == nil {
		return fmt.Errorf("invalid stream handle")
	}

	s.mutex.Lock()
	session, exists := s.sessions[handle.SessionID]
	if exists {
		delete(s.sessions, handle.SessionID)
	}
	s.mutex.Unlock()

	if !exists {
		return fmt.Errorf("stream session not found: %s", handle.SessionID)
	}

	// Cancel context
	session.Cancel()

	// Kill process if running
	if session.Process != nil && session.Process.Process != nil {
		pid := session.Process.Process.Pid
		
		if s.processRegistry != nil {
			if err := s.processRegistry.KillProcess(pid); err != nil {
				if s.logger != nil {
					s.logger.Error("failed to kill streaming process", "pid", pid, "error", err)
				}
			}
		} else {
			// Direct kill as fallback
			if err := session.Process.Process.Kill(); err != nil {
				if s.logger != nil {
					s.logger.Warn("failed to kill streaming process", "error", err)
				}
			}
		}
	}

	return nil
}

// GetSession retrieves a streaming session
func (s *Streamer) GetSession(sessionID string) (*StreamSession, error) {
	s.mutex.RLock()
	session, exists := s.sessions[sessionID]
	s.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

// GetAllSessions returns all active streaming sessions
func (s *Streamer) GetAllSessions() []*StreamSession {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	sessions := make([]*StreamSession, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}

	return sessions
}

// prepareOutputPaths prepares output directory and manifest path
func (s *Streamer) prepareOutputPaths(req types.TranscodeRequest, sessionID string) (string, string, error) {
	var outputDir string
	if req.OutputPath != "" {
		outputDir = req.OutputPath
	} else {
		baseDir := "/app/viewra-data/streaming"
		if envDir := os.Getenv("VIEWRA_STREAMING_DIR"); envDir != "" {
			baseDir = envDir
		}
		outputDir = filepath.Join(baseDir, fmt.Sprintf("%s_%s", req.Container, sessionID))
	}
	
	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create streaming directory: %w", err)
	}

	// Determine manifest filename
	var manifestFile string
	switch req.Container {
	case "dash":
		manifestFile = "manifest.mpd"
	case "hls":
		manifestFile = "playlist.m3u8"
	}

	outputPath := filepath.Join(outputDir, manifestFile)
	return outputDir, outputPath, nil
}

// FFmpeg argument generation is now handled by the centralized FFmpegArgsBuilder

// setupProcessLogging sets up logging for the FFmpeg process
func (s *Streamer) setupProcessLogging(cmd *exec.Cmd, outputDir string, sessionID string) error {
	logDir := filepath.Join(outputDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	stdoutPath := filepath.Join(logDir, fmt.Sprintf("stream_%s_stdout.log", sessionID))
	stderrPath := filepath.Join(logDir, fmt.Sprintf("stream_%s_stderr.log", sessionID))

	stdoutFile, err := os.Create(stdoutPath)
	if err != nil {
		return err
	}

	stderrFile, err := os.Create(stderrPath)
	if err != nil {
		stdoutFile.Close()
		return err
	}

	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile

	return nil
}

// handleStartError handles errors when starting a streaming session
func (s *Streamer) handleStartError(sessionID string, err error) {
	s.mutex.Lock()
	if session, exists := s.sessions[sessionID]; exists {
		if session.Handle != nil {
			session.Handle.Status = types.TranscodeStatusFailed
			session.Handle.Error = err.Error()
		}
		delete(s.sessions, sessionID)
	}
	s.mutex.Unlock()

	if s.logger != nil {
		s.logger.Error("failed to start streaming", "session_id", sessionID, "error", err)
	}
}

// monitorSession monitors a streaming session
func (s *Streamer) monitorSession(session *StreamSession) {
	var pid int
	if session.Process.Process != nil {
		pid = session.Process.Process.Pid
	}

	// Wait for process completion
	err := session.Process.Wait()

	// Unregister from process registry
	if s.processRegistry != nil && pid > 0 {
		s.processRegistry.Unregister(pid)
	}

	// Update status
	if err != nil {
		session.Handle.Status = types.TranscodeStatusFailed
		session.Handle.Error = err.Error()
		if s.logger != nil {
			s.logger.Error("streaming process failed", "session_id", session.ID, "error", err)
		}
	} else {
		session.Handle.Status = types.TranscodeStatusCompleted
		if s.logger != nil {
			s.logger.Info("streaming process completed", "session_id", session.ID)
		}
	}

	// Schedule cleanup
	go func() {
		time.Sleep(5 * time.Minute)
		s.mutex.Lock()
		delete(s.sessions, session.ID)
		s.mutex.Unlock()
		if s.logger != nil {
			s.logger.Debug("cleaned up completed stream session", "session_id", session.ID)
		}
	}()
}