package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/plugins"
)

// MusicBrainzEnricherPlugin implements metadata enrichment using MusicBrainz
type MusicBrainzEnricherPlugin struct {
	*plugins.BasicPlugin
	client     *MusicBrainzClient
	config     *MusicBrainzConfig
	rateLimiter *RateLimiter
}

// MusicBrainzConfig holds plugin configuration
type MusicBrainzConfig struct {
	Enabled             bool    `json:"enabled"`
	APIRateLimit        float64 `json:"api_rate_limit"`
	UserAgent           string  `json:"user_agent"`
	EnableArtwork       bool    `json:"enable_artwork"`
	ArtworkMaxSize      int     `json:"artwork_max_size"`
	ArtworkQuality      string  `json:"artwork_quality"`
	MatchThreshold      float64 `json:"match_threshold"`
	AutoEnrich          bool    `json:"auto_enrich"`
	OverwriteExisting   bool    `json:"overwrite_existing"`
	CacheDurationHours  int     `json:"cache_duration_hours"`
}

// MusicBrainzClient handles API communication
type MusicBrainzClient struct {
	baseURL   string
	userAgent string
	httpClient *http.Client
}

// RateLimiter implements rate limiting for API requests
type RateLimiter struct {
	lastRequest time.Time
	interval    time.Duration
}

// MusicBrainzResponse structures
type MusicBrainzRecording struct {
	ID       string                    `json:"id"`
	Title    string                    `json:"title"`
	Length   int                       `json:"length,omitempty"`
	Releases []MusicBrainzRelease      `json:"releases,omitempty"`
	Artists  []MusicBrainzArtistCredit `json:"artist-credit,omitempty"`
	Score    int                       `json:"score,omitempty"`
}

type MusicBrainzRelease struct {
	ID           string                    `json:"id"`
	Title        string                    `json:"title"`
	Date         string                    `json:"date,omitempty"`
	Country      string                    `json:"country,omitempty"`
	Status       string                    `json:"status,omitempty"`
	Artists      []MusicBrainzArtistCredit `json:"artist-credit,omitempty"`
	ReleaseGroup MusicBrainzReleaseGroup   `json:"release-group,omitempty"`
	Media        []MusicBrainzMedium       `json:"media,omitempty"`
}

type MusicBrainzReleaseGroup struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	PrimaryType  string `json:"primary-type,omitempty"`
	SecondaryTypes []string `json:"secondary-types,omitempty"`
}

type MusicBrainzMedium struct {
	Position int                   `json:"position"`
	Title    string                `json:"title,omitempty"`
	Format   string                `json:"format,omitempty"`
	Tracks   []MusicBrainzTrack    `json:"tracks,omitempty"`
}

type MusicBrainzTrack struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Position int    `json:"position"`
	Length   int    `json:"length,omitempty"`
}

type MusicBrainzArtistCredit struct {
	Name   string           `json:"name"`
	Artist MusicBrainzArtist `json:"artist"`
}

type MusicBrainzArtist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

type MusicBrainzSearchResponse struct {
	Recordings []MusicBrainzRecording `json:"recordings,omitempty"`
	Releases   []MusicBrainzRelease   `json:"releases,omitempty"`
	Count      int                    `json:"count"`
	Offset     int                    `json:"offset"`
}

// CoverArtArchiveResponse for artwork
type CoverArtArchiveResponse struct {
	Images []CoverArtImage `json:"images"`
	Release string         `json:"release"`
}

type CoverArtImage struct {
	ID         string   `json:"id"`
	Image      string   `json:"image"`
	Thumbnails struct {
		Small string `json:"250"`
		Large string `json:"500"`
	} `json:"thumbnails"`
	Front    bool     `json:"front"`
	Back     bool     `json:"back"`
	Types    []string `json:"types"`
	Comment  string   `json:"comment,omitempty"`
	Approved bool     `json:"approved"`
	Edit     int      `json:"edit"`
}

