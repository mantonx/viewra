package config

import (
	"time"
)

// Config represents the MusicBrainz enricher plugin configuration
type Config struct {
	// Core Settings
	Enabled bool `json:"enabled" default:"true"`

	// API Configuration
	API APIConfig `json:"api"`

	// Feature toggles
	Features FeatureConfig `json:"features"`

	// Cover Art Settings
	CoverArt CoverArtConfig `json:"cover_art"`

	// Matching Configuration
	Matching MatchingConfig `json:"matching"`

	// Data Enrichment Options
	Enrichment EnrichmentConfig `json:"enrichment"`

	// Reliability & Error Handling
	Reliability ReliabilityConfig `json:"reliability"`

	// Debug & Logging
	Debug DebugConfig `json:"debug"`
}

// APIConfig holds API-related configuration
type APIConfig struct {
	UserAgent          string `json:"user_agent" default:"Viewra/2.0 (https://github.com/mantonx/viewra)"`
	RequestTimeout     int    `json:"request_timeout" default:"30"` // seconds
	MaxConnections     int    `json:"max_connections" default:"5"`  // concurrent connections
	RequestDelay       int    `json:"request_delay" default:"1000"` // milliseconds between requests (MusicBrainz rate limiting)
	EnableCache        bool   `json:"enable_cache" default:"true"`
	CacheDurationHours int    `json:"cache_duration_hours" default:"168"` // 1 week
}

// FeatureConfig controls which features are enabled
type FeatureConfig struct {
	EnableArtists       bool `json:"enable_artists" default:"true"`
	EnableAlbums        bool `json:"enable_albums" default:"true"`
	EnableTracks        bool `json:"enable_tracks" default:"true"`
	EnableCoverArt      bool `json:"enable_cover_art" default:"true"`
	EnableRelationships bool `json:"enable_relationships" default:"true"` // Artist relationships, collaborations
	EnableGenres        bool `json:"enable_genres" default:"true"`
	EnableTags          bool `json:"enable_tags" default:"true"`
}

// CoverArtConfig controls cover art download behavior
type CoverArtConfig struct {
	DownloadCovers     bool     `json:"download_covers" default:"true"`
	DownloadThumbnails bool     `json:"download_thumbnails" default:"false"`
	PreferredSize      string   `json:"preferred_size" default:"500"` // 250, 500, 1200, original
	MaxSizeMB          int      `json:"max_size_mb" default:"5"`      // Maximum cover art file size
	SkipExisting       bool     `json:"skip_existing" default:"true"`
	CoverSources       []string `json:"cover_sources"` // Preferred sources in order
}

// MatchingConfig controls how content matching works
type MatchingConfig struct {
	MatchThreshold    float64 `json:"match_threshold" default:"0.80"` // Minimum match confidence
	AutoEnrich        bool    `json:"auto_enrich" default:"true"`
	OverwriteExisting bool    `json:"overwrite_existing" default:"false"`
	FuzzyMatching     bool    `json:"fuzzy_matching" default:"true"`
	MatchByISRC       bool    `json:"match_by_isrc" default:"true"`    // International Standard Recording Code
	MatchByBarcode    bool    `json:"match_by_barcode" default:"true"` // Album barcode matching
	MatchDuration     bool    `json:"match_duration" default:"true"`   // Track duration matching tolerance
	DurationTolerance int     `json:"duration_tolerance" default:"10"` // seconds tolerance for duration matching
}

// EnrichmentConfig controls what data is included in enrichment
type EnrichmentConfig struct {
	IncludeAliases        bool `json:"include_aliases" default:"true"`         // Alternative artist/album names
	IncludeAnnotations    bool `json:"include_annotations" default:"false"`    // Detailed annotations (can be verbose)
	IncludeRecordingLevel bool `json:"include_recording_level" default:"true"` // Recording-level metadata vs. track-level
	IncludeWorkInfo       bool `json:"include_work_info" default:"true"`       // Musical work information (compositions)
	IncludeLabelInfo      bool `json:"include_label_info" default:"true"`      // Record label information
	IncludeCountryInfo    bool `json:"include_country_info" default:"true"`    // Release country information
	MaxTagsPerItem        int  `json:"max_tags_per_item" default:"10"`         // Limit tags to avoid data bloat
	MaxGenresPerItem      int  `json:"max_genres_per_item" default:"5"`        // Limit genres per item
}

// ReliabilityConfig controls retry and circuit breaker behavior
type ReliabilityConfig struct {
	MaxRetries        int                  `json:"max_retries" default:"3"`
	InitialRetryDelay int                  `json:"initial_retry_delay" default:"2"`  // seconds
	MaxRetryDelay     int                  `json:"max_retry_delay" default:"30"`     // seconds
	BackoffMultiplier float64              `json:"backoff_multiplier" default:"2.0"` // exponential backoff
	TimeoutMultiplier float64              `json:"timeout_multiplier" default:"1.5"` // increase timeout on retries
	CircuitBreaker    CircuitBreakerConfig `json:"circuit_breaker"`
}

// CircuitBreakerConfig controls circuit breaker behavior
type CircuitBreakerConfig struct {
	FailureThreshold int `json:"failure_threshold" default:"5"` // failures before opening circuit
	SuccessThreshold int `json:"success_threshold" default:"3"` // successes to close circuit
	Timeout          int `json:"timeout" default:"60"`          // seconds before retry after failure
}

