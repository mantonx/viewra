package playbackmodule

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/core"
	"github.com/mantonx/viewra/internal/services"
	plugins "github.com/mantonx/viewra/sdk"
	"gorm.io/gorm"
)

// Manager handles all playback and transcoding operations
type Manager struct {
	logger   hclog.Logger
	db       *gorm.DB
	eventBus events.EventBus
	ctx      context.Context
	cancel   context.CancelFunc

	// Core services
	planner            PlaybackPlanner
	transcodingService services.TranscodingService // Use interface from services package
	sessionStore       *core.SessionStore
	errorRecovery      *ErrorRecoveryManager
	mediaValidator     MediaValidator

	// Configuration
	config      config.TranscodingConfig
	enabled     bool
	initialized bool
}

// NewManager creates a new playback manager
func NewManager(db *gorm.DB, eventBus events.EventBus, _ interface{}) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	// Get global config
	cfg := config.Get()

	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "playback-manager",
		Level: hclog.Info,
	})

	// Create core services
	sessionStore := core.NewSessionStore(db, logger.Named("session-store"))

	// Note: Transcoding service will be retrieved from service registry during initialization
	// Cleanup service is now managed by the transcoding module

	// Create error recovery manager
	errorRecovery := NewErrorRecoveryManager(logger.Named("error-recovery"))

	// Create media validator
	mediaValidator := NewStandardMediaValidator(logger.Named("media-validator"))

	return &Manager{
		logger:      logger,
		db:          db,
		eventBus:    eventBus,
		ctx:         ctx,
		cancel:      cancel,
		config:      cfg.Transcoding,
		enabled:     true,
		initialized: false,

		// Core services
		planner:            NewPlaybackPlanner(NewFFProbeMediaAnalyzer()),
		transcodingService: nil, // Will be set from service registry
		sessionStore:       sessionStore,
		errorRecovery:      errorRecovery,
		mediaValidator:     mediaValidator,
	}
}

// Initialize sets up the playback manager
func (m *Manager) Initialize() error {
	logger.Info("Initializing playback manager")

	if m.initialized {
		return nil
	}

	// Get transcoding service from service registry using lazy loading
	// This prevents circular dependency issues during module initialization
	transcodingService, err := services.GetTranscodingServiceLazy()
	if err != nil {
		logger.Warn("Transcoding service not available yet, will retry on demand", "error", err)
		// Don't fail initialization - the service will be loaded on first use
	} else {
		m.transcodingService = transcodingService
		logger.Info("Successfully obtained transcoding service from registry")
	}

	// Cleanup is now handled by the transcoding module
	// Start process registry cleanup on a regular interval
	go m.runProcessRegistryCleanup()

	// Publish initialization event
	if m.eventBus != nil {
		initEvent := events.NewSystemEvent(
			events.EventInfo,
			"Playback Manager Initialized",
			"Playback manager has been successfully initialized",
		)
		m.eventBus.PublishAsync(initEvent)
	}

	m.initialized = true
	logger.Info("Playback manager initialized successfully", "manager_instance", fmt.Sprintf("%p", m))
	return nil
}

// Shutdown gracefully shuts down the playback manager
func (m *Manager) Shutdown() error {
	logger.Info("Shutting down playback manager")

	// Cancel context to stop all background services
	m.cancel()

	// Stop all active sessions
	sessions, err := m.sessionStore.GetActiveSessions()
	if err == nil {
		for _, session := range sessions {
			if session.Status == "running" || session.Status == "queued" {
				_ = m.StopSession(session.ID)
			}
		}
	}

	m.initialized = false
	logger.Info("Playback manager shutdown complete")
	return nil
}

// Core functionality methods

// DecidePlayback analyzes media and determines playback strategy
func (m *Manager) DecidePlayback(mediaPath string, deviceProfile *DeviceProfile) (*PlaybackDecision, error) {
	if !m.initialized {
		return nil, fmt.Errorf("playback manager not initialized")
	}

	// First, validate the media file
	m.logger.Debug("Validating media file before playback decision", "path", mediaPath)

	validation, err := m.mediaValidator.QuickValidate(mediaPath)
	if err != nil {
		return nil, fmt.Errorf("media validation failed: %w", err)
	}

	if !validation.IsValid {
		return nil, NewMediaValidationError(
			mediaPath,
			validation.ErrorMessage,
			fmt.Errorf("file failed validation"),
		)
	}

	if len(validation.Warnings) > 0 {
		m.logger.Warn("Media file has validation warnings",
			"path", mediaPath,
			"warnings", validation.Warnings)
	}

	return m.planner.DecidePlayback(mediaPath, deviceProfile)
}

