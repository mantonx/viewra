// Package transcodingmodule manages all video transcoding operations for Viewra.
// It provides a unified interface for multiple transcoding providers, handles
// session lifecycle, and implements streaming-first transcoding for adaptive playback.
//
// The module supports:
// - Multiple transcoding providers (software, hardware-accelerated)
// - Streaming pipeline with real-time segment generation
// - Content-addressable storage with deduplication
// - Progress tracking and session management
// - Automatic cleanup and resource management
//
// Architecture:
//
//	MediaModule → PlaybackModule → TranscodingModule → Providers
//
// The transcoding module is responsible for:
// - Managing transcoding sessions and their lifecycle
// - Coordinating with transcoding providers (plugins)
// - Implementing the streaming pipeline provider for DASH/HLS
// - Managing segment-based storage and cleanup
// - Providing progress updates and status tracking
package transcodingmodule

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/modulemanager"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/api"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/types"
	"github.com/mantonx/viewra/internal/services"
	plugins "github.com/mantonx/viewra/sdk"
	"gorm.io/gorm"
)

// Auto-register the module when imported
func init() {
	Register()
}

const (
	// ModuleID is the unique identifier for the transcoding module
	ModuleID = "system.transcoding"

	// ModuleName is the display name for the transcoding module
	ModuleName = "Transcoding Manager"

	// ModuleVersion is the version of the transcoding module
	ModuleVersion = "1.0.0"
)

// Module implements the transcoding functionality as a module
type Module struct {
	manager      *Manager
	db           *gorm.DB
	eventBus     events.EventBus
	pluginModule types.PluginManagerInterface
	injectedServices map[string]interface{} // Services injected by module manager
}

// NewModule creates a new transcoding module
func NewModule(db *gorm.DB, eventBus events.EventBus, pluginModule types.PluginManagerInterface) *Module {
	return &Module{
		db:           db,
		eventBus:     eventBus,
		pluginModule: pluginModule,
	}
}

// ID returns the unique module identifier
func (m *Module) ID() string {
	return ModuleID
}

// Name returns the module display name
func (m *Module) Name() string {
	return ModuleName
}

// GetVersion returns the module version
func (m *Module) GetVersion() string {
	return ModuleVersion
}

// Core returns whether this is a core module
func (m *Module) Core() bool {
	return true // Transcoding is a core module
}

// Migrate performs any necessary database migrations
func (m *Module) Migrate(db *gorm.DB) error {
	logger.Info("Migrating transcoding database schema")

	// Migrate transcode session models
	if err := db.AutoMigrate(&database.TranscodeSession{}); err != nil {
		return fmt.Errorf("failed to migrate TranscodeSession: %w", err)
	}

	return nil
}

// Initialize sets up the transcoding module
func (m *Module) Initialize() error {
	logger.Info("Initializing transcoding module")
	return nil
}

// Init initializes the transcoding module with dependencies
func (m *Module) Init() error {
	logger.Info("Initializing transcoding module components")

	if m.db == nil {
		logger.Error("Transcoding module db is nil")
		m.db = database.GetDB()
	}

	if m.eventBus == nil {
		logger.Error("Transcoding module eventBus is nil")
		m.eventBus = events.GetGlobalEventBus()
	}

	// Get plugin service from injected services if we don't have it yet
	if m.pluginModule == nil && m.injectedServices != nil {
		if pluginService, ok := m.injectedServices["plugin"]; ok {
			// Create adapter from plugin service
			m.pluginModule = &pluginServiceAdapter{service: pluginService}
			logger.Info("Using injected plugin service")
		}
		
		if m.pluginModule == nil {
			logger.Warn("Plugin service not available in injected services")
		}
	}

	// Create manager with default configuration
	logger.Info("Creating transcoding manager", "pluginModule_nil", m.pluginModule == nil)
	manager, err := NewManager(m.db, m.eventBus, m.pluginModule, nil)
	if err != nil {
		logger.Error("Failed to create transcoding manager: %v", err)
		return fmt.Errorf("failed to create transcoding manager: %w", err)
	}
	m.manager = manager

	if m.manager == nil {
		logger.Error("Failed to create transcoding manager")
		return fmt.Errorf("failed to create transcoding manager")
	}

	// Initialize the manager
	if err := m.manager.Initialize(); err != nil {
		logger.Error("Failed to initialize transcoding manager: %v", err)
		return fmt.Errorf("failed to initialize transcoding manager: %w", err)
	}

	// Register the TranscodingService with the service registry
	transcodingService := NewTranscodingServiceImpl(m.manager)
	services.Register("transcoding", transcodingService)
	logger.Info("TranscodingService registered with service registry")

	// Subscribe to transcode request events
	m.subscribeToEvents()

	logger.Info("Transcoding module initialized successfully")
	return nil
}

