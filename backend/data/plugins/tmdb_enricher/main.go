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
	pluginID string  // Add pluginID field to store the plugin ID from context
	
	// Host service connections (using unified SDK client)
	hostServiceAddr     string
	unifiedClient       *plugins.UnifiedServiceClient
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
	
	// Basic Info
	EnrichedTitle      string    `json:"enriched_title,omitempty"`
	OriginalTitle      string    `json:"original_title,omitempty"`
	EnrichedOverview   string    `gorm:"type:text" json:"enriched_overview,omitempty"`
	EnrichedYear       int       `json:"enriched_year,omitempty"`
	
	// TV-Specific Fields
	Status             string    `json:"status,omitempty"`              // "Returning Series", "Ended", "Cancelled"
	TVType             string    `json:"tv_type,omitempty"`             // "Scripted", "Reality", "Documentary", etc.
	FirstAirDate       string    `json:"first_air_date,omitempty"`
	LastAirDate        string    `json:"last_air_date,omitempty"`
	InProduction       bool      `json:"in_production,omitempty"`
	NumberOfSeasons    int       `json:"number_of_seasons,omitempty"`
	NumberOfEpisodes   int       `json:"number_of_episodes,omitempty"`
	
	// Companies & Networks
	Networks           string    `gorm:"type:text" json:"networks,omitempty"`           // JSON array of networks
	ProductionCompanies string   `gorm:"type:text" json:"production_companies,omitempty"` // JSON array
	
	// Content & Rating
	EnrichedGenres     string    `json:"enriched_genres,omitempty"`
	Keywords           string    `gorm:"type:text" json:"keywords,omitempty"`          // JSON array
	ContentRatings     string    `gorm:"type:text" json:"content_ratings,omitempty"`  // JSON array
	EnrichedRating     float64   `json:"enriched_rating,omitempty"`
	VoteCount          int       `json:"vote_count,omitempty"`
	Popularity         float64   `json:"popularity,omitempty"`
	
	// Runtime & Technical
	EnrichedRuntime    int       `json:"enriched_runtime,omitempty"`   // Average episode runtime
	EpisodeRunTime     string    `json:"episode_run_time,omitempty"`   // JSON array of runtimes
	
	// Locations & Languages
	OriginCountry      string    `json:"origin_country,omitempty"`     // JSON array
	OriginalLanguage   string    `json:"original_language,omitempty"`
	SpokenLanguages    string    `gorm:"type:text" json:"spoken_languages,omitempty"` // JSON array
	ProductionCountries string   `gorm:"type:text" json:"production_countries,omitempty"` // JSON array
	
	// External IDs
	ExternalIDs        string    `gorm:"type:text" json:"external_ids,omitempty"`     // JSON object with IMDB, TVDB, etc.
	
	// Cast & Crew (top level only to avoid huge data)
	MainCast           string    `gorm:"type:text" json:"main_cast,omitempty"`        // JSON array of main cast
	MainCrew           string    `gorm:"type:text" json:"main_crew,omitempty"`        // JSON array of main crew
	CreatedBy          string    `gorm:"type:text" json:"created_by,omitempty"`       // JSON array of creators
	
	// Artwork
	PosterURL          string    `json:"poster_url,omitempty"`
	BackdropURL        string    `json:"backdrop_url,omitempty"`
	LogoURL            string    `json:"logo_url,omitempty"`
	PosterPath         string    `json:"poster_path,omitempty"`
	BackdropPath       string    `json:"backdrop_path,omitempty"`
	LogoPath           string    `json:"logo_path,omitempty"`
	
	// Season/Episode Specific
	SeasonPosterURL    string    `json:"season_poster_url,omitempty"`
	SeasonOverview     string    `gorm:"type:text" json:"season_overview,omitempty"`
	EpisodeTitle       string    `json:"episode_title,omitempty"`
	EpisodeOverview    string    `gorm:"type:text" json:"episode_overview,omitempty"`
	EpisodeAirDate     string    `json:"episode_air_date,omitempty"`
	EpisodeStillURL    string    `json:"episode_still_url,omitempty"`
	
	// Metadata
	MatchScore         float64   `json:"match_score"`
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

// TMDb API response types for comprehensive TV data
type TVSeriesDetails struct {
	ID                   int                    `json:"id"`
	Name                 string                 `json:"name"`
	OriginalName         string                 `json:"original_name"`
	Overview             string                 `json:"overview"`
	FirstAirDate         string                 `json:"first_air_date"`
	LastAirDate          string                 `json:"last_air_date"`
	Status               string                 `json:"status"`
	Type                 string                 `json:"type"`
	InProduction         bool                   `json:"in_production"`
	NumberOfSeasons      int                    `json:"number_of_seasons"`
	NumberOfEpisodes     int                    `json:"number_of_episodes"`
	EpisodeRunTime       []int                  `json:"episode_run_time"`
	Genres               []Genre                `json:"genres"`
	Networks             []Network              `json:"networks"`
	ProductionCompanies  []ProductionCompany    `json:"production_companies"`
	ProductionCountries  []ProductionCountry    `json:"production_countries"`
	SpokenLanguages      []SpokenLanguage       `json:"spoken_languages"`
	OriginCountry        []string               `json:"origin_country"`
	OriginalLanguage     string                 `json:"original_language"`
	VoteAverage          float64                `json:"vote_average"`
	VoteCount            int                    `json:"vote_count"`
	Popularity           float64                `json:"popularity"`
	PosterPath           string                 `json:"poster_path"`
	BackdropPath         string                 `json:"backdrop_path"`
	CreatedBy            []Creator              `json:"created_by"`
	// Support for append_to_response
	Credits              *Credits               `json:"credits,omitempty"`
	ExternalIDs          *ExternalIDs           `json:"external_ids,omitempty"`
	Keywords             *KeywordsResponse      `json:"keywords,omitempty"`
	ContentRatings       *ContentRatingsResponse `json:"content_ratings,omitempty"`
}

type Network struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	LogoPath      string `json:"logo_path"`
	OriginCountry string `json:"origin_country"`
}

type ProductionCompany struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	LogoPath      string `json:"logo_path"`
	OriginCountry string `json:"origin_country"`
}

type ProductionCountry struct {
	ISO31661 string `json:"iso_3166_1"`
	Name     string `json:"name"`
}

type SpokenLanguage struct {
	ISO6391     string `json:"iso_639_1"`
	EnglishName string `json:"english_name"`
	Name        string `json:"name"`
}

type Creator struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Gender      int    `json:"gender"`
	ProfilePath string `json:"profile_path"`
}

type Credits struct {
	Cast []CastMember `json:"cast"`
	Crew []CrewMember `json:"crew"`
}

type CastMember struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Character   string `json:"character"`
	Order       int    `json:"order"`
	ProfilePath string `json:"profile_path"`
	Gender      int    `json:"gender"`
}

type CrewMember struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Job         string `json:"job"`
	Department  string `json:"department"`
	ProfilePath string `json:"profile_path"`
	Gender      int    `json:"gender"`
}

type ExternalIDs struct {
	IMDBID      string `json:"imdb_id"`
	TVDBID      int    `json:"tvdb_id"`
	WikidataID  string `json:"wikidata_id"`
	FacebookID  string `json:"facebook_id"`
	InstagramID string `json:"instagram_id"`
	TwitterID   string `json:"twitter_id"`
}

type KeywordsResponse struct {
	Results []Keyword `json:"results"`
}

type Keyword struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ContentRatingsResponse struct {
	Results []ContentRating `json:"results"`
}

type ContentRating struct {
	ISO31661 string `json:"iso_3166_1"`
	Rating   string `json:"rating"`
}

