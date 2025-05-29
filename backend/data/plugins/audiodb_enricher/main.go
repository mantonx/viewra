package main

import (
	"context"
	"database/sql/driver"
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
	"gorm.io/gorm"
)

// MediaAssetService defines the interface for asset management operations
// This follows the same pattern as MetadataScraperService
type MediaAssetService interface {
	// SaveAsset saves an asset and returns the asset ID
	SaveAsset(ctx context.Context, request *AssetRequest) (*AssetResponse, error)
	
	// AssetExists checks if an asset already exists for the given parameters
	AssetExists(ctx context.Context, mediaFileID uint, assetType, category string) (bool, error)
	
	// GetAsset retrieves an asset by ID
	GetAsset(ctx context.Context, assetID uint) (*AssetResponse, error)
	
	// DeleteAsset removes an asset
	DeleteAsset(ctx context.Context, assetID uint) error
}

// AssetRequest represents a request to create/save an asset
type AssetRequest struct {
	MediaFileID uint   `json:"media_file_id"`
	Type        string `json:"type"`        // "music", "video", "image"
	Category    string `json:"category"`    // "album", "artist", "track" 
	Subtype     string `json:"subtype"`     // "artwork", "fanart", "logo"
	Data        []byte `json:"data"`        // Binary asset data
	MimeType    string `json:"mime_type"`   // "image/jpeg", "image/png"
	SourceURL   string `json:"source_url"`  // Original download URL
}

// AssetResponse represents the response from asset operations
type AssetResponse struct {
	ID          uint   `json:"id"`
	MediaFileID uint   `json:"media_file_id"`
	Type        string `json:"type"`
	Category    string `json:"category"`
	Subtype     string `json:"subtype"`
	Path        string `json:"path"`
	MimeType    string `json:"mime_type"`
	Size        int64  `json:"size"`
	Hash        string `json:"hash"`
	CreatedAt   int64  `json:"created_at"`
}

// AudioDBEnricher implements multiple plugin interfaces for metadata enrichment
type AudioDBEnricher struct {
	logger           hclog.Logger
	db               *gorm.DB
	config           *AudioDBConfig
	basePath         string
	assetService     MediaAssetService // Service for asset operations
}

// AudioDBConfig holds configuration for the AudioDB plugin
type AudioDBConfig struct {
	Enabled              bool    `json:"enabled"`
	APIKey               string  `json:"api_key,omitempty"`               // AudioDB API key (optional for basic usage)
	UserAgent            string  `json:"user_agent"`                      // User agent for API requests
	EnableArtwork        bool    `json:"enable_artwork"`                  // Whether to download artwork
	ArtworkMaxSize       int     `json:"artwork_max_size"`                // Max artwork size in pixels
	ArtworkQuality       string  `json:"artwork_quality"`                 // front, back, all
	DownloadAlbumArt     bool    `json:"download_album_art"`              // Download album artwork
	DownloadArtistImages bool    `json:"download_artist_images"`          // Download artist images
	PreferHighQuality    bool    `json:"prefer_high_quality"`             // Prefer high quality images
	MaxAssetSize         int64   `json:"max_asset_size"`                  // Max asset file size in bytes (0 = no limit)
	AssetTimeout         int     `json:"asset_timeout_sec"`               // Asset download timeout in seconds
	SkipExistingAssets   bool    `json:"skip_existing_assets"`            // Skip downloading if asset already exists
	RetryFailedDownloads bool    `json:"retry_failed_downloads"`          // Retry failed downloads
	MaxRetries           int     `json:"max_retries"`                     // Maximum number of retry attempts
	MatchThreshold       float64 `json:"match_threshold"`                 // Minimum match score (0.0-1.0)
	AutoEnrich           bool    `json:"auto_enrich"`                     // Auto-enrich during scan
	OverwriteExisting    bool    `json:"overwrite_existing"`              // Overwrite existing metadata
	CacheDurationHours   int     `json:"cache_duration_hours"`            // Cache duration in hours
	RequestDelay         int     `json:"request_delay_ms"`                // Delay between API requests (ms)
}

// Database models for AudioDB plugin
type AudioDBCache struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	SearchQuery string    `gorm:"uniqueIndex;size:255;not null" json:"search_query"`
	APIResponse string    `gorm:"type:longtext" json:"api_response"`
	CachedAt    time.Time `gorm:"not null" json:"cached_at"`
	ExpiresAt   time.Time `gorm:"index;not null" json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type AudioDBEnrichment struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	MediaFileID     uint      `gorm:"uniqueIndex;not null" json:"media_file_id"`
	AudioDBTrackID  string    `gorm:"index" json:"audiodb_track_id,omitempty"`
	AudioDBArtistID string    `gorm:"index" json:"audiodb_artist_id,omitempty"`
	AudioDBAlbumID  string    `gorm:"index" json:"audiodb_album_id,omitempty"`
	EnrichedTitle   string    `json:"enriched_title,omitempty"`
	EnrichedArtist  string    `json:"enriched_artist,omitempty"`
	EnrichedAlbum   string    `json:"enriched_album,omitempty"`
	EnrichedYear    int       `json:"enriched_year,omitempty"`
	EnrichedGenre   string    `json:"enriched_genre,omitempty"`
	MatchScore      float64   `json:"match_score"`
	ArtworkURL      string    `json:"artwork_url,omitempty"`
	ArtworkPath     string    `json:"artwork_path,omitempty"`
	BiographyURL    string    `json:"biography_url,omitempty"`
	EnrichedAt      time.Time `gorm:"not null" json:"enriched_at"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// AudioDB API response structures
type AudioDBArtistResponse struct {
	Artists []AudioDBArtist `json:"artists"`
}

