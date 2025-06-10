package config

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/mantonx/viewra/pkg/plugins"
)

// ExampleUsage demonstrates how to use the TMDb configuration management system
func ExampleUsage() {
	// 1. Initialize the configuration service
	configPath := filepath.Join(".", "tmdb_config.json")
	configService := NewTMDbConfigurationService(configPath)

	// Initialize with default configuration
	if err := configService.Initialize(); err != nil {
		log.Fatalf("Failed to initialize config service: %v", err)
	}

	// 2. Create a configuration manager for high-level operations
	configManager := NewConfigurationManager(configService)

	// 3. Setup initial configuration with API key
	apiKey := "your_32_character_tmdb_api_key_here"
	if err := configManager.SetupInitialConfiguration(apiKey); err != nil {
		log.Printf("Failed to setup initial config: %v", err)
	}

	// 4. Enable features with dependency checking
	// Enable movies first (no dependencies)
	if err := configManager.EnableFeature("movies", []string{}); err != nil {
		log.Printf("Failed to enable movies: %v", err)
	}

	// Enable artwork (depends on having content types enabled)
	if err := configManager.EnableFeature("artwork", []string{"movies"}); err != nil {
		log.Printf("Failed to enable artwork: %v", err)
	}

	// 5. Auto-configure for performance
	if err := configManager.AutoConfigureForPerformance("balanced"); err != nil {
		log.Printf("Failed to auto-configure: %v", err)
	}

	// 6. Get configuration summary
	summary := configManager.GetConfigurationSummary()
	fmt.Printf("Configuration Summary: %+v\n", summary)

	// 7. Update specific settings
	if err := configService.UpdateRateLimit(0.8); err != nil {
		log.Printf("Failed to update rate limit: %v", err)
	}

	if err := configService.UpdateCacheDuration(72); err != nil {
		log.Printf("Failed to update cache duration: %v", err)
	}

	// 8. Validate configuration
	validator := &ConfigurationValidator{}
	currentConfig := configService.GetTMDbConfig()

	if issues := validator.ValidateConfiguration(currentConfig); len(issues) > 0 {
		fmt.Printf("Configuration issues found: %v\n", issues)
	}

	if recommendations := validator.GetRecommendations(currentConfig); len(recommendations) > 0 {
		fmt.Printf("Configuration recommendations: %v\n", recommendations)
	}

	// 9. Export configuration for backup
	if err := configManager.ExportConfigurationToFile("tmdb_config_backup.json"); err != nil {
		log.Printf("Failed to export config: %v", err)
	}

	fmt.Println("Configuration management example completed successfully!")
}

// ExampleAdvancedUsage shows advanced configuration patterns
func ExampleAdvancedUsage() {
	configPath := filepath.Join(".", "tmdb_config.json")
	configService := NewTMDbConfigurationService(configPath)
	configManager := NewConfigurationManager(configService)

	// Initialize
	if err := configService.Initialize(); err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	// Example: Configure for different environments
	environments := map[string]string{
		"development": "conservative",
		"staging":     "balanced",
		"production":  "aggressive",
	}

	currentEnv := "production" // This would come from environment variable
	if perfMode, exists := environments[currentEnv]; exists {
		if err := configManager.AutoConfigureForPerformance(perfMode); err != nil {
			log.Printf("Failed to configure for %s: %v", currentEnv, err)
		}
		fmt.Printf("Configured for %s environment with %s performance profile\n", currentEnv, perfMode)
	}

	// Example: Feature flag management
	featureDependencies := map[string][]string{
		"artwork":     {"movies", "tv_shows"},
		"episodes":    {"tv_shows"},
		"auto_enrich": {"movies"},
	}

	// Enable features with proper dependency checking
	for feature, deps := range featureDependencies {
		if err := configManager.EnableFeature(feature, deps); err != nil {
			log.Printf("Could not enable %s: %v", feature, err)
		} else {
			fmt.Printf("Enabled feature: %s\n", feature)
		}
	}

	// Example: Configuration comparison
	originalConfig := configService.GetTMDbConfig()

	// Make some changes
	configService.UpdateFeature("debug", true)
	configService.UpdateRateLimit(1.2)

	newConfig := configService.GetTMDbConfig()
	diff := configManager.GetConfigurationDiff(originalConfig)

	if len(diff) > 0 {
		fmt.Printf("Configuration changes detected: %+v\n", diff)
		fmt.Printf("New config rate limit: %.2f\n", newConfig.API.RateLimit)
	}

	// Example: Rollback configuration
	if err := configService.UpdateTMDbConfig(originalConfig); err != nil {
		log.Printf("Failed to rollback configuration: %v", err)
	} else {
		fmt.Println("Configuration rolled back successfully")
	}
}