// StartTranscode initiates a new transcoding session with error recovery
func (m *Manager) StartTranscode(request *plugins.TranscodeRequest) (*database.TranscodeSession, error) {
	if !m.initialized {
		return nil, fmt.Errorf("playback manager not initialized")
	}

	if !m.enabled {
		return nil, fmt.Errorf("playback manager is disabled")
	}

	// Try to get transcoding service again if not available (lazy loading)
	if m.transcodingService == nil {
		transcodingService, err := services.GetTranscodingServiceLazy()
		if err != nil {
			return nil, fmt.Errorf("transcoding service not available: %w", err)
		}
		m.transcodingService = transcodingService
		m.logger.Info("Successfully obtained transcoding service via lazy loading")
	}

	m.logger.Info("TRACE: Manager.StartTranscode called",
		"transcoding_service_instance", fmt.Sprintf("%p", m.transcodingService),
		"request_container", request.Container,
		"request_input_path", request.InputPath)

	// Comprehensive media validation before transcoding
	m.logger.Debug("Performing comprehensive media validation", "input_path", request.InputPath)

	validation, err := m.mediaValidator.ValidateMedia(context.Background(), request.InputPath)
	if err != nil {
		return nil, fmt.Errorf("media validation failed: %w", err)
	}

	if !validation.IsValid {
		return nil, NewMediaValidationError(
			request.InputPath,
			validation.ErrorMessage,
			fmt.Errorf("file failed comprehensive validation"),
		)
	}

	if validation.IsCorrupted {
		return nil, NewMediaValidationError(
			request.InputPath,
			"file appears to be corrupted",
			fmt.Errorf("corruption detected"),
		)
	}

	if len(validation.Warnings) > 0 {
		m.logger.Warn("Media file has validation warnings",
			"input_path", request.InputPath,
			"warnings", validation.Warnings)
	}

	m.logger.Info("Media validation passed",
		"input_path", request.InputPath,
		"size_bytes", validation.SizeBytes,
		"format", validation.FileFormat,
		"has_video", validation.HasVideoTrack,
		"has_audio", validation.HasAudioTrack,
		"duration", validation.Duration)

	// Ensure plugins are available before starting transcode
	if !m.hasTranscodingProviders() {
		m.logger.Warn("No providers available, waiting for plugins")

		// Use our waitForPlugins method with a shorter timeout for responsiveness
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := m.waitForPlugins(ctx); err != nil {
			m.logger.Error("Failed to find transcoding providers", "error", err)
			return nil, fmt.Errorf("no transcoding providers available: %w", err)
		}
	}

	// Execute transcoding with error recovery and fallback
	var session *database.TranscodeSession

	fallbackErr := m.errorRecovery.ExecuteWithFallback(
		context.Background(),
		request,
		func(ctx context.Context, req *plugins.TranscodeRequest) error {
			var transcodeErr error
			session, transcodeErr = m.transcodingService.StartTranscode(ctx, req)
			return transcodeErr
		},
	)

	if fallbackErr != nil {
		m.logger.Error("Transcoding failed after all recovery attempts", "error", fallbackErr)
		return nil, fmt.Errorf("transcoding failed: %w", fallbackErr)
	}

	if session == nil {
		return nil, fmt.Errorf("no session returned from transcoding service")
	}

	return session, nil
}

// StartTranscodeFromMediaFile initiates a new transcoding session from a media file ID using intelligent decisions
func (m *Manager) StartTranscodeFromMediaFile(mediaFileID string, container string, seekSeconds float64, enableABR bool, deviceProfile *DeviceProfile) (*database.TranscodeSession, error) {
	m.logger.Info("StartTranscodeFromMediaFile called", "media_file_id", mediaFileID, "container", container, "enable_abr", enableABR)

	if !m.initialized {
		return nil, fmt.Errorf("playback manager not initialized")
	}

	// Look up media file from database
	var mediaFile database.MediaFile
	if err := m.db.Where("id = ?", mediaFileID).First(&mediaFile).Error; err != nil {
		m.logger.Error("failed to find media file", "media_file_id", mediaFileID, "error", err)
		return nil, fmt.Errorf("media file not found: %w", err)
	}

	m.logger.Info("found media file", "path", mediaFile.Path, "container", mediaFile.Container)

	// Use playback planner to make intelligent decisions
	decision, err := m.planner.DecidePlayback(mediaFile.Path, deviceProfile)
	if err != nil {
		m.logger.Error("failed to make playback decision", "error", err)
		return nil, fmt.Errorf("failed to make playback decision: %w", err)
	}

	// Check if transcoding is even needed
	if !decision.ShouldTranscode {
		m.logger.Info("direct play recommended", "reason", decision.Reason)
		return nil, fmt.Errorf("direct play recommended: %s", decision.Reason)
	}

	// Use intelligent transcoding parameters from the decision
	request := decision.TranscodeParams
	if request == nil {
		return nil, fmt.Errorf("no transcoding parameters in decision")
	}

	// Apply user-specified overrides where appropriate
	if container != "" {
		request.Container = container
	}
	if seekSeconds > 0 {
		request.Seek = time.Duration(seekSeconds * float64(time.Second))
	}
	// Override ABR setting if explicitly requested
	request.EnableABR = enableABR

	m.logger.Info("using intelligent transcode request",
		"media_file_id", mediaFileID,
		"input_path", request.InputPath,
		"container", request.Container,
		"video_codec", request.VideoCodec,
		"audio_codec", request.AudioCodec,
		"quality", request.Quality,
		"speed_priority", request.SpeedPriority,
		"enable_abr", request.EnableABR,
		"decision_reason", decision.Reason)

	return m.StartTranscode(request)
}

