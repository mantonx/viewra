// Package transcodingmodule provides video transcoding functionality.
// This is the main manager that coordinates all transcoding operations.
package transcodingmodule

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/cleanup"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/migration"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/pipeline"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/session"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/storage"

	"github.com/mantonx/viewra/internal/modules/transcodingmodule/types"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/utils/filemanager"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/utils/paths"
	"github.com/mantonx/viewra/internal/services"
	plugins "github.com/mantonx/viewra/sdk"
	"gorm.io/gorm"
)

// Manager coordinates all transcoding operations within the module.
type Manager struct {
	db            *gorm.DB
	eventBus      events.EventBus
	pluginManager types.PluginManagerInterface
	config        *types.Config

	// Provider management
	providers      map[string]plugins.TranscodingProvider
	providersMutex sync.RWMutex

	// Session management
	sessions       map[string]*types.SessionInfo
	sessionHandles map[string]*plugins.TranscodeHandle
	sessionsMutex  sync.RWMutex
	sessionStore   *session.SessionStore

	// Pipeline provider (built-in streaming provider)
	pipelineProvider plugins.TranscodingProvider

	// Content store
	contentStore *storage.ContentStore

	// Services
	cleanupService   *cleanup.Service
	fileManager      *filemanager.FileManager
	migrationService *migration.ContentMigrationService
	logger           hclog.Logger

	// Lifecycle
	initialized bool
	shutdownCh  chan struct{}
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
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

	// Create session store
	sessionStore := session.NewSessionStore(db, hclogger.Named("session-store"))

	// Create file manager
	fileManager := filemanager.NewFileManager(config.TranscodingDir, hclogger.Named("file-manager"))

	// Create content store
	contentStore, err := storage.NewContentStore(config.TranscodingDir, hclogger.Named("content-store"))
	if err != nil {
		cancel() // Clean up context
		return nil, fmt.Errorf("failed to create content store: %w", err)
	}

	// Create content migration service
	migrationService := migration.NewContentMigrationService(db)

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

	return &Manager{
		db:               db,
		eventBus:         eventBus,
		pluginManager:    pluginManager,
		config:           config,
		providers:        make(map[string]plugins.TranscodingProvider),
		sessions:         make(map[string]*types.SessionInfo),
		sessionHandles:   make(map[string]*plugins.TranscodeHandle),
		sessionStore:     sessionStore,
		fileManager:      fileManager,
		contentStore:     contentStore,
		cleanupService:   cleanupService,
		migrationService: migrationService,
		logger:           hclogger,
		shutdownCh:       make(chan struct{}),
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
		return fmt.Errorf("failed to create transcoding directory: %w", err)
	}

	// Register built-in providers
	m.registerBuiltinProviders()

	// Discover plugin providers
	if err := m.DiscoverProviders(); err != nil {
		logger.Error("Failed to discover providers: %v", err)
		// Not fatal - continue with available providers
	}

	// Start cleanup service
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.cleanupService.Run(m.ctx)
	}()

	m.initialized = true
	logger.Info("Transcoding manager initialized with %d providers", len(m.providers))

	return nil
}

// registerBuiltinProviders registers the built-in transcoding providers
func (m *Manager) registerBuiltinProviders() {
	m.providersMutex.Lock()
	defer m.providersMutex.Unlock()

	// Register streaming pipeline provider with content hash callback
	contentHashCallback := m.migrationService.CreateContentMigrationCallback()
	pipelineProvider := pipeline.NewProviderWithCallback(m.config.TranscodingDir, contentHashCallback)

	// Initialize the provider with required services
	pipelineProvider.Initialize(m.sessionStore, m.contentStore)

	m.pipelineProvider = pipelineProvider

	logger.Info("Registered built-in streaming provider")
}

// DiscoverProviders finds all available transcoding providers
func (m *Manager) DiscoverProviders() error {
	if m.pluginManager == nil {
		return fmt.Errorf("plugin manager not available")
	}

	providers := m.pluginManager.GetTranscodingProviders()

	m.providersMutex.Lock()
	defer m.providersMutex.Unlock()

	for _, provider := range providers {
		info := provider.GetInfo()
		m.providers[info.ID] = provider
		logger.Info("Registered transcoding provider", "id", info.ID, "name", info.Name)
	}

	return nil
}

