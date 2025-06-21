package plugin

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
	"time"

	"github.com/google/uuid"
	"github.com/mantonx/viewra/plugins/ffmpeg_transcoder/internal/config"
	"github.com/mantonx/viewra/plugins/ffmpeg_transcoder/internal/services"
	"github.com/mantonx/viewra/plugins/ffmpeg_transcoder/internal/types"
	plugins "github.com/mantonx/viewra/sdk"
)

// FFmpegTranscoderPlugin implements the plugin interface
type FFmpegTranscoderPlugin struct {
	logger             plugins.Logger
	config             *config.Config
	sessionManager     services.SessionManager
	cleanupService     services.CleanupService
	hardwareDetector   services.HardwareDetector
	transcodingService *services.DefaultTranscodingService

	ctx           context.Context
	cancel        context.CancelFunc
	cleanupTicker *time.Ticker
	mutex         sync.RWMutex
	activeHandles map[string]*plugins.TranscodeHandle
	streamHandles map[string]*plugins.StreamHandle
}

// New creates a new FFmpeg transcoder plugin instance
func New() *FFmpegTranscoderPlugin {
	return &FFmpegTranscoderPlugin{
		config:        config.DefaultConfig(),
		activeHandles: make(map[string]*plugins.TranscodeHandle),
		streamHandles: make(map[string]*plugins.StreamHandle),
	}
}

// Initialize initializes the plugin with the provided context
func (p *FFmpegTranscoderPlugin) Initialize(ctx *plugins.PluginContext) error {
	p.logger = ctx.Logger
	p.ctx, p.cancel = context.WithCancel(context.Background())

	p.logger.Info("initializing FFmpeg transcoder plugin",
		"version", Version,
		"build", BuildDate,
	)

	// Load configuration
	if err := p.loadConfiguration(ctx); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize services
	if err := p.initializeServices(); err != nil {
		return fmt.Errorf("failed to initialize services: %w", err)
	}

	// Detect hardware capabilities
	if p.config.Hardware.Enabled {
		if hwInfo, err := p.hardwareDetector.DetectHardware(); err == nil {
			p.logger.Info("hardware acceleration detected",
				"type", hwInfo.Type,
				"encoders", hwInfo.Encoders,
			)
		}
	}

	return nil
}

// Start starts the plugin operation
func (p *FFmpegTranscoderPlugin) Start() error {
	p.logger.Info("starting FFmpeg transcoder plugin")

	// Start cleanup service
	if p.config.Cleanup.IntervalMinutes > 0 {
		p.startCleanupService()
	}

	return nil
}

// Stop gracefully shuts down the plugin
func (p *FFmpegTranscoderPlugin) Stop() error {
	p.logger.Info("stopping FFmpeg transcoder plugin")

	// Cancel context
	if p.cancel != nil {
		p.cancel()
	}

	// Stop cleanup ticker
	if p.cleanupTicker != nil {
		p.cleanupTicker.Stop()
	}

	// Stop all active handles
	p.mutex.Lock()
	for _, handle := range p.activeHandles {
		if handle.CancelFunc != nil {
			handle.CancelFunc()
		}
	}
	p.mutex.Unlock()

	return nil
}

// Info returns plugin information
func (p *FFmpegTranscoderPlugin) Info() (*plugins.PluginInfo, error) {
	return &plugins.PluginInfo{
		ID:          "ffmpeg_transcoder",
		Name:        "FFmpeg Transcoder",
		Version:     Version,
		Type:        "transcoder",
		Description: "High-performance video transcoding using FFmpeg with hardware acceleration support",
		Author:      "Viewra Team",
	}, nil
}

// Health checks the plugin health
func (p *FFmpegTranscoderPlugin) Health() error {
	// Check if services are initialized
	if p.transcodingService == nil {
		return fmt.Errorf("transcoding service not initialized")
	}

	// Check active sessions
	sessions, err := p.sessionManager.ListActiveSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	// Check if we're over capacity
	if len(sessions) >= p.config.Sessions.MaxConcurrent {
		return fmt.Errorf("at maximum capacity: %d/%d sessions", len(sessions), p.config.Sessions.MaxConcurrent)
	}

	return nil
}

