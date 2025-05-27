// Package musicbrainz_enricher provides metadata enrichment for music files using MusicBrainz.
package musicbrainz_enricher

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	// PluginID is the unique identifier for this plugin
	PluginID = "musicbrainz_enricher"
	
	// PluginName is the human-readable name
	PluginName = "MusicBrainz Metadata Enricher"
	
	// PluginVersion follows semantic versioning
	PluginVersion = "1.0.0"
	
	// PluginDescription describes what this plugin does
	PluginDescription = "Enriches music metadata and artwork using the MusicBrainz database"
	
	// PluginAuthor identifies the plugin author
	PluginAuthor = "Viewra Team"
)

// =============================================================================
// PLUGIN INTERFACE DEFINITIONS (self-contained)
// =============================================================================

// PluginInfo contains metadata about this plugin
type PluginInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Description string    `json:"description"`
	Author      string    `json:"author"`
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Plugin interface types (copied from main app for self-contained plugin)
type PluginContext struct {
	PluginID   string
	Logger     PluginLogger
	Database   Database
	Config     PluginConfig
	HTTPClient HTTPClient
	FileSystem FileSystemAccess
	Events     EventBus
	Hooks      HookRegistry
}

type PluginType string
type PluginStatus string

const (
	PluginTypeMetadataScraper PluginType = "metadata_scraper"
	PluginStatusEnabled       PluginStatus = "enabled"
)

type PluginLogger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
}

type Database interface {
	GetDB() interface{}
}

type PluginConfig interface {
	Get(key string) interface{}
	Set(key string, value interface{}) error
	GetString(key string) string
	GetInt(key string) int
	GetBool(key string) bool
}

type HTTPClient interface {
	Get(url string) ([]byte, error)
	Post(url string, data []byte) ([]byte, error)
	Put(url string, data []byte) ([]byte, error)
	Delete(url string) ([]byte, error)
}

type FileSystemAccess interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte) error
	Exists(path string) bool
	ListFiles(dir string) ([]string, error)
	CreateDir(path string) error
}

type EventBus interface {
	Publish(event string, data interface{}) error
	Subscribe(event string, handler func(data interface{})) error
	Unsubscribe(event string, handler func(data interface{}) error) error
}

type HookRegistry interface {
	Register(hook string, handler func(data interface{}) interface{}) error
	Execute(hook string, data interface{}) interface{}
	Remove(hook string, handler func(data interface{}) interface{}) error
}

// Plugin interfaces
type Plugin interface {
	Initialize(ctx *PluginContext) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Info() *PluginInfo
	Health() error
}

type DatabasePlugin interface {
	Plugin
	GetModels() []interface{}
	Migrate(db interface{}) error
	Rollback(db interface{}) error
}

type MetadataScraperPlugin interface {
	Plugin
	CanHandle(filePath string, mimeType string) bool
	ExtractMetadata(ctx context.Context, filePath string) (map[string]interface{}, error)
	SupportedTypes() []string
}

type ScannerHookPlugin interface {
	Plugin
	OnMediaFileScanned(mediaFileID uint, filePath string, metadata map[string]interface{}) error
	OnScanStarted(scanJobID uint, libraryID uint, libraryPath string) error
	OnScanCompleted(scanJobID uint, libraryID uint, stats map[string]interface{}) error
}

// =============================================================================
// PLUGIN CONFIGURATION
// =============================================================================

// Config represents the plugin configuration
type Config struct {
	Enabled     bool   `json:"enabled"`
	AutoEnrich  bool   `json:"auto_enrich"`
	APIBaseURL  string `json:"api_base_url"`
	UserAgent   string `json:"user_agent"`
	RateLimit   int    `json:"rate_limit"`
	CacheTTL    int    `json:"cache_ttl"`
	MaxRetries  int    `json:"max_retries"`
	Timeout     int    `json:"timeout"`
}

// =============================================================================
// DATABASE MODELS (Plugin's own tables)
// =============================================================================

