package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mantonx/viewra/data/plugins/ffmpeg_transcoder/internal/config"
	"github.com/mantonx/viewra/data/plugins/ffmpeg_transcoder/internal/services"
	"github.com/mantonx/viewra/pkg/plugins"
)

// SimpleTranscodingAdapter provides a minimal implementation of TranscodingService
type SimpleTranscodingAdapter struct {
	config             *config.Config
	logger             plugins.Logger
	transcodingService *services.DefaultTranscodingService
	sessions           map[string]*plugins.TranscodeSession
	mutex              sync.RWMutex
}

// GetCapabilities returns basic FFmpeg capabilities
func (s *SimpleTranscodingAdapter) GetCapabilities(ctx context.Context) (*plugins.TranscodingCapabilities, error) {
	// Default max concurrent sessions if config is not available
	maxConcurrent := 4
	if s.config != nil {
		maxConcurrent = s.config.GetMaxConcurrentSessions()
	}

	return &plugins.TranscodingCapabilities{
		Name:                  "FFmpeg Transcoder",
		SupportedCodecs:       []string{"h264", "h265", "hevc", "vp8", "vp9", "av1"},
		SupportedResolutions:  []string{"240p", "360p", "480p", "720p", "1080p", "1440p", "4k"},
		SupportedContainers:   []string{"mp4", "webm", "mkv", "dash", "hls"},
		HardwareAcceleration:  true,
		MaxConcurrentSessions: maxConcurrent,
		Features: plugins.TranscodingFeatures{
			SubtitleBurnIn:      true,
			SubtitlePassthrough: true,
			MultiAudioTracks:    true,
			HDRSupport:          false,
			ToneMapping:         false,
			StreamingOutput:     true,
			SegmentedOutput:     true,
		},
		Priority: 100,
	}, nil
}

