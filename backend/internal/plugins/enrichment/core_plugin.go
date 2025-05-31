package enrichment

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/google/uuid"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/modules/assetmodule"
	"github.com/mantonx/viewra/internal/plugins"
	"gorm.io/gorm"
)

// EnrichmentCorePlugin handles extraction of metadata and artwork from music files
type EnrichmentCorePlugin struct {
	enabled bool
	db      *gorm.DB
}

// NewEnrichmentCorePlugin creates a new enrichment core plugin
func NewEnrichmentCorePlugin() *EnrichmentCorePlugin {
	return &EnrichmentCorePlugin{
		enabled: true,
	}
}

// GetName returns the plugin name
func (p *EnrichmentCorePlugin) GetName() string {
	return "music_metadata_extractor_plugin"
}

// GetSupportedExtensions returns supported file extensions
func (p *EnrichmentCorePlugin) GetSupportedExtensions() []string {
	return []string{".mp3", ".flac", ".m4a", ".aac", ".ogg", ".wav"}
}

// GetPluginType returns the plugin type
func (p *EnrichmentCorePlugin) GetPluginType() string {
	return "enrichment"
}

// Match determines if this plugin can handle the given file
func (p *EnrichmentCorePlugin) Match(path string, info fs.FileInfo) bool {
	if !p.enabled {
		return false
	}
	
	ext := strings.ToLower(filepath.Ext(path))
	for _, supportedExt := range p.GetSupportedExtensions() {
		if ext == supportedExt {
			return true
		}
	}
	return false
}

// HandleFile processes a music file and extracts metadata and artwork
func (p *EnrichmentCorePlugin) HandleFile(path string, ctx plugins.MetadataContext) error {
	if !p.enabled {
		return fmt.Errorf("enrichment core plugin is disabled")
	}

	log.Printf("INFO: Enrichment core plugin processing file: %s", path)

	// Get database connection
	db, ok := ctx.DB.(*gorm.DB)
	if !ok {
		return fmt.Errorf("invalid database connection")
	}
	p.db = db

	// Get event bus
	var eventBus events.EventBus
	if ctx.EventBus != nil {
		if eb, ok := ctx.EventBus.(events.EventBus); ok {
			eventBus = eb
		}
	}

	// Extract metadata from file
	metadata, err := p.extractMetadata(path)
	if err != nil {
		log.Printf("WARNING: Failed to extract metadata from %s: %v", path, err)
		// Continue with empty metadata rather than failing completely
		metadata = &database.MusicMetadata{
			MediaFileID: ctx.MediaFile.ID,
			Title:       filepath.Base(path),
			Artist:      "Unknown Artist",
			Album:       "Unknown Album",
		}
	}

	// Save metadata to database
	if err := p.saveMetadata(metadata, ctx.MediaFile.ID); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	// Extract and save artwork if present
	if err := p.extractAndSaveArtwork(path, ctx.MediaFile.ID); err != nil {
		log.Printf("WARNING: Failed to extract artwork from %s: %v", path, err)
		// Not a fatal error - continue without artwork
	}

	// Publish metadata extracted event
	if eventBus != nil {
		event := events.NewSystemEvent(
			"enrichment.metadata.extracted",
			"Metadata Extracted",
			fmt.Sprintf("Metadata extracted from %s", filepath.Base(path)),
		)
		event.Data = map[string]interface{}{
			"mediaFileID": ctx.MediaFile.ID,
			"title":       metadata.Title,
			"artist":      metadata.Artist,
			"album":       metadata.Album,
		}
		eventBus.PublishAsync(event)
	}

	log.Printf("INFO: Successfully processed file with enrichment core plugin: %s", path)
	return nil
}

// extractMetadata extracts metadata from a music file using tag library
func (p *EnrichmentCorePlugin) extractMetadata(path string) (*database.MusicMetadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Parse tags using dhowden/tag library
	metadata, err := tag.ReadFrom(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read tags: %w", err)
	}

	// Extract basic metadata
	musicMeta := &database.MusicMetadata{
		Title:  metadata.Title(),
		Artist: metadata.Artist(),
		Album:  metadata.Album(),
		Genre:  metadata.Genre(),
	}

	// Handle missing title
	if musicMeta.Title == "" {
		// Use filename without extension as fallback
		base := filepath.Base(path)
		musicMeta.Title = strings.TrimSuffix(base, filepath.Ext(base))
	}

	// Handle missing artist
	if musicMeta.Artist == "" {
		musicMeta.Artist = "Unknown Artist"
	}

	// Handle missing album
	if musicMeta.Album == "" {
		musicMeta.Album = "Unknown Album"
	}

	// Parse year
	if year := metadata.Year(); year != 0 {
		musicMeta.Year = year
	}

	// Parse track number
	if trackNum, _ := metadata.Track(); trackNum != 0 {
		musicMeta.TrackNumber = trackNum
	}

	// Get file info for additional metadata
	if fileInfo, err := os.Stat(path); err == nil {
		musicMeta.Duration = int(p.estimateDuration(fileInfo.Size(), path).Seconds())
	}

	return musicMeta, nil
}

