package assetmodule

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/chai2010/webp"
	"github.com/google/uuid"
	"github.com/mantonx/viewra/internal/events"
	"gorm.io/gorm"
)

// Manager handles all media asset operations
type Manager struct {
	db          *gorm.DB
	eventBus    events.EventBus
	dataDir     string
	assetsPath  string
	initialized bool
}

// NewManager creates a new asset manager
func NewManager(db *gorm.DB, eventBus events.EventBus) *Manager {
	return &Manager{
		db:       db,
		eventBus: eventBus,
	}
}

// Initialize sets up the asset manager
func (m *Manager) Initialize() error {
	// Get data directory from environment or use default
	m.dataDir = os.Getenv("VIEWRA_DATA_DIR")
	if m.dataDir == "" {
		m.dataDir = "./viewra-data"
	}

	m.assetsPath = filepath.Join(m.dataDir, "assets")

	// Ensure assets directory exists
	if err := m.ensureDirectoryStructure(); err != nil {
		return fmt.Errorf("failed to create directory structure: %w", err)
	}

	m.initialized = true
	log.Printf("Asset manager initialized with data dir: %s", m.dataDir)
	return nil
}

// ensureDirectoryStructure creates the required asset directory structure
func (m *Manager) ensureDirectoryStructure() error {
	entityDirs := []string{
		"artists", "albums", "tracks", "movies", "tv_shows", "episodes",
		"directors", "actors", "studios", "labels", "networks", "genres",
		"collections", "misc",
	}

	for _, dir := range entityDirs {
		dirPath := filepath.Join(m.assetsPath, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
		}
	}

	return nil
}

// SaveAsset saves a media asset to filesystem and database
func (m *Manager) SaveAsset(request *AssetRequest) (*AssetResponse, error) {
	if !m.initialized {
		return nil, fmt.Errorf("asset manager not initialized")
	}

	if err := m.validateRequest(request); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Convert image to WebP format with high quality (95)
	webpData, width, height, err := m.convertToWebP(request.Data, request.Format, 95)
	if err != nil {
		return nil, fmt.Errorf("failed to convert image to WebP: %w", err)
	}

	// Update request with WebP data and format
	request.Data = webpData
	request.Format = "image/webp"
	request.Width = width
	request.Height = height

	// Generate asset path using hash-based organization
	relativePath, err := m.generateHashedAssetPath(request)
	if err != nil {
		return nil, fmt.Errorf("failed to generate asset path: %w", err)
	}

	// Check if asset already exists
	var existing MediaAsset
	err = m.db.Where("entity_type = ? AND entity_id = ? AND type = ? AND source = ?",
		request.EntityType, request.EntityID, request.Type, request.Source).First(&existing).Error

	if err == nil {
		// Asset exists, update it
		return m.updateExistingAsset(&existing, request, relativePath)
	}

	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to check existing asset: %w", err)
	}

	// Save asset to filesystem
	fullPath := filepath.Join(m.assetsPath, relativePath)
	if err := m.saveAssetFile(fullPath, request.Data); err != nil {
		return nil, fmt.Errorf("failed to save asset file: %w", err)
	}

	// Create new asset record
	asset := &MediaAsset{
		ID: uuid.New(),

		// Legacy fields for database compatibility
		MediaID:   request.EntityID.String(), // Use entity ID as media ID for compatibility
		MediaType: m.mapEntityTypeToLegacyMediaType(request.EntityType),
		AssetType: string(request.Type), // Convert AssetType to string

		// New entity-based fields
		EntityType: request.EntityType,
		EntityID:   request.EntityID,
		Type:       request.Type,
		Source:     request.Source,
		PluginID:   request.PluginID,
		Path:       relativePath,
		Width:      request.Width,
		Height:     request.Height,
		Format:     request.Format,
		Preferred:  request.Preferred,
		Language:   request.Language,

		// Calculate file size for legacy compatibility
		SizeBytes:  int64(len(request.Data)),
		IsDefault:  request.Preferred,
		Resolution: m.formatResolution(request.Width, request.Height),

		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := m.db.Create(asset).Error; err != nil {
		// Clean up file if database save fails
		os.Remove(fullPath)
		return nil, fmt.Errorf("failed to save asset to database: %w", err)
	}

	// Publish event
	m.publishAssetEvent(events.EventAssetCreated, asset)

	return m.buildAssetResponse(asset), nil
}