// MovieDetails represents comprehensive movie details from TMDb API
type MovieDetails struct {
	ID                  int                    `json:"id"`
	Title               string                 `json:"title"`
	OriginalTitle       string                 `json:"original_title"`
	Overview            string                 `json:"overview"`
	Tagline             string                 `json:"tagline"`
	ReleaseDate         string                 `json:"release_date"`
	Runtime             int                    `json:"runtime"`
	Status              string                 `json:"status"` // Released, In Production, Post Production, etc.
	Adult               bool                   `json:"adult"`
	Video               bool                   `json:"video"`
	Homepage            string                 `json:"homepage"`
	
	// Financial Data
	Budget              int64                  `json:"budget"`
	Revenue             int64                  `json:"revenue"`
	
	// Ratings & Popularity
	VoteAverage         float64                `json:"vote_average"`
	VoteCount           int                    `json:"vote_count"`
	Popularity          float64                `json:"popularity"`
	
	// Language & Region
	OriginalLanguage    string                 `json:"original_language"`
	
	// Structured Data
	Genres              []Genre                `json:"genres"`
	ProductionCompanies []ProductionCompany    `json:"production_companies"`
	ProductionCountries []ProductionCountry    `json:"production_countries"`
	SpokenLanguages     []SpokenLanguage       `json:"spoken_languages"`
	
	// Collection/Franchise
	BelongsToCollection *Collection            `json:"belongs_to_collection"`
	
	// Artwork
	PosterPath          string                 `json:"poster_path"`
	BackdropPath        string                 `json:"backdrop_path"`
	
	// Support for append_to_response (comprehensive data)
	Credits             *Credits               `json:"credits,omitempty"`
	ExternalIDs         *MovieExternalIDs      `json:"external_ids,omitempty"`
	Keywords            *KeywordsResponse      `json:"keywords,omitempty"`
	Releases            *ReleasesResponse      `json:"releases,omitempty"`
	Videos              *VideosResponse        `json:"videos,omitempty"`
	Translations        *TranslationsResponse  `json:"translations,omitempty"`
	Reviews             *ReviewsResponse       `json:"reviews,omitempty"`
	Similar             *MoviesResponse        `json:"similar,omitempty"`
	Recommendations     *MoviesResponse        `json:"recommendations,omitempty"`
}

// Collection represents a movie collection/franchise
type Collection struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Overview     string `json:"overview"`
	PosterPath   string `json:"poster_path"`
	BackdropPath string `json:"backdrop_path"`
}

// MovieExternalIDs extends ExternalIDs with movie-specific fields
type MovieExternalIDs struct {
	IMDBID      string `json:"imdb_id"`
	WikidataID  string `json:"wikidata_id"`
	FacebookID  string `json:"facebook_id"`
	InstagramID string `json:"instagram_id"`
	TwitterID   string `json:"twitter_id"`
}

// ReleasesResponse contains release information by country
type ReleasesResponse struct {
	Countries []CountryRelease `json:"countries"`
}

type CountryRelease struct {
	ISO31661      string           `json:"iso_3166_1"`
	ReleaseDates  []ReleaseDate    `json:"release_dates"`
}

type ReleaseDate struct {
	Certification string `json:"certification"` // Rating like PG-13, R, etc.
	ISO6391       string `json:"iso_639_1"`
	Note          string `json:"note"`
	ReleaseDate   string `json:"release_date"`
	Type          int    `json:"type"` // 1=Premiere, 2=Theatrical (limited), 3=Theatrical, 4=Digital, 5=Physical, 6=TV
}

// VideosResponse contains trailers and other videos
type VideosResponse struct {
	Results []Video `json:"results"`
}

type Video struct {
	ID        string `json:"id"`
	ISO6391   string `json:"iso_639_1"`
	ISO31661  string `json:"iso_3166_1"`
	Name      string `json:"name"`
	Key       string `json:"key"`       // YouTube video key
	Site      string `json:"site"`      // YouTube, Vimeo, etc.
	Type      string `json:"type"`      // Trailer, Teaser, Behind the Scenes, etc.
	Official  bool   `json:"official"`
	Published string `json:"published_at"`
	Size      int    `json:"size"`      // 360, 480, 720, 1080
}

// TranslationsResponse contains movie translations
type TranslationsResponse struct {
	Translations []Translation `json:"translations"`
}

type Translation struct {
	ISO31661    string            `json:"iso_3166_1"`
	ISO6391     string            `json:"iso_639_1"`
	Name        string            `json:"name"`
	EnglishName string            `json:"english_name"`
	Data        TranslationData   `json:"data"`
}

type TranslationData struct {
	Title    string `json:"title"`
	Overview string `json:"overview"`
	Homepage string `json:"homepage"`
	Tagline  string `json:"tagline"`
}

// ReviewsResponse contains movie reviews
type ReviewsResponse struct {
	Page         int      `json:"page"`
	Results      []Review `json:"results"`
	TotalPages   int      `json:"total_pages"`
	TotalResults int      `json:"total_results"`
}

type Review struct {
	ID            string      `json:"id"`
	Author        string      `json:"author"`
	AuthorDetails AuthorInfo  `json:"author_details"`
	Content       string      `json:"content"`
	CreatedAt     string      `json:"created_at"`
	UpdatedAt     string      `json:"updated_at"`
	URL           string      `json:"url"`
}

type AuthorInfo struct {
	Name       string  `json:"name"`
	Username   string  `json:"username"`
	AvatarPath string  `json:"avatar_path"`
	Rating     float64 `json:"rating"`
}

// MoviesResponse for similar/recommendations
type MoviesResponse struct {
	Page         int      `json:"page"`
	Results      []Result `json:"results"`
	TotalPages   int      `json:"total_pages"`
	TotalResults int      `json:"total_results"`
}

// Plugin lifecycle methods

