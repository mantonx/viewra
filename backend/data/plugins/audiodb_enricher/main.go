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
	"github.com/mantonx/viewra/pkg/plugins"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// AudioDBEnricher implements the AudioDB enrichment plugin
type AudioDBEnricher struct {
	logger      hclog.Logger
	config      *Config
	db          *gorm.DB
	dbURL       string
	basePath    string
	lastAPICall *time.Time
	
	// Host service connection
	assetService    plugins.AssetServiceClient
	hostServiceAddr string
}

// Config holds the plugin configuration
type Config struct {
	Enabled              bool    `json:"enabled" default:"true"`
	APIKey               string  `json:"api_key"`                        // AudioDB API key (optional)
	UserAgent            string  `json:"user_agent" default:"Viewra/2.0"`
	EnableArtwork        bool    `json:"enable_artwork" default:"true"`
	ArtworkMaxSize       int     `json:"artwork_max_size" default:"1200"`
	ArtworkQuality       string  `json:"artwork_quality" default:"front"`
	
	// Album artwork settings
	DownloadAlbumArt     bool    `json:"download_album_art" default:"true"`
	DownloadAlbumThumb   bool    `json:"download_album_thumb" default:"true"`
	DownloadAlbumThumbHQ bool    `json:"download_album_thumb_hq" default:"true"`
	DownloadAlbumBack    bool    `json:"download_album_back" default:"true"`
	DownloadAlbumCDart   bool    `json:"download_album_cdart" default:"true"`
	
	// Artist artwork settings
	DownloadArtistImages bool    `json:"download_artist_images" default:"true"`
	DownloadArtistThumb  bool    `json:"download_artist_thumb" default:"true"`
	DownloadArtistLogo   bool    `json:"download_artist_logo" default:"true"`
	DownloadArtistClearart bool  `json:"download_artist_clearart" default:"true"`
	DownloadArtistFanart bool    `json:"download_artist_fanart" default:"true"`
	DownloadArtistFanart2 bool   `json:"download_artist_fanart2" default:"false"`
	DownloadArtistFanart3 bool   `json:"download_artist_fanart3" default:"false"`
	DownloadArtistBanner bool    `json:"download_artist_banner" default:"true"`
	
	// Track artwork settings
	DownloadTrackThumb   bool    `json:"download_track_thumb" default:"false"`
	
	PreferHighQuality    bool    `json:"prefer_high_quality" default:"true"`
	MaxAssetSize         int64   `json:"max_asset_size" default:"10485760"` // 10MB
	AssetTimeout         int     `json:"asset_timeout_sec" default:"30"`
	SkipExistingAssets   bool    `json:"skip_existing_assets" default:"true"`
	RetryFailedDownloads bool    `json:"retry_failed_downloads" default:"true"`
	MaxRetries           int     `json:"max_retries" default:"3"`
	MatchThreshold       float64 `json:"match_threshold" default:"0.85"`
	AutoEnrich           bool    `json:"auto_enrich" default:"true"`
	OverwriteExisting    bool    `json:"overwrite_existing" default:"false"`
	CacheDurationHours   int     `json:"cache_duration_hours" default:"168"` // 1 week
	RequestDelay         int     `json:"request_delay_ms" default:"100"`
}

// AudioDBEnrichment represents enriched metadata
type AudioDBEnrichment struct {
	ID              uint      `gorm:"primaryKey"`
	MediaFileID     uint      `gorm:"not null;index"`
	AudioDBTrackID  string    `gorm:"size:36"`
	AudioDBArtistID string    `gorm:"size:36"`
	AudioDBAlbumID  string    `gorm:"size:36"`
	EnrichedTitle   string    `gorm:"size:512"`
	EnrichedArtist  string    `gorm:"size:512"`
	EnrichedAlbum   string    `gorm:"size:512"`
	EnrichedYear    int
	EnrichedGenre   string    `gorm:"size:255"`
	MatchScore      float64
	ArtworkURL      string    `gorm:"size:1024"`
	ArtworkPath     string    `gorm:"size:1024"`
	BiographyURL    string    `gorm:"size:1024"`
	EnrichedAt      time.Time `gorm:"autoCreateTime"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime"`
}