// TranscodingProvider returns the transcoding provider interface
func (p *FFmpegTranscoderPlugin) TranscodingProvider() plugins.TranscodingProvider {
	return p
}

// ========================================================================
// TranscodingProvider Implementation
// ========================================================================

// GetInfo returns provider information
func (p *FFmpegTranscoderPlugin) GetInfo() plugins.ProviderInfo {
	return plugins.ProviderInfo{
		ID:          "ffmpeg_transcoder",
		Name:        "FFmpeg Transcoder",
		Description: "High-performance transcoding with hardware acceleration support",
		Version:     Version,
		Author:      "Viewra Team",
		Priority:    p.config.Core.Priority,
	}
}

// GetSupportedFormats returns supported container formats
func (p *FFmpegTranscoderPlugin) GetSupportedFormats() []plugins.ContainerFormat {
	return []plugins.ContainerFormat{
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

// GetHardwareAccelerators returns available hardware accelerators
func (p *FFmpegTranscoderPlugin) GetHardwareAccelerators() []plugins.HardwareAccelerator {
	if p.hardwareDetector == nil {
		return []plugins.HardwareAccelerator{}
	}

	hwInfo, err := p.hardwareDetector.DetectHardware()
	if err != nil || !hwInfo.Available {
		return []plugins.HardwareAccelerator{}
	}

	// Map internal hardware info to plugin format
	var accelerators []plugins.HardwareAccelerator

	switch hwInfo.Type {
	case "nvidia":
		accelerators = append(accelerators, plugins.HardwareAccelerator{
			Type:        "nvidia",
			ID:          "nvenc",
			Name:        "NVIDIA NVENC",
			Available:   true,
			DeviceCount: len(hwInfo.Encoders),
		})
	case "intel":
		accelerators = append(accelerators, plugins.HardwareAccelerator{
			Type:        "intel",
			ID:          "qsv",
			Name:        "Intel Quick Sync",
			Available:   true,
			DeviceCount: 1,
		})
	case "amd":
		accelerators = append(accelerators, plugins.HardwareAccelerator{
			Type:        "amd",
			ID:          "amf",
			Name:        "AMD AMF",
			Available:   true,
			DeviceCount: 1,
		})
	}

	return accelerators
}

// GetQualityPresets returns available quality presets
func (p *FFmpegTranscoderPlugin) GetQualityPresets() []plugins.QualityPreset {
	return []plugins.QualityPreset{
		{
			ID:          "ultrafast",
			Name:        "Ultra Fast",
			Description: "Fastest encoding, larger files",
			Quality:     30,
			SpeedRating: 10,
			SizeRating:  2,
		},
		{
			ID:          "fast",
			Name:        "Fast",
			Description: "Fast encoding, reasonable quality",
			Quality:     50,
			SpeedRating: 8,
			SizeRating:  4,
		},
		{
			ID:          "balanced",
			Name:        "Balanced",
			Description: "Good balance of speed and quality",
			Quality:     70,
			SpeedRating: 5,
			SizeRating:  6,
		},
		{
			ID:          "quality",
			Name:        "High Quality",
			Description: "Better quality, slower encoding",
			Quality:     85,
			SpeedRating: 3,
			SizeRating:  8,
		},
		{
			ID:          "best",
			Name:        "Best Quality",
			Description: "Maximum quality, very slow",
			Quality:     95,
			SpeedRating: 1,
			SizeRating:  10,
		},
	}
}

// StartTranscode starts a new transcoding operation
func (p *FFmpegTranscoderPlugin) StartTranscode(ctx context.Context, req plugins.TranscodeRequest) (*plugins.TranscodeHandle, error) {
	// Generate session ID if not provided
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	// Create context with cancel for this handle
	handleCtx, cancelFunc := context.WithCancel(ctx)

	// Determine output directory and file
	pluginID := "ffmpeg_transcoder"
	sessionDir := fmt.Sprintf("/app/viewra-data/transcoding/%s_%s_%s", req.Container, pluginID, sessionID)

	// Create output directory
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		cancelFunc()
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	// Determine output file based on container
	var outputFile string
	switch req.Container {
	case "dash":
		outputFile = filepath.Join(sessionDir, "manifest.mpd")
	case "hls":
		outputFile = filepath.Join(sessionDir, "playlist.m3u8")
	default:
		outputFile = filepath.Join(sessionDir, fmt.Sprintf("output.%s", req.Container))
	}

	// Convert to internal service request
	serviceReq := &services.TranscodingRequest{
		InputFile:  req.InputPath,
		OutputFile: outputFile,
		Settings: services.JobSettings{
			VideoCodec:   req.VideoCodec,
			AudioCodec:   req.AudioCodec,
			Container:    req.Container,
			Quality:      mapQualityToCRF(req.Quality, req.VideoCodec),
			Preset:       mapSpeedPriorityToPreset(req.SpeedPriority),
			AudioBitrate: 128, // Default
		},
	}

	// Handle seek requests
	if req.Seek > 0 {
		if serviceReq.Environment == nil {
			serviceReq.Environment = make(map[string]string)
		}
		serviceReq.Environment["SEEK_START"] = fmt.Sprintf("%d", int(req.Seek.Seconds()))
	}

	// Apply resolution if specified
	if req.Resolution != nil {
		if serviceReq.Environment == nil {
			serviceReq.Environment = make(map[string]string)
		}
		serviceReq.Environment["RESOLUTION"] = fmt.Sprintf("%dx%d", req.Resolution.Width, req.Resolution.Height)
	}

	// Start the job
	jobResp, err := p.transcodingService.StartJob(handleCtx, serviceReq)
	if err != nil {
		cancelFunc()
		return nil, fmt.Errorf("failed to start transcoding job: %w", err)
	}

	// Create handle
	handle := &plugins.TranscodeHandle{
		SessionID:   sessionID,
		Provider:    pluginID,
		StartTime:   time.Now(),
		Directory:   sessionDir,
		Context:     handleCtx,
		CancelFunc:  cancelFunc,
		PrivateData: jobResp.JobID, // Store internal job ID
	}

	// Store handle
	p.mutex.Lock()
	p.activeHandles[sessionID] = handle
	p.mutex.Unlock()

	// Create session in session manager
	if _, err := p.sessionManager.CreateSession(sessionID, req.InputPath, req.Container); err != nil {
		p.logger.Warn("failed to create session in manager", "error", err)
	}

	p.logger.Info("started transcoding",
		"session_id", sessionID,
		"input", req.InputPath,
		"output", outputFile,
		"container", req.Container,
	)

	return handle, nil
}

// GetProgress returns transcoding progress
func (p *FFmpegTranscoderPlugin) GetProgress(handle *plugins.TranscodeHandle) (*plugins.TranscodingProgress, error) {
	if handle == nil {
		return nil, fmt.Errorf("invalid handle")
	}

	// Try to get progress from the job first
	if jobID, ok := handle.PrivateData.(string); ok && jobID != "" {
		job, err := p.transcodingService.GetJobStatus(context.Background(), jobID)
		if err == nil && job != nil {
			// Convert internal job progress to plugin progress
			progress := &plugins.TranscodingProgress{
				PercentComplete: job.Progress.Percentage,
				TimeElapsed:     time.Since(handle.StartTime),
				CurrentSpeed:    job.Progress.Speed,
				AverageSpeed:    job.Progress.Speed, // Could track average over time
			}

			// Calculate time remaining based on progress
			if progress.PercentComplete > 0 && progress.PercentComplete < 100 && progress.CurrentSpeed > 0 {
				totalTime := float64(progress.TimeElapsed) / (progress.PercentComplete / 100)
				remaining := totalTime - float64(progress.TimeElapsed)
				progress.TimeRemaining = time.Duration(remaining)
			}

			// Update session progress
			p.sessionManager.UpdateSession(handle.SessionID, func(s *types.Session) error {
				s.Progress = progress.PercentComplete / 100 // Convert to 0-1 range
				return nil
			})

			return progress, nil
		}
	}

	// Fallback to session-based progress
	session, err := p.sessionManager.GetSession(handle.SessionID)
	if err != nil {
		// Create basic progress based on time
		elapsed := time.Since(handle.StartTime)
		return &plugins.TranscodingProgress{
			PercentComplete: 0,
			TimeElapsed:     elapsed,
			CurrentSpeed:    1.0,
			AverageSpeed:    1.0,
		}, nil
	}

	// Convert internal progress
	progress := &plugins.TranscodingProgress{
		PercentComplete: session.Progress * 100,
		TimeElapsed:     time.Since(session.StartTime),
		CurrentSpeed:    1.0,
		AverageSpeed:    1.0,
	}

	// Estimate time remaining
	if progress.PercentComplete > 0 && progress.PercentComplete < 100 {
		totalTime := float64(progress.TimeElapsed) / (progress.PercentComplete / 100)
		remaining := totalTime - float64(progress.TimeElapsed)
		progress.TimeRemaining = time.Duration(remaining)
	}

	return progress, nil
}

// StopTranscode stops a transcoding operation
func (p *FFmpegTranscoderPlugin) StopTranscode(handle *plugins.TranscodeHandle) error {
	if handle == nil {
		return fmt.Errorf("invalid handle")
	}

	// Cancel the context
	if handle.CancelFunc != nil {
		handle.CancelFunc()
	}

	// Stop the job using internal ID
	if jobID, ok := handle.PrivateData.(string); ok {
		if err := p.transcodingService.StopJob(context.Background(), jobID); err != nil {
			p.logger.Warn("failed to stop job", "job_id", jobID, "error", err)
		}
	}

	// Remove from active handles
	p.mutex.Lock()
	delete(p.activeHandles, handle.SessionID)
	p.mutex.Unlock()

	// Remove from session manager
	if err := p.sessionManager.RemoveSession(handle.SessionID); err != nil {
		p.logger.Warn("failed to remove session", "session_id", handle.SessionID, "error", err)
	}

	return nil
}

// GetDashboardSections returns custom dashboard sections
func (p *FFmpegTranscoderPlugin) GetDashboardSections() []plugins.DashboardSection {
	return []plugins.DashboardSection{
		{
			ID:          "ffmpeg_status",
			Title:       "FFmpeg Status",
			Description: "Current transcoding operations and statistics",
			Icon:        "film",
			Priority:    1,
		},
		{
			ID:          "hardware_info",
			Title:       "Hardware Acceleration",
			Description: "Available hardware encoders",
			Icon:        "cpu",
			Priority:    2,
		},
	}
}

// GetDashboardData returns data for a dashboard section
func (p *FFmpegTranscoderPlugin) GetDashboardData(sectionID string) (interface{}, error) {
	switch sectionID {
	case "ffmpeg_status":
		sessions, _ := p.sessionManager.ListActiveSessions()
		return map[string]interface{}{
			"active_sessions": len(sessions),
			"max_sessions":    p.config.Sessions.MaxConcurrent,
			"sessions":        sessions,
		}, nil

	case "hardware_info":
		hwInfo, _ := p.hardwareDetector.DetectHardware()
		return hwInfo, nil

	default:
		return nil, fmt.Errorf("unknown section: %s", sectionID)
	}
}

// ExecuteDashboardAction executes a dashboard action
func (p *FFmpegTranscoderPlugin) ExecuteDashboardAction(actionID string, params map[string]interface{}) error {
	switch actionID {
	case "stop_session":
		sessionID, ok := params["session_id"].(string)
		if !ok {
			return fmt.Errorf("missing session_id parameter")
		}

		p.mutex.RLock()
		handle, exists := p.activeHandles[sessionID]
		p.mutex.RUnlock()

		if !exists {
			return fmt.Errorf("session not found: %s", sessionID)
		}

		return p.StopTranscode(handle)

	default:
		return fmt.Errorf("unknown action: %s", actionID)
	}
}

// ========================================================================
// Streaming Methods Implementation
// ========================================================================

// StartStream starts a streaming transcoding operation
func (p *FFmpegTranscoderPlugin) StartStream(ctx context.Context, req plugins.TranscodeRequest) (*plugins.StreamHandle, error) {
	// Generate session ID if not provided
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	p.logger.Info("starting streaming transcode",
		"session_id", sessionID,
		"input", req.InputPath,
		"container", req.Container,
		"quality", req.Quality,
	)

	// Create context with cancel for this handle
	handleCtx, cancelFunc := context.WithCancel(ctx)

	// Build FFmpeg command for streaming
	args := p.buildStreamingArgs(req)

	// Create the command
	cmd := exec.CommandContext(handleCtx, p.config.FFmpeg.BinaryPath, args...)

	// Get stdout pipe for streaming
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancelFunc()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		cancelFunc()
		return nil, fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	// Create stream handle
	handle := &plugins.StreamHandle{
		SessionID:  sessionID,
		Provider:   "ffmpeg_transcoder",
		StartTime:  time.Now(),
		ProcessID:  cmd.Process.Pid,
		Context:    handleCtx,
		CancelFunc: cancelFunc,
		PrivateData: &streamPrivateData{
			cmd:    cmd,
			stdout: stdout,
		},
	}

	// Store handle
	p.mutex.Lock()
	p.streamHandles[sessionID] = handle
	p.mutex.Unlock()

	// Monitor process in background
	go func() {
		err := cmd.Wait()
		if err != nil && !strings.Contains(err.Error(), "signal: killed") {
			p.logger.Warn("FFmpeg streaming process ended with error",
				"session_id", sessionID,
				"error", err,
			)
		}
		// Clean up handle when process ends
		p.mutex.Lock()
		delete(p.streamHandles, sessionID)
		p.mutex.Unlock()
	}()

	p.logger.Info("streaming transcode started",
		"session_id", sessionID,
		"pid", cmd.Process.Pid,
	)

	return handle, nil
}

// GetStream returns the stream reader for a streaming handle
func (p *FFmpegTranscoderPlugin) GetStream(handle *plugins.StreamHandle) (io.ReadCloser, error) {
	if handle == nil {
		return nil, fmt.Errorf("invalid handle")
	}

	// Get private data
	data, ok := handle.PrivateData.(*streamPrivateData)
	if !ok {
		return nil, fmt.Errorf("invalid private data")
	}

	return data.stdout, nil
}

// StopStream stops a streaming transcoding operation
func (p *FFmpegTranscoderPlugin) StopStream(handle *plugins.StreamHandle) error {
	if handle == nil {
		return fmt.Errorf("invalid handle")
	}

	p.logger.Info("stopping streaming transcode", "session_id", handle.SessionID)

	// Cancel the context
	if handle.CancelFunc != nil {
		handle.CancelFunc()
	}

	// Kill the process if it's still running
	if data, ok := handle.PrivateData.(*streamPrivateData); ok && data.cmd != nil {
		if data.cmd.Process != nil {
			// Try graceful termination first
			if err := data.cmd.Process.Signal(os.Interrupt); err != nil {
				// Force kill if graceful fails
				data.cmd.Process.Kill()
			}
		}
		// Close stdout
		if data.stdout != nil {
			data.stdout.Close()
		}
	}

	// Remove from active handles
	p.mutex.Lock()
	delete(p.streamHandles, handle.SessionID)
	p.mutex.Unlock()

	return nil
}

// buildStreamingArgs builds FFmpeg arguments for streaming
func (p *FFmpegTranscoderPlugin) buildStreamingArgs(req plugins.TranscodeRequest) []string {
	var args []string

	// Input options
	args = append(args,
		"-i", req.InputPath,
		"-f", "mp4", // Force MP4 format for streaming
		"-movflags", "frag_keyframe+empty_moov+faststart", // Enable streaming-friendly MP4
	)

	// Video encoding
	videoCodec := req.VideoCodec
	if videoCodec == "" {
		videoCodec = "h264" // Default to H.264
	}

	args = append(args,
		"-c:v", videoCodec,
		"-preset", mapSpeedPriorityToPreset(req.SpeedPriority),
		"-crf", strconv.Itoa(mapQualityToCRF(req.Quality, videoCodec)),
	)

	// Apply resolution if specified
	if req.Resolution != nil {
		args = append(args,
			"-vf", fmt.Sprintf("scale=%d:%d", req.Resolution.Width, req.Resolution.Height),
		)
	}

	// Audio encoding
	audioCodec := req.AudioCodec
	if audioCodec == "" {
		audioCodec = "aac" // Default to AAC
	}

	args = append(args,
		"-c:a", audioCodec,
		"-b:a", "128k", // Default audio bitrate
		"-ac", "2", // Stereo
	)

	// Streaming optimization
	args = append(args,
		"-g", "30", // GOP size for faster seeking
		"-keyint_min", "30", // Minimum keyframe interval
		"-sc_threshold", "0", // Disable scene change detection
		"-bufsize", "1000k", // Buffer size
		"-maxrate", "3000k", // Max bitrate
		"pipe:1", // Output to stdout
	)

	return args
}

// streamPrivateData holds FFmpeg-specific data for streaming
type streamPrivateData struct {
	cmd    *exec.Cmd
	stdout io.ReadCloser
}

// ========================================================================
// Helper Functions
// ========================================================================

// mapQualityToCRF converts 0-100 quality to FFmpeg CRF value
func mapQualityToCRF(quality int, codec string) int {
	// Invert quality scale and map to CRF
	// Higher quality = lower CRF
	crf := 51 - int(float64(quality)/100.0*51)

	// Adjust for different codecs
	switch codec {
	case "h265", "hevc":
		crf = crf + 5 // HEVC uses higher CRF values
	case "vp9":
		crf = int(63 - float64(quality)/100.0*63) // VP9 uses 0-63 scale
	}

	// Clamp to valid range
	if crf < 0 {
		crf = 0
	}
	if codec == "vp9" && crf > 63 {
		crf = 63
	} else if crf > 51 {
		crf = 51
	}

	return crf
}

// mapSpeedPriorityToPreset converts speed priority to FFmpeg preset
func mapSpeedPriorityToPreset(priority plugins.SpeedPriority) string {
	switch priority {
	case plugins.SpeedPriorityFastest:
		return "ultrafast"
	case plugins.SpeedPriorityBalanced:
		return "medium"
	case plugins.SpeedPriorityQuality:
		return "slow"
	default:
		return "medium"
	}
}

// loadConfiguration loads the plugin configuration
func (p *FFmpegTranscoderPlugin) loadConfiguration(ctx *plugins.PluginContext) error {
	// TODO: Load from CUE configuration system
	// For now, use defaults and validate
	if err := p.config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	return nil
}

// initializeServices initializes all internal services
func (p *FFmpegTranscoderPlugin) initializeServices() error {
	// Initialize session manager
	p.sessionManager = services.NewSessionManager(p.logger)

	// Initialize hardware detector
	p.hardwareDetector = services.NewHardwareDetector(p.logger)

	// Initialize cleanup service
	p.cleanupService = services.NewCleanupService(p.logger, p.config)

	// Initialize transcoding service
	p.transcodingService = services.NewTranscodingService(p.config)

	return nil
}

// startCleanupService starts the periodic cleanup service
func (p *FFmpegTranscoderPlugin) startCleanupService() {
	interval := p.config.Cleanup.GetCleanupInterval()
	p.cleanupTicker = time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-p.cleanupTicker.C:
				if info, err := p.cleanupService.CleanupExpiredSessions(); err != nil {
					p.logger.Error("cleanup failed", "error", err)
				} else if info.DirectoriesRemoved > 0 {
					p.logger.Debug("cleanup completed",
						"removed", info.DirectoriesRemoved,
						"freed_gb", float64(info.SizeFreed)/(1024*1024*1024),
					)
				}
			}
		}
	}()
}