type AudioDBArtist struct {
	IDArtist           string `json:"idArtist"`
	StrArtist          string `json:"strArtist"`
	StrArtistStripped  string `json:"strArtistStripped"`
	StrArtistAlternate string `json:"strArtistAlternate"`
	StrLabel           string `json:"strLabel"`
	IntFormedYear      string `json:"intFormedYear"`
	IntBornYear        string `json:"intBornYear"`
	IntDiedYear        string `json:"intDiedYear"`
	StrDisbanded       string `json:"strDisbanded"`
	StrStyle           string `json:"strStyle"`
	StrGenre           string `json:"strGenre"`
	StrMood            string `json:"strMood"`
	StrWebsite         string `json:"strWebsite"`
	StrFacebook        string `json:"strFacebook"`
	StrTwitter         string `json:"strTwitter"`
	StrBiographyEN     string `json:"strBiographyEN"`
	StrCountry         string `json:"strCountry"`
	StrArtistThumb     string `json:"strArtistThumb"`
	StrArtistLogo      string `json:"strArtistLogo"`
	StrArtistCutout    string `json:"strArtistCutout"`
	StrArtistClearart  string `json:"strArtistClearart"`
	StrArtistWideThumb string `json:"strArtistWideThumb"`
	StrArtistFanart    string `json:"strArtistFanart"`
	StrArtistFanart2   string `json:"strArtistFanart2"`
	StrArtistFanart3   string `json:"strArtistFanart3"`
	StrArtistBanner    string `json:"strArtistBanner"`
	StrMusicBrainzID   string `json:"strMusicBrainzID"`
	StrISNIcode        string `json:"strISNIcode"`
	StrLastFMChart     string `json:"strLastFMChart"`
	IntCharted         string `json:"intCharted"`
	StrLocked          string `json:"strLocked"`
}

type AudioDBAlbumResponse struct {
	Album []AudioDBAlbum `json:"album"`
}

type AudioDBAlbum struct {
	IDAlbum            string `json:"idAlbum"`
	IDArtist           string `json:"idArtist"`
	StrAlbum           string `json:"strAlbum"`
	StrAlbumStripped   string `json:"strAlbumStripped"`
	StrArtist          string `json:"strArtist"`
	StrArtistStripped  string `json:"strArtistStripped"`
	IntYearReleased    string `json:"intYearReleased"`
	StrStyle           string `json:"strStyle"`
	StrGenre           string `json:"strGenre"`
	StrLabel           string `json:"strLabel"`
	StrReleaseFormat   string `json:"strReleaseFormat"`
	IntSales           string `json:"intSales"`
	StrAlbumThumb      string `json:"strAlbumThumb"`
	StrAlbumThumbHQ    string `json:"strAlbumThumbHQ"`
	StrAlbumThumbBack  string `json:"strAlbumThumbBack"`
	StrAlbumCDart      string `json:"strAlbumCDart"`
	StrAlbumSpine      string `json:"strAlbumSpine"`
	StrAlbum3DCase     string `json:"strAlbum3DCase"`
	StrAlbum3DFlat     string `json:"strAlbum3DFlat"`
	StrAlbum3DFace     string `json:"strAlbum3DFace"`
	StrAlbum3DThumb    string `json:"strAlbum3DThumb"`
	StrDescriptionEN   string `json:"strDescriptionEN"`
	IntLoved           string `json:"intLoved"`
	IntScore           string `json:"intScore"`
	IntScoreVotes      string `json:"intScoreVotes"`
	StrReview          string `json:"strReview"`
	StrMood            string `json:"strMood"`
	StrTheme           string `json:"strTheme"`
	StrSpeed           string `json:"strSpeed"`
	StrLocation        string `json:"strLocation"`
	StrMusicBrainzID   string `json:"strMusicBrainzID"`
	StrMusicBrainzArtistID string `json:"strMusicBrainzArtistID"`
	StrAllMusicID      string `json:"strAllMusicID"`
	StrBBCReviewID     string `json:"strBBCReviewID"`
	StrRateYourMusicID string `json:"strRateYourMusicID"`
	StrDiscogsID       string `json:"strDiscogsID"`
	StrWikidataID      string `json:"strWikidataID"`
	StrWikipediaID     string `json:"strWikipediaID"`
	StrGeniusID        string `json:"strGeniusID"`
	StrLyricFind       string `json:"strLyricFind"`
	StrMusicMozID      string `json:"strMusicMozID"`
	StrItunesID        string `json:"strItunesID"`
	StrAmazonID        string `json:"strAmazonID"`
	StrLocked          string `json:"strLocked"`
}

type AudioDBTrackResponse struct {
	Track []AudioDBTrack `json:"track"`
}

type AudioDBTrack struct {
	IDTrack            string `json:"idTrack"`
	IDArtist           string `json:"idArtist"`
	IDAlbum            string `json:"idAlbum"`
	IDIMVDB            string `json:"idIMVDB"`
	IDLyric            string `json:"idLyric"`
	StrTrack           string `json:"strTrack"`
	StrAlbum           string `json:"strAlbum"`
	StrArtist          string `json:"strArtist"`
	StrArtistAlternate string `json:"strArtistAlternate"`
	IntCD              string `json:"intCD"`
	IntTrackNumber     string `json:"intTrackNumber"`
	StrGenre           string `json:"strGenre"`
	StrMood            string `json:"strMood"`
	StrStyle           string `json:"strStyle"`
	StrTheme           string `json:"strTheme"`
	StrDescriptionEN   string `json:"strDescriptionEN"`
	StrTrackLyrics     string `json:"strTrackLyrics"`
	StrMVID            string `json:"strMVID"`
	StrTrackThumb      string `json:"strTrackThumb"`
	StrTrack3DCase     string `json:"strTrack3DCase"`
	IntLoved           string `json:"intLoved"`
	IntScore           string `json:"intScore"`
	IntScoreVotes      string `json:"intScoreVotes"`
	IntDuration        string `json:"intDuration"`
	StrLocked          string `json:"strLocked"`
	StrMusicVid        string `json:"strMusicVid"`
	StrMusicVidDirector string `json:"strMusicVidDirector"`
	StrMusicVidCompany string `json:"strMusicVidCompany"`
	StrMusicVidScreen1 string `json:"strMusicVidScreen1"`
	StrMusicVidScreen2 string `json:"strMusicVidScreen2"`
	StrMusicVidScreen3 string `json:"strMusicVidScreen3"`
	StrMusicBrainzID   string `json:"strMusicBrainzID"`
	StrMusicBrainzAlbumID string `json:"strMusicBrainzAlbumID"`
	StrMusicBrainzArtistID string `json:"strMusicBrainzArtistID"`
	StrLyricFind       string `json:"strLyricFind"`
}

