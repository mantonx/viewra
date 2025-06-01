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
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/pkg/plugins"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TMDbEnricher implements the plugin interfaces using the modular pkg/plugins
type TMDbEnricher struct {
	logger   plugins.Logger
	config   *Config
	db       *gorm.DB
	basePath string
	dbURL    string
	hostServiceAddr string
	assetService plugins.AssetServiceClient
}

// Config represents the plugin configuration
type Config struct {
	Enabled            bool    `json:"enabled" default:"true"`
	APIKey             string  `json:"api_key"`
	APIRateLimit       float64 `json:"api_rate_limit" default:"1.0"`
	UserAgent          string  `json:"user_agent" default:"Viewra/2.0"`
	Language           string  `json:"language" default:"en-US"`
	Region             string  `json:"region" default:"US"`
	EnableMovies       bool    `json:"enable_movies" default:"true"`
	EnableTVShows      bool    `json:"enable_tv_shows" default:"true"`
	EnableEpisodes     bool    `json:"enable_episodes" default:"true"`
	EnableArtwork      bool    `json:"enable_artwork" default:"true"`
	DownloadPosters    bool    `json:"download_posters" default:"true"`
	DownloadBackdrops  bool    `json:"download_backdrops" default:"true"`
	DownloadLogos      bool    `json:"download_logos" default:"true"`
	PosterSize         string  `json:"poster_size" default:"w500"`
	BackdropSize       string  `json:"backdrop_size" default:"w1280"`
	LogoSize           string  `json:"logo_size" default:"w500"`
	MatchThreshold     float64 `json:"match_threshold" default:"0.85"`
	AutoEnrich         bool    `json:"auto_enrich" default:"true"`
	OverwriteExisting  bool    `json:"overwrite_existing" default:"false"`
	MatchYear          bool    `json:"match_year" default:"true"`
	YearTolerance      int     `json:"year_tolerance" default:"2"`
	CacheDurationHours int     `json:"cache_duration_hours" default:"168"`
}

