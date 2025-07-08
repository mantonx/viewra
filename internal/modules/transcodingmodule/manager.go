// Package transcodingmodule provides video transcoding functionality.
// This is the main manager that coordinates all transcoding operations.
package transcodingmodule

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/cleanup"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/migration"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/pipeline"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/registry"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/resource"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/session"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/storage"

	"github.com/mantonx/viewra/internal/modules/transcodingmodule/types"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/utils/filemanager"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/utils/paths"
	"github.com/mantonx/viewra/internal/services"
	tErrors "github.com/mantonx/viewra/internal/modules/transcodingmodule/errors"
	plugins "github.com/mantonx/viewra/sdk"
	"gorm.io/gorm"
)

// Manager coordinates all transcoding operations within the module.
// It provides a clean interface for transcoding operations while delegating
// to specialized components for session management, storage, and resource control.
type Manager struct {
	db            *gorm.DB
	eventBus      events.EventBus
	pluginManager types.PluginManagerInterface
	config        *types.Config

	// Provider management - centralized registry
	providerRegistry *registry.ProviderRegistry

	// Core services - each handles its own domain
	sessionStore     *session.SessionStore     // Database-backed session persistence
	contentStore     *storage.ContentStore     // Content-addressable file storage
	resourceManager  *resource.Manager         // Concurrent session and resource limits
	cleanupService   *cleanup.Service          // Cleanup of old sessions and files
	migrationService *migration.ContentMigrationService // Content hash migration
	fileManager      *filemanager.FileManager  // File operations

	// Logging and error reporting
	logger        hclog.Logger
	errorReporter tErrors.ErrorReporter

	// Lifecycle management
	initialized bool
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewManager creates a new transcoding manager
func NewManager(db *gorm.DB, eventBus events.EventBus, pluginManager types.PluginManagerInterface, config *types.Config) (*Manager, error) {
	if config == nil {
		config = types.DefaultConfig()
	}

	// Override from environment if set
	if dir := paths.GetTranscodingBaseDir(); dir != "" {
		config.TranscodingDir = dir
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create logger
	hclogger := hclog.New(&hclog.LoggerOptions{
		Name:  "transcoding-manager",
		Level: hclog.Info,
	})
	
	// Create error reporter
	errorReporter := tErrors.NewErrorReporter(hclogger.Named("error-reporter"))

	// Create session store
	sessionStore := session.NewSessionStore(db, hclogger.Named("session-store"))

	// Create file manager
	fileManager := filemanager.NewFileManager(config.TranscodingDir, hclogger.Named("file-manager"))

	// Create content store
	contentStore, err := storage.NewContentStore(config.TranscodingDir, hclogger.Named("content-store"))
	if err != nil {
		cancel() // Clean up context
		return nil, tErrors.StorageError("create_content_store", err).
			WithDetail("dir", config.TranscodingDir)
	}

	// Create content migration service
	migrationService := migration.NewContentMigrationService(db)

	// Create resource manager
	resourceConfig := &resource.Config{
		MaxConcurrentSessions: config.MaxConcurrentSessions,
		SessionTimeout:        config.SessionTimeout,
		MaxQueueSize:          20,
		QueueTimeout:          10 * time.Minute,
	}
	resourceManager := resource.NewManager(resourceConfig, hclogger.Named("resource"))

	// Create cleanup service
	cleanupConfig := cleanup.Config{
		BaseDirectory:      config.TranscodingDir,
		RetentionHours:     24,
		ExtendedHours:      48,
		MaxTotalSizeGB:     100,
		CleanupInterval:    config.CleanupInterval,
		LargeFileThreshold: 1024 * 1024 * 1024, // 1GB
	}
	cleanupService := cleanup.NewService(cleanupConfig, sessionStore, fileManager, hclogger.Named("cleanup"))

	// Create provider registry
	providerRegistry := registry.NewProviderRegistry(hclogger.Named("provider-registry"))

	return &Manager{
		db:               db,
		eventBus:         eventBus,
		pluginManager:    pluginManager,
		config:           config,
		providerRegistry: providerRegistry,
		sessionStore:     sessionStore,
		contentStore:     contentStore,
		resourceManager:  resourceManager,
		cleanupService:   cleanupService,
		migrationService: migrationService,
		fileManager:      fileManager,
		logger:           hclogger,
		errorReporter:    errorReporter,
		ctx:              ctx,
		cancel:           cancel,
	}, nil
}

// Initialize sets up the manager and discovers providers
func (m *Manager) Initialize() error {
	if m.initialized {
		return nil
	}

	logger.Info("Initializing transcoding manager")

	// Create transcoding directory if it doesn't exist
	if err := paths.CreateSessionDirectories(m.config.TranscodingDir); err != nil {
		return tErrors.InternalError("create_directories", err).
			WithDetail("dir", m.config.TranscodingDir)
	}

	// Register all providers (built-in and plugins)
	if err := m.setupProviders(); err != nil {
		logger.Error("Failed to setup providers: %v", err)
		// Not fatal - continue with available providers
	}

	// Start cleanup service with safe goroutine
	m.wg.Add(1)
	tErrors.SafeGoContext(m.ctx, m.errorReporter, m.logger, "cleanup-service", func(ctx context.Context) error {
		defer m.wg.Done()
		m.cleanupService.Run(ctx)
		return nil
	})

	m.initialized = true
	logger.Info("Transcoding manager initialized with %d providers", m.providerRegistry.Count())

	return nil
}

// setupProviders registers all available transcoding providers.
// This includes both built-in providers and plugin providers.
func (m *Manager) setupProviders() error {
	// Register built-in file pipeline provider
	pipelineProvider := pipeline.NewProvider(
		m.config.TranscodingDir,
		m.sessionStore,
		m.contentStore,
		m.logger.Named("pipeline-provider"),
	)
	m.providerRegistry.Register(pipelineProvider)

	// Register plugin providers if available
	if m.pluginManager != nil {
		pluginProviders := m.pluginManager.GetTranscodingProviders()
		for _, provider := range pluginProviders {
			m.providerRegistry.Register(provider)
		}
	}

	return nil
}

// DiscoverProviders re-discovers all available plugin providers.
// This is called when the plugin manager is updated.
func (m *Manager) DiscoverProviders() error {
	if m.pluginManager == nil {
		return tErrors.InternalError("discover_providers", tErrors.ErrProviderNotAvailable).
			WithDetail("reason", "plugin_manager_nil")
	}

	// Register new plugin providers
	providers := m.pluginManager.GetTranscodingProviders()
	for _, provider := range providers {
		m.providerRegistry.Register(provider)
	}

	return nil
}

// StartTranscode initiates a new transcoding session with resource management.
// If resource limits are exceeded, the request will be queued.
func (m *Manager) StartTranscode(ctx context.Context, req plugins.TranscodeRequest) (*plugins.TranscodeHandle, error) {
	logger.Info("Manager.StartTranscode called",
		"container", req.Container,
		"inputPath", req.InputPath,
		"mediaID", req.MediaID,
		"sessionID", req.SessionID)

	// Check if there's already an active session for this content
	contentHash := m.sessionStore.GenerateContentHash(req.InputPath, req.MediaID, req.Container)
	logger.Info("Checking for existing sessions", "contentHash", contentHash)
	
	// Get active sessions for this content hash
	activeSessions, err := m.sessionStore.ListActiveSessionsByContentHash(contentHash)
	if err == nil && len(activeSessions) > 0 {
		// Check for a usable session
		for _, session := range activeSessions {
			if session.Status == "running" || session.Status == "completed" {
				logger.Info("Found existing active session, reusing it",
					"sessionID", session.ID,
					"status", session.Status,
					"contentHash", contentHash)
				
				// Recreate handle for existing session
				handle := &plugins.TranscodeHandle{
					SessionID:  session.ID,
					Provider:   session.Provider,
					StartTime:  session.CreatedAt,
					Status:     plugins.TranscodeStatus(session.Status),
					Directory:  filepath.Join(m.config.TranscodingDir, "sessions", session.ID),
					Context:    ctx,
				}
				
				return handle, nil
			}
		}
	}

	// Use resource manager to handle the transcoding request
	return m.resourceManager.StartTranscode(ctx, req, m.executeTranscode)
}

// executeTranscode is the actual transcoding execution function called by the resource manager
func (m *Manager) executeTranscode(ctx context.Context, req plugins.TranscodeRequest) (*plugins.TranscodeHandle, error) {
	// Select provider based on request
	provider, err := m.selectProvider(req)
	if err != nil {
		return nil, tErrors.Wrap(err, tErrors.ErrorTypeProvider, "select_provider")
	}

	// Create session in database first
	providerInfo := provider.GetInfo()
	dbSession, err := m.sessionStore.CreateSession(providerInfo.ID, &req)
	if err != nil {
		return nil, tErrors.SessionError("create_session", err).
			WithDetail("provider", providerInfo.ID).
			WithDetail("media_id", req.MediaID)
	}

	// Update request with database-generated session ID
	req.SessionID = dbSession.ID

	// Start transcoding with selected provider
	handle, err := provider.StartTranscode(ctx, req)
	if err != nil {
		// Mark session as failed if provider fails to start
		m.sessionStore.FailSession(dbSession.ID, err)
		return nil, tErrors.TranscodeError("start_transcode", err).
			WithSession(dbSession.ID).
			WithDetail("provider", providerInfo.ID)
	}

	// Update database session status to running
	if err := m.sessionStore.UpdateSessionStatus(req.SessionID, "running", ""); err != nil {
		m.logger.Warn("failed to update session status", "sessionID", req.SessionID, "error", err)
	}

	// Emit event
	if m.eventBus != nil {
		event := events.NewEventWithData(
			events.EventInfo,
			"transcoding",
			"Transcoding Started",
			fmt.Sprintf("Transcoding started for media %s", req.MediaID),
			map[string]interface{}{
				"sessionId": req.SessionID,
				"mediaId":   req.MediaID,
				"provider":  handle.Provider,
			},
		)
		m.eventBus.PublishAsync(event)
	}

	// Start progress monitoring with safe goroutine
	tErrors.SafeGoContext(m.ctx, m.errorReporter, m.logger, "progress-monitor", func(ctx context.Context) error {
		m.monitorProgress(ctx, req.SessionID, handle, provider)
		return nil
	})

	return handle, nil
}

// selectProvider chooses the best provider for the request.
// It delegates to the provider registry for selection logic.
func (m *Manager) selectProvider(req plugins.TranscodeRequest) (plugins.TranscodingProvider, error) {
	return m.providerRegistry.SelectProvider(req)
}

// GetProgress returns the progress of a transcoding session.
// It retrieves the session from the database and queries the provider for current progress.
func (m *Manager) GetProgress(sessionID string) (*plugins.TranscodingProgress, error) {
	// Get session from database
	session, err := m.sessionStore.GetSession(sessionID)
	if err != nil {
		return nil, tErrors.SessionError("get_progress", err).
			WithSession(sessionID)
	}

	// Get provider
	provider, err := m.GetProvider(session.Provider)
	if err != nil {
		return nil, tErrors.ProviderError("get_progress", err).
			WithSession(sessionID).
			WithDetail("provider_id", session.Provider)
	}

	// Recreate handle for progress query
	handle := &plugins.TranscodeHandle{
		SessionID: sessionID,
		Provider:  session.Provider,
		Status:    plugins.TranscodeStatus(session.Status),
	}

	return provider.GetProgress(handle)
}

// GetProvider returns a specific provider by ID.
func (m *Manager) GetProvider(providerID string) (plugins.TranscodingProvider, error) {
	return m.providerRegistry.GetProvider(providerID)
}

// monitorProgress monitors the progress of a transcoding session.
// It periodically queries the provider for progress and updates the database.
func (m *Manager) monitorProgress(ctx context.Context, sessionID string, handle *plugins.TranscodeHandle, provider plugins.TranscodingProvider) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			progress, err := provider.GetProgress(handle)
			if err != nil {
				// Report error but continue monitoring
				tErr := tErrors.TranscodeError("get_progress", err).
					WithSession(sessionID)
				m.errorReporter.ReportError(ctx, tErr)
				continue
			}

			// Update progress in database
			if err := m.sessionStore.UpdateProgress(sessionID, progress); err != nil {
				tErr := tErrors.SessionError("update_progress", err).
					WithSession(sessionID)
				m.errorReporter.ReportError(ctx, tErr)
			}

			// Update resource manager with current status
			if m.resourceManager != nil {
				m.resourceManager.UpdateSessionStatus(sessionID, handle.Status)
			}

			// Check if completed
			if handle.Status == plugins.TranscodeStatusCompleted ||
				handle.Status == plugins.TranscodeStatusFailed ||
				handle.Status == plugins.TranscodeStatusCancelled {
				
				// Update final status in database
				switch handle.Status {
				case plugins.TranscodeStatusCompleted:
					// Create a success result
					result := &plugins.TranscodeResult{
						Success: true,
					}
					m.sessionStore.CompleteSession(sessionID, result)
				case plugins.TranscodeStatusFailed:
					m.sessionStore.FailSession(sessionID, fmt.Errorf("transcoding failed"))
				}
				
				return
			}

		case <-ctx.Done():
		return
		}
	}
}