// ========================================================================
// Stub implementations for services not supported by this plugin
// ========================================================================

func (p *FFmpegTranscoderPlugin) MetadataScraperService() plugins.MetadataScraperService { return nil }
func (p *FFmpegTranscoderPlugin) ScannerHookService() plugins.ScannerHookService         { return nil }
func (p *FFmpegTranscoderPlugin) AssetService() plugins.AssetService                     { return nil }
func (p *FFmpegTranscoderPlugin) DatabaseService() plugins.DatabaseService               { return nil }
func (p *FFmpegTranscoderPlugin) AdminPageService() plugins.AdminPageService             { return nil }
func (p *FFmpegTranscoderPlugin) APIRegistrationService() plugins.APIRegistrationService { return nil }
func (p *FFmpegTranscoderPlugin) SearchService() plugins.SearchService                   { return nil }
func (p *FFmpegTranscoderPlugin) HealthMonitorService() plugins.HealthMonitorService     { return nil }
func (p *FFmpegTranscoderPlugin) ConfigurationService() plugins.ConfigurationService     { return nil }
func (p *FFmpegTranscoderPlugin) PerformanceMonitorService() plugins.PerformanceMonitorService {
	return nil
}
func (p *FFmpegTranscoderPlugin) EnhancedAdminPageService() plugins.EnhancedAdminPageService {
	return nil
}