// convertToWebP converts an image to WebP format with specified quality
func (m *Manager) convertToWebP(data []byte, originalFormat string, quality int) ([]byte, int, int, error) {
	// If already WebP, just decode to get dimensions and re-encode with specified quality
	if originalFormat == "image/webp" {
		img, err := webp.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, 0, 0, fmt.Errorf("failed to decode WebP image: %w", err)
		}

		bounds := img.Bounds()
		width := bounds.Dx()
		height := bounds.Dy()

		// Re-encode with specified quality
		var buf bytes.Buffer
		if err := webp.Encode(&buf, img, &webp.Options{Quality: float32(quality)}); err != nil {
			return nil, 0, 0, fmt.Errorf("failed to encode WebP image: %w", err)
		}

		return buf.Bytes(), width, height, nil
	}

	// Decode original image
	var img image.Image
	var err error

	reader := bytes.NewReader(data)
	switch originalFormat {
	case "image/jpeg", "image/jpg":
		img, err = jpeg.Decode(reader)
	case "image/png":
		img, err = png.Decode(reader)
	case "image/gif":
		img, err = gif.Decode(reader)
	default:
		// Try to decode as generic image
		img, _, err = image.Decode(reader)
	}

	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to decode image: %w", err)
	}

	// Get image dimensions
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Encode as WebP
	var buf bytes.Buffer
	options := &webp.Options{Quality: float32(quality)}
	if err := webp.Encode(&buf, img, options); err != nil {
		return nil, 0, 0, fmt.Errorf("failed to encode as WebP: %w", err)
	}

	return buf.Bytes(), width, height, nil
}

// GetAssetDataWithQuality retrieves the binary data for an asset with optional quality adjustment
func (m *Manager) GetAssetDataWithQuality(id uuid.UUID, quality int) ([]byte, string, error) {
	var asset MediaAsset
	if err := m.db.First(&asset, "id = ?", id).Error; err != nil {
		return nil, "", fmt.Errorf("asset not found: %w", err)
	}

	fullPath := filepath.Join(m.assetsPath, asset.Path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read asset file: %w", err)
	}

	// If quality is specified and different from original, re-encode
	if quality > 0 && quality < 100 && asset.Format == "image/webp" {
		// Decode and re-encode with new quality
		img, err := webp.Decode(bytes.NewReader(data))
		if err != nil {
			log.Printf("WARNING: Failed to decode WebP for quality adjustment: %v", err)
			// Return original data if re-encoding fails
			return data, asset.Format, nil
		}

		var buf bytes.Buffer
		options := &webp.Options{Quality: float32(quality)}
		if err := webp.Encode(&buf, img, options); err != nil {
			log.Printf("WARNING: Failed to re-encode WebP with quality %d: %v", quality, err)
			// Return original data if re-encoding fails
			return data, asset.Format, nil
		}

		return buf.Bytes(), asset.Format, nil
	}

	return data, asset.Format, nil
}

// generateHashedAssetPath creates a hash-based relative path for an asset to prevent conflicts
func (m *Manager) generateHashedAssetPath(request *AssetRequest) (string, error) {
	// Create a hash from entity information for the subdirectory
	entityHash := m.generateEntityHash(request.EntityType, request.EntityID)

	// Create a content hash for the filename
	contentHash := m.generateContentHash(request.Data)

	// All images are now WebP, so use .webp extension
	fileExt := ".webp"

	// Create path structure: {entity_type}/{entity_hash_prefix}/{content_hash}.webp
	// Use first 2 chars of entity hash for directory sharding
	entityHashPrefix := entityHash[:2]

	// Include asset type and source in filename to ensure uniqueness for same content
	filename := fmt.Sprintf("%s_%s_%s%s",
		request.Type,
		request.Source,
		contentHash[:16], // Use first 16 chars of content hash
		fileExt)

	return filepath.Join(string(request.EntityType), entityHashPrefix, filename), nil
}

// generateEntityHash creates a hash from entity type and ID
func (m *Manager) generateEntityHash(entityType EntityType, entityID uuid.UUID) string {
	data := fmt.Sprintf("%s:%s", entityType, entityID.String())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// generateContentHash creates a hash from content data
func (m *Manager) generateContentHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// generateAssetPath creates the relative path for an asset (legacy method, keeping for compatibility)
func (m *Manager) generateAssetPath(request *AssetRequest) (string, error) {
	return m.generateHashedAssetPath(request)
}

// saveAssetFile saves binary data to the filesystem
func (m *Manager) saveAssetFile(fullPath string, data []byte) error {
	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write file
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", fullPath, err)
	}

	return nil
}

