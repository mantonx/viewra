package main

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
}

// Config represents the plugin configuration
type Config struct {
	Enabled          bool    `json:"enabled"`
	APIRateLimit     float64 `json:"api_rate_limit"`
	UserAgent        string  `json:"user_agent"`
	EnableArtwork    bool    `json:"enable_artwork"`
	ArtworkMaxSize   int     `json:"artwork_max_size"`
	ArtworkQuality   string  `json:"artwork_quality"`
	MatchThreshold   float64 `json:"match_threshold"`
	AutoEnrich       bool    `json:"auto_enrich"`
	OverwriteExisting bool   `json:"overwrite_existing"`
	CacheDurationHours int   `json:"cache_duration_hours"`
}

// Database models
type MusicBrainzCache struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	QueryHash string    `gorm:"uniqueIndex;not null" json:"query_hash"`
	QueryType string    `gorm:"not null" json:"query_type"`
	Response  string    `gorm:"type:text;not null" json:"response"`
	ExpiresAt time.Time `gorm:"not null;index" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type MusicBrainzEnrichment struct {
	ID                     uint      `gorm:"primaryKey" json:"id"`
	MediaFileID            uint      `gorm:"uniqueIndex;not null" json:"media_file_id"`
	MusicBrainzRecordingID string    `gorm:"index" json:"musicbrainz_recording_id,omitempty"`
	MusicBrainzReleaseID   string    `gorm:"index" json:"musicbrainz_release_id,omitempty"`
	MusicBrainzArtistID    string    `gorm:"index" json:"musicbrainz_artist_id,omitempty"`
	EnrichedTitle          string    `json:"enriched_title,omitempty"`
	EnrichedArtist         string    `json:"enriched_artist,omitempty"`
	EnrichedAlbum          string    `json:"enriched_album,omitempty"`
	EnrichedYear           int       `json:"enriched_year,omitempty"`
	MatchScore             float64   `json:"match_score"`
	ArtworkURL             string    `json:"artwork_url,omitempty"`
	ArtworkPath            string    `json:"artwork_path,omitempty"`
	EnrichedAt             time.Time `gorm:"not null" json:"enriched_at"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
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
		AutoEnrich:         false,
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
	
	// Check MusicBrainz API connectivity
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
func (m *MusicBrainzEnricher) MetadataScraperService() plugins.MetadataScraperService {
	return m
}

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
func (m *MusicBrainzEnricher) ScannerHookService() plugins.ScannerHookService {
	if m.config != nil && m.config.AutoEnrich {
		return m
	}
	return nil
}

