package assetmodule

import (
	"fmt"
	"log"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/modules/modulemanager"
	"gorm.io/gorm"
)

// Auto-register the module when imported
func init() {
	Register()
}

// Register registers this module with the module system
func Register() {
	// Create module without database connection - it will be initialized later
	assetModule := &Module{
		id:      "system.media.assets",
		name:    "Media Asset Manager",
		version: "2.0.0",
		core:    true,
	}
	modulemanager.Register(assetModule)
}

// Module handles media asset management
type Module struct {
	id          string
	name        string
	version     string
	core        bool
	db          *gorm.DB
	eventBus    events.EventBus
	manager     *Manager
	initialized bool
}

// NewModule creates a new media asset module
func NewModule() *Module {
	return &Module{
		id:      "system.media.assets",
		name:    "Media Asset Manager",
		version: "2.0.0",
		core:    true,
	}
}

// ID returns the module ID
func (m *Module) ID() string {
	return m.id
}

// Name returns the module name
func (m *Module) Name() string {
	return m.name
}

// Core returns whether this is a core module
func (m *Module) Core() bool {
	return m.core
}

// Migrate handles database schema migrations
func (m *Module) Migrate(db *gorm.DB) error {
	log.Println("Migrating media asset module schema")

	// Auto-migrate media asset models with new schema
	err := db.AutoMigrate(&MediaAsset{})
	if err != nil {
		return fmt.Errorf("failed to migrate media asset schema: %w", err)
	}

	// Create indexes for optimal performance
	if err := m.createIndexes(db); err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	log.Println("Media asset module schema migration completed")
	return nil
}

// createIndexes creates database indexes for optimal query performance
func (m *Module) createIndexes(db *gorm.DB) error {
	// Composite index on entity_type and entity_id for fast entity lookups
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_media_assets_entity ON media_assets(entity_type, entity_id)").Error; err != nil {
		return err
	}

	// Index on type for asset type queries
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_media_assets_type ON media_assets(type)").Error; err != nil {
		return err
	}

	// Index on source for source-based queries
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_media_assets_source ON media_assets(source)").Error; err != nil {
		return err
	}

	// Index on preferred for preferred asset queries
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_media_assets_preferred ON media_assets(preferred)").Error; err != nil {
		return err
	}

	// Composite index for fast preferred asset lookups
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_media_assets_entity_type_preferred ON media_assets(entity_type, entity_id, type, preferred)").Error; err != nil {
		return err
	}

	return nil
}

// Init initializes the media asset module
func (m *Module) Init() error {
	log.Println("Initializing media asset module")

	// Get dependencies
	m.db = database.GetDB()
	m.eventBus = events.GetGlobalEventBus()

	// Initialize manager
	if err := m.initializeManager(); err != nil {
		return fmt.Errorf("failed to initialize asset manager: %w", err)
	}

	m.initialized = true

	// Publish initialization event
	if m.eventBus != nil {
		initEvent := events.NewSystemEvent(
			"mediaasset.module.initialized",
			"Media Asset Module Initialized",
			"Comprehensive media asset module has been successfully initialized",
		)
		initEvent.Data = map[string]interface{}{
			"module_id":      m.id,
			"module_name":    m.name,
			"module_version": m.version,
			"core_module":    m.core,
		}
		m.eventBus.PublishAsync(initEvent)
	}

	log.Println("Media asset module initialized successfully")
	return nil
}

// initializeManager initializes the asset manager
func (m *Module) initializeManager() error {
	log.Println("Initializing comprehensive asset manager")
	m.manager = NewManager(m.db, m.eventBus)
	if err := m.manager.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize asset manager: %w", err)
	}
	log.Println("Asset manager initialized successfully")
	return nil
}

// RegisterRoutes registers API routes for media assets
func (m *Module) RegisterRoutes(router *gin.Engine) {
	if !m.initialized {
		log.Println("WARNING: Media asset module not initialized, skipping route registration")
		return
	}

	api := router.Group("/api/v1/assets")
	{
		// Asset management endpoints
		api.POST("/", m.createAsset)
		api.GET("/:id", m.getAsset)
		api.PUT("/:id/preferred", m.setPreferredAsset)
		api.DELETE("/:id", m.deleteAsset)

		// Entity-based endpoints
		api.GET("/entity/:type/:id", m.getAssetsByEntity)
		api.GET("/entity/:type/:id/preferred/:asset_type", m.getPreferredAsset)
		api.GET("/entity/:type/:id/preferred/:asset_type/data", m.getPreferredAssetData)
		api.DELETE("/entity/:type/:id", m.deleteAssetsByEntity)

		// Asset data endpoints
		api.GET("/:id/data", m.getAssetData)

		// Statistics and management
		api.GET("/stats", m.getAssetStats)
		api.POST("/cleanup", m.cleanupOrphanedFiles)

		// Utility endpoints
		api.GET("/types", m.getValidTypes)
		api.GET("/sources", m.getValidSources)
		api.GET("/entity-types", m.getEntityTypes)
	}

	log.Println("Media asset API routes registered")
}