// AudioDBCache represents cached API responses
type AudioDBCache struct {
	ID          uint      `gorm:"primaryKey"`
	CacheKey    string    `gorm:"uniqueIndex;not null"`
	Data        string    `gorm:"type:text"`
	ExpiresAt   time.Time `gorm:"index"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
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
	StrGenre           string `json:"strGenre"`
	StrMood            string `json:"strMood"`
	StrStyle           string `json:"strStyle"`
	StrCountry         string `json:"strCountry"`
	StrBiographyEN     string `json:"strBiographyEN"`
	StrArtistThumb     string `json:"strArtistThumb"`
	StrArtistLogo      string `json:"strArtistLogo"`
	StrArtistClearart  string `json:"strArtistClearart"`
	StrArtistFanart    string `json:"strArtistFanart"`
	StrArtistFanart2   string `json:"strArtistFanart2"`
	StrArtistFanart3   string `json:"strArtistFanart3"`
	StrArtistBanner    string `json:"strArtistBanner"`
	IntFormedYear      string `json:"intFormedYear"`
	StrMusicBrainzID   string `json:"strMusicBrainzID"`
}

type AudioDBAlbumResponse struct {
	Album []AudioDBAlbum `json:"album"`
}

type AudioDBAlbum struct {
	IDAlbum            string `json:"idAlbum"`
	IDArtist           string `json:"idArtist"`
	StrAlbum           string `json:"strAlbum"`
	StrArtist          string `json:"strArtist"`
	IntYearReleased    string `json:"intYearReleased"`
	StrGenre           string `json:"strGenre"`
	StrStyle           string `json:"strStyle"`
	StrLabel           string `json:"strLabel"`
	StrAlbumThumb      string `json:"strAlbumThumb"`
	StrAlbumThumbHQ    string `json:"strAlbumThumbHQ"`
	StrAlbumThumbBack  string `json:"strAlbumThumbBack"`
	StrAlbumCDart      string `json:"strAlbumCDart"`
	StrDescriptionEN   string `json:"strDescriptionEN"`
	StrMood            string `json:"strMood"`
	StrTheme           string `json:"strTheme"`
	StrMusicBrainzID   string `json:"strMusicBrainzID"`
	StrMusicBrainzArtistID string `json:"strMusicBrainzArtistID"`
}

type AudioDBTrackResponse struct {
	Track []AudioDBTrack `json:"track"`
}

type AudioDBTrack struct {
	IDTrack            string `json:"idTrack"`
	IDArtist           string `json:"idArtist"`
	IDAlbum            string `json:"idAlbum"`
	StrTrack           string `json:"strTrack"`
	StrAlbum           string `json:"strAlbum"`
	StrArtist          string `json:"strArtist"`
	StrGenre           string `json:"strGenre"`
	StrMood            string `json:"strMood"`
	StrStyle           string `json:"strStyle"`
	StrTheme           string `json:"strTheme"`
	StrDescriptionEN   string `json:"strDescriptionEN"`
	StrTrackLyrics     string `json:"strTrackLyrics"`
	StrTrackThumb      string `json:"strTrackThumb"`
	IntDuration        string `json:"intDuration"`
	StrMusicBrainzID   string `json:"strMusicBrainzID"`
	StrMusicBrainzAlbumID string `json:"strMusicBrainzAlbumID"`
	StrMusicBrainzArtistID string `json:"strMusicBrainzArtistID"`
}

// AudioDB artwork downloading
type AudioDBArtworkType struct {
	Name     string
	URL      func(*AudioDBTrack, *AudioDBAlbum, *AudioDBArtist) string
	Category string
	Subtype  string
	Enabled  func(*Config) bool
}

var audiodbArtworkTypes = []AudioDBArtworkType{
	// Track artwork
	{
		Name:     "track_thumb",
		URL:      func(track *AudioDBTrack, album *AudioDBAlbum, artist *AudioDBArtist) string { return track.StrTrackThumb },
		Category: "track",
		Subtype:  "track_thumb",
		Enabled:  func(c *Config) bool { return c.DownloadTrackThumb },
	},
	
	// Album artwork
	{
		Name:     "album_thumb",
		URL:      func(track *AudioDBTrack, album *AudioDBAlbum, artist *AudioDBArtist) string { return album.StrAlbumThumb },
		Category: "album",
		Subtype:  "album_thumb",
		Enabled:  func(c *Config) bool { return c.DownloadAlbumThumb },
	},
	{
		Name:     "album_thumb_hq",
		URL:      func(track *AudioDBTrack, album *AudioDBAlbum, artist *AudioDBArtist) string { return album.StrAlbumThumbHQ },
		Category: "album",
		Subtype:  "album_thumb_hq",
		Enabled:  func(c *Config) bool { return c.DownloadAlbumThumbHQ },
	},
	{
		Name:     "album_back",
		URL:      func(track *AudioDBTrack, album *AudioDBAlbum, artist *AudioDBArtist) string { return album.StrAlbumThumbBack },
		Category: "album",
		Subtype:  "album_thumb_back",
		Enabled:  func(c *Config) bool { return c.DownloadAlbumBack },
	},
	{
		Name:     "album_cdart",
		URL:      func(track *AudioDBTrack, album *AudioDBAlbum, artist *AudioDBArtist) string { return album.StrAlbumCDart },
		Category: "album",
		Subtype:  "album_cdart",
		Enabled:  func(c *Config) bool { return c.DownloadAlbumCDart },
	},
	
	// Artist artwork
	{
		Name:     "artist_thumb",
		URL:      func(track *AudioDBTrack, album *AudioDBAlbum, artist *AudioDBArtist) string { return artist.StrArtistThumb },
		Category: "artist",
		Subtype:  "artist_thumb",
		Enabled:  func(c *Config) bool { return c.DownloadArtistThumb },
	},
	{
		Name:     "artist_logo",
		URL:      func(track *AudioDBTrack, album *AudioDBAlbum, artist *AudioDBArtist) string { return artist.StrArtistLogo },
		Category: "artist",
		Subtype:  "artist_logo",
		Enabled:  func(c *Config) bool { return c.DownloadArtistLogo },
	},
	{
		Name:     "artist_clearart",
		URL:      func(track *AudioDBTrack, album *AudioDBAlbum, artist *AudioDBArtist) string { return artist.StrArtistClearart },
		Category: "artist",
		Subtype:  "artist_clearart",
		Enabled:  func(c *Config) bool { return c.DownloadArtistClearart },
	},
	{
		Name:     "artist_fanart",
		URL:      func(track *AudioDBTrack, album *AudioDBAlbum, artist *AudioDBArtist) string { return artist.StrArtistFanart },
		Category: "artist",
		Subtype:  "artist_fanart",
		Enabled:  func(c *Config) bool { return c.DownloadArtistFanart },
	},
	{
		Name:     "artist_fanart2",
		URL:      func(track *AudioDBTrack, album *AudioDBAlbum, artist *AudioDBArtist) string { return artist.StrArtistFanart2 },
		Category: "artist",
		Subtype:  "artist_fanart2",
		Enabled:  func(c *Config) bool { return c.DownloadArtistFanart2 },
	},
	{
		Name:     "artist_fanart3",
		URL:      func(track *AudioDBTrack, album *AudioDBAlbum, artist *AudioDBArtist) string { return artist.StrArtistFanart3 },
		Category: "artist",
		Subtype:  "artist_fanart3",
		Enabled:  func(c *Config) bool { return c.DownloadArtistFanart3 },
	},
	{
		Name:     "artist_banner",
		URL:      func(track *AudioDBTrack, album *AudioDBAlbum, artist *AudioDBArtist) string { return artist.StrArtistBanner },
		Category: "artist",
		Subtype:  "artist_banner",
		Enabled:  func(c *Config) bool { return c.DownloadArtistBanner },
	},
}

// Plugin interface implementations
func (a *AudioDBEnricher) Initialize(ctx *plugins.PluginContext) error {
	a.logger = hclog.New(&hclog.LoggerOptions{
		Name:  "audiodb-enricher",
		Level: hclog.LevelFromString(ctx.LogLevel),
	})

	a.basePath = ctx.BasePath
	a.dbURL = ctx.DatabaseURL
	a.hostServiceAddr = ctx.HostServiceAddr

	// Set default configuration
	a.config = &Config{
		Enabled:              true,
		UserAgent:            "Viewra/2.0",
		EnableArtwork:        true,
		ArtworkMaxSize:       1200,
		ArtworkQuality:       "front",
		
		// Album artwork settings
		DownloadAlbumArt:     true,
		DownloadAlbumThumb:   true,
		DownloadAlbumThumbHQ: true,
		DownloadAlbumBack:    true,
		DownloadAlbumCDart:   true,
		
		// Artist artwork settings
		DownloadArtistImages: true,
		DownloadArtistThumb:  true,
		DownloadArtistLogo:   true,
		DownloadArtistClearart: true,
		DownloadArtistFanart: true,
		DownloadArtistFanart2: false,
		DownloadArtistFanart3: false,
		DownloadArtistBanner: true,
		
		// Track artwork settings
		DownloadTrackThumb:   false,
		
		PreferHighQuality:    true,
		MaxAssetSize:         10485760, // 10MB
		AssetTimeout:         30,
		SkipExistingAssets:   true,
		RetryFailedDownloads: true,
		MaxRetries:           3,
		MatchThreshold:       0.85,
		AutoEnrich:           true,
		OverwriteExisting:    false,
		CacheDurationHours:   168, // 1 week
		RequestDelay:         100,
	}

	a.logger.Info("Initializing AudioDB enricher plugin", 
		"base_path", a.basePath,
		"database_url", a.dbURL,
		"host_service_addr", a.hostServiceAddr,
		"auto_enrich", a.config.AutoEnrich,
		"enable_artwork", a.config.EnableArtwork)

	// Initialize database connection
	if err := a.initDatabase(); err != nil {
		a.logger.Error("Failed to initialize database", "error", err)
		return fmt.Errorf("database initialization failed: %w", err)
	}

	// Initialize host asset service connection if address provided
	if a.hostServiceAddr != "" {
		assetClient, err := plugins.NewAssetServiceClient(a.hostServiceAddr)
		if err != nil {
			a.logger.Error("Failed to connect to host asset service", "error", err, "addr", a.hostServiceAddr)
			return fmt.Errorf("failed to connect to host asset service: %w", err)
		}
		a.assetService = assetClient
		a.logger.Info("Connected to host asset service", "addr", a.hostServiceAddr)
	} else {
		a.logger.Warn("No host service address provided - asset saving will be disabled")
	}

	a.logger.Info("AudioDB enricher plugin initialized successfully")
	return nil
}

func (a *AudioDBEnricher) Start() error {
	a.logger.Info("AudioDB Enricher plugin started")
	return nil
}

func (a *AudioDBEnricher) Stop() error {
	a.logger.Info("AudioDB Enricher plugin stopped")
	
	// Close asset service connection
	if a.assetService != nil {
		if closer, ok := a.assetService.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				a.logger.Warn("Failed to close asset service connection", "error", err)
			}
		}
	}
	
	// Close database connection
	if a.db != nil {
		if sqlDB, err := a.db.DB(); err == nil {
			sqlDB.Close()
		}
	}
	
	return nil
}

func (a *AudioDBEnricher) Info() (*plugins.PluginInfo, error) {
	return &plugins.PluginInfo{
		ID:          "audiodb_enricher",
		Name:        "AudioDB Enricher",
		Version:     "2.0.0",
		Type:        "metadata_scraper",
		Description: "Enriches music metadata using The AudioDB API",
		Author:      "Viewra Team",
	}, nil
}

func (a *AudioDBEnricher) Health() error {
	if !a.config.Enabled {
		return fmt.Errorf("plugin is disabled")
	}

	// Test database connection
	if a.db != nil {
		sqlDB, err := a.db.DB()
		if err != nil {
			return fmt.Errorf("failed to get database instance: %w", err)
		}
		if err := sqlDB.Ping(); err != nil {
			return fmt.Errorf("database ping failed: %w", err)
		}
	}

	return nil
}

// Service implementations
func (a *AudioDBEnricher) MetadataScraperService() plugins.MetadataScraperService {
	return a
}

func (a *AudioDBEnricher) ScannerHookService() plugins.ScannerHookService {
	return a
}

func (a *AudioDBEnricher) AssetService() plugins.AssetService {
	return nil // AudioDB doesn't manage assets directly
}

func (a *AudioDBEnricher) DatabaseService() plugins.DatabaseService {
	return a
}

func (a *AudioDBEnricher) AdminPageService() plugins.AdminPageService {
	return nil // No admin page for now
}

func (a *AudioDBEnricher) APIRegistrationService() plugins.APIRegistrationService {
	return a
}

func (a *AudioDBEnricher) SearchService() plugins.SearchService {
	return a
}

// MetadataScraperService implementation
func (a *AudioDBEnricher) CanHandle(filePath, mimeType string) bool {
	if !a.config.Enabled {
		return false
	}

	// Handle audio files
	audioMimeTypes := []string{
		"audio/mpeg",
		"audio/mp4",
		"audio/m4a",
		"audio/flac",
		"audio/ogg",
		"audio/wav",
		"audio/aac",
		"audio/wma",
	}

	for _, supportedType := range audioMimeTypes {
		if mimeType == supportedType {
			return true
		}
	}

	// Also check by file extension
	supportedExtensions := []string{".mp3", ".m4a", ".flac", ".ogg", ".wav", ".aac", ".wma"}
	filePath = strings.ToLower(filePath)
	for _, ext := range supportedExtensions {
		if strings.HasSuffix(filePath, ext) {
			return true
		}
	}

	return false
}

func (a *AudioDBEnricher) ExtractMetadata(filePath string) (map[string]string, error) {
	// AudioDB doesn't extract metadata from files, it enriches existing metadata
	return nil, fmt.Errorf("audiodb_enricher does not extract metadata from files")
}

func (a *AudioDBEnricher) GetSupportedTypes() []string {
	return []string{
		"audio/mpeg",
		"audio/mp4", 
		"audio/m4a",
		"audio/flac",
		"audio/ogg",
		"audio/wav",
		"audio/aac",
		"audio/wma",
	}
}

// ScannerHookService implementation
func (a *AudioDBEnricher) OnMediaFileScanned(mediaFileID uint32, filePath string, metadata map[string]string) error {
	if !a.config.Enabled || !a.config.AutoEnrich {
		return nil
	}

	title := metadata["title"]
	artist := metadata["artist"]
	album := metadata["album"]

	if title == "" || artist == "" {
		a.logger.Debug("Insufficient metadata for enrichment", "file", filePath)
		return nil
	}

	return a.enrichTrack(uint(mediaFileID), title, artist, album)
}

func (a *AudioDBEnricher) OnScanStarted(scanJobID, libraryID uint32, libraryPath string) error {
	a.logger.Info("Scan started, AudioDB enricher ready", "scanJobID", scanJobID, "libraryID", libraryID)
	return nil
}

func (a *AudioDBEnricher) OnScanCompleted(scanJobID, libraryID uint32, stats map[string]string) error {
	a.logger.Info("Scan completed", "scanJobID", scanJobID, "libraryID", libraryID)
	return nil
}

// SearchService implementation
func (a *AudioDBEnricher) Search(ctx context.Context, query map[string]string, limit, offset uint32) ([]*plugins.SearchResult, uint32, bool, error) {
	if !a.config.Enabled {
		return nil, 0, false, fmt.Errorf("plugin is disabled")
	}

	title := query["title"]
	artist := query["artist"]
	album := query["album"]

	if artist == "" {
		return nil, 0, false, fmt.Errorf("artist is required for AudioDB search")
	}

	tracks, err := a.searchTracks(title, artist, album)
	if err != nil {
		return nil, 0, false, err
	}

	results := make([]*plugins.SearchResult, 0, len(tracks))
	for _, track := range tracks {
		score := a.calculateMatchScore(title, artist, album, track.StrTrack, track.StrArtist, track.StrAlbum)
		if score < a.config.MatchThreshold {
			continue
		}

		results = append(results, &plugins.SearchResult{
			ID:    track.IDTrack,
			Title: track.StrTrack,
			Type:  "track",
			Metadata: map[string]string{
				"artist":    track.StrArtist,
				"album":     track.StrAlbum,
				"genre":     track.StrGenre,
				"mood":      track.StrMood,
				"style":     track.StrStyle,
				"duration":  track.IntDuration,
				"thumb":     track.StrTrackThumb,
				"audiodb_id": track.IDTrack,
			},
		})

		if len(results) >= int(limit) {
			break
		}
	}

	hasMore := len(tracks) > int(limit+offset)
	return results, uint32(len(tracks)), hasMore, nil
}

func (a *AudioDBEnricher) GetSearchCapabilities(ctx context.Context) ([]string, bool, uint32, error) {
	return []string{"title", "artist", "album", "genre"}, true, 100, nil
}

// DatabaseService implementation
func (a *AudioDBEnricher) GetModels() []string {
	return []string{
		"AudioDBEnrichment",
		"AudioDBCache",
	}
}

func (a *AudioDBEnricher) Migrate(connectionString string) error {
	return a.initDatabase()
}

func (a *AudioDBEnricher) Rollback(connectionString string) error {
	return fmt.Errorf("rollback not implemented")
}

// APIRegistrationService implementation
func (a *AudioDBEnricher) GetRegisteredRoutes(ctx context.Context) ([]*plugins.APIRoute, error) {
	return []*plugins.APIRoute{
		{
			Method:      "GET",
			Path:        "/api/plugins/audiodb/search",
			Description: "Search AudioDB for tracks, artists, and albums",
		},
		{
			Method:      "POST",
			Path:        "/api/plugins/audiodb/enrich",
			Description: "Manually enrich media file with AudioDB data",
		},
	}, nil
}

// Database initialization
func (a *AudioDBEnricher) initDatabase() error {
	if a.dbURL == "" {
		return fmt.Errorf("database URL not provided")
	}

	db, err := gorm.Open(sqlite.Open(a.dbURL), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	a.db = db

	// Auto-migrate tables
	if err := db.AutoMigrate(&AudioDBEnrichment{}, &AudioDBCache{}); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	a.logger.Info("AudioDB database initialized successfully")
	return nil
}

// Core enrichment logic
func (a *AudioDBEnricher) enrichTrack(mediaFileID uint, title, artist, album string) error {
	// Check if already enriched and not overwriting
	if !a.config.OverwriteExisting {
		var existing AudioDBEnrichment
		if err := a.db.Where("media_file_id = ?", mediaFileID).First(&existing).Error; err == nil {
			a.logger.Debug("Track already enriched, skipping", "mediaFileID", mediaFileID)
			return nil
		}
	}

	tracks, err := a.searchTracks(title, artist, album)
	if err != nil {
		return fmt.Errorf("failed to search tracks: %w", err)
	}

	if len(tracks) == 0 {
		a.logger.Debug("No AudioDB matches found", "title", title, "artist", artist, "album", album)
		return nil
	}

	// Find best match
	var bestTrack *AudioDBTrack
	var bestScore float64
	for _, track := range tracks {
		score := a.calculateMatchScore(title, artist, album, track.StrTrack, track.StrArtist, track.StrAlbum)
		if score > bestScore && score >= a.config.MatchThreshold {
			bestScore = score
			bestTrack = &track
		}
	}

	if bestTrack == nil {
		a.logger.Debug("No matches above threshold", "threshold", a.config.MatchThreshold)
		return nil
	}

	// Save enrichment data
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
	}

	// Try to get year from album data
	if bestTrack.IDAlbum != "" {
		if albumResp, err := a.getAlbumInfo(bestTrack.IDAlbum); err == nil && len(albumResp.Album) > 0 {
			if year, err := strconv.Atoi(albumResp.Album[0].IntYearReleased); err == nil {
				enrichment.EnrichedYear = year
			}
		}
	}

	if err := a.db.Save(&enrichment).Error; err != nil {
		return fmt.Errorf("failed to save enrichment: %w", err)
	}

	a.logger.Info("Track enriched successfully", 
		"mediaFileID", mediaFileID, 
		"title", bestTrack.StrTrack, 
		"artist", bestTrack.StrArtist,
		"score", bestScore)

	// Download artwork if enabled
	if a.config.EnableArtwork {
		if err := a.downloadAllArtwork(context.Background(), uint32(mediaFileID), bestTrack); err != nil {
			a.logger.Warn("Failed to download artwork", "error", err, "mediaFileID", mediaFileID)
			// Don't fail enrichment if artwork download fails
		} else {
			a.logger.Info("Successfully downloaded artwork", "mediaFileID", mediaFileID)
		}
	}

	return nil
}

// AudioDB API calls
func (a *AudioDBEnricher) searchTracks(title, artist, album string) ([]AudioDBTrack, error) {
	// Rate limiting
	if a.lastAPICall != nil {
		elapsed := time.Since(*a.lastAPICall)
		minDelay := time.Duration(a.config.RequestDelay) * time.Millisecond
		if elapsed < minDelay {
			time.Sleep(minDelay - elapsed)
		}
	}
	now := time.Now()
	a.lastAPICall = &now

	// Try cache first
	cacheKey := a.getCacheKey("track", title, artist, album)
	if cached := a.getCachedResult(cacheKey); cached != nil {
		var tracks []AudioDBTrack
		if err := json.Unmarshal([]byte(*cached), &tracks); err == nil {
			return tracks, nil
		}
	}

	// Search by artist and track
	searchURL := fmt.Sprintf("https://www.theaudiodb.com/api/v1/json/1/searchtrack.php?s=%s&t=%s",
		url.QueryEscape(artist), url.QueryEscape(title))

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", a.config.UserAgent)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var trackResp AudioDBTrackResponse
	if err := json.NewDecoder(resp.Body).Decode(&trackResp); err != nil {
		return nil, err
	}

	// Cache the results
	if data, err := json.Marshal(trackResp.Track); err == nil {
		a.cacheResult(cacheKey, string(data))
	}

	return trackResp.Track, nil
}

func (a *AudioDBEnricher) getAlbumInfo(albumID string) (*AudioDBAlbumResponse, error) {
	searchURL := fmt.Sprintf("https://www.theaudiodb.com/api/v1/json/1/album.php?m=%s", albumID)

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", a.config.UserAgent)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var albumResp AudioDBAlbumResponse
	if err := json.NewDecoder(resp.Body).Decode(&albumResp); err != nil {
		return nil, err
	}

	return &albumResp, nil
}

// Utility functions
func (a *AudioDBEnricher) getCacheKey(searchType, title, artist, album string) string {
	data := fmt.Sprintf("%s:%s:%s:%s", searchType, title, artist, album)
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("%x", hash)
}

func (a *AudioDBEnricher) getCachedResult(cacheKey string) *string {
	var cache AudioDBCache
	if err := a.db.Where("cache_key = ? AND expires_at > ?", cacheKey, time.Now()).First(&cache).Error; err != nil {
		return nil
	}
	return &cache.Data
}

func (a *AudioDBEnricher) cacheResult(cacheKey, data string) {
	expiresAt := time.Now().Add(time.Duration(a.config.CacheDurationHours) * time.Hour)
	cache := AudioDBCache{
		CacheKey:  cacheKey,
		Data:      data,
		ExpiresAt: expiresAt,
	}
	a.db.Save(&cache)
}

func (a *AudioDBEnricher) calculateMatchScore(queryTitle, queryArtist, queryAlbum, resultTitle, resultArtist, resultAlbum string) float64 {
	var totalScore float64
	var factors int

	// Title similarity (weight: 40%)
	if queryTitle != "" && resultTitle != "" {
		titleScore := a.stringSimilarity(strings.ToLower(queryTitle), strings.ToLower(resultTitle))
		totalScore += titleScore * 0.4
		factors++
	}

	// Artist similarity (weight: 40%)
	if queryArtist != "" && resultArtist != "" {
		artistScore := a.stringSimilarity(strings.ToLower(queryArtist), strings.ToLower(resultArtist))
		totalScore += artistScore * 0.4
		factors++
	}

	// Album similarity (weight: 20%)
	if queryAlbum != "" && resultAlbum != "" {
		albumScore := a.stringSimilarity(strings.ToLower(queryAlbum), strings.ToLower(resultAlbum))
		totalScore += albumScore * 0.2
		factors++
	}

	if factors == 0 {
		return 0.0
	}

	// Adjust total score based on available factors
	if factors == 1 {
		return totalScore
	} else if factors == 2 && queryAlbum == "" {
		// Only title and artist available, adjust weights
		return totalScore / 0.8 // Normalize to full scale
	}

	return totalScore
}

func (a *AudioDBEnricher) stringSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}
	if len(s1) == 0 || len(s2) == 0 {
		return 0.0
	}

	// Calculate Levenshtein distance
	distance := a.levenshteinDistance(s1, s2)
	maxLen := len(s1)
	if len(s2) > maxLen {
		maxLen = len(s2)
	}

	// Convert distance to similarity score
	similarity := 1.0 - (float64(distance) / float64(maxLen))
	if similarity < 0 {
		similarity = 0
	}

	return similarity
}

func (a *AudioDBEnricher) levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Create a matrix
	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
	}

	// Initialize first row and column
	for i := 0; i <= len(s1); i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}

	// Fill the matrix
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

func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// downloadAllArtwork downloads all enabled artwork types for a track
func (a *AudioDBEnricher) downloadAllArtwork(ctx context.Context, mediaFileID uint32, track *AudioDBTrack) error {
	var album *AudioDBAlbum
	var artist *AudioDBArtist

	// Get album info if available
	if track.IDAlbum != "" {
		if albumResp, err := a.getAlbumInfo(track.IDAlbum); err == nil && len(albumResp.Album) > 0 {
			album = &albumResp.Album[0]
		}
	}

	// Get artist info if available
	if track.IDArtist != "" {
		if artistResp, err := a.getArtistInfo(track.IDArtist); err == nil && len(artistResp.Artists) > 0 {
			artist = &artistResp.Artists[0]
		}
	}

	var downloadErrors []string
	successCount := 0

	for _, artType := range audiodbArtworkTypes {
		if !artType.Enabled(a.config) {
			a.logger.Debug("Skipping artwork type (disabled)", "type", artType.Name)
			continue
		}

		artworkURL := artType.URL(track, album, artist)
		if artworkURL == "" {
			a.logger.Debug("No URL available for artwork type", "type", artType.Name)
			continue
		}

		if err := a.downloadArtworkFromURL(ctx, mediaFileID, artType, artworkURL); err != nil {
			downloadErrors = append(downloadErrors, fmt.Sprintf("%s: %v", artType.Name, err))
			a.logger.Debug("Failed to download artwork type", "type", artType.Name, "error", err)
		} else {
			successCount++
			a.logger.Debug("Successfully downloaded artwork", "type", artType.Name, "media_file_id", mediaFileID)
		}
	}

	a.logger.Info("AudioDB artwork download completed", 
		"media_file_id", mediaFileID, 
		"success_count", successCount, 
		"error_count", len(downloadErrors))

	// Return error only if all downloads failed
	if len(downloadErrors) > 0 && successCount == 0 {
		return fmt.Errorf("all artwork downloads failed: %s", strings.Join(downloadErrors, "; "))
	}

	return nil
}

// downloadArtworkFromURL downloads artwork from a specific URL
func (a *AudioDBEnricher) downloadArtworkFromURL(ctx context.Context, mediaFileID uint32, artType AudioDBArtworkType, artworkURL string) error {
	if artworkURL == "" {
		return fmt.Errorf("no artwork URL available")
	}

	a.logger.Debug("Downloading AudioDB artwork", "type", artType.Name, "url", artworkURL)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: time.Duration(a.config.AssetTimeout) * time.Second,
	}

	// Download with retry logic
	var imageData []byte
	var mimeType string
	var downloadErr error

	for attempt := 0; attempt <= a.config.MaxRetries; attempt++ {
		if attempt > 0 {
			a.logger.Debug("Retrying AudioDB artwork download", "type", artType.Name, "attempt", attempt)
			time.Sleep(time.Duration(attempt) * time.Second) // Progressive backoff
		}

		req, err := http.NewRequest("GET", artworkURL, nil)
		if err != nil {
			downloadErr = err
			continue
		}
		req.Header.Set("User-Agent", a.config.UserAgent)

		resp, err := client.Do(req)
		if err != nil {
			downloadErr = err
			continue
		}

		if resp.StatusCode == 404 {
			resp.Body.Close()
			return fmt.Errorf("no artwork available")
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()
			downloadErr = fmt.Errorf("download failed with status %d", resp.StatusCode)
			continue
		}

		// Check content length
		if resp.ContentLength > a.config.MaxAssetSize {
			resp.Body.Close()
			return fmt.Errorf("artwork too large: %d bytes (max: %d)", resp.ContentLength, a.config.MaxAssetSize)
		}

		// Read the image data
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			downloadErr = err
			continue
		}

		// Check actual size
		if int64(len(data)) > a.config.MaxAssetSize {
			return fmt.Errorf("artwork too large: %d bytes (max: %d)", len(data), a.config.MaxAssetSize)
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
		return fmt.Errorf("failed after %d attempts: %w", a.config.MaxRetries+1, downloadErr)
	}

	a.logger.Debug("Downloaded AudioDB artwork data", "type", artType.Name, "size", len(imageData), "mime_type", mimeType)

	// Save the artwork using the host's AssetService
	metadata := map[string]string{
		"source":    "audiodb",
		"art_type":  artType.Name,
		"category":  artType.Category,
	}

	return a.saveArtworkAsset(ctx, mediaFileID, artType.Category, artType.Subtype, imageData, mimeType, artworkURL, metadata)
}

// getArtistInfo retrieves artist information from AudioDB
func (a *AudioDBEnricher) getArtistInfo(artistID string) (*AudioDBArtistResponse, error) {
	searchURL := fmt.Sprintf("https://www.theaudiodb.com/api/v1/json/1/artist.php?i=%s", artistID)

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", a.config.UserAgent)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var artistResp AudioDBArtistResponse
	if err := json.NewDecoder(resp.Body).Decode(&artistResp); err != nil {
		return nil, err
	}

	return &artistResp, nil
}

// saveArtworkAsset saves artwork using the host's asset service
func (a *AudioDBEnricher) saveArtworkAsset(ctx context.Context, mediaFileID uint32, category, subtype string, data []byte, mimeType, sourceURL string, metadata map[string]string) error {
	if a.assetService == nil {
		a.logger.Warn("Asset service not available - cannot save artwork", "media_file_id", mediaFileID, "category", category, "subtype", subtype)
		return fmt.Errorf("asset service not available")
	}

	a.logger.Debug("Saving AudioDB artwork asset via host service", 
		"media_file_id", mediaFileID, 
		"category", category,
		"subtype", subtype, 
		"size", len(data), 
		"mime_type", mimeType,
		"source_url", sourceURL)

	// Create save asset request
	request := &plugins.SaveAssetRequest{
		MediaFileID: mediaFileID,
		AssetType:   "music",
		Category:    category,
		Subtype:     subtype,
		Data:        data,
		MimeType:    mimeType,
		SourceURL:   sourceURL,
		Metadata:    metadata,
	}

	// Call host asset service
	response, err := a.assetService.SaveAsset(ctx, request)
	if err != nil {
		a.logger.Error("Failed to save asset via host service", "error", err, "media_file_id", mediaFileID, "category", category, "subtype", subtype)
		return fmt.Errorf("failed to save asset: %w", err)
	}

	if !response.Success {
		a.logger.Error("Host service reported save failure", "error", response.Error, "media_file_id", mediaFileID, "category", category, "subtype", subtype)
		return fmt.Errorf("asset save failed: %s", response.Error)
	}

	a.logger.Info("Successfully saved AudioDB artwork asset", 
		"media_file_id", mediaFileID, 
		"category", category,
		"subtype", subtype, 
		"asset_id", response.AssetID,
		"hash", response.Hash,
		"path", response.RelativePath,
		"size", len(data))

	return nil
}

func main() {
	plugin := &AudioDBEnricher{}
	plugins.StartPlugin(plugin)
} 