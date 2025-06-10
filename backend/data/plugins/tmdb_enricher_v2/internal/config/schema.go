package config

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mantonx/viewra/pkg/plugins"
)

// TMDbConfigurationService extends the base configuration service with TMDb-specific functionality
type TMDbConfigurationService struct {
	*plugins.BaseConfigurationService
	config *Config
}

// NewTMDbConfigurationService creates a new TMDb-specific configuration service
func NewTMDbConfigurationService(configPath string) *TMDbConfigurationService {
	baseService := plugins.NewBaseConfigurationService("tmdb_enricher", configPath)

	service := &TMDbConfigurationService{
		BaseConfigurationService: baseService,
		config:                   DefaultConfig(),
	}

	// Set TMDb-specific schema
	service.SetConfigurationSchema(service.createTMDbSchema())

	// Add TMDb-specific configuration callback
	service.AddConfigurationCallback(service.onConfigurationChanged)

	return service
}

// GetTMDbConfig returns the current TMDb configuration
func (c *TMDbConfigurationService) GetTMDbConfig() *Config {
	return c.config
}

// UpdateTMDbConfig updates the TMDb configuration
func (c *TMDbConfigurationService) UpdateTMDbConfig(newConfig *Config) error {
	// Validate TMDb-specific configuration
	if err := newConfig.Validate(); err != nil {
		return fmt.Errorf("TMDb config validation failed: %w", err)
	}

	// Convert to plugin configuration format
	pluginConfig := c.tmdbToPluginConfig(newConfig)

	// Use base service to update
	ctx := c.BaseConfigurationService
	return ctx.UpdateConfiguration(nil, pluginConfig)
}

// GetAPIConfig returns the API configuration
func (c *TMDbConfigurationService) GetAPIConfig() *APIConfig {
	return &c.config.API
}

// GetFeaturesConfig returns the features configuration
func (c *TMDbConfigurationService) GetFeaturesConfig() *FeaturesConfig {
	return &c.config.Features
}

// GetArtworkConfig returns the artwork configuration
func (c *TMDbConfigurationService) GetArtworkConfig() *ArtworkConfig {
	return &c.config.Artwork
}

// GetCacheConfig returns the cache configuration
func (c *TMDbConfigurationService) GetCacheConfig() *CacheConfig {
	return &c.config.Cache
}

// UpdateAPIKey updates the TMDb API key
func (c *TMDbConfigurationService) UpdateAPIKey(apiKey string) error {
	c.config.API.Key = apiKey
	return c.UpdateTMDbConfig(c.config)
}

// UpdateFeature toggles a specific feature
func (c *TMDbConfigurationService) UpdateFeature(feature string, enabled bool) error {
	switch feature {
	case "movies":
		c.config.Features.EnableMovies = enabled
	case "tv_shows":
		c.config.Features.EnableTVShows = enabled
	case "episodes":
		c.config.Features.EnableEpisodes = enabled
	case "artwork":
		c.config.Features.EnableArtwork = enabled
	case "auto_enrich":
		c.config.Features.AutoEnrich = enabled
	case "overwrite_existing":
		c.config.Features.OverwriteExisting = enabled
	default:
		return fmt.Errorf("unknown feature: %s", feature)
	}

	return c.UpdateTMDbConfig(c.config)
}

// UpdateRateLimit updates the API rate limit
func (c *TMDbConfigurationService) UpdateRateLimit(rateLimit float64) error {
	if rateLimit <= 0 {
		return fmt.Errorf("rate limit must be positive")
	}

	c.config.API.RateLimit = rateLimit
	c.config.API.DelayMs = int(1000 / rateLimit) // Calculate delay from rate limit

	return c.UpdateTMDbConfig(c.config)
}

// UpdateCacheDuration updates the cache duration
func (c *TMDbConfigurationService) UpdateCacheDuration(hours int) error {
	if hours < 1 {
		return fmt.Errorf("cache duration must be at least 1 hour")
	}

	c.config.Cache.DurationHours = hours
	return c.UpdateTMDbConfig(c.config)
}

