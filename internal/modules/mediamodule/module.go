package mediamodule

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/mediamodule/api"
	"github.com/mantonx/viewra/internal/modules/mediamodule/core/library"
	"github.com/mantonx/viewra/internal/modules/mediamodule/core/metadata"
	"github.com/mantonx/viewra/internal/modules/mediamodule/service"
	"github.com/mantonx/viewra/internal/modules/modulemanager"
	"github.com/mantonx/viewra/internal/services"
	"gorm.io/gorm"
)

// Auto-register the module when imported
func init() {
	Register()
}

const (
	// ModuleID is the unique identifier for the media module
	ModuleID = "system.media"

	// ModuleName is the display name for the media module
	ModuleName = "Media Manager"

	// ModuleVersion is the version of the media module
	ModuleVersion = "1.0.0"
)

// Module implements the media functionality as a module
type Module struct {
	db          *gorm.DB
	eventBus    events.EventBus
	
	// Core components
	libraryManager  *library.Manager
	metadataManager *metadata.Manager
	
	// Service implementation
	service services.MediaService
}

// Register registers the media module with the module system
func Register() {
	mediaModule := &Module{}
	modulemanager.Register(mediaModule)
}

// ID returns the unique module identifier
func (m *Module) ID() string {
	return ModuleID
}

// Name returns the module display name
func (m *Module) Name() string {
	return ModuleName
}

// Core returns whether this is a core module
func (m *Module) Core() bool {
	return true
}

// Migrate performs database migrations
// NOTE: Currently using shared database models to avoid conflicts.
// TODO: Migrate to module-specific models in models/ directory in the future
// when service layer is implemented across all modules.
func (m *Module) Migrate(db *gorm.DB) error {
	logger.Info("Migrating media database schema")
	
	// Using shared database models for now to avoid conflicts
	if err := db.AutoMigrate(
		&database.MediaLibrary{},
		&database.MediaFile{},
		&database.Movie{},
		&database.TVShow{},
		&database.Season{},
		&database.Episode{},
		&database.Album{},
		&database.Track{},
	); err != nil {
		return fmt.Errorf("failed to migrate media models: %w", err)
	}
	
	return nil
}

// Init initializes the media module
func (m *Module) Init() error {
	logger.Info("Initializing media module")
	
	// Get database if not set
	if m.db == nil {
		m.db = database.GetDB()
	}
	
	// Get event bus if not set
	if m.eventBus == nil {
		m.eventBus = events.GetGlobalEventBus()
	}
	
	// Initialize core components
	m.libraryManager = library.NewManager(m.db)
	m.metadataManager = metadata.NewManager(m.db)
	
	// Create and register the media service
	m.service = service.NewMediaService(m.db, m.libraryManager, m.metadataManager)
	if err := services.Register("media", m.service); err != nil {
		return fmt.Errorf("failed to register media service: %w", err)
	}
	
	logger.Info("Media service registered with service registry")
	
	return nil
}

// RegisterRoutes registers HTTP routes
func (m *Module) RegisterRoutes(router *gin.Engine) {
	logger.Info("Registering media module routes")
	
	// Create API handler
	handler := api.NewHandler(m.service, m.libraryManager)
	
	// Register routes
	api.RegisterRoutes(router, handler)
	
	logger.Info("Media module routes registered successfully")
}

// Shutdown gracefully shuts down the module
func (m *Module) Shutdown(ctx context.Context) error {
	logger.Info("Shutting down media module")
	
	// Cleanup if needed
	
	logger.Info("Media module shut down successfully")
	return nil
}

// Dependencies returns module dependencies
func (m *Module) Dependencies() []string {
	return []string{
		"system.database",
		"system.events",
		"system.scanner", // We use scanner service for library scanning
	}
}

// RequiredServices returns services this module requires
func (m *Module) RequiredServices() []string {
	return []string{"scanner"} // Need scanner service for ScanLibrary
}