// Initialize implements the Implementation interface
func (a *AudioDBEnricher) Initialize(ctx *proto.PluginContext) error {
	a.logger.Info("Initializing AudioDB enricher plugin", "plugin_id", ctx.PluginId)
	
	// Store base path for file operations
	a.basePath = filepath.Dir(os.Args[0])
	
	// Load default configuration
	a.config = &AudioDBConfig{
		Enabled:              true,
		UserAgent:            "Viewra AudioDB Enricher/1.0.0",
		EnableArtwork:        true,
		ArtworkMaxSize:       1200,
		ArtworkQuality:       "front",
		DownloadAlbumArt:     true,
		DownloadArtistImages: false, // Default to false to avoid too many downloads
		PreferHighQuality:    true,
		MaxAssetSize:         10 * 1024 * 1024, // 10MB limit
		AssetTimeout:         30,                // 30 seconds timeout
		SkipExistingAssets:   true,              // Skip if asset already exists
		RetryFailedDownloads: true,              // Retry failed downloads
		MaxRetries:           3,                 // Maximum 3 retry attempts
		MatchThreshold:       0.75,
		AutoEnrich:           true,
		OverwriteExisting:    false,
		CacheDurationHours:   168, // 1 week
		RequestDelay:         1000, // 1 second between requests
	}
	
	// Apply any configuration overrides from context
	if ctx.Config != nil {
		if enabled, exists := ctx.Config["enabled"]; exists {
			if val, err := strconv.ParseBool(enabled); err == nil {
				a.config.Enabled = val
			}
		}
		if apiKey, exists := ctx.Config["api_key"]; exists {
			a.config.APIKey = apiKey
		}
		if userAgent, exists := ctx.Config["user_agent"]; exists {
			a.config.UserAgent = userAgent
		}
		if enableArtwork, exists := ctx.Config["enable_artwork"]; exists {
			if val, err := strconv.ParseBool(enableArtwork); err == nil {
				a.config.EnableArtwork = val
			}
		}
		if downloadAlbumArt, exists := ctx.Config["download_album_art"]; exists {
			if val, err := strconv.ParseBool(downloadAlbumArt); err == nil {
				a.config.DownloadAlbumArt = val
			}
		}
		if downloadArtistImages, exists := ctx.Config["download_artist_images"]; exists {
			if val, err := strconv.ParseBool(downloadArtistImages); err == nil {
				a.config.DownloadArtistImages = val
			}
		}
		if maxAssetSize, exists := ctx.Config["max_asset_size"]; exists {
			if val, err := strconv.ParseInt(maxAssetSize, 10, 64); err == nil {
				a.config.MaxAssetSize = val
			}
		}
		if assetTimeout, exists := ctx.Config["asset_timeout_sec"]; exists {
			if val, err := strconv.Atoi(assetTimeout); err == nil {
				a.config.AssetTimeout = val
			}
		}
		if skipExisting, exists := ctx.Config["skip_existing_assets"]; exists {
			if val, err := strconv.ParseBool(skipExisting); err == nil {
				a.config.SkipExistingAssets = val
			}
		}
		if retryDownloads, exists := ctx.Config["retry_failed_downloads"]; exists {
			if val, err := strconv.ParseBool(retryDownloads); err == nil {
				a.config.RetryFailedDownloads = val
			}
		}
		if maxRetries, exists := ctx.Config["max_retries"]; exists {
			if val, err := strconv.Atoi(maxRetries); err == nil {
				a.config.MaxRetries = val
			}
		}
		if autoEnrich, exists := ctx.Config["auto_enrich"]; exists {
			if val, err := strconv.ParseBool(autoEnrich); err == nil {
				a.config.AutoEnrich = val
			}
		}
		if matchThreshold, exists := ctx.Config["match_threshold"]; exists {
			if val, err := strconv.ParseFloat(matchThreshold, 64); err == nil {
				a.config.MatchThreshold = val
			}
		}
	}
	
	a.logger.Info("AudioDB enricher configuration loaded", 
		"enabled", a.config.Enabled,
		"auto_enrich", a.config.AutoEnrich,
		"enable_artwork", a.config.EnableArtwork,
		"download_album_art", a.config.DownloadAlbumArt,
		"download_artist_images", a.config.DownloadArtistImages,
		"max_asset_size_mb", a.config.MaxAssetSize/(1024*1024),
		"asset_timeout_sec", a.config.AssetTimeout,
		"skip_existing_assets", a.config.SkipExistingAssets,
		"retry_failed_downloads", a.config.RetryFailedDownloads,
		"max_retries", a.config.MaxRetries,
		"match_threshold", a.config.MatchThreshold)
	
	return nil
}

// Start implements the Implementation interface
func (a *AudioDBEnricher) Start() error {
	a.logger.Info("Starting AudioDB enricher plugin")
	
	if !a.config.Enabled {
		a.logger.Info("AudioDB enricher is disabled")
		return nil
	}
	
	// Initialize database connection via the host application
	if a.db == nil {
		return fmt.Errorf("database connection not available")
	}
	
	// Migrate our tables
	if err := a.Migrate(""); err != nil {
		return fmt.Errorf("failed to migrate AudioDB tables: %w", err)
	}
	
	a.logger.Info("AudioDB enricher plugin started successfully")
	return nil
}

// Stop implements the Implementation interface
func (a *AudioDBEnricher) Stop() error {
	a.logger.Info("Stopping AudioDB enricher plugin")
	return nil
}

// Info implements the Implementation interface
func (a *AudioDBEnricher) Info() (*proto.PluginInfo, error) {
	return &proto.PluginInfo{
		Id:          "audiodb_enricher",
		Name:        "AudioDB Metadata Enricher",
		Version:     "1.0.0",
		Description: "Enriches music metadata using The AudioDB database",
		Author:      "Viewra Team",
		Website:     "https://github.com/mantonx/viewra",
		Repository:  "https://github.com/mantonx/viewra",
		License:     "MIT",
		Type:        "metadata_scraper",
		Tags:        []string{"music", "metadata", "enrichment", "audiodb"},
		Status:      "enabled",
		InstallPath: a.basePath,
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	}, nil
}