// RegisterRoutes registers all transcoding module HTTP routes
func (m *Module) RegisterRoutes(router *gin.Engine) {
	logger.Info("Registering transcoding module routes")

	if m.manager == nil {
		logger.Error("Cannot register routes: transcoding manager is nil")
		return
	}

	// Create API handler instance
	handler := api.NewAPIHandler(m.manager)

	// Create content handler if content store is available
	var contentHandler *api.ContentAPIHandler
	if m.manager != nil {
		contentStore := m.manager.GetContentStore()
		sessionStore := m.manager.GetSessionStore()
		
		logger.Info("Checking stores for content handler", 
			"manager", m.manager != nil,
			"contentStore", contentStore != nil,
			"sessionStore", sessionStore != nil)
		
		if contentStore != nil && sessionStore != nil {
			contentHandler = api.NewContentAPIHandler(contentStore, sessionStore)
			logger.Info("Content API handler created with session fallback")
		} else {
			logger.Warn("Content store or session store not available, content routes will not be registered",
				"contentStore", contentStore != nil,
				"sessionStore", sessionStore != nil)
		}
	} else {
		logger.Warn("Manager not available for content handler")
	}

	// Register all routes from routes.go
	api.RegisterRoutes(router, handler, contentHandler)

	logger.Info("Transcoding module routes registered successfully")
}

// Shutdown gracefully shuts down the module
func (m *Module) Shutdown(ctx context.Context) error {
	logger.Info("Shutting down transcoding module")

	if m.manager == nil {
		return nil
	}

	// Shutdown the manager
	if err := m.manager.Shutdown(); err != nil {
		logger.Error("Error shutting down transcoding manager: %v", err)
		return err
	}

	logger.Info("Transcoding module shut down successfully")
	return nil
}



// ProvidedServices returns the list of services this module provides
func (m *Module) ProvidedServices() []string {
	return []string{"transcoding"}
}

// Dependencies returns the list of module IDs this module depends on
func (m *Module) Dependencies() []string {
	return []string{
		"system.database", // For session storage
		"system.events",   // For event notifications
		"system.plugins",  // For transcoding providers
	}
}

// RequiredServices returns the list of services this module requires
func (m *Module) RequiredServices() []string {
	return []string{"plugin"} // Requires plugin service to discover transcoding providers
}

// InjectServices implements the ServiceInjector interface
// This is called by the module manager to inject required services
func (m *Module) InjectServices(services map[string]interface{}) error {
	m.injectedServices = services
	
	// Extract plugin service if available
	if pluginService, ok := services["plugin"]; ok {
		m.pluginModule = &pluginServiceAdapter{service: pluginService}
		logger.Info("Plugin service injected successfully", "service_type", fmt.Sprintf("%T", pluginService))
	} else {
		logger.Warn("Plugin service not found in injected services", "available_services", len(services))
		for name := range services {
			logger.Debug("Available service", "name", name)
		}
	}
	
	return nil
}