// Database models
type TMDbCache struct {
	ID        uint32    `gorm:"primaryKey" json:"id"`
	QueryHash string    `gorm:"uniqueIndex;not null" json:"query_hash"`
	QueryType string    `gorm:"not null" json:"query_type"`
	Response  string    `gorm:"type:text;not null" json:"response"`
	ExpiresAt time.Time `gorm:"not null;index" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName specifies the table name for TMDbCache
func (TMDbCache) TableName() string {
	return "tm_db_caches"
}

type TMDbEnrichment struct {
	ID                 uint32    `gorm:"primaryKey" json:"id"`
	MediaFileID        string    `gorm:"uniqueIndex;not null" json:"media_file_id"`
	TMDbID             int       `gorm:"index" json:"tmdb_id,omitempty"`
	MediaType          string    `gorm:"not null" json:"media_type"` // "movie", "tv", "episode"
	SeasonNumber       int       `json:"season_number,omitempty"`
	EpisodeNumber      int       `json:"episode_number,omitempty"`
	EnrichedTitle      string    `json:"enriched_title,omitempty"`
	EnrichedOverview   string    `gorm:"type:text" json:"enriched_overview,omitempty"`
	EnrichedYear       int       `json:"enriched_year,omitempty"`
	EnrichedGenres     string    `json:"enriched_genres,omitempty"`
	EnrichedRating     float64   `json:"enriched_rating,omitempty"`
	EnrichedRuntime    int       `json:"enriched_runtime,omitempty"`
	MatchScore         float64   `json:"match_score"`
	PosterURL          string    `json:"poster_url,omitempty"`
	BackdropURL        string    `json:"backdrop_url,omitempty"`
	LogoURL            string    `json:"logo_url,omitempty"`
	PosterPath         string    `json:"poster_path,omitempty"`
	BackdropPath       string    `json:"backdrop_path,omitempty"`
	LogoPath           string    `json:"logo_path,omitempty"`
	EnrichedAt         time.Time `gorm:"not null" json:"enriched_at"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// TableName specifies the table name for TMDbEnrichment
func (TMDbEnrichment) TableName() string {
	return "tm_db_enrichments"
}

// TMDb API types
type SearchResponse struct {
	Page         int      `json:"page"`
	Results      []Result `json:"results"`
	TotalPages   int      `json:"total_pages"`
	TotalResults int      `json:"total_results"`
}

type Result struct {
	ID               int      `json:"id"`
	Title            string   `json:"title,omitempty"`            // Movies
	Name             string   `json:"name,omitempty"`             // TV Shows
	OriginalTitle    string   `json:"original_title,omitempty"`   // Movies
	OriginalName     string   `json:"original_name,omitempty"`    // TV Shows
	Overview         string   `json:"overview"`
	ReleaseDate      string   `json:"release_date,omitempty"`     // Movies
	FirstAirDate     string   `json:"first_air_date,omitempty"`   // TV Shows
	GenreIDs         []int    `json:"genre_ids"`
	VoteAverage      float64  `json:"vote_average"`
	VoteCount        int      `json:"vote_count"`
	Popularity       float64  `json:"popularity"`
	PosterPath       string   `json:"poster_path,omitempty"`
	BackdropPath     string   `json:"backdrop_path,omitempty"`
	Adult            bool     `json:"adult,omitempty"`
	Video            bool     `json:"video,omitempty"`
	MediaType        string   `json:"media_type,omitempty"`
	OriginCountry    []string `json:"origin_country,omitempty"`   // TV Shows
	OriginalLanguage string   `json:"original_language"`
}

type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Plugin lifecycle methods

// Initialize implements the plugins.Implementation interface
func (t *TMDbEnricher) Initialize(ctx *plugins.PluginContext) error {
	t.logger = hclog.New(&hclog.LoggerOptions{
		Name:  "tmdb_enricher",
		Level: hclog.Debug,
	})
	
	t.logger.Info("TMDb enricher plugin initializing", "database_url", ctx.DatabaseURL, "base_path", ctx.BasePath)
	
	// Store context information
	t.dbURL = ctx.DatabaseURL
	t.basePath = ctx.BasePath
	t.hostServiceAddr = ctx.HostServiceAddr
	
	// Load configuration
	if err := t.loadConfig(); err != nil {
		t.logger.Error("Failed to load configuration", "error", err)
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	
	t.logger.Info("Configuration loaded", "enabled", t.config.Enabled, "api_key_set", t.config.APIKey != "")
	
	// Initialize database connection
	if err := t.initDatabase(); err != nil {
		t.logger.Error("Failed to initialize database", "error", err)
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	
	t.logger.Info("Database initialized successfully")
	
	// Initialize asset service client if host service address is provided
	if t.hostServiceAddr != "" {
		assetClient, err := plugins.NewAssetServiceClient(t.hostServiceAddr)
		if err != nil {
			t.logger.Error("Failed to connect to host asset service", "error", err, "addr", t.hostServiceAddr)
			return fmt.Errorf("failed to connect to host asset service: %w", err)
		}
		t.assetService = assetClient
		t.logger.Info("Connected to host asset service", "addr", t.hostServiceAddr)
	} else {
		t.logger.Warn("No host service address provided - asset saving will be disabled")
	}

	t.logger.Info("TMDb enricher plugin initialized successfully")
	return nil
}

// loadConfig loads the plugin configuration
func (t *TMDbEnricher) loadConfig() error {
	// Initialize with default configuration
	t.config = &Config{
		Enabled:            true,
		APIKey:             "eyJhbGciOiJIUzI1NiJ9.eyJhdWQiOiI1YTU2ODc0YjRmMzU4YjIzZDhkM2YzZmI5ZDc4NDNiOSIsIm5iZiI6MTc0ODYzOTc1Ny40MDEsInN1YiI6IjY4M2EyMDBkNzA5OGI4MzMzNThmZThmOSIsInNjb3BlcyI6WyJhcGlfcmVhZCJdLCJ2ZXJzaW9uIjoxfQ.OXT68T0EtU-WXhcP7nwyWjMePuEuCpfWtDlvdntWKw8", // Load from plugin.cue
		APIRateLimit:       1.0,
		UserAgent:          "Viewra/2.0",
		Language:           "en-US",
		Region:             "US",
		EnableMovies:       true,
		EnableTVShows:      true,
		EnableEpisodes:     true,
		EnableArtwork:      true,
		DownloadPosters:    true,
		DownloadBackdrops:  true,
		DownloadLogos:      true,
		PosterSize:         "w500",
		BackdropSize:       "w1280",
		LogoSize:           "w500",
		MatchThreshold:     0.85,
		AutoEnrich:         true,
		OverwriteExisting:  false,
		MatchYear:          true,
		YearTolerance:      2,
		CacheDurationHours: 168,
	}
	return nil
}

// Start implements the plugins.Implementation interface
func (t *TMDbEnricher) Start() error {
	t.logger.Info("TMDb enricher started", "version", "1.0.0")
	return nil
}

// Stop implements the plugins.Implementation interface
func (t *TMDbEnricher) Stop() error {
	if t.db != nil {
		if sqlDB, err := t.db.DB(); err == nil {
			sqlDB.Close()
		}
	}
	t.logger.Info("TMDb enricher stopped")
	return nil
}

// Info implements the plugins.Implementation interface
func (t *TMDbEnricher) Info() (*plugins.PluginInfo, error) {
	return &plugins.PluginInfo{
		ID:          "tmdb_enricher",
		Name:        "TMDb Metadata Enricher",
		Version:     "1.0.0",
		Type:        "metadata_scraper",
		Description: "Enriches TV shows and movie metadata using The Movie Database (TMDb)",
		Author:      "Viewra Team",
	}, nil
}

// Health implements the plugins.Implementation interface
func (t *TMDbEnricher) Health() error {
	if !t.config.Enabled {
		return fmt.Errorf("plugin is disabled")
	}
	
	if t.config.APIKey == "" {
		return fmt.Errorf("TMDb API key not configured")
	}
	
	// Test database connection
	if t.db != nil {
		sqlDB, err := t.db.DB()
		if err != nil {
			return fmt.Errorf("failed to get database instance: %w", err)
		}
		if err := sqlDB.Ping(); err != nil {
			return fmt.Errorf("database ping failed: %w", err)
		}
	}
	
	// Check TMDb API connectivity
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", "https://api.themoviedb.org/3/configuration", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+t.config.APIKey)
	req.Header.Set("User-Agent", t.config.UserAgent)
	
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("TMDb API not reachable: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("TMDb API returned status %d", resp.StatusCode)
	}
	
	return nil
}

// Service interface implementations
func (t *TMDbEnricher) MetadataScraperService() plugins.MetadataScraperService {
	return t
}

func (t *TMDbEnricher) ScannerHookService() plugins.ScannerHookService {
	return t
}

func (t *TMDbEnricher) DatabaseService() plugins.DatabaseService {
	return t
}

func (t *TMDbEnricher) AdminPageService() plugins.AdminPageService {
	return nil
}

func (t *TMDbEnricher) APIRegistrationService() plugins.APIRegistrationService {
	return t
}

func (t *TMDbEnricher) AssetService() plugins.AssetService {
	return nil // We don't implement AssetService in this plugin
}

func (t *TMDbEnricher) SearchService() plugins.SearchService {
	return nil
}

// MetadataScraperService implementation

// CanHandle implements the plugins.MetadataScraperService interface
func (t *TMDbEnricher) CanHandle(filePath, mimeType string) bool {
	if !t.config.Enabled {
		return false
	}
	
	ext := strings.ToLower(filepath.Ext(filePath))
	
	// Video file extensions
	videoExts := []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".ts", ".mts", ".m2ts"}
	for _, videoExt := range videoExts {
		if ext == videoExt {
			return true
		}
	}
	
	// MIME type check
	if strings.HasPrefix(mimeType, "video/") {
		return true
	}
	
	return false
}

// ExtractMetadata implements the plugins.MetadataScraperService interface
func (t *TMDbEnricher) ExtractMetadata(filePath string) (map[string]string, error) {
	if !t.config.Enabled {
		return nil, fmt.Errorf("plugin is disabled")
	}
	
	// Basic metadata extraction from filename
	filename := filepath.Base(filePath)
	
	return map[string]string{
		"filename":   filename,
		"plugin":     "tmdb_enricher",
		"can_enrich": "true",
	}, nil
}

// GetSupportedTypes implements the plugins.MetadataScraperService interface
func (t *TMDbEnricher) GetSupportedTypes() []string {
	types := []string{}
	
	if t.config.EnableMovies {
		types = append(types, "movie")
	}
	if t.config.EnableTVShows {
		types = append(types, "tv")
	}
	if t.config.EnableEpisodes {
		types = append(types, "episode")
	}
	
	return types
}

// ScannerHookService implementation

// OnMediaFileScanned implements the plugins.ScannerHookService interface
func (t *TMDbEnricher) OnMediaFileScanned(mediaFileID string, filePath string, metadata map[string]string) error {
	// Add safety checks to prevent crashes
	if t == nil {
		return fmt.Errorf("plugin not initialized")
	}
	
	if t.config == nil {
		return fmt.Errorf("plugin config not initialized")
	}
	
	if t.logger == nil {
		// Can't log without logger, but don't crash
		return nil
	}
	
	t.logger.Info("TMDb OnMediaFileScanned ENTRY", "media_file_id", mediaFileID, "file_path", filePath)
	
	t.logger.Debug("TMDb scanner hook called", "media_file_id", mediaFileID, "file_path", filePath)
	
	if !t.config.Enabled || !t.config.AutoEnrich {
		t.logger.Debug("TMDb enrichment disabled", "enabled", t.config.Enabled, "auto_enrich", t.config.AutoEnrich)
		return nil
	}
	
	if t.config.APIKey == "" {
		t.logger.Warn("TMDb API key not configured, skipping enrichment")
		return nil
	}
	
	// Add database check
	if t.db == nil {
		t.logger.Warn("Database not initialized, skipping enrichment")
		return nil
	}
	
	t.logger.Debug("processing media file", "media_file_id", mediaFileID, "file_path", filePath)
	
	// Debug: Log the metadata being passed
	t.logger.Info("metadata received", "metadata", metadata, "media_file_id", mediaFileID)
	
	// Check if already enriched
	if !t.config.OverwriteExisting {
		var existing TMDbEnrichment
		if err := t.db.Where("media_file_id = ?", mediaFileID).First(&existing).Error; err == nil {
			t.logger.Debug("media file already enriched, skipping", "media_file_id", mediaFileID)
			return nil
		}
	}
	
	// Extract title and year from filename or metadata
	title := t.extractTitle(filePath, metadata)
	year := t.extractYear(filePath, metadata)
	
	if title == "" {
		t.logger.Debug("no title extracted, skipping enrichment", "media_file_id", mediaFileID)
		return nil
	}
	
	t.logger.Debug("searching TMDb", "title", title, "year", year, "file_path", filePath)
	
	// Search TMDb for matches
	results, err := t.searchContent(title, year)
	if err != nil {
		t.logger.Warn("Failed to search TMDb", "error", err, "title", title)
		return nil // Don't fail the scan for API errors
	}
	
	if len(results) == 0 {
		t.logger.Debug("no TMDb match found", "media_file_id", mediaFileID, "title", title)
		return nil
	}
	
	// Find best match
	bestMatch := t.findBestMatch(results, title, year)
	if bestMatch == nil {
		t.logger.Debug("no suitable match found", "media_file_id", mediaFileID, "title", title)
		return nil
	}
	
	t.logger.Info("Found TMDb match", "media_file_id", mediaFileID, "title", title, "tmdb_id", bestMatch.ID, "match_title", t.getResultTitle(*bestMatch))
	
	// Save enrichment (pass string mediaFileID to existing method)
	if err := t.saveEnrichment(mediaFileID, bestMatch); err != nil {
		t.logger.Warn("Failed to save enrichment", "error", err, "media_file_id", mediaFileID)
		return nil // Don't fail the scan for save errors
	}
	
	t.logger.Info("Successfully enriched media file", "media_file_id", mediaFileID, "tmdb_id", bestMatch.ID)
	return nil
}

// OnScanStarted implements the plugins.ScannerHookService interface
func (t *TMDbEnricher) OnScanStarted(scanJobID, libraryID uint32, libraryPath string) error {
	t.logger.Info("scan started", "scan_job_id", scanJobID, "library_id", libraryID)
	return nil
}

// OnScanCompleted implements the plugins.ScannerHookService interface
func (t *TMDbEnricher) OnScanCompleted(scanJobID, libraryID uint32, stats map[string]string) error {
	t.logger.Info("scan completed", "scan_job_id", scanJobID, "library_id", libraryID, "stats", stats)
	
	// Clean up old cache entries
	t.cleanupCache()
	
	return nil
}

// DatabaseService implementation

// GetModels implements the plugins.DatabaseService interface
func (t *TMDbEnricher) GetModels() []string {
	return []string{
		"TMDbCache",
		"TMDbEnrichment",
	}
}

// Migrate implements the plugins.DatabaseService interface
func (t *TMDbEnricher) Migrate(connectionString string) error {
	t.logger.Info("migrating TMDb enricher database models", "connection_string", connectionString)
	
	// Use the connection string provided by the core system instead of our own db
	// This ensures tables are created in the main Viewra database
	db, err := t.connectToDatabase(connectionString)
	if err != nil {
		return fmt.Errorf("failed to connect to database for migration: %w", err)
	}
	
	// Auto-migrate the TMDb tables to the main database
	if err := db.AutoMigrate(&TMDbCache{}, &TMDbEnrichment{}); err != nil {
		return fmt.Errorf("failed to migrate TMDb tables: %w", err)
	}
	
	t.logger.Info("TMDb enricher database migration completed successfully")
	return nil
}

// Rollback implements the plugins.DatabaseService interface
func (t *TMDbEnricher) Rollback(connectionString string) error {
	t.logger.Info("rolling back TMDb enricher database models")
	
	// Use the connection string provided by the core system
	db, err := t.connectToDatabase(connectionString)
	if err != nil {
		return fmt.Errorf("failed to connect to database for rollback: %w", err)
	}
	
	return db.Migrator().DropTable(&TMDbCache{}, &TMDbEnrichment{})
}

// APIRegistrationService implementation

// GetRegisteredRoutes implements the plugins.APIRegistrationService interface
func (t *TMDbEnricher) GetRegisteredRoutes(ctx context.Context) ([]*plugins.APIRoute, error) {
	// Add nil check to prevent panic during early plugin loading
	if t == nil {
		return []*plugins.APIRoute{}, nil
	}
	
	// Only log if logger is available
	if t.logger != nil {
		t.logger.Info("APIRegistrationService: GetRegisteredRoutes called for tmdb_enricher")
	}
	
	return []*plugins.APIRoute{
		{
			Method:      "GET",
			Path:        "/api/plugins/tmdb/search",
			Description: "Search TMDb for content. Example: ?title=...&year=...&type=...",
		},
		{
			Method:      "GET",
			Path:        "/api/plugins/tmdb/config",
			Description: "Get current TMDb enricher plugin configuration.",
		},
	}, nil
}

// Helper methods

// connectToDatabase creates a database connection using the provided connection string
func (t *TMDbEnricher) connectToDatabase(connectionString string) (*gorm.DB, error) {
	var dialector gorm.Dialector
	
	if strings.HasPrefix(connectionString, "sqlite://") {
		dbPath := strings.TrimPrefix(connectionString, "sqlite://")
		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
		dialector = sqlite.Open(dbPath)
	} else if connectionString != "" {
		// Treat as direct path for SQLite
		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(connectionString), 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
		dialector = sqlite.Open(connectionString)
	} else {
		return nil, fmt.Errorf("no database connection string provided")
	}
	
	// Open database connection
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	
	return db, nil
}

func (t *TMDbEnricher) initDatabase() error {
	t.logger.Info("initializing TMDb enricher database connection", "dbURL", t.dbURL)
	
	if t.dbURL == "" {
		return fmt.Errorf("database URL not provided")
	}
	
	// Connect to the shared database using the URL provided by the core system
	db, err := t.connectToDatabase(t.dbURL)
	if err != nil {
		return fmt.Errorf("failed to connect to shared database: %w", err)
	}
	
	t.db = db
	t.logger.Info("TMDb enricher connected to shared database successfully", "url", t.dbURL)
	
	// NOTE: We don't auto-migrate here anymore - migration is handled by the Migrate() method
	// which is called by the core system after plugin initialization
	
	return nil
}

func (t *TMDbEnricher) extractTitle(filePath string, metadata map[string]string) string {
	// For TV shows, prioritize show/series name over episode title
	if seriesName, exists := metadata["series_name"]; exists && seriesName != "" {
		return t.cleanupTitle(seriesName)
	}
	if showName, exists := metadata["show_name"]; exists && showName != "" {
		return t.cleanupTitle(showName)
	}
	// Check if "artist" field contains show name (common in TV metadata)
	if artist, exists := metadata["artist"]; exists && artist != "" {
		// Clean up the artist field which often contains the show name
		cleaned := t.cleanupTitle(artist)
		if cleaned != "" {
			t.logger.Info("extracted show title from artist metadata", "artist", artist, "cleaned", cleaned)
			return cleaned
		}
	}
	
	// Try to get title from metadata, but only if it looks like a show name, not episode title
	if title, exists := metadata["title"]; exists && title != "" {
		// If title contains quality tags or episode patterns, it's likely an episode title - skip it
		if t.looksLikeEpisodeTitle(title) {
			t.logger.Debug("skipping episode-like title from metadata", "title", title)
		} else {
			// If metadata provides a clean title, use it
			cleaned := t.cleanupTitle(title)
			if cleaned != "" {
				return cleaned
			}
		}
	}
	
	// Extract from filename
	filename := filepath.Base(filePath)
	filename = strings.TrimSuffix(filename, filepath.Ext(filename))
	
	t.logger.Info("extracting title from filename", "original", filename)
	
	// Try multiple TV show patterns
	if title := t.extractTVShowTitle(filename, filePath); title != "" {
		t.logger.Debug("extracted TV show title", "title", title, "filename", filename)
		return title
	}
	
	// Try movie patterns
	if title := t.extractMovieTitle(filename); title != "" {
		t.logger.Debug("extracted movie title", "title", title, "filename", filename)
		return title
	}
	
	// Fallback: clean up the filename as best as possible
	title := t.cleanupTitle(filename)
	t.logger.Debug("fallback title extraction", "title", title, "filename", filename)
	return title
}

// looksLikeEpisodeTitle checks if a title looks like an episode title rather than a show title
func (t *TMDbEnricher) looksLikeEpisodeTitle(title string) bool {
	// Check for quality tags
	qualityPatterns := []string{
		"[", "]", "720p", "1080p", "2160p", "4K", "WEBDL", "WEB-DL", "BluRay", "BDRip", 
		"DVDRip", "HDTV", "x264", "x265", "h264", "h265", "HEVC", "AAC", "AC3", "DTS",
		"5.1", "7.1", "2.0", "-", "Remux",
	}
	
	titleLower := strings.ToLower(title)
	qualityCount := 0
	for _, pattern := range qualityPatterns {
		if strings.Contains(titleLower, strings.ToLower(pattern)) {
			qualityCount++
		}
	}
	
	// If it contains multiple quality indicators, it's likely an episode title
	return qualityCount >= 2
}

// extractTVShowTitle handles various TV show filename patterns
func (t *TMDbEnricher) extractTVShowTitle(filename, filePath string) string {
	// Pattern 1: "Show Name (Year) - S##E## - Episode Title [Quality]" or "Show Name - S##E##"
	// Handle both " - S" and "- S" patterns
	seasonPatterns := []string{" - S", "- S", " -S", "-S"}
	for _, pattern := range seasonPatterns {
		if strings.Contains(filename, pattern) && strings.Contains(filename, "E") {
			seasonPos := strings.Index(filename, pattern)
			if seasonPos > 0 {
				showPart := filename[:seasonPos]
				cleaned := t.cleanupTitle(showPart)
				if cleaned != "" {
					t.logger.Debug("extracted show title via season pattern", "pattern", pattern, "show", cleaned, "filename", filename)
					return cleaned
				}
			}
		}
	}
	
	// Pattern 2: "Show Name S##E## Episode Title" (no separating dashes)
	seasonRegex := regexp.MustCompile(`^(.+?)\s+S\d+E\d+`)
	if matches := seasonRegex.FindStringSubmatch(filename); len(matches) > 1 {
		cleaned := t.cleanupTitle(matches[1])
		if cleaned != "" {
			t.logger.Debug("extracted show title via regex pattern", "show", cleaned, "filename", filename)
			return cleaned
		}
	}
	
	// Pattern 3: "Show Name.S##E##.Episode.Title"
	dotSeasonRegex := regexp.MustCompile(`^(.+?)\.S\d+E\d+`)
	if matches := dotSeasonRegex.FindStringSubmatch(filename); len(matches) > 1 {
		showName := strings.ReplaceAll(matches[1], ".", " ")
		cleaned := t.cleanupTitle(showName)
		if cleaned != "" {
			t.logger.Debug("extracted show title via dot pattern", "show", cleaned, "filename", filename)
			return cleaned
		}
	}
	
	// Pattern 4: "Show Name - Season ## - Episode Title"
	if strings.Contains(filename, " - Season ") {
		seasonPos := strings.Index(filename, " - Season ")
		if seasonPos > 0 {
			showPart := filename[:seasonPos]
			cleaned := t.cleanupTitle(showPart)
			if cleaned != "" {
				t.logger.Debug("extracted show title via season word pattern", "show", cleaned, "filename", filename)
				return cleaned
			}
		}
	}
	
	// Pattern 5: "Show Name - ##x## - Episode Title" (season x episode format)
	episodeRegex := regexp.MustCompile(`^(.+?)\s+-\s+\d+x\d+`)
	if matches := episodeRegex.FindStringSubmatch(filename); len(matches) > 1 {
		cleaned := t.cleanupTitle(matches[1])
		if cleaned != "" {
			t.logger.Debug("extracted show title via SxE pattern", "show", cleaned, "filename", filename)
			return cleaned
		}
	}
	
	// Pattern 6: Directory-based detection (if the filename doesn't contain show info)
	// Check if the parent directory might be the show name
	parentDir := filepath.Base(filepath.Dir(filePath))
	if parentDir != "" && parentDir != "." && !strings.Contains(parentDir, "Season") {
		// Clean up potential show name from directory
		if cleanDir := t.cleanupTitle(parentDir); cleanDir != "" {
			// Verify this looks like a show name (not just a random directory)
			if len(cleanDir) > 2 && !strings.Contains(strings.ToLower(cleanDir), "season") {
				t.logger.Debug("extracted show title from directory", "show", cleanDir, "directory", parentDir)
				return cleanDir
			}
		}
	}
	
	t.logger.Debug("no TV show pattern matched", "filename", filename)
	return ""
}

// extractMovieTitle handles various movie filename patterns
func (t *TMDbEnricher) extractMovieTitle(filename string) string {
	// Pattern 1: "Movie Title (Year) [Quality Tags]"
	// Pattern 2: "Movie Title Year [Quality Tags]"
	// Pattern 3: "Movie Title [Quality Tags]"
	
	title := filename
	
	// Remove common release group patterns at the end
	releaseGroupRegex := regexp.MustCompile(`-[A-Z][A-Za-z0-9]*$`)
	title = releaseGroupRegex.ReplaceAllString(title, "")
	
	// Remove quality tags in brackets and parentheses (but preserve year)
	qualityRegex := regexp.MustCompile(`\[[^\]]*\]`)
	title = qualityRegex.ReplaceAllString(title, "")
	
	// Remove other quality indicators
	qualityPatterns := []string{
		`\bBluRay\b`, `\bBDRip\b`, `\bBRRip\b`, `\bDVDRip\b`, `\bWEBRip\b`, `\bWEB-DL\b`,
		`\bHDTV\b`, `\bSDTV\b`, `\b720p\b`, `\b1080p\b`, `\b4K\b`, `\bUHD\b`,
		`\bx264\b`, `\bx265\b`, `\bH\.?264\b`, `\bH\.?265\b`, `\bHEVC\b`,
		`\bAAC\b`, `\bAC3\b`, `\bDTS\b`, `\bFLAC\b`, `\bMP3\b`,
		`\b2\.0\b`, `\b5\.1\b`, `\b7\.1\b`,
	}
	
	for _, pattern := range qualityPatterns {
		re := regexp.MustCompile(`(?i)` + pattern)
		title = re.ReplaceAllString(title, "")
	}
	
	// Clean up multiple spaces and trim
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")
	title = strings.TrimSpace(title)
	
	// Handle year extraction and removal for movies
	// Look for year in parentheses at the end: "Movie Title (2023)"
	yearRegex := regexp.MustCompile(`^(.+?)\s*\((\d{4})\)$`)
	if matches := yearRegex.FindStringSubmatch(title); len(matches) > 2 {
		movieTitle := strings.TrimSpace(matches[1])
		if movieTitle != "" {
			return movieTitle
		}
	}
	
	// Look for year without parentheses at the end: "Movie Title 2023"
	yearEndRegex := regexp.MustCompile(`^(.+?)\s+(\d{4})$`)
	if matches := yearEndRegex.FindStringSubmatch(title); len(matches) > 2 {
		year, _ := strconv.Atoi(matches[2])
		if year >= 1900 && year <= time.Now().Year()+5 {
			movieTitle := strings.TrimSpace(matches[1])
			if movieTitle != "" {
				return movieTitle
			}
		}
	}
	
	return t.cleanupTitle(title)
}

// cleanupTitle performs common cleanup operations on titles
func (t *TMDbEnricher) cleanupTitle(title string) string {
	if title == "" {
		return ""
	}
	
	// Remove file extensions that might have been missed
	title = strings.TrimSuffix(title, ".mkv")
	title = strings.TrimSuffix(title, ".mp4")
	title = strings.TrimSuffix(title, ".avi")
	title = strings.TrimSuffix(title, ".mov")
	
	// Remove quality tags in brackets
	qualityRegex := regexp.MustCompile(`\[[^\]]*\]`)
	title = qualityRegex.ReplaceAllString(title, "")
	
	// Remove common release group suffixes
	suffixes := []string{"-Pahe", "-RARBG", "-YTS", "-EZTV", "-TGx", "-BORDURE", "-OFT", "-DUSKLiGHT", "-MaG"}
	for _, suffix := range suffixes {
		if strings.HasSuffix(title, suffix) {
			title = strings.TrimSuffix(title, suffix)
			break
		}
	}
	
	// Remove year in parentheses if present
	if strings.Contains(title, "(") && strings.Contains(title, ")") {
		yearStart := strings.LastIndex(title, "(")
		yearEnd := strings.LastIndex(title, ")")
		if yearEnd > yearStart && yearEnd == len(title)-1 {
			yearStr := title[yearStart+1 : yearEnd]
			if len(yearStr) == 4 {
				if year, err := strconv.Atoi(yearStr); err == nil && year >= 1900 && year <= 2030 {
					title = strings.TrimSpace(title[:yearStart])
				}
			}
		}
	}
	
	// Replace dots and underscores with spaces (common in some naming conventions)
	title = strings.ReplaceAll(title, ".", " ")
	title = strings.ReplaceAll(title, "_", " ")
	
	// Clean up multiple spaces
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")
	
	// Trim and return
	return strings.TrimSpace(title)
}

func (t *TMDbEnricher) extractYear(filePath string, metadata map[string]string) int {
	// Try to get year from metadata first
	if yearStr, exists := metadata["year"]; exists && yearStr != "" {
		if year, err := strconv.Atoi(yearStr); err == nil {
			return year
		}
	}
	
	// Extract from filename using basic pattern matching
	filename := filepath.Base(filePath)
	
	// Look for 4-digit year patterns
	for i := len(filename) - 4; i >= 0; i-- {
		if i+4 <= len(filename) {
			substr := filename[i:i+4]
			if year, err := strconv.Atoi(substr); err == nil {
				if year >= 1900 && year <= time.Now().Year()+5 {
					return year
				}
			}
		}
	}
	
	return 0
}

func (t *TMDbEnricher) searchContent(title string, year int) ([]Result, error) {
	// Check cache first
	queryHash := t.generateQueryHash(fmt.Sprintf("search:%s:%d", title, year))
	if cached, err := t.getCachedResponse("search", queryHash); err == nil {
		return cached, nil
	}
	
	// Build search URL
	baseURL := "https://api.themoviedb.org/3/search/multi"
	params := url.Values{}
	params.Set("query", title)
	params.Set("language", t.config.Language)
	params.Set("region", t.config.Region)
	if year > 0 && t.config.MatchYear {
		params.Set("year", strconv.Itoa(year))
		params.Set("first_air_date_year", strconv.Itoa(year))
	}
	
	searchURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
	
	// Make API request
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+t.config.APIKey)
	req.Header.Set("User-Agent", t.config.UserAgent)
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
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
	
	// Cache the results
	t.cacheResults("search", queryHash, searchResp.Results)
	
	return searchResp.Results, nil
}

func (t *TMDbEnricher) findBestMatch(results []Result, title string, year int) *Result {
	var bestMatch *Result
	bestScore := 0.0
	
	for _, result := range results {
		score := t.calculateMatchScore(result, title, year)
		if score > bestScore && score >= t.config.MatchThreshold {
			bestScore = score
			bestMatch = &result
		}
	}
	
	if bestMatch != nil && t.logger != nil {
		t.logger.Debug("found best match", "title", t.getResultTitle(*bestMatch), "score", bestScore)
	}
	
	return bestMatch
}

func (t *TMDbEnricher) calculateMatchScore(result Result, title string, year int) float64 {
	score := 0.0
	
	// Title matching
	resultTitle := t.getResultTitle(result)
	if strings.EqualFold(resultTitle, title) {
		score += 0.8
	} else if strings.Contains(strings.ToLower(resultTitle), strings.ToLower(title)) {
		score += 0.6
	} else if strings.Contains(strings.ToLower(title), strings.ToLower(resultTitle)) {
		score += 0.4
	}
	
	// Year matching
	if year > 0 && t.config.MatchYear {
		resultYear := t.getResultYear(result)
		if resultYear > 0 {
			yearDiff := abs(year - resultYear)
			if yearDiff == 0 {
				score += 0.2
			} else if yearDiff <= t.config.YearTolerance {
				score += 0.1
			}
		}
	}
	
	return score
}

func (t *TMDbEnricher) getResultTitle(result Result) string {
	if result.Title != "" {
		return result.Title
	}
	return result.Name
}

func (t *TMDbEnricher) getResultYear(result Result) int {
	var dateStr string
	if result.ReleaseDate != "" {
		dateStr = result.ReleaseDate
	} else if result.FirstAirDate != "" {
		dateStr = result.FirstAirDate
	}
	
	if dateStr != "" && len(dateStr) >= 4 {
		if year, err := strconv.Atoi(dateStr[:4]); err == nil {
			return year
		}
	}
	
	return 0
}

func (t *TMDbEnricher) saveEnrichment(mediaFileID string, result *Result) error {
	enrichment := &TMDbEnrichment{
		MediaFileID:      mediaFileID,
		TMDbID:           result.ID,
		EnrichedTitle:    t.getResultTitle(*result),
		EnrichedOverview: result.Overview,
		EnrichedYear:     t.getResultYear(*result),
		EnrichedRating:   result.VoteAverage,
		MatchScore:       t.calculateMatchScore(*result, t.getResultTitle(*result), t.getResultYear(*result)),
		EnrichedAt:       time.Now(),
	}
	
	// Determine media type
	if result.Title != "" || result.ReleaseDate != "" {
		enrichment.MediaType = "movie"
	} else if result.Name != "" || result.FirstAirDate != "" {
		enrichment.MediaType = "tv"
	}
	
	// Set artwork URLs
	if t.config.EnableArtwork {
		if result.PosterPath != "" {
			enrichment.PosterURL = fmt.Sprintf("https://image.tmdb.org/t/p/%s%s", t.config.PosterSize, result.PosterPath)
		}
		if result.BackdropPath != "" {
			enrichment.BackdropURL = fmt.Sprintf("https://image.tmdb.org/t/p/%s%s", t.config.BackdropSize, result.BackdropPath)
		}
	}
	
	// Save to database
	if err := t.db.Save(enrichment).Error; err != nil {
		return fmt.Errorf("failed to save enrichment: %w", err)
	}

	// ALSO save to centralized MediaEnrichment table (for core system integration)
	if err := t.saveToCentralizedSystem(mediaFileID, result, enrichment.MediaType); err != nil {
		t.logger.Warn("Failed to save to centralized enrichment system", "error", err, "media_file_id", mediaFileID)
		// Don't fail the entire operation if centralized save fails
	}

	// Create TV show/movie entities if this is a new match
	if enrichment.MediaType == "tv" {
		if err := t.createTVShowFromFile(mediaFileID, result); err != nil {
			t.logger.Warn("Failed to create TV show entities", "error", err, "media_file_id", mediaFileID)
			// Don't fail the enrichment if entity creation fails
		}
	} else if enrichment.MediaType == "movie" {
		if err := t.createMovieFromFile(mediaFileID, result); err != nil {
			t.logger.Warn("Failed to create movie entity", "error", err, "media_file_id", mediaFileID)
			// Don't fail the enrichment if entity creation fails
		}
	}
	
	t.logger.Info("saved enrichment", 
		"media_file_id", mediaFileID, 
		"tmdb_id", result.ID,
		"title", enrichment.EnrichedTitle,
		"type", enrichment.MediaType,
		"score", enrichment.MatchScore)
	
	return nil
}

// createTVShowFromFile creates TV show, season, and episode records from a TV show file
func (t *TMDbEnricher) createTVShowFromFile(mediaFileID string, result *Result) error {
	// Get the media file to access its path
	var mediaFile struct {
		ID   string `gorm:"column:id"`
		Path string `gorm:"column:path"`
	}
	
	if err := t.db.Table("media_files").Select("id, path").Where("id = ?", mediaFileID).First(&mediaFile).Error; err != nil {
		return fmt.Errorf("failed to get media file: %w", err)
	}
	
	// Parse TV show information from file path
	showInfo := t.parseTVShowFromPath(mediaFile.Path)
	if showInfo == nil {
		return fmt.Errorf("could not parse TV show info from path: %s", mediaFile.Path)
	}
	
	// Create or get TV show
	tvShowID, err := t.createOrGetTVShow(result, showInfo.ShowName)
	if err != nil {
		return fmt.Errorf("failed to create TV show: %w", err)
	}
	
	// Create or get season
	seasonID, err := t.createOrGetSeason(tvShowID, showInfo.SeasonNumber)
	if err != nil {
		return fmt.Errorf("failed to create season: %w", err)
	}
	
	// Create or get episode
	episodeID, err := t.createOrGetEpisode(seasonID, showInfo.EpisodeNumber, showInfo.EpisodeTitle)
	if err != nil {
		return fmt.Errorf("failed to create episode: %w", err)
	}
	
	// Update media file to link to the episode
	if err := t.db.Table("media_files").Where("id = ?", mediaFile.ID).Updates(map[string]interface{}{
		"media_id":   episodeID,
		"media_type": "episode",
	}).Error; err != nil {
		return fmt.Errorf("failed to link media file to episode: %w", err)
	}
	
	t.logger.Info("Created TV show entities", 
		"media_file_id", mediaFileID,
		"tv_show_id", tvShowID,
		"season_id", seasonID,
		"episode_id", episodeID,
		"show_name", showInfo.ShowName,
		"season", showInfo.SeasonNumber,
		"episode", showInfo.EpisodeNumber)
	
	return nil
}

// createMovieFromFile creates a movie record from a movie file
func (t *TMDbEnricher) createMovieFromFile(mediaFileID string, result *Result) error {
	// Get the media file to access its path
	var mediaFile struct {
		ID   string `gorm:"column:id"`
		Path string `gorm:"column:path"`
	}
	
	if err := t.db.Table("media_files").Select("id, path").Where("id = ?", mediaFileID).First(&mediaFile).Error; err != nil {
		return fmt.Errorf("failed to get media file: %w", err)
	}
	
	// Create or get movie
	movieID, err := t.createOrGetMovie(result)
	if err != nil {
		return fmt.Errorf("failed to create movie: %w", err)
	}
	
	// Update media file to link to the movie
	if err := t.db.Table("media_files").Where("id = ?", mediaFile.ID).Updates(map[string]interface{}{
		"media_id":   movieID,
		"media_type": "movie",
	}).Error; err != nil {
		return fmt.Errorf("failed to link media file to movie: %w", err)
	}
	
	t.logger.Info("Created movie entity", 
		"media_file_id", mediaFileID,
		"movie_id", movieID,
		"title", result.Title)
	
	return nil
}

// TVShowInfo holds parsed TV show information
type TVShowInfo struct {
	ShowName      string
	SeasonNumber  int
	EpisodeNumber int
	EpisodeTitle  string
	Year          int
}

// parseTVShowFromPath extracts TV show information from file path
func (t *TMDbEnricher) parseTVShowFromPath(filePath string) *TVShowInfo {
	// Import required packages for regex
	// This is a simplified parser - could be enhanced with more sophisticated regex
	
	// Extract filename from path
	filename := filepath.Base(filePath)
	
	// Try to match common TV show patterns:
	// "Show Name (2024) - S01E01 - Episode Title.mkv"
	// "Show Name - S01E01 - Episode Title.mkv"
	
	// Remove file extension
	nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))
	
	// Find season/episode match
	var seasonNum, episodeNum int
	var showName, episodeTitle string
	
	// Simple parsing - split by " - " and look for patterns
	parts := strings.Split(nameWithoutExt, " - ")
	
	for i, part := range parts {
		// Check if this part contains season/episode info
		part = strings.TrimSpace(part)
		if strings.Contains(strings.ToLower(part), "s") && strings.Contains(strings.ToLower(part), "e") {
			// Try to extract season and episode numbers
			lowerPart := strings.ToLower(part)
			if sIndex := strings.Index(lowerPart, "s"); sIndex >= 0 {
				if eIndex := strings.Index(lowerPart, "e"); eIndex > sIndex {
					// Extract season number
					seasonStr := lowerPart[sIndex+1 : eIndex]
					if s, err := strconv.Atoi(seasonStr); err == nil {
						seasonNum = s
					}
					
					// Extract episode number (find next non-digit or end)
					episodeStart := eIndex + 1
					episodeEnd := episodeStart
					for episodeEnd < len(lowerPart) && lowerPart[episodeEnd] >= '0' && lowerPart[episodeEnd] <= '9' {
						episodeEnd++
					}
					if episodeEnd > episodeStart {
						episodeStr := lowerPart[episodeStart:episodeEnd]
						if e, err := strconv.Atoi(episodeStr); err == nil {
							episodeNum = e
						}
					}
				}
			}
			
			// Show name is everything before this part
			if i > 0 {
				showName = strings.Join(parts[:i], " - ")
			}
			
			// Episode title is everything after this part
			if i < len(parts)-1 {
				episodeTitle = strings.Join(parts[i+1:], " - ")
			}
			break
		}
	}
	
	// If we couldn't parse season/episode, assume it's the show name
	if seasonNum == 0 && episodeNum == 0 {
		showName = nameWithoutExt
		seasonNum = 1
		episodeNum = 1
	}
	
	// Clean up show name - remove year if present
	if showName != "" {
		// Remove year pattern like "(2024)"
		if idx := strings.LastIndex(showName, "("); idx > 0 {
			if idx2 := strings.Index(showName[idx:], ")"); idx2 > 0 {
				yearStr := showName[idx+1 : idx+idx2]
				if _, err := strconv.Atoi(yearStr); err == nil && len(yearStr) == 4 {
					showName = strings.TrimSpace(showName[:idx])
				}
			}
		}
	}
	
	if showName == "" || seasonNum == 0 || episodeNum == 0 {
		return nil
	}
	
	return &TVShowInfo{
		ShowName:      showName,
		SeasonNumber:  seasonNum,
		EpisodeNumber: episodeNum,
		EpisodeTitle:  episodeTitle,
	}
}