// GetContentStore returns the content store for content-addressable storage
func (m *Manager) GetContentStore() services.ContentStore {
	return storage.NewContentStoreWrapper(m.contentStore)
}

// GetSessionStore returns the session store for session management
func (m *Manager) GetSessionStore() services.SessionStore {
	return m.sessionStore
}

// SetPluginManager updates the plugin manager
func (m *Manager) SetPluginManager(pluginManager types.PluginManagerInterface) {
	m.pluginManager = pluginManager

	// Re-discover providers if already initialized
	if m.initialized {
		if err := m.DiscoverProviders(); err != nil {
			logger.Error("Failed to re-discover providers: %v", err)
		}
	}
}

// GetMigrationService returns the content migration service
func (m *Manager) GetMigrationService() *migration.ContentMigrationService {
	return m.migrationService
}

// GetResourceUsage returns current resource usage statistics
func (m *Manager) GetResourceUsage() *resource.ResourceUsage {
	if m.resourceManager == nil {
		return nil
	}
	return m.resourceManager.GetResourceUsage()
}

// Shutdown gracefully shuts down the manager and all its components.
func (m *Manager) Shutdown() error {
	m.logger.Info("shutting down transcoding manager")

	// Signal shutdown to all goroutines
	if m.cancel != nil {
		m.cancel()
	}

	// Shutdown components in order
	if m.resourceManager != nil {
		m.resourceManager.Shutdown()
	}

	// Wait for all goroutines to finish
	m.wg.Wait()

	m.logger.Info("transcoding manager shutdown complete")
	return nil
}