// Health implements the Implementation interface
func (a *AudioDBEnricher) Health() error {
	// Check database connection
	if a.db == nil {
		return fmt.Errorf("database connection not available")
	}
	
	// Check AudioDB API connectivity (quick test)
	resp, err := http.Get("https://www.theaudiodb.com/api/v1/json/1/search.php?s=Queen")
	if err != nil {
		return fmt.Errorf("AudioDB API not reachable: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return fmt.Errorf("AudioDB API returned status %d", resp.StatusCode)
	}
	
	return nil
}

// MetadataScraperService returns the metadata scraper implementation
func (a *AudioDBEnricher) MetadataScraperService() plugins.MetadataScraperService {
	return a
}

// ScannerHookService returns the scanner hook implementation
func (a *AudioDBEnricher) ScannerHookService() plugins.ScannerHookService {
	if a.config != nil && a.config.AutoEnrich {
		return a
	}
	return nil
}

// DatabaseService returns the database service implementation
func (a *AudioDBEnricher) DatabaseService() plugins.DatabaseService {
	return a
}

// AdminPageService returns nil as this plugin doesn't provide admin pages
func (a *AudioDBEnricher) AdminPageService() plugins.AdminPageService {
	return nil
}

// APIRegistrationService returns the API registration service implementation
func (a *AudioDBEnricher) APIRegistrationService() plugins.APIRegistrationService {
	return a
}

// SearchService returns the search service implementation
func (a *AudioDBEnricher) SearchService() plugins.SearchService {
	return a
}

// MediaAssetService returns the media asset service implementation
func (a *AudioDBEnricher) MediaAssetService() MediaAssetService {
	return a.assetService
}

// SetMediaAssetService sets the media asset service (dependency injection)
func (a *AudioDBEnricher) SetMediaAssetService(service MediaAssetService) {
	a.assetService = service
}

// Metadata scraper interface implementation
func (a *AudioDBEnricher) CanHandle(filePath, mimeType string) bool {
	if !a.config.Enabled {
		return false
	}
	
	// Check if it's an audio file
	audioTypes := []string{
		"audio/mpeg", "audio/mp3", "audio/flac", "audio/ogg",
		"audio/wav", "audio/aac", "audio/m4a", "audio/wma",
	}
	
	for _, audioType := range audioTypes {
		if strings.Contains(mimeType, audioType) {
			return true
		}
	}
	
	// Check file extension as fallback
	if strings.Contains(filePath, ".") {
		ext := strings.ToLower(filePath[strings.LastIndex(filePath, ".")+1:])
		audioExts := []string{"mp3", "flac", "ogg", "wav", "aac", "m4a", "wma", "opus", "ape"}
		
		for _, audioExt := range audioExts {
			if ext == audioExt {
				return true
			}
		}
	}
	
	return false
}

func (a *AudioDBEnricher) ExtractMetadata(filePath string) (map[string]string, error) {
	if !a.CanHandle(filePath, "") {
		return nil, fmt.Errorf("file type not supported: %s", filePath)
	}
	
	// This plugin enriches existing metadata rather than extracting raw metadata
	return map[string]string{
		"plugin":     "audiodb_enricher",
		"file_path":  filePath,
		"supported":  "true",
		"enrichment": "available",
	}, nil
}

func (a *AudioDBEnricher) GetSupportedTypes() []string {
	return []string{
		"audio/mpeg",
		"audio/flac",
		"audio/ogg",
		"audio/wav",
		"audio/aac",
		"audio/m4a",
		"audio/wma",
		"audio/opus",
		"audio/ape",
	}
}

// Scanner hook interface implementation
func (a *AudioDBEnricher) OnMediaFileScanned(mediaFileID uint32, filePath string, metadata map[string]string) error {
	if !a.config.Enabled || !a.config.AutoEnrich {
		return nil
	}
	
	a.logger.Debug("AudioDB: OnMediaFileScanned called", 
		"media_file_id", mediaFileID, 
		"file_path", filePath,
		"metadata_keys", len(metadata))
	
	// Extract basic metadata for enrichment
	title := metadata["title"]
	artist := metadata["artist"]
	album := metadata["album"]
	
	if title == "" || artist == "" {
		a.logger.Debug("AudioDB: Skipping enrichment - missing title or artist", 
			"file_path", filePath)
		return nil
	}
	
	// Check if already enriched
	var existing AudioDBEnrichment
	err := a.db.Where("media_file_id = ?", mediaFileID).First(&existing).Error
	if err == nil && !a.config.OverwriteExisting {
		a.logger.Debug("AudioDB: Already enriched, skipping", 
			"media_file_id", mediaFileID)
		return nil
	}
	
	// Perform enrichment
	go func() {
		if err := a.enrichTrack(uint(mediaFileID), title, artist, album); err != nil {
			a.logger.Error("AudioDB: Failed to enrich track", 
				"media_file_id", mediaFileID,
				"error", err)
		}
	}()
	
	return nil
}

func (a *AudioDBEnricher) OnScanStarted(scanJobID, libraryID uint32, libraryPath string) error {
	a.logger.Info("AudioDB: Scan started", 
		"scan_job_id", scanJobID,
		"library_id", libraryID,
		"library_path", libraryPath)
	return nil
}

func (a *AudioDBEnricher) OnScanCompleted(scanJobID, libraryID uint32, stats map[string]string) error {
	a.logger.Info("AudioDB: Scan completed", 
		"scan_job_id", scanJobID,
		"library_id", libraryID,
		"stats", stats)
	return nil
}

// Database service implementation
func (a *AudioDBEnricher) GetModels() []string {
	return []string{
		"AudioDBCache",
		"AudioDBEnrichment",
	}
}

func (a *AudioDBEnricher) Migrate(connectionString string) error {
	// Auto-migrate plugin tables
	return a.db.AutoMigrate(&AudioDBCache{}, &AudioDBEnrichment{})
}

func (a *AudioDBEnricher) Rollback(connectionString string) error {
	// Drop plugin tables
	return a.db.Migrator().DropTable(&AudioDBCache{}, &AudioDBEnrichment{})
}

// API registration service implementation
func (a *AudioDBEnricher) GetRegisteredRoutes(ctx context.Context) ([]*proto.APIRoute, error) {
	a.logger.Info("APIRegistrationService: GetRegisteredRoutes called for audiodb_enricher")
	routes := []*proto.APIRoute{
		{
			Path:        "/search",
			Method:      "GET",
			Description: "Search AudioDB for a track. Example: ?title=...&artist=...&album=...",
		},
		{
			Path:        "/config",
			Method:      "GET",
			Description: "Get current AudioDB enricher plugin configuration.",
		},
		{
			Path:        "/enrich/{mediaFileId}",
			Method:      "POST",
			Description: "Manually enrich a specific media file by ID.",
		},
		{
			Path:        "/artist/{artistName}",
			Method:      "GET",
			Description: "Get artist information from AudioDB.",
		},
		{
			Path:        "/album/{artistName}/{albumName}",
			Method:      "GET",
			Description: "Get album information from AudioDB.",
		},
	}
	return routes, nil
}

// Search service implementation
func (a *AudioDBEnricher) Search(ctx context.Context, query map[string]string, limit, offset uint32) ([]*proto.SearchResult, uint32, bool, error) {
	if !a.config.Enabled {
		return nil, 0, false, fmt.Errorf("AudioDB enricher is disabled")
	}
	
	title := query["title"]
	artist := query["artist"]
	album := query["album"]
	
	if title == "" || artist == "" {
		return nil, 0, false, fmt.Errorf("missing required fields: title and artist")
	}
	
	a.logger.Debug("AudioDB: Search request", 
		"title", title,
		"artist", artist,
		"album", album)
	
	// Search for tracks using AudioDB API
	results, err := a.searchTracks(title, artist, album)
	if err != nil {
		return nil, 0, false, fmt.Errorf("search failed: %w", err)
	}
	
	// Convert to proto format
	searchResults := make([]*proto.SearchResult, 0, len(results))
	for _, track := range results {
		score := a.calculateMatchScore(title, artist, album, track.StrTrack, track.StrArtist, track.StrAlbum)
		
		searchResults = append(searchResults, &proto.SearchResult{
			Id:     track.IDTrack,
			Title:  track.StrTrack,
			Artist: track.StrArtist,
			Album:  track.StrAlbum,
			Score:  score,
			Metadata: map[string]string{
				"audiodb_track_id":  track.IDTrack,
				"audiodb_artist_id": track.IDArtist,
				"audiodb_album_id":  track.IDAlbum,
				"genre":             track.StrGenre,
				"mood":              track.StrMood,
				"style":             track.StrStyle,
				"duration":          track.IntDuration,
				"track_number":      track.IntTrackNumber,
			},
		})
	}
	
	// Apply pagination
	totalCount := uint32(len(searchResults))
	hasMore := false
	
	if offset > 0 && offset < uint32(len(searchResults)) {
		searchResults = searchResults[offset:]
	}
	
	if limit > 0 && limit < uint32(len(searchResults)) {
		searchResults = searchResults[:limit]
		hasMore = totalCount > (offset + limit)
	}
	
	return searchResults, totalCount, hasMore, nil
}

func (a *AudioDBEnricher) GetSearchCapabilities(ctx context.Context) ([]string, bool, uint32, error) {
	return []string{"title", "artist", "album", "genre"}, false, 50, nil
}

// Internal helper methods

func (a *AudioDBEnricher) enrichTrack(mediaFileID uint, title, artist, album string) error {
	a.logger.Debug("AudioDB: Starting track enrichment", 
		"media_file_id", mediaFileID,
		"title", title,
		"artist", artist,
		"album", album)
	
	// Add delay to respect API rate limits
	time.Sleep(time.Duration(a.config.RequestDelay) * time.Millisecond)
	
	// Search for tracks
	tracks, err := a.searchTracks(title, artist, album)
	if err != nil {
		return fmt.Errorf("track search failed: %w", err)
	}
	
	if len(tracks) == 0 {
		a.logger.Debug("AudioDB: No tracks found", 
			"title", title,
			"artist", artist)
		return nil
	}
	
	// Find best match
	var bestTrack *AudioDBTrack
	bestScore := 0.0
	
	for _, track := range tracks {
		score := a.calculateMatchScore(title, artist, album, track.StrTrack, track.StrArtist, track.StrAlbum)
		if score > bestScore {
			bestScore = score
			bestTrack = &track
		}
	}
	
	if bestScore < a.config.MatchThreshold {
		a.logger.Debug("AudioDB: No matches above threshold", 
			"best_score", bestScore,
			"threshold", a.config.MatchThreshold)
		return nil
	}
	
	// Create enrichment record
	enrichment := AudioDBEnrichment{
		MediaFileID:     mediaFileID,
		AudioDBTrackID:  bestTrack.IDTrack,
		AudioDBArtistID: bestTrack.IDArtist,
		AudioDBAlbumID:  bestTrack.IDAlbum,
		EnrichedTitle:   bestTrack.StrTrack,
		EnrichedArtist:  bestTrack.StrArtist,
		EnrichedAlbum:   bestTrack.StrAlbum,
		EnrichedGenre:   bestTrack.StrGenre,
		MatchScore:      bestScore,
		EnrichedAt:      time.Now(),
	}
	
	// Parse year if available (from album)
	if bestTrack.IDAlbum != "" {
		if albumInfo, err := a.getAlbumInfo(bestTrack.IDAlbum); err == nil && len(albumInfo.Album) > 0 {
			if year, err := strconv.Atoi(albumInfo.Album[0].IntYearReleased); err == nil {
				enrichment.EnrichedYear = year
			}
			
			// Store the artwork URL for reference
			if a.config.EnableArtwork && albumInfo.Album[0].StrAlbumThumb != "" {
				enrichment.ArtworkURL = albumInfo.Album[0].StrAlbumThumb
			}
			
			// Download and store album artwork if enabled
			if a.config.DownloadAlbumArt && a.config.EnableArtwork {
				go func(album AudioDBAlbum) {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					
					if err := a.downloadAlbumArtwork(ctx, album, mediaFileID); err != nil {
						a.logger.Warn("Failed to download album artwork", "error", err, "media_file_id", mediaFileID)
					}
				}(albumInfo.Album[0])
			}
		}
	}
	
	// Download artist images if enabled
	if a.config.DownloadArtistImages && a.config.EnableArtwork {
		// First we need to get the artist information
		go func(artistID string) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			
			if artist, err := a.getArtistInfo(artistID); err == nil && len(artist.Artists) > 0 {
				if err := a.downloadArtistImages(ctx, artist.Artists[0], mediaFileID); err != nil {
					a.logger.Warn("Failed to download artist images", "error", err, "media_file_id", mediaFileID)
				}
			}
		}(bestTrack.IDArtist)
	}
	
	// Save enrichment
	if err := a.db.Save(&enrichment).Error; err != nil {
		return fmt.Errorf("failed to save enrichment: %w", err)
	}
	
	a.logger.Info("AudioDB: Track enriched successfully", 
		"media_file_id", mediaFileID,
		"match_score", bestScore,
		"audiodb_track_id", bestTrack.IDTrack)
	
	return nil
}