// StartTranscode initiates a new transcoding session
func (m *Manager) StartTranscode(ctx context.Context, req plugins.TranscodeRequest) (*plugins.TranscodeHandle, error) {
	// Select provider based on request
	provider, err := m.selectProvider(req)
	if err != nil {
		return nil, fmt.Errorf("failed to select provider: %w", err)
	}

	// Generate session ID if not provided
	if req.SessionID == "" {
		req.SessionID = generateSessionID()
	}

	// Create session in database first
	providerInfo := provider.GetInfo()
	dbSession, err := m.sessionStore.CreateSession(providerInfo.ID, &req)
	if err != nil {
		return nil, fmt.Errorf("failed to create session in database: %w", err)
	}

	// Update request with database-generated session ID
	req.SessionID = dbSession.ID

	// Start transcoding with selected provider
	handle, err := provider.StartTranscode(ctx, req)
	if err != nil {
		// Mark session as failed if provider fails to start
		m.sessionStore.FailSession(dbSession.ID, err)
		return nil, fmt.Errorf("provider failed to start transcode: %w", err)
	}

	// Create session info
	info := &types.SessionInfo{
		SessionID:   req.SessionID,
		MediaID:     req.MediaID,
		Provider:    handle.Provider,
		Container:   req.Container,
		Status:      handle.Status,
		Progress:    0,
		StartTime:   handle.StartTime,
		Directory:   handle.Directory,
		ContentHash: dbSession.ContentHash,
	}

	// Store session
	m.sessionsMutex.Lock()
	m.sessions[req.SessionID] = info
	m.sessionHandles[req.SessionID] = handle
	m.sessionsMutex.Unlock()

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

	// Start progress monitoring
	go m.monitorProgress(req.SessionID, handle, provider)

	return handle, nil
}