// StopTranscode stops an active transcoding session.
func (m *Manager) StopTranscode(sessionID string) error {
	// Get session from database
	session, err := m.sessionStore.GetSession(sessionID)
	if err != nil {
		return tErrors.SessionError("stop_transcode", err).
			WithSession(sessionID)
	}

	// Get provider
	provider, err := m.GetProvider(session.Provider)
	if err != nil {
		return tErrors.ProviderError("stop_transcode", err).
			WithSession(sessionID).
			WithDetail("provider_id", session.Provider)
	}

	// Recreate handle for stop operation
	handle := &plugins.TranscodeHandle{
		SessionID: sessionID,
		Provider:  session.Provider,
		Status:    plugins.TranscodeStatus(session.Status),
	}

	// Stop transcoding
	if err := provider.StopTranscode(handle); err != nil {
		return tErrors.TranscodeError("stop_transcode", err).
			WithSession(sessionID).
			WithDetail("provider", session.Provider)
	}

	// Update session status in database
	if err := m.sessionStore.UpdateSessionStatus(sessionID, "cancelled", "User cancelled"); err != nil {
		// Log but don't fail - transcoding was stopped
		tErr := tErrors.SessionError("update_session_status", err).
			WithSession(sessionID)
		m.logger.Warn("failed to update session status", 
			"error", tErr,
			"sessionID", sessionID)
	}

	// Remove from resource manager
	if m.resourceManager != nil {
		m.resourceManager.RemoveSession(sessionID)
	}

	// Emit event
	if m.eventBus != nil {
		event := events.NewEventWithData(
			events.EventInfo,
			"transcoding",
			"Transcoding Stopped",
			fmt.Sprintf("Transcoding stopped for session %s", sessionID),
			map[string]interface{}{
				"sessionId": sessionID,
			},
		)
		m.eventBus.PublishAsync(event)
	}

	return nil
}