// StopSession stops a transcoding session
func (m *Manager) StopSession(sessionID string) error {
	if !m.initialized {
		return fmt.Errorf("playback manager not initialized")
	}

	// Try to get transcoding service again if not available (lazy loading)
	if m.transcodingService == nil {
		transcodingService, err := services.GetTranscodingServiceLazy()
		if err != nil {
			return fmt.Errorf("transcoding service not available: %w", err)
		}
		m.transcodingService = transcodingService
		m.logger.Info("Successfully obtained transcoding service via lazy loading")
	}

	return m.transcodingService.StopSession(sessionID)
}

// GetSession retrieves session information
func (m *Manager) GetSession(sessionID string) (*database.TranscodeSession, error) {
	if !m.initialized {
		return nil, fmt.Errorf("playback manager not initialized")
	}

	return m.sessionStore.GetSession(sessionID)
}

// GetSessionStore returns the session store for direct access
func (m *Manager) GetSessionStore() *core.SessionStore {
	return m.sessionStore
}

// ListSessions returns all sessions
func (m *Manager) ListSessions() ([]*database.TranscodeSession, error) {
	if !m.initialized {
		return nil, fmt.Errorf("playback manager not initialized")
	}

	return m.sessionStore.GetActiveSessions()
}

// GetStats returns transcoding statistics
func (m *Manager) GetStats() (*TranscodingStats, error) {
	if !m.initialized {
		return nil, fmt.Errorf("playback manager not initialized")
	}

	// Get sessions directly from session store
	sessions, err := m.sessionStore.GetActiveSessions()
	if err != nil {
		return nil, err
	}

	// Build basic stats
	stats := &TranscodingStats{
		ActiveSessions:    len(sessions),
		TotalSessions:     0,
		CompletedSessions: 0,
		FailedSessions:    0,
		Backends:          make(map[string]*BackendStats),
		RecentSessions:    sessions,
	}

	// Get provider info from transcoding service
	if m.transcodingService != nil {
		providers := m.transcodingService.GetProviders()
		for _, info := range providers {
			stats.Backends[info.ID] = &BackendStats{
				Name:         info.Name,
				Priority:     info.Priority,
				Capabilities: make(map[string]interface{}),
			}
		}
	}

	return stats, nil
}

// RefreshTranscodingPlugins refreshes the list of available transcoding plugins
func (m *Manager) RefreshTranscodingPlugins() error {
	if !m.initialized {
		return fmt.Errorf("playback manager not initialized")
	}

	return m.discoverTranscodingPlugins()
}

// Getters for components

// GetPlanner returns the playback planner
func (m *Manager) GetPlanner() PlaybackPlanner {
	return m.planner
}

// GetTranscodeService returns the transcode service
func (m *Manager) GetTranscodeService() services.TranscodingService {
	return m.transcodingService
}

// IsEnabled returns whether the manager is enabled
func (m *Manager) IsEnabled() bool {
	return m.enabled
}

// SetEnabled enables or disables the manager
func (m *Manager) SetEnabled(enabled bool) {
	m.enabled = enabled
	if !enabled {
		// Stop all active sessions when disabling
		sessions, err := m.sessionStore.GetActiveSessions()
		if err == nil {
			for _, session := range sessions {
				if session.Status == "running" || session.Status == "queued" {
					_ = m.StopSession(session.ID)
				}
			}
		}
	}
}

