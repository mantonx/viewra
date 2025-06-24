// Package transcoding provides video transcoding capabilities using FFmpeg.
// This implementation uses specialized components for different aspects of
// transcoding to ensure maintainability and reliability. The transcoder handles
// FFmpeg operations while providing a consistent API for video processing.
//
// Architecture Overview:
// - Session Management: Tracks all active transcoding operations
// - Process Monitoring: Ensures proper lifecycle management of FFmpeg processes
// - Argument Building: Generates optimized FFmpeg commands for quality output
// - ABR Generation: Creates adaptive bitrate ladders for streaming
// - Output Validation: Ensures transcoded content meets quality standards
//
// Key Features:
// - Graceful process termination with proper cleanup
// - Automatic validation of DASH/HLS output
// - Progress tracking and monitoring
// - Resource leak prevention through process registry
// - Modular design for easy extension and testing
//
// Example usage:
//   transcoder := transcoding.NewTranscoder("my-transcoder", "My Transcoder", "1.0", "Author", 100)
//   transcoder.SetLogger(logger)
//   
//   req := types.TranscodeRequest{
//       InputPath:  "/path/to/input.mp4",
//       Container:  "dash",
//       EnableABR:  true,
//       Quality:    80,
//   }
//   
//   handle, err := transcoder.StartTranscode(ctx, req)
//   if err != nil {
//       log.Fatal(err)
//   }
//   
//   // Monitor progress
//   progress, _ := transcoder.GetProgress(handle)
//   fmt.Printf("Progress: %.2f%%\n", progress.PercentComplete)
package transcoding

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mantonx/viewra/sdk/transcoding/abr"
	"github.com/mantonx/viewra/sdk/transcoding/ffmpeg"
	"github.com/mantonx/viewra/sdk/transcoding/process"
	"github.com/mantonx/viewra/sdk/transcoding/session"
	"github.com/mantonx/viewra/sdk/transcoding/types"
	"github.com/mantonx/viewra/sdk/transcoding/validation"
)

// Transcoder provides a clean FFmpeg transcoder implementation
// that delegates to specialized components for better maintainability.
type Transcoder struct {
	logger types.Logger
	
	// Configuration
	name        string
	description string
	version     string
	author      string
	priority    int
	
	// Modular components
	sessionManager *session.Manager
	processMonitor *process.Monitor
	processRegistry *process.Registry
	argsBuilder    *ffmpeg.FFmpegArgsBuilder
	abrGenerator   *abr.Generator
}

// NewTranscoder creates a new transcoder  
func NewTranscoder(name, description, version, author string, priority int) *Transcoder {
	return &Transcoder{
		name:        name,
		description: description,
		version:     version,
		author:      author,
		priority:    priority,
	}
}

// SetLogger sets the logger and initializes all components
func (t *Transcoder) SetLogger(logger types.Logger) {
	t.logger = logger
	
	// Initialize modular components
	t.processRegistry = process.GetTranscoderProcessRegistry(logger)
	t.sessionManager = session.NewManager(logger)
	t.processMonitor = process.NewMonitor(logger, t.processRegistry)
	t.argsBuilder = ffmpeg.NewFFmpegArgsBuilder(logger)
	t.abrGenerator = abr.NewGenerator(logger)
}

// GetInfo returns provider information
func (t *Transcoder) GetInfo() types.ProviderInfo {
	return types.ProviderInfo{
		ID:          t.name,
		Name:        t.name,
		Description: t.description,
		Version:     t.version,
		Author:      t.author,
		Priority:    t.priority,
	}
}

// GetSupportedFormats returns supported container formats
func (t *Transcoder) GetSupportedFormats() []types.ContainerFormat {
	return []types.ContainerFormat{
		{
			Format:      "hls",
			MimeType:    "application/vnd.apple.mpegurl",
			Extensions:  []string{".m3u8", ".ts"},
			Description: "HLS Adaptive Streaming",
			Adaptive:    true,
		},
		{
			Format:      "dash",
			MimeType:    "application/dash+xml",
			Extensions:  []string{".mpd", ".m4s"},
			Description: "DASH Adaptive Streaming",
			Adaptive:    true,
		},
		{
			Format:      "mp4",
			MimeType:    "video/mp4",
			Extensions:  []string{".mp4"},
			Description: "MP4 Container",
			Adaptive:    false,
		},
	}
}

