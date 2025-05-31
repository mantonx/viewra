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
			"hasArtwork":  metadata.HasArtwork,
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
		Title:       metadata.Title(),
		Artist:      metadata.Artist(),
		Album:       metadata.Album(),
		AlbumArtist: metadata.AlbumArtist(),
		Genre:       metadata.Genre(),
		Format:      strings.ToUpper(filepath.Ext(path)[1:]), // Remove dot and uppercase
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
	if trackNum, trackTotal := metadata.Track(); trackNum != 0 {
		musicMeta.Track = trackNum
		musicMeta.TrackTotal = trackTotal
	}

	// Parse disc number
	if discNum, discTotal := metadata.Disc(); discNum != 0 {
		musicMeta.Disc = discNum
		musicMeta.DiscTotal = discTotal
	}

	// Get file info for additional metadata
	if fileInfo, err := os.Stat(path); err == nil {
		musicMeta.Duration = p.estimateDuration(fileInfo.Size(), path)
	}

	// Check if artwork is present
	if artwork := metadata.Picture(); artwork != nil && len(artwork.Data) > 0 {
		musicMeta.HasArtwork = true
	}

	// Detect audio format details if possible
	p.detectAudioFormat(musicMeta, metadata)

	return musicMeta, nil
}

// saveMetadata saves music metadata to the database
func (p *EnrichmentCorePlugin) saveMetadata(metadata *database.MusicMetadata, mediaFileID uint) error {
	metadata.MediaFileID = mediaFileID

	// Check if metadata already exists
	var existingMeta database.MusicMetadata
	err := p.db.Where("media_file_id = ?", mediaFileID).First(&existingMeta).Error
	
	if err == gorm.ErrRecordNotFound {
		// Create new metadata record
		if err := p.db.Create(metadata).Error; err != nil {
			return fmt.Errorf("failed to create metadata: %w", err)
		}
		log.Printf("INFO: Created new metadata record for media file %d", mediaFileID)
	} else if err != nil {
		return fmt.Errorf("failed to check existing metadata: %w", err)
	} else {
		// Update existing metadata record
		metadata.ID = existingMeta.ID
		if err := p.db.Save(metadata).Error; err != nil {
			return fmt.Errorf("failed to update metadata: %w", err)
		}
		log.Printf("INFO: Updated existing metadata record for media file %d", mediaFileID)
	}

	return nil
}

// extractAndSaveArtwork extracts embedded artwork and saves it using the asset system
func (p *EnrichmentCorePlugin) extractAndSaveArtwork(path string, mediaFileID uint) error {
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

	// Get artwork
	artwork := metadata.Picture()
	if artwork == nil || len(artwork.Data) == 0 {
		return nil // No artwork present, not an error
	}

	// Generate deterministic album UUID for asset system
	albumIDString := fmt.Sprintf("album-placeholder-%d", mediaFileID)
	albumID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(albumIDString))

	// Determine MIME type from artwork
	mimeType := artwork.MIMEType
	if mimeType == "" {
		mimeType = p.detectImageMimeType(artwork.Data)
	}

	// Create asset request for the new asset system
	request := &assetmodule.AssetRequest{
		EntityType: assetmodule.EntityTypeAlbum,
		EntityID:   albumID,
		Type:       assetmodule.AssetTypeCover,
		Source:     assetmodule.SourceEmbedded, // Indicates it's extracted from file
		Data:       artwork.Data,
		Format:     mimeType,
		Preferred:  true, // Embedded artwork is preferred
	}

	// Save using the asset manager
	response, err := assetmodule.SaveMediaAsset(request)
	if err != nil {
		return fmt.Errorf("failed to save artwork: %w", err)
	}

	log.Printf("INFO: Successfully saved embedded artwork for media file %d: %s", 
		mediaFileID, response.ID.String())

	return nil
}

// detectImageMimeType detects MIME type from image data
func (p *EnrichmentCorePlugin) detectImageMimeType(data []byte) string {
	if len(data) < 16 {
		return "image/jpeg" // Default fallback
	}

	// Check for common image signatures
	switch {
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF: // JPEG
		return "image/jpeg"
	case len(data) >= 8 && data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47: // PNG
		return "image/png"
	case len(data) >= 6 && data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46: // GIF
		return "image/gif"
	case len(data) >= 12 && data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46: // WebP
		if len(data) >= 12 && data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
			return "image/webp"
		}
		return "image/jpeg"
	case len(data) >= 2 && data[0] == 0x42 && data[1] == 0x4D: // BMP
		return "image/bmp"
	default:
		return "image/jpeg" // Default fallback
	}
}

// estimateDuration provides a rough duration estimate based on file size and format
func (p *EnrichmentCorePlugin) estimateDuration(size int64, path string) time.Duration {
	ext := strings.ToLower(filepath.Ext(path))
	
	// Rough estimates based on typical bitrates
	var estimatedBitrate float64
	switch ext {
	case ".mp3":
		estimatedBitrate = 192000 // 192 kbps
	case ".flac":
		estimatedBitrate = 1000000 // 1000 kbps (lossless)
	case ".m4a", ".aac":
		estimatedBitrate = 256000 // 256 kbps
	case ".ogg":
		estimatedBitrate = 192000 // 192 kbps
	case ".wav":
		estimatedBitrate = 1411200 // 1411.2 kbps (CD quality)
	default:
		estimatedBitrate = 256000 // Default
	}
	
	// Calculate duration: (file_size_bits) / (bitrate) = seconds
	durationSeconds := float64(size*8) / estimatedBitrate
	
	return time.Duration(durationSeconds * float64(time.Second))
}

// detectAudioFormat attempts to detect additional audio format details
func (p *EnrichmentCorePlugin) detectAudioFormat(musicMeta *database.MusicMetadata, metadata tag.Metadata) {
	// Try to extract additional format details from the metadata
	// This is basic - could be enhanced with more sophisticated audio analysis
	
	// Set some reasonable defaults based on format
	switch strings.ToUpper(musicMeta.Format) {
	case "MP3":
		musicMeta.Bitrate = 192 // Default assumption
		musicMeta.SampleRate = 44100
		musicMeta.Channels = 2
	case "FLAC":
		musicMeta.Bitrate = 1000 // Lossless estimate
		musicMeta.SampleRate = 44100
		musicMeta.Channels = 2
	case "M4A", "AAC":
		musicMeta.Bitrate = 256
		musicMeta.SampleRate = 44100
		musicMeta.Channels = 2
	case "OGG":
		musicMeta.Bitrate = 192
		musicMeta.SampleRate = 44100
		musicMeta.Channels = 2
	case "WAV":
		musicMeta.Bitrate = 1411 // CD quality
		musicMeta.SampleRate = 44100
		musicMeta.Channels = 2
	default:
		musicMeta.Bitrate = 192
		musicMeta.SampleRate = 44100
		musicMeta.Channels = 2
	}
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