// API Handlers

// createAsset creates a new media asset
func (m *Module) createAsset(c *gin.Context) {
	var request AssetRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request format", "details": err.Error()})
		return
	}

	response, err := m.manager.SaveAsset(&request)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to create asset", "details": err.Error()})
		return
	}

	c.JSON(201, gin.H{"asset": response, "success": true})
}

// getAsset retrieves an asset by ID
func (m *Module) getAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid asset ID format"})
		return
	}

	response, err := m.manager.GetAsset(id)
	if err != nil {
		c.JSON(404, gin.H{"error": "Asset not found", "details": err.Error()})
		return
	}

	c.JSON(200, gin.H{"asset": response, "success": true})
}

// getAssetsByEntity retrieves assets for an entity
func (m *Module) getAssetsByEntity(c *gin.Context) {
	entityType := EntityType(c.Param("type"))
	entityIDStr := c.Param("id")
	entityID, err := uuid.Parse(entityIDStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid entity ID format"})
		return
	}

	// Parse query parameters for filtering
	var filter AssetFilter
	if err := c.ShouldBindQuery(&filter); err == nil {
		// Use filter from query params
	}

	assets, err := m.manager.GetAssetsByEntity(entityType, entityID, &filter)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to retrieve assets", "details": err.Error()})
		return
	}

	c.JSON(200, gin.H{"assets": assets, "total": len(assets), "success": true})
}

// getPreferredAsset gets the preferred asset of a type for an entity
func (m *Module) getPreferredAsset(c *gin.Context) {
	entityType := EntityType(c.Param("type"))
	entityIDStr := c.Param("id")
	assetType := AssetType(c.Param("asset_type"))

	entityID, err := uuid.Parse(entityIDStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid entity ID format"})
		return
	}

	asset, err := m.manager.GetPreferredAsset(entityType, entityID, assetType)
	if err != nil {
		c.JSON(404, gin.H{"error": "Preferred asset not found", "details": err.Error()})
		return
	}

	c.JSON(200, gin.H{"asset": asset, "success": true})
}

// getPreferredAssetData serves binary data for the preferred asset of a type for an entity
func (m *Module) getPreferredAssetData(c *gin.Context) {
	entityType := EntityType(c.Param("type"))
	entityIDStr := c.Param("id")
	assetType := AssetType(c.Param("asset_type"))

	entityID, err := uuid.Parse(entityIDStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid entity ID format"})
		return
	}

	asset, err := m.manager.GetPreferredAsset(entityType, entityID, assetType)
	if err != nil {
		c.JSON(404, gin.H{"error": "Preferred asset not found", "details": err.Error()})
		return
	}

	// Get quality parameter (optional)
	qualityStr := c.Query("quality")
	quality := 0 // Default to original quality
	if qualityStr != "" {
		if q, err := strconv.Atoi(qualityStr); err == nil && q > 0 && q <= 100 {
			quality = q
		}
	}

	// Get the asset data using the asset ID
	data, format, err := m.manager.GetAssetDataWithQuality(asset.ID, quality)
	if err != nil {
		c.JSON(404, gin.H{"error": "Asset data not found", "details": err.Error()})
		return
	}

	// Set appropriate headers for image serving
	c.Header("Content-Type", format)
	c.Header("Cache-Control", "public, max-age=31536000") // 1 year cache
	
	// Add quality info to headers if quality was adjusted
	if quality > 0 {
		c.Header("X-Quality", qualityStr)
	}
	
	// Serve the image data
	c.Data(200, format, data)
}

// setPreferredAsset sets an asset as preferred
func (m *Module) setPreferredAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid asset ID format"})
		return
	}

	if err := m.manager.SetPreferredAsset(id); err != nil {
		c.JSON(500, gin.H{"error": "Failed to set preferred asset", "details": err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "Asset set as preferred"})
}

// deleteAsset deletes an asset
func (m *Module) deleteAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid asset ID format"})
		return
	}

	if err := m.manager.RemoveAsset(id); err != nil {
		c.JSON(500, gin.H{"error": "Failed to delete asset", "details": err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "Asset deleted successfully"})
}

