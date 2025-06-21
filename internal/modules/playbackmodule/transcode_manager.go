package playbackmodule

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/core"
	plugins "github.com/mantonx/viewra/sdk"
	"gorm.io/gorm"
)

// TranscodeManagerImpl is the main implementation of TranscodeManager
type TranscodeManagerImpl struct {
	logger        hclog.Logger
	db            *gorm.DB
	service       *core.TranscodeService
	pluginManager PluginManagerInterface
	mu            sync.RWMutex
	streamHandles map[string]*plugins.StreamHandle
}

// NewTranscodeManager creates a new TranscodeManager implementation
func NewTranscodeManager(logger hclog.Logger, db *gorm.DB, pluginManager interface{}) TranscodeManager {
	// Convert plugin manager interface
	var pm PluginManagerInterface
	if pluginManager != nil {
		if manager, ok := pluginManager.(PluginManagerInterface); ok {
			pm = manager
		}
	}

	// Get transcoding config from global config
	cfg := config.Get()

	// Create the main transcode service
	service, err := core.NewTranscodeService(cfg.Transcoding, db, logger)
	if err != nil {
		logger.Error("failed to create transcode service", "error", err)
		// Return a basic manager without service
		return &TranscodeManagerImpl{
			logger:        logger.Named("transcode-manager"),
			db:            db,
			pluginManager: pm,
			streamHandles: make(map[string]*plugins.StreamHandle),
		}
	}

	return &TranscodeManagerImpl{
		logger:        logger.Named("transcode-manager"),
		db:            db,
		service:       service,
		pluginManager: pm,
		streamHandles: make(map[string]*plugins.StreamHandle),
	}
}

// SetPluginManager sets the plugin manager for late binding
func (tm *TranscodeManagerImpl) SetPluginManager(pluginManager PluginManagerInterface) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.logger.Info("TranscodeManagerImpl.SetPluginManager called", "pluginManager_nil", pluginManager == nil)
	tm.pluginManager = pluginManager
}

// Initialize sets up the transcoding manager
func (tm *TranscodeManagerImpl) Initialize() error {
	tm.logger.Info("initializing transcode manager")

	if tm.service == nil {
		return fmt.Errorf("transcode service not initialized")
	}

	tm.logger.Info("transcode manager initialized successfully")
	return nil
}

// RegisterProvider registers a transcoding provider from a plugin
func (tm *TranscodeManagerImpl) RegisterProvider(pluginID string, provider plugins.TranscodingProvider) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.service == nil {
		return fmt.Errorf("transcode service not initialized")
	}

	if err := tm.service.RegisterProvider(provider); err != nil {
		return fmt.Errorf("failed to register provider: %w", err)
	}

	tm.logger.Info("registered transcoding provider", "plugin_id", pluginID)
	return nil
}

// DiscoverTranscodingPlugins discovers and registers all available transcoding plugins
func (tm *TranscodeManagerImpl) DiscoverTranscodingPlugins() error {
	tm.logger.Info("discovering transcoding plugins")

	// Debug print to verify this is being called
	fmt.Printf("DEBUG: DiscoverTranscodingPlugins called, pluginManager = %v\n", tm.pluginManager)

	// If no plugin manager is available, skip discovery
	if tm.pluginManager == nil {
		tm.logger.Debug("no plugin manager available for transcoding plugin discovery")
		fmt.Printf("DEBUG: pluginManager is nil!\n")
		return nil
	}

	// Get list of running plugins from plugin manager
	runningPlugins := tm.pluginManager.GetRunningPlugins()
	tm.logger.Info("found running plugins", "count", len(runningPlugins))

	// Debug log all plugins
	for i, p := range runningPlugins {
		tm.logger.Debug("plugin in list",
			"index", i,
			"id", p.ID,
			"name", p.Name,
			"type", p.Type,
		)
	}

	discoveredCount := 0

	for _, pluginInfo := range runningPlugins {
		tm.logger.Debug("examining plugin",
			"plugin_id", pluginInfo.ID,
			"name", pluginInfo.Name,
			"type", pluginInfo.Type,
			"version", pluginInfo.Version,
		)

		// Only process transcoder type plugins
		if pluginInfo.Type != "transcoder" {
			tm.logger.Debug("skipping non-transcoder plugin",
				"plugin_id", pluginInfo.ID,
				"type", pluginInfo.Type,
			)
			continue
		}

		tm.logger.Info("found transcoder plugin", "plugin_id", pluginInfo.ID, "name", pluginInfo.Name)

		// Get the plugin interface
		pluginInterface, exists := tm.pluginManager.GetRunningPluginInterface(pluginInfo.ID)
		if !exists {
			tm.logger.Error("transcoder plugin interface not found", "plugin_id", pluginInfo.ID)
			continue
		}

		// Check if the plugin interface provides a transcoding provider
		if pluginImpl, ok := pluginInterface.(interface {
			TranscodingProvider() plugins.TranscodingProvider
		}); ok {
			provider := pluginImpl.TranscodingProvider()
			if provider != nil {
				// Register the transcoding provider
				if err := tm.RegisterProvider(pluginInfo.ID, provider); err != nil {
					tm.logger.Error("failed to register transcoding provider", "plugin_id", pluginInfo.ID, "error", err)
					continue
				}

				discoveredCount++
				tm.logger.Info("successfully registered transcoding provider", "plugin_id", pluginInfo.ID, "name", pluginInfo.Name)
			} else {
				tm.logger.Error("transcoding plugin returned nil provider", "plugin_id", pluginInfo.ID)
			}
		} else {
			tm.logger.Error("transcoding plugin does not implement TranscodingProvider interface",
				"plugin_id", pluginInfo.ID,
				"interface_type", fmt.Sprintf("%T", pluginInterface),
			)
		}
	}

	tm.logger.Info("transcoding plugin discovery completed", "discovered_count", discoveredCount)
	return nil
}