// createOrGetTVShow creates or retrieves a TV show record
func (t *TMDbEnricher) createOrGetTVShow(result *Result, showName string) (string, error) {
	// Generate UUID for TV show
	tvShowID := t.generateUUID()
	
	// Check if TV show already exists with this TMDb ID
	var existingShow struct {
		ID string `gorm:"column:id"`
	}
	
	if err := t.db.Table("tv_shows").Select("id").Where("tmdb_id = ?", fmt.Sprintf("%d", result.ID)).First(&existingShow).Error; err == nil {
		return existingShow.ID, nil
	}
	
	// Parse first air date
	var firstAirDate *time.Time
	if result.FirstAirDate != "" {
		if date, err := time.Parse("2006-01-02", result.FirstAirDate); err == nil {
			firstAirDate = &date
		}
	}
	
	// Create TV show record
	tvShow := map[string]interface{}{
		"id":             tvShowID,
		"title":          t.getResultTitle(*result),
		"description":    result.Overview,
		"first_air_date": firstAirDate,
		"status":         "Unknown",
		"tmdb_id":        fmt.Sprintf("%d", result.ID),
		"created_at":     time.Now(),
		"updated_at":     time.Now(),
	}
	
	// Add poster/backdrop if available
	if result.PosterPath != "" {
		tvShow["poster"] = fmt.Sprintf("https://image.tmdb.org/t/p/%s%s", t.config.PosterSize, result.PosterPath)
	}
	if result.BackdropPath != "" {
		tvShow["backdrop"] = fmt.Sprintf("https://image.tmdb.org/t/p/%s%s", t.config.BackdropSize, result.BackdropPath)
	}
	
	if err := t.db.Table("tv_shows").Create(tvShow).Error; err != nil {
		return "", fmt.Errorf("failed to create TV show: %w", err)
	}
	
	return tvShowID, nil
}