// StartTranscode starts a new transcoding session using modular components
func (t *Transcoder) StartTranscode(ctx context.Context, req types.TranscodeRequest) (*types.TranscodeHandle, error) {
	// Create session through session manager
	sess, err := t.sessionManager.CreateSession(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Prepare output directory
	outputDir, outputPath, err := t.prepareOutputPaths(req, sess.ID)
	if err != nil {
		t.sessionManager.RemoveSession(sess.ID)
		return nil, err
	}

	// Update session handle with directory
	sess.Handle.Directory = outputDir

	// Build FFmpeg arguments using the args builder
	args := t.argsBuilder.BuildArgs(req, outputPath)

	// Create and configure FFmpeg command
	cmd := exec.Command("ffmpeg", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group
	}
	cmd.Dir = outputDir

	// Set up logging
	if err := t.setupProcessLogging(cmd, outputDir); err != nil {
		if t.logger != nil {
			t.logger.Warn("failed to setup process logging", "error", err)
		}
	}

	// Log the command
	if t.logger != nil {
		t.logger.Info("starting FFmpeg transcoding",
			"session_id", sess.ID,
			"command", fmt.Sprintf("ffmpeg %v", args),
		)
	}

	// Update session with process
	t.sessionManager.UpdateSession(sess.ID, func(s *session.Session) {
		s.Process = cmd
		s.Status = session.SessionStatusStarting
	})

	// Start the process
	if err := cmd.Start(); err != nil {
		t.handleStartError(sess.ID, err)
		return nil, fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	// Start monitoring the process
	if err := t.processMonitor.MonitorProcess(cmd, sess.ID, t.name); err != nil {
		if t.logger != nil {
			t.logger.Warn("failed to start process monitoring", "error", err)
		}
	}

	// Update session status
	t.sessionManager.UpdateSession(sess.ID, func(s *session.Session) {
		s.Status = session.SessionStatusRunning
	})

	// Start validation for DASH output
	if req.Container == "dash" {
		go t.validateOutput(sess.ID, outputDir)
	}

	// Start progress monitoring
	go t.monitorProgress(sess.ID)

	return sess.Handle, nil
}

// GetProgress returns transcoding progress
func (t *Transcoder) GetProgress(handle *types.TranscodeHandle) (*types.TranscodingProgress, error) {
	if handle == nil {
		return nil, fmt.Errorf("invalid handle")
	}

	sess, err := t.sessionManager.GetSession(handle.SessionID)
	if err != nil {
		return nil, err
	}

	// Calculate progress based on session data
	elapsed := time.Since(sess.StartTime)
	progress := &types.TranscodingProgress{
		PercentComplete: sess.Progress,
		TimeElapsed:     elapsed,
		CurrentSpeed:    1.0,
		AverageSpeed:    1.0,
	}

	return progress, nil
}

// StopTranscode stops a transcoding session
func (t *Transcoder) StopTranscode(handle *types.TranscodeHandle) error {
	if handle == nil {
		return fmt.Errorf("invalid handle")
	}

	// Stop through session manager
	if err := t.sessionManager.StopSession(handle.SessionID); err != nil {
		return err
	}

	// Get session to access process
	sess, err := t.sessionManager.GetSession(handle.SessionID)
	if err != nil {
		return err
	}

	// Stop the process if running
	if sess.Process != nil && sess.Process.Process != nil {
		pid := sess.Process.Process.Pid
		if err := t.processMonitor.StopProcess(pid, 10*time.Second); err != nil {
			if t.logger != nil {
				t.logger.Error("failed to stop process", "pid", pid, "error", err)
			}
		}
	}

	// Update session status
	t.sessionManager.UpdateSession(handle.SessionID, func(s *session.Session) {
		s.Status = session.SessionStatusStopped
	})

	// Schedule session removal
	go func() {
		time.Sleep(5 * time.Minute)
		t.sessionManager.RemoveSession(handle.SessionID)
	}()

	return nil
}

// prepareOutputPaths prepares the output directory and path
func (t *Transcoder) prepareOutputPaths(req types.TranscodeRequest, sessionID string) (string, string, error) {
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
		return "", "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Determine output filename based on container
	var outputPath string
	switch req.Container {
	case "hls":
		outputPath = filepath.Join(outputDir, "playlist.m3u8")
	case "dash":
		outputPath = filepath.Join(outputDir, "manifest.mpd")
	default:
		outputPath = filepath.Join(outputDir, "output.mp4")
	}

	return outputDir, outputPath, nil
}

// setupProcessLogging sets up log files for FFmpeg output
func (t *Transcoder) setupProcessLogging(cmd *exec.Cmd, outputDir string) error {
	stdoutPath := filepath.Join(outputDir, "ffmpeg-stdout.log")
	stderrPath := filepath.Join(outputDir, "ffmpeg-stderr.log")
	
	stdoutFile, err := os.Create(stdoutPath)
	if err != nil {
		return fmt.Errorf("failed to create stdout log: %w", err)
	}
	
	stderrFile, err := os.Create(stderrPath)
	if err != nil {
		stdoutFile.Close()
		return fmt.Errorf("failed to create stderr log: %w", err)
	}
	
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile
	
	return nil
}

// handleStartError handles errors when starting a transcoding session
func (t *Transcoder) handleStartError(sessionID string, err error) {
	t.sessionManager.UpdateSession(sessionID, func(s *session.Session) {
		s.Status = session.SessionStatusFailed
	})
	
	t.sessionManager.RemoveSession(sessionID)
	
	if t.logger != nil {
		t.logger.Error("failed to start transcoding", "session_id", sessionID, "error", err)
	}
}

// validateOutput validates DASH/HLS output after transcoding starts
func (t *Transcoder) validateOutput(sessionID string, outputDir string) {
	// Wait for initial files to be created
	time.Sleep(10 * time.Second)
	
	// Run validation
	result := validation.ValidatePlaybackOutput(outputDir)
	
	if !result.IsValid() {
		if t.logger != nil {
			t.logger.Warn("output validation failed",
				"session_id", sessionID,
				"issues", result.GetIssues(),
			)
		}
		
		// Generate validation report
		reportPath := filepath.Join(outputDir, "validation-report.txt")
		if err := validation.GenerateValidationReport(result, reportPath); err != nil {
			if t.logger != nil {
				t.logger.Error("failed to generate validation report",
					"session_id", sessionID,
					"error", err,
				)
			}
		}
	} else {
		if t.logger != nil {
			t.logger.Info("output validation passed", "session_id", sessionID)
		}
	}
}

// monitorProgress monitors transcoding progress
func (t *Transcoder) monitorProgress(sessionID string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sess, err := t.sessionManager.GetSession(sessionID)
			if err != nil {
				return // Session removed
			}

			// Check if process is still running
			if sess.Process != nil && sess.Process.ProcessState != nil {
				// Process completed
				exitCode := sess.Process.ProcessState.ExitCode()
				
				if exitCode == 0 {
					t.sessionManager.UpdateSession(sessionID, func(s *session.Session) {
						s.Progress = 100.0
						s.Status = session.SessionStatusComplete
					})
				} else {
					t.sessionManager.UpdateSession(sessionID, func(s *session.Session) {
						s.Status = session.SessionStatusFailed
					})
				}
				
				return
			}

			// TODO: Parse FFmpeg progress from stderr log
			// For now, just increment progress
			t.sessionManager.UpdateSession(sessionID, func(s *session.Session) {
				if s.Progress < 95 {
					s.Progress += 5
				}
			})
		}
	}
}

