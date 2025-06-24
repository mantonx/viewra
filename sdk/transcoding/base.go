// Package transcoding provides base functionality for building transcoding providers.
// The BaseTranscoder can be embedded in plugin implementations to provide common
// transcoding operations while allowing customization of specific behaviors.
//
// This base implementation handles:
// - Session lifecycle management
// - Process monitoring and cleanup
// - Progress tracking
// - Resource management
// - Error handling and recovery
//
// Plugin developers can embed BaseTranscoder and override specific methods
// to customize behavior while leveraging the robust foundation.
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

	"github.com/mantonx/viewra/sdk/transcoding/dashboard"
	"github.com/mantonx/viewra/sdk/transcoding/ffmpeg"
	"github.com/mantonx/viewra/sdk/transcoding/process"
	"github.com/mantonx/viewra/sdk/transcoding/session"
	"github.com/mantonx/viewra/sdk/transcoding/streaming"
	"github.com/mantonx/viewra/sdk/transcoding/types"
	"github.com/mantonx/viewra/sdk/transcoding/utils"
	"github.com/mantonx/viewra/sdk/transcoding/validation"
)

// BaseTranscoder provides common transcoding functionality that can be embedded in plugins
type BaseTranscoder struct {
	// Core components
	logger          types.Logger
	sessionManager  *session.Manager
	processMonitor  *process.Monitor
	processRegistry *process.ProcessRegistry
	argsBuilder     *ffmpeg.FFmpegArgsBuilder
	streamer        *streaming.Streamer
	dashboardProvider *dashboard.Provider
	
	// Configuration
	name        string
	description string
	version     string
	author      string
	priority    int
	ffmpegPath  string
}

// NewBaseTranscoder creates a new base transcoder
func NewBaseTranscoder(name, description, version, author string, priority int) *BaseTranscoder {
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
		ffmpegPath:  ffmpegPath,
	}
}

// SetLogger sets the logger and initializes all components
func (bt *BaseTranscoder) SetLogger(logger types.Logger) {
	bt.logger = logger
	
	// Initialize components
	bt.processRegistry = process.GetTranscoderProcessRegistry(logger)
	bt.sessionManager = session.NewManager(logger)
	bt.processMonitor = process.NewMonitor(logger, bt.processRegistry)
	bt.argsBuilder = ffmpeg.NewFFmpegArgsBuilder(logger)
	bt.streamer = streaming.NewStreamer(logger, bt.ffmpegPath, bt.processRegistry)
	bt.dashboardProvider = dashboard.NewProvider(logger)
	
	// Register default dashboard sections
	bt.setupDefaultDashboard()
}

// GetInfo returns provider information
func (bt *BaseTranscoder) GetInfo() types.ProviderInfo {
	return types.ProviderInfo{
		ID:          bt.name,
		Name:        bt.name,
		Description: bt.description,
		Version:     bt.version,
		Author:      bt.author,
		Priority:    bt.priority,
	}
}