// DebugConfig controls debug and logging options
type DebugConfig struct {
	EnableDebugLogs  bool `json:"enable_debug_logs" default:"false"`
	LogAPIRequests   bool `json:"log_api_requests" default:"false"`
	LogMatchDetails  bool `json:"log_match_details" default:"false"`
	SaveAPIResponses bool `json:"save_api_responses" default:"false"` // For debugging API issues
}

// GetDefaultConfig returns a configuration with all default values set
func GetDefaultConfig() *Config {
	return &Config{
		Enabled: true,
		API: APIConfig{
			UserAgent:          "Viewra/2.0 (https://github.com/mantonx/viewra)",
			RequestTimeout:     30,
			MaxConnections:     5,
			RequestDelay:       1000,
			EnableCache:        true,
			CacheDurationHours: 168,
		},
		Features: FeatureConfig{
			EnableArtists:       true,
			EnableAlbums:        true,
			EnableTracks:        true,
			EnableCoverArt:      true,
			EnableRelationships: true,
			EnableGenres:        true,
			EnableTags:          true,
		},
		CoverArt: CoverArtConfig{
			DownloadCovers:     true,
			DownloadThumbnails: false,
			PreferredSize:      "500",
			MaxSizeMB:          5,
			SkipExisting:       true,
			CoverSources: []string{
				"Cover Art Archive",
				"Amazon",
				"Discogs",
				"Last.fm",
			},
		},
		Matching: MatchingConfig{
			MatchThreshold:    0.80,
			AutoEnrich:        true,
			OverwriteExisting: false,
			FuzzyMatching:     true,
			MatchByISRC:       true,
			MatchByBarcode:    true,
			MatchDuration:     true,
			DurationTolerance: 10,
		},
		Enrichment: EnrichmentConfig{
			IncludeAliases:        true,
			IncludeAnnotations:    false,
			IncludeRecordingLevel: true,
			IncludeWorkInfo:       true,
			IncludeLabelInfo:      true,
			IncludeCountryInfo:    true,
			MaxTagsPerItem:        10,
			MaxGenresPerItem:      5,
		},
		Reliability: ReliabilityConfig{
			MaxRetries:        3,
			InitialRetryDelay: 2,
			MaxRetryDelay:     30,
			BackoffMultiplier: 2.0,
			TimeoutMultiplier: 1.5,
			CircuitBreaker: CircuitBreakerConfig{
				FailureThreshold: 5,
				SuccessThreshold: 3,
				Timeout:          60,
			},
		},
		Debug: DebugConfig{
			EnableDebugLogs:  false,
			LogAPIRequests:   false,
			LogMatchDetails:  false,
			SaveAPIResponses: false,
		},
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Add validation logic here
	if c.API.RequestTimeout < 5 || c.API.RequestTimeout > 300 {
		return &ValidationError{Field: "api.request_timeout", Message: "must be between 5 and 300 seconds"}
	}

	if c.API.RequestDelay < 500 || c.API.RequestDelay > 5000 {
		return &ValidationError{Field: "api.request_delay", Message: "must be between 500 and 5000 milliseconds"}
	}

	if c.Matching.MatchThreshold < 0.1 || c.Matching.MatchThreshold > 1.0 {
		return &ValidationError{Field: "matching.match_threshold", Message: "must be between 0.1 and 1.0"}
	}

	if c.Matching.DurationTolerance < 1 || c.Matching.DurationTolerance > 60 {
		return &ValidationError{Field: "matching.duration_tolerance", Message: "must be between 1 and 60 seconds"}
	}

	if c.Reliability.MaxRetries < 1 || c.Reliability.MaxRetries > 10 {
		return &ValidationError{Field: "reliability.max_retries", Message: "must be between 1 and 10"}
	}

	if c.Enrichment.MaxTagsPerItem < 1 || c.Enrichment.MaxTagsPerItem > 50 {
		return &ValidationError{Field: "enrichment.max_tags_per_item", Message: "must be between 1 and 50"}
	}

	if c.CoverArt.MaxSizeMB < 1 || c.CoverArt.MaxSizeMB > 20 {
		return &ValidationError{Field: "cover_art.max_size_mb", Message: "must be between 1 and 20 MB"}
	}

	return nil
}

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return "validation error in field '" + e.Field + "': " + e.Message
}

// Helper methods for duration conversion
func (c *Config) GetRequestTimeout() time.Duration {
	return time.Duration(c.API.RequestTimeout) * time.Second
}

func (c *Config) GetRequestDelay() time.Duration {
	return time.Duration(c.API.RequestDelay) * time.Millisecond
}

func (c *Config) GetCacheDuration() time.Duration {
	return time.Duration(c.API.CacheDurationHours) * time.Hour
}

func (c *Config) GetInitialRetryDelay() time.Duration {
	return time.Duration(c.Reliability.InitialRetryDelay) * time.Second
}

func (c *Config) GetMaxRetryDelay() time.Duration {
	return time.Duration(c.Reliability.MaxRetryDelay) * time.Second
}

func (c *Config) GetCircuitBreakerTimeout() time.Duration {
	return time.Duration(c.Reliability.CircuitBreaker.Timeout) * time.Second
}