// CanTranscode checks if any plugin can handle the request
func (tm *TranscodeManagerImpl) CanTranscode(req *plugins.TranscodeRequest) error {
	if tm.service == nil {
		return fmt.Errorf("transcode service not initialized")
	}

	providers := tm.service.GetProviders()
	if len(providers) == 0 {
		return fmt.Errorf("no transcoding providers available")
	}

	// Check if at least one provider can handle this
	ctx := context.Background()
	for _, info := range providers {
		// Would need provider access to check CanTranscode
		// For now, assume if we have providers, we can transcode
		_ = ctx
		_ = info
	}

	return nil
}

// StartTranscode starts a new transcoding session using the best available plugin
func (tm *TranscodeManagerImpl) StartTranscode(req *plugins.TranscodeRequest) (*database.TranscodeSession, error) {
	if tm.service == nil {
		return nil, fmt.Errorf("transcode service not initialized")
	}

	ctx := context.Background()
	session, err := tm.service.StartTranscode(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to start transcoding session: %w", err)
	}

	return session, nil
}

// GetSession retrieves a transcoding session
func (tm *TranscodeManagerImpl) GetSession(sessionID string) (*database.TranscodeSession, error) {
	if tm.service == nil {
		return nil, fmt.Errorf("transcode service not initialized")
	}

	return tm.service.GetSession(sessionID)
}

// StopSession stops a transcoding session
func (tm *TranscodeManagerImpl) StopSession(sessionID string) error {
	if tm.service == nil {
		return fmt.Errorf("transcode service not initialized")
	}

	return tm.service.StopTranscode(sessionID)
}

