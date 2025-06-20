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
	"github.com/mantonx/viewra/pkg/plugins"
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
	planner          PlaybackPlanner
	transcodeManager TranscodeManager
	transcodeService *core.TranscodeService
	cleanupService   *core.CleanupService
	fileManager      *core.FileManager
	providerManager  *core.ProviderManager
	sessionStore     *core.SessionStore

	// Plugin integration
	pluginManager PluginManagerInterface

	// Configuration
	config      *ManagerConfig
	enabled     bool
	initialized bool
}

// ManagerConfig contains configuration for the playback manager
type ManagerConfig struct {
	// Transcoding settings
	MaxConcurrentSessions int
	SessionTimeout        time.Duration
	CleanupInterval       time.Duration

	// Paths
	TranscodingDir string
	TempDir        string

	// Resource limits
	MaxDiskUsageGB int
	MaxMemoryMB    int

	// Retention policy
	RetentionHours         int
	ExtendedRetentionHours int
	LargeFileSizeMB        int
}

// NewManager creates a new playback manager
func NewManager(db *gorm.DB, eventBus events.EventBus, pluginManager PluginManagerInterface, managerConfig *ManagerConfig) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	// Set defaults
	if managerConfig == nil {
		managerConfig = &ManagerConfig{
			MaxConcurrentSessions:  10,
			SessionTimeout:         2 * time.Hour,
			CleanupInterval:        30 * time.Second,
			TranscodingDir:         "/viewra-data/transcoding",
			TempDir:                "/tmp/viewra",
			MaxDiskUsageGB:         50,
			MaxMemoryMB:            4096,
			RetentionHours:         24,
			ExtendedRetentionHours: 48,
			LargeFileSizeMB:        500,
		}
	}

	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "playback-manager",
		Level: hclog.Info,
	})

	// Create core services with correct signatures
	sessionStore := core.NewSessionStore(db, logger.Named("session-store"))
	fileManager := core.NewFileManager(managerConfig.TranscodingDir, logger.Named("file-manager"))
	providerManager := core.NewProviderManager(sessionStore, logger.Named("provider-manager"))

	// Create cleanup config
	cleanupConfig := core.CleanupConfig{
		BaseDirectory:      managerConfig.TranscodingDir,
		RetentionHours:     managerConfig.RetentionHours,
		ExtendedHours:      managerConfig.ExtendedRetentionHours,
		MaxTotalSizeGB:     int64(managerConfig.MaxDiskUsageGB),
		CleanupInterval:    managerConfig.CleanupInterval,
		LargeFileThreshold: int64(managerConfig.LargeFileSizeMB),
	}

	cleanupService := core.NewCleanupService(cleanupConfig, sessionStore, fileManager, logger.Named("cleanup-service"))

	// Create transcoding config from system config
	cfg := config.Get()
	transcodeService, err := core.NewTranscodeService(cfg.Transcoding, db, logger.Named("transcode-service"))
	if err != nil {
		// Log error but continue - the service can be initialized later
		logger.Error("failed to create transcode service", "error", err)
	}

	return &Manager{
		logger:      logger,
		db:          db,
		eventBus:    eventBus,
		ctx:         ctx,
		cancel:      cancel,
		config:      managerConfig,
		enabled:     true,
		initialized: false,

		// Core services
		planner:          NewPlaybackPlanner(),
		transcodeManager: NewTranscodeManager(logger, db, pluginManager),
		transcodeService: transcodeService,
		cleanupService:   cleanupService,
		fileManager:      fileManager,
		providerManager:  providerManager,
		sessionStore:     sessionStore,

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

	// Initialize the transcoding manager
	if initializer, ok := m.transcodeManager.(interface{ Initialize() error }); ok {
		if err := initializer.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize transcode manager: %w", err)
		}
	}

	// Discover transcoding plugins only if using plugin manager
	if m.pluginManager != nil {
		if err := m.transcodeManager.DiscoverTranscodingPlugins(); err != nil {
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
	logger.Info("Playback manager initialized successfully")
	return nil
}

// Shutdown gracefully shuts down the playback manager
func (m *Manager) Shutdown() error {
	logger.Info("Shutting down playback manager")

	// Cancel context to stop all background services
	m.cancel()

	// Stop all active sessions
	sessions, err := m.transcodeManager.ListSessions()
	if err == nil {
		for _, session := range sessions {
			if session.Status == "running" || session.Status == "pending" {
				_ = m.transcodeManager.StopSession(session.ID)
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

	return m.transcodeManager.StartTranscode(request)
}

// StopSession stops a transcoding session
func (m *Manager) StopSession(sessionID string) error {
	if !m.initialized {
		return fmt.Errorf("playback manager not initialized")
	}

	return m.transcodeManager.StopSession(sessionID)
}

// GetSession retrieves session information
func (m *Manager) GetSession(sessionID string) (*database.TranscodeSession, error) {
	if !m.initialized {
		return nil, fmt.Errorf("playback manager not initialized")
	}

	return m.transcodeManager.GetSession(sessionID)
}

// ListSessions returns all sessions
func (m *Manager) ListSessions() ([]*database.TranscodeSession, error) {
	if !m.initialized {
		return nil, fmt.Errorf("playback manager not initialized")
	}

	return m.transcodeManager.ListSessions()
}

// GetStats returns transcoding statistics
func (m *Manager) GetStats() (*TranscodingStats, error) {
	if !m.initialized {
		return nil, fmt.Errorf("playback manager not initialized")
	}

	return m.transcodeManager.GetStats()
}

// RefreshTranscodingPlugins refreshes the list of available transcoding plugins
func (m *Manager) RefreshTranscodingPlugins() error {
	if !m.initialized {
		return fmt.Errorf("playback manager not initialized")
	}

	return m.transcodeManager.DiscoverTranscodingPlugins()
}

// Getters for components

// GetPlanner returns the playback planner
func (m *Manager) GetPlanner() PlaybackPlanner {
	return m.planner
}

// GetTranscodeManager returns the transcode manager
func (m *Manager) GetTranscodeManager() TranscodeManager {
	return m.transcodeManager
}

// GetTranscodeService returns the transcode service
func (m *Manager) GetTranscodeService() *core.TranscodeService {
	return m.transcodeService
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
		sessions, err := m.transcodeManager.ListSessions()
		if err == nil {
			for _, session := range sessions {
				if session.Status == "running" || session.Status == "pending" {
					_ = m.transcodeManager.StopSession(session.ID)
				}
			}
		}
	}
}

// SetPluginManager sets the plugin manager (used for late binding)
func (m *Manager) SetPluginManager(pluginManager PluginManagerInterface) {
	m.pluginManager = pluginManager

	// Update transcoding manager if it supports plugin manager updates
	if updater, ok := m.transcodeManager.(interface{ SetPluginManager(PluginManagerInterface) }); ok {
		updater.SetPluginManager(pluginManager)
	}
}
