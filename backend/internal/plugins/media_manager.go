package plugins

import (
	"bytes"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/assetmodule"
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

// saveMusicItem saves music metadata and assets
func (mm *MediaManager) saveMusicItem(item *MediaItem, assets []MediaAsset) error {
	fmt.Printf("INFO: Saving music item with %d assets\n", len(assets))

	// TODO: With the new schema, music metadata is stored in Artist/Album/Track tables
	// For now, we'll just save the assets and skip the old MusicMetadata approach
	fmt.Printf("INFO: Music metadata storage updated to use Artist/Album/Track entities\n")

	// Save assets using the new asset system
	for _, asset := range assets {
		if err := mm.saveMediaAsset(asset); err != nil {
			fmt.Printf("WARNING: Failed to save %s asset: %v\n", asset.Type, err)
		}
	}

	return nil
}

// saveVideoItem saves video metadata and assets using the new asset system
func (mm *MediaManager) saveVideoItem(item *MediaItem, assets []MediaAsset) error {
	log.Printf("DEBUG: Processing video item: %s", item.MediaFile.Path)
	
	// Create basic video metadata record using existing Movie structure if available
	// For now, just save the assets as video metadata handling follows the same pattern as music
	
	// Save assets
	for _, asset := range assets {
		if err := mm.saveMediaAsset(asset); err != nil {
			log.Printf("WARNING: Failed to save %s asset: %v", asset.Type, err)
		} else {
			log.Printf("DEBUG: Saved %s asset for video: %s", asset.Type, item.MediaFile.Path)
		}
	}
	
	return nil
}

// saveImageItem saves image metadata and assets using the new asset system  
func (mm *MediaManager) saveImageItem(item *MediaItem, assets []MediaAsset) error {
	log.Printf("DEBUG: Processing image item: %s", item.MediaFile.Path)
	
	// Create basic image metadata record
	// Images typically don't need complex metadata storage like music/video
	
	// Save assets (thumbnails, etc.)
	for _, asset := range assets {
		if err := mm.saveMediaAsset(asset); err != nil {
			log.Printf("WARNING: Failed to save %s asset: %v", asset.Type, err)
		} else {
			log.Printf("DEBUG: Saved %s asset for image: %s", asset.Type, item.MediaFile.Path)
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
	fmt.Printf("INFO: Converting plugin asset to new entity-based asset system\n")
	
	// Get the MediaFile and follow the relationship to find the real Album.ID
	var mediaFile database.MediaFile
	if err := mm.db.Where("id = ?", asset.MediaFileID).First(&mediaFile).Error; err != nil {
		return fmt.Errorf("failed to find media file %s: %w", asset.MediaFileID, err)
	}
	
	// Only handle track media types for now
	if mediaFile.MediaType != database.MediaTypeTrack {
		return fmt.Errorf("album artwork only supported for music tracks")
	}
	
	// Get the track to find the album
	var track database.Track
	if err := mm.db.Preload("Album").Where("id = ?", mediaFile.MediaID).First(&track).Error; err != nil {
		return fmt.Errorf("failed to find track for media file %s: %w", asset.MediaFileID, err)
	}
	
	// Parse Album.ID as UUID
	albumUUID, err := uuid.Parse(track.Album.ID)
	if err != nil {
		return fmt.Errorf("invalid album ID format for track %s: %w", track.ID, err)
	}
	
	fmt.Printf("INFO: Saving plugin artwork for media file ID %s as album cover (album ID: %s, album title: %s)\n", 
		asset.MediaFileID, track.Album.ID, track.Album.Title)
	
	// Detect proper MIME type from data if the provided one is generic
	mimeType := asset.MimeType
	if mimeType == "" || mimeType == "application/octet-stream" || mimeType == "binary/octet-stream" {
		mimeType = mm.detectImageMimeType(asset.Data)
		fmt.Printf("INFO: Detected MIME type as %s for media file ID %s\n", mimeType, asset.MediaFileID)
	}
	
	// Determine source based on metadata or context
	// If this is from embedded artwork in the file, use SourceEmbedded
	// Otherwise, check metadata for the actual plugin source
	var source assetmodule.AssetSource = assetmodule.SourceEmbedded
	if sourceHint, exists := asset.Metadata["source"]; exists {
		switch sourceHint {
		case "embedded", "file":
			source = assetmodule.SourceEmbedded
		case "local":
			source = assetmodule.SourceLocal
		case "user":
			source = assetmodule.SourceUser
		default:
			// For all external plugin sources, use SourcePlugin
			source = assetmodule.SourcePlugin
		}
		fmt.Printf("INFO: Determined asset source as %s from metadata for media file ID %s\n", source, asset.MediaFileID)
	} else {
		fmt.Printf("INFO: No source metadata found, using SourceEmbedded for media file ID %s\n", asset.MediaFileID)
	}
	
	// Create asset request for album cover using new system with real Album.ID
	request := &assetmodule.AssetRequest{
		EntityType: assetmodule.EntityTypeAlbum,
		EntityID:   albumUUID,
		Type:       assetmodule.AssetTypeCover,
		Source:     source, // Use the determined source
		PluginID:   asset.PluginID, // Use plugin ID from the asset
		Data:       asset.Data,
		Format:     mimeType,
		Preferred:  true, // Mark plugin artwork as preferred by default
	}

	// Save using the new asset manager
	_, err = assetmodule.SaveMediaAsset(request)
	if err != nil {
		return fmt.Errorf("failed to save artwork with new asset system: %w", err)
	}

	fmt.Printf("INFO: Successfully saved plugin artwork for media file ID %s as album cover (album: %s, source: %s)\n", 
		asset.MediaFileID, track.Album.Title, source)
	return nil
}

// detectImageMimeType detects MIME type from image data
func (mm *MediaManager) detectImageMimeType(data []byte) string {
	if len(data) < 16 {
		return "image/jpeg" // Default fallback
	}
	
	// Check for common image signatures
	switch {
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}): // JPEG
		return "image/jpeg"
	case bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47}): // PNG
		return "image/png"
	case bytes.HasPrefix(data, []byte{0x47, 0x49, 0x46}): // GIF
		return "image/gif"
	case bytes.HasPrefix(data, []byte{0x52, 0x49, 0x46, 0x46}) && bytes.Contains(data[8:12], []byte("WEBP")): // WebP
		return "image/webp"
	case bytes.HasPrefix(data, []byte{0x42, 0x4D}): // BMP
		return "image/bmp"
	default:
		return "image/jpeg" // Default fallback
	}
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