// MusicBrainzCache represents cached MusicBrainz API responses
type MusicBrainzCache struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	QueryHash string    `gorm:"uniqueIndex;not null" json:"query_hash"`
	QueryType string    `gorm:"not null" json:"query_type"` // "recording", "release", "artist"
	Response  string    `gorm:"type:text;not null" json:"response"`
	ExpiresAt time.Time `gorm:"not null;index" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// MusicBrainzEnrichment represents enrichment data for media files
type MusicBrainzEnrichment struct {
	ID                     uint       `gorm:"primaryKey" json:"id"`
	MediaFileID            uint       `gorm:"uniqueIndex;not null" json:"media_file_id"`
	MusicBrainzRecordingID string     `gorm:"index" json:"musicbrainz_recording_id,omitempty"`
	MusicBrainzReleaseID   string     `gorm:"index" json:"musicbrainz_release_id,omitempty"`
	MusicBrainzArtistID    string     `gorm:"index" json:"musicbrainz_artist_id,omitempty"`
	EnrichedTitle          string     `json:"enriched_title,omitempty"`
	EnrichedArtist         string     `json:"enriched_artist,omitempty"`
	EnrichedAlbum          string     `json:"enriched_album,omitempty"`
	EnrichedAlbumArtist    string     `json:"enriched_album_artist,omitempty"`
	EnrichedYear           int        `json:"enriched_year,omitempty"`
	EnrichedGenre          string     `json:"enriched_genre,omitempty"`
	EnrichedTrackNumber    int        `json:"enriched_track_number,omitempty"`
	EnrichedDiscNumber     int        `json:"enriched_disc_number,omitempty"`
	MatchScore             float64    `json:"match_score"`
	ArtworkURL             string     `json:"artwork_url,omitempty"`
	ArtworkPath            string     `json:"artwork_path,omitempty"`
	EnrichedAt             time.Time  `gorm:"not null" json:"enriched_at"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