// GetSupportedFormats returns supported container formats
func (bt *BaseTranscoder) GetSupportedFormats() []types.ContainerFormat {
	return []types.ContainerFormat{
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
func (bt *BaseTranscoder) StartTranscode(ctx context.Context, req types.TranscodeRequest) (*types.TranscodeHandle, error) {
	// Create session through session manager
	sess, err := bt.sessionManager.CreateSession(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Prepare output paths
	outputDir, outputPath, err := bt.prepareOutputPaths(req, sess.ID)
	if err != nil {
		bt.sessionManager.RemoveSession(sess.ID)
		return nil, err
	}

	// Update session handle with directory
	sess.Handle.Directory = outputDir

	// Build FFmpeg arguments
	args := bt.argsBuilder.BuildArgs(req, outputPath)

	// Validate FFmpeg arguments before execution
	if err := ffmpeg.ValidateArgs(args); err != nil {
		bt.sessionManager.RemoveSession(sess.ID)
		return nil, fmt.Errorf("invalid FFmpeg arguments: %w", err)
	}

	// Create command
	cmd := exec.Command(bt.ffmpegPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group
	}
	cmd.Dir = outputDir

	// Set up logging
	if err := bt.setupProcessLogging(cmd, outputDir, sess.ID); err != nil {
		if bt.logger != nil {
			bt.logger.Warn("failed to setup process logging", "error", err)
		}
	}

	// Log the command
	if bt.logger != nil {
		bt.logger.Info("starting FFmpeg transcoding",
			"session_id", sess.ID,
			"command", fmt.Sprintf("ffmpeg %v", args),
		)
	}

	// Update session with process
	bt.sessionManager.UpdateSession(sess.ID, func(s *session.Session) {
		s.Process = cmd
		s.Status = session.SessionStatusStarting
	})

	// Start the process
	if err := cmd.Start(); err != nil {
		bt.handleStartError(sess.ID, err)
		return nil, fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	// Start monitoring
	if err := bt.processMonitor.MonitorProcess(cmd, sess.ID, bt.name); err != nil {
		if bt.logger != nil {
			bt.logger.Warn("failed to start process monitoring", "error", err)
		}
	}

	// Update status
	bt.sessionManager.UpdateSession(sess.ID, func(s *session.Session) {
		s.Status = session.SessionStatusRunning
	})

	// Start validation for DASH output
	if req.Container == "dash" {
		go bt.validateOutput(sess.ID, outputDir)
	}

	return sess.Handle, nil
}

// GetProgress returns transcoding progress
func (bt *BaseTranscoder) GetProgress(handle *types.TranscodeHandle) (*types.TranscodingProgress, error) {
	if handle == nil {
		return nil, fmt.Errorf("invalid handle")
	}

	sess, err := bt.sessionManager.GetSession(handle.SessionID)
	if err != nil {
		return nil, err
	}

	// Calculate progress
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
func (bt *BaseTranscoder) StopTranscode(handle *types.TranscodeHandle) error {
	if handle == nil {
		return fmt.Errorf("invalid handle")
	}

	// Stop through session manager
	if err := bt.sessionManager.StopSession(handle.SessionID); err != nil {
		return err
	}

	// Get session to access process
	sess, err := bt.sessionManager.GetSession(handle.SessionID)
	if err != nil {
		return err
	}

	// Stop the process if running
	if sess.Process != nil && sess.Process.Process != nil {
		pid := sess.Process.Process.Pid
		if err := bt.processMonitor.StopProcess(pid, 10*time.Second); err != nil {
			if bt.logger != nil {
				bt.logger.Error("failed to stop process", "pid", pid, "error", err)
			}
		}
	}

	// Update status and schedule cleanup
	bt.sessionManager.UpdateSession(handle.SessionID, func(s *session.Session) {
		s.Status = session.SessionStatusStopped
	})

	// Schedule session removal
	go func() {
		time.Sleep(5 * time.Minute)
		bt.sessionManager.RemoveSession(handle.SessionID)
	}()

	return nil
}

// Streaming methods delegate to the streamer component
func (bt *BaseTranscoder) StartStream(ctx context.Context, req types.TranscodeRequest) (*types.StreamHandle, error) {
	return bt.streamer.StartStream(ctx, req)
}

func (bt *BaseTranscoder) GetStream(handle *types.StreamHandle) (io.ReadCloser, error) {
	return bt.streamer.GetManifest(handle)
}

func (bt *BaseTranscoder) StopStream(handle *types.StreamHandle) error {
	return bt.streamer.StopStream(handle)
}

// Dashboard methods delegate to the dashboard provider
func (bt *BaseTranscoder) GetDashboardSections() []types.DashboardSection {
	return bt.dashboardProvider.GetSections()
}

func (bt *BaseTranscoder) GetDashboardData(sectionID string) (interface{}, error) {
	return bt.dashboardProvider.GetSectionData(sectionID)
}

func (bt *BaseTranscoder) ExecuteDashboardAction(actionID string, params map[string]interface{}) error {
	return bt.dashboardProvider.ExecuteAction(actionID, params)
}

// prepareOutputPaths prepares the output directory and path
func (bt *BaseTranscoder) prepareOutputPaths(req types.TranscodeRequest, sessionID string) (string, string, error) {
	var outputDir string
	if req.OutputPath != "" {
		outputDir = req.OutputPath
	} else {
		baseDir := "/app/viewra-data/transcoding"
		if envDir := os.Getenv("VIEWRA_TRANSCODING_DIR"); envDir != "" {
			baseDir = envDir
		}
		outputDir = filepath.Join(baseDir, fmt.Sprintf("%s_%s_%s", req.Container, bt.name, sessionID))
	}
	
	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Determine output filename
	var outputPath string
	switch req.Container {
	case "dash":
		outputPath = filepath.Join(outputDir, "manifest.mpd")
	case "hls":
		outputPath = filepath.Join(outputDir, "playlist.m3u8")
	default:
		outputPath = filepath.Join(outputDir, fmt.Sprintf("output.%s", req.Container))
	}

	return outputDir, outputPath, nil
}

// setupProcessLogging sets up logging for the FFmpeg process
func (bt *BaseTranscoder) setupProcessLogging(cmd *exec.Cmd, outputDir string, sessionID string) error {
	// Try debug logging first
	debugLog, err := utils.SetupDebugLogging(sessionID, outputDir, cmd.Args)
	if err == nil && debugLog != nil {
		cmd.Stdout = debugLog
		cmd.Stderr = debugLog
		return nil
	}

	// Fall back to standard logging
	stdoutFile, stderrFile, err := utils.SetupProcessLogging(outputDir, sessionID)
	if err != nil {
		// Fall back to logger-based logging
		cmd.Stdout = &utils.LogWriter{Logger: bt.logger, Prefix: "[stdout] "}
		cmd.Stderr = &utils.LogWriter{Logger: bt.logger, Prefix: "[stderr] "}
		return err
	}

	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile
	return nil
}

// handleStartError handles errors when starting a transcoding session
func (bt *BaseTranscoder) handleStartError(sessionID string, err error) {
	bt.sessionManager.UpdateSession(sessionID, func(s *session.Session) {
		s.Status = session.SessionStatusFailed
	})
	
	bt.sessionManager.RemoveSession(sessionID)
	
	if bt.logger != nil {
		bt.logger.Error("failed to start transcoding", "session_id", sessionID, "error", err)
	}
}

// validateOutput validates DASH/HLS output
func (bt *BaseTranscoder) validateOutput(sessionID string, outputDir string) {
	// Wait for initial files
	time.Sleep(10 * time.Second)
	
	// Run validation
	result := validation.ValidatePlaybackOutput(outputDir)
	
	if !result.IsValid() {
		if bt.logger != nil {
			bt.logger.Warn("output validation failed",
				"session_id", sessionID,
				"issues", result.GetIssues(),
			)
		}
		
		// Generate report
		reportPath := filepath.Join(outputDir, "validation-report.txt")
		if err := validation.GenerateValidationReport(result, reportPath); err != nil {
			if bt.logger != nil {
				bt.logger.Error("failed to generate validation report",
					"session_id", sessionID,
					"error", err,
				)
			}
		}
	} else {
		if bt.logger != nil {
			bt.logger.Info("output validation passed", "session_id", sessionID)
		}
	}
}

// setupDefaultDashboard sets up default dashboard sections
func (bt *BaseTranscoder) setupDefaultDashboard() {
	// Overview stats
	bt.dashboardProvider.RegisterStatsSection(
		"overview",
		"Overview",
		"General statistics and status",
		func() (interface{}, error) {
			return map[string]interface{}{
				"provider":        bt.name,
				"version":         bt.version,
				"active_sessions": bt.sessionManager.GetSessionCount(),
			}, nil
		},
	)

	// Active sessions table
	bt.dashboardProvider.RegisterTableSection(
		"sessions",
		"Active Sessions",
		"Currently running transcoding sessions",
		func() (interface{}, error) {
			sessions := bt.sessionManager.GetAllSessions()
			return sessions, nil
		},
	)

	// Register actions
	bt.dashboardProvider.RegisterAction("stop_all", func(params map[string]interface{}) error {
		bt.sessionManager.StopAllSessions()
		return nil
	})

	bt.dashboardProvider.RegisterAction("cleanup_stale", func(params map[string]interface{}) error {
		bt.sessionManager.CleanupStaleSessions(30 * time.Minute)
		return nil
	})
}