// NewMusicBrainzEnricherPlugin creates a new plugin instance
func NewMusicBrainzEnricherPlugin() *MusicBrainzEnricherPlugin {
	info := &plugins.PluginInfo{
		ID:          "musicbrainz_enricher",
		Name:        "MusicBrainz Metadata Enricher",
		Version:     "1.0.0",
		Description: "Enriches music metadata and artwork using the MusicBrainz database",
		Author:      "Viewra Team",
		Type:        plugins.PluginTypeMetadataScraper,
		Status:      plugins.PluginStatusEnabled,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	plugin := &MusicBrainzEnricherPlugin{
		BasicPlugin: plugins.NewBasicPlugin(info),
		config: &MusicBrainzConfig{
			Enabled:            true,
			APIRateLimit:       0.8,
			UserAgent:          "Viewra/1.0.0 (https://github.com/viewra/viewra)",
			EnableArtwork:      true,
			ArtworkMaxSize:     1200,
			ArtworkQuality:     "front",
			MatchThreshold:     0.85,
			AutoEnrich:         false,
			OverwriteExisting:  false,
			CacheDurationHours: 168, // 1 week
		},
	}

	plugin.client = &MusicBrainzClient{
		baseURL:   "https://musicbrainz.org/ws/2",
		userAgent: plugin.config.UserAgent,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	plugin.rateLimiter = &RateLimiter{
		interval: time.Duration(1.0/plugin.config.APIRateLimit) * time.Second,
	}

	return plugin
}

// Initialize implements Plugin.Initialize
func (p *MusicBrainzEnricherPlugin) Initialize(ctx *plugins.PluginContext) error {
	if err := p.BasicPlugin.Initialize(ctx); err != nil {
		return err
	}

	// Load configuration
	if err := p.loadConfig(); err != nil {
		ctx.Logger.Warn("Failed to load config, using defaults", "error", err)
	}

	ctx.Logger.Info("MusicBrainz Enricher plugin initialized")
	return nil
}

// Start implements Plugin.Start
func (p *MusicBrainzEnricherPlugin) Start(ctx context.Context) error {
	if err := p.BasicPlugin.Start(ctx); err != nil {
		return err
	}

	// Register hooks for automatic enrichment
	if p.config.AutoEnrich {
		// Note: Hook registration would need to be implemented through the plugin manager
		// p.ctx.Hooks.Register("media_file_scanned", p.handleMediaFileScanned)
	}

	// Access context through BasicPlugin methods
	// p.ctx.Logger.Info("MusicBrainz Enricher plugin started")
	return nil
}

// CanHandle implements MetadataScraperPlugin.CanHandle
func (p *MusicBrainzEnricherPlugin) CanHandle(filePath string, mimeType string) bool {
	if !p.config.Enabled {
		return false
	}

	// Handle audio files
	ext := strings.ToLower(filepath.Ext(filePath))
	audioExts := []string{".mp3", ".flac", ".m4a", ".aac", ".ogg", ".wav", ".wma"}
	
	for _, audioExt := range audioExts {
		if ext == audioExt {
			return true
		}
	}

	return strings.HasPrefix(mimeType, "audio/")
}

// ExtractMetadata implements MetadataScraperPlugin.ExtractMetadata
func (p *MusicBrainzEnricherPlugin) ExtractMetadata(ctx context.Context, filePath string) (map[string]interface{}, error) {
	if !p.config.Enabled {
		return nil, fmt.Errorf("MusicBrainz enricher is disabled")
	}

	// Get existing metadata from database
	mediaFile, musicMetadata, err := p.getExistingMetadata(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing metadata: %w", err)
	}

	if musicMetadata == nil {
		return nil, fmt.Errorf("no existing music metadata found for file")
	}

	// Check if we should skip enrichment
	if !p.config.OverwriteExisting && p.hasRichMetadata(musicMetadata) {
		// p.ctx.Logger.Debug("Skipping enrichment - file already has rich metadata", "file", filePath)
		return nil, nil
	}

	// Search MusicBrainz
	enrichedData, err := p.searchAndEnrich(ctx, musicMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to enrich metadata: %w", err)
	}

	// Download artwork if enabled
	if p.config.EnableArtwork && enrichedData["release_id"] != nil {
		if artworkPath, err := p.downloadArtwork(ctx, enrichedData["release_id"].(string), mediaFile.ID); err == nil {
			enrichedData["artwork_path"] = artworkPath
		}
	}

	// p.ctx.Logger.Info("Successfully enriched metadata", "file", filePath)
	return enrichedData, nil
}

// SupportedTypes implements MetadataScraperPlugin.SupportedTypes
func (p *MusicBrainzEnricherPlugin) SupportedTypes() []string {
	return []string{
		"audio/mpeg", "audio/flac", "audio/mp4", "audio/aac",
		"audio/ogg", "audio/wav", "audio/x-ms-wma",
		".mp3", ".flac", ".m4a", ".aac", ".ogg", ".wav", ".wma",
	}
}

// RegisterRoutes implements AdminPagePlugin.RegisterRoutes
func (p *MusicBrainzEnricherPlugin) RegisterRoutes(router *gin.RouterGroup) error {
	pluginGroup := router.Group("/plugins/" + p.Info().ID)
	{
		// Status endpoint
		pluginGroup.GET("/status", p.handleStatus)
		
		// Configuration endpoints
		pluginGroup.GET("/config", p.handleGetConfig)
		pluginGroup.POST("/config", p.handleSetConfig)
		
		// Enrichment endpoints
		pluginGroup.POST("/enrich/:mediaFileId", p.handleEnrichFile)
		pluginGroup.POST("/enrich-batch", p.handleEnrichBatch)
		
		// Search endpoint
		pluginGroup.GET("/search", p.handleSearch)
		
		// Statistics endpoint
		pluginGroup.GET("/stats", p.handleStats)
	}

	// p.ctx.Logger.Info("MusicBrainz Enricher routes registered")
	return nil
}

// GetAdminPages implements AdminPagePlugin.GetAdminPages
func (p *MusicBrainzEnricherPlugin) GetAdminPages() []plugins.AdminPageConfig {
	return []plugins.AdminPageConfig{
		{
			ID:       "musicbrainz_dashboard",
			Title:    "MusicBrainz Enricher",
			Path:     "/admin/plugins/musicbrainz",
			Icon:     "music-note",
			Category: "Metadata",
			URL:      "/plugins/musicbrainz_enricher/dashboard.html",
			Type:     "iframe",
		},
		{
			ID:       "musicbrainz_settings",
			Title:    "MusicBrainz Settings",
			Path:     "/admin/plugins/musicbrainz-settings",
			Icon:     "cog",
			Category: "Metadata",
			URL:      "/plugins/musicbrainz_enricher/settings.html",
			Type:     "iframe",
		},
	}
}

// Helper methods

func (p *MusicBrainzEnricherPlugin) loadConfig() error {
	// Load configuration from plugin context
	// Note: This would need to be implemented through the plugin manager
	// For now, use default configuration
	return nil
}

func (p *MusicBrainzEnricherPlugin) getExistingMetadata(filePath string) (*database.MediaFile, *database.MusicMetadata, error) {
	// This would query the database for existing metadata
	// Implementation depends on database access patterns
	return nil, nil, fmt.Errorf("not implemented")
}

func (p *MusicBrainzEnricherPlugin) hasRichMetadata(metadata *database.MusicMetadata) bool {
	// Check if metadata already has rich information
	return metadata.MusicBrainzRecordingID != "" || 
		   metadata.MusicBrainzReleaseID != "" ||
		   metadata.MusicBrainzArtistID != ""
}

func (p *MusicBrainzEnricherPlugin) searchAndEnrich(ctx context.Context, metadata *database.MusicMetadata) (map[string]interface{}, error) {
	// Rate limit
	p.rateLimiter.Wait()

	// Build search query
	query := p.buildSearchQuery(metadata)
	
	// Search MusicBrainz
	recordings, err := p.client.SearchRecordings(ctx, query)
	if err != nil {
		return nil, err
	}

	if len(recordings) == 0 {
		return nil, fmt.Errorf("no matches found")
	}

	// Find best match
	bestMatch := p.findBestMatch(recordings, metadata)
	if bestMatch == nil {
		return nil, fmt.Errorf("no suitable match found")
	}

	// Convert to enriched data
	return p.convertToEnrichedData(bestMatch), nil
}

func (p *MusicBrainzEnricherPlugin) buildSearchQuery(metadata *database.MusicMetadata) string {
	var parts []string
	
	if metadata.Title != "" {
		parts = append(parts, fmt.Sprintf("recording:\"%s\"", metadata.Title))
	}
	if metadata.Artist != "" {
		parts = append(parts, fmt.Sprintf("artist:\"%s\"", metadata.Artist))
	}
	if metadata.Album != "" {
		parts = append(parts, fmt.Sprintf("release:\"%s\"", metadata.Album))
	}
	
	return strings.Join(parts, " AND ")
}

func (p *MusicBrainzEnricherPlugin) findBestMatch(recordings []MusicBrainzRecording, metadata *database.MusicMetadata) *MusicBrainzRecording {
	var bestMatch *MusicBrainzRecording
	var bestScore float64

	for _, recording := range recordings {
		score := p.calculateMatchScore(&recording, metadata)
		if score > bestScore && score >= p.config.MatchThreshold {
			bestScore = score
			bestMatch = &recording
		}
	}

	return bestMatch
}

func (p *MusicBrainzEnricherPlugin) calculateMatchScore(recording *MusicBrainzRecording, metadata *database.MusicMetadata) float64 {
	// Implement fuzzy string matching algorithm
	// This is a simplified version
	score := 0.0
	
	if strings.EqualFold(recording.Title, metadata.Title) {
		score += 0.4
	}
	
	if len(recording.Artists) > 0 && strings.EqualFold(recording.Artists[0].Name, metadata.Artist) {
		score += 0.3
	}
	
	if len(recording.Releases) > 0 && strings.EqualFold(recording.Releases[0].Title, metadata.Album) {
		score += 0.3
	}
	
	return score
}

func (p *MusicBrainzEnricherPlugin) convertToEnrichedData(recording *MusicBrainzRecording) map[string]interface{} {
	data := map[string]interface{}{
		"musicbrainz_recording_id": recording.ID,
		"enriched_at":              time.Now(),
		"enriched_by":              p.Info().ID,
	}

	if len(recording.Artists) > 0 {
		data["musicbrainz_artist_id"] = recording.Artists[0].Artist.ID
		data["enriched_artist"] = recording.Artists[0].Name
	}

	if len(recording.Releases) > 0 {
		release := recording.Releases[0]
		data["musicbrainz_release_id"] = release.ID
		data["enriched_album"] = release.Title
		
		if release.Date != "" {
			data["enriched_year"] = release.Date
		}
		
		if release.ReleaseGroup.PrimaryType != "" {
			data["enriched_album_type"] = release.ReleaseGroup.PrimaryType
		}
	}

	return data
}

func (p *MusicBrainzEnricherPlugin) downloadArtwork(ctx context.Context, releaseID string, mediaFileID uint) (string, error) {
	// Rate limit
	p.rateLimiter.Wait()

	// Get artwork from Cover Art Archive
	artworkURL := fmt.Sprintf("https://coverartarchive.org/release/%s", releaseID)
	
	resp, err := p.client.httpClient.Get(artworkURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("artwork not found: %d", resp.StatusCode)
	}

	var coverArt CoverArtArchiveResponse
	if err := json.NewDecoder(resp.Body).Decode(&coverArt); err != nil {
		return "", err
	}

	// Find front cover
	var imageURL string
	for _, image := range coverArt.Images {
		if image.Front {
			imageURL = image.Image
			break
		}
	}

	if imageURL == "" && len(coverArt.Images) > 0 {
		imageURL = coverArt.Images[0].Image
	}

	if imageURL == "" {
		return "", fmt.Errorf("no suitable artwork found")
	}

	// Download and save artwork
	return p.saveArtwork(ctx, imageURL, mediaFileID)
}

func (p *MusicBrainzEnricherPlugin) saveArtwork(ctx context.Context, imageURL string, mediaFileID uint) (string, error) {
	// Download image
	resp, err := p.client.httpClient.Get(imageURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Generate filename
	filename := fmt.Sprintf("musicbrainz_%d_%d.jpg", mediaFileID, time.Now().Unix())
	artworkPath := filepath.Join("artwork", filename)

	// Save using plugin file system access
	imageData := make([]byte, resp.ContentLength)
	if _, err := resp.Body.Read(imageData); err != nil {
		return "", err
	}

	// if err := p.ctx.FileSystem.WriteFile(artworkPath, imageData); err != nil {
	//	return "", err
	// }
	// TODO: Implement file system access through plugin manager

	return artworkPath, nil
}

func (p *MusicBrainzEnricherPlugin) handleMediaFileScanned(data interface{}) interface{} {
	// Handle automatic enrichment when new files are scanned
	if mediaFile, ok := data.(*database.MediaFile); ok {
		go func() {
			ctx := context.Background()
			if _, err := p.ExtractMetadata(ctx, mediaFile.Path); err != nil {
				// p.ctx.Logger.Error("Failed to auto-enrich file", "file", mediaFile.Path, "error", err)
				// TODO: Implement logging through plugin manager
			}
		}()
	}
	return data
}

// HTTP handlers

func (p *MusicBrainzEnricherPlugin) handleStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"plugin":  p.Info().ID,
		"status":  "running",
		"health":  p.Health() == nil,
		"config":  p.config,
		"version": p.Info().Version,
	})
}