// Streaming methods (not implemented in this version)
func (t *Transcoder) StartStream(ctx context.Context, req types.TranscodeRequest) (*types.StreamHandle, error) {
	return nil, fmt.Errorf("streaming not implemented")
}

func (t *Transcoder) GetStream(handle *types.StreamHandle) (io.ReadCloser, error) {
	return nil, fmt.Errorf("streaming not implemented")
}

func (t *Transcoder) StopStream(handle *types.StreamHandle) error {
	return fmt.Errorf("streaming not implemented")
}

// Dashboard methods
func (t *Transcoder) GetDashboardSections() []types.DashboardSection {
	return []types.DashboardSection{
		{
			ID:          "overview",
			Title:       "Overview",
			Type:        "stats",
			Description: "Transcoding session statistics",
		},
		{
			ID:          "sessions",
			Title:       "Active Sessions",
			Type:        "table",
			Description: "Currently running transcoding sessions",
		},
	}
}

func (t *Transcoder) GetDashboardData(sectionID string) (interface{}, error) {
	switch sectionID {
	case "overview":
		stats := t.sessionManager.GetSessionStats()
		stats["provider"] = t.name
		stats["version"] = t.version
		return stats, nil
		
	case "sessions":
		sessions := t.sessionManager.GetAllSessions()
		return sessions, nil
		
	default:
		return nil, fmt.Errorf("unknown section: %s", sectionID)
	}
}

func (t *Transcoder) ExecuteDashboardAction(actionID string, params map[string]interface{}) error {
	switch actionID {
	case "stop_session":
		sessionID, ok := params["session_id"].(string)
		if !ok {
			return fmt.Errorf("missing session_id parameter")
		}
		return t.sessionManager.StopSession(sessionID)
		
	case "cleanup_stale":
		t.sessionManager.CleanupStaleSessions(30 * time.Minute)
		return nil
		
	default:
		return fmt.Errorf("unknown action: %s", actionID)
	}
}