// GetAsset retrieves an asset for a specific media file (legacy compatibility)
func (mm *MediaManager) GetAsset(mediaFileID string, assetType string) (*MediaAsset, error) {
	// Legacy compatibility - convert to new asset system lookup
	log.Printf("DEBUG: Legacy asset retrieval for media file %s, type %s", mediaFileID, assetType)
	
	// In the new system, assets are handled through MediaAsset entities
	// For now, return a basic error indicating migration to new system is needed
	return nil, fmt.Errorf("legacy asset system deprecated - use entity-based asset system via gRPC")
}

// getArtworkAsset retrieves artwork for a specific media file (legacy compatibility)
func (mm *MediaManager) getArtworkAsset(mediaFileID string) (*MediaAsset, error) {
	log.Printf("DEBUG: Legacy artwork retrieval for media file %s", mediaFileID)
	return nil, fmt.Errorf("legacy artwork retrieval deprecated - use entity-based asset system")
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

// DeleteAssets removes all assets for a specific media file (legacy compatibility)
func (mm *MediaManager) DeleteAssets(mediaFileID string) error {
	log.Printf("DEBUG: Legacy asset deletion for media file %s", mediaFileID)
	
	// In the new system, asset cleanup is handled by the asset module
	// For now, this is a no-op that logs the attempt
	log.Printf("INFO: Asset deletion handled by new asset module system")
	return nil
} 