// GetProviders returns information about all available providers.
func (m *Manager) GetProviders() []plugins.ProviderInfo {
	return m.providerRegistry.GetProviders()
}

// GetSession returns information about a specific session.
func (m *Manager) GetSession(sessionID string) (*types.SessionInfo, error) {
	session, err := m.sessionStore.GetSession(sessionID)
	if err != nil {
		return nil, tErrors.SessionError("get_session", err).
			WithSession(sessionID)
	}

	// Convert database session to SessionInfo
	info := &types.SessionInfo{
		SessionID:   session.ID,
		Provider:    session.Provider,
		Status:      plugins.TranscodeStatus(session.Status),
		StartTime:   session.StartTime,
		ContentHash: session.ContentHash,
		Directory:   session.DirectoryPath,
	}

	// Parse request to get additional info
	var req plugins.TranscodeRequest
	if err := json.Unmarshal([]byte(session.Request), &req); err == nil {
		info.MediaID = req.MediaID
		info.Container = req.Container
	}

	return info, nil
}

// GetAllSessions returns all active sessions.
func (m *Manager) GetAllSessions() []*types.SessionInfo {
	sessions, err := m.sessionStore.GetActiveSessions()
	if err != nil {
		m.logger.Error("failed to get active sessions", "error", err)
		return []*types.SessionInfo{}
	}

	result := make([]*types.SessionInfo, 0, len(sessions))
	for _, session := range sessions {
		info := &types.SessionInfo{
			SessionID:   session.ID,
			Provider:    session.Provider,
			Status:      plugins.TranscodeStatus(session.Status),
			StartTime:   session.StartTime,
			ContentHash: session.ContentHash,
			Directory:   session.DirectoryPath,
		}

		// Parse request to get additional info
		var req plugins.TranscodeRequest
		if err := json.Unmarshal([]byte(session.Request), &req); err == nil {
			info.MediaID = req.MediaID
			info.Container = req.Container
		}

		result = append(result, info)
	}

	return result
}

// GetPipelineStatus returns the status of the pipeline provider.
// This provides statistics about the file-based transcoding provider.
func (m *Manager) GetPipelineStatus() *types.PipelineStatus {
	// Find the file pipeline provider using registry
	_, err := m.providerRegistry.GetProvider("file_pipeline")

	status := &types.PipelineStatus{
		Available:        err == nil,
		SupportedFormats: []string{"mp4", "mkv"},
		ActiveJobs:       0,
		CompletedJobs:    0,
		FailedJobs:       0,
	}

	if err != nil {
		return status
	}

	// Count jobs from active sessions
	sessions := m.GetAllSessions()
	for _, session := range sessions {
		if session.Provider == "file_pipeline" {
			switch session.Status {
			case plugins.TranscodeStatusRunning, plugins.TranscodeStatusStarting:
				status.ActiveJobs++
			case plugins.TranscodeStatusCompleted:
				status.CompletedJobs++
			case plugins.TranscodeStatusFailed:
				status.FailedJobs++
			}
		}
	}

	return status
}
