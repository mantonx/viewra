package mediamodule

import (
	"context"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/modules/modulemanager"
	"github.com/mantonx/viewra/internal/modules/pluginmodule"

	"gorm.io/gorm"
)

// Module represents the Media Management module
type Module struct {
	id           string
	name         string
	version      string
	core         bool
	initialized  bool
	db           *gorm.DB
	eventBus     events.EventBus
	pluginModule *pluginmodule.PluginModule

	// Media management components
	libraryManager  *LibraryManager
	fileProcessor   *FileProcessor
	metadataManager *MetadataManager
}

// Auto-register the module when imported
func init() {
	Register()
}

// Register registers this module with the module system
func Register() {
	// Create module without database connection - it will be initialized later
	mediaModule := &Module{
		id:      "system.media",
		name:    "Media Manager",
		version: "1.0.0",
		core:    true,
	}
	modulemanager.Register(mediaModule)
}

// ID returns the module ID
func (m *Module) ID() string {
	return m.id
}

// Name returns the module name
func (m *Module) Name() string {
	return m.name
}

// GetVersion returns the module version
func (m *Module) GetVersion() string {
	return m.version
}

// Core returns whether this is a core module
func (m *Module) Core() bool {
	return m.core
}

// IsInitialized returns whether the module is initialized
func (m *Module) IsInitialized() bool {
	return m.initialized
}

// Initialize sets up the media module
func (m *Module) Initialize() error {
	log.Println("INFO: Migrating media module schema")

	// Auto-migrate media-related models
	err := m.db.AutoMigrate(
		&database.MediaLibrary{},
		&database.MediaFile{},
		&database.MediaAsset{},
		&database.People{},
		&database.Roles{},
		&database.Artist{},
		&database.Album{},
		&database.Track{},
		&database.Movie{},
		&database.TVShow{},
		&database.Season{},
		&database.Episode{},
		&database.MediaExternalIDs{},
		&database.MediaEnrichment{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate media schema: %w", err)
	}

	return nil
}

// Migrate performs any pending migrations
func (m *Module) Migrate(db *gorm.DB) error {
	log.Println("INFO: Migrating media module schema")

	// Auto-migrate media-related models
	err := db.AutoMigrate(
		&database.MediaLibrary{},
		&database.MediaFile{},
		&database.MediaAsset{},
		&database.People{},
		&database.Roles{},
		&database.Artist{},
		&database.Album{},
		&database.Track{},
		&database.Movie{},
		&database.TVShow{},
		&database.Season{},
		&database.Episode{},
		&database.MediaExternalIDs{},
		&database.MediaEnrichment{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate media schema: %w", err)
	}

	return nil
}

// Init initializes the media module components
func (m *Module) Init() error {
	log.Println("INFO: Initializing media module")

	// Get the database connection and event bus from the global system
	m.db = database.GetDB()
	m.eventBus = events.GetGlobalEventBus()

	// Initialize media management components
	if err := m.initializeComponents(); err != nil {
		return fmt.Errorf("failed to initialize media components: %w", err)
	}

	m.initialized = true

	// Publish initialization event
	if m.eventBus != nil {
		initEvent := events.NewSystemEvent(
			"media.module.initialized",
			"Media Module Initialized",
			"Media module has been successfully initialized",
		)
		m.eventBus.PublishAsync(initEvent)
	}

	log.Println("INFO: Media module initialized successfully")
	return nil
}

// initializeComponents initializes all media management components
func (m *Module) initializeComponents() error {
	log.Println("INFO: Initializing media library manager")
	m.libraryManager = NewLibraryManager(m.db, m.eventBus)
	if err := m.libraryManager.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize library manager: %w", err)
	}
	log.Println("INFO: Library manager initialized successfully")

	log.Println("INFO: Initializing media file processor")
	m.fileProcessor = NewFileProcessor(m.db, m.eventBus, m.pluginModule)
	if err := m.fileProcessor.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize file processor: %w", err)
	}
	if m.pluginModule != nil {
		log.Println("INFO: File processor initialized with plugin module support")
	} else {
		log.Println("INFO: File processor initialized without plugin module (limited functionality)")
	}

	log.Println("INFO: Initializing metadata manager")
	m.metadataManager = NewMetadataManager(m.db, m.eventBus, m.pluginModule)
	if err := m.metadataManager.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize metadata manager: %w", err)
	}
	if m.pluginModule != nil {
		log.Println("INFO: Metadata manager initialized with plugin module support")
	} else {
		log.Println("INFO: Metadata manager initialized without plugin module (limited functionality)")
	}

	return nil
}

