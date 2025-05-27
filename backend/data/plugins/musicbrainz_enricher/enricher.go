package musicbrainz_enricher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"musicbrainz_enricher/config"
	"musicbrainz_enricher/internal"
	"musicbrainz_enricher/musicbrainz"
)

// Database models for the plugin
type (
	// Cache represents cached MusicBrainz API responses
	Cache struct {
		ID        uint      `gorm:"primaryKey" json:"id"`
		QueryHash string    `gorm:"uniqueIndex;not null" json:"query_hash"`
		QueryType string    `gorm:"not null" json:"query_type"` // "recording", "release", "artist"
		Response  string    `gorm:"type:text;not null" json:"response"`
		ExpiresAt time.Time `gorm:"not null;index" json:"expires_at"`
		CreatedAt time.Time `json:"created_at"`
	}

	// Enrichment represents enrichment data for media files
	Enrichment struct {
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

	// Stats represents plugin statistics
	Stats struct {
		ID                uint      `gorm:"primaryKey" json:"id"`
		TotalEnriched     int       `gorm:"default:0" json:"total_enriched"`
		TotalAPIRequests  int       `gorm:"default:0" json:"total_api_requests"`
		TotalCacheHits    int       `gorm:"default:0" json:"total_cache_hits"`
		TotalCacheMisses  int       `gorm:"default:0" json:"total_cache_misses"`
		ArtworkDownloaded int       `gorm:"default:0" json:"artwork_downloaded"`
		LastEnrichedAt    *time.Time `json:"last_enriched_at,omitempty"`
		UpdatedAt         time.Time `json:"updated_at"`
	}
)

// Enricher handles the core enrichment logic
type Enricher struct {
	db       *gorm.DB
	config   *config.Config
	client   *musicbrainz.Client
	matcher  *musicbrainz.Matcher
	stats    *Stats
}

// NewEnricher creates a new enricher instance
func NewEnricher(db *gorm.DB, cfg *config.Config) *Enricher {
	client := musicbrainz.NewClient(cfg.UserAgent, cfg.APIRateLimit)
	matcher := musicbrainz.NewMatcher(cfg.MatchThreshold)

	return &Enricher{
		db:      db,
		config:  cfg,
		client:  client,
		matcher: matcher,
	}
}

// Initialize sets up the enricher and creates database tables
func (e *Enricher) Initialize() error {
	// Auto-migrate plugin tables
	if err := e.db.AutoMigrate(&Cache{}, &Enrichment{}, &Stats{}); err != nil {
		return fmt.Errorf("failed to migrate plugin tables: %w", err)
	}

	// Initialize or get stats
	if err := e.initializeStats(); err != nil {
		return fmt.Errorf("failed to initialize stats: %w", err)
	}

	return nil
}

// Health performs a health check
func (e *Enricher) Health() error {
	// Check database connection
	if e.db == nil {
		return fmt.Errorf("database connection is nil")
	}

	// Check if we can query the stats table
	var count int64
	if err := e.db.Model(&Stats{}).Count(&count).Error; err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}

	return nil
}

// RegisterRoutes registers HTTP endpoints for the enricher
func (e *Enricher) RegisterRoutes(router *gin.RouterGroup) {
	pluginGroup := router.Group("/musicbrainz")
	{
		// Status and statistics
		pluginGroup.GET("/status", e.handleStatus)
		pluginGroup.GET("/stats", e.handleStats)

		// Enrichment operations
		pluginGroup.POST("/enrich/:mediaFileId", e.handleEnrichFile)
		pluginGroup.POST("/enrich-batch", e.handleEnrichBatch)

		// Search and lookup
		pluginGroup.GET("/search", e.handleSearch)

		// Cache management
		pluginGroup.DELETE("/cache", e.handleClearCache)
		pluginGroup.GET("/cache/stats", e.handleCacheStats)

		// Enrichment data
		pluginGroup.GET("/enrichments", e.handleGetEnrichments)
		pluginGroup.GET("/enrichments/:mediaFileId", e.handleGetEnrichment)
		pluginGroup.DELETE("/enrichments/:mediaFileId", e.handleDeleteEnrichment)
	}
}