func (p *MusicBrainzEnricherPlugin) handleGetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"config": p.config,
	})
}

func (p *MusicBrainzEnricherPlugin) handleSetConfig(c *gin.Context) {
	var newConfig MusicBrainzConfig
	if err := c.ShouldBindJSON(&newConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	p.config = &newConfig
	
	// Save configuration
	// TODO: Implement config saving through plugin manager
	// if err := p.ctx.Config.Set("config", newConfig); err != nil {
	//	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save config"})
	//	return
	// }

	c.JSON(http.StatusOK, gin.H{
		"message": "Configuration updated successfully",
		"config":  p.config,
	})
}

func (p *MusicBrainzEnricherPlugin) handleEnrichFile(c *gin.Context) {
	mediaFileIDStr := c.Param("mediaFileId")
	mediaFileID, err := strconv.ParseUint(mediaFileIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid media file ID"})
		return
	}

	// Get file path from database
	// This would need to be implemented based on database access
	filePath := "" // TODO: Get from database

	enrichedData, err := p.ExtractMetadata(c.Request.Context(), filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "File enriched successfully",
		"media_file_id":  uint(mediaFileID),
		"enriched_data":  enrichedData,
	})
}

func (p *MusicBrainzEnricherPlugin) handleEnrichBatch(c *gin.Context) {
	var request struct {
		MediaFileIDs []uint `json:"media_file_ids"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Process batch enrichment
	results := make(map[uint]interface{})
	for _, mediaFileID := range request.MediaFileIDs {
		// TODO: Implement batch processing
		results[mediaFileID] = "pending"
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Batch enrichment started",
		"results": results,
	})
}

func (p *MusicBrainzEnricherPlugin) handleSearch(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter 'q' is required"})
		return
	}

	recordings, err := p.client.SearchRecordings(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"query":     query,
		"results":   recordings,
		"count":     len(recordings),
	})
}

func (p *MusicBrainzEnricherPlugin) handleStats(c *gin.Context) {
	// TODO: Implement statistics gathering
	c.JSON(http.StatusOK, gin.H{
		"enriched_files": 0,
		"cached_entries": 0,
		"api_requests":   0,
		"last_enriched":  nil,
	})
}

// MusicBrainzClient methods

func (c *MusicBrainzClient) SearchRecordings(ctx context.Context, query string) ([]MusicBrainzRecording, error) {
	encodedQuery := url.QueryEscape(query)
	searchURL := fmt.Sprintf("%s/recording?query=%s&fmt=json&inc=artists+releases+release-groups", c.baseURL, encodedQuery)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MusicBrainz API error: %d", resp.StatusCode)
	}

	var searchResponse MusicBrainzSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResponse); err != nil {
		return nil, err
	}

	return searchResponse.Recordings, nil
}

// RateLimiter methods

func (r *RateLimiter) Wait() {
	now := time.Now()
	if elapsed := now.Sub(r.lastRequest); elapsed < r.interval {
		time.Sleep(r.interval - elapsed)
	}
	r.lastRequest = time.Now()
}

// Plugin entry point
func main() {
	plugin := NewMusicBrainzEnricherPlugin()
	
	// This would be the plugin execution logic
	// For now, just log that the plugin was loaded
	fmt.Printf("MusicBrainz Enricher Plugin v%s loaded\n", plugin.Info().Version)
} 