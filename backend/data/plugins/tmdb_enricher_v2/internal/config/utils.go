package config

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ConfigurationManager provides high-level configuration management utilities
type ConfigurationManager struct {
	service *TMDbConfigurationService
}

// NewConfigurationManager creates a new configuration manager
func NewConfigurationManager(service *TMDbConfigurationService) *ConfigurationManager {
	return &ConfigurationManager{
		service: service,
	}
}

// ValidateAPIKey validates a TMDb API key format
func ValidateAPIKey(apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	// TMDb API keys are typically 32 character hexadecimal strings
	if len(apiKey) != 32 {
		return fmt.Errorf("API key must be 32 characters long")
	}

	// Check if it's hexadecimal
	matched, err := regexp.MatchString("^[a-f0-9]{32}$", apiKey)
	if err != nil {
		return fmt.Errorf("invalid API key format: %w", err)
	}

	if !matched {
		return fmt.Errorf("API key must contain only lowercase hexadecimal characters")
	}

	return nil
}

// ValidateLanguageCode validates an ISO 639-1 language code
func ValidateLanguageCode(langCode string) error {
	if langCode == "" {
		return fmt.Errorf("language code cannot be empty")
	}

	// Format: en-US, fr-FR, etc.
	matched, err := regexp.MatchString("^[a-z]{2}-[A-Z]{2}$", langCode)
	if err != nil {
		return fmt.Errorf("invalid language code format: %w", err)
	}

	if !matched {
		return fmt.Errorf("language code must be in format 'en-US'")
	}

	return nil
}

// SetupInitialConfiguration sets up the plugin with initial configuration
func (cm *ConfigurationManager) SetupInitialConfiguration(apiKey string) error {
	// Validate API key
	if err := ValidateAPIKey(apiKey); err != nil {
		return fmt.Errorf("invalid API key: %w", err)
	}

	// Get current config
	config := cm.service.GetTMDbConfig()

	// Update API key
	config.API.Key = apiKey

	// Set conservative defaults for initial setup
	config.API.RateLimit = 0.5                // Conservative rate limit
	config.Features.AutoEnrich = false        // Disable auto-enrich initially
	config.Features.OverwriteExisting = false // Don't overwrite existing data

	// Save configuration
	if err := cm.service.UpdateTMDbConfig(config); err != nil {
		return fmt.Errorf("failed to save initial configuration: %w", err)
	}

	return nil
}

// EnableFeature enables a specific feature with validation
func (cm *ConfigurationManager) EnableFeature(feature string, dependencies []string) error {
	// Check if dependencies are met
	for _, dep := range dependencies {
		if !cm.IsFeatureEnabled(dep) {
			return fmt.Errorf("feature '%s' requires '%s' to be enabled first", feature, dep)
		}
	}

	return cm.service.UpdateFeature(feature, true)
}

// DisableFeature disables a feature and any dependent features
func (cm *ConfigurationManager) DisableFeature(feature string, dependents []string) error {
	// Disable dependents first
	for _, dep := range dependents {
		if cm.IsFeatureEnabled(dep) {
			if err := cm.service.UpdateFeature(dep, false); err != nil {
				return fmt.Errorf("failed to disable dependent feature '%s': %w", dep, err)
			}
		}
	}

	return cm.service.UpdateFeature(feature, false)
}

// IsFeatureEnabled checks if a feature is currently enabled
func (cm *ConfigurationManager) IsFeatureEnabled(feature string) bool {
	config := cm.service.GetTMDbConfig()

	switch feature {
	case "movies":
		return config.Features.EnableMovies
	case "tv_shows":
		return config.Features.EnableTVShows
	case "episodes":
		return config.Features.EnableEpisodes
	case "artwork":
		return config.Features.EnableArtwork
	case "auto_enrich":
		return config.Features.AutoEnrich
	case "overwrite_existing":
		return config.Features.OverwriteExisting
	case "debug":
		return config.Debug.Enabled
	default:
		return false
	}
}