// Scanner Hook Interface Implementation

// OnMediaFileScanned is called when a media file is scanned and processed
func (e *Enricher) OnMediaFileScanned(mediaFileID uint, filePath string, metadata map[string]interface{}) error {
	if !e.config.Enabled || !e.config.AutoEnrich {
		return nil
	}

	// Check if file is audio
	if !internal.IsAudioFile(filePath) {
		return nil
	}

	// Enrich in background to avoid blocking the scanner
	go func() {
		ctx := context.Background()
		if err := e.enrichMediaFile(ctx, mediaFileID, metadata); err != nil {
			fmt.Printf("Failed to auto-enrich file %d: %v\n", mediaFileID, err)
		}
	}()

	return nil
}

// OnScanStarted is called when a scan job starts
func (e *Enricher) OnScanStarted(scanJobID uint, libraryID uint, libraryPath string) error {
	if !e.config.Enabled {
		return nil
	}

	fmt.Printf("MusicBrainz Enricher: Scan started for library %d (job %d) at path: %s\n",
		libraryID, scanJobID, libraryPath)
	return nil
}

// OnScanCompleted is called when a scan job completes
func (e *Enricher) OnScanCompleted(scanJobID uint, libraryID uint, stats map[string]interface{}) error {
	if !e.config.Enabled {
		return nil
	}

	fmt.Printf("MusicBrainz Enricher: Scan completed for library %d (job %d)\n",
		libraryID, scanJobID)

	// Log enrichment statistics only if database is available
	if e.db != nil {
		var enrichmentCount int64
		e.db.Model(&Enrichment{}).Count(&enrichmentCount)
		fmt.Printf("MusicBrainz Enricher: Total enriched files: %d\n", enrichmentCount)
	}

	return nil
}

// Metadata Scraper Interface Implementation

// CanHandle checks if this plugin can handle the given file
func (e *Enricher) CanHandle(filePath string, mimeType string) bool {
	if !e.config.Enabled {
		return false
	}
	
	// Check if it's an audio file
	return internal.IsAudioFile(filePath)
}

// ExtractMetadata extracts metadata from a file
// Note: This plugin enriches existing metadata rather than extracting raw metadata
// So this method returns basic file information and delegates to enrichment
func (e *Enricher) ExtractMetadata(ctx context.Context, filePath string) (map[string]interface{}, error) {
	if !e.CanHandle(filePath, "") {
		return nil, fmt.Errorf("file type not supported: %s", filePath)
	}

	// This plugin doesn't extract raw metadata from files
	// It enriches existing metadata using MusicBrainz
	// Return basic file information
	return map[string]interface{}{
		"plugin_id":   PluginID,
		"plugin_name": PluginName,
		"file_path":   filePath,
		"supported":   true,
		"note":        "This plugin enriches existing metadata rather than extracting raw metadata",
	}, nil
}

// SupportedTypes returns the file types this plugin supports
func (e *Enricher) SupportedTypes() []string {
	return []string{
		"audio/mpeg",     // MP3
		"audio/flac",     // FLAC
		"audio/ogg",      // OGG
		"audio/mp4",      // M4A
		"audio/x-wav",    // WAV
		"audio/aac",      // AAC
		"audio/x-aiff",   // AIFF
		"audio/x-ms-wma", // WMA
	}
}

// Core Enrichment Logic

