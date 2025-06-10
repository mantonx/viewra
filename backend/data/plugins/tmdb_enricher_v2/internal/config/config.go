package config

import (
	"fmt"
	"time"
)

// Config represents the complete plugin configuration structure
// This mirrors the CUE schema defined in plugin.cue
type Config struct {
	API         APIConfig         `json:"api"`
	Features    FeaturesConfig    `json:"features"`
	Artwork     ArtworkConfig     `json:"artwork"`
	Matching    MatchingConfig    `json:"matching"`
	Cache       CacheConfig       `json:"cache"`
	Reliability ReliabilityConfig `json:"reliability"`
	Debug       DebugConfig       `json:"debug"`
}

// APIConfig contains TMDb API-related settings
type APIConfig struct {
	Key        string  `json:"key"`         // TMDb API key (sensitive)
	RateLimit  float64 `json:"rate_limit"`  // Requests per second
	UserAgent  string  `json:"user_agent"`  // User agent for requests
	Language   string  `json:"language"`    // Preferred language (e.g., "en-US")
	Region     string  `json:"region"`      // Preferred region (e.g., "US")
	TimeoutSec int     `json:"timeout_sec"` // Request timeout in seconds
	DelayMs    int     `json:"delay_ms"`    // Delay between requests in milliseconds
}

// FeaturesConfig contains feature toggle settings
type FeaturesConfig struct {
	EnableMovies      bool `json:"enable_movies"`      // Enable movie enrichment
	EnableTVShows     bool `json:"enable_tv_shows"`    // Enable TV show enrichment
	EnableEpisodes    bool `json:"enable_episodes"`    // Enable episode-level enrichment
	EnableArtwork     bool `json:"enable_artwork"`     // Enable artwork downloads
	AutoEnrich        bool `json:"auto_enrich"`        // Automatically enrich during scanning
	OverwriteExisting bool `json:"overwrite_existing"` // Overwrite existing metadata
}

// ArtworkConfig contains artwork download settings
type ArtworkConfig struct {
	// Download toggles
	DownloadPosters       bool `json:"download_posters"`        // Download movie/show posters
	DownloadBackdrops     bool `json:"download_backdrops"`      // Download backdrop images
	DownloadLogos         bool `json:"download_logos"`          // Download logo images
	DownloadStills        bool `json:"download_stills"`         // Download episode stills
	DownloadSeasonPosters bool `json:"download_season_posters"` // Download season posters
	DownloadEpisodeStills bool `json:"download_episode_stills"` // Download episode stills

	// Image sizes (TMDb size options)
	PosterSize   string `json:"poster_size"`   // w92, w154, w185, w342, w500, w780, original
	BackdropSize string `json:"backdrop_size"` // w300, w780, w1280, original
	LogoSize     string `json:"logo_size"`     // w45, w92, w154, w185, w300, w500, original
	StillSize    string `json:"still_size"`    // w92, w185, w300, original

	// Download limits
	MaxAssetSizeMB     int  `json:"max_asset_size_mb"`    // Maximum asset size in MB
	AssetTimeoutSec    int  `json:"asset_timeout_sec"`    // Asset download timeout
	SkipExistingAssets bool `json:"skip_existing_assets"` // Skip downloading existing assets
}

// MatchingConfig contains content matching settings
type MatchingConfig struct {
	MatchThreshold float64 `json:"match_threshold"` // Minimum similarity score for matches
	MatchYear      bool    `json:"match_year"`      // Use release year for matching
	YearTolerance  int     `json:"year_tolerance"`  // Allow +/- years difference
}

// CacheConfig contains caching settings
type CacheConfig struct {
	DurationHours   int `json:"duration_hours"`   // Cache duration in hours
	CleanupInterval int `json:"cleanup_interval"` // Cleanup interval in hours
}

// ReliabilityConfig contains retry and reliability settings
type ReliabilityConfig struct {
	MaxRetries           int     `json:"max_retries"`            // Maximum retry attempts
	InitialDelaySeconds  int     `json:"initial_delay_sec"`      // Initial retry delay
	MaxDelaySeconds      int     `json:"max_delay_sec"`          // Maximum retry delay
	BackoffMultiplier    float64 `json:"backoff_multiplier"`     // Exponential backoff multiplier
	RetryFailedDownloads bool    `json:"retry_failed_downloads"` // Retry failed artwork downloads
}

