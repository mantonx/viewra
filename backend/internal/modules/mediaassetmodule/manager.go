package mediaassetmodule

import (
	"fmt"
	"log"
	"time"

	"github.com/mantonx/viewra/internal/events"
	"gorm.io/gorm"
)

// Manager provides the main logic for managing media assets
type Manager struct {
	db          *gorm.DB
	eventBus    events.EventBus
	fileStore   *FileStore
	pathUtil    *PathUtil
	hashCalc    *HashCalculator
	initialized bool
}

// NewManager creates a new asset manager instance
func NewManager(db *gorm.DB, eventBus events.EventBus) *Manager {
	pathUtil := GetDefaultPathUtil()
	fileStore := NewFileStore(pathUtil)
	hashCalc := NewHashCalculator()
	
	return &Manager{
		db:        db,
		eventBus:  eventBus,
		fileStore: fileStore,
		pathUtil:  pathUtil,
		hashCalc:  hashCalc,
	}
}

// Initialize sets up the asset manager
func (m *Manager) Initialize() error {
	if m.initialized {
		return nil
	}

	// Create root directory if it doesn't exist
	rootPath := m.pathUtil.GetRootPath()
	if err := m.pathUtil.EnsurePath(AssetTypeMusic, CategoryAlbum, "00"); err != nil {
		return fmt.Errorf("failed to create root asset directory: %w", err)
	}

	log.Printf("INFO: Asset manager initialized with root path: %s\n", rootPath)
	m.initialized = true
	return nil
}

// SaveAsset saves a media asset to disk and database
func (m *Manager) SaveAsset(request *AssetRequest) (*MediaAsset, error) {
	if request == nil {
		return nil, fmt.Errorf("asset request cannot be nil")
	}

	if len(request.Data) == 0 {
		return nil, fmt.Errorf("asset data cannot be empty")
	}

	if request.MediaFileID == 0 {
		return nil, fmt.Errorf("media file ID cannot be zero")
	}

	// Calculate hash of the data
	hash := m.hashCalc.CalculateDataHash(request.Data)
	
	// Check if we already have this asset
	var existingAsset MediaAsset
	err := m.db.Where("media_file_id = ? AND type = ? AND category = ? AND subtype = ? AND hash = ?",
		request.MediaFileID, request.Type, request.Category, request.Subtype, hash).First(&existingAsset).Error
	
	if err == nil {
		// Asset already exists, return it
		return &existingAsset, nil
	}
	
	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to check for existing asset: %w", err)
	}

	// Save asset to filesystem
	relativePath, err := m.fileStore.SaveAsset(request.Type, request.Category, hash, request.MimeType, request.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to save asset to filesystem: %w", err)
	}

	// Create database record
	asset := &MediaAsset{
		MediaFileID:  request.MediaFileID,
		Type:         request.Type,
		Category:     request.Category,
		Subtype:      request.Subtype,
		RelativePath: relativePath,
		Hash:         hash,
		MimeType:     request.MimeType,
		Size:         int64(len(request.Data)),
		Width:        request.Width,
		Height:       request.Height,
	}

	if err := m.db.Create(asset).Error; err != nil {
		// Clean up filesystem if database save fails
		m.fileStore.RemoveAsset(relativePath)
		return nil, fmt.Errorf("failed to save asset to database: %w", err)
	}

	// Publish asset saved event
	if m.eventBus != nil {
		event := events.NewSystemEvent(
			"mediaasset.saved",
			"Media Asset Saved",
			fmt.Sprintf("Media asset saved: %s/%s for media file %d", asset.Type, asset.Category, asset.MediaFileID),
		)
		event.Data = map[string]interface{}{
			"asset_id":     asset.ID,
			"media_file_id": asset.MediaFileID,
			"type":         string(asset.Type),
			"category":     string(asset.Category),
			"subtype":      string(asset.Subtype),
			"hash":         asset.Hash,
			"size":         asset.Size,
			"mime_type":    asset.MimeType,
			"width":        asset.Width,
			"height":       asset.Height,
			"relative_path": asset.RelativePath,
			"action":       "created",
		}
		m.eventBus.PublishAsync(event)
	}

	return asset, nil
}

// GetAsset retrieves an asset by ID
func (m *Manager) GetAsset(id uint) (*AssetResponse, error) {
	var asset MediaAsset
	if err := m.db.First(&asset, id).Error; err != nil {
		return nil, fmt.Errorf("asset not found: %w", err)
	}

	return m.buildAssetResponse(&asset), nil
}