// updateExistingAsset updates an existing asset
func (m *Manager) updateExistingAsset(existing *MediaAsset, request *AssetRequest, newPath string) (*AssetResponse, error) {
	oldPath := filepath.Join(m.assetsPath, existing.Path)
	newFullPath := filepath.Join(m.assetsPath, newPath)

	// Save new file
	if err := m.saveAssetFile(newFullPath, request.Data); err != nil {
		return nil, fmt.Errorf("failed to save updated asset file: %w", err)
	}

	// Update database record
	updates := map[string]interface{}{
		"path":      newPath,
		"width":     request.Width,
		"height":    request.Height,
		"format":    request.Format,
		"preferred": request.Preferred,
		"language":  request.Language,
		"plugin_id": request.PluginID,
		// Update legacy fields for compatibility
		"size_bytes": int64(len(request.Data)),
		"is_default": request.Preferred,
		"resolution": m.formatResolution(request.Width, request.Height),
		"updated_at": time.Now(),
	}

	if err := m.db.Model(existing).Updates(updates).Error; err != nil {
		// Clean up new file if database update fails
		os.Remove(newFullPath)
		return nil, fmt.Errorf("failed to update asset in database: %w", err)
	}

	// Remove old file if path changed
	if oldPath != newFullPath {
		os.Remove(oldPath)
	}

	// Update the existing asset struct
	existing.Path = newPath
	existing.Width = request.Width
	existing.Height = request.Height
	existing.Format = request.Format
	existing.Preferred = request.Preferred
	existing.Language = request.Language
	existing.PluginID = request.PluginID
	existing.UpdatedAt = time.Now()

	m.publishAssetEvent(events.EventAssetUpdated, existing)

	return m.buildAssetResponse(existing), nil
}

// GetAsset retrieves an asset by ID
func (m *Manager) GetAsset(id uuid.UUID) (*AssetResponse, error) {
	var asset MediaAsset
	if err := m.db.First(&asset, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("asset not found: %w", err)
	}

	return m.buildAssetResponse(&asset), nil
}