// MusicBrainzStats represents plugin statistics
type MusicBrainzStats struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	TotalEnriched     int       `gorm:"default:0" json:"total_enriched"`
	TotalAPIRequests  int       `gorm:"default:0" json:"total_api_requests"`
	TotalCacheHits    int       `gorm:"default:0" json:"total_cache_hits"`
	TotalCacheMisses  int       `gorm:"default:0" json:"total_cache_misses"`
	ArtworkDownloaded int       `gorm:"default:0" json:"artwork_downloaded"`
	LastEnrichedAt    *time.Time `json:"last_enriched_at,omitempty"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// =============================================================================
// MUSICBRAINZ API TYPES
// =============================================================================

// Recording represents a MusicBrainz recording
type Recording struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	Score        float64       `json:"score"`
	ArtistCredit []ArtistCredit `json:"artist-credit"`
	Releases     []Release     `json:"releases"`
}

// ArtistCredit represents artist credit information
type ArtistCredit struct {
	Name   string `json:"name"`
	Artist Artist `json:"artist"`
}

// Artist represents a MusicBrainz artist
type Artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Release represents a MusicBrainz release
type Release struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Date  string `json:"date"`
}

// SearchResponse represents a MusicBrainz search response
type SearchResponse struct {
	Recordings []Recording `json:"recordings"`
	Count      int         `json:"count"`
	Offset     int         `json:"offset"`
}

// =============================================================================
// PLUGIN IMPLEMENTATION
// =============================================================================

// MusicBrainzEnricher implements all the plugin interfaces
type MusicBrainzEnricher struct {
	ctx    *PluginContext
	db     *gorm.DB
	config *Config
	stats  *MusicBrainzStats
}

// NewPlugin creates a new instance of the MusicBrainz enricher plugin
func NewPlugin() Plugin {
	return &MusicBrainzEnricher{}
}

// Initialize sets up the plugin
func (e *MusicBrainzEnricher) Initialize(ctx *PluginContext) error {
	e.ctx = ctx
	
	// Get database connection
	dbInterface := ctx.Database.GetDB()
	if dbInterface == nil {
		return fmt.Errorf("database not available")
	}
	e.db = dbInterface.(*gorm.DB)
	
	// Load configuration with defaults
	e.config = &Config{
		Enabled:     true,
		AutoEnrich:  true,
		APIBaseURL:  "https://musicbrainz.org/ws/2",
		UserAgent:   "Viewra/1.0 (https://github.com/mantonx/viewra)",
		RateLimit:   1,     // 1 request per second (MusicBrainz rate limit)
		CacheTTL:    86400, // 24 hours
		MaxRetries:  3,
		Timeout:     30,
	}
	
	// Override with user configuration if available
	if ctx.Config.GetBool("enabled") {
		e.config.Enabled = ctx.Config.GetBool("enabled")
	}
	if ctx.Config.GetBool("auto_enrich") {
		e.config.AutoEnrich = ctx.Config.GetBool("auto_enrich")
	}
	if apiURL := ctx.Config.GetString("api_base_url"); apiURL != "" {
		e.config.APIBaseURL = apiURL
	}
	if userAgent := ctx.Config.GetString("user_agent"); userAgent != "" {
		e.config.UserAgent = userAgent
	}
	if rateLimit := ctx.Config.GetInt("rate_limit"); rateLimit > 0 {
		e.config.RateLimit = rateLimit
	}
	if cacheTTL := ctx.Config.GetInt("cache_ttl"); cacheTTL > 0 {
		e.config.CacheTTL = cacheTTL
	}
	if maxRetries := ctx.Config.GetInt("max_retries"); maxRetries > 0 {
		e.config.MaxRetries = maxRetries
	}
	if timeout := ctx.Config.GetInt("timeout"); timeout > 0 {
		e.config.Timeout = timeout
	}
	
	// Initialize stats
	e.stats = &MusicBrainzStats{}
	if err := e.db.FirstOrCreate(e.stats, MusicBrainzStats{ID: 1}).Error; err != nil {
		return fmt.Errorf("failed to initialize stats: %w", err)
	}
	
	ctx.Logger.Info("MusicBrainz enricher initialized")
	return nil
}

// Start begins plugin operation
func (e *MusicBrainzEnricher) Start(ctx context.Context) error {
	// Register scanner hook for automatic enrichment
	if e.config.AutoEnrich {
		e.ctx.Hooks.Register("media_file_scanned", func(data interface{}) interface{} {
			// Extract media file info from hook data
			hookData, ok := data.(map[string]interface{})
			if !ok {
				return nil
			}
			
			mediaFileID, ok := hookData["media_file_id"].(uint)
			if !ok {
				return nil
			}
			
			// Enrich in background
			go func() {
				if err := e.EnrichMediaFile(context.Background(), mediaFileID); err != nil {
					e.ctx.Logger.Error("Failed to enrich media file", "media_file_id", mediaFileID, "error", err)
				}
			}()
			
			return nil
		})
		
		e.ctx.Logger.Info("Auto-enrichment enabled - registered scanner hook")
	}
	
	e.ctx.Logger.Info("MusicBrainz enricher started")
	return nil
}

// Stop gracefully shuts down the plugin
func (e *MusicBrainzEnricher) Stop(ctx context.Context) error {
	e.ctx.Logger.Info("MusicBrainz enricher stopped")
	return nil
}

// Info returns plugin metadata
func (e *MusicBrainzEnricher) Info() *PluginInfo {
	return &PluginInfo{
		ID:          PluginID,
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Author:      PluginAuthor,
		Type:        string(PluginTypeMetadataScraper),
		Status:      string(PluginStatusEnabled),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// Health performs a health check on the plugin
func (e *MusicBrainzEnricher) Health() error {
	// Check database connection
	if e.db == nil {
		return fmt.Errorf("database connection not available")
	}
	
	// Check MusicBrainz API connectivity
	resp, err := http.Get(e.config.APIBaseURL + "/artist/5b11f4ce-a62d-471e-81fc-a69a8278c7da?fmt=json")
	if err != nil {
		return fmt.Errorf("MusicBrainz API not reachable: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return fmt.Errorf("MusicBrainz API returned status %d", resp.StatusCode)
	}
	
	return nil
}

// =============================================================================
// DATABASE PLUGIN INTERFACE
// =============================================================================

// GetModels returns the database models that this plugin needs
func (e *MusicBrainzEnricher) GetModels() []interface{} {
	return []interface{}{
		&MusicBrainzCache{},
		&MusicBrainzEnrichment{},
		&MusicBrainzStats{},
	}
}

// Migrate creates/updates the plugin's database tables
func (e *MusicBrainzEnricher) Migrate(db interface{}) error {
	// Custom migration logic if needed
	// The auto-migration is handled by the plugin manager
	return nil
}

// Rollback removes the plugin's database tables
func (e *MusicBrainzEnricher) Rollback(db interface{}) error {
	gormDB := db.(*gorm.DB)
	return gormDB.Migrator().DropTable(
		&MusicBrainzCache{},
		&MusicBrainzEnrichment{},
		&MusicBrainzStats{},
	)
}

// =============================================================================
// SCANNER HOOK PLUGIN INTERFACE
// =============================================================================

// OnMediaFileScanned is called when a media file is scanned and processed
func (e *MusicBrainzEnricher) OnMediaFileScanned(mediaFileID uint, filePath string, metadata map[string]interface{}) error {
	if !e.config.AutoEnrich {
		return nil
	}
	
	return e.EnrichMediaFile(context.Background(), mediaFileID)
}

// OnScanStarted is called when a scan job starts
func (e *MusicBrainzEnricher) OnScanStarted(scanJobID uint, libraryID uint, libraryPath string) error {
	e.ctx.Logger.Info("Scan started - MusicBrainz enricher ready", "scan_job_id", scanJobID, "library_id", libraryID)
	return nil
}

// OnScanCompleted is called when a scan job completes
func (e *MusicBrainzEnricher) OnScanCompleted(scanJobID uint, libraryID uint, stats map[string]interface{}) error {
	e.ctx.Logger.Info("Scan completed", "scan_job_id", scanJobID, "library_id", libraryID)
	return nil
}

// =============================================================================
// METADATA SCRAPER PLUGIN INTERFACE
// =============================================================================

// CanHandle checks if this plugin can handle the given file
func (e *MusicBrainzEnricher) CanHandle(filePath string, mimeType string) bool {
	if !e.config.Enabled {
		return false
	}
	
	// Check if it's an audio file
	audioTypes := []string{
		"audio/mpeg", "audio/mp3", "audio/flac", "audio/ogg", 
		"audio/wav", "audio/aac", "audio/m4a",
	}
	
	for _, audioType := range audioTypes {
		if strings.Contains(mimeType, audioType) {
			return true
		}
	}
	
	// Check file extension as fallback
	if strings.Contains(filePath, ".") {
		ext := strings.ToLower(filePath[strings.LastIndex(filePath, ".")+1:])
		audioExts := []string{"mp3", "flac", "ogg", "wav", "aac", "m4a", "wma"}
		
		for _, audioExt := range audioExts {
			if ext == audioExt {
				return true
			}
		}
	}
	
	return false
}

// ExtractMetadata extracts metadata from a file
func (e *MusicBrainzEnricher) ExtractMetadata(ctx context.Context, filePath string) (map[string]interface{}, error) {
	if !e.CanHandle(filePath, "") {
		return nil, fmt.Errorf("file type not supported: %s", filePath)
	}
	
	// This plugin enriches existing metadata rather than extracting raw metadata
	// Return basic file information
	return map[string]interface{}{
		"plugin":      "musicbrainz_enricher",
		"file_path":   filePath,
		"supported":   true,
		"enrichment":  "available",
	}, nil
}

// SupportedTypes returns the file types this plugin supports
func (e *MusicBrainzEnricher) SupportedTypes() []string {
	return []string{
		"audio/mpeg",     // MP3
		"audio/flac",     // FLAC
		"audio/ogg",      // OGG
		"audio/wav",      // WAV
		"audio/aac",      // AAC
		"audio/m4a",      // M4A
		"audio/wma",      // WMA
	}
}

// =============================================================================
// CORE ENRICHMENT FUNCTIONALITY
// =============================================================================

// EnrichMediaFile enriches a media file with MusicBrainz data
func (e *MusicBrainzEnricher) EnrichMediaFile(ctx context.Context, mediaFileID uint) error {
	// Check if already enriched
	var existing MusicBrainzEnrichment
	if err := e.db.Where("media_file_id = ?", mediaFileID).First(&existing).Error; err == nil {
		e.ctx.Logger.Debug("Media file already enriched", "media_file_id", mediaFileID)
		return nil
	}
	
	// Get media file and metadata from main database
	var mediaFile struct {
		ID            uint   `json:"id"`
		Path          string `json:"path"`
		MusicMetadata *struct {
			Title  string `json:"title"`
			Artist string `json:"artist"`
			Album  string `json:"album"`
			Year   int    `json:"year"`
			Track  int    `json:"track"`
		} `json:"music_metadata"`
	}
	
	if err := e.db.Table("media_files").
		Select("media_files.id, media_files.path").
		Joins("LEFT JOIN music_metadata ON music_metadata.media_file_id = media_files.id").
		Where("media_files.id = ?", mediaFileID).
		First(&mediaFile).Error; err != nil {
		return fmt.Errorf("failed to get media file: %w", err)
	}
	
	if mediaFile.MusicMetadata == nil {
		e.ctx.Logger.Debug("No music metadata available for enrichment", "media_file_id", mediaFileID)
		return nil
	}
	
	// Search MusicBrainz
	recording, err := e.searchRecording(mediaFile.MusicMetadata.Title, mediaFile.MusicMetadata.Artist, mediaFile.MusicMetadata.Album)
	if err != nil {
		return fmt.Errorf("failed to search MusicBrainz: %w", err)
	}
	
	if recording == nil {
		e.ctx.Logger.Debug("No MusicBrainz match found", "media_file_id", mediaFileID)
		return nil
	}
	
	// Create enrichment record
	enrichment := &MusicBrainzEnrichment{
		MediaFileID:            mediaFileID,
		MusicBrainzRecordingID: recording.ID,
		EnrichedTitle:          recording.Title,
		MatchScore:             recording.Score,
		EnrichedAt:             time.Now(),
	}
	
	// Add artist information
	if len(recording.ArtistCredit) > 0 {
		enrichment.EnrichedArtist = recording.ArtistCredit[0].Name
		enrichment.MusicBrainzArtistID = recording.ArtistCredit[0].Artist.ID
	}
	
	// Add release information if available
	if len(recording.Releases) > 0 {
		release := recording.Releases[0]
		enrichment.MusicBrainzReleaseID = release.ID
		enrichment.EnrichedAlbum = release.Title
		if release.Date != "" && len(release.Date) >= 4 {
			if year, err := strconv.Atoi(release.Date[:4]); err == nil {
				enrichment.EnrichedYear = year
			}
		}
	}
	
	// Save enrichment
	if err := e.db.Create(enrichment).Error; err != nil {
		return fmt.Errorf("failed to save enrichment: %w", err)
	}
	
	// Update stats
	e.updateStats()
	
	e.ctx.Logger.Info("Media file enriched successfully", 
		"media_file_id", mediaFileID, 
		"recording_id", recording.ID,
		"match_score", recording.Score)
	
	return nil
}

// searchRecording searches MusicBrainz for a recording
func (e *MusicBrainzEnricher) searchRecording(title, artist, album string) (*Recording, error) {
	// Build search query
	query := fmt.Sprintf("recording:\"%s\" AND artist:\"%s\"", title, artist)
	if album != "" {
		query += fmt.Sprintf(" AND release:\"%s\"", album)
	}
	
	// Generate cache key
	queryHash := e.generateQueryHash(query)
	
	// Check cache first
	if cached, err := e.getCachedResponse("recording", queryHash); err == nil {
		e.updateCacheStats(true)
		if len(cached) > 0 {
			return &cached[0], nil
		}
		return nil, nil
	}
	
	// Make API request
	apiURL := fmt.Sprintf("%s/recording?query=%s&fmt=json&limit=5", 
		e.config.APIBaseURL, url.QueryEscape(query))
	
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var searchResp SearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	// Cache the response
	e.cacheRecordings("recording", queryHash, searchResp.Recordings)
	e.updateCacheStats(false)
	e.updateAPIStats()
	
	// Return best match (first result is usually best)
	if len(searchResp.Recordings) > 0 {
		return &searchResp.Recordings[0], nil
	}
	
	return nil, nil
}

// =============================================================================
// HELPER METHODS
// =============================================================================

// generateQueryHash generates a hash for caching queries
func (e *MusicBrainzEnricher) generateQueryHash(query string) string {
	h := md5.New()
	h.Write([]byte(query))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// getCachedResponse retrieves a cached API response
func (e *MusicBrainzEnricher) getCachedResponse(queryType, queryHash string) ([]Recording, error) {
	var cache MusicBrainzCache
	err := e.db.Where("query_type = ? AND query_hash = ? AND expires_at > ?",
		queryType, queryHash, time.Now()).First(&cache).Error
	if err != nil {
		return nil, err
	}
	
	var recordings []Recording
	if err := json.Unmarshal([]byte(cache.Response), &recordings); err != nil {
		return nil, err
	}
	
	return recordings, nil
}

// cacheRecordings caches API responses
func (e *MusicBrainzEnricher) cacheRecordings(queryType, queryHash string, recordings []Recording) {
	responseData, err := json.Marshal(recordings)
	if err != nil {
		return // Skip caching on error
	}
	
	expiresAt := time.Now().Add(time.Duration(e.config.CacheTTL) * time.Second)
	
	cache := MusicBrainzCache{
		QueryHash: queryHash,
		QueryType: queryType,
		Response:  string(responseData),
		ExpiresAt: expiresAt,
	}
	
	e.db.Save(&cache)
}

// updateStats updates plugin statistics
func (e *MusicBrainzEnricher) updateStats() {
	e.db.Model(&MusicBrainzStats{}).Where("id = ?", e.stats.ID).Updates(map[string]interface{}{
		"total_enriched":   gorm.Expr("total_enriched + 1"),
		"last_enriched_at": time.Now(),
		"updated_at":       time.Now(),
	})
}

// updateCacheStats updates cache statistics
func (e *MusicBrainzEnricher) updateCacheStats(hit bool) {
	updates := map[string]interface{}{
		"updated_at": time.Now(),
	}
	
	if hit {
		updates["total_cache_hits"] = gorm.Expr("total_cache_hits + 1")
	} else {
		updates["total_cache_misses"] = gorm.Expr("total_cache_misses + 1")
	}
	
	e.db.Model(&MusicBrainzStats{}).Where("id = ?", e.stats.ID).Updates(updates)
}

// updateAPIStats updates API request statistics
func (e *MusicBrainzEnricher) updateAPIStats() {
	e.db.Model(&MusicBrainzStats{}).Where("id = ?", e.stats.ID).Updates(map[string]interface{}{
		"total_api_requests": gorm.Expr("total_api_requests + 1"),
		"updated_at":         time.Now(),
	})
}

// =============================================================================
// PLUGIN ENTRY POINT
// =============================================================================

// Plugin variable for Go plugin system
var PluginInstance MusicBrainzEnricher

 