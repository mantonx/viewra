package plugin

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mantonx/viewra/data/plugins/ffmpeg_transcoder/internal/config"
	"github.com/mantonx/viewra/data/plugins/ffmpeg_transcoder/internal/services"
	"github.com/mantonx/viewra/data/plugins/ffmpeg_transcoder/internal/types"
	"github.com/mantonx/viewra/pkg/plugins"
)

// FFmpegTranscoderPlugin implements the plugin interface
type FFmpegTranscoderPlugin struct {
	logger             plugins.Logger
	config             *config.Config
	transcodingService services.TranscodingService
	sessionManager     services.SessionManager
	cleanupService     services.CleanupService
	hardwareDetector   services.HardwareDetector

	ctx           context.Context
	cancel        context.CancelFunc
	cleanupTicker *time.Ticker
	mutex         sync.RWMutex
}

// New creates a new FFmpeg transcoder plugin instance
func New() *FFmpegTranscoderPlugin {
	return &FFmpegTranscoderPlugin{
		config: config.DefaultConfig(),
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
	if p.config.Hardware.Acceleration {
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

	// Stop all active sessions
	if sessions, err := p.sessionManager.ListActiveSessions(); err == nil {
		for _, session := range sessions {
			if err := p.transcodingService.StopSession(session.ID); err != nil {
				p.logger.Warn("failed to stop session",
					"session_id", session.ID,
					"error", err,
				)
			}
		}
	}

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
	// Check if transcoding service is healthy
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

// TranscodingService returns the transcoding service interface
func (p *FFmpegTranscoderPlugin) TranscodingService() plugins.TranscodingService {
	// Use the proper constructor that initializes the map
	return newTranscodingServiceAdapter(p)
}

// loadConfiguration loads the plugin configuration
func (p *FFmpegTranscoderPlugin) loadConfiguration(ctx *plugins.PluginContext) error {
	// TODO: Load from CUE configuration system
	// For now, use defaults
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

	// Initialize transcoding service using the existing implementation
	// Note: The existing DefaultTranscodingService doesn't match our interface
	// so we'll need to create an adapter or modify the interface
	p.transcodingService = &transcodingServiceWrapper{
		service:          services.NewTranscodingService(p.config),
		logger:           p.logger,
		config:           p.config,
		sessionManager:   p.sessionManager,
		hardwareDetector: p.hardwareDetector,
	}

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
				} else {
					p.logger.Debug("cleanup completed",
						"removed", info.DirectoriesRemoved,
						"freed_gb", float64(info.SizeFreed)/(1024*1024*1024),
					)
				}
			}
		}
	}()
}

// transcodingServiceWrapper wraps the existing DefaultTranscodingService to match our interface
type transcodingServiceWrapper struct {
	service          *services.DefaultTranscodingService
	logger           plugins.Logger
	config           *config.Config
	sessionManager   services.SessionManager
	hardwareDetector services.HardwareDetector
}

// StartTranscode starts a new transcoding session
func (w *transcodingServiceWrapper) StartTranscode(ctx context.Context, req *plugins.TranscodeRequest) (*types.Session, error) {
	// Generate the proper output path for DASH/HLS
	container := "mp4"
	if req.CodecOpts != nil && req.CodecOpts.Container != "" {
		container = req.CodecOpts.Container
	}

	// Use the session ID provided by the main app
	sessionUUID := req.SessionID
	if sessionUUID == "" {
		// Check environment for session ID
		if envSessionID, ok := req.Environment["session_id"]; ok && envSessionID != "" {
			sessionUUID = envSessionID
		} else {
			// Fall back to generating a UUID if no session ID provided
			sessionUUID = uuid.New().String()
			w.logger.Warn("No session ID provided in request, generating new UUID", "sessionId", sessionUUID)
		}
	}

	// Build the output path based on container type
	// Use /app/viewra-data/transcoding which maps to the host's viewra-data/transcoding
	outputFile := ""
	sessionDir := ""
	if container == "dash" {
		// For DASH, create a directory and point to the manifest
		sessionDir = fmt.Sprintf("/app/viewra-data/transcoding/dash_ffmpeg_transcoder_%s", sessionUUID)
		outputFile = fmt.Sprintf("%s/manifest.mpd", sessionDir)
	} else if container == "hls" {
		// For HLS, create a directory and point to the playlist
		sessionDir = fmt.Sprintf("/app/viewra-data/transcoding/hls_ffmpeg_transcoder_%s", sessionUUID)
		outputFile = fmt.Sprintf("%s/playlist.m3u8", sessionDir)
	} else {
		// For progressive download
		sessionDir = fmt.Sprintf("/app/viewra-data/transcoding/%s_ffmpeg_transcoder_%s", container, sessionUUID)
		outputFile = fmt.Sprintf("%s/%s.%s", sessionDir, sessionUUID, container)
	}

	// Convert plugin request to service request
	serviceReq := &services.TranscodingRequest{
		InputFile:  req.InputPath,
		OutputFile: outputFile,
		Settings: services.JobSettings{
			VideoCodec:   req.CodecOpts.Video,
			AudioCodec:   req.CodecOpts.Audio,
			Container:    req.CodecOpts.Container,
			Quality:      req.CodecOpts.Quality,
			Preset:       req.CodecOpts.Preset,
			AudioBitrate: 128, // Default
		},
		Environment: req.Environment, // Pass through the environment variables
	}

	// Check if this is a seek session by parsing the session ID
	// Format: {original_id}_seek_{seconds}
	if strings.Contains(sessionUUID, "_seek_") {
		parts := strings.Split(sessionUUID, "_seek_")
		if len(parts) == 2 {
			seekTimeStr := parts[1]
			if serviceReq.Environment == nil {
				serviceReq.Environment = make(map[string]string)
			}
			serviceReq.Environment["SEEK_START"] = seekTimeStr
			w.logger.Info("detected seek session",
				"session_id", sessionUUID,
				"seek_time", seekTimeStr)
		}
	}

	// Start the job
	_, err := w.service.StartJob(ctx, serviceReq)
	if err != nil {
		return nil, err
	}

	// Create session in our session manager using the UUID
	session, err := w.sessionManager.CreateSession(sessionUUID, req.InputPath, container)
	if err != nil {
		return nil, err
	}

	// Update session with output path and status to running
	w.sessionManager.UpdateSession(sessionUUID, func(s *types.Session) error {
		s.OutputPath = outputFile
		s.Status = types.StatusRunning // Set to running since FFmpeg has started
		s.SessionDir = sessionDir
		s.ProcessPID = 0 // Will be updated when process starts
		return nil
	})

	// Store mapping between our UUID and the internal job ID
	w.sessionManager.UpdateSession(sessionUUID, func(s *types.Session) error {
		// We'll need to track the internal job ID somehow
		// For now, just return the session with our UUID
		return nil
	})

	return session, nil
}

