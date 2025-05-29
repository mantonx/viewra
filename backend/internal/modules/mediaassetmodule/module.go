package mediaassetmodule

import (
	"context"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/modules/modulemanager"
	"gorm.io/gorm"
)

// Module represents the Media Asset Management module
type Module struct {
	id          string
	name        string
	version     string
	core        bool
	initialized bool
	db          *gorm.DB
	eventBus    events.EventBus

	// Media asset management components
	manager *Manager
}

// Auto-register the module when imported
func init() {
	log.Println("DEBUG: mediaassetmodule init() called - registering module")
	Register()
}

// Register registers this module with the module system
func Register() {
	log.Println("DEBUG: mediaassetmodule Register() called - creating module")
	// Create module without database connection - it will be initialized later
	assetModule := &Module{
		id:      "system.media.assets",
		name:    "Media Asset Manager",
		version: "1.0.0",
		core:    true,
	}
	log.Printf("DEBUG: mediaassetmodule calling modulemanager.Register with module: %s", assetModule.ID())
	modulemanager.Register(assetModule)
	log.Println("DEBUG: mediaassetmodule registration completed")
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

// Migrate performs any pending migrations
func (m *Module) Migrate(db *gorm.DB) error {
	log.Println("INFO: Migrating media asset module schema")

	// Auto-migrate media asset models
	err := db.AutoMigrate(
		&MediaAsset{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate media asset schema: %w", err)
	}

	return nil
}

// Init initializes the media asset module components
func (m *Module) Init() error {
	log.Println("INFO: Initializing media asset module")

	// Get the database connection and event bus from the global system
	m.db = database.GetDB()
	m.eventBus = events.GetGlobalEventBus()

	// Initialize asset management components
	if err := m.initializeComponents(); err != nil {
		return fmt.Errorf("failed to initialize asset components: %w", err)
	}

	m.initialized = true

	// Publish initialization event
	if m.eventBus != nil {
		initEvent := events.NewSystemEvent(
			"mediaasset.module.initialized",
			"Media Asset Module Initialized",
			"Media asset module has been successfully initialized",
		)
		initEvent.Data = map[string]interface{}{
			"module_id":      m.id,
			"module_name":    m.name,
			"module_version": m.version,
			"core_module":    m.core,
			"root_path":      m.manager.pathUtil.GetRootPath(),
		}
		m.eventBus.PublishAsync(initEvent)
	}

	log.Println("INFO: Media asset module initialized successfully")
	return nil
}

// initializeComponents initializes all asset management components
func (m *Module) initializeComponents() error {
	log.Println("INFO: Initializing asset manager")
	m.manager = NewManager(m.db, m.eventBus)
	if err := m.manager.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize asset manager: %w", err)
	}
	log.Println("INFO: Asset manager initialized successfully")

	return nil
}

// RegisterRoutes registers the media asset module API routes
func (m *Module) RegisterRoutes(router *gin.Engine) {
	log.Println("INFO: Registering media asset module routes")

	assetGroup := router.Group("/api/assets")
	{
		// Asset management endpoints
		assetGroup.POST("/", m.createAsset)
		assetGroup.GET("/:id", m.getAsset)
		assetGroup.GET("/:id/data", m.getAssetData)
		assetGroup.PUT("/:id", m.updateAsset)
		assetGroup.DELETE("/:id", m.deleteAsset)

		// Asset query endpoints
		assetGroup.GET("/", m.listAssets)
		assetGroup.GET("/media/:media_id", m.getAssetsByMediaFile)
		assetGroup.GET("/category/:category", m.getAssetsByCategory)
		assetGroup.GET("/hash/:hash", m.getAssetByHash)

		// Asset utility endpoints
		assetGroup.GET("/stats", m.getStats)
		assetGroup.POST("/validate", m.validateIntegrity)
		assetGroup.DELETE("/media/:media_id", m.deleteAssetsByMediaFile)

		// Module status endpoints
		assetGroup.GET("/health", m.getHealth)
		assetGroup.GET("/status", m.getStatus)
	}

	log.Println("INFO: Asset module routes registered successfully")
}

// Shutdown gracefully shuts down the media asset module
func (m *Module) Shutdown(ctx context.Context) error {
	log.Println("INFO: Shutting down media asset module")

	// Publish shutdown event
	if m.eventBus != nil {
		shutdownEvent := events.NewSystemEvent(
			"mediaasset.module.shutdown",
			"Media Asset Module Shutdown",
			"Media asset module is shutting down",
		)
		shutdownEvent.Data = map[string]interface{}{
			"module_id":   m.id,
			"module_name": m.name,
		}
		m.eventBus.PublishAsync(shutdownEvent)
	}

	m.initialized = false
	log.Println("INFO: Media asset module shutdown complete")
	return nil
}

// GetManager returns the asset manager instance
func (m *Module) GetManager() *Manager {
	return m.manager
}

// GetFileStore returns the file store instance
func (m *Module) GetFileStore() *FileStore {
	if m.manager != nil {
		return m.manager.fileStore
	}
	return nil
}

// GetPathUtil returns the path utility instance
func (m *Module) GetPathUtil() *PathUtil {
	if m.manager != nil {
		return m.manager.pathUtil
	}
	return nil
}

// Public API functions for easy access from other modules

// GetAssetManager returns the global asset manager instance
func GetAssetManager() *Manager {
	if module, exists := modulemanager.GetModule("system.media.assets"); exists {
		if assetModule, ok := module.(*Module); ok && assetModule.initialized {
			return assetModule.manager
		}
	}
	return nil
}

// SaveMediaAsset saves a media asset to disk and database
func SaveMediaAsset(request *AssetRequest) (*AssetResponse, error) {
	manager := GetAssetManager()
	if manager == nil {
		return nil, fmt.Errorf("asset manager not available")
	}

	asset, err := manager.SaveAsset(request)
	if err != nil {
		return nil, err
	}

	return manager.buildAssetResponse(asset), nil
}

// GetMediaAsset retrieves a media asset using the global manager
func GetMediaAsset(id uint) (*AssetResponse, error) {
	manager := GetAssetManager()
	if manager == nil {
		return nil, fmt.Errorf("asset manager not available")
	}
	return manager.GetAsset(id)
}

// ExistsMediaAsset checks if an asset exists for a media file
func ExistsMediaAsset(mediaFileID uint, assetType AssetType, category AssetCategory) (bool, *AssetResponse, error) {
	manager := GetAssetManager()
	if manager == nil {
		return false, nil, fmt.Errorf("asset manager not available")
	}

	return manager.ExistsAsset(mediaFileID, assetType, category)
}

// RemoveMediaAsset removes a media asset using the global manager
func RemoveMediaAsset(id uint) error {
	manager := GetAssetManager()
	if manager == nil {
		return fmt.Errorf("asset manager not available")
	}
	return manager.RemoveAsset(id)
}

// RemoveMediaAssetsByMediaFile removes all assets for a media file using the global manager
func RemoveMediaAssetsByMediaFile(mediaFileID uint) error {
	manager := GetAssetManager()
	if manager == nil {
		return fmt.Errorf("asset manager not available")
	}
	return manager.RemoveAssetsByMediaFile(mediaFileID)
} 