// selectProvider chooses the best provider for the request
func (m *Manager) selectProvider(req plugins.TranscodeRequest) (plugins.TranscodingProvider, error) {
	m.providersMutex.RLock()
	defer m.providersMutex.RUnlock()

	// Streaming-first architecture: only support DASH/HLS through pipeline provider
	if req.Container == "dash" || req.Container == "hls" {
		if m.pipelineProvider != nil {
			return m.pipelineProvider, nil
		}
		return nil, fmt.Errorf("streaming pipeline provider not available")
	}

	// Check external plugin providers for other formats
	var candidates []plugins.TranscodingProvider
	for _, provider := range m.providers {
		formats := provider.GetSupportedFormats()
		for _, format := range formats {
			if format.Format == req.Container {
				candidates = append(candidates, provider)
				break
			}
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no provider supports format: %s - only streaming formats (dash/hls) are supported natively", req.Container)
	}

	// Select provider with highest priority
	var selected plugins.TranscodingProvider
	highestPriority := -1

	for _, provider := range candidates {
		info := provider.GetInfo()
		if info.Priority > highestPriority {
			selected = provider
			highestPriority = info.Priority
		}
	}

	return selected, nil
}

// GetProgress returns the progress of a transcoding session
func (m *Manager) GetProgress(sessionID string) (*plugins.TranscodingProgress, error) {
	m.sessionsMutex.RLock()
	handle, exists := m.sessionHandles[sessionID]
	info := m.sessions[sessionID]
	m.sessionsMutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Get provider
	provider, err := m.GetProvider(info.Provider)
	if err != nil {
		return nil, fmt.Errorf("provider not found: %s", info.Provider)
	}

	return provider.GetProgress(handle)
}

// GetProvider returns a specific provider by ID
func (m *Manager) GetProvider(providerID string) (plugins.TranscodingProvider, error) {
	m.providersMutex.RLock()
	defer m.providersMutex.RUnlock()

	// Check pipeline provider first
	if m.pipelineProvider != nil {
		info := m.pipelineProvider.GetInfo()
		if info.ID == providerID {
			return m.pipelineProvider, nil
		}
	}

	provider, exists := m.providers[providerID]
	if !exists {
		return nil, fmt.Errorf("provider not found: %s", providerID)
	}

	return provider, nil
}

// monitorProgress monitors the progress of a transcoding session
func (m *Manager) monitorProgress(sessionID string, handle *plugins.TranscodeHandle, provider plugins.TranscodingProvider) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			progress, err := provider.GetProgress(handle)
			if err != nil {
				logger.Warn("Failed to get progress", "sessionId", sessionID, "error", err)
				continue
			}

			// Update session info
			m.sessionsMutex.Lock()
			if info, exists := m.sessions[sessionID]; exists {
				info.Progress = progress.PercentComplete
				info.Status = handle.Status
			}
			m.sessionsMutex.Unlock()

			// Check if completed
			if handle.Status == plugins.TranscodeStatusCompleted ||
				handle.Status == plugins.TranscodeStatusFailed ||
				handle.Status == plugins.TranscodeStatusCancelled {
				return
			}

		case <-m.shutdownCh:
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

// Shutdown gracefully shuts down the manager
func (m *Manager) Shutdown() error {
	logger.Info("Shutting down transcoding manager")

	// Signal shutdown
	if m.cancel != nil {
		m.cancel()
	}

	// Only close shutdownCh if it hasn't been closed already
	select {
	case <-m.shutdownCh:
		// Already closed
	default:
		close(m.shutdownCh)
	}

	// Wait for routines to finish
	m.wg.Wait()

	return nil
}

// generateSessionID creates a unique session ID
func generateSessionID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// StopTranscode stops an active transcoding session
func (m *Manager) StopTranscode(sessionID string) error {
	m.sessionsMutex.RLock()
	handle, exists := m.sessionHandles[sessionID]
	info := m.sessions[sessionID]
	m.sessionsMutex.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Get provider
	provider, err := m.GetProvider(info.Provider)
	if err != nil {
		return fmt.Errorf("provider not found: %s", info.Provider)
	}

	// Stop transcoding
	if err := provider.StopTranscode(handle); err != nil {
		return fmt.Errorf("failed to stop transcode: %w", err)
	}

	// Update session status
	m.sessionsMutex.Lock()
	if session, exists := m.sessions[sessionID]; exists {
		session.Status = plugins.TranscodeStatusCancelled
	}
	m.sessionsMutex.Unlock()

	// Emit event
	if m.eventBus != nil {
		event := events.NewEvent(
			events.EventInfo,
			"transcoding",
			"Transcoding Stopped",
			fmt.Sprintf("Transcoding stopped for session %s", sessionID),
		)
		event.Data["sessionId"] = sessionID
		m.eventBus.PublishAsync(event)
	}

	return nil
}

// GetProviders returns information about all available providers
func (m *Manager) GetProviders() []plugins.ProviderInfo {
	m.providersMutex.RLock()
	defer m.providersMutex.RUnlock()

	providers := make([]plugins.ProviderInfo, 0, len(m.providers))
	for _, provider := range m.providers {
		providers = append(providers, provider.GetInfo())
	}

	// Add pipeline provider if available
	if m.pipelineProvider != nil {
		providers = append(providers, m.pipelineProvider.GetInfo())
	}

	return providers
}

// GetSession returns information about a specific session
func (m *Manager) GetSession(sessionID string) (*types.SessionInfo, error) {
	m.sessionsMutex.RLock()
	defer m.sessionsMutex.RUnlock()

	info, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return info, nil
}

// GetAllSessions returns all active sessions
func (m *Manager) GetAllSessions() []*types.SessionInfo {
	m.sessionsMutex.RLock()
	defer m.sessionsMutex.RUnlock()

	sessions := make([]*types.SessionInfo, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}

	return sessions
}

// GetPipelineStatus returns the status of the pipeline provider
func (m *Manager) GetPipelineStatus() *types.PipelineStatus {
	status := &types.PipelineStatus{
		Available:        m.pipelineProvider != nil,
		SupportedFormats: []string{"dash", "hls"},
		ActiveJobs:       0,
		CompletedJobs:    0,
		FailedJobs:       0,
	}

	if m.pipelineProvider == nil {
		return status
	}

	// Count active, completed, and failed jobs from sessions
	m.sessionsMutex.RLock()
	for _, session := range m.sessions {
		if session.Provider == "streaming_pipeline" {
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
	m.sessionsMutex.RUnlock()

	return status
}