// saveMetadata saves music metadata to the database
func (p *EnrichmentCorePlugin) saveMetadata(metadata *database.MusicMetadata, mediaFileID string) error {
	// Set the media file ID
	metadata.MediaFileID = mediaFileID

	// Check if metadata already exists
	var existing database.MusicMetadata
	result := p.db.Where("media_file_id = ?", mediaFileID).First(&existing)
	
	if result.Error == nil {
		// Update existing metadata
		metadata.ID = existing.ID
		metadata.CreatedAt = existing.CreatedAt
		return p.db.Save(metadata).Error
	} else {
		// Create new metadata
		return p.db.Create(metadata).Error
	}
}

// extractAndSaveArtwork extracts and saves artwork from a music file
func (p *EnrichmentCorePlugin) extractAndSaveArtwork(path string, mediaFileID string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Parse tags to get artwork
	metadata, err := tag.ReadFrom(file)
	if err != nil {
		return fmt.Errorf("failed to read tags: %w", err)
	}

	artwork := metadata.Picture()
	if artwork == nil || len(artwork.Data) == 0 {
		return fmt.Errorf("no artwork found in file")
	}

	// Detect MIME type
	mimeType := p.detectImageMimeType(artwork.Data)
	if mimeType == "" {
		return fmt.Errorf("unsupported image format")
	}

	// Save artwork using asset module
	assetManager := assetmodule.GetAssetManager()
	if assetManager == nil {
		return fmt.Errorf("asset manager not available")
	}

	// Generate unique entity ID for the album (we'll use a placeholder for now)
	entityID := uuid.New()

	// Create asset request
	request := &assetmodule.AssetRequest{
		EntityType: assetmodule.EntityTypeAlbum,
		EntityID:   entityID,
		Type:       assetmodule.AssetTypeCover,
		Source:     assetmodule.SourceEmbedded,
		Data:       artwork.Data,
		Format:     mimeType,
		Preferred:  true,
	}

	// Save the artwork
	_, err = assetManager.SaveAsset(request)
	if err != nil {
		return fmt.Errorf("failed to save artwork: %w", err)
	}

	log.Printf("INFO: Saved artwork for media file %s", mediaFileID)
	return nil
}

// detectImageMimeType detects the MIME type of image data
func (p *EnrichmentCorePlugin) detectImageMimeType(data []byte) string {
	if len(data) < 12 {
		return ""
	}

	// Check for JPEG
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8 {
		return "image/jpeg"
	}

	// Check for PNG
	if len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n" {
		return "image/png"
	}

	// Check for WebP
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}

	// Check for GIF
	if len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a") {
		return "image/gif"
	}

	return ""
}

// estimateDuration estimates the duration of an audio file based on file size
func (p *EnrichmentCorePlugin) estimateDuration(size int64, path string) time.Duration {
	// Very rough estimation based on file size and format
	// This is a fallback when proper duration extraction fails
	
	ext := strings.ToLower(filepath.Ext(path))
	var avgBitrate int64 // bits per second
	
	switch ext {
	case ".mp3":
		avgBitrate = 128000 // 128 kbps average
	case ".flac":
		avgBitrate = 1000000 // ~1 Mbps average for lossless
	case ".wav":
		avgBitrate = 1411200 // CD quality: 44.1kHz * 16bit * 2ch
	case ".m4a", ".aac":
		avgBitrate = 128000 // 128 kbps average
	case ".ogg":
		avgBitrate = 160000 // 160 kbps average
	default:
		avgBitrate = 128000 // Default assumption
	}
	
	// Calculate duration: (file_size_bytes * 8) / bitrate_bps
	durationSeconds := (size * 8) / avgBitrate
	
	// Sanity check: limit to reasonable range (1 second to 2 hours)
	if durationSeconds < 1 {
		durationSeconds = 1
	} else if durationSeconds > 7200 { // 2 hours
		durationSeconds = 7200
	}
	
	return time.Duration(durationSeconds) * time.Second
}

// IsEnabled returns whether the plugin is enabled
func (p *EnrichmentCorePlugin) IsEnabled() bool {
	return p.enabled
}

// Initialize initializes the plugin
func (p *EnrichmentCorePlugin) Initialize() error {
	log.Printf("INFO: Initializing enrichment core plugin")
	p.enabled = true
	return nil
}

// Shutdown shuts down the plugin
func (p *EnrichmentCorePlugin) Shutdown() error {
	log.Printf("INFO: Shutting down enrichment core plugin")
	p.enabled = false
	return nil
} 