// createOrGetSeason creates or retrieves a season record
func (t *TMDbEnricher) createOrGetSeason(tvShowID string, seasonNumber int) (string, error) {
	// Check if season already exists
	var existingSeason struct {
		ID string `gorm:"column:id"`
	}
	
	if err := t.db.Table("seasons").Select("id").Where("tv_show_id = ? AND season_number = ?", tvShowID, seasonNumber).First(&existingSeason).Error; err == nil {
		return existingSeason.ID, nil
	}
	
	// Generate UUID for season
	seasonID := t.generateUUID()
	
	// Create season record
	season := map[string]interface{}{
		"id":            seasonID,
		"tv_show_id":    tvShowID,
		"season_number": seasonNumber,
		"description":   fmt.Sprintf("Season %d", seasonNumber),
		"created_at":    time.Now(),
		"updated_at":    time.Now(),
	}
	
	if err := t.db.Table("seasons").Create(season).Error; err != nil {
		return "", fmt.Errorf("failed to create season: %w", err)
	}
	
	return seasonID, nil
}

// createOrGetEpisode creates or retrieves an episode record
func (t *TMDbEnricher) createOrGetEpisode(seasonID string, episodeNumber int, episodeTitle string) (string, error) {
	// Check if episode already exists
	var existingEpisode struct {
		ID string `gorm:"column:id"`
	}
	
	if err := t.db.Table("episodes").Select("id").Where("season_id = ? AND episode_number = ?", seasonID, episodeNumber).First(&existingEpisode).Error; err == nil {
		return existingEpisode.ID, nil
	}
	
	// Generate UUID for episode
	episodeID := t.generateUUID()
	
	// Use episode title if available, otherwise generate one
	if episodeTitle == "" {
		episodeTitle = fmt.Sprintf("Episode %d", episodeNumber)
	}
	
	// Create episode record
	episode := map[string]interface{}{
		"id":             episodeID,
		"season_id":      seasonID,
		"title":          episodeTitle,
		"episode_number": episodeNumber,
		"created_at":     time.Now(),
		"updated_at":     time.Now(),
	}
	
	if err := t.db.Table("episodes").Create(episode).Error; err != nil {
		return "", fmt.Errorf("failed to create episode: %w", err)
	}
	
	return episodeID, nil
}

