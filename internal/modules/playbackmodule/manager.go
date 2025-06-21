package playbackmodule

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/core"
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
	planner         PlaybackPlanner
	transcodingService *core.TranscodeService
	cleanupService  *core.CleanupService
	fileManager     *core.FileManager
	sessionStore    *core.SessionStore

	// Plugin integration
	pluginManager PluginManagerInterface

	// Configuration
	config      config.TranscodingConfig
	enabled     bool
	initialized bool
}

// NewManager creates a new playback manager
func NewManager(db *gorm.DB, eventBus events.EventBus, pluginManager PluginManagerInterface, _ interface{}) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	// Get global config
	cfg := config.Get()

	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "playback-manager",
		Level: hclog.Info,
	})

	// Create core services
	sessionStore := core.NewSessionStore(db, logger.Named("session-store"))
	fileManager := core.NewFileManager(cfg.Transcoding.DataDir, logger.Named("file-manager"))

	// Create transcoding service
	transcodingService, err := core.NewTranscodeService(cfg.Transcoding, db, logger.Named("transcode-service"))
	if err != nil {
		logger.Error("failed to create transcoding service", "error", err)
		// Continue without transcoding service for now
	}

	// Create cleanup config
	cleanupConfig := core.CleanupConfig{
		BaseDirectory:      cfg.Transcoding.DataDir,
		RetentionHours:     cfg.Transcoding.RetentionHours,
		ExtendedHours:      cfg.Transcoding.ExtendedHours,
		MaxTotalSizeGB:     cfg.Transcoding.MaxDiskUsageGB,
		CleanupInterval:    cfg.Transcoding.CleanupInterval,
		LargeFileThreshold: cfg.Transcoding.LargeFileThreshold * 1024 * 1024,
	}

	cleanupService := core.NewCleanupService(cleanupConfig, sessionStore, fileManager, logger.Named("cleanup-service"))

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
		planner:            NewPlaybackPlanner(),
		transcodingService: transcodingService,
		cleanupService:     cleanupService,
		fileManager:        fileManager,
		sessionStore:       sessionStore,

		// Plugin integration
		pluginManager: pluginManager,
	}
}

// Initialize sets up the playback manager
func (m *Manager) Initialize() error {
	logger.Info("Initializing playback manager")

	if m.initialized {
		return nil
	}

	// Discover transcoding plugins directly
	if m.pluginManager != nil {
		if err := m.discoverTranscodingPlugins(); err != nil {
			m.logger.Warn("failed to discover transcoding plugins", "error", err)
		}
	}

	// Start the cleanup service
	go m.cleanupService.Run(m.ctx)

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

	return m.planner.DecidePlayback(mediaPath, deviceProfile)
}

// StartTranscode initiates a new transcoding session
func (m *Manager) StartTranscode(request *plugins.TranscodeRequest) (*database.TranscodeSession, error) {
	if !m.initialized {
		return nil, fmt.Errorf("playback manager not initialized")
	}

	if !m.enabled {
		return nil, fmt.Errorf("playback manager is disabled")
	}

	if m.transcodingService == nil {
		return nil, fmt.Errorf("transcoding service not available")
	}

	m.logger.Info("TRACE: Manager.StartTranscode called",
		"transcoding_service_instance", fmt.Sprintf("%p", m.transcodingService),
		"request_container", request.Container,
		"request_input_path", request.InputPath)
	
	// ROBUSTNESS FIX: If no providers are available, try to re-run plugin discovery
	// This handles timing issues where discovery ran before plugins were ready
	providerManager := m.transcodingService.GetProviderManager()
	if providerManager != nil && len(providerManager.GetProviders()) == 0 {
		m.logger.Warn("No providers available when starting transcode - attempting to re-run plugin discovery")
		if m.pluginManager != nil {
			if err := m.discoverTranscodingPlugins(); err != nil {
				m.logger.Error("Failed to re-run plugin discovery", "error", err)
			} else {
				m.logger.Info("Plugin discovery re-run completed", "providers_now", len(providerManager.GetProviders()))
			}
		}
	}
	
	return m.transcodingService.StartTranscode(context.Background(), request)
}