func (a *AudioDBEnricher) searchTracks(title, artist, album string) ([]AudioDBTrack, error) {
	// Create cache key
	cacheKey := fmt.Sprintf("track:%s:%s:%s", 
		strings.ToLower(title),
		strings.ToLower(artist),
		strings.ToLower(album))
	
	// Check cache first
	var cached AudioDBCache
	err := a.db.Where("search_query = ? AND expires_at > ?", cacheKey, time.Now()).First(&cached).Error
	if err == nil {
		a.logger.Debug("AudioDB: Using cached result", "cache_key", cacheKey)
		var response AudioDBTrackResponse
		if err := json.Unmarshal([]byte(cached.APIResponse), &response); err == nil {
			return response.Track, nil
		}
	}

	// Apply rate limiting
	if a.config.RequestDelay > 0 {
		time.Sleep(time.Duration(a.config.RequestDelay) * time.Millisecond)
	}

	// Make API request
	apiURL := "https://www.theaudiodb.com/api/v1/json"
	if a.config.APIKey != "" {
		apiURL = fmt.Sprintf("https://www.theaudiodb.com/api/v1/json/%s", a.config.APIKey)
	} else {
		apiURL = "https://www.theaudiodb.com/api/v1/json/1"
	}
	
	// Search by artist first to get artist info
	// Normalize artist name for AudioDB (case sensitive)
	normalizedArtist := strings.ToLower(strings.TrimSpace(artist))
	searchURL := fmt.Sprintf("%s/search.php?s=%s", apiURL, url.QueryEscape(normalizedArtist))
	
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("User-Agent", a.config.UserAgent)
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	// Read response body first
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read artist response body: %w", err)
	}
	
	// Debug logging for response content
	previewLength := 500
	if len(bodyBytes) < 500 {
		previewLength = len(bodyBytes)
	}
	a.logger.Debug("AudioDB artist search response", 
		"url", searchURL,
		"status", resp.StatusCode,
		"content_length", len(bodyBytes),
		"content_preview", string(bodyBytes[:previewLength]))
	
	var artistResponse AudioDBArtistResponse
	if err := json.Unmarshal(bodyBytes, &artistResponse); err != nil {
		a.logger.Error("Failed to decode artist response", 
			"error", err,
			"response_length", len(bodyBytes),
			"response_content", string(bodyBytes))
		return nil, fmt.Errorf("failed to decode artist response: %w", err)
	}
	
	if len(artistResponse.Artists) == 0 {
		return []AudioDBTrack{}, nil
	}
	
	// Rate limiting for second request
	if a.config.RequestDelay > 0 {
		time.Sleep(time.Duration(a.config.RequestDelay) * time.Millisecond)
	}
	
	// Get all albums for the artist first
	artistID := artistResponse.Artists[0].IDArtist
	albumsURL := fmt.Sprintf("%s/album.php?i=%s", apiURL, artistID)
	
	req, err = http.NewRequest("GET", albumsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create albums request: %w", err)
	}
	
	req.Header.Set("User-Agent", a.config.UserAgent)
	
	resp, err = client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("albums API request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("albums API returned status %d", resp.StatusCode)
	}
	
	// Read response body first  
	bodyBytes, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read albums response body: %w", err)
	}
	
	var albumResponse AudioDBAlbumResponse
	if err := json.Unmarshal(bodyBytes, &albumResponse); err != nil {
		return nil, fmt.Errorf("failed to decode albums response: %w", err)
	}
	
	var allTracks []AudioDBTrack
	
	// Get tracks from each album
	for _, albumData := range albumResponse.Album {
		// Rate limiting for each album request
		if a.config.RequestDelay > 0 {
			time.Sleep(time.Duration(a.config.RequestDelay) * time.Millisecond)
		}
		
		tracksURL := fmt.Sprintf("%s/track.php?m=%s", apiURL, albumData.IDAlbum)
		
		req, err = http.NewRequest("GET", tracksURL, nil)
		if err != nil {
			a.logger.Warn("Failed to create tracks request", "album_id", albumData.IDAlbum, "error", err)
			continue
		}
		
		req.Header.Set("User-Agent", a.config.UserAgent)
		
		resp, err = client.Do(req)
		if err != nil {
			a.logger.Warn("Tracks API request failed", "album_id", albumData.IDAlbum, "error", err)
			continue
		}
		
		if resp.StatusCode != 200 {
			resp.Body.Close()
			a.logger.Warn("Tracks API returned non-200 status", "album_id", albumData.IDAlbum, "status", resp.StatusCode)
			continue
		}
		
		// Read tracks response body first
		trackBodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			a.logger.Warn("Failed to read tracks response body", "album_id", albumData.IDAlbum, "error", err)
			continue
		}
		
		var trackResponse AudioDBTrackResponse
		if err := json.Unmarshal(trackBodyBytes, &trackResponse); err != nil {
			a.logger.Warn("Failed to decode tracks response", "album_id", albumData.IDAlbum, "error", err)
			continue
		}
		
		// Add tracks from this album
		allTracks = append(allTracks, trackResponse.Track...)
	}
	
	// Cache the result
	trackResult := AudioDBTrackResponse{Track: allTracks}
	responseBytes, _ := json.Marshal(trackResult)
	cacheEntry := AudioDBCache{
		SearchQuery: cacheKey,
		APIResponse: string(responseBytes),
		CachedAt:    time.Now(),
		ExpiresAt:   time.Now().Add(time.Duration(a.config.CacheDurationHours) * time.Hour),
	}
	a.db.Save(&cacheEntry)
	
	return allTracks, nil
}

