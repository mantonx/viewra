package plugins

import (
	"bytes"
	"fmt"
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
	fmt.Printf("INFO: Converting plugin asset to new entity-based asset system\n")
	
	// For embedded music artwork, we'll associate it with the album entity
	// Generate a deterministic UUID for the album based on mediaFileID
	// In a real system, you'd lookup the actual album ID from metadata
	albumIDString := fmt.Sprintf("album-placeholder-%d", asset.MediaFileID)
	albumID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(albumIDString))
	
	fmt.Printf("INFO: Saving plugin artwork for media file ID %d as album cover (album UUID: %s)\n", asset.MediaFileID, albumID)
	
	// Detect proper MIME type from data if the provided one is generic
	mimeType := asset.MimeType
	if mimeType == "" || mimeType == "application/octet-stream" || mimeType == "binary/octet-stream" {
		mimeType = mm.detectImageMimeType(asset.Data)
		fmt.Printf("INFO: Detected MIME type as %s for media file ID %d\n", mimeType, asset.MediaFileID)
	}
	
	// Determine source based on metadata or context
	// If this is from embedded artwork in the file, use SourceEmbedded
	// Otherwise, check metadata for the actual plugin source
	var source assetmodule.AssetSource = assetmodule.SourceEmbedded
	if sourceHint, exists := asset.Metadata["source"]; exists {
		switch sourceHint {
		case "musicbrainz_cover_art_archive", "musicbrainz":
			source = assetmodule.SourceMusicBrainz
		case "audiodb", "theaudiodb":
			source = assetmodule.SourceAudioDB
		case "embedded", "file":
			source = assetmodule.SourceEmbedded
		default:
			source = assetmodule.SourcePlugin // Generic fallback
		}
		fmt.Printf("INFO: Determined asset source as %s from metadata for media file ID %d\n", source, asset.MediaFileID)
	} else {
		fmt.Printf("INFO: No source metadata found, using SourceEmbedded for media file ID %d\n", asset.MediaFileID)
	}
	
	// Create asset request for album cover using new system
	request := &assetmodule.AssetRequest{
		EntityType: assetmodule.EntityTypeAlbum,
		EntityID:   albumID,
		Type:       assetmodule.AssetTypeCover,
		Source:     source, // Use the determined source
		Data:       asset.Data,
		Format:     mimeType,
		Preferred:  true, // Mark plugin artwork as preferred by default
	}

	// Save using the new asset manager
	_, err := assetmodule.SaveMediaAsset(request)
	if err != nil {
		return fmt.Errorf("failed to save artwork with new asset system: %w", err)
	}

	fmt.Printf("INFO: Successfully saved plugin artwork for media file ID %d as album cover (source: %s)\n", asset.MediaFileID, source)
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

// GetAsset retrieves a MediaAsset by type and media file ID
func (mm *MediaManager) GetAsset(mediaFileID uint, assetType string) (*MediaAsset, error) {
	// TODO: This is a compatibility stub for the old plugin asset system
	// Plugin asset handling needs to be updated for the new entity-based asset system
	return nil, fmt.Errorf("plugin asset interface is deprecated - please update plugin to use new entity-based asset system")
}

// getArtworkAsset retrieves artwork data using the new asset system
func (mm *MediaManager) getArtworkAsset(mediaFileID uint) (*MediaAsset, error) {
	// TODO: This is a compatibility stub for the old plugin asset system
	// Plugin asset handling needs to be updated for the new entity-based asset system
	return nil, fmt.Errorf("plugin asset interface is deprecated - please update plugin to use new entity-based asset system")
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
	// TODO: This is a compatibility stub for the old plugin asset system
	// Plugin asset handling needs to be updated for the new entity-based asset system
	fmt.Printf("WARNING: Plugin asset interface is deprecated - asset deletion disabled\n")
	return nil
} 