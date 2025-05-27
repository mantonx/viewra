// Package config handles configuration loading and validation for the MusicBrainz enricher plugin.
package config

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the plugin configuration
type Config struct {
	Enabled             bool    `yaml:"enabled" json:"enabled"`
	APIRateLimit        float64 `yaml:"api_rate_limit" json:"api_rate_limit"`
	UserAgent           string  `yaml:"user_agent" json:"user_agent"`
	EnableArtwork       bool    `yaml:"enable_artwork" json:"enable_artwork"`
	ArtworkMaxSize      int     `yaml:"artwork_max_size" json:"artwork_max_size"`
	ArtworkQuality      string  `yaml:"artwork_quality" json:"artwork_quality"`
	MatchThreshold      float64 `yaml:"match_threshold" json:"match_threshold"`
	AutoEnrich          bool    `yaml:"auto_enrich" json:"auto_enrich"`
	OverwriteExisting   bool    `yaml:"overwrite_existing" json:"overwrite_existing"`
	CacheDurationHours  int     `yaml:"cache_duration_hours" json:"cache_duration_hours"`
}

// Default returns a configuration with sensible defaults
func Default() *Config {
	return &Config{
		Enabled:            true,
		APIRateLimit:       0.8,
		UserAgent:          "Viewra/1.0.0 (https://github.com/viewra/viewra)",
		EnableArtwork:      true,
		ArtworkMaxSize:     1200,
		ArtworkQuality:     "front",
		MatchThreshold:     0.85,
		AutoEnrich:         false,
		OverwriteExisting:  false,
		CacheDurationHours: 168, // 1 week
	}
}

// LoadFromYAML loads configuration from YAML data
func LoadFromYAML(data []byte) (*Config, error) {
	var manifest struct {
		Config Config `yaml:"config"`
	}
	
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}
	
	cfg := &manifest.Config
	
	// Apply defaults for missing values
	if cfg.UserAgent == "" {
		cfg.UserAgent = Default().UserAgent
	}
	if cfg.APIRateLimit == 0 {
		cfg.APIRateLimit = Default().APIRateLimit
	}
	if cfg.MatchThreshold == 0 {
		cfg.MatchThreshold = Default().MatchThreshold
	}
	if cfg.CacheDurationHours == 0 {
		cfg.CacheDurationHours = Default().CacheDurationHours
	}
	if cfg.ArtworkMaxSize == 0 {
		cfg.ArtworkMaxSize = Default().ArtworkMaxSize
	}
	if cfg.ArtworkQuality == "" {
		cfg.ArtworkQuality = Default().ArtworkQuality
	}
	
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	return cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.APIRateLimit < 0.1 || c.APIRateLimit > 1.0 {
		return fmt.Errorf("api_rate_limit must be between 0.1 and 1.0, got %f", c.APIRateLimit)
	}
	
	if c.MatchThreshold < 0.5 || c.MatchThreshold > 1.0 {
		return fmt.Errorf("match_threshold must be between 0.5 and 1.0, got %f", c.MatchThreshold)
	}
	
	if c.ArtworkMaxSize < 250 || c.ArtworkMaxSize > 2000 {
		return fmt.Errorf("artwork_max_size must be between 250 and 2000, got %d", c.ArtworkMaxSize)
	}
	
	if c.CacheDurationHours < 1 || c.CacheDurationHours > 8760 {
		return fmt.Errorf("cache_duration_hours must be between 1 and 8760, got %d", c.CacheDurationHours)
	}
	
	validQualities := []string{"front", "back", "booklet", "medium", "obi", "spine", "track", "liner", "sticker", "poster"}
	validQuality := false
	for _, quality := range validQualities {
		if c.ArtworkQuality == quality {
			validQuality = true
			break
		}
	}
	if !validQuality {
		return fmt.Errorf("artwork_quality must be one of %v, got %s", validQualities, c.ArtworkQuality)
	}
	
	return nil
}

// RateLimitInterval returns the time interval between API requests
func (c *Config) RateLimitInterval() time.Duration {
	return time.Duration(1.0/c.APIRateLimit) * time.Second
}

// CacheDuration returns the cache duration as a time.Duration
func (c *Config) CacheDuration() time.Duration {
	return time.Duration(c.CacheDurationHours) * time.Hour
} 