func (a *AudioDBEnricher) getAlbumInfo(albumID string) (*AudioDBAlbumResponse, error) {
	apiURL := "https://www.theaudiodb.com/api/v1/json"
	if a.config.APIKey != "" {
		apiURL = fmt.Sprintf("https://www.theaudiodb.com/api/v1/json/%s", a.config.APIKey)
	} else {
		apiURL = "https://www.theaudiodb.com/api/v1/json/1"
	}
	
	searchURL := fmt.Sprintf("%s/album.php?m=%s", apiURL, albumID)
	
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("User-Agent", a.config.UserAgent)
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	var albumResponse AudioDBAlbumResponse
	if err := json.NewDecoder(resp.Body).Decode(&albumResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return &albumResponse, nil
}

// getArtistInfo retrieves detailed artist information from AudioDB
func (a *AudioDBEnricher) getArtistInfo(artistID string) (*AudioDBArtistResponse, error) {
	apiURL := "https://www.theaudiodb.com/api/v1/json"
	if a.config.APIKey != "" {
		apiURL = fmt.Sprintf("https://www.theaudiodb.com/api/v1/json/%s", a.config.APIKey)
	} else {
		apiURL = "https://www.theaudiodb.com/api/v1/json/1"
	}
	
	searchURL := fmt.Sprintf("%s/artist.php?i=%s", apiURL, artistID)
	
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("User-Agent", a.config.UserAgent)
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	var artistResponse AudioDBArtistResponse
	if err := json.NewDecoder(resp.Body).Decode(&artistResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return &artistResponse, nil
}

// downloadAlbumArtwork downloads various types of album artwork
func (a *AudioDBEnricher) downloadAlbumArtwork(ctx context.Context, album AudioDBAlbum, mediaFileID uint) error {
	a.logger.Debug("Downloading album artwork", "album", album.StrAlbum, "media_file_id", mediaFileID)
	
	// Collect all available artwork URLs
	artworkURLs := make(map[string]string)
	
	// Prefer high quality if enabled
	if a.config.PreferHighQuality && album.StrAlbumThumbHQ != "" {
		artworkURLs["album_thumb_hq"] = album.StrAlbumThumbHQ
	} else if album.StrAlbumThumb != "" {
		artworkURLs["album_thumb"] = album.StrAlbumThumb
	}
	
	// Add other artwork types based on quality setting
	if a.config.ArtworkQuality == "all" || a.config.ArtworkQuality == "back" {
		if album.StrAlbumThumbBack != "" {
			artworkURLs["album_back"] = album.StrAlbumThumbBack
		}
	}
	
	if a.config.ArtworkQuality == "all" {
		if album.StrAlbumCDart != "" {
			artworkURLs["album_cd"] = album.StrAlbumCDart
		}
		if album.StrAlbum3DCase != "" {
			artworkURLs["album_3d"] = album.StrAlbum3DCase
		}
	}
	
	// Download each artwork type
	var downloadedCount int
	for artworkType, imageURL := range artworkURLs {
		if err := a.downloadAndStoreImage(ctx, imageURL, mediaFileID, 
			"music", "album", artworkType); err != nil {
			a.logger.Warn("Failed to download album artwork", 
				"type", artworkType, "url", imageURL, "error", err)
		} else {
			downloadedCount++
		}
		
		// Add delay between downloads
		if a.config.RequestDelay > 0 {
			time.Sleep(time.Duration(a.config.RequestDelay) * time.Millisecond)
		}
	}
	
	if downloadedCount > 0 {
		a.logger.Info("Successfully downloaded album artwork", 
			"album", album.StrAlbum, "count", downloadedCount, "media_file_id", mediaFileID)
	}
	
	return nil
}

// downloadArtistImages downloads various types of artist images
func (a *AudioDBEnricher) downloadArtistImages(ctx context.Context, artist AudioDBArtist, mediaFileID uint) error {
	a.logger.Debug("Downloading artist images", "artist", artist.StrArtist, "media_file_id", mediaFileID)
	
	// Collect all available image URLs
	imageURLs := make(map[string]string)
	
	if artist.StrArtistThumb != "" {
		imageURLs["artist_thumb"] = artist.StrArtistThumb
	}
	if artist.StrArtistLogo != "" {
		imageURLs["artist_logo"] = artist.StrArtistLogo
	}
	if artist.StrArtistFanart != "" {
		imageURLs["artist_fanart"] = artist.StrArtistFanart
	}
	if artist.StrArtistFanart2 != "" {
		imageURLs["artist_fanart2"] = artist.StrArtistFanart2
	}
	if artist.StrArtistFanart3 != "" {
		imageURLs["artist_fanart3"] = artist.StrArtistFanart3
	}
	if artist.StrArtistBanner != "" {
		imageURLs["artist_banner"] = artist.StrArtistBanner
	}
	
	// Download each image type
	var downloadedCount int
	for imageType, imageURL := range imageURLs {
		if err := a.downloadAndStoreImage(ctx, imageURL, mediaFileID, 
			"music", "artist", imageType); err != nil {
			a.logger.Warn("Failed to download artist image", 
				"type", imageType, "url", imageURL, "error", err)
		} else {
			downloadedCount++
		}
		
		// Add delay between downloads
		if a.config.RequestDelay > 0 {
			time.Sleep(time.Duration(a.config.RequestDelay) * time.Millisecond)
		}
	}
	
	if downloadedCount > 0 {
		a.logger.Info("Successfully downloaded artist images", 
			"artist", artist.StrArtist, "count", downloadedCount, "media_file_id", mediaFileID)
	}
	
	return nil
}

// downloadAndStoreImage downloads an image from URL and stores it using the MediaAssetService
func (a *AudioDBEnricher) downloadAndStoreImage(ctx context.Context, imageURL string, mediaFileID uint, assetType, category, imageType string) error {
	if imageURL == "" {
		return fmt.Errorf("image URL is empty")
	}

	// Check if we should skip existing assets
	if a.config.SkipExistingAssets {
		if a.assetService == nil {
			a.logger.Warn("MediaAssetService not available, skipping asset existence check")
		} else {
			exists, err := a.assetService.AssetExists(ctx, mediaFileID, assetType, category)
			if err != nil {
				a.logger.Debug("Failed to check for existing asset", "error", err)
			} else if exists {
				a.logger.Debug("Asset already exists, skipping download", 
					"media_file_id", mediaFileID, "type", assetType, "category", category)
				return nil
			}
		}
	}

	// Download the image with retry logic
	var imageData []byte
	var mimeType string
	var err error
	
	maxRetries := 1
	if a.config.RetryFailedDownloads {
		maxRetries = a.config.MaxRetries
	}
	
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			a.logger.Debug("Retrying image download", "attempt", attempt+1, "url", imageURL)
			// Add exponential backoff delay
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}
		
		// Create context with timeout
		downloadCtx := ctx
		if a.config.AssetTimeout > 0 {
			var cancel context.CancelFunc
			downloadCtx, cancel = context.WithTimeout(ctx, time.Duration(a.config.AssetTimeout)*time.Second)
			defer cancel()
		}
		
		imageData, mimeType, err = a.downloadImageFromURL(downloadCtx, imageURL)
		if err == nil {
			break // Success, exit retry loop
		}
		
		a.logger.Warn("Image download attempt failed", 
			"attempt", attempt+1, "url", imageURL, "error", err)
	}
	
	if err != nil {
		return fmt.Errorf("failed to download image after %d attempts: %w", maxRetries, err)
	}

	if len(imageData) == 0 {
		return fmt.Errorf("downloaded image data is empty")
	}

	// Check file size limits
	if a.config.MaxAssetSize > 0 && int64(len(imageData)) > a.config.MaxAssetSize {
		return fmt.Errorf("image size (%d bytes) exceeds maximum allowed size (%d bytes)", 
			len(imageData), a.config.MaxAssetSize)
	}

	// Use MediaAssetService instead of direct module calls
	if a.assetService == nil {
		a.logger.Warn("MediaAssetService not available, cannot save asset", 
			"media_file_id", mediaFileID, "type", assetType)
		return fmt.Errorf("MediaAssetService not available")
	}

	// Create asset request
	request := &AssetRequest{
		MediaFileID: mediaFileID,
		Type:        assetType,
		Category:    category,
		Subtype:     "artwork",
		Data:        imageData,
		MimeType:    mimeType,
		SourceURL:   imageURL,
	}

	// Save using MediaAssetService
	asset, err := a.assetService.SaveAsset(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to save asset via service: %w", err)
	}

	a.logger.Debug("Successfully downloaded and stored image", 
		"type", imageType, "media_file_id", mediaFileID, "size", len(imageData), 
		"asset_id", asset.ID, "attempts", maxRetries)
	
	return nil
}

