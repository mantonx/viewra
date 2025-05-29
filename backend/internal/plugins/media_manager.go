package plugins

import (
	"fmt"
	"strings"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/mediaassetmodule"
	"gorm.io/gorm"
)

// MediaManager handles saving MediaItems and MediaAssets
type MediaManager struct {
	db *gorm.DB
}

// NewMediaManager creates a new MediaManager instance
func NewMediaManager(db *gorm.DB) *MediaManager {
	return &MediaManager{
		db: db,
	}
}

// SaveMediaItem saves a MediaItem and its associated MediaAssets
func (mm *MediaManager) SaveMediaItem(item *MediaItem, assets []MediaAsset) error {
	if item == nil {
		return fmt.Errorf("media item is nil")
	}

	// Save metadata based on type
	switch item.Type {
	case "music":
		return mm.saveMusicItem(item, assets)
	case "video":
		return mm.saveVideoItem(item, assets)
	case "image":
		return mm.saveImageItem(item, assets)
	default:
		return fmt.Errorf("unsupported media type: %s", item.Type)
	}
}

// saveMusicItem saves music metadata using the new asset system
func (mm *MediaManager) saveMusicItem(item *MediaItem, assets []MediaAsset) error {
	// Validate input
	if item == nil || item.MediaFile == nil {
		return fmt.Errorf("invalid music item or media file")
	}

	// Extract MusicMetadata from the interface
	musicMeta, ok := item.Metadata.(*database.MusicMetadata)
	if !ok {
		return fmt.Errorf("expected MusicMetadata, got %T", item.Metadata)
	}

	// Set the MediaFileID
	musicMeta.MediaFileID = item.MediaFile.ID
	musicMeta.HasArtwork = len(assets) > 0 // Set based on whether we have assets

	// Save to database
	if err := mm.db.Create(musicMeta).Error; err != nil {
		// Check if this is a duplicate - if so, update instead
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			updateErr := mm.db.Where("media_file_id = ?", item.MediaFile.ID).Updates(musicMeta).Error
			if updateErr != nil {
				return fmt.Errorf("failed to update music metadata: %w", updateErr)
			}
		} else {
			return fmt.Errorf("failed to save music metadata: %w", err)
		}
	}

	// Save assets using the new asset system
	for _, asset := range assets {
		if err := mm.saveMediaAsset(asset); err != nil {
			fmt.Printf("WARNING: Failed to save %s asset: %v\n", asset.Type, err)
		}
	}

	return nil
}

// saveVideoItem saves video metadata and assets (placeholder)
func (mm *MediaManager) saveVideoItem(item *MediaItem, assets []MediaAsset) error {
	fmt.Printf("DEBUG: Video metadata handling not yet implemented\n")
	
	// Save assets even if metadata isn't implemented yet
	for _, asset := range assets {
		if err := mm.saveMediaAsset(asset); err != nil {
			fmt.Printf("WARNING: Failed to save %s asset: %v\n", asset.Type, err)
		}
	}
	
	return nil
}

// saveImageItem saves image metadata and assets (placeholder)
func (mm *MediaManager) saveImageItem(item *MediaItem, assets []MediaAsset) error {
	fmt.Printf("DEBUG: Image metadata handling not yet implemented\n")
	
	// Save assets even if metadata isn't implemented yet
	for _, asset := range assets {
		if err := mm.saveMediaAsset(asset); err != nil {
			fmt.Printf("WARNING: Failed to save %s asset: %v\n", asset.Type, err)
		}
	}
	
	return nil
}

// saveMediaAsset saves a MediaAsset using the new asset system
func (mm *MediaManager) saveMediaAsset(asset MediaAsset) error {
	switch asset.Type {
	case "artwork":
		return mm.saveArtworkAsset(asset)
	case "subtitle":
		return mm.saveSubtitleAsset(asset)
	case "thumbnail":
		return mm.saveThumbnailAsset(asset)
	default:
		return fmt.Errorf("unsupported asset type: %s", asset.Type)
	}
}