// RegisterRoutes registers the media module API routes
func (m *Module) RegisterRoutes(router *gin.Engine) {
	log.Printf("INFO: Registering media module routes (initialized: %v, db: %v)", m.initialized, m.db != nil)

	mediaGroup := router.Group("/api/media")
	{
		// Library management endpoints
		mediaGroup.GET("/libraries", m.getLibraries)
		mediaGroup.POST("/libraries", m.createLibrary)
		mediaGroup.DELETE("/libraries/:id", m.deleteLibrary)
		mediaGroup.GET("/libraries/:id/stats", m.getLibraryStats)
		mediaGroup.GET("/libraries/:id/files", m.getLibraryFiles)

		// File management endpoints
		mediaGroup.GET("/files", m.getFiles)
		mediaGroup.GET("/files/:id", m.getFile)
		mediaGroup.DELETE("/files/:id", m.deleteFile)
		mediaGroup.GET("/files/:id/stream", m.streamFile)
		mediaGroup.HEAD("/files/:id/stream", m.streamFile) // Add explicit HEAD support
		mediaGroup.GET("/files/:id/manifest.m3u8", m.generateHLSManifest)
		mediaGroup.GET("/files/:id/transcode.mp4", m.transcodeToMP4) // Transcode MKV to MP4 for Shaka Player
		mediaGroup.GET("/files/:id/metadata", m.getFileMetadata)
		mediaGroup.GET("/files/:id/album-id", m.getFileAlbumId)
		mediaGroup.GET("/files/:id/album-artwork", m.getFileAlbumArtwork)

		// TV Shows endpoints
		mediaGroup.GET("/tv-shows", m.getTVShows)

		// Upload endpoints removed as app doesn't support uploads

		// Metadata endpoints
		mediaGroup.POST("/files/:id/metadata/extract", m.extractMetadata)
		mediaGroup.PUT("/files/:id/metadata", m.updateMetadata)

		// Processing endpoints
		mediaGroup.POST("/files/:id/process", m.processFile)
		mediaGroup.GET("/processing/status", m.getProcessingStatus)

		// Module status endpoints
		mediaGroup.GET("/health", m.getHealth)
		mediaGroup.GET("/status", m.getStatus)
		mediaGroup.GET("/stats", m.getStats)
	}
}

// Shutdown gracefully shuts down the media module
func (m *Module) Shutdown(ctx context.Context) error {
	log.Println("INFO: Shutting down media module")

	// Shutdown components in reverse order
	// Upload handler shutdown code removed

	if m.metadataManager != nil {
		if err := m.metadataManager.Shutdown(ctx); err != nil {
			log.Printf("ERROR: Failed to shutdown metadata manager: %v", err)
		}
	}

	if m.fileProcessor != nil {
		if err := m.fileProcessor.Shutdown(ctx); err != nil {
			log.Printf("ERROR: Failed to shutdown file processor: %v", err)
		}
	}

	if m.libraryManager != nil {
		if err := m.libraryManager.Shutdown(ctx); err != nil {
			log.Printf("ERROR: Failed to shutdown library manager: %v", err)
		}
	}

	m.initialized = false
	log.Println("INFO: Media module shutdown complete")
	return nil
}

// GetLibraryManager returns the library manager
func (m *Module) GetLibraryManager() *LibraryManager {
	return m.libraryManager
}

// GetFileProcessor returns the file processor
func (m *Module) GetFileProcessor() *FileProcessor {
	return m.fileProcessor
}

// GetMetadataManager returns the metadata manager
func (m *Module) GetMetadataManager() *MetadataManager {
	return m.metadataManager
}

// Upload handler functionality has been removed

// SetPluginModule sets the plugin module for media operations
func (m *Module) SetPluginModule(pluginModule *pluginmodule.PluginModule) {
	m.pluginModule = pluginModule

	// Re-initialize components if module is already initialized
	if m.initialized && pluginModule != nil {
		log.Printf("INFO: Updating media module components with plugin module")

		// Update file processor
		if m.fileProcessor != nil {
			m.fileProcessor = NewFileProcessor(m.db, m.eventBus, pluginModule)
			m.fileProcessor.Initialize()
		}

		// Update metadata manager
		if m.metadataManager != nil {
			m.metadataManager = NewMetadataManager(m.db, m.eventBus, pluginModule)
			m.metadataManager.Initialize()
		}

		log.Printf("✅ Media module components updated with plugin module")
	}
}
