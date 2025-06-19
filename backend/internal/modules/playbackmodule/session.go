package playbackmodule

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/pkg/plugins"
	"gorm.io/gorm"
)

// SessionManager handles all transcoding session management
type SessionManager struct {
	logger   hclog.Logger
	db       *gorm.DB
	sessions map[string]*plugins.TranscodeSession
	mutex    sync.RWMutex
	plugins  map[string]plugins.TranscodingService
}

// NewSessionManager creates a new session manager
func NewSessionManager(logger hclog.Logger, db *gorm.DB) *SessionManager {
	return &SessionManager{
		logger:   logger.Named("session-manager"),
		db:       db,
		sessions: make(map[string]*plugins.TranscodeSession),
		plugins:  make(map[string]plugins.TranscodingService),
	}
}

// RegisterPlugin registers a transcoding plugin service
func (sm *SessionManager) RegisterPlugin(pluginID string, service plugins.TranscodingService) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.plugins[pluginID] = service
	sm.logger.Info("registered transcoding plugin", "plugin_id", pluginID)

	return nil
}

// GetBestPlugin returns the best plugin for a given request
func (sm *SessionManager) GetBestPlugin(req *plugins.TranscodeRequest) (string, plugins.TranscodingService, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	var bestPluginID string
	var bestPlugin plugins.TranscodingService
	var highestPriority int = -1

	// Find the highest priority plugin that can handle the request
	for pluginID, plugin := range sm.plugins {
		// Get plugin capabilities
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		capabilities, err := plugin.GetCapabilities(ctx)
		cancel()

		if err != nil {
			sm.logger.Warn("failed to get plugin capabilities", "plugin_id", pluginID, "error", err)
			continue
		}

		// Check if plugin can handle this request (basic validation)
		if !sm.canPluginHandleRequest(capabilities, req) {
			sm.logger.Debug("plugin cannot handle request",
				"plugin_id", pluginID,
				"video_codec", req.CodecOpts.Video,
				"container", req.CodecOpts.Container,
				"supported_codecs", capabilities.SupportedCodecs,
				"supported_containers", capabilities.SupportedContainers)
			continue
		}

		if capabilities.Priority > highestPriority {
			highestPriority = capabilities.Priority
			bestPlugin = plugin
			bestPluginID = pluginID
		}
	}

	if bestPlugin == nil {
		return "", nil, fmt.Errorf("no suitable transcoding plugin found for request")
	}

	return bestPluginID, bestPlugin, nil
}