// deleteAssetsByEntity deletes all assets for an entity
func (m *Module) deleteAssetsByEntity(c *gin.Context) {
	entityType := EntityType(c.Param("type"))
	entityIDStr := c.Param("id")
	entityID, err := uuid.Parse(entityIDStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid entity ID format"})
		return
	}

	if err := m.manager.RemoveAssetsByEntity(entityType, entityID); err != nil {
		c.JSON(500, gin.H{"error": "Failed to delete assets", "details": err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "Assets deleted successfully"})
}

// getAssetData serves asset binary data
func (m *Module) getAssetData(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid asset ID format"})
		return
	}

	// Get quality parameter (optional)
	qualityStr := c.Query("quality")
	quality := 0 // Default to original quality
	if qualityStr != "" {
		if q, err := strconv.Atoi(qualityStr); err == nil && q > 0 && q <= 100 {
			quality = q
		}
	}

	data, format, err := m.manager.GetAssetDataWithQuality(id, quality)
	if err != nil {
		c.JSON(404, gin.H{"error": "Asset data not found", "details": err.Error()})
		return
	}

	c.Header("Content-Type", format)
	c.Header("Cache-Control", "public, max-age=31536000") // 1 year cache
	
	// Add quality info to headers if quality was adjusted
	if quality > 0 {
		c.Header("X-Quality", qualityStr)
	}
	
	c.Data(200, format, data)
}

// getAssetStats returns asset statistics
func (m *Module) getAssetStats(c *gin.Context) {
	stats, err := m.manager.GetStats()
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to get asset statistics", "details": err.Error()})
		return
	}

	c.JSON(200, gin.H{"stats": stats, "success": true})
}

// cleanupOrphanedFiles removes orphaned asset files
func (m *Module) cleanupOrphanedFiles(c *gin.Context) {
	if err := m.manager.CleanupOrphanedFiles(); err != nil {
		c.JSON(500, gin.H{"error": "Failed to cleanup orphaned files", "details": err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "Orphaned files cleaned up successfully"})
}

// getValidTypes returns valid asset types for an entity type
func (m *Module) getValidTypes(c *gin.Context) {
	entityType := c.Query("entity_type")
	if entityType == "" {
		// Return all asset types
		c.JSON(200, gin.H{
			"asset_types": []AssetType{
				AssetTypeLogo, AssetTypePhoto, AssetTypeBackground, AssetTypeBanner,
				AssetTypeThumb, AssetTypeFanart, AssetTypeClearart, AssetTypeCover,
				AssetTypeDisc, AssetTypeBooklet, AssetTypeWaveform, AssetTypeSpectrogram,
				AssetTypePoster, AssetTypeNetworkLogo, AssetTypeScreenshot, AssetTypeHeadshot,
				AssetTypePortrait, AssetTypeSignature, AssetTypeHQPhoto, AssetTypeIcon,
			},
			"success": true,
		})
		return
	}

	validTypes := GetValidAssetTypes(EntityType(entityType))
	c.JSON(200, gin.H{"asset_types": validTypes, "entity_type": entityType, "success": true})
}

// getValidSources returns valid asset sources
func (m *Module) getValidSources(c *gin.Context) {
	sources := GetValidSources()
	c.JSON(200, gin.H{"sources": sources, "success": true})
}

// getEntityTypes returns valid entity types
func (m *Module) getEntityTypes(c *gin.Context) {
	entityTypes := GetValidEntityTypes()
	c.JSON(200, gin.H{"entity_types": entityTypes, "success": true})
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

// SaveMediaAsset saves a media asset using the global manager
func SaveMediaAsset(request *AssetRequest) (*AssetResponse, error) {
	manager := GetAssetManager()
	if manager == nil {
		return nil, fmt.Errorf("asset manager not available")
	}
	return manager.SaveAsset(request)
}

// GetMediaAsset retrieves a media asset using the global manager
func GetMediaAsset(id uuid.UUID) (*AssetResponse, error) {
	manager := GetAssetManager()
	if manager == nil {
		return nil, fmt.Errorf("asset manager not available")
	}
	return manager.GetAsset(id)
}

// GetMediaAssetsByEntity retrieves assets for an entity using the global manager
func GetMediaAssetsByEntity(entityType EntityType, entityID uuid.UUID, filter *AssetFilter) ([]*AssetResponse, error) {
	manager := GetAssetManager()
	if manager == nil {
		return nil, fmt.Errorf("asset manager not available")
	}
	return manager.GetAssetsByEntity(entityType, entityID, filter)
}

// GetPreferredMediaAsset gets the preferred asset using the global manager
func GetPreferredMediaAsset(entityType EntityType, entityID uuid.UUID, assetType AssetType) (*AssetResponse, error) {
	manager := GetAssetManager()
	if manager == nil {
		return nil, fmt.Errorf("asset manager not available")
	}
	return manager.GetPreferredAsset(entityType, entityID, assetType)
}

// RemoveMediaAsset removes a media asset using the global manager
func RemoveMediaAsset(id uuid.UUID) error {
	manager := GetAssetManager()
	if manager == nil {
		return fmt.Errorf("asset manager not available")
	}
	return manager.RemoveAsset(id)
}

// RemoveMediaAssetsByEntity removes all assets for an entity using the global manager
func RemoveMediaAssetsByEntity(entityType EntityType, entityID uuid.UUID) error {
	manager := GetAssetManager()
	if manager == nil {
		return fmt.Errorf("asset manager not available")
	}
	return manager.RemoveAssetsByEntity(entityType, entityID)
} 