// GetAssetByHash retrieves an asset by its hash
func (m *Manager) GetAssetByHash(hash string) (*AssetResponse, error) {
	var asset MediaAsset
	if err := m.db.Where("hash = ?", hash).First(&asset).Error; err != nil {
		return nil, fmt.Errorf("asset not found: %w", err)
	}

	return m.buildAssetResponse(&asset), nil
}

// GetAssetsByMediaFile retrieves all assets for a media file
func (m *Manager) GetAssetsByMediaFile(mediaFileID uint, assetType AssetType) ([]*AssetResponse, error) {
	var assets []MediaAsset
	query := m.db.Where("media_file_id = ?", mediaFileID)
	
	if assetType != "" {
		query = query.Where("type = ?", assetType)
	}
	
	if err := query.Find(&assets).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve assets: %w", err)
	}

	responses := make([]*AssetResponse, len(assets))
	for i, asset := range assets {
		responses[i] = m.buildAssetResponse(&asset)
	}

	return responses, nil
}

// GetAssetsByCategory retrieves assets by category
func (m *Manager) GetAssetsByCategory(category AssetCategory, filter *AssetFilter) ([]*AssetResponse, error) {
	query := m.db.Where("category = ?", category)
	
	if filter != nil {
		if filter.MediaFileID != 0 {
			query = query.Where("media_file_id = ?", filter.MediaFileID)
		}
		if filter.Type != "" {
			query = query.Where("type = ?", filter.Type)
		}
		if filter.Subtype != "" {
			query = query.Where("subtype = ?", filter.Subtype)
		}
		if filter.Limit > 0 {
			query = query.Limit(filter.Limit)
		}
		if filter.Offset > 0 {
			query = query.Offset(filter.Offset)
		}
	}

	var assets []MediaAsset
	if err := query.Find(&assets).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve assets: %w", err)
	}

	responses := make([]*AssetResponse, len(assets))
	for i, asset := range assets {
		responses[i] = m.buildAssetResponse(&asset)
	}

	return responses, nil
}

// GetAssetData retrieves the binary data for an asset
func (m *Manager) GetAssetData(id uint) ([]byte, error) {
	var asset MediaAsset
	if err := m.db.First(&asset, id).Error; err != nil {
		return nil, fmt.Errorf("asset not found: %w", err)
	}

	return m.fileStore.GetAssetData(asset.RelativePath)
}

// ExistsAsset checks if an asset exists for the given criteria
func (m *Manager) ExistsAsset(mediaFileID uint, assetType AssetType, category AssetCategory) (bool, *AssetResponse, error) {
	var asset MediaAsset
	err := m.db.Where("media_file_id = ? AND type = ? AND category = ?", mediaFileID, assetType, category).First(&asset).Error
	
	if err == gorm.ErrRecordNotFound {
		return false, nil, nil
	}
	
	if err != nil {
		return false, nil, fmt.Errorf("failed to check asset existence: %w", err)
	}

	return true, m.buildAssetResponse(&asset), nil
}

// UpdateAsset updates an existing asset
func (m *Manager) UpdateAsset(id uint, request *AssetRequest) (*AssetResponse, error) {
	var existingAsset MediaAsset
	if err := m.db.First(&existingAsset, id).Error; err != nil {
		return nil, fmt.Errorf("asset not found: %w", err)
	}

	// Calculate new hash
	newHash := m.hashCalc.CalculateDataHash(request.Data)

	// If hash is the same, no need to update filesystem
	if existingAsset.Hash == newHash {
		// Update only database fields that might have changed
		updates := map[string]interface{}{
			"width":     request.Width,
			"height":    request.Height,
			"updated_at": time.Now(),
		}
		
		if err := m.db.Model(&existingAsset).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("failed to update asset in database: %w", err)
		}
	} else {
		// Hash is different, need to save new file and update database
		newRelativePath, err := m.fileStore.SaveAsset(request.Type, request.Category, newHash, request.MimeType, request.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to save updated asset to filesystem: %w", err)
		}

		// Update database record
		updates := map[string]interface{}{
			"relative_path": newRelativePath,
			"hash":         newHash,
			"mime_type":    request.MimeType,
			"size":         int64(len(request.Data)),
			"width":        request.Width,
			"height":       request.Height,
			"updated_at":   time.Now(),
		}
		
		if err := m.db.Model(&existingAsset).Updates(updates).Error; err != nil {
			// Clean up new file if database update fails
			m.fileStore.RemoveAsset(newRelativePath)
			return nil, fmt.Errorf("failed to update asset in database: %w", err)
		}

		// Remove old file if path changed
		if existingAsset.RelativePath != newRelativePath {
			if err := m.fileStore.RemoveAsset(existingAsset.RelativePath); err != nil {
				log.Printf("WARNING: Failed to remove old asset file: %v\n", err)
			}
		}

		// Update the existing asset struct with new values
		existingAsset.RelativePath = newRelativePath
		existingAsset.Hash = newHash
		existingAsset.MimeType = request.MimeType
		existingAsset.Size = int64(len(request.Data))
	}

	existingAsset.Width = request.Width
	existingAsset.Height = request.Height

	// Publish asset updated event
	if m.eventBus != nil {
		event := events.NewSystemEvent(
			"mediaasset.updated",
			"Media Asset Updated",
			fmt.Sprintf("Media asset updated: %s/%s for media file %d", existingAsset.Type, existingAsset.Category, existingAsset.MediaFileID),
		)
		event.Data = map[string]interface{}{
			"asset_id":     existingAsset.ID,
			"media_file_id": existingAsset.MediaFileID,
			"type":         string(existingAsset.Type),
			"category":     string(existingAsset.Category),
			"subtype":      string(existingAsset.Subtype),
			"hash":         existingAsset.Hash,
			"size":         existingAsset.Size,
			"mime_type":    existingAsset.MimeType,
			"width":        existingAsset.Width,
			"height":       existingAsset.Height,
			"relative_path": existingAsset.RelativePath,
			"action":       "updated",
		}
		m.eventBus.PublishAsync(event)
	}

	return m.buildAssetResponse(&existingAsset), nil
}