// enrichMediaFile performs the actual enrichment of a media file
func (e *Enricher) enrichMediaFile(ctx context.Context, mediaFileID uint, metadata map[string]interface{}) error {
	// Check if already enriched and not overwriting
	if !e.config.OverwriteExisting {
		var existing Enrichment
		if err := e.db.Where("media_file_id = ?", mediaFileID).First(&existing).Error; err == nil {
			return nil // Already enriched
		}
	}

	// Extract basic metadata
	title := internal.ExtractMetadataString(metadata, "title")
	artist := internal.ExtractMetadataString(metadata, "artist")
	album := internal.ExtractMetadataString(metadata, "album")

	if title == "" || artist == "" {
		return fmt.Errorf("insufficient metadata for enrichment (title: %q, artist: %q)", title, artist)
	}

	// Search MusicBrainz with caching
	recordings, err := e.searchWithCache(ctx, title, artist, album)
	if err != nil {
		return fmt.Errorf("failed to search MusicBrainz: %w", err)
	}

	if len(recordings) == 0 {
		return fmt.Errorf("no matches found in MusicBrainz")
	}

	// Find best match
	matchResult := e.matcher.FindBestMatch(recordings, title, artist, album)
	if !matchResult.Matched {
		return fmt.Errorf("no suitable match found (best score: %.2f, threshold: %.2f)",
			matchResult.Score, e.config.MatchThreshold)
	}

	// Convert to enriched metadata
	enrichedMeta := musicbrainz.MapToEnrichedMetadata(matchResult.Recording, matchResult.Score)

	// Create enrichment record
	enrichment := &Enrichment{
		MediaFileID:            mediaFileID,
		MusicBrainzRecordingID: enrichedMeta.MusicBrainzRecordingID,
		MusicBrainzReleaseID:   enrichedMeta.MusicBrainzReleaseID,
		MusicBrainzArtistID:    enrichedMeta.MusicBrainzArtistID,
		EnrichedTitle:          enrichedMeta.EnrichedTitle,
		EnrichedArtist:         enrichedMeta.EnrichedArtist,
		EnrichedAlbum:          enrichedMeta.EnrichedAlbum,
		EnrichedAlbumArtist:    enrichedMeta.EnrichedAlbumArtist,
		EnrichedYear:           enrichedMeta.EnrichedYear,
		EnrichedGenre:          enrichedMeta.EnrichedGenre,
		EnrichedTrackNumber:    enrichedMeta.EnrichedTrackNumber,
		EnrichedDiscNumber:     enrichedMeta.EnrichedDiscNumber,
		MatchScore:             enrichedMeta.MatchScore,
		EnrichedAt:             time.Now(),
	}

	// Download artwork if enabled
	if e.config.EnableArtwork && enrichment.MusicBrainzReleaseID != "" {
		if artworkURL, artworkPath, err := e.downloadArtwork(ctx, enrichment.MusicBrainzReleaseID, mediaFileID); err == nil {
			enrichment.ArtworkURL = artworkURL
			enrichment.ArtworkPath = artworkPath
			e.updateStats("artwork_downloaded", 1)
		}
	}

	// Save enrichment
	if err := e.db.Save(enrichment).Error; err != nil {
		return fmt.Errorf("failed to save enrichment: %w", err)
	}

	// Update statistics
	e.updateStats("total_enriched", 1)
	e.updateLastEnrichedAt()

	fmt.Printf("Successfully enriched media file %d with MusicBrainz data (score: %.2f)\n",
		mediaFileID, matchResult.Score)
	return nil
}

// searchWithCache searches MusicBrainz with caching support
func (e *Enricher) searchWithCache(ctx context.Context, title, artist, album string) ([]musicbrainz.Recording, error) {
	// Generate cache key
	query := fmt.Sprintf("title:%s artist:%s album:%s", title, artist, album)
	queryHash := internal.GenerateQueryHash(query)

	// Check cache first
	if cached, err := e.getCachedResponse("recording", queryHash); err == nil {
		e.updateStats("total_cache_hits", 1)
		return cached, nil
	}

	// Cache miss - make API request
	recordings, err := e.client.SearchRecordings(ctx, title, artist, album)
	if err != nil {
		return nil, err
	}

	// Cache the response
	e.cacheRecordings("recording", queryHash, recordings)
	e.updateStats("total_api_requests", 1)
	e.updateStats("total_cache_misses", 1)

	return recordings, nil
}

