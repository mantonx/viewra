package playbackmodule

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/pkg/plugins"
	"gorm.io/gorm"
)

// TranscodeManagerImpl is the main implementation of TranscodeManager
type TranscodeManagerImpl struct {
	logger         hclog.Logger
	sessionManager *SessionManager

	// Plugin registry
	registeredPlugins map[string]plugins.TranscodingService
	mu                sync.RWMutex

	pluginManager PluginManagerInterface
}

// NewTranscodeManager creates a new TranscodeManager implementation
func NewTranscodeManager(logger hclog.Logger, db *gorm.DB, pluginManager interface{}) TranscodeManager {
	sessionManager := NewSessionManager(logger, db)

	// Convert plugin manager interface
	var pm PluginManagerInterface
	if pluginManager != nil {
		if manager, ok := pluginManager.(PluginManagerInterface); ok {
			pm = manager
		}
	}

	return &TranscodeManagerImpl{
		logger:            logger.Named("transcode-manager"),
		sessionManager:    sessionManager,
		registeredPlugins: make(map[string]plugins.TranscodingService),
		pluginManager:     pm,
	}
}

// Initialize sets up the transcoding manager
func (tm *TranscodeManagerImpl) Initialize() error {
	tm.logger.Info("initializing transcode manager")
	tm.logger.Info("transcode manager initialized successfully")
	return nil
}

// RegisterTranscoder registers a transcoding plugin
func (tm *TranscodeManagerImpl) RegisterTranscoder(name string, transcoder plugins.TranscodingService) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.registeredPlugins[name] = transcoder
	if err := tm.sessionManager.RegisterPlugin(name, transcoder); err != nil {
		return fmt.Errorf("failed to register plugin with session manager: %w", err)
	}

	tm.logger.Info("registered transcoding plugin", "name", name)
	return nil
}

// DiscoverTranscodingPlugins discovers and registers all available transcoding plugins
func (tm *TranscodeManagerImpl) DiscoverTranscodingPlugins() error {
	tm.logger.Debug("discovering transcoding plugins")

	// If no plugin manager is available, skip discovery
	if tm.pluginManager == nil {
		tm.logger.Debug("no plugin manager available for transcoding plugin discovery")
		return nil
	}

	// Get list of running plugins from plugin manager
	runningPlugins := tm.pluginManager.GetRunningPlugins()
	discoveredCount := 0

	for _, pluginInfo := range runningPlugins {
		// Only process transcoder type plugins
		if pluginInfo.Type != "transcoder" {
			continue
		}

		tm.logger.Debug("found transcoder plugin", "plugin_id", pluginInfo.ID, "name", pluginInfo.Name)

		// Get the plugin interface
		pluginInterface, exists := tm.pluginManager.GetRunningPluginInterface(pluginInfo.ID)
		if !exists {
			tm.logger.Warn("transcoder plugin interface not found", "plugin_id", pluginInfo.ID)
			continue
		}

		// Check if the plugin interface provides a transcoding service
		if pluginImpl, ok := pluginInterface.(interface {
			TranscodingService() plugins.TranscodingService
		}); ok {
			transcodingService := pluginImpl.TranscodingService()
			if transcodingService != nil {
				// Register the transcoding service
				if err := tm.RegisterTranscoder(pluginInfo.ID, transcodingService); err != nil {
					tm.logger.Error("failed to register transcoding plugin", "plugin_id", pluginInfo.ID, "error", err)
					continue
				}

				discoveredCount++
				tm.logger.Info("registered transcoding plugin", "plugin_id", pluginInfo.ID, "name", pluginInfo.Name)
			} else {
				tm.logger.Warn("transcoding plugin returned nil service", "plugin_id", pluginInfo.ID)
			}
		} else {
			tm.logger.Warn("transcoding plugin does not implement TranscodingService interface", "plugin_id", pluginInfo.ID)
		}
	}

	tm.logger.Info("transcoding plugin discovery completed", "discovered_count", discoveredCount, "total_registered", len(tm.registeredPlugins))
	return nil
}

// CanTranscode checks if any plugin can handle the request
func (tm *TranscodeManagerImpl) CanTranscode(req *plugins.TranscodeRequest) error {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if len(tm.registeredPlugins) == 0 {
		return fmt.Errorf("no transcoding plugins available")
	}

	// Try to find a plugin that can handle this request
	_, _, err := tm.sessionManager.GetBestPlugin(req)
	if err != nil {
		return fmt.Errorf("no suitable plugin found: %w", err)
	}

	return nil
}

// StartTranscode starts a new transcoding session using the best available plugin
func (tm *TranscodeManagerImpl) StartTranscode(req *plugins.TranscodeRequest) (*plugins.TranscodeSession, error) {
	ctx := context.Background()
	session, err := tm.sessionManager.StartSession(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to start transcoding session: %w", err)
	}

	return session, nil
}

// GetSession retrieves a transcoding session
func (tm *TranscodeManagerImpl) GetSession(sessionID string) (*plugins.TranscodeSession, error) {
	return tm.sessionManager.GetSession(sessionID)
}

// StopSession stops a transcoding session
func (tm *TranscodeManagerImpl) StopSession(sessionID string) error {
	return tm.sessionManager.StopSession(sessionID)
}

// ListSessions lists all active sessions across all plugins
func (tm *TranscodeManagerImpl) ListSessions() ([]*plugins.TranscodeSession, error) {
	return tm.sessionManager.ListActiveSessions()
}

// GetStats returns transcoding statistics across all plugins
func (tm *TranscodeManagerImpl) GetStats() (*TranscodingStats, error) {
	return tm.sessionManager.GetStats()
}

// GetTranscodeStream returns the transcoding service for streaming a session
func (tm *TranscodeManagerImpl) GetTranscodeStream(sessionID string) (plugins.TranscodingService, error) {
	return tm.sessionManager.GetTranscodeStream(sessionID)
}

// Cleanup performs cleanup of expired sessions
func (tm *TranscodeManagerImpl) Cleanup() {
	tm.logger.Debug("cleanup called - session manager handles cleanup automatically")
}

// GetAvailablePlugins returns a list of available transcoding plugins
func (tm *TranscodeManagerImpl) GetAvailablePlugins() []string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var plugins []string
	for pluginID := range tm.registeredPlugins {
		plugins = append(plugins, pluginID)
	}

	return plugins
}