// createOrGetMovie creates or retrieves a movie record
func (t *TMDbEnricher) createOrGetMovie(result *Result) (string, error) {
	// Check if movie already exists with this TMDb ID
	var existingMovie struct {
		ID string `gorm:"column:id"`
	}
	
	if err := t.db.Table("movies").Select("id").Where("tmdb_id = ?", fmt.Sprintf("%d", result.ID)).First(&existingMovie).Error; err == nil {
		return existingMovie.ID, nil
	}
	
	// Generate UUID for movie
	movieID := t.generateUUID()
	
	// Parse release date
	var releaseDate *time.Time
	if result.ReleaseDate != "" {
		if date, err := time.Parse("2006-01-02", result.ReleaseDate); err == nil {
			releaseDate = &date
		}
	}
	
	// Create movie record
	movie := map[string]interface{}{
		"id":           movieID,
		"title":        result.Title,
		"plot":         result.Overview,
		"release_date": releaseDate,
		"rating":       result.VoteAverage,
		"tmdb_id":      fmt.Sprintf("%d", result.ID),
		"created_at":   time.Now(),
		"updated_at":   time.Now(),
	}
	
	// Add poster/backdrop if available
	if result.PosterPath != "" {
		movie["poster"] = fmt.Sprintf("https://image.tmdb.org/t/p/%s%s", t.config.PosterSize, result.PosterPath)
	}
	if result.BackdropPath != "" {
		movie["backdrop"] = fmt.Sprintf("https://image.tmdb.org/t/p/%s%s", t.config.BackdropSize, result.BackdropPath)
	}
	
	if err := t.db.Table("movies").Create(movie).Error; err != nil {
		return "", fmt.Errorf("failed to create movie: %w", err)
	}
	
	return movieID, nil
}

