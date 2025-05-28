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
	"github.com/hashicorp/go-plugin"
	"github.com/mantonx/viewra/internal/plugins"
	"github.com/mantonx/viewra/internal/plugins/proto"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// MusicBrainzEnricher implements the plugin interfaces using HashiCorp go-plugin
type MusicBrainzEnricher struct {
	logger   hclog.Logger
	config   *Config
	db       *gorm.DB
	dbURL    string
	basePath string
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
type Recording struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	Score        float64       `json:"score"`
	ArtistCredit []ArtistCredit `json:"artist-credit"`
	Releases     []Release     `json:"releases"`
}

type ArtistCredit struct {
	Name   string `json:"name"`
	Artist Artist `json:"artist"`
}

type Artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Release struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Date  string `json:"date"`
}

type SearchResponse struct {
	Recordings []Recording `json:"recordings"`
	Count      int         `json:"count"`
	Offset     int         `json:"offset"`
}

// MusicBrainz API response structures
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
	ID             string                     `json:"id"`
	Name           string                     `json:"name"`
	SortName       string                     `json:"sort-name"`
	Disambiguation string                     `json:"disambiguation"`
	Aliases        []MusicBrainzAlias         `json:"aliases"`
}

type MusicBrainzAlias struct {
	SortName string `json:"sort-name"`
	Name     string `json:"name"`
	Locale   string `json:"locale"`
	Type     string `json:"type"`
	Primary  bool   `json:"primary"`
	Begin    string `json:"begin"`
	End      string `json:"end"`
}

type MusicBrainzRelease struct {
	ID           string                    `json:"id"`
	Title        string                    `json:"title"`
	StatusID     string                    `json:"status-id"`
	Status       string                    `json:"status"`
	Date         string                    `json:"date"`
	Country      string                    `json:"country"`
	ReleaseGroup MusicBrainzReleaseGroup   `json:"release-group"`
	Media        []MusicBrainzMedia        `json:"media"`
}

type MusicBrainzReleaseGroup struct {
	ID             string `json:"id"`
	TypeID         string `json:"type-id"`
	PrimaryTypeID  string `json:"primary-type-id"`
	Title          string `json:"title"`
	PrimaryType    string `json:"primary-type"`
	Disambiguation string `json:"disambiguation"`
}

type MusicBrainzMedia struct {
	Position  int                    `json:"position"`
	Format    string                 `json:"format"`
	Title     string                 `json:"title"`
	TrackCount int                   `json:"track-count"`
	Tracks    []MusicBrainzTrack     `json:"tracks"`
}