// downloadImageFromURL downloads an image from the given URL and returns the raw data
func (a *AudioDBEnricher) downloadImageFromURL(ctx context.Context, imageURL string) ([]byte, string, error) {
	if imageURL == "" {
		return nil, "", fmt.Errorf("image URL is empty")
	}

	// Create request with timeout context
	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("User-Agent", a.config.UserAgent)

	// Execute request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("image download failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("image download error: HTTP %d", resp.StatusCode)
	}

	// Read image data
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read image data: %w", err)
	}

	// Get content type from headers
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		// Try to detect from URL extension
		contentType = a.detectContentTypeFromURL(imageURL)
	}

	return imageData, contentType, nil
}

// detectContentTypeFromURL tries to detect content type from URL extension
func (a *AudioDBEnricher) detectContentTypeFromURL(url string) string {
	url = strings.ToLower(url)
	if strings.Contains(url, ".jpg") || strings.Contains(url, ".jpeg") {
		return "image/jpeg"
	}
	if strings.Contains(url, ".png") {
		return "image/png"
	}
	if strings.Contains(url, ".gif") {
		return "image/gif"
	}
	if strings.Contains(url, ".webp") {
		return "image/webp"
	}
	// Default to JPEG
	return "image/jpeg"
}

// calculateMatchScore calculates the similarity score between query and result metadata
func (a *AudioDBEnricher) calculateMatchScore(queryTitle, queryArtist, queryAlbum, resultTitle, resultArtist, resultAlbum string) float64 {
	// Simple string similarity scoring
	titleScore := a.stringSimilarity(strings.ToLower(queryTitle), strings.ToLower(resultTitle))
	artistScore := a.stringSimilarity(strings.ToLower(queryArtist), strings.ToLower(resultArtist))
	
	// Weight: title=40%, artist=40%, album=20%
	score := titleScore*0.4 + artistScore*0.4
	
	if queryAlbum != "" && resultAlbum != "" {
		albumScore := a.stringSimilarity(strings.ToLower(queryAlbum), strings.ToLower(resultAlbum))
		score = titleScore*0.35 + artistScore*0.35 + albumScore*0.3
	}
	
	return score
}