// StartTranscodeFromMediaFile initiates a new transcoding session from a media file ID
func (m *Manager) StartTranscodeFromMediaFile(mediaFileID string, container string) (*database.TranscodeSession, error) {
	m.logger.Info("StartTranscodeFromMediaFile called", "media_file_id", mediaFileID, "container", container)
	
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

	// Default container if not specified
	if container == "" {
		container = "dash"
	}

	// Create transcode request from media file
	request := &plugins.TranscodeRequest{
		InputPath:     mediaFile.Path,
		Container:     container,
		VideoCodec:    "h264", // Default codec
		AudioCodec:    "aac",  // Default audio codec
		Quality:       70,     // Default quality
		SpeedPriority: plugins.SpeedPriorityBalanced,
	}

	m.logger.Info("created transcode request from media file",
		"media_file_id", mediaFileID,
		"input_path", request.InputPath,
		"container", request.Container)

	return m.StartTranscode(request)
}

// StopSession stops a transcoding session
func (m *Manager) StopSession(sessionID string) error {
	if !m.initialized {
		return fmt.Errorf("playback manager not initialized")
	}

	if m.transcodingService == nil {
		return fmt.Errorf("transcoding service not available")
	}

	return m.transcodingService.StopTranscode(sessionID)
}

// GetSession retrieves session information
func (m *Manager) GetSession(sessionID string) (*database.TranscodeSession, error) {
	if !m.initialized {
		return nil, fmt.Errorf("playback manager not initialized")
	}

	return m.sessionStore.GetSession(sessionID)
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
func (m *Manager) GetTranscodeService() *core.TranscodeService {
	return m.transcodingService
}

// GetCleanupService returns the cleanup service
func (m *Manager) GetCleanupService() *core.CleanupService {
	return m.cleanupService
}

// GetFileManager returns the file manager
func (m *Manager) GetFileManager() *core.FileManager {
	return m.fileManager
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

// SetPluginManager sets the plugin manager (used for late binding)
func (m *Manager) SetPluginManager(pluginManager PluginManagerInterface) {
	logger.Info("Manager.SetPluginManager called", "pluginManager_nil", pluginManager == nil)
	m.pluginManager = pluginManager

	// Discover plugins immediately
	if pluginManager != nil {
		if err := m.discoverTranscodingPlugins(); err != nil {
			m.logger.Warn("failed to discover plugins after setting plugin manager", "error", err)
		}
	}
}

// discoverTranscodingPlugins discovers and registers transcoding providers from plugins
func (m *Manager) discoverTranscodingPlugins() error {
	m.logger.Info("discovering transcoding plugins")

	if m.pluginManager == nil {
		m.logger.Debug("no plugin manager available")
		return nil
	}

	runningPlugins := m.pluginManager.GetRunningPlugins()
	m.logger.Info("found plugins", "count", len(runningPlugins))

	discoveredCount := 0

	for _, pluginInfo := range runningPlugins {
		if pluginInfo.Type != "transcoder" {
			continue
		}

		pluginInterface, exists := m.pluginManager.GetRunningPluginInterface(pluginInfo.ID)
		if !exists {
			m.logger.Error("plugin interface not found", "plugin_id", pluginInfo.ID)
			continue
		}

		// Check if plugin provides transcoding
		if pluginImpl, ok := pluginInterface.(interface {
			TranscodingProvider() plugins.TranscodingProvider
		}); ok {
			provider := pluginImpl.TranscodingProvider()
			if provider != nil {
				// Register only with transcoding service
				if m.transcodingService != nil {
					if err := m.transcodingService.RegisterProvider(provider); err != nil {
						m.logger.Error("failed to register provider", "plugin_id", pluginInfo.ID, "error", err)
						continue
					}
					discoveredCount++
					m.logger.Info("registered provider", "plugin_id", pluginInfo.ID, "name", pluginInfo.Name)
				} else {
					m.logger.Error("transcoding service not available for provider registration", "plugin_id", pluginInfo.ID)
				}
			}
		}
	}

	m.logger.Info("plugin discovery completed", "count", discoveredCount)
	return nil
}