// StartTranscode starts a transcoding session - returns success to indicate capability
func (s *SimpleTranscodingAdapter) StartTranscode(ctx context.Context, req *plugins.TranscodeRequest) (*plugins.TranscodeSession, error) {
	// Write debug info to a log file
	debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if debugFile != nil {
		fmt.Fprintf(debugFile, "🎯 [%s] StartTranscode called - input='%s', codec='%s', container='%s'\n",
			time.Now().Format("15:04:05"), req.InputPath, req.TargetCodec, req.TargetContainer)
		debugFile.Close()
	}

	if s.logger != nil {
		s.logger.Info("StartTranscode called - FFmpeg plugin is available for transcoding", "input", req.InputPath, "codec", req.TargetCodec)
	}

	// Generate session ID that will be used for directory naming
	sessionID := fmt.Sprintf("ffmpeg_%d", time.Now().UnixNano())

	// Create a MINIMAL session to test GRPC conversion
	session := &plugins.TranscodeSession{
		ID:        sessionID,
		Status:    plugins.TranscodeStatusStarting,
		Progress:  0.0,
		StartTime: time.Now(),
		Backend:   "ffmpeg",
		Error:     "",
		EndTime:   nil,
		// Initialize ALL fields to prevent nil pointer issues
		Request: &plugins.TranscodeRequest{
			InputPath:       req.InputPath,       // Get from incoming request
			TargetCodec:     req.TargetCodec,     // Get from incoming request
			TargetContainer: req.TargetContainer, // Get from incoming request
			Resolution:      req.Resolution,      // Get from incoming request
			Bitrate:         req.Bitrate,         // Get from incoming request
			AudioCodec:      req.AudioCodec,      // Get from incoming request
			AudioBitrate:    req.AudioBitrate,    // Get from incoming request
			Quality:         req.Quality,         // Get from incoming request
			Preset:          req.Preset,          // Get from incoming request
			Priority:        req.Priority,        // Get from incoming request
			Options:         make(map[string]string),
			// Leave nested pointers as nil - they have nil checks in convertSessionToProto
		},
		Metadata: make(map[string]interface{}),
		Stats: &plugins.TranscodeStats{
			Duration:        time.Duration(0),
			BytesProcessed:  0,
			BytesGenerated:  0,
			FramesProcessed: 0,
			CurrentFPS:      0.0,
			AverageFPS:      0.0,
			CPUUsage:        0.0,
			MemoryUsage:     0,
			Speed:           0.0,
		},
	}

	// Copy options from incoming request if available
	if req.Options != nil {
		session.Request.Options = make(map[string]string)
		for k, v := range req.Options {
			session.Request.Options[k] = v
		}
	}

	// If we have a transcoding service and valid input, start the actual transcoding job
	if s.logger != nil {
		s.logger.Info("Checking transcoding service", "service_nil", s.transcodingService == nil, "input_path", req.InputPath)
	}

	// Write to debug log
	if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
		fmt.Fprintf(debugFile, "🔍 [%s] transcoding service check - service_nil=%t, input_path='%s'\n",
			time.Now().Format("15:04:05"), s.transcodingService == nil, req.InputPath)
		debugFile.Close()
	}

	if s.transcodingService != nil && req.InputPath != "" {
		// Create output directory using the session ID for consistent naming
		outputDir := filepath.Join(s.config.GetOutputDir(), fmt.Sprintf("dash_%s", sessionID))
		if s.logger != nil {
			s.logger.Info("Creating transcoding request", "output_dir", outputDir, "session_id", sessionID)
		}

		// Write to debug log
		if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
			fmt.Fprintf(debugFile, "📁 [%s] Creating transcoding request - output_dir='%s', session_id='%s'\n",
				time.Now().Format("15:04:05"), outputDir, sessionID)
			debugFile.Close()
		}

		transcodingReq := &services.TranscodingRequest{
			InputFile:  req.InputPath,
			OutputFile: filepath.Join(outputDir, "manifest.mpd"),
			Settings: services.JobSettings{
				VideoCodec:   req.TargetCodec,
				AudioCodec:   s.getAudioCodec(req.AudioCodec),
				Quality:      s.getQuality(req.Quality),
				AudioBitrate: s.getAudioBitrate(req.AudioBitrate),
				Preset:       s.getPreset(req.Preset),
				Container:    req.TargetContainer,
			},
		}

		if s.logger != nil {
			s.logger.Info("About to start transcoding job", "input_file", transcodingReq.InputFile, "output_file", transcodingReq.OutputFile)
		}

		// Write to debug log
		if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
			fmt.Fprintf(debugFile, "🎬 [%s] About to call transcodingService.StartJob - input='%s', output='%s', container='%s'\n",
				time.Now().Format("15:04:05"), transcodingReq.InputFile, transcodingReq.OutputFile, transcodingReq.Settings.Container)
			debugFile.Close()
		}

		// Start the transcoding job
		if response, err := s.transcodingService.StartJob(ctx, transcodingReq); err != nil {
			if s.logger != nil {
				s.logger.Error("Failed to start transcoding job", "error", err, "session_id", sessionID)
			}

			// Write to debug log
			if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
				fmt.Fprintf(debugFile, "❌ [%s] StartJob failed - error='%v'\n", time.Now().Format("15:04:05"), err)
				debugFile.Close()
			}

			session.Status = plugins.TranscodeStatusFailed
			session.Error = err.Error()
		} else {
			if s.logger != nil {
				s.logger.Info("Started transcoding job successfully", "session_id", sessionID, "output_dir", outputDir, "job_id", response.JobID)
			}

			// Write to debug log
			if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
				fmt.Fprintf(debugFile, "✅ [%s] StartJob succeeded - job_id='%s', status='%s'\n",
					time.Now().Format("15:04:05"), response.JobID, response.Status)
				debugFile.Close()
			}

			session.Status = plugins.TranscodeStatusRunning
			// Store job info in metadata
			session.Metadata["job_id"] = response.JobID
			session.Metadata["output_dir"] = outputDir

			// CRITICAL: Store session for proper tracking and cleanup
			s.mutex.Lock()
			s.sessions[sessionID] = session
			s.mutex.Unlock()
		}
	} else {
		if s.logger != nil {
			s.logger.Warn("Cannot start transcoding", "service_nil", s.transcodingService == nil, "input_path_empty", req.InputPath == "")
		}

		// Write to debug log
		if debugFile, _ := os.OpenFile("/app/viewra-data/transcoding/plugin_debug.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); debugFile != nil {
			fmt.Fprintf(debugFile, "⚠️ [%s] Cannot start transcoding - service_nil=%t, input_path_empty=%t\n",
				time.Now().Format("15:04:05"), s.transcodingService == nil, req.InputPath == "")
			debugFile.Close()
		}

		session.Error = "Transcoding service not available or input path empty"
	}

	// Always store the session for tracking, even if transcoding failed to start
	s.mutex.Lock()
	s.sessions[sessionID] = session
	s.mutex.Unlock()

	if s.logger != nil {
		s.logger.Info("FFmpeg transcoding session created", "session_id", session.ID, "status", session.Status)
	}
	return session, nil
}

