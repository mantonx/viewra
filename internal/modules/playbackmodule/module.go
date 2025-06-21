package playbackmodule

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/modulemanager"
	"gorm.io/gorm"
)

// Auto-register the module when imported
func init() {
	Register()
}

const (
	// ModuleID is the unique identifier for the playback module
	ModuleID = "system.playback"

	// ModuleName is the display name for the playback module
	ModuleName = "Playback Manager"

	// ModuleVersion is the version of the playback module
	ModuleVersion = "2.0.0"
)

// Module implements the playback functionality as a module
type Module struct {
	manager      *Manager
	db           *gorm.DB
	eventBus     events.EventBus
	pluginModule PluginManagerInterface
}

// NewModule creates a new playback module
func NewModule(db *gorm.DB, eventBus events.EventBus, pluginModule PluginManagerInterface) *Module {
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
	return true // Playback is a core module
}

// IsInitialized returns whether the module is initialized
func (m *Module) IsInitialized() bool {
	return m.manager != nil && m.manager.initialized
}

// Migrate performs any necessary database migrations
func (m *Module) Migrate(db *gorm.DB) error {
	logger.Info("Migrating playback database schema")

	// Migrate transcode session models
	if err := db.AutoMigrate(&database.TranscodeSession{}); err != nil {
		return fmt.Errorf("failed to migrate TranscodeSession: %w", err)
	}

	// Any other playback-related models

	return nil
}

// Initialize sets up the playback module
func (m *Module) Initialize() error {
	logger.Info("Initializing playback module")

	// This is called early before dependencies are ready
	// Just mark that we're ready to be initialized
	return nil
}

// Init initializes the playback module with dependencies
func (m *Module) Init() error {
	logger.Info("Initializing playback module components")

	if m.db == nil {
		logger.Error("Playback module db is nil")
		m.db = database.GetDB()
	}

	if m.eventBus == nil {
		logger.Error("Playback module eventBus is nil")
		m.eventBus = events.GetGlobalEventBus()
	}

	// Create manager with default configuration
	// Configuration can be overridden via environment variables
	m.manager = NewManager(m.db, m.eventBus, m.pluginModule, nil)

	if m.manager == nil {
		logger.Error("Failed to create playback manager")
		return fmt.Errorf("failed to create playback manager")
	}

	// Initialize the manager
	if err := m.manager.Initialize(); err != nil {
		logger.Error("Failed to initialize playback manager: %v", err)
		return fmt.Errorf("failed to initialize playback manager: %w", err)
	}

	logger.Info("Playback module initialized successfully with manager: %v", m.manager)

	return nil
}

// RegisterRoutes registers all playback module HTTP routes
func (m *Module) RegisterRoutes(router *gin.Engine) {
	logger.Info("Registering playback module routes")

	if m.manager == nil {
		logger.Error("Cannot register routes: playback manager is nil")
		return
	}

	// Create API handler instance
	handler := NewAPIHandler(m.manager)

	// Register all routes from routes.go
	RegisterRoutes(router, handler)

	logger.Info("Playback module routes registered successfully")
}

// Shutdown gracefully shuts down the module
func (m *Module) Shutdown(ctx context.Context) error {
	logger.Info("Shutting down playback module")

	if m.manager == nil {
		return nil
	}

	// Shutdown the manager
	if err := m.manager.Shutdown(); err != nil {
		logger.Error("Error shutting down playback manager: %v", err)
		return err
	}

	logger.Info("Playback module shut down successfully")
	return nil
}

// GetManager returns the underlying playback manager
func (m *Module) GetManager() *Manager {
	if m.manager == nil {
		logger.Error("Manager is nil in GetManager()")

		// Try to re-initialize
		if m.db == nil {
			logger.Error("Module database is nil, getting from global database")
			m.db = database.GetDB()
		}

		if m.eventBus == nil {
			logger.Error("Module eventBus is nil, getting from global event bus")
			m.eventBus = events.GetGlobalEventBus()
		}

		if m.db == nil {
			logger.Error("CRITICAL: Cannot create playback manager - database connection is nil")
			return nil
		}

		m.manager = NewManager(m.db, m.eventBus, m.pluginModule, nil)
		logger.Info("Re-initialized playback manager: %v", m.manager)

		// Initialize it
		if m.manager != nil {
			if err := m.manager.Initialize(); err != nil {
				logger.Error("Failed to initialize re-created manager: %v", err)
			}
		}

		// Double-check that the manager was created properly
		if m.manager == nil {
			logger.Error("CRITICAL: Failed to create playback manager")
			return nil
		}
	}

	return m.manager
}

// SetPluginModule sets the plugin module for the playback system
func (m *Module) SetPluginModule(pluginModule PluginManagerInterface) {
	logger.Info("SetPluginModule called", "pluginModule_nil", pluginModule == nil)
	m.pluginModule = pluginModule

	// Update manager if it exists
	if m.manager != nil {
		logger.Info("Updating manager with plugin module")
		m.manager.SetPluginManager(pluginModule)
	} else {
		logger.Warn("Manager is nil when setting plugin module")
	}
}

// Register registers the playback module with the module system
func Register() {
	playbackModule := &Module{
		db:       nil, // Will be set during Init
		eventBus: nil, // Will be set during Init
	}
	modulemanager.Register(playbackModule)
}

// Legacy compatibility methods for external code that might use these

// GetPlaybackCore returns the core playback manager (legacy compatibility)
func (m *Module) GetPlaybackCore() interface{} {
	return m.manager
}

// SetPlaybackCore sets the core playback manager (legacy compatibility)
func (m *Module) SetPlaybackCore(core interface{}) {
	// This is a no-op now since we manage our own manager
	logger.Warn("SetPlaybackCore called - this is deprecated and has no effect")
}