// Initialize loads and validates the TMDb configuration
func (c *TMDbConfigurationService) Initialize() error {
	// Initialize base service
	if err := c.BaseConfigurationService.Initialize(); err != nil {
		return err
	}

	// Load TMDb-specific configuration
	pluginConfig, err := c.BaseConfigurationService.GetConfiguration(nil)
	if err != nil {
		return err
	}

	// Convert plugin config to TMDb config
	c.config = c.pluginToTMDbConfig(pluginConfig)

	// Validate TMDb configuration
	if err := c.config.Validate(); err != nil {
		// If validation fails, reset to default and save
		c.config = DefaultConfig()
		return c.UpdateTMDbConfig(c.config)
	}

	return nil
}

// Private helper methods

// onConfigurationChanged is called when configuration changes
func (c *TMDbConfigurationService) onConfigurationChanged(oldConfig, newConfig *plugins.PluginConfiguration) error {
	// Convert plugin config to TMDb config
	c.config = c.pluginToTMDbConfig(newConfig)

	// Validate the new configuration
	if err := c.config.Validate(); err != nil {
		return fmt.Errorf("invalid TMDb configuration: %w", err)
	}

	return nil
}

// tmdbToPluginConfig converts TMDb config to plugin configuration format
func (c *TMDbConfigurationService) tmdbToPluginConfig(tmdbConfig *Config) *plugins.PluginConfiguration {
	// Serialize TMDb config to JSON
	configBytes, _ := json.Marshal(tmdbConfig)
	var configMap map[string]interface{}
	json.Unmarshal(configBytes, &configMap)

	return &plugins.PluginConfiguration{
		Version:  "1.0.0",
		Enabled:  true,
		Settings: configMap,
		Features: map[string]bool{
			"movies":             tmdbConfig.Features.EnableMovies,
			"tv_shows":           tmdbConfig.Features.EnableTVShows,
			"episodes":           tmdbConfig.Features.EnableEpisodes,
			"artwork":            tmdbConfig.Features.EnableArtwork,
			"auto_enrich":        tmdbConfig.Features.AutoEnrich,
			"overwrite_existing": tmdbConfig.Features.OverwriteExisting,
			"debug":              tmdbConfig.Debug.Enabled,
		},
		Thresholds: &plugins.HealthThresholds{
			MaxMemoryUsage:      256 * 1024 * 1024, // 256MB
			MaxCPUUsage:         70.0,
			MaxErrorRate:        5.0,
			MaxResponseTime:     time.Duration(tmdbConfig.API.TimeoutSec) * time.Second,
			HealthCheckInterval: 30 * time.Second,
		},
		LastModified: time.Now(),
		ModifiedBy:   "tmdb_plugin",
	}
}

// pluginToTMDbConfig converts plugin configuration to TMDb config format
func (c *TMDbConfigurationService) pluginToTMDbConfig(pluginConfig *plugins.PluginConfiguration) *Config {
	// Try to extract TMDb config from settings
	if configData, exists := pluginConfig.Settings["tmdb_config"]; exists {
		if configBytes, err := json.Marshal(configData); err == nil {
			var tmdbConfig Config
			if err := json.Unmarshal(configBytes, &tmdbConfig); err == nil {
				return &tmdbConfig
			}
		}
	}

	// Fallback: construct from individual settings
	config := DefaultConfig()

	// Map plugin settings to TMDb config
	if val, exists := pluginConfig.Settings["api"]; exists {
		if apiMap, ok := val.(map[string]interface{}); ok {
			if key, ok := apiMap["key"].(string); ok {
				config.API.Key = key
			}
			if rateLimit, ok := apiMap["rate_limit"].(float64); ok {
				config.API.RateLimit = rateLimit
			}
			if timeout, ok := apiMap["timeout_sec"].(float64); ok {
				config.API.TimeoutSec = int(timeout)
			}
		}
	}

	// Map features
	config.Features.EnableMovies = pluginConfig.Features["movies"]
	config.Features.EnableTVShows = pluginConfig.Features["tv_shows"]
	config.Features.EnableEpisodes = pluginConfig.Features["episodes"]
	config.Features.EnableArtwork = pluginConfig.Features["artwork"]
	config.Features.AutoEnrich = pluginConfig.Features["auto_enrich"]
	config.Features.OverwriteExisting = pluginConfig.Features["overwrite_existing"]
	config.Debug.Enabled = pluginConfig.Features["debug"]

	return config
}