// Initialize implements the plugins.Implementation interface
func (t *TMDbEnricher) Initialize(ctx *plugins.PluginContext) error {
	t.logger = hclog.New(&hclog.LoggerOptions{
		Name:  ctx.PluginID, // Use dynamic plugin ID instead of hard-coded name
		Level: hclog.Debug,
	})
	
	t.logger.Info("TMDb enricher plugin initializing", "database_url", ctx.DatabaseURL, "base_path", ctx.BasePath)
	
	// Store context information
	t.dbURL = ctx.DatabaseURL
	t.basePath = ctx.BasePath
	t.hostServiceAddr = ctx.HostServiceAddr
	t.pluginID = ctx.PluginID  // Store pluginID from context
	
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
		// Initialize Unified Service client
		unifiedClient, err := plugins.NewUnifiedServiceClient(t.hostServiceAddr)
		if err != nil {
			t.logger.Error("Failed to connect to host service", "error", err, "addr", t.hostServiceAddr)
			return fmt.Errorf("failed to connect to host service: %w", err)
		}
		t.unifiedClient = unifiedClient
		
		t.logger.Info("Connected to host services", "addr", t.hostServiceAddr, "services", "unified")
	} else {
		t.logger.Warn("No host service address provided - asset saving and enrichment will be disabled")
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
	// Close unified service connection
	if t.unifiedClient != nil {
		if err := t.unifiedClient.Close(); err != nil {
			t.logger.Warn("Failed to close unified service connection", "error", err)
		} else {
			t.logger.Debug("Closed unified service connection")
		}
	}
	
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
		ID:          t.pluginID,
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
		"plugin":     t.pluginID,
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
	
	// IMPORTANT: Add library type filtering - only process files from appropriate libraries
	// Get the media file to check its library type
	var mediaFile struct {
		ID        string `gorm:"column:id"`
		LibraryID uint32 `gorm:"column:library_id"`
	}
	
	if err := t.db.Table("media_files").Select("id, library_id").Where("id = ?", mediaFileID).First(&mediaFile).Error; err != nil {
		t.logger.Warn("Failed to get media file info for library type check", "error", err, "media_file_id", mediaFileID)
		return nil // Don't fail the scan, just skip enrichment
	}
	
	// Get library information to check type
	var library struct {
		ID   uint32 `gorm:"column:id"`
		Type string `gorm:"column:type"`
		Path string `gorm:"column:path"`
	}
	
	if err := t.db.Table("media_libraries").Select("id, type, path").Where("id = ?", mediaFile.LibraryID).First(&library).Error; err != nil {
		t.logger.Warn("Failed to get library info for type check", "error", err, "library_id", mediaFile.LibraryID)
		return nil // Don't fail the scan, just skip enrichment
	}
	
	// Only process files from movie and tv libraries
	if library.Type != "movie" && library.Type != "tv" {
		t.logger.Debug("Skipping file - not from movie or TV library", 
			"media_file_id", mediaFileID, 
			"library_type", library.Type,
			"library_path", library.Path,
			"file_path", filePath)
		return nil
	}
	
	// Log which library type we're processing
	t.logger.Info("Processing file from supported library", 
		"media_file_id", mediaFileID, 
		"library_type", library.Type,
		"library_id", mediaFile.LibraryID,
		"file_path", filePath)
	
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
	
	// Search for content
	results, err := t.searchContent(title, year)
	if err != nil {
		t.logger.Warn("failed to search for content", "error", err, "title", title)
		return nil
	}
	
	// Find best match with file path context for better TV vs movie classification
	bestMatch := t.findBestMatchWithContext(results, title, year, filePath)
	if bestMatch == nil {
		t.logger.Debug("no suitable match found", "title", title, "threshold", t.config.MatchThreshold)
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
		t.logger.Info("APIRegistrationService: GetRegisteredRoutes called", "plugin_id", t.pluginID)
	}
	
	return []*plugins.APIRoute{
		{
			Method:      "GET",
			Path:        fmt.Sprintf("/api/plugins/%s/search", t.pluginID),
			Description: "Search TMDb for content. Example: ?title=...&year=...&type=...",
		},
		{
			Method:      "GET",
			Path:        fmt.Sprintf("/api/plugins/%s/config", t.pluginID),
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

func (t *TMDbEnricher) findBestMatchWithContext(results []Result, title string, year int, filePath string) *Result {
	var bestMatch *Result
	bestScore := 0.0
	
	// Determine if file path suggests TV show or movie content
	isLikelyTVShow := t.looksLikeEpisodeTitle(title) || 
		strings.Contains(strings.ToLower(filePath), "/tv/") ||
		strings.Contains(strings.ToLower(filePath), "/shows/") ||
		strings.Contains(strings.ToLower(filePath), "/series/") ||
		strings.Contains(strings.ToLower(filePath), "season") ||
		strings.Contains(strings.ToLower(filePath), "episode") ||
		strings.Contains(strings.ToLower(filePath), " s0") ||
		strings.Contains(strings.ToLower(filePath), " s1") ||
		strings.Contains(strings.ToLower(filePath), " s2") ||
		strings.Contains(strings.ToLower(filePath), " e0") ||
		strings.Contains(strings.ToLower(filePath), " e1") ||
		strings.Contains(strings.ToLower(filePath), " e2")
		
	isLikelyMovie := strings.Contains(strings.ToLower(filePath), "/movies/") ||
		strings.Contains(strings.ToLower(filePath), "/films/") ||
		strings.Contains(strings.ToLower(filePath), "/cinema/")
	
	for _, result := range results {
		score := t.calculateMatchScore(result, title, year)
		
		// Add context bonus for media type matching
		if isLikelyTVShow && (result.MediaType == "tv" || result.FirstAirDate != "" || (result.Name != "" && result.Title == "")) {
			score += 0.15 // Prefer TV results for TV-like paths
		} else if isLikelyMovie && (result.MediaType == "movie" || result.ReleaseDate != "" || (result.Title != "" && result.Name == "")) {
			score += 0.15 // Prefer movie results for movie-like paths
		}
		
		// Slight penalty for wrong media type in obvious cases
		if isLikelyTVShow && result.MediaType == "movie" {
			score -= 0.05
		} else if isLikelyMovie && result.MediaType == "tv" {
			score -= 0.05
		}
		
		if score > bestScore && score >= t.config.MatchThreshold {
			bestScore = score
			bestMatch = &result
		}
	}
	
	if bestMatch != nil && t.logger != nil {
		t.logger.Debug("found best context match", "title", t.getResultTitle(*bestMatch), "type", bestMatch.MediaType, "score", bestScore, "tv_likely", isLikelyTVShow, "movie_likely", isLikelyMovie)
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
		VoteCount:        result.VoteCount,
		Popularity:       result.Popularity,
		MatchScore:       t.calculateMatchScore(*result, t.getResultTitle(*result), t.getResultYear(*result)),
		EnrichedAt:       time.Now(),
	}
	
	// Determine media type - fix classification logic
	// First check if MediaType is explicitly set in search results
	if result.MediaType == "tv" {
		enrichment.MediaType = "tv"
		enrichment.OriginalTitle = result.OriginalName
		enrichment.FirstAirDate = result.FirstAirDate
		enrichment.OriginalLanguage = result.OriginalLanguage
	} else if result.MediaType == "movie" {
		enrichment.MediaType = "movie"
		enrichment.OriginalTitle = result.OriginalTitle
	} else {
		// Fallback logic if MediaType not set - be more specific about TV vs Movie indicators
		if (result.Name != "" && result.FirstAirDate != "") || 
		   (result.Name != "" && result.Title == "") {
			// Strong TV indicators: has Name and FirstAirDate, or Name but no Title
			enrichment.MediaType = "tv"
			enrichment.OriginalTitle = result.OriginalName
			enrichment.FirstAirDate = result.FirstAirDate
			enrichment.OriginalLanguage = result.OriginalLanguage
		} else if (result.Title != "" && result.ReleaseDate != "") || 
				  (result.Title != "" && result.Name == "") {
			// Strong movie indicators: has Title and ReleaseDate, or Title but no Name
			enrichment.MediaType = "movie"
			enrichment.OriginalTitle = result.OriginalTitle
		} else {
			// Last resort: prefer TV if we have Name, movie if we have Title
			if result.Name != "" {
				enrichment.MediaType = "tv"
				enrichment.OriginalTitle = result.OriginalName
				enrichment.FirstAirDate = result.FirstAirDate
				enrichment.OriginalLanguage = result.OriginalLanguage
			} else {
				enrichment.MediaType = "movie"
				enrichment.OriginalTitle = result.OriginalTitle
			}
		}
	}
	
	// Fetch comprehensive details for TV shows
	if enrichment.MediaType == "tv" {
		if tvDetails, err := t.fetchTVSeriesDetails(result.ID); err == nil {
			t.populateTVEnrichment(enrichment, tvDetails)
		} else {
			t.logger.Warn("Failed to fetch comprehensive TV details", "error", err, "tmdb_id", result.ID)
		}
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

	// Download artwork assets if enabled and URLs are available
	if t.config.EnableArtwork && t.unifiedClient != nil {
		if err := t.downloadAssets(mediaFileID, enrichment); err != nil {
			t.logger.Warn("Failed to download TMDB assets", "error", err, "media_file_id", mediaFileID)
			// Don't fail the enrichment if asset downloads fail
		}
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
		} else {
			// Process people and roles for TV show
			t.logger.Debug("attempting to process people/roles for TV show", "tmdb_id", result.ID)
			if tvDetails, err := t.fetchTVSeriesDetails(result.ID); err == nil && tvDetails.Credits != nil {
				t.logger.Debug("fetched TV details with credits", "tmdb_id", result.ID, "cast_count", len(tvDetails.Credits.Cast), "crew_count", len(tvDetails.Credits.Crew))
				// Get the TV show ID to link people to
				if tvShowID, err := t.getTVShowIDByTMDbID(result.ID); err == nil {
					t.logger.Debug("got TV show ID for people processing", "tmdb_id", result.ID, "tv_show_id", tvShowID)
					if err := t.processCreditsAndPeople(tvShowID, "tv_show", tvDetails.Credits); err != nil {
						t.logger.Warn("Failed to process TV show people/roles", "error", err, "tmdb_id", result.ID)
					} else {
						t.logger.Info("Successfully processed people/roles for TV show", "tmdb_id", result.ID, "tv_show_id", tvShowID)
					}
				} else {
					t.logger.Warn("Failed to get TV show ID for people processing", "error", err, "tmdb_id", result.ID)
				}
			} else {
				if err != nil {
					t.logger.Warn("Failed to fetch TV series details for people processing", "error", err, "tmdb_id", result.ID)
				} else {
					t.logger.Debug("TV details fetched but no credits found", "tmdb_id", result.ID)
				}
			}
		}
	} else if enrichment.MediaType == "movie" {
		if err := t.createMovieFromFile(mediaFileID, result); err != nil {
			t.logger.Warn("Failed to create movie entity", "error", err, "media_file_id", mediaFileID)
			// Don't fail the enrichment if entity creation fails
		} else {
			// Process people and roles for movie
			if movieDetails, err := t.fetchMovieDetails(result.ID); err == nil {
				// Populate comprehensive movie metadata
				t.populateMovieEnrichment(enrichment, movieDetails)
				
				if movieDetails.Credits != nil {
					// Get the movie ID to link people to
					if movieID, err := t.getMovieIDByTMDbID(result.ID); err == nil {
						if err := t.processCreditsAndPeople(movieID, "movie", movieDetails.Credits); err != nil {
							t.logger.Warn("Failed to process movie people/roles", "error", err, "tmdb_id", result.ID)
						} else {
							t.logger.Info("Successfully processed people/roles for movie", "tmdb_id", result.ID, "movie_id", movieID)
						}
					} else {
						t.logger.Warn("Failed to get movie ID for people processing", "error", err, "tmdb_id", result.ID)
					}
				}
			} else {
				t.logger.Warn("Failed to fetch movie details for comprehensive enrichment", "error", err, "tmdb_id", result.ID)
			}
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

// downloadAssets downloads poster and backdrop assets for TMDB content
func (t *TMDbEnricher) downloadAssets(mediaFileID string, enrichment *TMDbEnrichment) error {
	t.logger.Debug("Starting TMDB asset downloads", "media_file_id", mediaFileID, "media_type", enrichment.MediaType)
	
	var downloadErrors []string
	successCount := 0

	// Download poster if available and enabled
	if t.config.DownloadPosters && enrichment.PosterURL != "" {
		if err := t.downloadAsset(mediaFileID, enrichment.MediaType, "poster", enrichment.PosterURL); err != nil {
			downloadErrors = append(downloadErrors, fmt.Sprintf("poster: %v", err))
			t.logger.Debug("Failed to download poster", "error", err, "media_file_id", mediaFileID)
		} else {
			successCount++
			t.logger.Debug("Successfully downloaded poster", "media_file_id", mediaFileID)
		}
	}

	// Download backdrop if available and enabled
	if t.config.DownloadBackdrops && enrichment.BackdropURL != "" {
		if err := t.downloadAsset(mediaFileID, enrichment.MediaType, "backdrop", enrichment.BackdropURL); err != nil {
			downloadErrors = append(downloadErrors, fmt.Sprintf("backdrop: %v", err))
			t.logger.Debug("Failed to download backdrop", "error", err, "media_file_id", mediaFileID)
		} else {
			successCount++
			t.logger.Debug("Successfully downloaded backdrop", "media_file_id", mediaFileID)
		}
	}

	t.logger.Info("TMDB asset downloads completed", 
		"media_file_id", mediaFileID, 
		"media_type", enrichment.MediaType,
		"success_count", successCount, 
		"error_count", len(downloadErrors))

	// Return error only if all downloads failed and we had URLs to download
	totalAttempts := 0
	if t.config.DownloadPosters && enrichment.PosterURL != "" {
		totalAttempts++
	}
	if t.config.DownloadBackdrops && enrichment.BackdropURL != "" {
		totalAttempts++
	}
	
	if len(downloadErrors) > 0 && successCount == 0 && totalAttempts > 0 {
		return fmt.Errorf("all TMDB asset downloads failed: %s", strings.Join(downloadErrors, "; "))
	}

	return nil
}

// downloadAsset downloads a single asset from TMDB
func (t *TMDbEnricher) downloadAsset(mediaFileID, mediaType, assetType, assetURL string) error {
	if assetURL == "" {
		return fmt.Errorf("no asset URL provided")
	}

	t.logger.Debug("Downloading TMDB asset", "type", assetType, "url", assetURL, "media_file_id", mediaFileID)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Download with retry logic (max 3 attempts)
	var imageData []byte
	var mimeType string
	var downloadErr error
	maxRetries := 3

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			t.logger.Debug("Retrying TMDB asset download", "type", assetType, "attempt", attempt)
			time.Sleep(time.Duration(attempt) * time.Second) // Progressive backoff
		}

		req, err := http.NewRequest("GET", assetURL, nil)
		if err != nil {
			downloadErr = err
			continue
		}
		req.Header.Set("User-Agent", t.config.UserAgent)

		resp, err := client.Do(req)
		if err != nil {
			downloadErr = err
			continue
		}

		if resp.StatusCode == 404 {
			resp.Body.Close()
			return fmt.Errorf("asset not found")
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()
			downloadErr = fmt.Errorf("download failed with status %d", resp.StatusCode)
			continue
		}

		// Check content length (max 10MB)
		maxSize := int64(10 * 1024 * 1024)
		if resp.ContentLength > maxSize {
			resp.Body.Close()
			return fmt.Errorf("asset too large: %d bytes (max: %d)", resp.ContentLength, maxSize)
		}

		// Read the image data
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			downloadErr = err
			continue
		}

		// Check actual size
		if int64(len(data)) > maxSize {
			return fmt.Errorf("asset too large: %d bytes (max: %d)", len(data), maxSize)
		}

		// Get MIME type
		mimeType = resp.Header.Get("Content-Type")
		if mimeType == "" {
			mimeType = "image/jpeg" // Default fallback
		}

		imageData = data
		downloadErr = nil
		break
	}

	if downloadErr != nil {
		return fmt.Errorf("failed after %d attempts: %w", maxRetries+1, downloadErr)
	}

	t.logger.Debug("Downloaded TMDB asset data", "type", assetType, "size", len(imageData), "mime_type", mimeType)

	// Determine the correct asset category and subtype based on media type
	var category, subtype string
	switch assetType {
	case "poster":
		if mediaType == "movie" {
			category = "movie"
			subtype = "poster"
		} else {
			category = "tv"
			subtype = "poster"
		}
	case "backdrop":
		if mediaType == "movie" {
			category = "movie"
			subtype = "backdrop"
		} else {
			category = "tv"
			subtype = "backdrop"
		}
	default:
		category = mediaType
		subtype = assetType
	}

	// Save the asset using the host's AssetService
	metadata := map[string]string{
		"source":     "tmdb",
		"media_type": mediaType,
		"asset_type": assetType,
	}

	return t.saveAssetViaService(mediaFileID, category, subtype, imageData, mimeType, assetURL, metadata)
}

// saveAssetViaService saves an asset using the host's asset service
func (t *TMDbEnricher) saveAssetViaService(mediaFileID, category, subtype string, data []byte, mimeType, sourceURL string, metadata map[string]string) error {
	if t.unifiedClient == nil {
		return fmt.Errorf("asset service not available")
	}

	t.logger.Debug("Saving TMDB asset via host service", 
		"media_file_id", mediaFileID, 
		"category", category,
		"subtype", subtype, 
		"size", len(data), 
		"mime_type", mimeType,
		"source_url", sourceURL)

	// Create save asset request with proper plugin ID
	request := &plugins.SaveAssetRequest{
		MediaFileID: mediaFileID,
		AssetType:   category,
		Category:    category, 
		Subtype:     subtype,
		Data:        data,
		MimeType:    mimeType,
		SourceURL:   sourceURL,
		PluginID:    t.pluginID, // Set the plugin ID for asset tracking
		Metadata:    metadata,
	}

	// Call host asset service
	ctx := context.Background()
	response, err := t.unifiedClient.AssetService().SaveAsset(ctx, request)
	if err != nil {
		t.logger.Error("Failed to save TMDB asset via host service", "error", err, "media_file_id", mediaFileID, "subtype", subtype)
		return fmt.Errorf("failed to save asset: %w", err)
	}

	if !response.Success {
		t.logger.Error("Asset save failed", "error", response.Error, "media_file_id", mediaFileID, "subtype", subtype)
		return fmt.Errorf("asset save failed: %s", response.Error)
	}

	t.logger.Info("Successfully saved TMDB asset", 
		"media_file_id", mediaFileID, 
		"subtype", subtype, 
		"asset_id", response.AssetID,
		"hash", response.Hash,
		"path", response.RelativePath,
		"size", len(data))

	return nil
}

// populateTVEnrichment populates comprehensive TV metadata from TMDb TV details
func (t *TMDbEnricher) populateTVEnrichment(enrichment *TMDbEnrichment, tvDetails *TVSeriesDetails) {
	// Basic TV info
	enrichment.Status = tvDetails.Status
	enrichment.TVType = tvDetails.Type
	enrichment.LastAirDate = tvDetails.LastAirDate
	enrichment.InProduction = tvDetails.InProduction
	enrichment.NumberOfSeasons = tvDetails.NumberOfSeasons
	enrichment.NumberOfEpisodes = tvDetails.NumberOfEpisodes
	
	// Runtime
	if len(tvDetails.EpisodeRunTime) > 0 {
		// Calculate average runtime
		total := 0
		for _, runtime := range tvDetails.EpisodeRunTime {
			total += runtime
		}
		enrichment.EnrichedRuntime = total / len(tvDetails.EpisodeRunTime)
		
		// Store all runtimes as JSON
		if runtimesJSON, err := json.Marshal(tvDetails.EpisodeRunTime); err == nil {
			enrichment.EpisodeRunTime = string(runtimesJSON)
		}
	}
	
	// Genres
	if len(tvDetails.Genres) > 0 {
		genreNames := make([]string, len(tvDetails.Genres))
		for i, genre := range tvDetails.Genres {
			genreNames[i] = genre.Name
		}
		enrichment.EnrichedGenres = strings.Join(genreNames, ", ")
	}
	
	// Networks
	if len(tvDetails.Networks) > 0 {
		if networksJSON, err := json.Marshal(tvDetails.Networks); err == nil {
			enrichment.Networks = string(networksJSON)
		}
	}
	
	// Production Companies
	if len(tvDetails.ProductionCompanies) > 0 {
		if companiesJSON, err := json.Marshal(tvDetails.ProductionCompanies); err == nil {
			enrichment.ProductionCompanies = string(companiesJSON)
		}
	}
	
	// Countries and Languages
	if len(tvDetails.OriginCountry) > 0 {
		if countryJSON, err := json.Marshal(tvDetails.OriginCountry); err == nil {
			enrichment.OriginCountry = string(countryJSON)
		}
	}
	
	if len(tvDetails.SpokenLanguages) > 0 {
		if langJSON, err := json.Marshal(tvDetails.SpokenLanguages); err == nil {
			enrichment.SpokenLanguages = string(langJSON)
		}
	}
	
	if len(tvDetails.ProductionCountries) > 0 {
		if prodCountryJSON, err := json.Marshal(tvDetails.ProductionCountries); err == nil {
			enrichment.ProductionCountries = string(prodCountryJSON)
		}
	}
	
	// Creators
	if len(tvDetails.CreatedBy) > 0 {
		if creatorsJSON, err := json.Marshal(tvDetails.CreatedBy); err == nil {
			enrichment.CreatedBy = string(creatorsJSON)
		}
	}
	
	// External IDs
	if tvDetails.ExternalIDs != nil {
		if externalIDsJSON, err := json.Marshal(tvDetails.ExternalIDs); err == nil {
			enrichment.ExternalIDs = string(externalIDsJSON)
		}
	}
	
	// Keywords
	if tvDetails.Keywords != nil && len(tvDetails.Keywords.Results) > 0 {
		if keywordsJSON, err := json.Marshal(tvDetails.Keywords.Results); err == nil {
			enrichment.Keywords = string(keywordsJSON)
		}
	}
	
	// Content Ratings
	if tvDetails.ContentRatings != nil && len(tvDetails.ContentRatings.Results) > 0 {
		if ratingsJSON, err := json.Marshal(tvDetails.ContentRatings.Results); err == nil {
			enrichment.ContentRatings = string(ratingsJSON)
		}
	}
	
	// Cast & Crew (limit to main cast/crew to avoid huge data)
	if tvDetails.Credits != nil {
		// Main cast (top 10)
		mainCast := tvDetails.Credits.Cast
		if len(mainCast) > 10 {
			mainCast = mainCast[:10]
		}
		if len(mainCast) > 0 {
			if castJSON, err := json.Marshal(mainCast); err == nil {
				enrichment.MainCast = string(castJSON)
			}
		}
		
		// Main crew (directors, writers, producers)
		var mainCrew []CrewMember
		for _, crew := range tvDetails.Credits.Crew {
			if crew.Job == "Director" || crew.Job == "Writer" || crew.Job == "Executive Producer" || 
			   crew.Job == "Producer" || crew.Job == "Creator" {
				mainCrew = append(mainCrew, crew)
			}
		}
		if len(mainCrew) > 0 {
			if crewJSON, err := json.Marshal(mainCrew); err == nil {
				enrichment.MainCrew = string(crewJSON)
			}
		}
	}
	
	t.logger.Info("populated comprehensive TV metadata", 
		"tmdb_id", tvDetails.ID,
		"title", tvDetails.Name,
		"status", tvDetails.Status,
		"seasons", tvDetails.NumberOfSeasons,
		"episodes", tvDetails.NumberOfEpisodes)
}

// processCreditsAndPeople processes cast and crew data to populate people and roles tables
func (t *TMDbEnricher) processCreditsAndPeople(mediaID string, mediaType string, credits *Credits) error {
	t.logger.Debug("processCreditsAndPeople ENTRY", "media_id", mediaID, "media_type", mediaType, "cast_count", len(credits.Cast), "crew_count", len(credits.Crew))
	
	if credits == nil {
		t.logger.Debug("credits is nil, skipping people processing")
		return nil
	}

	// Process cast members
	for _, castMember := range credits.Cast {
		// Create or get person
		personID, err := t.createOrGetPerson(castMember.ID, castMember.Name, castMember.ProfilePath, castMember.Gender)
		if err != nil {
			t.logger.Warn("Failed to create/get cast person", "error", err, "name", castMember.Name)
			continue
		}

		// Create role for actor
		roleDesc := "Actor"
		if castMember.Character != "" {
			roleDesc = fmt.Sprintf("Actor (%s)", castMember.Character)
		}
		
		if err := t.createRole(personID, mediaID, mediaType, roleDesc); err != nil {
			t.logger.Warn("Failed to create cast role", "error", err, "person", castMember.Name, "character", castMember.Character)
		} else {
			t.logger.Debug("Created cast role", "person", castMember.Name, "character", castMember.Character, "person_id", personID)
		}
	}

	// Process crew members
	for _, crewMember := range credits.Crew {
		// Create or get person
		personID, err := t.createOrGetPerson(crewMember.ID, crewMember.Name, crewMember.ProfilePath, crewMember.Gender)
		if err != nil {
			t.logger.Warn("Failed to create/get crew person", "error", err, "name", crewMember.Name)
			continue
		}

		// Create role for crew member
		roleDesc := crewMember.Job
		if crewMember.Department != "" && crewMember.Department != crewMember.Job {
			roleDesc = fmt.Sprintf("%s (%s)", crewMember.Job, crewMember.Department)
		}
		
		if err := t.createRole(personID, mediaID, mediaType, roleDesc); err != nil {
			t.logger.Warn("Failed to create crew role", "error", err, "person", crewMember.Name, "job", crewMember.Job)
		}
	}

	t.logger.Info("processed credits and people", 
		"media_id", mediaID,
		"media_type", mediaType,
		"cast_count", len(credits.Cast),
		"crew_count", len(credits.Crew))

	return nil
}

// createOrGetPerson creates a new person record or returns existing one
func (t *TMDbEnricher) createOrGetPerson(tmdbID int, name, profilePath string, gender int) (string, error) {
	// First check if person already exists by TMDb ID (we'll store this in a custom field)
	// Since the peoples table doesn't have a tmdb_id field, we'll check by name for now
	// TODO: Consider adding tmdb_id field to peoples table for better deduplication
	
	type Person struct {
		ID        string    `gorm:"primaryKey;column:id"`
		Name      string    `gorm:"column:name"`
		Birthdate *time.Time `gorm:"column:birthdate"`
		Image     string    `gorm:"column:image"`
		CreatedAt time.Time `gorm:"column:created_at"`
		UpdatedAt time.Time `gorm:"column:updated_at"`
	}

	var existingPerson Person
	
	// Try to find existing person by name (basic deduplication)
	if err := t.db.Table("peoples").Where("name = ?", name).First(&existingPerson).Error; err == nil {
		// Person exists, update image if we have a better one
		if profilePath != "" && existingPerson.Image == "" {
			imageURL := fmt.Sprintf("https://image.tmdb.org/t/p/w500%s", profilePath)
			t.db.Table("peoples").Where("id = ?", existingPerson.ID).Update("image", imageURL)
		}
		return existingPerson.ID, nil
	}

	// Create new person
	personID := t.generateUUID()
	imageURL := ""
	if profilePath != "" {
		imageURL = fmt.Sprintf("https://image.tmdb.org/t/p/w500%s", profilePath)
	}

	newPerson := Person{
		ID:        personID,
		Name:      name,
		Image:     imageURL,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := t.db.Table("peoples").Create(&newPerson).Error; err != nil {
		return "", fmt.Errorf("failed to create person: %w", err)
	}

	t.logger.Debug("created new person", "id", personID, "name", name, "tmdb_id", tmdbID)
	return personID, nil
}

// createRole creates a role relationship between a person and media
func (t *TMDbEnricher) createRole(personID, mediaID, mediaType, roleDesc string) error {
	type Role struct {
		PersonID  string    `gorm:"column:person_id"`
		MediaID   string    `gorm:"column:media_id"`
		MediaType string    `gorm:"column:media_type"`
		Role      string    `gorm:"column:role"`
		CreatedAt time.Time `gorm:"column:created_at"`
		UpdatedAt time.Time `gorm:"column:updated_at"`
	}

	// Check if role already exists to avoid duplicates
	var existingRole Role
	if err := t.db.Table("roles").Where("person_id = ? AND media_id = ? AND media_type = ? AND role = ?", 
		personID, mediaID, mediaType, roleDesc).First(&existingRole).Error; err == nil {
		// Role already exists
		return nil
	}

	// Create new role
	newRole := Role{
		PersonID:  personID,
		MediaID:   mediaID,
		MediaType: mediaType,
		Role:      roleDesc,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := t.db.Table("roles").Create(&newRole).Error; err != nil {
		return fmt.Errorf("failed to create role: %w", err)
	}

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

// createOrGetMovie creates or retrieves a movie record with comprehensive metadata
func (t *TMDbEnricher) createOrGetMovie(result *Result) (string, error) {
	// Check if movie already exists with this TMDb ID
	var existingMovie struct {
		ID string `gorm:"column:id"`
	}
	
	if err := t.db.Table("movies").Select("id").Where("tmdb_id = ?", fmt.Sprintf("%d", result.ID)).First(&existingMovie).Error; err == nil {
		return existingMovie.ID, nil
	}
	
	// Fetch comprehensive movie details
	movieDetails, err := t.fetchMovieDetails(result.ID)
	if err != nil {
		t.logger.Warn("Failed to fetch comprehensive movie details, using basic info", "tmdb_id", result.ID, "error", err)
		// Fall back to basic movie creation
		return t.createBasicMovie(result)
	}
	
	// Generate UUID for movie
	movieID := t.generateUUID()
	
	// Create comprehensive movie record
	movie, err := t.buildComprehensiveMovieRecord(movieID, movieDetails)
	if err != nil {
		t.logger.Warn("Failed to build comprehensive movie record, using basic info", "tmdb_id", result.ID, "error", err)
		return t.createBasicMovie(result)
	}
	
	if err := t.db.Table("movies").Create(movie).Error; err != nil {
		return "", fmt.Errorf("failed to create comprehensive movie: %w", err)
	}
	
	t.logger.Info("Created comprehensive movie record", 
		"movie_id", movieID,
		"tmdb_id", result.ID,
		"title", movieDetails.Title,
		"budget", movieDetails.Budget,
		"revenue", movieDetails.Revenue)
	
	return movieID, nil
}

// buildComprehensiveMovieRecord builds a complete movie record from TMDb details
func (t *TMDbEnricher) buildComprehensiveMovieRecord(movieID string, details *MovieDetails) (map[string]interface{}, error) {
	now := time.Now()
	
	// Parse release date
	var releaseDate *time.Time
	if details.ReleaseDate != "" {
		if date, err := time.Parse("2006-01-02", details.ReleaseDate); err == nil {
			releaseDate = &date
		}
	}
	
	movie := map[string]interface{}{
		"id":               movieID,
		"title":            details.Title,
		"original_title":   details.OriginalTitle,
		"overview":         details.Overview,
		"tagline":          details.Tagline,
		"release_date":     releaseDate,
		"runtime":          details.Runtime,
		"status":           details.Status,
		"adult":            details.Adult,
		"video":            details.Video,
		"budget":           details.Budget,
		"revenue":          details.Revenue,
		"tmdb_rating":      details.VoteAverage,
		"vote_count":       details.VoteCount,
		"popularity":       details.Popularity,
		"original_language": details.OriginalLanguage,
		"tmdb_id":          fmt.Sprintf("%d", details.ID),
		"created_at":       now,
		"updated_at":       now,
	}
	
	// Add poster/backdrop URLs if available
	if details.PosterPath != "" {
		movie["poster"] = fmt.Sprintf("https://image.tmdb.org/t/p/%s%s", t.config.PosterSize, details.PosterPath)
	}
	if details.BackdropPath != "" {
		movie["backdrop"] = fmt.Sprintf("https://image.tmdb.org/t/p/%s%s", t.config.BackdropSize, details.BackdropPath)
	}
	
	// Process genres
	if len(details.Genres) > 0 {
		if genresJSON, err := json.Marshal(details.Genres); err == nil {
			movie["genres"] = string(genresJSON)
		}
	}
	
	// Process production companies
	if len(details.ProductionCompanies) > 0 {
		if companiesJSON, err := json.Marshal(details.ProductionCompanies); err == nil {
			movie["production_companies"] = string(companiesJSON)
		}
	}
	
	// Process production countries
	if len(details.ProductionCountries) > 0 {
		if countriesJSON, err := json.Marshal(details.ProductionCountries); err == nil {
			movie["production_countries"] = string(countriesJSON)
		}
	}
	
	// Process spoken languages
	if len(details.SpokenLanguages) > 0 {
		if languagesJSON, err := json.Marshal(details.SpokenLanguages); err == nil {
			movie["spoken_languages"] = string(languagesJSON)
		}
	}
	
	// Process keywords
	if details.Keywords != nil && len(details.Keywords.Results) > 0 {
		if keywordsJSON, err := json.Marshal(details.Keywords.Results); err == nil {
			movie["keywords"] = string(keywordsJSON)
		}
	}
	
	// Process collection/franchise information
	if details.BelongsToCollection != nil {
		if collectionJSON, err := json.Marshal(details.BelongsToCollection); err == nil {
			movie["collection"] = string(collectionJSON)
		}
	}
	
	// Process external IDs
	if details.ExternalIDs != nil {
		if externalIDsJSON, err := json.Marshal(details.ExternalIDs); err == nil {
			movie["external_ids"] = string(externalIDsJSON)
		}
		// Set specific external IDs
		if details.ExternalIDs.IMDBID != "" {
			movie["imdb_id"] = details.ExternalIDs.IMDBID
		}
	}
	
	// Process main cast and crew (limit to avoid huge data)
	if details.Credits != nil {
		// Main cast (top 15)
		mainCast := details.Credits.Cast
		if len(mainCast) > 15 {
			mainCast = mainCast[:15]
		}
		if len(mainCast) > 0 {
			if castJSON, err := json.Marshal(mainCast); err == nil {
				movie["main_cast"] = string(castJSON)
			}
		}
		
		// Main crew (directors, writers, producers)
		var mainCrew []CrewMember
		for _, crew := range details.Credits.Crew {
			if crew.Job == "Director" || crew.Job == "Writer" || crew.Job == "Executive Producer" || 
			   crew.Job == "Producer" || crew.Job == "Screenplay" || crew.Job == "Story" {
				mainCrew = append(mainCrew, crew)
			}
		}
		if len(mainCrew) > 0 {
			if crewJSON, err := json.Marshal(mainCrew); err == nil {
				movie["main_crew"] = string(crewJSON)
			}
		}
	}
	
	// Extract rating from releases data
	if details.Releases != nil {
		rating := t.extractUSRating(details.Releases)
		if rating != "" {
			movie["rating"] = rating
		}
	}
	
	return movie, nil
}

// createBasicMovie creates a basic movie record (fallback when comprehensive data fails)
func (t *TMDbEnricher) createBasicMovie(result *Result) (string, error) {
	movieID := t.generateUUID()
	
	// Parse release date
	var releaseDate *time.Time
	if result.ReleaseDate != "" {
		if date, err := time.Parse("2006-01-02", result.ReleaseDate); err == nil {
			releaseDate = &date
		}
	}
	
	// Create basic movie record
	movie := map[string]interface{}{
		"id":           movieID,
		"title":        result.Title,
		"original_title": result.OriginalTitle,
		"overview":     result.Overview,
		"release_date": releaseDate,
		"tmdb_rating":  result.VoteAverage,
		"vote_count":   result.VoteCount,
		"popularity":   result.Popularity,
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
		return "", fmt.Errorf("failed to create basic movie: %w", err)
	}
	
	return movieID, nil
}

// extractUSRating extracts the US movie rating (PG, PG-13, R, etc.) from releases data
func (t *TMDbEnricher) extractUSRating(releases *ReleasesResponse) string {
	for _, country := range releases.Countries {
		if country.ISO31661 == "US" {
			for _, release := range country.ReleaseDates {
				if release.Certification != "" && release.Type == 3 { // Theatrical release
					return release.Certification
				}
			}
		}
	}
	return ""
}

// generateUUID generates a robust UUID using Google's UUID library
func (t *TMDbEnricher) generateUUID() string {
	return uuid.New().String()
}

// saveToCentralizedSystem saves enrichment data to the centralized system via gRPC
func (t *TMDbEnricher) saveToCentralizedSystem(mediaFileID string, result *Result, mediaType string) error {
	if t.unifiedClient == nil {
		t.logger.Warn("Enrichment service not available - cannot save enrichment data", "media_file_id", mediaFileID)
		return fmt.Errorf("enrichment service not available")
	}

	// Create enrichment fields map
	enrichments := make(map[string]string)
	
	// Core fields
	enrichments["tmdb_id"] = fmt.Sprintf("%d", result.ID)
	enrichments["title"] = t.getResultTitle(*result)
	enrichments["overview"] = result.Overview
	enrichments["year"] = fmt.Sprintf("%d", t.getResultYear(*result))
	enrichments["rating"] = fmt.Sprintf("%.2f", result.VoteAverage)
	enrichments["popularity"] = fmt.Sprintf("%.2f", result.Popularity)
	enrichments["media_type"] = mediaType
	enrichments["original_language"] = result.OriginalLanguage

	// Type-specific fields
	if mediaType == "movie" {
		enrichments["release_date"] = result.ReleaseDate
		enrichments["original_title"] = result.OriginalTitle
	} else if mediaType == "tv" {
		enrichments["first_air_date"] = result.FirstAirDate
		enrichments["original_name"] = result.OriginalName
		if len(result.OriginCountry) > 0 {
			enrichments["origin_country"] = strings.Join(result.OriginCountry, ",")
		}
	}

	// Add artwork paths if available
	if result.PosterPath != "" {
		enrichments["poster_path"] = result.PosterPath
		enrichments["poster_url"] = fmt.Sprintf("https://image.tmdb.org/t/p/%s%s", t.config.PosterSize, result.PosterPath)
	}
	if result.BackdropPath != "" {
		enrichments["backdrop_path"] = result.BackdropPath
		enrichments["backdrop_url"] = fmt.Sprintf("https://image.tmdb.org/t/p/%s%s", t.config.BackdropSize, result.BackdropPath)
	}

	// Additional metadata
	matchMetadata := make(map[string]string)
	matchMetadata["source"] = "tmdb"
	matchMetadata["vote_count"] = fmt.Sprintf("%d", result.VoteCount)
	matchMetadata["adult"] = fmt.Sprintf("%t", result.Adult)
	if len(result.GenreIDs) > 0 {
		var genreIDStrs []string
		for _, id := range result.GenreIDs {
			genreIDStrs = append(genreIDStrs, fmt.Sprintf("%d", id))
		}
		matchMetadata["genre_ids"] = strings.Join(genreIDStrs, ",")
	}

	// Calculate match score
	matchScore := t.calculateMatchScore(*result, t.getResultTitle(*result), t.getResultYear(*result))

	// Create RegisterEnrichment request
	request := &plugins.RegisterEnrichmentRequest{
		MediaFileID:     mediaFileID,
		SourceName:      "tmdb",
		Enrichments:     enrichments,
		ConfidenceScore: matchScore,
		MatchMetadata:   matchMetadata,
	}

	t.logger.Info("Sending enrichment to centralized system via gRPC", 
		"media_file_id", mediaFileID,
		"tmdb_id", result.ID,
		"media_type", mediaType,
		"enrichments_count", len(enrichments),
		"confidence_score", request.ConfidenceScore,
		"metadata_count", len(matchMetadata))

	// Call the EnrichmentService via gRPC
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := t.unifiedClient.EnrichmentService().RegisterEnrichment(ctx, request)
	if err != nil {
		t.logger.Error("Failed to register enrichment via gRPC", "error", err, "media_file_id", mediaFileID)
		return fmt.Errorf("failed to register enrichment: %w", err)
	}

	if !response.Success {
		t.logger.Error("Enrichment registration failed", "error", response.Message, "media_file_id", mediaFileID)
		return fmt.Errorf("enrichment registration failed: %s", response.Message)
	}

	t.logger.Info("Successfully registered enrichment via gRPC", 
		"media_file_id", mediaFileID,
		"tmdb_id", result.ID,
		"job_id", response.JobID,
		"source", "tmdb")

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

// fetchTVSeriesDetails fetches comprehensive TV series details from TMDb API
func (t *TMDbEnricher) fetchTVSeriesDetails(tmdbID int) (*TVSeriesDetails, error) {
	// Check cache first
	queryHash := t.generateQueryHash(fmt.Sprintf("tv_details:%d", tmdbID))
	if cached, err := t.getCachedTVDetails("tv_details", queryHash); err == nil {
		return cached, nil
	}
	
	// Build URL with append_to_response for comprehensive data
	baseURL := fmt.Sprintf("https://api.themoviedb.org/3/tv/%d", tmdbID)
	params := url.Values{}
	params.Set("language", t.config.Language)
	// Get comprehensive data in one request
	params.Set("append_to_response", "credits,external_ids,keywords,content_ratings")
	
	detailsURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
	
	// Make API request
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", detailsURL, nil)
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
	
	var tvDetails TVSeriesDetails
	if err := json.Unmarshal(body, &tvDetails); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	// Cache the results
	t.cacheTVDetails("tv_details", queryHash, &tvDetails)
	
	t.logger.Info("fetched comprehensive TV details", "tmdb_id", tmdbID, "title", tvDetails.Name, "seasons", tvDetails.NumberOfSeasons, "episodes", tvDetails.NumberOfEpisodes)
	
	return &tvDetails, nil
}

func (t *TMDbEnricher) getCachedTVDetails(queryType, queryHash string) (*TVSeriesDetails, error) {
	var cache TMDbCache
	if err := t.db.Where("query_type = ? AND query_hash = ? AND expires_at > ?", 
		queryType, queryHash, time.Now()).First(&cache).Error; err != nil {
		return nil, err
	}
	
	var tvDetails TVSeriesDetails
	if err := json.Unmarshal([]byte(cache.Response), &tvDetails); err != nil {
		return nil, err
	}
	
	return &tvDetails, nil
}

func (t *TMDbEnricher) cacheTVDetails(queryType, queryHash string, tvDetails *TVSeriesDetails) {
	data, err := json.Marshal(tvDetails)
	if err != nil {
		t.logger.Error("failed to marshal TV details cache data", "error", err)
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

// getTVShowIDByTMDbID gets the internal TV show ID from TMDb ID
func (t *TMDbEnricher) getTVShowIDByTMDbID(tmdbID int) (string, error) {
	type TVShow struct {
		ID     string `gorm:"column:id"`
		TMDbID string `gorm:"column:tmdb_id"`
	}
	
	var tvShow TVShow
	if err := t.db.Table("tv_shows").Where("tmdb_id = ?", fmt.Sprintf("%d", tmdbID)).First(&tvShow).Error; err != nil {
		return "", fmt.Errorf("TV show not found for TMDb ID %d: %w", tmdbID, err)
	}
	
	return tvShow.ID, nil
}

// getMovieIDByTMDbID gets the internal movie ID from TMDb ID
func (t *TMDbEnricher) getMovieIDByTMDbID(tmdbID int) (string, error) {
	type Movie struct {
		ID     string `gorm:"column:id"`
		TMDbID string `gorm:"column:tmdb_id"`
	}
	
	var movie Movie
	if err := t.db.Table("movies").Where("tmdb_id = ?", fmt.Sprintf("%d", tmdbID)).First(&movie).Error; err != nil {
		return "", fmt.Errorf("Movie not found for TMDb ID %d: %w", tmdbID, err)
	}
	
	return movie.ID, nil
}

// fetchMovieDetails fetches comprehensive movie details from TMDb API
func (t *TMDbEnricher) fetchMovieDetails(tmdbID int) (*MovieDetails, error) {
	queryHash := t.generateQueryHash(fmt.Sprintf("movie_details_%d", tmdbID))
	queryType := "movie_details"
	
	// Check cache first
	if cached, err := t.getCachedMovieDetails(queryType, queryHash); err == nil {
		return cached, nil
	}
	
	// Build comprehensive API URL with append_to_response for all metadata
	baseURL := fmt.Sprintf("https://api.themoviedb.org/3/movie/%d", tmdbID)
	params := url.Values{}
	params.Set("language", t.config.Language)
	// Get comprehensive movie data in one request
	params.Set("append_to_response", "credits,external_ids,keywords,releases,videos,translations,reviews,similar,recommendations")
	
	apiURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
	
	t.logger.Debug("fetching comprehensive movie details", "tmdb_id", tmdbID, "url", apiURL)
	
	// Rate limiting
	time.Sleep(time.Duration(1000/t.config.APIRateLimit) * time.Millisecond)
	
	// Make API request
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
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
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("TMDb API error: %d - %s", resp.StatusCode, string(body))
	}
	
	var movieDetails MovieDetails
	if err := json.NewDecoder(resp.Body).Decode(&movieDetails); err != nil {
		return nil, fmt.Errorf("failed to decode movie details: %w", err)
	}
	
	// Cache the result
	t.cacheMovieDetails(queryType, queryHash, &movieDetails)
	
	t.logger.Debug("fetched comprehensive movie details", 
		"tmdb_id", tmdbID,
		"title", movieDetails.Title,
		"runtime", movieDetails.Runtime,
		"budget", movieDetails.Budget,
		"revenue", movieDetails.Revenue,
		"cast_count", func() int {
			if movieDetails.Credits != nil {
				return len(movieDetails.Credits.Cast)
			}
			return 0
		}(),
		"crew_count", func() int {
			if movieDetails.Credits != nil {
				return len(movieDetails.Credits.Crew)
			}
			return 0
		}())
	
	return &movieDetails, nil
}

// getCachedMovieDetails retrieves cached movie details
func (t *TMDbEnricher) getCachedMovieDetails(queryType, queryHash string) (*MovieDetails, error) {
	var cache TMDbCache
	if err := t.db.Where("query_type = ? AND query_hash = ? AND expires_at > ?", 
		queryType, queryHash, time.Now()).First(&cache).Error; err != nil {
		return nil, err
	}
	
	var movieDetails MovieDetails
	if err := json.Unmarshal([]byte(cache.Response), &movieDetails); err != nil {
		return nil, err
	}
	
	return &movieDetails, nil
}

// cacheMovieDetails caches movie details
func (t *TMDbEnricher) cacheMovieDetails(queryType, queryHash string, movieDetails *MovieDetails) {
	movieDetailsJSON, err := json.Marshal(movieDetails)
	if err != nil {
		t.logger.Warn("Failed to marshal movie details for caching", "error", err)
		return
	}
	
	cache := TMDbCache{
		QueryHash: queryHash,
		QueryType: queryType,
		Response:  string(movieDetailsJSON),
		ExpiresAt: time.Now().Add(time.Duration(t.config.CacheDurationHours) * time.Hour),
	}
	
	t.db.Create(&cache)
}

// populateMovieEnrichment populates comprehensive movie metadata from TMDb movie details
func (t *TMDbEnricher) populateMovieEnrichment(enrichment *TMDbEnrichment, movieDetails *MovieDetails) {
	// Basic movie info
	enrichment.Status = movieDetails.Status
	enrichment.EnrichedRuntime = movieDetails.Runtime
	enrichment.OriginalLanguage = movieDetails.OriginalLanguage
	
	// Financial data (stored in enrichment for analysis)
	if movieDetails.Budget > 0 {
		// Store budget as string in a custom field or use existing field creatively
		enrichment.EpisodeRunTime = fmt.Sprintf("{\"budget\": %d, \"revenue\": %d}", movieDetails.Budget, movieDetails.Revenue)
	}
	
	// Genres
	if len(movieDetails.Genres) > 0 {
		genreNames := make([]string, len(movieDetails.Genres))
		for i, genre := range movieDetails.Genres {
			genreNames[i] = genre.Name
		}
		enrichment.EnrichedGenres = strings.Join(genreNames, ", ")
	}
	
	// Production Companies
	if len(movieDetails.ProductionCompanies) > 0 {
		if companiesJSON, err := json.Marshal(movieDetails.ProductionCompanies); err == nil {
			enrichment.ProductionCompanies = string(companiesJSON)
		}
	}
	
	// Countries and Languages
	if len(movieDetails.ProductionCountries) > 0 {
		if countriesJSON, err := json.Marshal(movieDetails.ProductionCountries); err == nil {
			enrichment.ProductionCountries = string(countriesJSON)
		}
	}
	
	if len(movieDetails.SpokenLanguages) > 0 {
		if languagesJSON, err := json.Marshal(movieDetails.SpokenLanguages); err == nil {
			enrichment.SpokenLanguages = string(languagesJSON)
		}
	}
	
	// External IDs
	if movieDetails.ExternalIDs != nil {
		if externalIDsJSON, err := json.Marshal(movieDetails.ExternalIDs); err == nil {
			enrichment.ExternalIDs = string(externalIDsJSON)
		}
	}
	
	// Keywords
	if movieDetails.Keywords != nil && len(movieDetails.Keywords.Results) > 0 {
		if keywordsJSON, err := json.Marshal(movieDetails.Keywords.Results); err == nil {
			enrichment.Keywords = string(keywordsJSON)
		}
	}
	
	// Content Ratings from releases
	if movieDetails.Releases != nil && len(movieDetails.Releases.Countries) > 0 {
		if ratingsJSON, err := json.Marshal(movieDetails.Releases.Countries); err == nil {
			enrichment.ContentRatings = string(ratingsJSON)
		}
	}
	
	// Cast & Crew (limit to main cast/crew to avoid huge data)
	if movieDetails.Credits != nil {
		// Main cast (top 15)
		mainCast := movieDetails.Credits.Cast
		if len(mainCast) > 15 {
			mainCast = mainCast[:15]
		}
		if len(mainCast) > 0 {
			if castJSON, err := json.Marshal(mainCast); err == nil {
				enrichment.MainCast = string(castJSON)
			}
		}
		
		// Main crew (directors, writers, producers)
		var mainCrew []CrewMember
		for _, crew := range movieDetails.Credits.Crew {
			if crew.Job == "Director" || crew.Job == "Writer" || crew.Job == "Executive Producer" || 
			   crew.Job == "Producer" || crew.Job == "Screenplay" || crew.Job == "Story" {
				mainCrew = append(mainCrew, crew)
			}
		}
		if len(mainCrew) > 0 {
			if crewJSON, err := json.Marshal(mainCrew); err == nil {
				enrichment.MainCrew = string(crewJSON)
			}
		}
	}
	
	// Collection/Franchise info (stored in CreatedBy field for movies since it's not used otherwise)
	if movieDetails.BelongsToCollection != nil {
		if collectionJSON, err := json.Marshal(movieDetails.BelongsToCollection); err == nil {
			enrichment.CreatedBy = string(collectionJSON) // Reuse this field for collection data
		}
	}
	
	// Store additional metadata in Networks field (not used for movies otherwise)
	additionalData := map[string]interface{}{
		"tagline": movieDetails.Tagline,
		"homepage": movieDetails.Homepage,
		"adult": movieDetails.Adult,
		"video": movieDetails.Video,
	}
	
	// Add videos data if available (trailers, etc.)
	if movieDetails.Videos != nil && len(movieDetails.Videos.Results) > 0 {
		// Store first few trailers/videos
		videos := movieDetails.Videos.Results
		if len(videos) > 5 {
			videos = videos[:5] // Limit to 5 videos
		}
		additionalData["videos"] = videos
	}
	
	if additionalJSON, err := json.Marshal(additionalData); err == nil {
		enrichment.Networks = string(additionalJSON) // Reuse Networks field for additional movie data
	}
	
	t.logger.Info("populated comprehensive movie metadata", 
		"tmdb_id", movieDetails.ID,
		"title", movieDetails.Title,
		"budget", movieDetails.Budget,
		"revenue", movieDetails.Revenue,
		"runtime", movieDetails.Runtime)
}

func main() {
	plugin := &TMDbEnricher{}
	plugins.StartPlugin(plugin)
}