// stringSimilarity calculates the similarity between two strings using Levenshtein distance
func (a *AudioDBEnricher) stringSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}
	
	// Simple Levenshtein-based similarity
	maxLen := len(s1)
	if len(s2) > maxLen {
		maxLen = len(s2)
	}
	
	if maxLen == 0 {
		return 1.0
	}
	
	distance := a.levenshteinDistance(s1, s2)
	return 1.0 - float64(distance)/float64(maxLen)
}

// levenshteinDistance calculates the Levenshtein distance between two strings
func (a *AudioDBEnricher) levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}
	
	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
		matrix[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}
	
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}
	
	return matrix[len(s1)][len(s2)]
}

// min returns the minimum of three integers
func min(a, b, c int) int {
	if a < b && a < c {
		return a
	}
	if b < c {
		return b
	}
	return c
}

// Implement the Value and Scan methods for GORM compatibility
func (a AudioDBConfig) Value() (driver.Value, error) {
	return json.Marshal(a)
}

func (a *AudioDBConfig) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into AudioDBConfig", value)
	}
	return json.Unmarshal(bytes, a)
}

func main() {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "audiodb-enricher-plugin",
		Level: hclog.Info,
	})

	enricher := &AudioDBEnricher{
		logger: logger,
	}

	// Verify that our enricher implements the correct interface
	var _ plugins.Implementation = enricher

	// pluginMap is the map of plugins we can dispense.
	grpcPlugin := &plugins.GRPCPlugin{Impl: enricher}
	var pluginMap = map[string]plugin.Plugin{
		"plugin": grpcPlugin,
	}

	logger.Info("AudioDB enricher plugin starting")
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugins.Handshake,
		Plugins:         pluginMap,
		GRPCServer:      plugin.DefaultGRPCServer,
		Logger:          logger,
	})
	logger.Info("AudioDB enricher plugin stopped")
} 