// createTMDbSchema creates the JSON schema for TMDb configuration
func (c *TMDbConfigurationService) createTMDbSchema() *plugins.ConfigurationSchema {
	return &plugins.ConfigurationSchema{
		Schema: map[string]interface{}{
			"api": map[string]interface{}{
				"type":        "object",
				"description": "TMDb API configuration",
				"properties": map[string]interface{}{
					"key": map[string]interface{}{
						"type":        "string",
						"description": "TMDb API key",
						"minLength":   32,
						"pattern":     "^[a-f0-9]{32}$",
					},
					"rate_limit": map[string]interface{}{
						"type":        "number",
						"description": "API requests per second",
						"minimum":     0.1,
						"maximum":     10.0,
						"default":     0.6,
					},
					"timeout_sec": map[string]interface{}{
						"type":        "integer",
						"description": "Request timeout in seconds",
						"minimum":     5,
						"maximum":     300,
						"default":     60,
					},
					"language": map[string]interface{}{
						"type":        "string",
						"description": "Preferred language (ISO 639-1)",
						"pattern":     "^[a-z]{2}-[A-Z]{2}$",
						"default":     "en-US",
					},
				},
				"required": []string{"key"},
			},
			"features": map[string]interface{}{
				"type":        "object",
				"description": "Feature toggles",
				"properties": map[string]interface{}{
					"enable_movies": map[string]interface{}{
						"type":        "boolean",
						"description": "Enable movie enrichment",
						"default":     true,
					},
					"enable_tv_shows": map[string]interface{}{
						"type":        "boolean",
						"description": "Enable TV show enrichment",
						"default":     true,
					},
					"enable_episodes": map[string]interface{}{
						"type":        "boolean",
						"description": "Enable episode enrichment",
						"default":     true,
					},
					"enable_artwork": map[string]interface{}{
						"type":        "boolean",
						"description": "Enable artwork downloads",
						"default":     true,
					},
					"auto_enrich": map[string]interface{}{
						"type":        "boolean",
						"description": "Automatically enrich during scanning",
						"default":     true,
					},
				},
			},
			"artwork": map[string]interface{}{
				"type":        "object",
				"description": "Artwork download settings",
				"properties": map[string]interface{}{
					"download_posters": map[string]interface{}{
						"type":        "boolean",
						"description": "Download poster images",
						"default":     true,
					},
					"download_backdrops": map[string]interface{}{
						"type":        "boolean",
						"description": "Download backdrop images",
						"default":     false,
					},
					"poster_size": map[string]interface{}{
						"type":        "string",
						"description": "Poster image size",
						"enum":        []string{"w92", "w154", "w185", "w342", "w500", "w780", "original"},
						"default":     "w500",
					},
					"max_asset_size_mb": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum asset size in MB",
						"minimum":     1,
						"maximum":     100,
						"default":     10,
					},
				},
			},
			"cache": map[string]interface{}{
				"type":        "object",
				"description": "Cache configuration",
				"properties": map[string]interface{}{
					"duration_hours": map[string]interface{}{
						"type":        "integer",
						"description": "Cache duration in hours",
						"minimum":     1,
						"maximum":     8760, // 1 year
						"default":     168,  // 1 week
					},
					"cleanup_interval": map[string]interface{}{
						"type":        "integer",
						"description": "Cleanup interval in hours",
						"minimum":     1,
						"maximum":     168,
						"default":     24,
					},
				},
			},
		},
		Examples: map[string]interface{}{
			"minimal": map[string]interface{}{
				"api": map[string]interface{}{
					"key": "your_tmdb_api_key_here",
				},
			},
			"full": map[string]interface{}{
				"api": map[string]interface{}{
					"key":         "your_tmdb_api_key_here",
					"rate_limit":  0.6,
					"timeout_sec": 60,
					"language":    "en-US",
				},
				"features": map[string]interface{}{
					"enable_movies":   true,
					"enable_tv_shows": true,
					"enable_episodes": true,
					"enable_artwork":  true,
					"auto_enrich":     true,
				},
				"artwork": map[string]interface{}{
					"download_posters":   true,
					"download_backdrops": false,
					"poster_size":        "w500",
					"max_asset_size_mb":  10,
				},
				"cache": map[string]interface{}{
					"duration_hours":   168,
					"cleanup_interval": 24,
				},
			},
		},
		Defaults: map[string]interface{}{
			"api_rate_limit":          0.6,
			"api_timeout_sec":         60,
			"api_language":            "en-US",
			"features_enable_movies":  true,
			"features_enable_tv":      true,
			"features_enable_artwork": true,
			"artwork_poster_size":     "w500",
			"cache_duration_hours":    168,
		},
	}
}