// GetConfigurationSummary returns a human-readable configuration summary
func (cm *ConfigurationManager) GetConfigurationSummary() map[string]interface{} {
	config := cm.service.GetTMDbConfig()

	return map[string]interface{}{
		"plugin_status": map[string]interface{}{
			"api_configured": config.API.Key != "",
			"auto_enrich":    config.Features.AutoEnrich,
			"overwrite_mode": config.Features.OverwriteExisting,
		},
		"features": map[string]bool{
			"movies":   config.Features.EnableMovies,
			"tv_shows": config.Features.EnableTVShows,
			"episodes": config.Features.EnableEpisodes,
			"artwork":  config.Features.EnableArtwork,
			"debug":    config.Debug.Enabled,
		},
		"api_settings": map[string]interface{}{
			"rate_limit":  config.API.RateLimit,
			"timeout_sec": config.API.TimeoutSec,
			"language":    config.API.Language,
			"region":      config.API.Region,
		},
		"artwork_settings": map[string]interface{}{
			"download_posters":   config.Artwork.DownloadPosters,
			"download_backdrops": config.Artwork.DownloadBackdrops,
			"poster_size":        config.Artwork.PosterSize,
			"max_asset_size_mb":  config.Artwork.MaxAssetSizeMB,
		},
		"cache_settings": map[string]interface{}{
			"duration_hours":   config.Cache.DurationHours,
			"cleanup_interval": config.Cache.CleanupInterval,
		},
		"performance": map[string]interface{}{
			"max_retries":       config.Reliability.MaxRetries,
			"initial_delay_sec": config.Reliability.InitialDelaySeconds,
			"max_delay_sec":     config.Reliability.MaxDelaySeconds,
		},
	}
}

// AutoConfigureForPerformance automatically configures settings for optimal performance
func (cm *ConfigurationManager) AutoConfigureForPerformance(mode string) error {
	config := cm.service.GetTMDbConfig()

	switch strings.ToLower(mode) {
	case "conservative":
		config.API.RateLimit = 0.3 // Very conservative
		config.API.DelayMs = 3500
		config.Reliability.MaxRetries = 3
		config.Cache.DurationHours = 336 // 2 weeks

	case "balanced":
		config.API.RateLimit = 0.6 // Default
		config.API.DelayMs = 1800
		config.Reliability.MaxRetries = 5
		config.Cache.DurationHours = 168 // 1 week

	case "aggressive":
		config.API.RateLimit = 1.0 // Fast but risky
		config.API.DelayMs = 1000
		config.Reliability.MaxRetries = 2
		config.Cache.DurationHours = 72 // 3 days

	default:
		return fmt.Errorf("unknown performance mode: %s (valid: conservative, balanced, aggressive)", mode)
	}

	return cm.service.UpdateTMDbConfig(config)
}

// ImportConfigurationFromFile imports configuration from a JSON file
func (cm *ConfigurationManager) ImportConfigurationFromFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read configuration file: %w", err)
	}

	// Try to parse as TMDb config first
	var tmdbConfig Config
	if err := json.Unmarshal(data, &tmdbConfig); err != nil {
		return fmt.Errorf("failed to parse configuration file: %w", err)
	}

	// Validate the imported configuration
	if err := tmdbConfig.Validate(); err != nil {
		return fmt.Errorf("imported configuration is invalid: %w", err)
	}

	return cm.service.UpdateTMDbConfig(&tmdbConfig)
}

// ExportConfigurationToFile exports current configuration to a JSON file
func (cm *ConfigurationManager) ExportConfigurationToFile(filePath string) error {
	config := cm.service.GetTMDbConfig()

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}

	return nil
}

