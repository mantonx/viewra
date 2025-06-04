package main

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/pkg/plugins"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// MusicBrainzEnricher implements the plugin interfaces
type MusicBrainzEnricher struct {
	logger      hclog.Logger
	config      *Config
	db          *gorm.DB
	dbURL       string
	basePath    string
	pluginID    string  // Add pluginID field to store the plugin ID from context
	lastAPICall *time.Time
	
	// Host service connections (using unified SDK client)
	unifiedClient       *plugins.UnifiedServiceClient
	hostServiceAddr     string
}

// Config represents the plugin configuration
type Config struct {
	Enabled             bool    `json:"enabled" default:"true"`
	APIRateLimit        float64 `json:"api_rate_limit" default:"0.5"`
	UserAgent           string  `json:"user_agent" default:"Viewra/2.0"`
	EnableArtwork       bool    `json:"enable_artwork" default:"true"`
	ArtworkMaxSize      int     `json:"artwork_max_size" default:"1200"`
	ArtworkQuality      string  `json:"artwork_quality" default:"front"`
	
	// Cover Art Archive settings
	DownloadFrontCover  bool    `json:"download_front_cover" default:"true"`
	DownloadBackCover   bool    `json:"download_back_cover" default:"false"`
	DownloadBooklet     bool    `json:"download_booklet" default:"false"`
	DownloadMedium      bool    `json:"download_medium" default:"false"`
	DownloadTray        bool    `json:"download_tray" default:"false"`
	DownloadObi         bool    `json:"download_obi" default:"false"`
	DownloadSpine       bool    `json:"download_spine" default:"false"`
	DownloadLiner       bool    `json:"download_liner" default:"false"`
	DownloadSticker     bool    `json:"download_sticker" default:"false"`
	DownloadPoster      bool    `json:"download_poster" default:"false"`
	
	MaxAssetSize        int64   `json:"max_asset_size" default:"10485760"` // 10MB
	AssetTimeout        int     `json:"asset_timeout_sec" default:"60"`
	SkipExistingAssets  bool    `json:"skip_existing_assets" default:"true"`
	RetryFailedDownloads bool   `json:"retry_failed_downloads" default:"true"`
	MaxRetries          int     `json:"max_retries" default:"5"`
	
	// New retry configuration
	InitialRetryDelay   int     `json:"initial_retry_delay_sec" default:"2"`   // Initial delay before first retry
	MaxRetryDelay       int     `json:"max_retry_delay_sec" default:"30"`      // Maximum delay between retries
	BackoffMultiplier   float64 `json:"backoff_multiplier" default:"2.0"`     // Exponential backoff multiplier
	
	// New rate limiting settings
	APIRequestDelay     int     `json:"api_request_delay_ms" default:"1250"`   // 1.25 seconds between requests
	BurstLimit          int     `json:"burst_limit" default:"1"`              // Allow 1 burst request
	
	MatchThreshold      float64 `json:"match_threshold" default:"0.85"`
	AutoEnrich          bool    `json:"auto_enrich" default:"true"`
	OverwriteExisting   bool    `json:"overwrite_existing" default:"false"`
	CacheDurationHours  int     `json:"cache_duration_hours" default:"168"`
}