// DebugConfig contains debug and monitoring settings
type DebugConfig struct {
	Enabled        bool `json:"enabled"`          // Enable debug logging
	LogAPIRequests bool `json:"log_api_requests"` // Log all API requests
	LogCacheHits   bool `json:"log_cache_hits"`   // Log cache hit/miss
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		API: APIConfig{
			Key:        "",           // Must be provided by user
			RateLimit:  0.6,          // Conservative: 0.6 requests/second
			UserAgent:  "Viewra/2.0", // Default user agent
			Language:   "en-US",      // English (US)
			Region:     "US",         // United States
			TimeoutSec: 60,           // 60 second timeout
			DelayMs:    1200,         // 1.2 seconds between requests
		},
		Features: FeaturesConfig{
			EnableMovies:      true,  // Enable movie enrichment
			EnableTVShows:     true,  // Enable TV show enrichment
			EnableEpisodes:    true,  // Enable episode enrichment
			EnableArtwork:     true,  // Enable artwork downloads
			AutoEnrich:        true,  // Auto-enrich during scanning
			OverwriteExisting: false, // Don't overwrite existing metadata
		},
		Artwork: ArtworkConfig{
			// Conservative defaults for artwork
			DownloadPosters:       true,  // Download posters
			DownloadBackdrops:     false, // Skip backdrops by default
			DownloadLogos:         false, // Skip logos by default
			DownloadStills:        false, // Skip stills by default
			DownloadSeasonPosters: true,  // Download season posters
			DownloadEpisodeStills: true,  // Download episode stills

			// Reasonable sizes
			PosterSize:   "w500", // 500px width posters
			BackdropSize: "w780", // 780px width backdrops
			LogoSize:     "w300", // 300px width logos
			StillSize:    "w300", // 300px width stills

			// Download limits
			MaxAssetSizeMB:     10,   // 10MB max per asset
			AssetTimeoutSec:    60,   // 60 second timeout
			SkipExistingAssets: true, // Skip existing assets
		},
		Matching: MatchingConfig{
			MatchThreshold: 0.85, // 85% similarity threshold
			MatchYear:      true, // Use year for matching
			YearTolerance:  2,    // Allow +/- 2 years difference
		},
		Cache: CacheConfig{
			DurationHours:   168, // 1 week cache duration
			CleanupInterval: 24,  // Daily cleanup
		},
		Reliability: ReliabilityConfig{
			MaxRetries:           5,    // 5 retry attempts
			InitialDelaySeconds:  2,    // 2 second initial delay
			MaxDelaySeconds:      30,   // 30 second max delay
			BackoffMultiplier:    2.0,  // 2x exponential backoff
			RetryFailedDownloads: true, // Retry failed downloads
		},
		Debug: DebugConfig{
			Enabled:        false, // Debug disabled by default
			LogAPIRequests: false, // Don't log API requests
			LogCacheHits:   false, // Don't log cache hits
		},
	}
}

// GetRequestDelay returns the delay duration between API requests
func (c *APIConfig) GetRequestDelay() time.Duration {
	return time.Duration(c.DelayMs) * time.Millisecond
}

// GetRequestTimeout returns the request timeout duration
func (c *APIConfig) GetRequestTimeout() time.Duration {
	return time.Duration(c.TimeoutSec) * time.Second
}

// GetCacheDuration returns the cache duration
func (c *CacheConfig) GetCacheDuration() time.Duration {
	return time.Duration(c.DurationHours) * time.Hour
}

// GetCleanupInterval returns the cleanup interval duration
func (c *CacheConfig) GetCleanupInterval() time.Duration {
	return time.Duration(c.CleanupInterval) * time.Hour
}

// GetInitialRetryDelay returns the initial retry delay duration
func (c *ReliabilityConfig) GetInitialRetryDelay() time.Duration {
	return time.Duration(c.InitialDelaySeconds) * time.Second
}

// GetMaxRetryDelay returns the maximum retry delay duration
func (c *ReliabilityConfig) GetMaxRetryDelay() time.Duration {
	return time.Duration(c.MaxDelaySeconds) * time.Second
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate API configuration
	if c.API.Key == "" {
		return fmt.Errorf("TMDb API key is required")
	}

	if c.API.RateLimit <= 0 {
		return fmt.Errorf("API rate limit must be positive")
	}

	if c.API.TimeoutSec <= 0 {
		return fmt.Errorf("API timeout must be positive")
	}

	// Validate matching configuration
	if c.Matching.MatchThreshold < 0 || c.Matching.MatchThreshold > 1 {
		return fmt.Errorf("match threshold must be between 0 and 1")
	}

	// Validate cache configuration
	if c.Cache.DurationHours <= 0 {
		return fmt.Errorf("cache duration must be positive")
	}

	// Validate reliability configuration
	if c.Reliability.MaxRetries < 0 {
		return fmt.Errorf("max retries must be non-negative")
	}

	if c.Reliability.BackoffMultiplier <= 1 {
		return fmt.Errorf("backoff multiplier must be greater than 1")
	}

	return nil
}
