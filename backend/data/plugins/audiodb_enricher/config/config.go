package config

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
)

// Config holds configuration for the AudioDB plugin
type Config struct {
	Enabled              bool    `json:"enabled"`
	APIKey               string  `json:"api_key,omitempty"`               // AudioDB API key (optional for basic usage)
	UserAgent            string  `json:"user_agent"`                      // User agent for API requests
	EnableArtwork        bool    `json:"enable_artwork"`                  // Whether to download artwork
	ArtworkMaxSize       int     `json:"artwork_max_size"`                // Max artwork size in pixels
	ArtworkQuality       string  `json:"artwork_quality"`                 // front, back, all
	MatchThreshold       float64 `json:"match_threshold"`                 // Minimum match score (0.0-1.0)
	AutoEnrich           bool    `json:"auto_enrich"`                     // Auto-enrich during scan
	OverwriteExisting    bool    `json:"overwrite_existing"`              // Overwrite existing metadata
	CacheDurationHours   int     `json:"cache_duration_hours"`            // Cache duration in hours
	RequestDelay         int     `json:"request_delay_ms"`                // Delay between API requests (ms)
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled:              true,
		UserAgent:            "Viewra AudioDB Enricher/1.0.0",
		EnableArtwork:        true,
		ArtworkMaxSize:       1200,
		ArtworkQuality:       "front",
		MatchThreshold:       0.75,
		AutoEnrich:           true,
		OverwriteExisting:    false,
		CacheDurationHours:   168, // 1 week
		RequestDelay:         1000, // 1 second between requests
	}
}

// ApplyOverrides applies configuration overrides from a map
func (c *Config) ApplyOverrides(overrides map[string]string) error {
	for key, value := range overrides {
		switch key {
		case "enabled":
			if val, err := strconv.ParseBool(value); err == nil {
				c.Enabled = val
			} else {
				return fmt.Errorf("invalid boolean value for enabled: %s", value)
			}
		case "api_key":
			c.APIKey = value
		case "user_agent":
			c.UserAgent = value
		case "enable_artwork":
			if val, err := strconv.ParseBool(value); err == nil {
				c.EnableArtwork = val
			} else {
				return fmt.Errorf("invalid boolean value for enable_artwork: %s", value)
			}
		case "artwork_max_size":
			if val, err := strconv.Atoi(value); err == nil {
				c.ArtworkMaxSize = val
			} else {
				return fmt.Errorf("invalid integer value for artwork_max_size: %s", value)
			}
		case "artwork_quality":
			if value == "front" || value == "back" || value == "all" {
				c.ArtworkQuality = value
			} else {
				return fmt.Errorf("invalid artwork_quality value: %s (must be front, back, or all)", value)
			}
		case "match_threshold":
			if val, err := strconv.ParseFloat(value, 64); err == nil {
				if val >= 0.0 && val <= 1.0 {
					c.MatchThreshold = val
				} else {
					return fmt.Errorf("match_threshold must be between 0.0 and 1.0: %f", val)
				}
			} else {
				return fmt.Errorf("invalid float value for match_threshold: %s", value)
			}
		case "auto_enrich":
			if val, err := strconv.ParseBool(value); err == nil {
				c.AutoEnrich = val
			} else {
				return fmt.Errorf("invalid boolean value for auto_enrich: %s", value)
			}
		case "overwrite_existing":
			if val, err := strconv.ParseBool(value); err == nil {
				c.OverwriteExisting = val
			} else {
				return fmt.Errorf("invalid boolean value for overwrite_existing: %s", value)
			}
		case "cache_duration_hours":
			if val, err := strconv.Atoi(value); err == nil {
				if val > 0 {
					c.CacheDurationHours = val
				} else {
					return fmt.Errorf("cache_duration_hours must be positive: %d", val)
				}
			} else {
				return fmt.Errorf("invalid integer value for cache_duration_hours: %s", value)
			}
		case "request_delay_ms":
			if val, err := strconv.Atoi(value); err == nil {
				if val >= 0 {
					c.RequestDelay = val
				} else {
					return fmt.Errorf("request_delay_ms must be non-negative: %d", val)
				}
			} else {
				return fmt.Errorf("invalid integer value for request_delay_ms: %s", value)
			}
		default:
			// Ignore unknown configuration keys
		}
	}
	
	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.MatchThreshold < 0.0 || c.MatchThreshold > 1.0 {
		return fmt.Errorf("match_threshold must be between 0.0 and 1.0")
	}
	
	if c.ArtworkQuality != "front" && c.ArtworkQuality != "back" && c.ArtworkQuality != "all" {
		return fmt.Errorf("artwork_quality must be 'front', 'back', or 'all'")
	}
	
	if c.ArtworkMaxSize <= 0 {
		return fmt.Errorf("artwork_max_size must be positive")
	}
	
	if c.CacheDurationHours <= 0 {
		return fmt.Errorf("cache_duration_hours must be positive")
	}
	
	if c.RequestDelay < 0 {
		return fmt.Errorf("request_delay_ms must be non-negative")
	}
	
	return nil
}

// ToMap converts the configuration to a map for serialization
func (c *Config) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"enabled":                c.Enabled,
		"api_key":                c.APIKey,
		"user_agent":             c.UserAgent,
		"enable_artwork":         c.EnableArtwork,
		"artwork_max_size":       c.ArtworkMaxSize,
		"artwork_quality":        c.ArtworkQuality,
		"match_threshold":        c.MatchThreshold,
		"auto_enrich":            c.AutoEnrich,
		"overwrite_existing":     c.OverwriteExisting,
		"cache_duration_hours":   c.CacheDurationHours,
		"request_delay_ms":       c.RequestDelay,
	}
}

// Implement GORM's database driver interface for storing configuration
func (c Config) Value() (driver.Value, error) {
	return json.Marshal(c)
}

func (c *Config) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into Config", value)
	}
	return json.Unmarshal(bytes, c)
} 