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
	"github.com/mantonx/viewra/internal/modules/assetmodule"
	"github.com/mantonx/viewra/internal/modules/pluginmodule"
	"gorm.io/gorm"
)

// Register Enrichment core plugin with the correct pluginmodule registry
func init() {
	pluginmodule.RegisterCorePluginFactory("enrichment", func() pluginmodule.CorePlugin {
		return NewEnrichmentCorePlugin()
	})
}

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

// GetType returns the plugin type (implements BasePlugin)
func (p *EnrichmentCorePlugin) GetType() string {
	return "music"
}

// GetSupportedExtensions returns supported file extensions
func (p *EnrichmentCorePlugin) GetSupportedExtensions() []string {
	return []string{".mp3", ".flac", ".m4a", ".aac", ".ogg", ".wav"}
}

// GetPluginType returns the plugin type
func (p *EnrichmentCorePlugin) GetPluginType() string {
	return "enrichment"
}

// GetDisplayName returns a human-readable display name for the plugin
func (p *EnrichmentCorePlugin) GetDisplayName() string {
	return "Music Metadata Extractor Core Plugin"
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
func (p *EnrichmentCorePlugin) HandleFile(path string, ctx *pluginmodule.MetadataContext) error {
	if !p.enabled {
		return fmt.Errorf("enrichment core plugin is disabled")
	}

	log.Printf("INFO: Enrichment core plugin processing file: %s", path)

	// Get database connection from context
	db := ctx.DB
	p.db = db

	// IMPORTANT: Only process files from music libraries, not other library types
	// Check if the media file belongs to a music library
	if ctx.MediaFile != nil && ctx.MediaFile.LibraryID != 0 {
		var library database.MediaLibrary
		if err := db.First(&library, ctx.MediaFile.LibraryID).Error; err != nil {
			return fmt.Errorf("failed to get library info: %w", err)
		}

		// Only process if this is a music library
		if library.Type != "music" {
			log.Printf("DEBUG: Skipping file %s - not from music library (library type: %s)", path, library.Type)
			return nil
		}

		log.Printf("DEBUG: Processing music file from music library: %s", path)
	}
	
	// IMPORTANT: Skip image files - they should not be processed as tracks
	// Image files are now properly classified as MediaTypeImage by the scanner
	if ctx.MediaFile != nil && ctx.MediaFile.MediaType == database.MediaTypeImage {
		log.Printf("DEBUG: Skipping image file %s - not an audio track (media_type: %s)", path, ctx.MediaFile.MediaType)
		return nil
	}
	
	// IMPORTANT: Only process track files - skip movies, episodes, etc.
	if ctx.MediaFile != nil && ctx.MediaFile.MediaType != database.MediaTypeTrack {
		log.Printf("DEBUG: Skipping file %s - not a track (media_type: %s)", path, ctx.MediaFile.MediaType)
		return nil
	}

	// Extract metadata from file
	trackInfo, err := p.extractMetadata(path)
	if err != nil {
		log.Printf("WARNING: Failed to extract metadata from %s: %v", path, err)
		// Continue with empty metadata rather than failing completely
		trackInfo = &TrackInfo{
			Title:  filepath.Base(path),
			Artist: "Unknown Artist",
			Album:  "Unknown Album",
		}
	}

	// Create or get Artist
	artist, err := p.createOrGetArtist(trackInfo.Artist)
	if err != nil {
		return fmt.Errorf("failed to create/get artist: %w", err)
	}

	// Create or get Album
	album, err := p.createOrGetAlbum(trackInfo.Album, artist.ID, trackInfo.Year)
	if err != nil {
		return fmt.Errorf("failed to create/get album: %w", err)
	}

	// Create or update Track
	track, err := p.createOrUpdateTrack(trackInfo, artist.ID, album.ID)
	if err != nil {
		return fmt.Errorf("failed to create/update track: %w", err)
	}

	// Update MediaFile to link to the track
	if err := p.linkMediaFileToTrack(ctx.MediaFile.ID, track.ID); err != nil {
		return fmt.Errorf("failed to link media file to track: %w", err)
	}

	// DEBUG: Verify the database update was successful
	var verifyMediaFile database.MediaFile
	if err := p.db.Where("id = ?", ctx.MediaFile.ID).First(&verifyMediaFile).Error; err == nil {
		log.Printf("DEBUG: Verified MediaFile update - ID: %s, MediaID: %s, MediaType: %s", 
			verifyMediaFile.ID, verifyMediaFile.MediaID, verifyMediaFile.MediaType)
	} else {
		log.Printf("ERROR: Failed to verify MediaFile update: %v", err)
	}

	// Create MediaEnrichment record to track that this plugin processed the media
	enrichment := database.MediaEnrichment{
		MediaID:   track.ID,
		MediaType: database.MediaTypeTrack,
		Plugin:    ctx.PluginID,
		Payload: fmt.Sprintf("{\"title\":\"%s\",\"artist\":\"%s\",\"album\":\"%s\",\"source\":\"file_tags\"}",
			trackInfo.Title, trackInfo.Artist, trackInfo.Album),
		UpdatedAt: time.Now(),
	}

	// Use UPSERT to handle existing records
	if err := db.Save(&enrichment).Error; err != nil {
		log.Printf("WARNING: Failed to create enrichment record: %v", err)
		// Not a fatal error - continue without enrichment tracking
	}

	// Extract and save artwork if present
	if err := p.extractAndSaveArtwork(path, album.ID); err != nil {
		log.Printf("WARNING: Failed to extract artwork from %s: %v", path, err)
		// Not a fatal error - continue without artwork
	}

	log.Printf("INFO: Successfully processed file with enrichment core plugin: %s", path)
	return nil
}

// TrackInfo holds extracted track information
type TrackInfo struct {
	Title       string
	Artist      string
	Album       string
	Genre       string
	Year        int
	TrackNumber int
	Duration    int
}

// extractMetadata extracts metadata from a music file using tag library
func (p *EnrichmentCorePlugin) extractMetadata(path string) (*TrackInfo, error) {
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
	trackInfo := &TrackInfo{
		Title:  metadata.Title(),
		Artist: metadata.Artist(),
		Album:  metadata.Album(),
		Genre:  metadata.Genre(),
	}

	// Handle missing title
	if trackInfo.Title == "" {
		// Use filename without extension as fallback
		base := filepath.Base(path)
		trackInfo.Title = strings.TrimSuffix(base, filepath.Ext(base))
	}

	// Handle missing artist
	if trackInfo.Artist == "" {
		trackInfo.Artist = "Unknown Artist"
	}

	// Handle missing album
	if trackInfo.Album == "" {
		trackInfo.Album = "Unknown Album"
	}

	// Parse year
	if year := metadata.Year(); year != 0 {
		trackInfo.Year = year
	}

	// Parse track number
	if trackNum, _ := metadata.Track(); trackNum != 0 {
		trackInfo.TrackNumber = trackNum
	}

	// Get file info for additional metadata
	if fileInfo, err := os.Stat(path); err == nil {
		trackInfo.Duration = int(p.estimateDuration(fileInfo.Size(), path).Seconds())
	}

	return trackInfo, nil
}

// createOrGetArtist creates a new artist or returns existing one
func (p *EnrichmentCorePlugin) createOrGetArtist(artistName string) (*database.Artist, error) {
	var artist database.Artist

	// Check if artist already exists
	result := p.db.Where("name = ?", artistName).First(&artist)
	if result.Error == nil {
		return &artist, nil
	}

	// Create new artist
	artist = database.Artist{
		ID:   uuid.New().String(),
		Name: artistName,
	}

	if err := p.db.Create(&artist).Error; err != nil {
		return nil, fmt.Errorf("failed to create artist: %w", err)
	}

	return &artist, nil
}

// createOrGetAlbum creates a new album or returns existing one
func (p *EnrichmentCorePlugin) createOrGetAlbum(albumTitle string, artistID string, year int) (*database.Album, error) {
	var album database.Album

	// Check if album already exists for this artist
	result := p.db.Where("title = ? AND artist_id = ?", albumTitle, artistID).First(&album)
	if result.Error == nil {
		return &album, nil
	}

	// Create new album
	album = database.Album{
		ID:       uuid.New().String(),
		Title:    albumTitle,
		ArtistID: artistID,
	}

	// Set release date if year is provided
	if year > 0 {
		releaseDate := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
		album.ReleaseDate = &releaseDate
	}

	if err := p.db.Create(&album).Error; err != nil {
		return nil, fmt.Errorf("failed to create album: %w", err)
	}

	return &album, nil
}

// createOrUpdateTrack creates a new track or updates existing one
func (p *EnrichmentCorePlugin) createOrUpdateTrack(trackInfo *TrackInfo, artistID string, albumID string) (*database.Track, error) {
	var track database.Track

	// Check if track already exists for this album
	result := p.db.Where("title = ? AND album_id = ?", trackInfo.Title, albumID).First(&track)

	if result.Error == nil {
		// Update existing track
		track.ArtistID = artistID
		track.TrackNumber = trackInfo.TrackNumber
		track.Duration = trackInfo.Duration

		if err := p.db.Save(&track).Error; err != nil {
			return nil, fmt.Errorf("failed to update track: %w", err)
		}

		return &track, nil
	}

	// Create new track
	track = database.Track{
		ID:          uuid.New().String(),
		Title:       trackInfo.Title,
		AlbumID:     albumID,
		ArtistID:    artistID,
		TrackNumber: trackInfo.TrackNumber,
		Duration:    trackInfo.Duration,
	}

	if err := p.db.Create(&track).Error; err != nil {
		return nil, fmt.Errorf("failed to create track: %w", err)
	}

	return &track, nil
}

// linkMediaFileToTrack updates the MediaFile to reference the track
func (p *EnrichmentCorePlugin) linkMediaFileToTrack(mediaFileID string, trackID string) error {
	return p.db.Model(&database.MediaFile{}).
		Where("id = ?", mediaFileID).
		Updates(map[string]interface{}{
			"media_id":   trackID,
			"media_type": database.MediaTypeTrack,
		}).Error
}

// extractAndSaveArtwork extracts and saves artwork from a music file
func (p *EnrichmentCorePlugin) extractAndSaveArtwork(path string, albumID string) error {
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

	// Parse the albumID string to UUID
	albumUUID, err := uuid.Parse(albumID)
	if err != nil {
		return fmt.Errorf("invalid album ID format: %w", err)
	}

	// Create asset request using the real Album.ID
	request := &assetmodule.AssetRequest{
		EntityType: assetmodule.EntityTypeAlbum,
		EntityID:   albumUUID,
		Type:       assetmodule.AssetTypeCover,
		Source:     assetmodule.SourceEmbedded,
		PluginID:   "core_enrichment",
		Data:       artwork.Data,
		Format:     mimeType,
		Preferred:  true,
	}

	// Save the artwork
	_, err = assetManager.SaveAsset(request)
	if err != nil {
		return fmt.Errorf("failed to save artwork: %w", err)
	}

	log.Printf("INFO: Saved artwork for album %s", albumID)
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

// IsEnabled returns whether the plugin is enabled (implements CorePlugin)
func (p *EnrichmentCorePlugin) IsEnabled() bool {
	return p.enabled
}

// Enable enables the plugin (implements CorePlugin)
func (p *EnrichmentCorePlugin) Enable() error {
	p.enabled = true
	return p.Initialize()
}

// Disable disables the plugin (implements CorePlugin)
func (p *EnrichmentCorePlugin) Disable() error {
	p.enabled = false
	return p.Shutdown()
}

// Initialize performs any setup needed for the plugin (implements CorePlugin)
func (p *EnrichmentCorePlugin) Initialize() error {
	log.Printf("✅ Music Metadata Extractor initialized - music metadata extraction available")
	return nil
}

// Shutdown performs any cleanup needed when the plugin is disabled (implements CorePlugin)
func (p *EnrichmentCorePlugin) Shutdown() error {
	log.Printf("DEBUG: Shutting down Music Metadata Extractor Core Plugin")
	return nil
}