// downloadArtwork downloads artwork for a release
func (e *Enricher) downloadArtwork(ctx context.Context, releaseID string, mediaFileID uint) (string, string, error) {
	// Get cover art information
	coverArt, err := e.client.GetCoverArt(ctx, releaseID)
	if err != nil {
		return "", "", err
	}

	// Find best artwork image
	bestImage := e.client.FindBestArtwork(coverArt, e.config.ArtworkQuality)
	if bestImage == nil {
		return "", "", fmt.Errorf("no suitable artwork found")
	}

	// Map artwork info
	artworkURL, artworkPath := musicbrainz.MapArtworkInfo(coverArt, bestImage, mediaFileID, releaseID)

	// For now, just return the URL and path
	// In a full implementation, you would download and save the image
	return artworkURL, artworkPath, nil
}

// Statistics and caching helper methods

// initializeStats initializes or retrieves plugin statistics
func (e *Enricher) initializeStats() error {
	var stats Stats
	if err := e.db.First(&stats).Error; err != nil {
		// Create initial stats record
		stats = Stats{
			TotalEnriched:     0,
			TotalAPIRequests:  0,
			TotalCacheHits:    0,
			TotalCacheMisses:  0,
			ArtworkDownloaded: 0,
		}
		if err := e.db.Create(&stats).Error; err != nil {
			return err
		}
	}
	e.stats = &stats
	return nil
}

// updateStats updates a specific statistic field
func (e *Enricher) updateStats(field string, increment int) {
	updates := map[string]interface{}{
		"updated_at": time.Now(),
	}

	switch field {
	case "total_enriched":
		updates["total_enriched"] = gorm.Expr("total_enriched + ?", increment)
	case "total_api_requests":
		updates["total_api_requests"] = gorm.Expr("total_api_requests + ?", increment)
	case "total_cache_hits":
		updates["total_cache_hits"] = gorm.Expr("total_cache_hits + ?", increment)
	case "total_cache_misses":
		updates["total_cache_misses"] = gorm.Expr("total_cache_misses + ?", increment)
	case "artwork_downloaded":
		updates["artwork_downloaded"] = gorm.Expr("artwork_downloaded + ?", increment)
	}

	e.db.Model(&Stats{}).Where("id = ?", e.stats.ID).Updates(updates)
}

// updateLastEnrichedAt updates the last enriched timestamp
func (e *Enricher) updateLastEnrichedAt() {
	now := time.Now()
	e.db.Model(&Stats{}).Where("id = ?", e.stats.ID).Updates(map[string]interface{}{
		"last_enriched_at": &now,
		"updated_at":       now,
	})
}

// getCachedResponse retrieves a cached API response
func (e *Enricher) getCachedResponse(queryType, queryHash string) ([]musicbrainz.Recording, error) {
	var cache Cache
	err := e.db.Where("query_type = ? AND query_hash = ? AND expires_at > ?",
		queryType, queryHash, time.Now()).First(&cache).Error
	if err != nil {
		return nil, err
	}

	var recordings []musicbrainz.Recording
	if err := json.Unmarshal([]byte(cache.Response), &recordings); err != nil {
		return nil, err
	}

	return recordings, nil
}

// cacheRecordings caches API response data
func (e *Enricher) cacheRecordings(queryType, queryHash string, recordings []musicbrainz.Recording) {
	responseData, err := json.Marshal(recordings)
	if err != nil {
		return // Skip caching on error
	}

	expiresAt := time.Now().Add(e.config.CacheDuration())

	cache := Cache{
		QueryHash: queryHash,
		QueryType: queryType,
		Response:  string(responseData),
		ExpiresAt: expiresAt,
	}

	// Use UPSERT to handle duplicates
	e.db.Save(&cache)
}

// HTTP Handlers

// handleStatus returns plugin status and configuration
func (e *Enricher) handleStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"plugin":  "musicbrainz_enricher",
		"status":  "running",
		"enabled": e.config.Enabled,
		"config":  e.config,
	})
}