// discoverTranscodingPlugins is now a no-op since the transcoding module manages its own providers
func (m *Manager) discoverTranscodingPlugins() error {
	m.logger.Info("plugin discovery is now handled by transcoding module")
	return nil
}

// waitForPlugins waits for at least one transcoding plugin to be available
func (m *Manager) waitForPlugins(ctx context.Context) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	m.logger.Info("Waiting for transcoding plugins to be available")
	attempts := 0

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for plugins: %w", ctx.Err())
		case <-ticker.C:
			attempts++
			// Try to discover plugins via service registry
			if err := m.discoverTranscodingPlugins(); err != nil {
				m.logger.Debug("Plugin discovery attempt failed", "attempt", attempts, "error", err)
			}

			// Check if we have providers now
			if m.hasTranscodingProviders() {
				m.logger.Info("Transcoding plugins are now available", "attempts", attempts)
				return nil
			}
			m.logger.Debug("Still waiting for transcoding plugins", "attempt", attempts)
		}
	}
}

// hasTranscodingProviders checks if any transcoding providers are registered
func (m *Manager) hasTranscodingProviders() bool {
	// Try to get transcoding service if not available (lazy loading)
	if m.transcodingService == nil {
		transcodingService, err := services.GetTranscodingServiceLazy()
		if err != nil {
			return false
		}
		m.transcodingService = transcodingService
	}
	// Check if there are any providers available
	providers := m.transcodingService.GetProviders()
	return len(providers) > 0
}

// runProcessRegistryCleanup runs periodic cleanup of the process registry
func (m *Manager) runProcessRegistryCleanup() {
	// Run cleanup every 30 seconds
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	processRegistry := core.GetProcessRegistry(m.logger)

	for {
		select {
		case <-m.ctx.Done():
			m.logger.Info("stopping process registry cleanup")
			return
		case <-ticker.C:
			// Run the cleanup
			killedCount := processRegistry.CleanupOrphaned()
			if killedCount > 0 {
				m.logger.Info("process registry cleanup killed processes", "count", killedCount)
			}
		}
	}
}

// KillZombieProcesses manually triggers cleanup of zombie FFmpeg processes
func (m *Manager) KillZombieProcesses() (int, error) {
	if !m.initialized {
		return 0, fmt.Errorf("playback manager not initialized")
	}

	m.logger.Info("manually triggering zombie process cleanup")

	// First run process registry cleanup
	processRegistry := core.GetProcessRegistry(m.logger)
	registryKilled := processRegistry.CleanupOrphaned()

	// Cleanup service orphan detection is now handled by transcoding module

	m.logger.Info("zombie process cleanup completed", "killed_count", registryKilled)
	return registryKilled, nil
}

// GetErrorRecoveryStats returns error recovery and circuit breaker statistics
func (m *Manager) GetErrorRecoveryStats() map[string]interface{} {
	if m.errorRecovery == nil {
		return map[string]interface{}{
			"error": "error recovery manager not available",
		}
	}

	return m.errorRecovery.GetStats()
}

// ValidateMediaFile validates a media file using the media validator
func (m *Manager) ValidateMediaFile(ctx context.Context, mediaPath string, quick bool) (*MediaValidationResult, error) {
	if !m.initialized {
		return nil, fmt.Errorf("playback manager not initialized")
	}

	if m.mediaValidator == nil {
		return nil, fmt.Errorf("media validator not available")
	}

	if quick {
		return m.mediaValidator.QuickValidate(mediaPath)
	}

	return m.mediaValidator.ValidateMedia(ctx, mediaPath)
}

// GetContentStore returns the content store interface
func (m *Manager) GetContentStore() interface{} {
	// Content store is now managed by transcoding module
	// We'll remove this method and update the module to handle it differently
	return nil
}

// GetURLGenerator returns the URL generator interface
func (m *Manager) GetURLGenerator() interface{} {
	// URL generator is now managed by transcoding module
	// We'll remove this method and update the module to handle it differently
	return nil
}

// GetMediaFilePath resolves a media file ID to its file path
func (m *Manager) GetMediaFilePath(mediaFileID string) (string, error) {
	if !m.initialized {
		return "", fmt.Errorf("playback manager not initialized")
	}

	// Look up media file from database
	var mediaFile database.MediaFile
	if err := m.db.Where("id = ?", mediaFileID).First(&mediaFile).Error; err != nil {
		m.logger.Error("failed to find media file", "media_file_id", mediaFileID, "error", err)
		return "", fmt.Errorf("media file not found: %w", err)
	}

	m.logger.Info("resolved media file path", "media_file_id", mediaFileID, "path", mediaFile.Path)
	return mediaFile.Path, nil
}