// GetTranscodeSession returns session info
func (s *SimpleTranscodingAdapter) GetTranscodeSession(ctx context.Context, sessionID string) (*plugins.TranscodeSession, error) {
	if s.logger != nil {
		s.logger.Debug("GetTranscodeSession called", "session_id", sessionID)
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if session, exists := s.sessions[sessionID]; exists {
		// Update session status from the underlying service if available
		if s.transcodingService != nil {
			if jobID, ok := session.Metadata["job_id"].(string); ok {
				if status, err := s.transcodingService.GetJobStatus(ctx, jobID); err == nil {
					session.Status = s.convertStatus(status.Status)
					session.Progress = status.Progress.Percentage / 100.0 // Convert percentage to 0.0-1.0 range
					if status.Error != "" {
						session.Error = status.Error
					}
				}
			}
		}
		return session, nil
	}

	return nil, fmt.Errorf("session not found: %s", sessionID)
}

// StopTranscode stops a transcoding session
func (s *SimpleTranscodingAdapter) StopTranscode(ctx context.Context, sessionID string) error {
	if s.logger != nil {
		s.logger.Info("StopTranscode called", "session_id", sessionID)
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		if s.logger != nil {
			s.logger.Warn("Session not found in adapter, attempting emergency cleanup", "session_id", sessionID)
		}
		// Even if session not found, try emergency process cleanup
		return s.emergencyProcessCleanup(sessionID)
	}

	// Stop the underlying job if we have a job ID
	if s.transcodingService != nil {
		if jobID, ok := session.Metadata["job_id"].(string); ok {
			if err := s.transcodingService.StopJob(ctx, jobID); err != nil {
				if s.logger != nil {
					s.logger.Warn("Failed to cancel underlying job", "job_id", jobID, "error", err)
				}
				// If normal stop fails, try emergency cleanup
				s.emergencyProcessCleanup(sessionID)
			}
		}
	}

	// Update session status and remove from tracking
	session.Status = plugins.TranscodeStatusCancelled
	delete(s.sessions, sessionID)

	if s.logger != nil {
		s.logger.Info("Successfully stopped transcoding session", "session_id", sessionID)
	}

	return nil
}

// ListActiveSessions returns active sessions
func (s *SimpleTranscodingAdapter) ListActiveSessions(ctx context.Context) ([]*plugins.TranscodeSession, error) {
	if s.logger != nil {
		s.logger.Debug("ListActiveSessions called")
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var activeSessions []*plugins.TranscodeSession
	for _, session := range s.sessions {
		// Update session status from the underlying service if available
		if s.transcodingService != nil {
			if jobID, ok := session.Metadata["job_id"].(string); ok {
				if status, err := s.transcodingService.GetJobStatus(ctx, jobID); err == nil {
					session.Status = s.convertStatus(status.Status)
					session.Progress = status.Progress.Percentage / 100.0 // Convert percentage to 0.0-1.0 range
					if status.Error != "" {
						session.Error = status.Error
					}
				}
			}
		}

		// Only include sessions that are not completed or failed
		if session.Status == plugins.TranscodeStatusRunning ||
			session.Status == plugins.TranscodeStatusPending ||
			session.Status == plugins.TranscodeStatusStarting {
			activeSessions = append(activeSessions, session)
		}
	}

	return activeSessions, nil
}

// GetTranscodeStream returns the stream
func (s *SimpleTranscodingAdapter) GetTranscodeStream(ctx context.Context, sessionID string) (io.ReadCloser, error) {
	if s.logger != nil {
		s.logger.Debug("GetTranscodeStream called", "session_id", sessionID)
	}
	return nil, fmt.Errorf("direct streaming not supported - use DASH/HLS manifests")
}

// Helper methods to provide default values when request parameters are empty

// getAudioCodec returns the request audio codec or default if empty
func (s *SimpleTranscodingAdapter) getAudioCodec(requestCodec string) string {
	if requestCodec != "" {
		return requestCodec
	}
	if s.config != nil {
		return s.config.Transcoding.AudioCodec
	}
	return "aac" // Fallback default
}

// getQuality returns the request quality or default if zero
func (s *SimpleTranscodingAdapter) getQuality(requestQuality int) int {
	if requestQuality > 0 {
		return requestQuality
	}
	if s.config != nil {
		return s.config.Transcoding.Quality
	}
	return 23 // Fallback default
}

// getAudioBitrate returns the request bitrate or default if zero
func (s *SimpleTranscodingAdapter) getAudioBitrate(requestBitrate int) int {
	if requestBitrate > 0 {
		return requestBitrate
	}
	if s.config != nil {
		return s.config.Transcoding.AudioBitrate
	}
	return 128 // Fallback default
}

// getPreset returns the request preset or default if empty
func (s *SimpleTranscodingAdapter) getPreset(requestPreset string) string {
	if requestPreset != "" {
		return requestPreset
	}
	if s.config != nil {
		return s.config.Transcoding.Preset
	}
	return "medium" // Fallback default
}

// convertStatus converts internal status to plugin status
func (s *SimpleTranscodingAdapter) convertStatus(status services.TranscodingStatus) plugins.TranscodeStatus {
	switch status {
	case services.StatusQueued:
		return plugins.TranscodeStatusPending
	case services.StatusProcessing:
		return plugins.TranscodeStatusRunning
	case services.StatusCompleted:
		return plugins.TranscodeStatusCompleted
	case services.StatusFailed:
		return plugins.TranscodeStatusFailed
	case services.StatusCancelled:
		return plugins.TranscodeStatusCancelled
	case services.StatusTimeout:
		return plugins.TranscodeStatusFailed // Map timeout to failed
	default:
		return plugins.TranscodeStatusStarting
	}
}

// emergencyProcessCleanup kills orphaned FFmpeg processes for a session
func (s *SimpleTranscodingAdapter) emergencyProcessCleanup(sessionID string) error {
	if s.logger != nil {
		s.logger.Warn("Performing emergency process cleanup", "session_id", sessionID)
	}

	// Use pkill to find and kill FFmpeg processes related to this session
	// The session ID appears in the output path, so we can search for it
	cmd := fmt.Sprintf("pkill -f 'ffmpeg.*%s'", sessionID)

	// Execute the kill command
	if err := exec.Command("sh", "-c", cmd).Run(); err != nil {
		if s.logger != nil {
			s.logger.Warn("Emergency cleanup command failed", "session_id", sessionID, "cmd", cmd, "error", err)
		}
		// Don't return error - this is best effort cleanup
	} else {
		if s.logger != nil {
			s.logger.Info("Emergency cleanup completed", "session_id", sessionID)
		}
	}

	return nil
}

// FFmpegTranscoderPlugin implements the plugins.Implementation interface
type FFmpegTranscoderPlugin struct {
	ctx                *plugins.PluginContext
	config             *config.Config
	configService      *config.FFmpegConfigurationService
	transcodingService *services.DefaultTranscodingService
	logger             plugins.Logger
	pluginID           string
	startTime          time.Time
	adapter            *SimpleTranscodingAdapter // Persistent adapter to maintain session state
}

// Initialize sets up the plugin with the provided context
func (p *FFmpegTranscoderPlugin) Initialize(ctx *plugins.PluginContext) error {
	p.ctx = ctx
	p.logger = ctx.Logger // Set the logger field
	p.pluginID = ctx.PluginID
	p.startTime = time.Now()

	// Load configuration FIRST
	cfg := config.DefaultConfig()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	p.config = cfg

	// Initialize transcoding service IMMEDIATELY after config (BEFORE any other setup)
	p.transcodingService = services.NewTranscodingService(cfg)

	// Log that the core service is now available
	ctx.Logger.Info("🚀 Core transcoding service initialized early for GRPC registration")

	// Initialize configuration service
	configPath := filepath.Join(ctx.PluginBasePath, "ffmpeg_config.json")
	p.configService = config.NewFFmpegConfigurationService(configPath)

	// Set the loaded configuration
	if err := p.configService.UpdateFFmpegConfig(cfg); err != nil {
		ctx.Logger.Error("Configuration service validation failed", "error", err)
		return fmt.Errorf("failed to validate FFmpeg configuration: %w", err)
	}

	ctx.Logger.Info("FFmpeg transcoder plugin initialized",
		"ffmpeg_path", cfg.GetFFmpegPath(),
		"output_dir", cfg.GetOutputDir(),
		"max_concurrent", cfg.GetMaxConcurrentSessions())

	return nil
}

// Start begins plugin operation
func (p *FFmpegTranscoderPlugin) Start() error {
	p.ctx.Logger.Info("Starting FFmpeg transcoder plugin")

	// Validate FFmpeg installation
	executor := services.NewFFmpegExecutor(p.config)
	if err := executor.ValidateInstallation(context.Background()); err != nil {
		return fmt.Errorf("FFmpeg validation failed: %w", err)
	}

	// Get and log FFmpeg version
	if version, err := executor.GetVersion(context.Background()); err == nil {
		p.ctx.Logger.Info("FFmpeg version detected", "version", version)
	}

	// Clean up any orphaned processes from previous runs
	p.cleanupOrphanedProcesses()

	// Start periodic cleanup routine
	go p.startCleanupRoutine()

	p.ctx.Logger.Info("FFmpeg transcoder plugin started successfully")

	// Debug: Test if TranscodingService() works
	p.ctx.Logger.Info("🔍 Testing TranscodingService() during startup")
	if service := p.TranscodingService(); service != nil {
		p.ctx.Logger.Info("✅ TranscodingService() returned valid service during startup")
	} else {
		p.ctx.Logger.Error("❌ TranscodingService() returned nil during startup")
	}

	return nil
}

// Stop gracefully shuts down the plugin
func (p *FFmpegTranscoderPlugin) Stop() error {
	p.ctx.Logger.Info("Stopping FFmpeg transcoder plugin")

	// Clean up any active transcoding jobs
	if p.transcodingService != nil {
		// Get system stats to see active jobs
		if stats, err := p.transcodingService.GetSystemStats(context.Background()); err == nil {
			if stats.ActiveJobs > 0 {
				p.ctx.Logger.Warn("Stopping plugin with active transcoding jobs",
					"active_jobs", stats.ActiveJobs)
			}
		}
	}

	p.ctx.Logger.Info("FFmpeg transcoder plugin stopped")
	return nil
}

// Info returns plugin information
func (p *FFmpegTranscoderPlugin) Info() (*plugins.PluginInfo, error) {
	return &plugins.PluginInfo{
		ID:          "ffmpeg_transcoder",
		Name:        "FFmpeg Transcoder",
		Version:     "1.0.0",
		Type:        "transcoder",
		Description: "Video transcoding plugin using FFmpeg",
		Author:      "Viewra Team",
	}, nil
}

// Health returns the current health status of the plugin
func (p *FFmpegTranscoderPlugin) Health() error {
	// Check FFmpeg availability
	if p.transcodingService != nil {
		executor := services.NewFFmpegExecutor(p.config)
		if err := executor.ValidateInstallation(context.Background()); err != nil {
			return fmt.Errorf("FFmpeg not available: %w", err)
		}

		// Get system stats to check for issues
		if stats, err := p.transcodingService.GetSystemStats(context.Background()); err == nil {
			// Determine health based on job statistics
			if stats.TotalJobs > 0 {
				errorRate := float64(stats.FailedJobs) / float64(stats.TotalJobs) * 100
				if errorRate > 80 {
					return fmt.Errorf("critical error rate: %.1f%% of jobs failed", errorRate)
				}
			}
		}
	}

	return nil
}

// Service implementations - return nil for services not supported by this plugin
func (p *FFmpegTranscoderPlugin) MetadataScraperService() plugins.MetadataScraperService {
	return nil
}

func (p *FFmpegTranscoderPlugin) ScannerHookService() plugins.ScannerHookService {
	return nil
}

func (p *FFmpegTranscoderPlugin) AssetService() plugins.AssetService {
	return nil
}

func (p *FFmpegTranscoderPlugin) DatabaseService() plugins.DatabaseService {
	return p
}

func (p *FFmpegTranscoderPlugin) AdminPageService() plugins.AdminPageService {
	return nil
}

func (p *FFmpegTranscoderPlugin) APIRegistrationService() plugins.APIRegistrationService {
	return nil
}

func (p *FFmpegTranscoderPlugin) SearchService() plugins.SearchService {
	return nil
}

func (p *FFmpegTranscoderPlugin) HealthMonitorService() plugins.HealthMonitorService {
	return nil
}

func (p *FFmpegTranscoderPlugin) ConfigurationService() plugins.ConfigurationService {
	if p.configService == nil {
		return nil
	}
	return p.configService
}

func (p *FFmpegTranscoderPlugin) PerformanceMonitorService() plugins.PerformanceMonitorService {
	return nil
}

// TranscodingService returns the transcoding service interface
func (p *FFmpegTranscoderPlugin) TranscodingService() plugins.TranscodingService {
	if p.logger != nil {
		p.logger.Info("🎬 TranscodingService() called - returning persistent adapter")
	}

	// If transcodingService is nil, create it on-demand with default config
	if p.transcodingService == nil {
		if p.logger != nil {
			p.logger.Info("⚡ TranscodingService() - creating service on-demand for GRPC registration")
		}

		// Create default config if we don't have one yet
		cfg := p.config
		if cfg == nil {
			cfg = config.DefaultConfig()
		}

		// Initialize the transcoding service on-demand
		p.transcodingService = services.NewTranscodingService(cfg)
		p.config = cfg // Ensure config is set
	}

	// Create persistent adapter only once, not on every call
	if p.adapter == nil {
		if p.logger != nil {
			p.logger.Info("🔧 Creating persistent adapter for session management")
		}

		p.adapter = &SimpleTranscodingAdapter{
			config:             p.config,
			logger:             p.logger,
			transcodingService: p.transcodingService,
			sessions:           make(map[string]*plugins.TranscodeSession),
			mutex:              sync.RWMutex{},
		}
	}

	if p.logger != nil {
		p.logger.Info("✅ TranscodingService() - returning persistent adapter", "adapter_logger_nil", p.adapter.logger == nil, "adapter_service_nil", p.adapter.transcodingService == nil)
	}

	// Return the persistent adapter that maintains session state
	return p.adapter
}

func (p *FFmpegTranscoderPlugin) EnhancedAdminPageService() plugins.EnhancedAdminPageService {
	return nil
}

// GetModels returns empty slice since this plugin doesn't require database models
func (p *FFmpegTranscoderPlugin) GetModels() []string {
	return []string{}
}

// Migrate is a no-op since this plugin doesn't require database changes
func (p *FFmpegTranscoderPlugin) Migrate(connectionString string) error {
	return nil
}

// Rollback is a no-op since this plugin doesn't require database changes
func (p *FFmpegTranscoderPlugin) Rollback(connectionString string) error {
	return nil
}

// cleanupOrphanedProcesses kills any leftover FFmpeg processes
func (p *FFmpegTranscoderPlugin) cleanupOrphanedProcesses() {
	p.ctx.Logger.Info("🧹 Cleaning up orphaned FFmpeg processes")

	// Kill any FFmpeg processes that are transcoding files (not just the plugin binary)
	cmd := exec.Command("sh", "-c", "pkill -f 'ffmpeg.*dash_' || true")
	if err := cmd.Run(); err != nil {
		p.ctx.Logger.Warn("Failed to cleanup orphaned processes", "error", err)
	} else {
		p.ctx.Logger.Info("✅ Orphaned process cleanup completed")
	}
}

// startCleanupRoutine runs a periodic cleanup routine
func (p *FFmpegTranscoderPlugin) startCleanupRoutine() {
	ticker := time.NewTicker(1 * time.Minute) // Check every minute
	defer ticker.Stop()

	p.ctx.Logger.Info("🔄 Starting FFmpeg process cleanup routine (every 1 minute)")

	for {
		select {
		case <-ticker.C:
			p.performPeriodicCleanup()
		}
	}
}

// performPeriodicCleanup performs regular maintenance cleanup
func (p *FFmpegTranscoderPlugin) performPeriodicCleanup() {
	// Count current FFmpeg processes
	cmd := exec.Command("sh", "-c", "ps aux | grep -c 'ffmpeg.*dash_' || echo '0'")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	processCount := strings.TrimSpace(string(output))
	if processCount != "0" && processCount != "" {
		p.ctx.Logger.Debug("🔍 Periodic cleanup found FFmpeg processes", "count", processCount)

		// Clean up any processes older than 10 minutes without activity
		// This uses a more sophisticated approach to find truly orphaned processes
		cleanupCmd := exec.Command("sh", "-c", `
			# Find FFmpeg processes older than 10 minutes
			ps -eo pid,etime,cmd | grep 'ffmpeg.*dash_' | awk '$2 ~ /^[1-9][0-9]:[0-9][0-9]/ || $2 ~ /^[0-9]+-/ {print $1}' | while read pid; do
				echo "Killing old FFmpeg process: $pid"
				kill -TERM "$pid" 2>/dev/null || kill -KILL "$pid" 2>/dev/null || true
			done
		`)

		if err := cleanupCmd.Run(); err != nil {
			p.ctx.Logger.Debug("Periodic cleanup command failed", "error", err)
		}
	}
}

func main() {
	plugin := &FFmpegTranscoderPlugin{}
	plugins.StartPlugin(plugin)
}