// GetConfigurationDiff compares current config with another config and returns differences
func (cm *ConfigurationManager) GetConfigurationDiff(otherConfig *Config) map[string]interface{} {
	current := cm.service.GetTMDbConfig()
	diff := make(map[string]interface{})

	// Compare API settings
	if current.API.Key != otherConfig.API.Key {
		diff["api_key"] = map[string]interface{}{
			"current": maskAPIKey(current.API.Key),
			"other":   maskAPIKey(otherConfig.API.Key),
		}
	}

	if current.API.RateLimit != otherConfig.API.RateLimit {
		diff["rate_limit"] = map[string]interface{}{
			"current": current.API.RateLimit,
			"other":   otherConfig.API.RateLimit,
		}
	}

	// Compare features
	featuresChanged := false
	featureDiff := make(map[string]interface{})

	if current.Features.EnableMovies != otherConfig.Features.EnableMovies {
		featureDiff["movies"] = map[string]bool{
			"current": current.Features.EnableMovies,
			"other":   otherConfig.Features.EnableMovies,
		}
		featuresChanged = true
	}

	if current.Features.EnableTVShows != otherConfig.Features.EnableTVShows {
		featureDiff["tv_shows"] = map[string]bool{
			"current": current.Features.EnableTVShows,
			"other":   otherConfig.Features.EnableTVShows,
		}
		featuresChanged = true
	}

	if featuresChanged {
		diff["features"] = featureDiff
	}

	return diff
}

// Helper function to mask API key for logging/display
func maskAPIKey(apiKey string) string {
	if len(apiKey) <= 8 {
		return strings.Repeat("*", len(apiKey))
	}
	return apiKey[:4] + strings.Repeat("*", len(apiKey)-8) + apiKey[len(apiKey)-4:]
}

// ConfigurationValidator provides additional validation utilities
type ConfigurationValidator struct{}

// ValidateConfiguration performs comprehensive validation
func (cv *ConfigurationValidator) ValidateConfiguration(config *Config) []string {
	var issues []string

	// API validation
	if err := ValidateAPIKey(config.API.Key); err != nil {
		issues = append(issues, fmt.Sprintf("API Key: %v", err))
	}

	if config.API.RateLimit <= 0 || config.API.RateLimit > 10 {
		issues = append(issues, "API rate limit should be between 0.1 and 10 requests/second")
	}

	if config.API.TimeoutSec < 5 || config.API.TimeoutSec > 300 {
		issues = append(issues, "API timeout should be between 5 and 300 seconds")
	}

	// Language validation
	if err := ValidateLanguageCode(config.API.Language); err != nil {
		issues = append(issues, fmt.Sprintf("Language: %v", err))
	}

	// Artwork validation
	if config.Artwork.MaxAssetSizeMB <= 0 || config.Artwork.MaxAssetSizeMB > 100 {
		issues = append(issues, "Max asset size should be between 1 and 100 MB")
	}

	validSizes := []string{"w92", "w154", "w185", "w342", "w500", "w780", "original"}
	posterSizeValid := false
	for _, size := range validSizes {
		if config.Artwork.PosterSize == size {
			posterSizeValid = true
			break
		}
	}
	if !posterSizeValid {
		issues = append(issues, fmt.Sprintf("Invalid poster size '%s', valid options: %v", config.Artwork.PosterSize, validSizes))
	}

	// Cache validation
	if config.Cache.DurationHours < 1 || config.Cache.DurationHours > 8760 {
		issues = append(issues, "Cache duration should be between 1 hour and 1 year")
	}

	return issues
}

// GetRecommendations provides configuration recommendations
func (cv *ConfigurationValidator) GetRecommendations(config *Config) []string {
	var recommendations []string

	// Performance recommendations
	if config.API.RateLimit > 1.0 {
		recommendations = append(recommendations, "Consider reducing API rate limit to avoid hitting TMDb rate limits")
	}

	if config.Cache.DurationHours < 24 {
		recommendations = append(recommendations, "Consider increasing cache duration to reduce API calls")
	}

	if config.Artwork.DownloadBackdrops && config.Artwork.MaxAssetSizeMB < 5 {
		recommendations = append(recommendations, "Backdrop images are typically large; consider increasing max asset size")
	}

	// Feature recommendations
	if config.Features.EnableArtwork && !config.Features.EnableMovies && !config.Features.EnableTVShows {
		recommendations = append(recommendations, "Artwork download enabled but no content types enabled")
	}

	if config.Features.OverwriteExisting && config.Features.AutoEnrich {
		recommendations = append(recommendations, "Auto-enrich with overwrite enabled may modify existing metadata")
	}

	return recommendations
}