type MusicBrainzTrack struct {
	ID       string `json:"id"`
	Number   string `json:"number"`
	Title    string `json:"title"`
	Length   int    `json:"length"`
	Position int    `json:"position"`
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

// Plugin interface implementations for HashiCorp go-plugin

// Initialize implements the PluginImpl interface
func (m *MusicBrainzEnricher) Initialize(ctx *proto.PluginContext) error {
	m.logger = hclog.New(&hclog.LoggerOptions{
		Name:  "musicbrainz-enricher",
		Level: hclog.LevelFromString(ctx.LogLevel),
	})
	
	m.dbURL = ctx.DatabaseUrl
	m.basePath = ctx.BasePath
	
	// Parse configuration
	m.config = &Config{
		Enabled:            true,
		APIRateLimit:       0.8,
		UserAgent:          "Viewra/2.0",
		EnableArtwork:      true,
		ArtworkMaxSize:     1200,
		ArtworkQuality:     "front",
		MatchThreshold:     0.85,
		AutoEnrich:         true, // Enable auto-enrichment by default
		OverwriteExisting:  false,
		CacheDurationHours: 168,
	}
	
	// Override with provided config
	for key, value := range ctx.Config {
		switch key {
		case "enabled":
			if v, err := strconv.ParseBool(value); err == nil {
				m.config.Enabled = v
			}
		case "api_rate_limit":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				m.config.APIRateLimit = v
			}
		case "user_agent":
			m.config.UserAgent = value
		case "enable_artwork":
			if v, err := strconv.ParseBool(value); err == nil {
				m.config.EnableArtwork = v
			}
		case "auto_enrich":
			if v, err := strconv.ParseBool(value); err == nil {
				m.config.AutoEnrich = v
			}
		case "overwrite_existing":
			if v, err := strconv.ParseBool(value); err == nil {
				m.config.OverwriteExisting = v
			}
		}
	}
	
	// Initialize database connection
	if err := m.initDatabase(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	
	m.logger.Info("MusicBrainz enricher initialized", "config", m.config)
	return nil
}

// Start implements the PluginImpl interface
func (m *MusicBrainzEnricher) Start() error {
	m.logger.Info("MusicBrainz enricher started")
	return nil
}

// Stop implements the PluginImpl interface
func (m *MusicBrainzEnricher) Stop() error {
	m.logger.Info("MusicBrainz enricher stopped")
	if m.db != nil {
		if sqlDB, err := m.db.DB(); err == nil {
			sqlDB.Close()
		}
	}
	return nil
}

// Info implements the PluginImpl interface
func (m *MusicBrainzEnricher) Info() (*proto.PluginInfo, error) {
	return &proto.PluginInfo{
		Id:          "musicbrainz_enricher",
		Name:        "MusicBrainz Metadata Enricher",
		Version:     "1.0.0",
		Description: "Enriches music metadata using the MusicBrainz database via HashiCorp go-plugin",
		Author:      "Viewra Team",
		Website:     "https://github.com/mantonx/viewra",
		Repository:  "https://github.com/mantonx/viewra",
		License:     "MIT",
		Type:        "metadata_scraper",
		Tags:        []string{"music", "metadata", "enrichment", "musicbrainz"},
		Status:      "enabled",
		InstallPath: m.basePath,
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	}, nil
}

// Health implements the PluginImpl interface
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

// Metadata scraper interface implementation
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

// Scanner hook interface implementation
func (m *MusicBrainzEnricher) OnMediaFileScanned(mediaFileID uint32, filePath string, metadata map[string]string) error {
	if !m.config.AutoEnrich {
		return nil
	}
	
	m.logger.Debug("processing media file for enrichment", "media_file_id", mediaFileID, "file_path", filePath)

	// Extract metadata from map
	title := metadata["title"]
	artist := metadata["artist"]
	album := metadata["album"]
	
	if title == "" || artist == "" {
		m.logger.Debug("insufficient metadata for enrichment", "media_file_id", mediaFileID)
		return nil
	}

	// Check if already enriched
	var existing MusicBrainzEnrichment
	err := m.db.Where("media_file_id = ?", mediaFileID).First(&existing).Error
	if err == nil && !m.config.OverwriteExisting {
		m.logger.Debug("file already enriched", "media_file_id", mediaFileID)
		return nil
	}

	// Search MusicBrainz
	recording, err := m.searchRecording(title, artist, album)
	if err != nil {
		return fmt.Errorf("failed to search MusicBrainz: %w", err)
	}

	if recording == nil {
		m.logger.Debug("no MusicBrainz match found", "media_file_id", mediaFileID)
		return nil
	}

	if recording.Score < m.config.MatchThreshold {
		m.logger.Debug("match score below threshold", "media_file_id", mediaFileID, "score", recording.Score, "threshold", m.config.MatchThreshold)
		return nil
	}

	// Save enrichment
	m.logger.Info("enriching media file", "media_file_id", mediaFileID, "title", title, "artist", artist, "score", recording.Score)
	return m.saveEnrichment(uint(mediaFileID), recording)
}

func (m *MusicBrainzEnricher) OnScanStarted(scanJobID, libraryID uint32, libraryPath string) error {
	if !m.config.AutoEnrich {
		return nil
	}
	m.logger.Info("scan started", "scan_job_id", scanJobID, "library_id", libraryID)
	return nil
}

func (m *MusicBrainzEnricher) OnScanCompleted(scanJobID, libraryID uint32, stats map[string]string) error {
	if !m.config.AutoEnrich {
		return nil
	}
	m.logger.Info("scan completed", "scan_job_id", scanJobID, "library_id", libraryID)
	return nil
}

// Service interface implementations (required by plugins.Implementation)

// MetadataScraperService returns the metadata scraper service implementation
func (m *MusicBrainzEnricher) MetadataScraperService() plugins.MetadataScraperService {
	return m
}

// ScannerHookService returns the scanner hook service implementation
func (m *MusicBrainzEnricher) ScannerHookService() plugins.ScannerHookService {
	// Always return self - the AutoEnrich check is done in individual hook methods
	return m
}

// DatabaseService returns the database service implementation
func (m *MusicBrainzEnricher) DatabaseService() plugins.DatabaseService {
	return m
}

// AdminPageService returns nil as this plugin doesn't provide admin pages
func (m *MusicBrainzEnricher) AdminPageService() plugins.AdminPageService {
	return nil
}

// APIRegistrationService returns the API registration service implementation
func (m *MusicBrainzEnricher) APIRegistrationService() plugins.APIRegistrationService {
	return m
}

// SearchService returns the search service implementation
func (m *MusicBrainzEnricher) SearchService() plugins.SearchService {
	return m // Return self as the SearchService implementation
}

// SearchService interface implementation - GO INTERFACE (correct)
func (m *MusicBrainzEnricher) Search(ctx context.Context, query map[string]string, limit, offset uint32) ([]*proto.SearchResult, uint32, bool, error) {
	// Add defensive logging and error handling
	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("PANIC in Search method", "panic", r)
		}
	}()
	
	m.logger.Info("SearchService.Search called via Go interface", "query", query, "limit", limit, "offset", offset)
	
	// Validate inputs
	if query == nil {
		m.logger.Error("query map is nil")
		return nil, 0, false, fmt.Errorf("query map cannot be nil")
	}
	
	// Extract search parameters
	title := query["title"]
	artist := query["artist"]
	album := query["album"]
	
	if title == "" || artist == "" {
		return nil, 0, false, fmt.Errorf("title and artist are required search parameters")
	}
	
	// Set reasonable defaults for limit
	if limit == 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50 // Cap at 50 to prevent abuse
	}
	
	// Ensure database connection is available
	if m.db == nil {
		m.logger.Warn("Database not initialized, initializing now...")
		if err := m.initDatabase(); err != nil {
			m.logger.Error("Failed to initialize database", "error", err)
			// Continue without caching
		}
	}
	
	m.logger.Info("Searching MusicBrainz with caching enabled", "title", title, "artist", artist, "album", album, "limit", limit, "db_available", m.db != nil)
	
	// Search MusicBrainz using helper method with caching
	recordings, err := m.searchRecordings(title, artist, album, int(limit))
	if err != nil {
		m.logger.Error("MusicBrainz API search failed", "error", err)
		return nil, 0, false, fmt.Errorf("MusicBrainz search failed: %w", err)
	}
	
	// Convert MusicBrainz recordings to SearchResult format
	var results []*proto.SearchResult
	for _, recording := range recordings {
		// Use MusicBrainz's native score (typically 0-100) 
		// Convert to 0-1 scale for consistency
		mbScore := recording.Score / 100.0
		
		m.logger.Debug("Processing recording", 
			"mb_title", recording.Title, 
			"search_title", title,
			"mb_artist", func() string {
				if len(recording.ArtistCredit) > 0 {
					return recording.ArtistCredit[0].Name
				}
				return "N/A"
			}(),
			"search_artist", artist,
			"mb_score", recording.Score,
			"normalized_score", mbScore)
		
		// Use configurable threshold
		if mbScore < m.config.MatchThreshold {
			m.logger.Debug("Skipping result below threshold", "title", recording.Title, "score", mbScore, "threshold", m.config.MatchThreshold)
			continue
		}
		
		// Get primary artist name
		artistName := ""
		if len(recording.ArtistCredit) > 0 {
			artistName = recording.ArtistCredit[0].Name
		}
		
		// Get primary release info
		albumTitle := ""
		releaseDate := ""
		if len(recording.Releases) > 0 {
			albumTitle = recording.Releases[0].Title
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
		result := &proto.SearchResult{
			Id:       recording.ID,
			Title:    recording.Title,
			Artist:   artistName,
			Album:    albumTitle,
			Score:    mbScore,
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
	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("PANIC in GetSearchCapabilities method", "panic", r)
		}
	}()
	
	m.logger.Info("SearchService.GetSearchCapabilities called via Go interface")
	
	// Return capabilities using Go interface signature
	supportedFields := []string{"title", "artist", "album"}
	supportsPagination := false
	maxResults := uint32(10)
	
	return supportedFields, supportsPagination, maxResults, nil
}

