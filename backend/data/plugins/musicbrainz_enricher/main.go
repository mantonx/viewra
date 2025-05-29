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
	lastAPICall *time.Time
}

// Config represents the plugin configuration
type Config struct {
	Enabled             bool    `json:"enabled" default:"true"`
	APIRateLimit        float64 `json:"api_rate_limit" default:"0.8"`
	UserAgent           string  `json:"user_agent" default:"Viewra/2.0"`
	EnableArtwork       bool    `json:"enable_artwork" default:"true"`
	ArtworkMaxSize      int     `json:"artwork_max_size" default:"1200"`
	ArtworkQuality      string  `json:"artwork_quality" default:"front"`
	MatchThreshold      float64 `json:"match_threshold" default:"0.85"`
	AutoEnrich          bool    `json:"auto_enrich" default:"true"`
	OverwriteExisting   bool    `json:"overwrite_existing" default:"false"`
	CacheDurationHours  int     `json:"cache_duration_hours" default:"168"`
}

// Database models for plugin data
type MusicBrainzEnrichment struct {
	ID                     uint      `gorm:"primaryKey"`
	MediaFileID            uint      `gorm:"not null;index"`
	MusicBrainzRecordingID string    `gorm:"size:36"`
	MusicBrainzArtistID    string    `gorm:"size:36"`
	MusicBrainzReleaseID   string    `gorm:"size:36"`
	EnrichedTitle          string    `gorm:"size:512"`
	EnrichedArtist         string    `gorm:"size:512"`
	EnrichedAlbum          string    `gorm:"size:512"`
	EnrichedGenre          string    `gorm:"size:255"`
	EnrichedYear           int
	MatchScore             float64
	EnrichedAt             time.Time `gorm:"autoCreateTime"`
	UpdatedAt              time.Time `gorm:"autoUpdateTime"`
}