// ListSessions lists all active sessions across all plugins
func (tm *TranscodeManagerImpl) ListSessions() ([]*database.TranscodeSession, error) {
	if tm.service == nil {
		return nil, fmt.Errorf("transcode service not initialized")
	}

	// Get active sessions directly - no filter needed for active sessions
	var sessions []*database.TranscodeSession
	err := tm.db.Where("status IN ?", []string{"queued", "running", "starting"}).
		Order("start_time DESC").
		Find(&sessions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	return sessions, nil
}

// GetStats returns transcoding statistics across all plugins
func (tm *TranscodeManagerImpl) GetStats() (*TranscodingStats, error) {
	if tm.service == nil {
		return nil, fmt.Errorf("transcode service not initialized")
	}

	// Get active sessions
	sessions, err := tm.ListSessions()
	if err != nil {
		return nil, err
	}

	// Build stats
	stats := &TranscodingStats{
		ActiveSessions:    len(sessions),
		TotalSessions:     0, // Would need to query database
		CompletedSessions: 0, // Would need to query database
		FailedSessions:    0, // Would need to query database
		Backends:          make(map[string]*BackendStats),
		RecentSessions:    sessions,
	}

	// Add backend stats for each provider
	providers := tm.service.GetProviders()
	for _, info := range providers {
		stats.Backends[info.ID] = &BackendStats{
			Name:         info.Name,
			Priority:     info.Priority,
			Capabilities: make(map[string]interface{}), // Empty for now
		}
	}

	return stats, nil
}

// GetTranscodeService returns the core transcode service
func (tm *TranscodeManagerImpl) GetTranscodeService() *core.TranscodeService {
	return tm.service
}

// Cleanup performs cleanup of expired sessions
func (tm *TranscodeManagerImpl) Cleanup() {
	if tm.service == nil {
		tm.logger.Warn("cleanup called but service not initialized")
		return
	}

	tm.logger.Debug("manual cleanup triggered")
	// The cleanup service runs automatically, this is just for manual triggers
	cleanupStats, err := tm.service.GetCleanupStats()
	if err != nil {
		tm.logger.Error("failed to get cleanup stats", "error", err)
		return
	}

	tm.logger.Info("cleanup stats",
		"total_size_gb", float64(cleanupStats.TotalSize)/(1024*1024*1024),
		"sessions", cleanupStats.TotalSessions,
	)
}

// GetCleanupStats returns cleanup-related statistics
func (tm *TranscodeManagerImpl) GetCleanupStats() (*CleanupStats, error) {
	if tm.service == nil {
		return nil, fmt.Errorf("transcode service not initialized")
	}

	stats, err := tm.service.GetCleanupStats()
	if err != nil {
		return nil, err
	}

	return &CleanupStats{
		TotalDirectories:       stats.TotalSessions,
		TotalSizeGB:            float64(stats.TotalSize) / (1024 * 1024 * 1024),
		DirectoriesRemoved:     0, // Not tracked in CleanupStats
		SizeFreedGB:            0, // Not tracked in CleanupStats
		LastCleanupTime:        stats.Timestamp,
		NextCleanupTime:        stats.Timestamp.Add(30 * time.Minute), // Estimate
		RetentionHours:         stats.RetentionHours,
		ExtendedRetentionHours: stats.RetentionHours * 2, // Default extended
		MaxSizeLimitGB:         int(stats.MaxSizeGB),
	}, nil
}

// ========================================================================
// Streaming Methods
// ========================================================================

// StartStreamingTranscode starts a streaming transcoding operation
func (tm *TranscodeManagerImpl) StartStreamingTranscode(req *plugins.TranscodeRequest) (*plugins.StreamHandle, error) {
	tm.logger.Info("starting streaming transcode",
		"input", req.InputPath,
		"container", req.Container,
	)

	// Use the transcode service to select the best provider
	if tm.service == nil {
		return nil, fmt.Errorf("transcode service not initialized")
	}

	// Get available providers
	providers := tm.service.GetProviders()
	if len(providers) == 0 {
		return nil, fmt.Errorf("no transcoding providers available")
	}

	// For now, use the first available provider (highest priority)
	// In the future, this could be smarter about provider selection
	providerInfo := providers[0]

	// Get the plugin interface
	pluginInterface, exists := tm.pluginManager.GetRunningPluginInterface(providerInfo.ID)
	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", providerInfo.ID)
	}

	// Type assert to get the TranscodingProvider
	var provider plugins.TranscodingProvider
	if pluginImpl, ok := pluginInterface.(interface {
		TranscodingProvider() plugins.TranscodingProvider
	}); ok {
		provider = pluginImpl.TranscodingProvider()
		if provider == nil {
			return nil, fmt.Errorf("plugin returned nil provider")
		}
	} else {
		return nil, fmt.Errorf("plugin does not implement TranscodingProvider")
	}

	// Start streaming
	handle, err := provider.StartStream(context.Background(), *req)
	if err != nil {
		return nil, fmt.Errorf("failed to start streaming: %w", err)
	}

	// Track the stream handle
	tm.mu.Lock()
	tm.streamHandles[handle.SessionID] = handle
	tm.mu.Unlock()

	return handle, nil
}

// GetStream returns the stream reader for a streaming handle
func (tm *TranscodeManagerImpl) GetStream(sessionID string) (io.ReadCloser, error) {
	tm.mu.RLock()
	handle, exists := tm.streamHandles[sessionID]
	tm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("stream not found: %s", sessionID)
	}

	// Get the plugin interface
	pluginInterface, exists := tm.pluginManager.GetRunningPluginInterface(handle.Provider)
	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", handle.Provider)
	}

	// Type assert to get the TranscodingProvider
	var provider plugins.TranscodingProvider
	if pluginImpl, ok := pluginInterface.(interface {
		TranscodingProvider() plugins.TranscodingProvider
	}); ok {
		provider = pluginImpl.TranscodingProvider()
		if provider == nil {
			return nil, fmt.Errorf("plugin returned nil provider")
		}
	} else {
		return nil, fmt.Errorf("plugin does not implement TranscodingProvider")
	}

	return provider.GetStream(handle)
}

// StopStreamingTranscode stops a streaming transcoding operation
func (tm *TranscodeManagerImpl) StopStreamingTranscode(sessionID string) error {
	tm.mu.Lock()
	handle, exists := tm.streamHandles[sessionID]
	if exists {
		delete(tm.streamHandles, sessionID)
	}
	tm.mu.Unlock()

	if !exists {
		return fmt.Errorf("stream not found: %s", sessionID)
	}

	// Get the plugin interface
	pluginInterface, exists := tm.pluginManager.GetRunningPluginInterface(handle.Provider)
	if !exists {
		// Still try to clean up even if we can't get the plugin
		tm.logger.Warn("plugin not found for stream cleanup", "provider", handle.Provider)
		return nil
	}

	// Type assert to get the TranscodingProvider
	var provider plugins.TranscodingProvider
	if pluginImpl, ok := pluginInterface.(interface {
		TranscodingProvider() plugins.TranscodingProvider
	}); ok {
		provider = pluginImpl.TranscodingProvider()
		if provider == nil {
			tm.logger.Warn("plugin returned nil provider for cleanup")
			return nil
		}
	} else {
		tm.logger.Warn("plugin does not implement TranscodingProvider for cleanup")
		return nil
	}

	return provider.StopStream(handle)
}