// Database service implementation
func (m *MusicBrainzEnricher) GetModels() []string {
	return []string{
		"MusicBrainzCache",
		"MusicBrainzEnrichment",
	}
}

func (m *MusicBrainzEnricher) Migrate(connectionString string) error {
	// Auto-migrate plugin tables
	return m.db.AutoMigrate(&MusicBrainzCache{}, &MusicBrainzEnrichment{})
}

func (m *MusicBrainzEnricher) Rollback(connectionString string) error {
	// Drop plugin tables
	return m.db.Migrator().DropTable(&MusicBrainzCache{}, &MusicBrainzEnrichment{})
}

// API registration service implementation
func (m *MusicBrainzEnricher) GetRegisteredRoutes(ctx context.Context) ([]*proto.APIRoute, error) {
	m.logger.Info("APIRegistrationService: GetRegisteredRoutes called for musicbrainz_enricher")
	routes := []*proto.APIRoute{
		{
			Path:        "/search",
			Method:      "GET", 
			Description: "Search MusicBrainz for a track. Example: ?title=...&artist=...",
		},
		{
			Path:        "/config",
			Method:      "GET",
			Description: "Get current MusicBrainz enricher plugin configuration.",
		},
	}
	return routes, nil
}

// Internal methods

func (m *MusicBrainzEnricher) initDatabase() error {
	if m.dbURL == "" {
		m.logger.Error("database URL is empty")
		return fmt.Errorf("database URL is empty")
	}
	
	m.logger.Info("initializing database connection", "db_url", m.dbURL)
	
	// Parse database URL and create connection
	// For now, assume it's SQLite
	if strings.HasPrefix(m.dbURL, "sqlite://") {
		dbPath := strings.TrimPrefix(m.dbURL, "sqlite://")
		m.logger.Info("parsed database path", "db_path", dbPath)
		
		// Ensure directory exists
		dbDir := filepath.Dir(dbPath)
		m.logger.Info("ensuring database directory exists", "db_dir", dbDir)
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			m.logger.Error("failed to create database directory", "error", err)
			return fmt.Errorf("failed to create database directory: %w", err)
		}
		
		m.logger.Info("opening database connection", "db_path", dbPath)
		db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			m.logger.Error("failed to connect to database", "error", err)
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		
		// Test the connection
		m.logger.Info("testing database connection")
		sqlDB, err := db.DB()
		if err != nil {
			m.logger.Error("failed to get underlying sql.DB", "error", err)
			return fmt.Errorf("failed to get underlying sql.DB: %w", err)
		}
		
		if err := sqlDB.Ping(); err != nil {
			m.logger.Error("failed to ping database", "error", err)
			return fmt.Errorf("failed to ping database: %w", err)
		}
		
		m.db = db
		m.logger.Info("database connection established successfully")
		
		// Auto-migrate tables
		m.logger.Info("starting database table migration")
		if err := m.db.AutoMigrate(&MusicBrainzCache{}, &MusicBrainzEnrichment{}); err != nil {
			m.logger.Error("failed to migrate database tables", "error", err)
			return fmt.Errorf("failed to migrate database: %w", err)
		}
		
		m.logger.Info("database tables migrated successfully")
		m.logger.Info("database initialized successfully", "db_path", dbPath)
		return nil
	}
	
	m.logger.Error("unsupported database URL", "db_url", m.dbURL)
	return fmt.Errorf("unsupported database URL: %s", m.dbURL)
}