type MusicBrainzCache struct {
	ID        uint      `gorm:"primaryKey"`
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

// Core plugin interface implementation
func (m *MusicBrainzEnricher) Initialize(ctx *plugins.PluginContext) error {
	m.logger = hclog.New(&hclog.LoggerOptions{
		Name:  "musicbrainz-enricher",
		Level: hclog.LevelFromString(ctx.LogLevel),
	})

	m.basePath = ctx.BasePath
	m.dbURL = ctx.DatabaseURL

	// Initialize configuration with defaults
	m.config = &Config{
		Enabled:             true,
		APIRateLimit:        0.8,
		UserAgent:           "Viewra/2.0",
		EnableArtwork:       true,
		ArtworkMaxSize:      1200,
		ArtworkQuality:      "front",
		MatchThreshold:      0.85,
		AutoEnrich:          true,
		OverwriteExisting:   false,
		CacheDurationHours:  168,
	}

	m.logger.Info("Initializing MusicBrainz enricher plugin", 
		"base_path", m.basePath,
		"database_url", m.dbURL,
		"api_rate_limit", m.config.APIRateLimit,
		"match_threshold", m.config.MatchThreshold)

	// Initialize database connection
	if err := m.initDatabase(); err != nil {
		m.logger.Error("Failed to initialize database", "error", err)
		return fmt.Errorf("database initialization failed: %w", err)
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
	if m.db != nil {
		if sqlDB, err := m.db.DB(); err == nil {
			sqlDB.Close()
		}
	}
	return nil
}

func (m *MusicBrainzEnricher) Info() (*plugins.PluginInfo, error) {
	return &plugins.PluginInfo{
		ID:          "musicbrainz_enricher",
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
		"plugin":     "musicbrainz_enricher",
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
func (m *MusicBrainzEnricher) OnMediaFileScanned(mediaFileID uint32, filePath string, metadata map[string]string) error {
	if !m.config.AutoEnrich {
		return nil
	}

	title := metadata["title"]
	artist := metadata["artist"]
	album := metadata["album"]

	if title == "" || artist == "" {
		m.logger.Debug("Skipping enrichment - missing title or artist", "file", filePath)
		return nil
	}

	m.logger.Info("Enriching media file", "id", mediaFileID, "title", title, "artist", artist)

	// Search for recording
	recording, err := m.searchRecording(title, artist, album)
	if err != nil {
		m.logger.Error("Failed to search MusicBrainz", "error", err)
		return err
	}

	if recording == nil {
		m.logger.Debug("No MusicBrainz match found", "title", title, "artist", artist)
		return nil
	}

	// Save enrichment
	if err := m.saveEnrichment(uint(mediaFileID), recording); err != nil {
		m.logger.Error("Failed to save enrichment", "error", err)
		return err
	}

	return nil
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
			"plugin":          "musicbrainz_enricher",
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
		"MusicBrainzEnrichment",
	}
}

func (m *MusicBrainzEnricher) Migrate(connectionString string) error {
	return m.db.AutoMigrate(&MusicBrainzCache{}, &MusicBrainzEnrichment{})
}

func (m *MusicBrainzEnricher) Rollback(connectionString string) error {
	return m.db.Migrator().DropTable(&MusicBrainzCache{}, &MusicBrainzEnrichment{})
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
	if err := m.db.AutoMigrate(&MusicBrainzCache{}, &MusicBrainzEnrichment{}); err != nil {
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
		minInterval := time.Duration(float64(time.Second) / m.config.APIRateLimit)
		if elapsed < minInterval {
			time.Sleep(minInterval - elapsed)
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
		minInterval := time.Duration(float64(time.Second) / m.config.APIRateLimit)
		if elapsed < minInterval {
			time.Sleep(minInterval - elapsed)
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

func (m *MusicBrainzEnricher) saveEnrichment(mediaFileID uint, recording *MusicBrainzRecording) error {
	if m.db == nil {
		return fmt.Errorf("database not available")
	}
	
	// Get primary artist name
	artistName := ""
	if len(recording.ArtistCredit) > 0 {
		artistName = recording.ArtistCredit[0].Name
	}
	
	// Get primary release info
	albumTitle := ""
	releaseYear := 0
	if len(recording.Releases) > 0 {
		albumTitle = recording.Releases[0].Title
		if recording.Releases[0].Date != "" {
			if year, err := strconv.Atoi(recording.Releases[0].Date[:4]); err == nil {
				releaseYear = year
			}
		}
	}
	
	// Get genre from release group
	genre := ""
	if len(recording.Releases) > 0 {
		genre = recording.Releases[0].ReleaseGroup.PrimaryType
	}
	
	enrichment := MusicBrainzEnrichment{
		MediaFileID:            mediaFileID,
		MusicBrainzRecordingID: recording.ID,
		EnrichedTitle:          recording.Title,
		EnrichedArtist:         artistName,
		EnrichedAlbum:          albumTitle,
		EnrichedGenre:          genre,
		EnrichedYear:           releaseYear,
		MatchScore:             recording.Score,
	}
	
	// Add artist and release IDs if available
	if len(recording.ArtistCredit) > 0 {
		enrichment.MusicBrainzArtistID = recording.ArtistCredit[0].Artist.ID
	}
	if len(recording.Releases) > 0 {
		enrichment.MusicBrainzReleaseID = recording.Releases[0].ID
	}
	
	// Save or update enrichment
	result := m.db.Where("media_file_id = ?", mediaFileID).Save(&enrichment)
	if result.Error != nil {
		return fmt.Errorf("failed to save enrichment: %w", result.Error)
	}
	
	m.logger.Info("Saved MusicBrainz enrichment", 
		"media_file_id", mediaFileID,
		"recording_id", recording.ID,
		"title", recording.Title,
		"artist", artistName,
		"score", recording.Score)
	
	return nil
}

func main() {
	plugin := &MusicBrainzEnricher{}
	plugins.StartPlugin(plugin)
} 