// StartSession starts a new transcoding session with automatic plugin selection
func (sm *SessionManager) StartSession(ctx context.Context, req *plugins.TranscodeRequest) (*plugins.TranscodeSession, error) {
	// Generate a UUID session ID if one isn't provided
	if req.SessionID == "" {
		req.SessionID = generateSessionID()
	}

	// Find the best plugin for this request
	pluginID, plugin, err := sm.GetBestPlugin(req)
	if err != nil {
		return nil, err
	}

	// Start the transcoding session
	session, err := plugin.StartTranscode(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to start transcoding with plugin %s: %w", pluginID, err)
	}

	// Store session in memory for quick access
	sm.mutex.Lock()
	sm.sessions[session.ID] = session
	sm.mutex.Unlock()

	sm.logger.Info("transcoding session started",
		"session_id", session.ID,
		"plugin_id", pluginID,
		"input_path", req.InputPath,
		"target_codec", req.CodecOpts.Video,
		"resolution", req.Environment["resolution"])

	return session, nil
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(sessionID string) (*plugins.TranscodeSession, error) {
	// Try memory first for active sessions
	sm.mutex.RLock()
	if session, exists := sm.sessions[sessionID]; exists {
		sm.mutex.RUnlock()

		// Try to get updated status from the plugin
		for _, plugin := range sm.plugins {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			updatedSession, err := plugin.GetTranscodeSession(ctx, sessionID)
			cancel()

			if err == nil && updatedSession != nil {
				// Update our cached session
				sm.mutex.Lock()
				sm.sessions[sessionID] = updatedSession
				sm.mutex.Unlock()
				return updatedSession, nil
			}
		}

		return session, nil
	}
	sm.mutex.RUnlock()

	// If not in memory, try all plugins
	for _, plugin := range sm.plugins {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		session, err := plugin.GetTranscodeSession(ctx, sessionID)
		cancel()

		if err == nil && session != nil {
			// Cache it for future lookups
			sm.mutex.Lock()
			sm.sessions[sessionID] = session
			sm.mutex.Unlock()
			return session, nil
		}
	}

	return nil, fmt.Errorf("session not found: %s", sessionID)
}

// StopSession stops a transcoding session
func (sm *SessionManager) StopSession(sessionID string) error {
	// Find which plugin owns this session and stop it
	for pluginID, plugin := range sm.plugins {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := plugin.StopTranscode(ctx, sessionID)
		cancel()

		if err == nil {
			// Remove from memory cache
			sm.mutex.Lock()
			delete(sm.sessions, sessionID)
			sm.mutex.Unlock()

			sm.logger.Info("stopped transcoding session", "session_id", sessionID, "plugin_id", pluginID)
			return nil
		}
	}

	return fmt.Errorf("session not found or could not be stopped: %s", sessionID)
}

// ListActiveSessions returns all active sessions across all plugins
func (sm *SessionManager) ListActiveSessions() ([]*plugins.TranscodeSession, error) {
	var allSessions []*plugins.TranscodeSession

	// Get sessions from all plugins
	for pluginID, plugin := range sm.plugins {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		sessions, err := plugin.ListActiveSessions(ctx)
		cancel()

		if err != nil {
			sm.logger.Warn("failed to get sessions from plugin", "plugin_id", pluginID, "error", err)
			continue
		}

		allSessions = append(allSessions, sessions...)
	}

	// Update our memory cache with fresh data
	sm.mutex.Lock()
	sm.sessions = make(map[string]*plugins.TranscodeSession)
	for _, session := range allSessions {
		sm.sessions[session.ID] = session
	}
	sm.mutex.Unlock()

	return allSessions, nil
}

// GetTranscodeStream returns the stream for a session
func (sm *SessionManager) GetTranscodeStream(sessionID string) (plugins.TranscodingService, error) {
	// Find which plugin owns this session
	for pluginID, plugin := range sm.plugins {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		session, err := plugin.GetTranscodeSession(ctx, sessionID)
		cancel()

		if err == nil && session != nil {
			sm.logger.Debug("found session in plugin", "session_id", sessionID, "plugin_id", pluginID)
			return plugin, nil
		}
	}

	return nil, fmt.Errorf("session not found in any plugin: %s", sessionID)
}

// GetStats returns unified statistics across all plugins
func (sm *SessionManager) GetStats() (*TranscodingStats, error) {
	stats := &TranscodingStats{
		Backends: make(map[string]*BackendStats),
	}

	// Get stats from all plugins
	for pluginID, plugin := range sm.plugins {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		capabilities, err := plugin.GetCapabilities(ctx)
		cancel()

		if err != nil {
			sm.logger.Warn("failed to get capabilities for stats", "plugin_id", pluginID, "error", err)
			continue
		}

		// Get active sessions for this plugin
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		sessions, err := plugin.ListActiveSessions(ctx)
		cancel()

		if err != nil {
			sm.logger.Warn("failed to get sessions for stats", "plugin_id", pluginID, "error", err)
			sessions = []*plugins.TranscodeSession{} // Empty slice
		}

		backendStats := &BackendStats{
			Name:           capabilities.Name,
			Priority:       capabilities.Priority,
			ActiveSessions: len(sessions),
			TotalSessions:  int64(len(sessions)), // This could be tracked better in the future
			Capabilities:   capabilities,
		}

		stats.Backends[pluginID] = backendStats
		stats.ActiveSessions += len(sessions)
		stats.TotalSessions += int64(len(sessions))
	}

	return stats, nil
}

// Helper function to check if a plugin can handle a request
func (sm *SessionManager) canPluginHandleRequest(capabilities *plugins.TranscodingCapabilities, req *plugins.TranscodeRequest) bool {
	if req.CodecOpts == nil {
		return true // No specific requirements
	}

	// Check codec support - allow "auto" as it means the plugin will choose the best codec
	if req.CodecOpts.Video != "" && req.CodecOpts.Video != "auto" && !stringInSlice(req.CodecOpts.Video, capabilities.SupportedCodecs) {
		return false
	}

	// Check container support
	if req.CodecOpts.Container != "" && !stringInSlice(req.CodecOpts.Container, capabilities.SupportedContainers) {
		return false
	}

	// Check resolution support (basic check)
	if resolution := req.Environment["resolution"]; resolution != "" && len(capabilities.SupportedResolutions) > 0 && !stringInSlice(resolution, capabilities.SupportedResolutions) {
		return false
	}

	return true
}

func stringInSlice(item string, slice []string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetCleanupStats returns cleanup-related statistics
func (sm *SessionManager) GetCleanupStats() (*CleanupStats, error) {
	// Get basic transcoding directory stats
	transcodingDir := "/viewra-data/transcoding"
	if dir := os.Getenv("TRANSCODING_DATA_DIR"); dir != "" {
		transcodingDir = dir
	}

	stats := &CleanupStats{
		RetentionHours:         2,                                // Default from config
		ExtendedRetentionHours: 8,                                // Default from config
		MaxSizeLimitGB:         10,                               // Default from config
		LastCleanupTime:        time.Now(),                       // This would be tracked in a real implementation
		NextCleanupTime:        time.Now().Add(30 * time.Second), // Next cleanup cycle
	}

	// Calculate directory statistics
	if entries, err := os.ReadDir(transcodingDir); err == nil {
		var totalSize int64
		dirCount := 0

		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "dash_") {
				dirPath := filepath.Join(transcodingDir, entry.Name())
				dirSize := sm.calculateDirectorySize(dirPath)
				totalSize += dirSize
				dirCount++
			}
		}

		stats.TotalDirectories = dirCount
		stats.TotalSizeGB = float64(totalSize) / (1024 * 1024 * 1024)
	}

	return stats, nil
}

// calculateDirectorySize calculates the size of a directory
func (sm *SessionManager) calculateDirectorySize(dirPath string) int64 {
	var size int64

	filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			if info, err := d.Info(); err == nil {
				size += info.Size()
			}
		}
		return nil
	})

	return size
}

// generateSessionID creates a new UUID for session identification
func generateSessionID() string {
	return uuid.New().String()
}
