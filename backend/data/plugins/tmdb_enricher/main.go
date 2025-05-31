package main

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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

type TMDbEnrichment struct {
	ID                 uint32    `gorm:"primaryKey" json:"id"`
	MediaFileID        uint32    `gorm:"uniqueIndex;not null" json:"media_file_id"`
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
	t.logger = ctx.Logger
	t.basePath = ctx.BasePath
	
	// Initialize with default configuration
	t.config = &Config{
		Enabled:            true,
		APIKey:             "",
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
	
	// Initialize database if provided
	if ctx.DatabaseURL != "" {
		if err := t.initDatabase(ctx.DatabaseURL); err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
	}
	
	t.logger.Info("TMDb enricher initialized", "base_path", t.basePath)
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
	return nil
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
func (t *TMDbEnricher) OnMediaFileScanned(mediaFileID uint32, filePath string, metadata map[string]string) error {
	if !t.config.Enabled || !t.config.AutoEnrich {
		return nil
	}
	
	t.logger.Debug("processing media file", "media_file_id", mediaFileID, "file_path", filePath)
	
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
	
	// Search TMDb for matches
	results, err := t.searchContent(title, year)
	if err != nil {
		return fmt.Errorf("failed to search TMDb: %w", err)
	}
	
	if len(results) == 0 {
		t.logger.Debug("no TMDb match found", "media_file_id", mediaFileID, "title", title)
		return nil
	}
	
	// Find best match
	bestMatch := t.findBestMatch(results, title, year)
	if bestMatch == nil {
		t.logger.Debug("no suitable match found", "media_file_id", mediaFileID)
		return nil
	}
	
	// Save enrichment
	return t.saveEnrichment(uint32(mediaFileID), bestMatch)
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
	t.logger.Info("migrating TMDb enricher database models")
	return t.db.AutoMigrate(&TMDbCache{}, &TMDbEnrichment{})
}

// Rollback implements the plugins.DatabaseService interface
func (t *TMDbEnricher) Rollback(connectionString string) error {
	t.logger.Info("rolling back TMDb enricher database models")
	return t.db.Migrator().DropTable(&TMDbCache{}, &TMDbEnrichment{})
}

// APIRegistrationService implementation

// GetRegisteredRoutes implements the plugins.APIRegistrationService interface
func (t *TMDbEnricher) GetRegisteredRoutes(ctx context.Context) ([]*plugins.APIRoute, error) {
	t.logger.Info("APIRegistrationService: GetRegisteredRoutes called for tmdb_enricher")
	
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

func (t *TMDbEnricher) initDatabase(connectionString string) error {
	gormLogger := logger.Default.LogMode(logger.Silent)
	
	db, err := gorm.Open(sqlite.Open(connectionString), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	
	t.db = db
	
	// Auto-migrate tables
	if err := t.db.AutoMigrate(&TMDbCache{}, &TMDbEnrichment{}); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}
	
	t.logger.Info("database initialized successfully")
	return nil
}

func (t *TMDbEnricher) extractTitle(filePath string, metadata map[string]string) string {
	// Try to get title from metadata first
	if title, exists := metadata["title"]; exists && title != "" {
		return title
	}
	
	// Extract from filename
	filename := filepath.Base(filePath)
	filename = strings.TrimSuffix(filename, filepath.Ext(filename))
	
	// Remove common patterns like year, quality, etc.
	// This is a basic implementation - could be enhanced with regex
	title := filename
	title = strings.TrimSpace(title)
	
	return title
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
	
	if bestMatch != nil {
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

func (t *TMDbEnricher) saveEnrichment(mediaFileID uint32, result *Result) error {
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
	
	t.logger.Info("saved enrichment", 
		"media_file_id", mediaFileID, 
		"tmdb_id", result.ID,
		"title", enrichment.EnrichedTitle,
		"type", enrichment.MediaType,
		"score", enrichment.MatchScore)
	
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