// ExampleConfigurationCallbacks shows how to use configuration change callbacks
func ExampleConfigurationCallbacks() {
	configPath := filepath.Join(".", "tmdb_config.json")
	configService := NewTMDbConfigurationService(configPath)

	// Add a custom callback that logs configuration changes
	configService.AddConfigurationCallback(func(oldConfig, newConfig *plugins.PluginConfiguration) error {
		fmt.Printf("Configuration changed!\n")

		// Check if API settings changed
		oldAPI, oldExists := oldConfig.Settings["api"]
		newAPI, newExists := newConfig.Settings["api"]

		if oldExists && newExists {
			fmt.Printf("API settings changed from %+v to %+v\n", oldAPI, newAPI)
		}

		// Check if features changed
		for feature, enabled := range newConfig.Features {
			if oldEnabled, exists := oldConfig.Features[feature]; exists && oldEnabled != enabled {
				fmt.Printf("Feature '%s' changed from %v to %v\n", feature, oldEnabled, enabled)
			}
		}

		return nil
	})

	// Initialize and make some changes to trigger callbacks
	if err := configService.Initialize(); err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	// These changes will trigger the callback
	configService.UpdateFeature("movies", true)
	configService.UpdateRateLimit(0.7)

	fmt.Println("Configuration callbacks example completed!")
}

// ExampleProductionSetup shows a typical production configuration setup
func ExampleProductionSetup() {
	configPath := filepath.Join("/etc/viewra/plugins/tmdb", "config.json")
	configService := NewTMDbConfigurationService(configPath)
	configManager := NewConfigurationManager(configService)

	// Production configuration
	productionConfig := &Config{
		API: APIConfig{
			Key:        "production_api_key_from_env",
			RateLimit:  0.4, // Conservative for production
			TimeoutSec: 30,
			Language:   "en-US",
			Region:     "US",
		},
		Features: FeaturesConfig{
			EnableMovies:      true,
			EnableTVShows:     true,
			EnableEpisodes:    true,
			EnableArtwork:     true,
			AutoEnrich:        true,
			OverwriteExisting: false, // Never overwrite in production
		},
		Artwork: ArtworkConfig{
			DownloadPosters:    true,
			DownloadBackdrops:  false, // Save bandwidth
			PosterSize:         "w500",
			MaxAssetSizeMB:     5, // Conservative limit
			SkipExistingAssets: true,
		},
		Cache: CacheConfig{
			DurationHours:   336, // 2 weeks for production
			CleanupInterval: 24,
		},
		Reliability: ReliabilityConfig{
			MaxRetries:           3,
			InitialDelaySeconds:  5,
			BackoffMultiplier:    2.0,
			RetryFailedDownloads: true,
		},
		Debug: DebugConfig{
			Enabled:        false, // Disable debug in production
			LogAPIRequests: false,
		},
	}

	// Apply production configuration
	if err := configService.UpdateTMDbConfig(productionConfig); err != nil {
		log.Fatalf("Failed to apply production config: %v", err)
	}

	// Validate production configuration
	validator := &ConfigurationValidator{}
	if issues := validator.ValidateConfiguration(productionConfig); len(issues) > 0 {
		log.Printf("Production config validation issues: %v", issues)
	}

	// Get recommendations for production
	if recommendations := validator.GetRecommendations(productionConfig); len(recommendations) > 0 {
		log.Printf("Production config recommendations: %v", recommendations)
	}

	// Export production configuration for backup
	if err := configManager.ExportConfigurationToFile("production_config_backup.json"); err != nil {
		log.Printf("Failed to export production config: %v", err)
	}

	fmt.Println("Production configuration setup completed!")
}