// saveArtworkAsset saves artwork using the new media asset module
func (mm *MediaManager) saveArtworkAsset(asset MediaAsset) error {
	if len(asset.Data) == 0 {
		return fmt.Errorf("artwork data is empty")
	}

	if asset.Extension == "" {
		return fmt.Errorf("artwork extension is missing")
	}

	// Determine MIME type from extension
	mimeType := mm.getMimeTypeFromExtension(asset.Extension)
	
	// Create asset request for the new system
	request := &mediaassetmodule.AssetRequest{
		MediaFileID: asset.MediaFileID,
		Type:        mediaassetmodule.AssetTypeMusic,
		Category:    mediaassetmodule.CategoryAlbum,
		Subtype:     mediaassetmodule.SubtypeArtwork,
		Data:        asset.Data,
		MimeType:    mimeType,
		Metadata:    asset.Metadata,
	}

	// Save using the new asset manager
	_, err := mediaassetmodule.SaveMediaAsset(request)
	if err != nil {
		return fmt.Errorf("failed to save artwork with new asset system: %w", err)
	}

	fmt.Printf("DEBUG: Saved artwork asset for file ID %d: %s (%d bytes)\n", 
		asset.MediaFileID, asset.Extension, asset.Size)
	return nil
}

// getMimeTypeFromExtension converts file extension to MIME type
func (mm *MediaManager) getMimeTypeFromExtension(ext string) string {
	ext = strings.ToLower(ext)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".tiff", ".tif":
		return "image/tiff"
	default:
		return "image/jpeg" // Default to JPEG
	}
}

// saveSubtitleAsset saves subtitle files (placeholder)
func (mm *MediaManager) saveSubtitleAsset(asset MediaAsset) error {
	fmt.Printf("DEBUG: Subtitle asset saving not yet implemented\n")
	return nil
}

// saveThumbnailAsset saves thumbnail files (placeholder)  
func (mm *MediaManager) saveThumbnailAsset(asset MediaAsset) error {
	fmt.Printf("DEBUG: Thumbnail asset saving not yet implemented\n")
	return nil
}

// GetAsset retrieves a MediaAsset by type and media file ID
func (mm *MediaManager) GetAsset(mediaFileID uint, assetType string) (*MediaAsset, error) {
	switch assetType {
	case "artwork":
		return mm.getArtworkAsset(mediaFileID)
	default:
		return nil, fmt.Errorf("unsupported asset type: %s", assetType)
	}
}

// getArtworkAsset retrieves artwork data using the new asset system
func (mm *MediaManager) getArtworkAsset(mediaFileID uint) (*MediaAsset, error) {
	// Get assets for this media file using the new system
	manager := mediaassetmodule.GetAssetManager()
	if manager == nil {
		return nil, fmt.Errorf("asset manager not available")
	}

	assets, err := manager.GetAssetsByMediaFile(mediaFileID, mediaassetmodule.AssetTypeMusic)
	if err != nil {
		return nil, fmt.Errorf("failed to get assets: %w", err)
	}

	if len(assets) == 0 {
		return nil, fmt.Errorf("no artwork found for media file ID %d", mediaFileID)
	}

	// Convert to the legacy MediaAsset format
	assetResponse := assets[0]
	
	// Get the file extension from MIME type
	ext := mm.getExtensionFromMimeType(assetResponse.MimeType)
	
	return &MediaAsset{
		Type:        string(assetResponse.Type),
		Path:        assetResponse.RelativePath,
		Extension:   ext,
		MediaFileID: assetResponse.MediaFileID,
		MimeType:    assetResponse.MimeType,
		Size:        assetResponse.Size,
	}, nil
}

// getExtensionFromMimeType converts MIME type to file extension
func (mm *MediaManager) getExtensionFromMimeType(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/bmp":
		return ".bmp"
	case "image/tiff":
		return ".tiff"
	default:
		return ".jpg" // Default to JPEG
	}
}

// DeleteAssets removes all assets for a media file using the new system
func (mm *MediaManager) DeleteAssets(mediaFileID uint) error {
	return mediaassetmodule.RemoveMediaAssetsByMediaFile(mediaFileID)
} 