// subscribeToEvents subscribes to relevant events for cross-module communication
func (m *Module) subscribeToEvents() {
	eventHandler := modulemanager.GetModuleEventHandler()

	// Subscribe to transcode request events
	_, err := eventHandler.SubscribeModule(ModuleID, events.EventFilter{
		Types: []events.EventType{events.EventTranscodeRequested},
	}, func(event events.Event) error {
		// Extract request data
		mediaFileID, ok := event.Data["media_file_id"].(string)
		if !ok {
			logger.Error("Invalid media_file_id in transcode request event")
			return fmt.Errorf("invalid media_file_id")
		}

		container, ok := event.Data["container"].(string)
		if !ok {
			container = "dash" // Default to DASH
		}

		// Device profile handling could be added here if needed
		// For now, we'll process without device-specific optimizations

		// Start transcoding session
		logger.Info("Handling transcode request from event",
			"media_file_id", mediaFileID,
			"container", container,
			"requester", event.Data["requester"])

		// Get media service to retrieve file path
		mediaService, err := services.GetMediaService()
		if err != nil {
			logger.Error("Failed to get media service", "error", err)
			return fmt.Errorf("media service not available: %w", err)
		}

		// Get the media file
		mediaFile, err := mediaService.GetFile(context.Background(), mediaFileID)
		if err != nil {
			logger.Error("Failed to get media file", "error", err)
			return fmt.Errorf("failed to get media file: %w", err)
		}

		// Create transcode request
		req := plugins.TranscodeRequest{
			MediaID:   mediaFileID,
			InputPath: mediaFile.Path,
			Container: container,
			EnableABR: getBool(event.Data, "enable_abr"),
		}

		// Start transcoding
		handle, err := m.manager.StartTranscode(context.Background(), req)

		if err != nil {
			logger.Error("Failed to start transcode from event", "error", err)
			// Publish transcode failed event
			failedEvent := events.NewTranscodeEvent(
				events.EventTranscodeFailed,
				"",
				mediaFileID,
				"failed",
			)
			failedEvent.Data["error"] = err.Error()
			eventHandler.PublishModuleEvent(ModuleID, failedEvent)
			return err
		}

		// Get session info from manager
		sessionInfo, err := m.manager.GetSession(handle.SessionID)
		if err != nil {
			logger.Warn("Failed to get session info", "error", err)
		}

		// Publish transcode started event
		startedEvent := events.NewTranscodeEvent(
			events.EventTranscodeRequested,
			handle.SessionID,
			mediaFileID,
			"started",
		)
		if sessionInfo != nil {
			startedEvent.Data["content_hash"] = sessionInfo.ContentHash
			startedEvent.Data["manifest_url"] = fmt.Sprintf("/api/v1/content/%s/manifest.%s", sessionInfo.ContentHash, container)
		}
		eventHandler.PublishModuleEvent(ModuleID, startedEvent)

		return nil
	})

	if err != nil {
		logger.Error("Failed to subscribe to transcode request events", "error", err)
	}
}

// Helper functions for event data extraction
func getString(data map[string]interface{}, key string) string {
	if val, ok := data[key].(string); ok {
		return val
	}
	return ""
}

func getInt(data interface{}, key string) int {
	switch v := data.(type) {
	case map[string]interface{}:
		if val, ok := v[key].(float64); ok {
			return int(val)
		}
		if val, ok := v[key].(int); ok {
			return val
		}
	}
	return 0
}

func getBool(data interface{}, key string) bool {
	switch v := data.(type) {
	case map[string]interface{}:
		if val, ok := v[key].(bool); ok {
			return val
		}
	case bool:
		return v
	}
	return false
}

// Register registers the transcoding module with the module system
func Register() {
	transcodingModule := &Module{
		db:       nil, // Will be set during Init
		eventBus: nil, // Will be set during Init
	}
	modulemanager.Register(transcodingModule)
}

// pluginServiceAdapter adapts the plugin service interface to PluginManagerInterface
type pluginServiceAdapter struct {
	service interface{} // Store as interface{} since services.PluginService isn't defined
}

// GetTranscodingProviders returns all available transcoding providers
func (a *pluginServiceAdapter) GetTranscodingProviders() []plugins.TranscodingProvider {
	if a.service == nil {
		return nil
	}
	// Use type assertion to call the method
	if ps, ok := a.service.(interface{ GetTranscodingProviders() []plugins.TranscodingProvider }); ok {
		return ps.GetTranscodingProviders()
	}
	return nil
}

// GetTranscodingProvider returns a specific provider by ID
func (a *pluginServiceAdapter) GetTranscodingProvider(id string) (plugins.TranscodingProvider, error) {
	if a.service == nil {
		return nil, fmt.Errorf("plugin service not available")
	}
	// For now, get all providers and find the one with matching ID
	providers := a.GetTranscodingProviders()
	for _, provider := range providers {
		info := provider.GetInfo()
		if info.ID == id {
			return provider, nil
		}
	}
	return nil, fmt.Errorf("provider not found: %s", id)
}
