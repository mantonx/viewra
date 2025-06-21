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
	"github.com/mantonx/viewra/internal/services"

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

	// Playback integration for intelligent streaming
	playbackIntegration *PlaybackIntegration
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

	// Initialize playback integration using service registry
	if playbackService, err := services.GetService[services.PlaybackService]("playback"); err == nil {
		m.playbackIntegration = NewPlaybackIntegration(m.db, playbackService)
		log.Println("✅ Playback integration initialized using service registry")
	} else {
		log.Printf("WARN: ⚠️ PlaybackService not available in service registry: %v", err)
		log.Println("ℹ️ Playback integration disabled - service not registered")
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

		// Modern DASH/HLS streaming - use new PlaybackModule workflow exclusively
		if m.playbackIntegration != nil {
			mediaGroup.POST("/files/:id/playback-decision", m.playbackIntegration.HandlePlaybackDecision)
			mediaGroup.GET("/files/:id/stream", m.playbackIntegration.HandleIntelligentStream)
			mediaGroup.HEAD("/files/:id/stream", m.playbackIntegration.HandleIntelligentStreamHead)
			log.Println("INFO: ✅ Registered DASH/HLS intelligent streaming routes")
		} else {
			// If no playback integration, redirect to use PlaybackModule directly
			mediaGroup.POST("/files/:id/playback-decision", m.redirectToPlaybackModule)
			mediaGroup.GET("/files/:id/stream", m.redirectToPlaybackModule)
			mediaGroup.HEAD("/files/:id/stream", m.redirectToPlaybackModule)
			log.Println("WARN: ⚠️ Playback integration unavailable - requests will redirect to PlaybackModule API")
		}

		// File metadata and management
		mediaGroup.GET("/files/:id/metadata", m.getFileMetadata)
		mediaGroup.GET("/files/:id/album-id", m.getFileAlbumId)
		mediaGroup.GET("/files/:id/album-artwork", m.getFileAlbumArtwork)

		// TV Shows endpoints
		mediaGroup.GET("/tv-shows", m.getTVShows)

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

	log.Println("INFO: 🎬 Media module configured for DASH/HLS-first streaming workflow")
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

		// PlaybackService integration is already set up during module initialization
		// No need to recreate it when plugins are updated
		log.Printf("✅ Plugin module updated - playback integration uses service registry")

		log.Printf("✅ Media module components updated with plugin module")
	}
}