// generateUUID generates a robust UUID using Google's UUID library
func (t *TMDbEnricher) generateUUID() string {
	return uuid.New().String()
}

// saveToCentralizedSystem saves enrichment data to the centralized MediaEnrichment table
func (t *TMDbEnricher) saveToCentralizedSystem(mediaFileID string, result *Result, mediaType string) error {
	// Get current time
	now := time.Now()
	
	// Define MediaEnrichment struct to match the existing table structure
	type MediaEnrichment struct {
		MediaID   string    `gorm:"primaryKey;column:media_id"`
		MediaType string    `gorm:"primaryKey;column:media_type"`
		Plugin    string    `gorm:"primaryKey;column:plugin"`
		Payload   string    `gorm:"column:payload"`
		UpdatedAt time.Time `gorm:"column:updated_at"`
	}
	
	// Prepare enrichment payload as JSON
	payload := map[string]interface{}{
		"tmdb_id":     result.ID,
		"title":       t.getResultTitle(*result),
		"overview":    result.Overview,
		"year":        t.getResultYear(*result),
		"rating":      result.VoteAverage,
		"popularity":  result.Popularity,
		"media_type":  mediaType,
		"match_score": t.calculateMatchScore(*result, t.getResultTitle(*result), t.getResultYear(*result)),
	}
	
	// Add type-specific fields
	if mediaType == "movie" {
		payload["release_date"] = result.ReleaseDate
		payload["original_title"] = result.OriginalTitle
	} else if mediaType == "tv" {
		payload["first_air_date"] = result.FirstAirDate
		payload["original_name"] = result.OriginalName
		payload["origin_country"] = result.OriginCountry
	}
	
	// Add artwork URLs if available
	if result.PosterPath != "" {
		payload["poster_url"] = fmt.Sprintf("https://image.tmdb.org/t/p/%s%s", t.config.PosterSize, result.PosterPath)
	}
	if result.BackdropPath != "" {
		payload["backdrop_url"] = fmt.Sprintf("https://image.tmdb.org/t/p/%s%s", t.config.BackdropSize, result.BackdropPath)
	}
	
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	
	// Create enrichment record using the existing table structure
	enrichment := MediaEnrichment{
		MediaID:   mediaFileID, // Use UUID string directly
		MediaType: mediaType,   // "movie", "tv", or "episode"
		Plugin:    "tmdb",
		Payload:   string(payloadJSON),
		UpdatedAt: now,
	}
	
	// Use raw SQL INSERT OR REPLACE since the table doesn't have proper primary key constraints
	result_db := t.db.Exec(`
		INSERT OR REPLACE INTO media_enrichments (media_id, media_type, plugin, payload, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, enrichment.MediaID, enrichment.MediaType, enrichment.Plugin, enrichment.Payload, enrichment.UpdatedAt)
	
	if result_db.Error != nil {
		return fmt.Errorf("failed to save enrichment to centralized system: %w", result_db.Error)
	}
	
	// Also save external IDs to MediaExternalIDs table if it exists
	type MediaExternalIDs struct {
		MediaID      string    `gorm:"column:media_id"`
		MediaType    string    `gorm:"column:media_type"`
		Source       string    `gorm:"column:source"`
		ExternalID   string    `gorm:"column:external_id"`
		CreatedAt    time.Time `gorm:"column:created_at"`
		UpdatedAt    time.Time `gorm:"column:updated_at"`
	}
	
	// Check if MediaExternalIDs table exists
	if t.db.Migrator().HasTable("media_external_ids") {
		externalID := MediaExternalIDs{
			MediaID:    mediaFileID, // Use UUID string directly
			MediaType:  mediaType,   // "movie", "tv", or "episode"
			Source:     "tmdb",
			ExternalID: fmt.Sprintf("%d", result.ID),
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		
		// Save external ID using proper GORM operations
		result_ext := t.db.Table("media_external_ids").Create(&externalID)
		if result_ext.Error != nil {
			// Try update if insert fails (might be duplicate)
			updateResult := t.db.Table("media_external_ids").
				Where("media_id = ? AND media_type = ? AND source = ?", externalID.MediaID, externalID.MediaType, externalID.Source).
				Updates(map[string]interface{}{
					"external_id": externalID.ExternalID,
					"updated_at":  externalID.UpdatedAt,
				})
			if updateResult.Error != nil {
				t.logger.Warn("Failed to save/update external ID", "error", updateResult.Error, "external_id", externalID.ExternalID)
			} else {
				t.logger.Debug("Updated external ID", "external_id", externalID.ExternalID, "media_type", externalID.MediaType)
			}
		} else {
			t.logger.Debug("Saved external ID", "external_id", externalID.ExternalID, "media_type", externalID.MediaType)
		}
	}
	
	t.logger.Info("Successfully saved enrichment to centralized system", 
		"media_file_id", mediaFileID,
		"tmdb_id", result.ID,
		"media_type", mediaType,
		"payload_size", len(payloadJSON))
	
	return nil
}

func (t *TMDbEnricher) generateQueryHash(query string) string {
	hash := md5.Sum([]byte(query))
	return fmt.Sprintf("%x", hash)
}

func (t *TMDbEnricher) getCachedResponse(queryType, queryHash string) ([]Result, error) {
	var cache TMDbCache
	if err := t.db.Where("query_type = ? AND query_hash = ? AND expires_at > ?", 
		queryType, queryHash, time.Now()).First(&cache).Error; err != nil {
		return nil, err
	}
	
	var results []Result
	if err := json.Unmarshal([]byte(cache.Response), &results); err != nil {
		return nil, err
	}
	
	return results, nil
}

func (t *TMDbEnricher) cacheResults(queryType, queryHash string, results []Result) {
	data, err := json.Marshal(results)
	if err != nil {
		t.logger.Error("failed to marshal cache data", "error", err)
		return
	}
	
	cache := &TMDbCache{
		QueryHash: queryHash,
		QueryType: queryType,
		Response:  string(data),
		ExpiresAt: time.Now().Add(time.Duration(t.config.CacheDurationHours) * time.Hour),
	}
	
	t.db.Save(cache)
}

func (t *TMDbEnricher) cleanupCache() {
	result := t.db.Where("expires_at < ?", time.Now()).Delete(&TMDbCache{})
	if result.Error != nil {
		t.logger.Error("failed to cleanup cache", "error", result.Error)
	} else if result.RowsAffected > 0 {
		t.logger.Info("cleaned up expired cache entries", "count", result.RowsAffected)
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func main() {
	plugin := &TMDbEnricher{}
	plugins.StartPlugin(plugin)
}