// RemoveAsset removes an asset by ID
func (m *Manager) RemoveAsset(id uint) error {
	var asset MediaAsset
	if err := m.db.First(&asset, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil // Asset doesn't exist, consider it already removed
		}
		return fmt.Errorf("failed to find asset: %w", err)
	}

	// Remove from filesystem first
	if err := m.fileStore.RemoveAsset(asset.RelativePath); err != nil {
		log.Printf("WARNING: Failed to remove asset file: %v\n", err)
	}

	// Remove from database
	if err := m.db.Delete(&asset).Error; err != nil {
		return fmt.Errorf("failed to remove asset from database: %w", err)
	}

	// Publish asset removed event
	if m.eventBus != nil {
		event := events.NewSystemEvent(
			"mediaasset.removed",
			"Media Asset Removed",
			fmt.Sprintf("Media asset removed: %s/%s for media file %d", asset.Type, asset.Category, asset.MediaFileID),
		)
		event.Data = map[string]interface{}{
			"asset_id":     asset.ID,
			"media_file_id": asset.MediaFileID,
			"type":         string(asset.Type),
			"category":     string(asset.Category),
			"subtype":      string(asset.Subtype),
			"hash":         asset.Hash,
			"size":         asset.Size,
			"mime_type":    asset.MimeType,
			"relative_path": asset.RelativePath,
			"action":       "removed",
		}
		m.eventBus.PublishAsync(event)
	}

	return nil
}

// RemoveAssetsByMediaFile removes all assets for a media file
func (m *Manager) RemoveAssetsByMediaFile(mediaFileID uint) error {
	var assets []MediaAsset
	if err := m.db.Where("media_file_id = ?", mediaFileID).Find(&assets).Error; err != nil {
		return fmt.Errorf("failed to find assets: %w", err)
	}

	for _, asset := range assets {
		if err := m.RemoveAsset(asset.ID); err != nil {
			log.Printf("WARNING: Failed to remove asset ID %d: %v\n", asset.ID, err)
		}
	}

	return nil
}