// Database models for plugin data
type MusicBrainzCache struct {
	ID        uint32    `gorm:"primaryKey"`
	CacheKey  string    `gorm:"uniqueIndex;not null"`
	Data      string    `gorm:"type:text"`
	ExpiresAt time.Time `gorm:"index"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// MusicBrainz API types
type MusicBrainzSearchResponse struct {
	Created    string                `json:"created"`
	Count      int                   `json:"count"`
	Offset     int                   `json:"offset"`
	Recordings []MusicBrainzRecording `json:"recordings"`
}

type MusicBrainzRecording struct {
	ID           string                     `json:"id"`
	Score        float64                    `json:"score"`
	Title        string                     `json:"title"`
	Length       int                        `json:"length"`
	Disambiguation string                   `json:"disambiguation"`
	ArtistCredit []MusicBrainzArtistCredit  `json:"artist-credit"`
	Releases     []MusicBrainzRelease       `json:"releases"`
	Tags         []MusicBrainzTag           `json:"tags"`
	Genres       []MusicBrainzGenre         `json:"genres"`
}

type MusicBrainzArtistCredit struct {
	Name   string           `json:"name"`
	Artist MusicBrainzArtist `json:"artist"`
}

type MusicBrainzArtist struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	SortName       string `json:"sort-name"`
	Disambiguation string `json:"disambiguation"`
}

type MusicBrainzRelease struct {
	ID           string                    `json:"id"`
	Title        string                    `json:"title"`
	StatusID     string                    `json:"status-id"`
	Status       string                    `json:"status"`
	Date         string                    `json:"date"`
	Country      string                    `json:"country"`
	ReleaseGroup MusicBrainzReleaseGroup   `json:"release-group"`
}

type MusicBrainzReleaseGroup struct {
	ID             string `json:"id"`
	TypeID         string `json:"type-id"`
	PrimaryTypeID  string `json:"primary-type-id"`
	Title          string `json:"title"`
	PrimaryType    string `json:"primary-type"`
	Disambiguation string `json:"disambiguation"`
}

type MusicBrainzTag struct {
	Count int    `json:"count"`
	Name  string `json:"name"`
}

type MusicBrainzGenre struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Cover Art Archive artwork downloading
type CoverArtType struct {
	Name    string
	Subtype string
	Enabled func(*Config) bool
}

var coverArtTypes = []CoverArtType{
	{"front", "album_front", func(c *Config) bool { return c.DownloadFrontCover }},
	{"back", "album_back", func(c *Config) bool { return c.DownloadBackCover }},
	{"booklet", "album_booklet", func(c *Config) bool { return c.DownloadBooklet }},
	{"medium", "album_medium", func(c *Config) bool { return c.DownloadMedium }},
	{"tray", "album_tray", func(c *Config) bool { return c.DownloadTray }},
	{"obi", "album_obi", func(c *Config) bool { return c.DownloadObi }},
	{"spine", "album_spine", func(c *Config) bool { return c.DownloadSpine }},
	{"liner", "album_liner", func(c *Config) bool { return c.DownloadLiner }},
	{"sticker", "album_sticker", func(c *Config) bool { return c.DownloadSticker }},
	{"poster", "album_poster", func(c *Config) bool { return c.DownloadPoster }},
}

// validateConfig validates and adjusts configuration settings
func (m *MusicBrainzEnricher) validateConfig() error {
	// Validate MaxAssetSize
	const minAssetSize = int64(100 * 1024)      // 100KB minimum
	const maxAssetSize = int64(50 * 1024 * 1024) // 50MB maximum
	
	if m.config.MaxAssetSize < minAssetSize {
		m.logger.Warn("MaxAssetSize too small, adjusting to minimum", 
			"configured", m.config.MaxAssetSize, 
			"minimum", minAssetSize)
		m.config.MaxAssetSize = minAssetSize
	}
	
	if m.config.MaxAssetSize > maxAssetSize {
		m.logger.Warn("MaxAssetSize too large, adjusting to maximum", 
			"configured", m.config.MaxAssetSize, 
			"maximum", maxAssetSize)
		m.config.MaxAssetSize = maxAssetSize
	}
	
	// Validate rate limiting
	if m.config.APIRequestDelay < 500 {
		m.logger.Warn("APIRequestDelay too short, adjusting to minimum", 
			"configured", m.config.APIRequestDelay, 
			"minimum", 500)
		m.config.APIRequestDelay = 500
	}
	
	// Validate timeouts
	if m.config.AssetTimeout < 10 {
		m.config.AssetTimeout = 10
	}
	if m.config.AssetTimeout > 300 {
		m.config.AssetTimeout = 300
	}
	
	// Log effective settings
	effectiveMaxSize := m.calculateMaxAssetSize()
	m.logger.Info("Configuration validated", 
		"max_asset_size_config", m.config.MaxAssetSize,
		"max_asset_size_effective", effectiveMaxSize,
		"api_request_delay_ms", m.config.APIRequestDelay,
		"asset_timeout_sec", m.config.AssetTimeout,
		"enable_artwork", m.config.EnableArtwork)
	
	return nil
}

// Core plugin interface implementation
func (m *MusicBrainzEnricher) Initialize(ctx *plugins.PluginContext) error {
	m.logger = hclog.New(&hclog.LoggerOptions{
		Name:  ctx.PluginID, // Use dynamic plugin ID instead of hard-coded name
		Level: hclog.LevelFromString(ctx.LogLevel),
	})

	m.basePath = ctx.BasePath
	m.dbURL = ctx.DatabaseURL
	m.hostServiceAddr = ctx.HostServiceAddr
	m.pluginID = ctx.PluginID  // Store pluginID from context

	// Initialize configuration with defaults
	m.config = &Config{
		Enabled:             true,
		APIRateLimit:        0.5,
		UserAgent:           "Viewra/2.0",
		EnableArtwork:       true,
		ArtworkMaxSize:      1200,
		ArtworkQuality:      "front",
		
		// Cover Art Archive settings
		DownloadFrontCover:  true,
		DownloadBackCover:   false,
		DownloadBooklet:     false,
		DownloadMedium:      false,
		DownloadTray:        false,
		DownloadObi:         false,
		DownloadSpine:       false,
		DownloadLiner:       false,
		DownloadSticker:     false,
		DownloadPoster:      false,
		
		MaxAssetSize:        10485760, // 10MB
		AssetTimeout:        60,
		SkipExistingAssets:  true,
		RetryFailedDownloads: true,
		MaxRetries:          5,
		
		// New retry configuration
		InitialRetryDelay:   2,
		MaxRetryDelay:       30,
		BackoffMultiplier:   2.0,
		
		// New rate limiting settings
		APIRequestDelay:     1250,
		BurstLimit:          1,
		
		MatchThreshold:      0.85,
		AutoEnrich:          true,
		OverwriteExisting:   false,
		CacheDurationHours:  168,
	}

	m.logger.Info("Initializing MusicBrainz enricher plugin", 
		"base_path", m.basePath,
		"database_url", m.dbURL,
		"host_service_addr", m.hostServiceAddr,
		"api_rate_limit", m.config.APIRateLimit,
		"match_threshold", m.config.MatchThreshold)

	// Validate and adjust configuration
	if err := m.validateConfig(); err != nil {
		m.logger.Error("Configuration validation failed", "error", err)
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Initialize database connection
	if err := m.initDatabase(); err != nil {
		m.logger.Error("Failed to initialize database", "error", err)
		return fmt.Errorf("database initialization failed: %w", err)
	}

	// Initialize host asset service connection if address provided
	if m.hostServiceAddr != "" {
		// Initialize Unified Service client
		unifiedClient, err := plugins.NewUnifiedServiceClient(m.hostServiceAddr)
		if err != nil {
			m.logger.Error("Failed to connect to host service", "error", err, "addr", m.hostServiceAddr)
			return fmt.Errorf("failed to connect to host service: %w", err)
		}
		m.unifiedClient = unifiedClient
		
		m.logger.Info("Connected to host services", "addr", m.hostServiceAddr, "services", "unified")
	} else {
		m.logger.Warn("No host service address provided - asset saving and enrichment will be disabled")
	}

	m.logger.Info("MusicBrainz enricher plugin initialized successfully")
	return nil
}

func (m *MusicBrainzEnricher) Start() error {
	m.logger.Info("MusicBrainz enricher started")
	return nil
}

func (m *MusicBrainzEnricher) Stop() error {
	m.logger.Info("MusicBrainz enricher stopped")
	
	// Close unified service connection
	if m.unifiedClient != nil {
		if err := m.unifiedClient.Close(); err != nil {
			m.logger.Warn("Failed to close unified service connection", "error", err)
		} else {
			m.logger.Debug("Closed unified service connection")
		}
	}
	
	// Close database connection
	if m.db != nil {
		if sqlDB, err := m.db.DB(); err == nil {
			sqlDB.Close()
		}
	}
	return nil
}

func (m *MusicBrainzEnricher) Info() (*plugins.PluginInfo, error) {
	return &plugins.PluginInfo{
		ID:          m.pluginID,
		Name:        "MusicBrainz Metadata Enricher",
		Version:     "1.0.0",
		Description: "Enriches music metadata using the MusicBrainz database",
		Author:      "Viewra Team",
		Type:        plugins.PluginTypeMetadataScraper,
	}, nil
}

func (m *MusicBrainzEnricher) Health() error {
	// Check database connection
	if m.db == nil {
		return fmt.Errorf("database connection not available")
	}
	
	// Check MusicBrainz API connectivity (quick test)
	resp, err := http.Get("https://musicbrainz.org/ws/2/artist/5b11f4ce-a62d-471e-81fc-a69a8278c7da?fmt=json")
	if err != nil {
		return fmt.Errorf("MusicBrainz API not reachable: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return fmt.Errorf("MusicBrainz API returned status %d", resp.StatusCode)
	}
	
	return nil
}

// Service interface implementations
func (m *MusicBrainzEnricher) MetadataScraperService() plugins.MetadataScraperService {
	return m
}

func (m *MusicBrainzEnricher) ScannerHookService() plugins.ScannerHookService {
	return m
}

func (m *MusicBrainzEnricher) AssetService() plugins.AssetService {
	return nil // Not implemented
}

func (m *MusicBrainzEnricher) DatabaseService() plugins.DatabaseService {
	return m
}

func (m *MusicBrainzEnricher) AdminPageService() plugins.AdminPageService {
	return nil // Not implemented
}

func (m *MusicBrainzEnricher) APIRegistrationService() plugins.APIRegistrationService {
	return m
}

func (m *MusicBrainzEnricher) SearchService() plugins.SearchService {
	return m
}

// MetadataScraperService implementation
func (m *MusicBrainzEnricher) CanHandle(filePath, mimeType string) bool {
	if !m.config.Enabled {
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

func (m *MusicBrainzEnricher) ExtractMetadata(filePath string) (map[string]string, error) {
	if !m.CanHandle(filePath, "") {
		return nil, fmt.Errorf("file type not supported: %s", filePath)
	}
	
	// This plugin enriches existing metadata rather than extracting raw metadata
	return map[string]string{
		"plugin":     m.pluginID,
		"file_path":  filePath,
		"supported":  "true",
		"enrichment": "available",
	}, nil
}

func (m *MusicBrainzEnricher) GetSupportedTypes() []string {
	return []string{
		"audio/mpeg",
		"audio/flac",
		"audio/ogg",
		"audio/wav",
		"audio/aac",
		"audio/m4a",
		"audio/wma",
	}
}

// ScannerHookService implementation
func (m *MusicBrainzEnricher) OnMediaFileScanned(mediaFileID string, filePath string, metadata map[string]string) error {
	if !m.config.Enabled || !m.config.AutoEnrich {
		m.logger.Debug("MusicBrainz enrichment disabled", "enabled", m.config.Enabled, "auto_enrich", m.config.AutoEnrich)
		return nil
	}

	m.logger.Info("MusicBrainz OnMediaFileScanned ENTRY", "media_file_id", mediaFileID, "file_path", filePath)
	m.logger.Info("MusicBrainz received metadata", "metadata", metadata, "metadata_count", len(metadata))

	// Log each metadata field for debugging
	for key, value := range metadata {
		m.logger.Debug("MusicBrainz metadata field", "key", key, "value", value)
	}
	
	// IMPORTANT: Check if this is an audio track before processing
	// Skip image files and other non-audio files
	mediaType := metadata["media_type"]
	if mediaType != "track" {
		m.logger.Debug("MusicBrainz: Skipping non-track file", 
			"media_file_id", mediaFileID, 
			"file_path", filePath,
			"media_type", mediaType)
		return nil
	}
	
	// Additional check: Skip image files based on file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	imageExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".bmp": true,
		".tiff": true, ".tif": true, ".webp": true, ".svg": true,
	}
	if imageExts[ext] {
		m.logger.Debug("MusicBrainz: Skipping image file", 
			"media_file_id", mediaFileID, 
			"file_path", filePath,
			"extension", ext)
		return nil
	}

	// Extract metadata for search
	title := metadata["title"]
	artist := metadata["artist"]
	album := metadata["album"]

	m.logger.Info("MusicBrainz extracted fields", "title", title, "artist", artist, "album", album)

	if title == "" || artist == "" {
		m.logger.Warn("MusicBrainz: Insufficient metadata for enrichment", 
			"media_file_id", mediaFileID, 
			"title", title, 
			"artist", artist,
			"available_fields", getMapKeys(metadata))
		return nil
	}

	m.logger.Info("MusicBrainz: Starting API search", "title", title, "artist", artist, "album", album)

	// Search for recording using MusicBrainz API
	recording, err := m.searchRecording(title, artist, album)
	if err != nil {
		m.logger.Error("MusicBrainz API search failed", "error", err, "media_file_id", mediaFileID, "title", title, "artist", artist)
		return nil // Don't fail the scan for enrichment failures
	}

	if recording == nil {
		m.logger.Warn("No MusicBrainz match found", 
			"media_file_id", mediaFileID, 
			"title", title, 
			"artist", artist,
			"album", album)
		return nil
	}

	m.logger.Info("Found MusicBrainz match", 
		"media_file_id", mediaFileID, 
		"recording_id", recording.ID, 
		"score", recording.Score, 
		"title", recording.Title)

	// Save enrichment to centralized system
	var artistName, albumTitle string
	var releaseYear int
	var genre string

	if len(recording.ArtistCredit) > 0 {
		artistName = recording.ArtistCredit[0].Artist.Name
	}

	if len(recording.Releases) > 0 {
		albumTitle = recording.Releases[0].Title
		if recording.Releases[0].Date != "" {
			if year, err := strconv.Atoi(recording.Releases[0].Date[:4]); err == nil {
				releaseYear = year
			}
		}
	}

	if len(recording.Genres) > 0 {
		genre = recording.Genres[0].Name
	} else if len(recording.Tags) > 0 {
		genre = recording.Tags[0].Name
	}

	m.logger.Info("MusicBrainz: Saving enrichment data", 
		"media_file_id", mediaFileID,
		"artist_name", artistName,
		"album_title", albumTitle,
		"release_year", releaseYear,
		"genre", genre)

	// Save to centralized system (modern UUID-based format)
	if err := m.saveToCentralizedSystem(mediaFileID, recording, artistName, albumTitle, releaseYear, genre); err != nil {
		m.logger.Error("Failed to save to centralized system", "error", err, "media_file_id", mediaFileID)
		// Don't fail the scan for enrichment save failures
	} else {
		m.logger.Info("Successfully saved MusicBrainz enrichment", "media_file_id", mediaFileID)
	}

	// Download artwork if enabled and we have a release
	if m.config.EnableArtwork && len(recording.Releases) > 0 {
		releaseID := recording.Releases[0].ID
		m.logger.Info("MusicBrainz: Starting artwork download", "release_id", releaseID, "media_file_id", mediaFileID)
		if err := m.downloadAllArtwork(context.Background(), releaseID, mediaFileID); err != nil {
			m.logger.Warn("Artwork download failed", "error", err, "release_id", releaseID, "media_file_id", mediaFileID)
			// Don't fail for artwork download issues
		} else {
			m.logger.Info("Successfully downloaded MusicBrainz artwork", "release_id", releaseID, "media_file_id", mediaFileID)
		}
	}

	return nil
}

// Helper function to get map keys for debugging
func getMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (m *MusicBrainzEnricher) OnScanStarted(scanJobID, libraryID uint32, libraryPath string) error {
	m.logger.Info("Scan started", "job_id", scanJobID, "library_id", libraryID, "path", libraryPath)
	return nil
}

func (m *MusicBrainzEnricher) OnScanCompleted(scanJobID, libraryID uint32, stats map[string]string) error {
	m.logger.Info("Scan completed", "job_id", scanJobID, "library_id", libraryID, "stats", stats)
	return nil
}

// SearchService implementation
func (m *MusicBrainzEnricher) Search(ctx context.Context, query map[string]string, limit, offset uint32) ([]*plugins.SearchResult, uint32, bool, error) {
	title := query["title"]
	artist := query["artist"]
	album := query["album"]

	if title == "" || artist == "" {
		return nil, 0, false, fmt.Errorf("title and artist are required")
	}

	recordings, err := m.searchRecordings(title, artist, album, int(limit))
	if err != nil {
		return nil, 0, false, err
	}

	var results []*plugins.SearchResult
	for _, recording := range recordings {
		// Use MusicBrainz's native score (typically 0-100) 
		// Convert to 0-1 scale for consistency
		mbScore := recording.Score / 100.0
		
		// Use configurable threshold
		if mbScore < m.config.MatchThreshold {
			continue
		}
		
		// Get primary artist name
		artistName := ""
		if len(recording.ArtistCredit) > 0 {
			artistName = recording.ArtistCredit[0].Name
		}
		
		// Get primary release info
		releaseDate := ""
		if len(recording.Releases) > 0 {
			releaseDate = recording.Releases[0].Date
		}
		
		// Build comprehensive metadata
		metadata := map[string]string{
			"source":          "musicbrainz",
			"plugin":          m.pluginID,
			"recording_id":    recording.ID,
			"length":          fmt.Sprintf("%d", recording.Length),
			"disambiguation":  recording.Disambiguation,
		}
		
		// Add artist info if available
		if len(recording.ArtistCredit) > 0 {
			metadata["artist_id"] = recording.ArtistCredit[0].Artist.ID
			metadata["artist_sort_name"] = recording.ArtistCredit[0].Artist.SortName
		}
		
		// Add release info if available
		if len(recording.Releases) > 0 {
			metadata["release_id"] = recording.Releases[0].ID
			metadata["release_date"] = releaseDate
			metadata["release_status"] = recording.Releases[0].Status
			metadata["release_country"] = recording.Releases[0].Country
		}
		
		// Add tags/genres if available
		if len(recording.Tags) > 0 {
			var tags []string
			for _, tag := range recording.Tags {
				tags = append(tags, tag.Name)
			}
			metadata["tags"] = strings.Join(tags, ", ")
		}
		
		if len(recording.Genres) > 0 {
			var genres []string
			for _, genre := range recording.Genres {
				genres = append(genres, genre.Name)
			}
			metadata["genres"] = strings.Join(genres, ", ")
		}
		
		// Create SearchResult
		result := &plugins.SearchResult{
			ID:       recording.ID,
			Title:    recording.Title,
			Subtitle: artistName,
			URL:      fmt.Sprintf("https://musicbrainz.org/recording/%s", recording.ID),
			Metadata: metadata,
		}
		
		results = append(results, result)
	}
	
	// Calculate pagination info
	totalCount := uint32(len(results))
	hasMore := false // MusicBrainz doesn't provide total count easily, so we'll keep it simple
	
	// Apply offset if needed (simple implementation)
	if offset > 0 && offset < uint32(len(results)) {
		results = results[offset:]
	}
	
	m.logger.Info("MusicBrainz search completed", 
		"total_found", len(recordings), 
		"above_threshold", len(results), 
		"threshold", m.config.MatchThreshold)
	
	return results, totalCount, hasMore, nil
}

func (m *MusicBrainzEnricher) GetSearchCapabilities(ctx context.Context) ([]string, bool, uint32, error) {
	// Return capabilities
	supportedFields := []string{"title", "artist", "album"}
	supportsPagination := false
	maxResults := uint32(10)
	
	return supportedFields, supportsPagination, maxResults, nil
}

// DatabaseService implementation
func (m *MusicBrainzEnricher) GetModels() []string {
	return []string{
		"MusicBrainzCache",
	}
}

func (m *MusicBrainzEnricher) Migrate(connectionString string) error {
	return m.db.AutoMigrate(&MusicBrainzCache{})
}

func (m *MusicBrainzEnricher) Rollback(connectionString string) error {
	return m.db.Migrator().DropTable(&MusicBrainzCache{})
}

// APIRegistrationService implementation
func (m *MusicBrainzEnricher) GetRegisteredRoutes(ctx context.Context) ([]*plugins.APIRoute, error) {
	routes := []*plugins.APIRoute{
		{
			Path:        "/search",
			Method:      "GET",
			Description: "Search MusicBrainz for recordings",
			Public:      false,
		},
		{
			Path:        "/config",
			Method:      "GET",
			Description: "Get plugin configuration",
			Public:      false,
		},
	}
	
	return routes, nil
}

// Helper methods
func (m *MusicBrainzEnricher) initDatabase() error {
	if m.dbURL == "" {
		// Use local SQLite database if no URL provided
		dbPath := filepath.Join(m.basePath, "musicbrainz.db")
		m.dbURL = "sqlite://" + dbPath
	}

	// Parse database URL
	var dialector gorm.Dialector
	if strings.HasPrefix(m.dbURL, "sqlite://") {
		dbPath := strings.TrimPrefix(m.dbURL, "sqlite://")
		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			return fmt.Errorf("failed to create database directory: %w", err)
		}
		dialector = sqlite.Open(dbPath)
	} else {
		return fmt.Errorf("unsupported database URL: %s", m.dbURL)
	}

	// Open database connection
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	m.db = db

	// Auto-migrate tables
	if err := m.db.AutoMigrate(&MusicBrainzCache{}); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	m.logger.Info("Database initialized successfully", "url", m.dbURL)
	return nil
}

func (m *MusicBrainzEnricher) searchRecording(title, artist, album string) (*MusicBrainzRecording, error) {
	// Check cache first
	cacheKey := m.getCacheKey(title, artist, album)
	if cached := m.getCachedResult(cacheKey); cached != nil {
		m.logger.Debug("using cached MusicBrainz result", "title", title, "artist", artist)
		return cached, nil
	}

	// Rate limiting
	if m.lastAPICall != nil {
		elapsed := time.Since(*m.lastAPICall)
		// Use the new APIRequestDelay configuration for more conservative rate limiting
		minInterval := time.Duration(m.config.APIRequestDelay) * time.Millisecond
		if elapsed < minInterval {
			waitTime := minInterval - elapsed
			m.logger.Debug("Rate limiting: waiting before API request", "wait_time", waitTime)
			time.Sleep(waitTime)
		}
	}
	now := time.Now()
	m.lastAPICall = &now

	// Build query
	query := m.buildSearchQuery(title, artist, album)
	
	// Make API request
	apiURL := fmt.Sprintf("https://musicbrainz.org/ws/2/recording?query=%s&fmt=json&limit=5&inc=releases",
		url.QueryEscape(query))
	
	m.logger.Info("searching MusicBrainz", "query", query, "url", apiURL)
	
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("User-Agent", m.config.UserAgent)
	req.Header.Set("Accept", "application/json")
	
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}
	
	// Parse response
	var mbResponse MusicBrainzSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&mbResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	// Find best match
	bestMatch := m.findBestMatch(mbResponse.Recordings, title, artist, album)
	if bestMatch == nil {
		m.logger.Debug("no suitable match found", "title", title, "artist", artist)
		return nil, nil
	}
	
	// Cache result
	m.cacheResult(cacheKey, bestMatch)
	
	m.logger.Info("found MusicBrainz match", 
		"title", title, 
		"artist", artist, 
		"mbid", bestMatch.ID, 
		"score", bestMatch.Score)
	
	return bestMatch, nil
}

func (m *MusicBrainzEnricher) searchRecordings(title, artist, album string, limit int) ([]MusicBrainzRecording, error) {
	// Check cache first if database is available
	var cacheKey string
	if m.db != nil {
		cacheKey = m.getCacheKey(title, artist, album)
		if cached := m.getCachedResults(cacheKey); cached != nil && len(cached) > 0 {
			m.logger.Debug("using cached MusicBrainz results", "title", title, "artist", artist, "count", len(cached))
			if len(cached) > limit {
				return cached[:limit], nil
			}
			return cached, nil
		}
	}

	// Rate limiting
	if m.lastAPICall != nil {
		elapsed := time.Since(*m.lastAPICall)
		// Use the new APIRequestDelay configuration for more conservative rate limiting
		minInterval := time.Duration(m.config.APIRequestDelay) * time.Millisecond
		if elapsed < minInterval {
			waitTime := minInterval - elapsed
			m.logger.Debug("Rate limiting: waiting before API request", "wait_time", waitTime)
			time.Sleep(waitTime)
		}
	}
	now := time.Now()
	m.lastAPICall = &now

	// Build query
	query := m.buildSearchQuery(title, artist, album)
	
	// Make API request
	apiURL := fmt.Sprintf("https://musicbrainz.org/ws/2/recording?query=%s&fmt=json&limit=%d&inc=releases",
		url.QueryEscape(query), limit)
	
	m.logger.Info("searching MusicBrainz", "query", query, "url", apiURL, "limit", limit)
	
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("User-Agent", m.config.UserAgent)
	req.Header.Set("Accept", "application/json")
	
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}
	
	// Parse response
	var mbResponse MusicBrainzSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&mbResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	// Cache results if database is available
	if m.db != nil {
		m.cacheResults(cacheKey, mbResponse.Recordings)
	}
	
	return mbResponse.Recordings, nil
}

func (m *MusicBrainzEnricher) buildSearchQuery(title, artist, album string) string {
	var parts []string
	
	if title != "" {
		parts = append(parts, fmt.Sprintf("recording:\"%s\"", title))
	}
	if artist != "" {
		parts = append(parts, fmt.Sprintf("artist:\"%s\"", artist))
	}
	if album != "" {
		parts = append(parts, fmt.Sprintf("release:\"%s\"", album))
	}
	
	return strings.Join(parts, " AND ")
}

func (m *MusicBrainzEnricher) findBestMatch(recordings []MusicBrainzRecording, title, artist, album string) *MusicBrainzRecording {
	if len(recordings) == 0 {
		return nil
	}
	
	var bestMatch *MusicBrainzRecording
	
	for i := range recordings {
		recording := &recordings[i]
		score := m.calculateMatchScore(recording, title, artist, album)
		recording.Score = score
		
		if score >= m.config.MatchThreshold && (bestMatch == nil || score > bestMatch.Score) {
			bestMatch = recording
		}
	}
	
	return bestMatch
}

func (m *MusicBrainzEnricher) calculateMatchScore(recording *MusicBrainzRecording, title, artist, album string) float64 {
	var score float64
	
	// Title similarity (40% weight)
	titleScore := m.stringSimilarity(strings.ToLower(recording.Title), strings.ToLower(title))
	score += titleScore * 0.4
	
	// Artist similarity (40% weight)
	var artistScore float64
	if len(recording.ArtistCredit) > 0 {
		artistScore = m.stringSimilarity(strings.ToLower(recording.ArtistCredit[0].Name), strings.ToLower(artist))
	}
	score += artistScore * 0.4
	
	// Album similarity (20% weight)
	var albumScore float64
	if album != "" && len(recording.Releases) > 0 {
		albumScore = m.stringSimilarity(strings.ToLower(recording.Releases[0].Title), strings.ToLower(album))
	} else if album == "" {
		albumScore = 1.0 // No penalty if album not provided
	}
	score += albumScore * 0.2
	
	return score
}

func (m *MusicBrainzEnricher) stringSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}
	
	// Simple similarity based on common words
	words1 := strings.Fields(s1)
	words2 := strings.Fields(s2)
	
	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}
	
	commonWords := 0
	for _, w1 := range words1 {
		for _, w2 := range words2 {
			if w1 == w2 {
				commonWords++
				break
			}
		}
	}
	
	maxWords := len(words1)
	if len(words2) > maxWords {
		maxWords = len(words2)
	}
	
	return float64(commonWords) / float64(maxWords)
}

func (m *MusicBrainzEnricher) getCacheKey(title, artist, album string) string {
	data := fmt.Sprintf("%s|%s|%s", title, artist, album)
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("%x", hash)
}

func (m *MusicBrainzEnricher) getCachedResult(cacheKey string) *MusicBrainzRecording {
	if m.db == nil {
		return nil
	}
	
	var cache MusicBrainzCache
	if err := m.db.Where("cache_key = ? AND expires_at > ?", cacheKey, time.Now()).First(&cache).Error; err != nil {
		return nil
	}
	
	var recording MusicBrainzRecording
	if err := json.Unmarshal([]byte(cache.Data), &recording); err != nil {
		m.logger.Warn("failed to unmarshal cached recording", "error", err)
		return nil
	}
	
	return &recording
}

func (m *MusicBrainzEnricher) getCachedResults(cacheKey string) []MusicBrainzRecording {
	if m.db == nil {
		return nil
	}
	
	var cache MusicBrainzCache
	if err := m.db.Where("cache_key = ? AND expires_at > ?", cacheKey, time.Now()).First(&cache).Error; err != nil {
		return nil
	}
	
	var recordings []MusicBrainzRecording
	if err := json.Unmarshal([]byte(cache.Data), &recordings); err != nil {
		m.logger.Warn("failed to unmarshal cached recordings", "error", err)
		return nil
	}
	
	return recordings
}

func (m *MusicBrainzEnricher) cacheResult(cacheKey string, recording *MusicBrainzRecording) {
	if m.db == nil {
		return
	}
	
	data, err := json.Marshal(recording)
	if err != nil {
		m.logger.Warn("failed to marshal recording for cache", "error", err)
		return
	}
	
	cache := MusicBrainzCache{
		CacheKey:  cacheKey,
		Data:      string(data),
		ExpiresAt: time.Now().Add(time.Duration(m.config.CacheDurationHours) * time.Hour),
	}
	
	m.db.Save(&cache)
}

func (m *MusicBrainzEnricher) cacheResults(cacheKey string, recordings []MusicBrainzRecording) {
	if m.db == nil {
		return
	}
	
	data, err := json.Marshal(recordings)
	if err != nil {
		m.logger.Warn("failed to marshal recordings for cache", "error", err)
		return
	}
	
	cache := MusicBrainzCache{
		CacheKey:  cacheKey,
		Data:      string(data),
		ExpiresAt: time.Now().Add(time.Duration(m.config.CacheDurationHours) * time.Hour),
	}
	
	m.db.Save(&cache)
}

func (m *MusicBrainzEnricher) saveToCentralizedSystem(mediaFileID string, recording *MusicBrainzRecording, artistName, albumTitle string, releaseYear int, genre string) error {
	if m.unifiedClient == nil {
		m.logger.Warn("Unified service not available - cannot save enrichment data", "media_file_id", mediaFileID)
		return fmt.Errorf("unified service not available")
	}

	// Create enrichment fields map
	enrichments := make(map[string]string)
	
	// Core fields
	enrichments["recording_id"] = recording.ID
	enrichments["title"] = recording.Title
	if artistName != "" {
		enrichments["artist"] = artistName
	}
	if albumTitle != "" {
		enrichments["album"] = albumTitle
	}
	if genre != "" {
		enrichments["genre"] = genre
	}
	if releaseYear > 0 {
		enrichments["year"] = fmt.Sprintf("%d", releaseYear)
	}
	if recording.Length > 0 {
		enrichments["length"] = fmt.Sprintf("%d", recording.Length)
	}

	// Additional metadata
	matchMetadata := make(map[string]string)
	matchMetadata["source"] = "musicbrainz"
	matchMetadata["match_score"] = fmt.Sprintf("%.3f", recording.Score)
	
	// Add external IDs to metadata
	if len(recording.ArtistCredit) > 0 {
		matchMetadata["artist_id"] = recording.ArtistCredit[0].Artist.ID
		matchMetadata["artist_sort_name"] = recording.ArtistCredit[0].Artist.SortName
	}
	if len(recording.Releases) > 0 {
		matchMetadata["release_id"] = recording.Releases[0].ID
		matchMetadata["release_date"] = recording.Releases[0].Date
		matchMetadata["release_status"] = recording.Releases[0].Status
		matchMetadata["release_country"] = recording.Releases[0].Country
	}

	// Add tags/genres to metadata
	if len(recording.Tags) > 0 {
		var tags []string
		for _, tag := range recording.Tags {
			tags = append(tags, tag.Name)
		}
		matchMetadata["tags"] = strings.Join(tags, ", ")
	}
	if len(recording.Genres) > 0 {
		var genres []string
		for _, genre := range recording.Genres {
			genres = append(genres, genre.Name)
		}
		matchMetadata["genres"] = strings.Join(genres, ", ")
	}

	// Create RegisterEnrichment request
	request := &plugins.RegisterEnrichmentRequest{
		MediaFileID:     mediaFileID,
		SourceName:      "musicbrainz",
		Enrichments:     enrichments,
		ConfidenceScore: recording.Score / 100.0, // Convert from 0-100 to 0-1
		MatchMetadata:   matchMetadata,
	}

	m.logger.Info("Sending enrichment to centralized system via unified service", 
		"media_file_id", mediaFileID,
		"enrichments_count", len(enrichments),
		"confidence_score", request.ConfidenceScore,
		"metadata_count", len(matchMetadata))

	// Call the UnifiedService via unified SDK
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := m.unifiedClient.EnrichmentService().RegisterEnrichment(ctx, request)
	if err != nil {
		m.logger.Error("Failed to register enrichment via unified service", "error", err, "media_file_id", mediaFileID)
		return fmt.Errorf("failed to register enrichment: %w", err)
	}

	if !response.Success {
		m.logger.Error("Enrichment registration failed", "error", response.Message, "media_file_id", mediaFileID)
		return fmt.Errorf("enrichment registration failed: %s", response.Message)
	}

	m.logger.Info("Successfully registered enrichment via unified service", 
		"media_file_id", mediaFileID,
		"job_id", response.JobID,
		"source", "musicbrainz")

	return nil
}

// calculateMaxAssetSize returns the effective maximum asset size considering both config and gRPC limits
func (m *MusicBrainzEnricher) calculateMaxAssetSize() int64 {
	maxSize := m.config.MaxAssetSize
	grpcLimit := int64(15 * 1024 * 1024) // 15MB gRPC safe limit
	if maxSize > grpcLimit {
		maxSize = grpcLimit
	}
	return maxSize
}

// downloadAllArtwork downloads all enabled artwork types for a release
func (m *MusicBrainzEnricher) downloadAllArtwork(ctx context.Context, releaseID string, mediaFileID string) error {
	if releaseID == "" {
		return fmt.Errorf("no release ID available for artwork download")
	}

	var downloadErrors []string
	successCount := 0
	skippedCount := 0
	enabledTypes := 0

	// Count enabled types for better progress reporting
	for _, artType := range coverArtTypes {
		if artType.Enabled(m.config) {
			enabledTypes++
		}
	}

	m.logger.Info("Starting artwork download", 
		"release_id", releaseID, 
		"media_file_id", mediaFileID, 
		"enabled_types", enabledTypes)

	for i, artType := range coverArtTypes {
		if !artType.Enabled(m.config) {
			m.logger.Debug("Skipping artwork type (disabled)", "type", artType.Name)
			skippedCount++
			continue
		}

		m.logger.Debug("Downloading artwork type", 
			"type", artType.Name, 
			"progress", fmt.Sprintf("%d/%d", i+1-skippedCount, enabledTypes))

		if err := m.downloadArtworkType(ctx, releaseID, mediaFileID, artType); err != nil {
			if err.Error() == "no artwork available" {
				m.logger.Debug("No artwork available for type", "type", artType.Name)
			} else {
				downloadErrors = append(downloadErrors, fmt.Sprintf("%s: %v", artType.Name, err))
				m.logger.Warn("Failed to download artwork type", "type", artType.Name, "error", err)
			}
		} else {
			successCount++
			m.logger.Info("Successfully downloaded artwork", "type", artType.Name, "media_file_id", mediaFileID)
		}

		// Add small delay between artwork downloads to be respectful
		if i < len(coverArtTypes)-1 && artType.Enabled(m.config) {
			time.Sleep(500 * time.Millisecond)
		}
	}

	m.logger.Info("Artwork download completed", 
		"release_id", releaseID, 
		"media_file_id", mediaFileID, 
		"success_count", successCount, 
		"error_count", len(downloadErrors),
		"enabled_types", enabledTypes)

	// Return error only if all downloads failed and we had enabled types
	if len(downloadErrors) > 0 && successCount == 0 && enabledTypes > 0 {
		return fmt.Errorf("all artwork downloads failed: %s", strings.Join(downloadErrors, "; "))
	}

	return nil
}

// downloadArtworkType downloads a specific type of artwork
func (m *MusicBrainzEnricher) downloadArtworkType(ctx context.Context, releaseID string, mediaFileID string, artType CoverArtType) error {
	// Check if asset already exists
	if m.config.SkipExistingAssets {
		// TODO: Check if asset already exists via AssetService
		// For now, continue with download
	}

	// Construct Cover Art Archive URL
	downloadURL := fmt.Sprintf("https://coverartarchive.org/release/%s/%s", releaseID, artType.Name)
	
	m.logger.Debug("Downloading artwork", "type", artType.Name, "release_id", releaseID, "url", downloadURL)
	
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: time.Duration(m.config.AssetTimeout) * time.Second,
	}

	// Download with retry logic
	var imageData []byte
	var mimeType string
	var downloadErr error

	for attempt := 0; attempt <= m.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff with jitter
			baseDelay := time.Duration(m.config.InitialRetryDelay) * time.Second
			backoffDelay := time.Duration(float64(baseDelay) * float64(attempt) * m.config.BackoffMultiplier)
			maxDelay := time.Duration(m.config.MaxRetryDelay) * time.Second
			
			if backoffDelay > maxDelay {
				backoffDelay = maxDelay
			}
			
			m.logger.Debug("Retrying artwork download", 
				"type", artType.Name, 
				"attempt", attempt, 
				"delay", backoffDelay,
				"previous_error", downloadErr)
			time.Sleep(backoffDelay)
		}

		// Check artwork size before downloading to avoid gRPC message size limits
		resp, err := http.Head(downloadURL)
		if err != nil {
			downloadErr = fmt.Errorf("failed to check artwork size: %w", err)
			continue
		}
		resp.Body.Close()

		// Check content length (if provided by server)
		contentLength := resp.Header.Get("Content-Length")
		if contentLength != "" {
			if size, err := strconv.ParseInt(contentLength, 10, 64); err == nil {
				// Use configurable MaxAssetSize instead of hard-coded 10MB
				// Also ensure we don't exceed gRPC message size limits (keep under 15MB)
				maxSize := m.calculateMaxAssetSize()
				
				if size > maxSize {
					downloadErr = fmt.Errorf("artwork too large: %d bytes (max %d bytes)", size, maxSize)
					m.logger.Debug("Skipping large artwork", 
						"type", artType.Name, 
						"size_bytes", size, 
						"max_bytes", maxSize,
						"config_max", m.config.MaxAssetSize,
						"grpc_limit", maxSize,
						"url", downloadURL)
					
					// If all enabled art types are too large, this is not a retry-able error
					if attempt == m.config.MaxRetries {
						return fmt.Errorf("artwork consistently too large: %d bytes (max %d bytes)", size, maxSize)
					}
					continue
				}
				
				m.logger.Debug("Pre-download size check passed", 
					"type", artType.Name, 
					"size_bytes", size, 
					"max_bytes", maxSize)
			} else {
				m.logger.Debug("Could not parse Content-Length header", 
					"type", artType.Name, 
					"content_length", contentLength)
			}
		} else {
			m.logger.Debug("No Content-Length header provided by server, will check during download", 
				"type", artType.Name)
		}

		// Download artwork
		resp, err = client.Get(downloadURL)
		if err != nil {
			downloadErr = fmt.Errorf("network error: %w", err)
			continue
		}

		if resp.StatusCode == 404 {
			resp.Body.Close()
			m.logger.Debug("No artwork available for type", "type", artType.Name, "status", resp.StatusCode)
			return fmt.Errorf("no artwork available")
		}

		if resp.StatusCode == 503 {
			resp.Body.Close()
			downloadErr = fmt.Errorf("rate limited (503) - will retry")
			continue
		}

		if resp.StatusCode == 400 {
			resp.Body.Close()
			m.logger.Debug("Bad request for artwork type", "type", artType.Name, "status", resp.StatusCode)
			return fmt.Errorf("no artwork available")
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()
			downloadErr = fmt.Errorf("download failed with status %d", resp.StatusCode)
			continue
		}

		// Check content length using consistent size limits
		maxSize := m.calculateMaxAssetSize()
		
		if resp.ContentLength > 0 && resp.ContentLength > maxSize {
			resp.Body.Close()
			return fmt.Errorf("artwork too large: %d bytes (max: %d)", resp.ContentLength, maxSize)
		}

		// Read the image data
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			downloadErr = fmt.Errorf("failed to read response body: %w", err)
			continue
		}

		// Check actual downloaded size with consistent limits
		if int64(len(data)) > maxSize {
			return fmt.Errorf("artwork too large after download: %d bytes (max: %d)", len(data), maxSize)
		}
		
		m.logger.Debug("Post-download size check passed", 
			"type", artType.Name, 
			"actual_size", len(data), 
			"max_size", maxSize)

		// Get MIME type
		mimeType = resp.Header.Get("Content-Type")
		if mimeType == "" {
			mimeType = "image/jpeg" // Default fallback
		}

		imageData = data
		downloadErr = nil
		m.logger.Debug("Successfully downloaded artwork", "type", artType.Name, "size", len(imageData), "attempt", attempt+1)
		break
	}

	if downloadErr != nil {
		return fmt.Errorf("failed after %d attempts: %w", m.config.MaxRetries+1, downloadErr)
	}

	m.logger.Debug("Downloaded artwork data", "type", artType.Name, "size", len(imageData), "mime_type", mimeType)

	// Save the artwork using the host's AssetService via the plugin SDK
	metadata := map[string]string{
		"source":     "musicbrainz_cover_art_archive",
		"release_id": releaseID,
		"art_type":   artType.Name,
	}

	// Use the plugin context to save the asset
	// This will be handled by the host's AssetService
	return m.saveArtworkAsset(ctx, mediaFileID, artType.Subtype, imageData, mimeType, downloadURL, metadata)
}

// saveArtworkAsset saves artwork using the host's asset service
func (m *MusicBrainzEnricher) saveArtworkAsset(ctx context.Context, mediaFileID string, subtype string, data []byte, mimeType, downloadURL string, metadata map[string]string) error {
	if m.unifiedClient == nil {
		m.logger.Warn("Unified service not available - cannot save artwork", "media_file_id", mediaFileID, "subtype", subtype)
		return fmt.Errorf("unified service not available")
	}

	m.logger.Debug("Saving artwork asset via unified service", 
		"media_file_id", mediaFileID, 
		"subtype", subtype, 
		"size", len(data), 
		"mime_type", mimeType,
		"source_url", downloadURL)

	// Create save asset request
	request := &plugins.SaveAssetRequest{
		MediaFileID: mediaFileID,
		AssetType:   "music",
		Category:    "album", 
		Subtype:     subtype,
		Data:        data,
		MimeType:    mimeType,
		SourceURL:   downloadURL,
		PluginID:    m.pluginID, // Set the plugin ID for asset tracking
		Metadata:    metadata,
	}

	// Call unified service
	response, err := m.unifiedClient.AssetService().SaveAsset(ctx, request)
	if err != nil {
		m.logger.Error("Failed to save asset via unified service", "error", err, "media_file_id", mediaFileID, "subtype", subtype)
		return fmt.Errorf("failed to save asset: %w", err)
	}

	if !response.Success {
		m.logger.Error("Unified service reported save failure", "error", response.Error, "media_file_id", mediaFileID, "subtype", subtype)
		return fmt.Errorf("asset save failed: %s", response.Error)
	}

	m.logger.Info("Successfully saved artwork asset", 
		"media_file_id", mediaFileID, 
		"subtype", subtype, 
		"asset_id", response.AssetID,
		"hash", response.Hash,
		"path", response.RelativePath,
		"plugin_id", m.pluginID,
		"size", len(data))

	return nil
}

func main() {
	plugin := &MusicBrainzEnricher{}
	plugins.StartPlugin(plugin)
} 