// handleStats returns plugin statistics
func (e *Enricher) handleStats(c *gin.Context) {
	var stats Stats
	if err := e.db.First(&stats).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// handleEnrichFile enriches a specific media file
func (e *Enricher) handleEnrichFile(c *gin.Context) {
	mediaFileIDStr := c.Param("mediaFileId")
	mediaFileID, err := strconv.ParseUint(mediaFileIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid media file ID"})
		return
	}

	// Get metadata from request body
	var request struct {
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := e.enrichMediaFile(c.Request.Context(), uint(mediaFileID), request.Metadata); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "File enriched successfully",
		"media_file_id": uint(mediaFileID),
	})
}

// handleEnrichBatch enriches multiple media files
func (e *Enricher) handleEnrichBatch(c *gin.Context) {
	var request struct {
		MediaFiles []struct {
			ID       uint                   `json:"id"`
			Metadata map[string]interface{} `json:"metadata"`
		} `json:"media_files"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	results := make(map[uint]string)
	for _, mediaFile := range request.MediaFiles {
		if err := e.enrichMediaFile(c.Request.Context(), mediaFile.ID, mediaFile.Metadata); err != nil {
			results[mediaFile.ID] = fmt.Sprintf("error: %v", err)
		} else {
			results[mediaFile.ID] = "success"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Batch enrichment completed",
		"results": results,
	})
}

// handleSearch searches MusicBrainz database
func (e *Enricher) handleSearch(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter 'q' is required"})
		return
	}

	// Parse query for title, artist, album
	title := c.Query("title")
	artist := c.Query("artist")
	album := c.Query("album")

	// If no specific fields, try to parse from general query
	if title == "" && artist == "" && album == "" {
		// Simple parsing - in production you'd want more sophisticated parsing
		title = query
	}

	recordings, err := e.client.SearchRecordings(c.Request.Context(), title, artist, album)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"query":   query,
		"results": recordings,
		"count":   len(recordings),
	})
}

// handleClearCache clears the API response cache
func (e *Enricher) handleClearCache(c *gin.Context) {
	if err := e.db.Where("1 = 1").Delete(&Cache{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear cache"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Cache cleared successfully"})
}

// handleCacheStats returns cache statistics
func (e *Enricher) handleCacheStats(c *gin.Context) {
	var count int64
	e.db.Model(&Cache{}).Where("expires_at > ?", time.Now()).Count(&count)

	c.JSON(http.StatusOK, gin.H{
		"active_entries": count,
		"cache_duration": fmt.Sprintf("%d hours", e.config.CacheDurationHours),
	})
}

// handleGetEnrichments returns all enrichments with pagination
func (e *Enricher) handleGetEnrichments(c *gin.Context) {
	var enrichments []Enrichment

	query := e.db.Model(&Enrichment{})

	// Add pagination
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if err := query.Limit(limit).Offset(offset).Find(&enrichments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get enrichments"})
		return
	}

	var total int64
	e.db.Model(&Enrichment{}).Count(&total)

	c.JSON(http.StatusOK, gin.H{
		"enrichments": enrichments,
		"total":       total,
		"limit":       limit,
		"offset":      offset,
	})
}

// handleGetEnrichment returns enrichment for a specific media file
func (e *Enricher) handleGetEnrichment(c *gin.Context) {
	mediaFileIDStr := c.Param("mediaFileId")
	mediaFileID, err := strconv.ParseUint(mediaFileIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid media file ID"})
		return
	}

	var enrichment Enrichment
	if err := e.db.Where("media_file_id = ?", mediaFileID).First(&enrichment).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Enrichment not found"})
		return
	}

	c.JSON(http.StatusOK, enrichment)
}

// handleDeleteEnrichment deletes enrichment for a specific media file
func (e *Enricher) handleDeleteEnrichment(c *gin.Context) {
	mediaFileIDStr := c.Param("mediaFileId")
	mediaFileID, err := strconv.ParseUint(mediaFileIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid media file ID"})
		return
	}

	if err := e.db.Where("media_file_id = ?", mediaFileID).Delete(&Enrichment{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete enrichment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Enrichment deleted successfully"})
} 