// GetStats returns statistics about stored assets
func (m *Manager) GetStats() (*AssetStats, error) {
	stats := &AssetStats{
		AssetsByType:    make(map[AssetType]int64),
		AssetsByCategory: make(map[AssetCategory]int64),
		AssetsBySubtype: make(map[AssetSubtype]int64),
	}

	// Get total count and size
	var totalCount int64
	var totalSize int64
	
	if err := m.db.Model(&MediaAsset{}).Count(&totalCount).Error; err != nil {
		return nil, fmt.Errorf("failed to get total asset count: %w", err)
	}
	
	if err := m.db.Model(&MediaAsset{}).Select("COALESCE(SUM(size), 0)").Scan(&totalSize).Error; err != nil {
		return nil, fmt.Errorf("failed to get total asset size: %w", err)
	}

	stats.TotalAssets = totalCount
	stats.TotalSize = totalSize

	if totalCount > 0 {
		stats.AverageSize = float64(totalSize) / float64(totalCount)
	}

	// Get largest asset size
	var largestSize int64
	if err := m.db.Model(&MediaAsset{}).Select("COALESCE(MAX(size), 0)").Scan(&largestSize).Error; err != nil {
		return nil, fmt.Errorf("failed to get largest asset size: %w", err)
	}
	stats.LargestAsset = largestSize

	// Get counts by type
	type CountResult struct {
		Type  AssetType
		Count int64
	}
	var typeResults []CountResult
	if err := m.db.Model(&MediaAsset{}).Select("type, COUNT(*) as count").Group("type").Scan(&typeResults).Error; err != nil {
		return nil, fmt.Errorf("failed to get assets by type: %w", err)
	}
	for _, result := range typeResults {
		stats.AssetsByType[result.Type] = result.Count
	}

	// Get counts by category
	type CategoryResult struct {
		Category AssetCategory
		Count    int64
	}
	var categoryResults []CategoryResult
	if err := m.db.Model(&MediaAsset{}).Select("category, COUNT(*) as count").Group("category").Scan(&categoryResults).Error; err != nil {
		return nil, fmt.Errorf("failed to get assets by category: %w", err)
	}
	for _, result := range categoryResults {
		stats.AssetsByCategory[result.Category] = result.Count
	}

	// Get counts by subtype
	type SubtypeResult struct {
		Subtype AssetSubtype
		Count   int64
	}
	var subtypeResults []SubtypeResult
	if err := m.db.Model(&MediaAsset{}).Select("subtype, COUNT(*) as count").Group("subtype").Scan(&subtypeResults).Error; err != nil {
		return nil, fmt.Errorf("failed to get assets by subtype: %w", err)
	}
	for _, result := range subtypeResults {
		stats.AssetsBySubtype[result.Subtype] = result.Count
	}

	return stats, nil
}

// ValidateAssetIntegrity validates that all database records have corresponding files
func (m *Manager) ValidateAssetIntegrity() error {
	var assets []MediaAsset
	if err := m.db.Find(&assets).Error; err != nil {
		return fmt.Errorf("failed to retrieve assets: %w", err)
	}

	var failedAssets []uint
	var validatedCount int

	for _, asset := range assets {
		if err := m.fileStore.ValidateAssetIntegrity(asset.RelativePath, asset.Hash, asset.Size); err != nil {
			log.Printf("WARNING: Asset integrity validation failed for asset %d: %v\n", asset.ID, err)
			failedAssets = append(failedAssets, asset.ID)
		} else {
			validatedCount++
		}
	}

	// Publish integrity validation event
	if m.eventBus != nil {
		event := events.NewSystemEvent(
			"mediaasset.integrity.validated",
			"Media Asset Integrity Validated",
			fmt.Sprintf("Validated %d assets, %d failed", validatedCount, len(failedAssets)),
		)
		event.Data = map[string]interface{}{
			"total_assets":    len(assets),
			"validated_count": validatedCount,
			"failed_count":    len(failedAssets),
			"failed_assets":   failedAssets,
		}
		m.eventBus.PublishAsync(event)
	}

	if len(failedAssets) > 0 {
		return fmt.Errorf("integrity validation failed for %d assets", len(failedAssets))
	}

	return nil
}

// validateAssetRequest validates an asset request
func (m *Manager) validateAssetRequest(request *AssetRequest) error {
	if request.MediaFileID == 0 {
		return fmt.Errorf("media file ID cannot be zero")
	}
	
	if request.Type == "" {
		return fmt.Errorf("asset type cannot be empty")
	}
	
	if request.Category == "" {
		return fmt.Errorf("asset category cannot be empty")
	}
	
	if request.Subtype == "" {
		return fmt.Errorf("asset subtype cannot be empty")
	}
	
	if len(request.Data) == 0 {
		return fmt.Errorf("asset data cannot be empty")
	}
	
	if request.MimeType == "" {
		return fmt.Errorf("MIME type cannot be empty")
	}
	
	return nil
}

// buildAssetResponse builds an asset response from a MediaAsset
func (m *Manager) buildAssetResponse(asset *MediaAsset) *AssetResponse {
	return &AssetResponse{
		ID:           asset.ID,
		MediaFileID:  asset.MediaFileID,
		Type:         asset.Type,
		Category:     asset.Category,
		Subtype:      asset.Subtype,
		RelativePath: asset.RelativePath,
		Hash:         asset.Hash,
		MimeType:     asset.MimeType,
		Size:         asset.Size,
		Width:        asset.Width,
		Height:       asset.Height,
		CreatedAt:    asset.CreatedAt,
		UpdatedAt:    asset.UpdatedAt,
	}
} 