// GetAssetsByEntity retrieves all assets for an entity
func (m *Manager) GetAssetsByEntity(entityType EntityType, entityID uuid.UUID, filter *AssetFilter) ([]*AssetResponse, error) {
	query := m.db.Where("entity_type = ? AND entity_id = ?", entityType, entityID)

	if filter != nil {
		if filter.Type != "" {
			query = query.Where("type = ?", filter.Type)
		}
		if filter.Source != "" {
			query = query.Where("source = ?", filter.Source)
		}
		if filter.Preferred != nil {
			query = query.Where("preferred = ?", *filter.Preferred)
		}
		if filter.Language != "" {
			query = query.Where("language = ?", filter.Language)
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
func (m *Manager) GetAssetData(id uuid.UUID) ([]byte, string, error) {
	return m.GetAssetDataWithQuality(id, 0) // 0 means original quality
}

// GetPreferredAsset gets the preferred asset of a type for an entity
func (m *Manager) GetPreferredAsset(entityType EntityType, entityID uuid.UUID, assetType AssetType) (*AssetResponse, error) {
	var asset MediaAsset
	err := m.db.Where("entity_type = ? AND entity_id = ? AND type = ? AND preferred = ?",
		entityType, entityID, assetType, true).First(&asset).Error

	if err == gorm.ErrRecordNotFound {
		// No preferred asset, get any asset of this type
		err = m.db.Where("entity_type = ? AND entity_id = ? AND type = ?",
			entityType, entityID, assetType).First(&asset).Error
	}

	if err != nil {
		return nil, fmt.Errorf("no asset found: %w", err)
	}

	return m.buildAssetResponse(&asset), nil
}

// SetPreferredAsset sets an asset as preferred
func (m *Manager) SetPreferredAsset(id uuid.UUID) error {
	var asset MediaAsset
	if err := m.db.First(&asset, "id = ?", id).Error; err != nil {
		return fmt.Errorf("asset not found: %w", err)
	}

	// Unset all other preferred assets of the same type for this entity
	err := m.db.Model(&MediaAsset{}).
		Where("entity_type = ? AND entity_id = ? AND type = ? AND id != ?",
			asset.EntityType, asset.EntityID, asset.Type, id).
		Update("preferred", false).Error
	if err != nil {
		return fmt.Errorf("failed to unset other preferred assets: %w", err)
	}

	// Set this asset as preferred
	if err := m.db.Model(&asset).Update("preferred", true).Error; err != nil {
		return fmt.Errorf("failed to set asset as preferred: %w", err)
	}

	m.publishAssetEvent(events.EventAssetPreferred, &asset)

	return nil
}

// RemoveAsset removes an asset
func (m *Manager) RemoveAsset(id uuid.UUID) error {
	var asset MediaAsset
	if err := m.db.First(&asset, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil // Already removed
		}
		return fmt.Errorf("failed to find asset: %w", err)
	}

	// Remove file
	fullPath := filepath.Join(m.assetsPath, asset.Path)
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		log.Printf("WARNING: Failed to remove asset file %s: %v", fullPath, err)
	}

	// Remove from database
	if err := m.db.Delete(&asset).Error; err != nil {
		return fmt.Errorf("failed to remove asset from database: %w", err)
	}

	m.publishAssetEvent(events.EventAssetRemoved, &asset)

	return nil
}

// RemoveAssetsByEntity removes all assets for an entity
func (m *Manager) RemoveAssetsByEntity(entityType EntityType, entityID uuid.UUID) error {
	var assets []MediaAsset
	if err := m.db.Where("entity_type = ? AND entity_id = ?", entityType, entityID).Find(&assets).Error; err != nil {
		return fmt.Errorf("failed to find assets: %w", err)
	}

	for _, asset := range assets {
		if err := m.RemoveAsset(asset.ID); err != nil {
			log.Printf("WARNING: Failed to remove asset %s: %v", asset.ID, err)
		}
	}

	return nil
}

// GetStats returns asset statistics
func (m *Manager) GetStats() (*AssetStats, error) {
	stats := &AssetStats{
		AssetsByEntity: make(map[EntityType]int64),
		AssetsByType:   make(map[AssetType]int64),
		AssetsBySource: make(map[AssetSource]int64),
	}

	// Total assets
	if err := m.db.Model(&MediaAsset{}).Count(&stats.TotalAssets).Error; err != nil {
		return nil, err
	}

	// Calculate total size by reading all asset files
	var assets []MediaAsset
	if err := m.db.Find(&assets).Error; err != nil {
		return nil, err
	}

	var totalSize int64
	var validAssets int64
	for _, asset := range assets {
		fullPath := filepath.Join(m.assetsPath, asset.Path)
		if info, err := os.Stat(fullPath); err == nil {
			totalSize += info.Size()
			validAssets++
		}
	}

	stats.TotalSize = totalSize
	if validAssets > 0 {
		stats.AverageSize = float64(totalSize) / float64(validAssets)
	}

	// Preferred assets count
	m.db.Model(&MediaAsset{}).Where("preferred = ?", true).Count(&stats.PreferredAssets)

	// Group by entity type
	var entityResults []struct {
		EntityType EntityType
		Count      int64
	}
	m.db.Model(&MediaAsset{}).Select("entity_type, COUNT(*) as count").Group("entity_type").Scan(&entityResults)
	for _, result := range entityResults {
		stats.AssetsByEntity[result.EntityType] = result.Count
	}

	// Group by asset type
	var typeResults []struct {
		Type  AssetType
		Count int64
	}
	m.db.Model(&MediaAsset{}).Select("type, COUNT(*) as count").Group("type").Scan(&typeResults)
	for _, result := range typeResults {
		stats.AssetsByType[result.Type] = result.Count
	}

	// Group by source
	var sourceResults []struct {
		Source AssetSource
		Count  int64
	}
	m.db.Model(&MediaAsset{}).Select("source, COUNT(*) as count").Group("source").Scan(&sourceResults)
	for _, result := range sourceResults {
		stats.AssetsBySource[result.Source] = result.Count
	}

	// Supported formats - now primarily WebP
	var formats []string
	m.db.Model(&MediaAsset{}).Distinct("format").Pluck("format", &formats)
	stats.SupportedFormats = formats

	return stats, nil
}

// validateRequest validates an asset request
func (m *Manager) validateRequest(request *AssetRequest) error {
	if request.EntityType == "" {
		return fmt.Errorf("entity_type is required")
	}
	if request.EntityID == uuid.Nil {
		return fmt.Errorf("entity_id is required")
	}
	if request.Type == "" {
		return fmt.Errorf("type is required")
	}
	if request.Source == "" {
		return fmt.Errorf("source is required")
	}
	if len(request.Data) == 0 {
		return fmt.Errorf("data is required")
	}
	if request.Format == "" {
		return fmt.Errorf("format is required")
	}
	if !IsSupportedImageFormat(request.Format) {
		return fmt.Errorf("unsupported format: %s", request.Format)
	}

	// Validate entity type and asset type combination
	validTypes := GetValidAssetTypes(request.EntityType)
	isValid := false
	for _, validType := range validTypes {
		if validType == request.Type {
			isValid = true
			break
		}
	}
	if !isValid {
		return fmt.Errorf("asset type %s is not valid for entity type %s", request.Type, request.EntityType)
	}

	return nil
}

// buildAssetResponse creates an AssetResponse from a MediaAsset
func (m *Manager) buildAssetResponse(asset *MediaAsset) *AssetResponse {
	return &AssetResponse{
		ID:         asset.ID,
		EntityType: asset.EntityType,
		EntityID:   asset.EntityID,
		Type:       asset.Type,
		Source:     asset.Source,
		PluginID:   asset.PluginID,
		Path:       asset.Path,
		Width:      asset.Width,
		Height:     asset.Height,
		Format:     asset.Format,
		Preferred:  asset.Preferred,
		Language:   asset.Language,
		CreatedAt:  asset.CreatedAt,
		UpdatedAt:  asset.UpdatedAt,
	}
}

// publishAssetEvent publishes an asset-related event
func (m *Manager) publishAssetEvent(eventType events.EventType, asset *MediaAsset) {
	if m.eventBus == nil {
		return
	}

	event := events.NewSystemEvent(
		eventType,
		fmt.Sprintf("Asset %s", eventType),
		fmt.Sprintf("Asset %s: %s/%s for %s %s", eventType, asset.EntityType, asset.Type, asset.EntityType, asset.EntityID),
	)
	event.Data = map[string]interface{}{
		"asset_id":    asset.ID,
		"entity_type": string(asset.EntityType),
		"entity_id":   asset.EntityID.String(),
		"type":        string(asset.Type),
		"source":      string(asset.Source),
		"path":        asset.Path,
		"format":      asset.Format,
		"preferred":   asset.Preferred,
	}

	m.eventBus.PublishAsync(event)
}

// generateHash creates a SHA-256 hash of the given data
func (m *Manager) generateHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// CleanupOrphanedFiles removes files that don't have corresponding database records
func (m *Manager) CleanupOrphanedFiles() error {
	if !m.initialized {
		return fmt.Errorf("asset manager not initialized")
	}

	var removedCount int

	// Walk through all asset directories
	err := filepath.Walk(m.assetsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Get relative path
		relativePath, err := filepath.Rel(m.assetsPath, path)
		if err != nil {
			return err
		}

		// Check if file has corresponding database record
		var count int64
		m.db.Model(&MediaAsset{}).Where("path = ?", relativePath).Count(&count)

		if count == 0 {
			// Orphaned file, remove it
			if err := os.Remove(path); err != nil {
				log.Printf("WARNING: Failed to remove orphaned file %s: %v", path, err)
			} else {
				removedCount++
				log.Printf("Removed orphaned asset file: %s", relativePath)
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk asset directory: %w", err)
	}

	log.Printf("Cleanup completed. Removed %d orphaned files", removedCount)
	return nil
}

// mapEntityTypeToLegacyMediaType maps new entity types to legacy media types
func (m *Manager) mapEntityTypeToLegacyMediaType(entityType EntityType) string {
	switch entityType {
	case EntityTypeTrack:
		return "track"
	case EntityTypeAlbum:
		return "album"
	case EntityTypeArtist:
		return "artist"
	case EntityTypeMovie:
		return "movie"
	case EntityTypeEpisode:
		return "episode"
	case EntityTypeTVShow:
		return "tv_show"
	default:
		return string(entityType) // Fallback to entity type string
	}
}

// formatResolution creates a resolution string from width and height
func (m *Manager) formatResolution(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	return fmt.Sprintf("%dx%d", width, height)
}