// searchRecording searches MusicBrainz for a recording
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
	apiURL := fmt.Sprintf("https://musicbrainz.org/ws/2/recording?query=%s&fmt=json&limit=5",
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

// buildSearchQuery builds a MusicBrainz search query
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

// findBestMatch finds the best matching recording from search results
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

// calculateMatchScore calculates similarity score between recording and search terms
func (m *MusicBrainzEnricher) calculateMatchScore(recording *MusicBrainzRecording, title, artist, album string) float64 {
	var scores []float64
	
	// Title similarity (most important)
	titleScore := m.stringSimilarity(recording.Title, title)
	scores = append(scores, titleScore*0.5) // 50% weight
	
	// Artist similarity
	if len(recording.ArtistCredit) > 0 {
		artistScore := m.stringSimilarity(recording.ArtistCredit[0].Name, artist)
		scores = append(scores, artistScore*0.3) // 30% weight
	}
	
	// Album similarity (if provided)
	if album != "" && len(recording.Releases) > 0 {
		albumScore := m.stringSimilarity(recording.Releases[0].Title, album)
		scores = append(scores, albumScore*0.2) // 20% weight
	}
	
	// Calculate weighted average
	var total float64
	for _, score := range scores {
		total += score
	}
	
	return total
}

// stringSimilarity calculates similarity between two strings (simple implementation)
func (m *MusicBrainzEnricher) stringSimilarity(s1, s2 string) float64 {
	s1 = strings.ToLower(strings.TrimSpace(s1))
	s2 = strings.ToLower(strings.TrimSpace(s2))
	
	if s1 == s2 {
		return 1.0
	}
	
	// Simple similarity: check if one contains the other
	if strings.Contains(s1, s2) || strings.Contains(s2, s1) {
		return 0.8
	}
	
	// Calculate Levenshtein-like similarity (simplified)
	longer := s1
	shorter := s2
	if len(s2) > len(s1) {
		longer = s2
		shorter = s1
	}
	
	if len(longer) == 0 {
		return 0.0
	}
	
	// Count matching characters
	matches := 0
	for i, char := range shorter {
		if i < len(longer) && longer[i] == byte(char) {
			matches++
		}
	}
	
	return float64(matches) / float64(len(longer))
}

// getCacheKey generates a cache key for the search terms
func (m *MusicBrainzEnricher) getCacheKey(title, artist, album string) string {
	data := fmt.Sprintf("%s|%s|%s", title, artist, album)
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// getCachedResult retrieves cached MusicBrainz result (single)
func (m *MusicBrainzEnricher) getCachedResult(cacheKey string) *MusicBrainzRecording {
	if m.db == nil {
		return nil
	}
	
	var cache MusicBrainzCache
	err := m.db.Where("cache_key = ? AND expires_at > ?", cacheKey, time.Now()).First(&cache).Error
	if err != nil {
		if err.Error() != "record not found" {
			m.logger.Debug("cache lookup failed", "error", err, "cache_key", cacheKey)
		}
		return nil
	}
	
	var recording MusicBrainzRecording
	if err := json.Unmarshal([]byte(cache.Data), &recording); err != nil {
		m.logger.Warn("failed to unmarshal cached result", "error", err, "cache_key", cacheKey)
		// Delete corrupted cache entry
		m.db.Delete(&cache)
		return nil
	}
	
	m.logger.Debug("cache hit (single)", "cache_key", cacheKey)
	return &recording
}

// cacheResult caches a single MusicBrainz result
func (m *MusicBrainzEnricher) cacheResult(cacheKey string, recording *MusicBrainzRecording) {
	if m.db == nil {
		m.logger.Debug("database not available, skipping cache storage (single)")
		return
	}
	
	// Serialize recording to JSON
	data, err := json.Marshal(recording)
	if err != nil {
		m.logger.Warn("failed to marshal result for caching", "error", err, "cache_key", cacheKey)
		return
	}
	
	// Calculate expiration time
	expiresAt := time.Now().Add(time.Duration(m.config.CacheDurationHours) * time.Hour)
	
	// Save to cache using proper UPSERT
	cache := MusicBrainzCache{
		CacheKey:  cacheKey,
		Data:      string(data),
		ExpiresAt: expiresAt,
	}
	
	// Use proper UPSERT - find existing or create new
	result := m.db.Where("cache_key = ?", cacheKey).FirstOrCreate(&cache)
	if result.Error != nil {
		m.logger.Warn("failed to cache result", "error", result.Error, "cache_key", cacheKey)
	} else {
		m.logger.Debug("cache stored (single)", "cache_key", cacheKey, "expires_at", expiresAt)
	}
}

func (m *MusicBrainzEnricher) saveEnrichment(mediaFileID uint, recording *MusicBrainzRecording) error {
	// Check if already enriched and not overwriting
	if !m.config.OverwriteExisting {
		var existing MusicBrainzEnrichment
		if err := m.db.Where("media_file_id = ?", mediaFileID).First(&existing).Error; err == nil {
			m.logger.Debug("media file already enriched", "media_file_id", mediaFileID)
			return nil
		}
	}
	
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
	
	// Save or update enrichment
	if m.config.OverwriteExisting {
		m.db.Where("media_file_id = ?", mediaFileID).Delete(&MusicBrainzEnrichment{})
	}
	
	if err := m.db.Create(enrichment).Error; err != nil {
		return fmt.Errorf("failed to save enrichment: %w", err)
	}
	
	m.logger.Info("media file enriched successfully",
		"media_file_id", mediaFileID,
		"recording_id", recording.ID,
		"match_score", recording.Score)
	
	return nil
}

// searchRecordings searches MusicBrainz for multiple recordings
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
	} else {
		m.logger.Debug("Database not available, skipping cache lookup")
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
	apiURL := fmt.Sprintf("https://musicbrainz.org/ws/2/recording?query=%s&fmt=json&limit=%d",
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
	
	// Keep MusicBrainz's original scores and add our calculated scores
	scoredRecordings := make([]MusicBrainzRecording, 0, len(mbResponse.Recordings))
	for _, recording := range mbResponse.Recordings {
		// Keep MusicBrainz's original score but also calculate our own for comparison
		ourScore := m.calculateMatchScore(&recording, title, artist, album)
		
		// Use the higher of the two scores (MusicBrainz or our calculation)
		if ourScore > recording.Score/100.0 {
			recording.Score = ourScore * 100.0 // Convert back to 0-100 scale
		}
		
		scoredRecordings = append(scoredRecordings, recording)
	}
	
	// Cache results if database is available
	m.logger.Info("checking cache conditions", "db_available", m.db != nil, "cache_key", cacheKey, "cache_key_empty", cacheKey == "", "results_count", len(scoredRecordings))
	
	if m.db != nil && cacheKey != "" {
		m.logger.Info("calling cacheResults", "cache_key", cacheKey, "results_count", len(scoredRecordings))
		m.cacheResults(cacheKey, scoredRecordings)
	} else {
		if m.db == nil {
			m.logger.Warn("Database not available, skipping cache storage")
		}
		if cacheKey == "" {
			m.logger.Warn("Cache key is empty, skipping cache storage")
		}
	}
	
	m.logger.Info("found MusicBrainz matches", 
		"title", title, 
		"artist", artist, 
		"total_results", len(scoredRecordings))
	
	return scoredRecordings, nil
}

// getCachedResults retrieves cached MusicBrainz search results (multiple)
func (m *MusicBrainzEnricher) getCachedResults(cacheKey string) []MusicBrainzRecording {
	if m.db == nil {
		return nil
	}
	
	var cache MusicBrainzCache
	err := m.db.Where("cache_key = ? AND expires_at > ?", cacheKey, time.Now()).First(&cache).Error
	if err != nil {
		if err.Error() != "record not found" {
			m.logger.Debug("cache lookup failed", "error", err, "cache_key", cacheKey)
		}
		return nil
	}
	
	var recordings []MusicBrainzRecording
	if err := json.Unmarshal([]byte(cache.Data), &recordings); err != nil {
		m.logger.Warn("failed to unmarshal cached search results", "error", err, "cache_key", cacheKey)
		// Delete corrupted cache entry
		m.db.Delete(&cache)
		return nil
	}
	
	m.logger.Debug("cache hit", "cache_key", cacheKey, "results_count", len(recordings))
	return recordings
}

// cacheResults caches multiple MusicBrainz search results
func (m *MusicBrainzEnricher) cacheResults(cacheKey string, recordings []MusicBrainzRecording) {
	m.logger.Info("cacheResults called", "cache_key", cacheKey, "recordings_count", len(recordings), "db_available", m.db != nil)
	
	if m.db == nil {
		m.logger.Debug("database not available, skipping cache storage")
		return
	}
	
	// Serialize recordings to JSON
	data, err := json.Marshal(recordings)
	if err != nil {
		m.logger.Warn("failed to marshal search results for caching", "error", err, "cache_key", cacheKey)
		return
	}
	
	m.logger.Info("attempting to cache results", "cache_key", cacheKey, "data_length", len(data))
	
	// Calculate expiration time
	expiresAt := time.Now().Add(time.Duration(m.config.CacheDurationHours) * time.Hour)
	
	// Save to cache using proper UPSERT
	cache := MusicBrainzCache{
		CacheKey:  cacheKey,
		Data:      string(data),
		ExpiresAt: expiresAt,
	}
	
	// Use proper UPSERT - find existing or create new
	result := m.db.Where("cache_key = ?", cacheKey).FirstOrCreate(&cache)
	if result.Error != nil {
		m.logger.Error("failed to cache search results", "error", result.Error, "cache_key", cacheKey)
	} else {
		m.logger.Info("cache stored successfully", "cache_key", cacheKey, "results_count", len(recordings), "expires_at", expiresAt)
	}
}

// HashiCorp go-plugin main function
func main() {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "musicbrainz-enricher-plugin",
		Level: hclog.Info,
	})

	enricher := &MusicBrainzEnricher{
		logger: logger,
	}

	// Verify that our enricher implements the correct interface
	var _ plugins.Implementation = enricher

	// pluginMap is the map of plugins we can dispense.
	grpcPlugin := &plugins.GRPCPlugin{Impl: enricher}
	var pluginMap = map[string]plugin.Plugin{
		"plugin": grpcPlugin,
	}

	logger.Info("MusicBrainz enricher plugin starting")
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugins.Handshake,
		Plugins:         pluginMap,
		GRPCServer:      plugin.DefaultGRPCServer,
		Logger:          logger,
	})
	logger.Info("MusicBrainz enricher plugin stopped")
} 