// GetSession retrieves session information
func (w *transcodingServiceWrapper) GetSession(sessionID string) (*types.Session, error) {
	return w.sessionManager.GetSession(sessionID)
}

// StopSession stops a transcoding session
func (w *transcodingServiceWrapper) StopSession(sessionID string) error {
	// Stop in the service
	if err := w.service.StopJob(context.Background(), sessionID); err != nil {
		w.logger.Warn("failed to stop job in service", "error", err)
	}

	// Remove from session manager
	return w.sessionManager.RemoveSession(sessionID)
}

// ListSessions returns all active sessions
func (w *transcodingServiceWrapper) ListSessions() ([]*types.Session, error) {
	return w.sessionManager.ListActiveSessions()
}

// GetCapabilities returns transcoding capabilities
func (w *transcodingServiceWrapper) GetCapabilities() *plugins.TranscodingCapabilities {
	hwInfo, _ := w.hardwareDetector.DetectHardware()

	return &plugins.TranscodingCapabilities{
		Name: "FFmpeg Transcoder",
		SupportedCodecs: []string{
			"h264", "h265", "vp8", "vp9", "av1",
			"aac", "mp3", "opus", "ac3", "dts",
		},
		SupportedResolutions: []string{
			"480p", "720p", "1080p", "1440p", "2160p",
		},
		SupportedContainers: []string{
			"mp4", "mkv", "webm", "dash", "hls",
		},
		HardwareAcceleration:  hwInfo != nil && hwInfo.Available,
		MaxConcurrentSessions: w.config.Sessions.MaxConcurrent,
		Features: plugins.TranscodingFeatures{
			SubtitleBurnIn:      true,
			SubtitlePassthrough: true,
			MultiAudioTracks:    true,
			HDRSupport:          true,
			ToneMapping:         true,
			StreamingOutput:     true,
			SegmentedOutput:     true,
		},
		Priority: w.config.Priority,
	}
}

// Stub implementations for services not supported by this plugin
func (p *FFmpegTranscoderPlugin) MetadataScraperService() plugins.MetadataScraperService { return nil }
func (p *FFmpegTranscoderPlugin) ScannerHookService() plugins.ScannerHookService         { return nil }
func (p *FFmpegTranscoderPlugin) AssetService() plugins.AssetService                     { return nil }
func (p *FFmpegTranscoderPlugin) DatabaseService() plugins.DatabaseService {
	// Return a stub database service that provides empty models
	return &stubDatabaseService{}
}
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

// stubDatabaseService provides empty database models to satisfy the plugin system
type stubDatabaseService struct{}

// GetModels returns empty models since this plugin doesn't need database tables
func (s *stubDatabaseService) GetModels() []string {
	return []string{}
}

// Migrate is a no-op for this plugin
func (s *stubDatabaseService) Migrate(dbPath string) error {
	return nil
}

// GetMigrations returns empty migrations
func (s *stubDatabaseService) GetMigrations() []string {
	return []string{}
}

// Rollback is a no-op for this plugin
func (s *stubDatabaseService) Rollback(connectionString string) error {
	return nil
}