func (m *MusicBrainzEnricher) OnMediaFileScanned(mediaFileID uint32, filePath string, metadata map[string]string) error {
	if !m.config.AutoEnrich {
		return nil
	}
	
	m.logger.Info("auto-enriching media file", "media_file_id", mediaFileID, "file_path", filePath)
	
	// Extract metadata from map
	title := metadata["title"]
	artist := metadata["artist"]
	album := metadata["album"]
	
	if title == "" || artist == "" {
		m.logger.Debug("insufficient metadata for enrichment", "media_file_id", mediaFileID)
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
	
	// Save enrichment
	return m.saveEnrichment(uint(mediaFileID), recording)
}

func (m *MusicBrainzEnricher) OnScanStarted(scanJobID, libraryID uint32, libraryPath string) error {
	m.logger.Info("scan started", "scan_job_id", scanJobID, "library_id", libraryID)
	return nil
}

func (m *MusicBrainzEnricher) OnScanCompleted(scanJobID, libraryID uint32, stats map[string]string) error {
	m.logger.Info("scan completed", "scan_job_id", scanJobID, "library_id", libraryID)
	return nil
}

// Database interface implementation
func (m *MusicBrainzEnricher) DatabaseService() plugins.DatabaseService {
	return m
}

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

// Admin page interface - not implemented for this plugin
func (m *MusicBrainzEnricher) AdminPageService() plugins.AdminPageService {
	return nil
}

// APIRegistrationService returns the APIRegistrationService implementation.
func (m *MusicBrainzEnricher) APIRegistrationService() plugins.APIRegistrationService {
	// Return `m` if MusicBrainzEnricher itself implements GetRegisteredRoutes
	// or return a dedicated struct that implements it.
	return m
}

// GetRegisteredRoutes implements the APIRegistrationService interface method.
// This is called by the host to get routes this plugin wants to register.
func (m *MusicBrainzEnricher) GetRegisteredRoutes(ctx context.Context) ([]*proto.APIRoute, error) {
	m.logger.Info("APIRegistrationService: GetRegisteredRoutes called for musicbrainz_enricher")
	routes := []*proto.APIRoute{
		{
			Path:        "/search", // Will be prefixed by host, e.g., /api/plugins/musicbrainz_enricher/search
			Method:      "GET",
			Description: "Search MusicBrainz for a track (test endpoint). Example: ?title=...&artist=...",
		},
		{
			Path:        "/config",
			Method:      "GET",
			Description: "Get current MusicBrainz enricher plugin configuration (test endpoint).",
		},
	}
	return routes, nil
}

// Internal methods

func (m *MusicBrainzEnricher) initDatabase() error {
	// Parse database URL and create connection
	// For now, assume it's SQLite
	if strings.HasPrefix(m.dbURL, "sqlite://") {
		dbPath := strings.TrimPrefix(m.dbURL, "sqlite://")
		
		db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
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
		
		return nil
	}
	
	return fmt.Errorf("unsupported database URL: %s", m.dbURL)
}

func (m *MusicBrainzEnricher) searchRecording(title, artist, album string) (*Recording, error) {
	// Build search query
	query := fmt.Sprintf("recording:\"%s\" AND artist:\"%s\"", title, artist)
	if album != "" {
		query += fmt.Sprintf(" AND release:\"%s\"", album)
	}
	
	// Generate cache key
	queryHash := m.generateQueryHash(query)
	
	// Check cache first
	if cached, err := m.getCachedResponse("recording", queryHash); err == nil {
		if len(cached) > 0 {
			return &cached[0], nil
		}
		return nil, nil
	}
	
	// Make API request
	apiURL := fmt.Sprintf("https://musicbrainz.org/ws/2/recording?query=%s&fmt=json&limit=5",
		url.QueryEscape(query))
	
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("User-Agent", m.config.UserAgent)
	
	// Rate limiting
	time.Sleep(time.Duration(1.0/m.config.APIRateLimit) * time.Second)
	
	resp, err := http.DefaultClient.Do(req)
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
	m.cacheRecordings("recording", queryHash, searchResp.Recordings)
	
	// Return best match (first result is usually best)
	if len(searchResp.Recordings) > 0 {
		return &searchResp.Recordings[0], nil
	}
	
	return nil, nil
}

func (m *MusicBrainzEnricher) generateQueryHash(query string) string {
	h := md5.New()
	h.Write([]byte(query))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (m *MusicBrainzEnricher) getCachedResponse(queryType, queryHash string) ([]Recording, error) {
	var cache MusicBrainzCache
	err := m.db.Where("query_hash = ? AND query_type = ? AND expires_at > ?",
		queryHash, queryType, time.Now()).First(&cache).Error
	if err != nil {
		return nil, err
	}
	
	var recordings []Recording
	if err := json.Unmarshal([]byte(cache.Response), &recordings); err != nil {
		return nil, err
	}
	
	return recordings, nil
}

func (m *MusicBrainzEnricher) cacheRecordings(queryType, queryHash string, recordings []Recording) {
	data, err := json.Marshal(recordings)
	if err != nil {
		m.logger.Error("failed to marshal recordings for cache", "error", err)
		return
	}
	
	cache := &MusicBrainzCache{
		QueryHash: queryHash,
		QueryType: queryType,
		Response:  string(data),
		ExpiresAt: time.Now().Add(time.Duration(m.config.CacheDurationHours) * time.Hour),
	}
	
	m.db.Create(cache)
}

func (m *MusicBrainzEnricher) saveEnrichment(mediaFileID uint, recording *Recording) error {
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

func main() {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "musicbrainz-enricher-plugin",
		Level: hclog.Info,
	})

	enricher := &MusicBrainzEnricher{
		logger: logger,
	}

	// pluginMap is the map of plugins we can dispense.
	var pluginMap = map[string]plugin.Plugin{
		"musicbrainz_enricher": &plugins.GRPCPlugin{Impl: enricher},
	}

	logger.Debug("MusicBrainz enricher plugin starting")
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugins.Handshake,
		Plugins:         pluginMap,
		GRPCServer:      plugin.DefaultGRPCServer, // Use default GRPC server
		Logger:          logger,
	})
	logger.Debug("MusicBrainz enricher plugin stopped")
} 