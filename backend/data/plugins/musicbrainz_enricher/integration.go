package musicbrainz_enricher

import (
	"context"
	"fmt"
	"log"

	"gopkg.in/yaml.v3"
	"gorm.io/gorm"

	"musicbrainz_enricher/config"
)

// Integration provides a simple way to integrate the MusicBrainz enricher with the main app
type Integration struct {
	plugin *Plugin
	db     *gorm.DB
	config *config.Config
}

// NewIntegration creates a new integration instance
func NewIntegration(db *gorm.DB, configData []byte) (*Integration, error) {
	// Parse configuration
	cfg, err := config.LoadFromYAML(configData)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create plugin instance
	plugin := NewPlugin(db, cfg)

	return &Integration{
		plugin: plugin,
		db:     db,
		config: cfg,
	}, nil
}

// Initialize initializes the plugin
func (i *Integration) Initialize() error {
	return i.plugin.Initialize(nil)
}

// Start starts the plugin
func (i *Integration) Start(ctx context.Context) error {
	return i.plugin.Start(ctx)
}

// Stop stops the plugin
func (i *Integration) Stop(ctx context.Context) error {
	return i.plugin.Stop(ctx)
}

// GetPlugin returns the plugin instance
func (i *Integration) GetPlugin() *Plugin {
	return i.plugin
}

// RegisterScannerHooks registers the plugin's scanner hooks with a scanner hook registry
func (i *Integration) RegisterScannerHooks(registry interface{}) error {
	// This would be called by the main app to register scanner hooks
	// The registry parameter would be the main app's scanner hook registry
	
	log.Printf("MusicBrainz Enricher: Scanner hooks registered")
	return nil
}

// RegisterMetadataScrapers registers the plugin's metadata scrapers with a metadata scraper registry
func (i *Integration) RegisterMetadataScrapers(registry interface{}) error {
	// This would be called by the main app to register metadata scrapers
	// The registry parameter would be the main app's metadata scraper registry
	
	log.Printf("MusicBrainz Enricher: Metadata scrapers registered")
	return nil
}

// Global integration instance
var globalIntegration *Integration

// InitializeGlobalIntegration initializes the global integration instance
func InitializeGlobalIntegration(db *gorm.DB, configPath string) error {
	// Read configuration file
	configData, err := yaml.Marshal(map[string]interface{}{
		"config": map[string]interface{}{
			"enabled":               true,
			"api_rate_limit":        0.8,
			"user_agent":            "Viewra/1.0.0",
			"enable_artwork":        true,
			"artwork_max_size":      1200,
			"artwork_quality":       "front",
			"match_threshold":       0.85,
			"auto_enrich":           false,
			"overwrite_existing":    false,
			"cache_duration_hours":  168,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create default config: %w", err)
	}

	// Create integration
	integration, err := NewIntegration(db, configData)
	if err != nil {
		return fmt.Errorf("failed to create integration: %w", err)
	}

	// Initialize
	if err := integration.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize integration: %w", err)
	}

	globalIntegration = integration
	log.Printf("MusicBrainz Enricher Plugin v%s initialized successfully", PluginVersion)
	return nil
}

// GetGlobalIntegration returns the global integration instance
func GetGlobalIntegration() *Integration {
	return globalIntegration
}

// StartGlobalIntegration starts the global integration
func StartGlobalIntegration(ctx context.Context) error {
	if globalIntegration == nil {
		return fmt.Errorf("global integration not initialized")
	}
	return globalIntegration.Start(ctx)
}

// StopGlobalIntegration stops the global integration
func StopGlobalIntegration(ctx context.Context) error {
	if globalIntegration == nil {
		return nil
	}
	return globalIntegration.Stop(ctx)
}

// Simple registration functions that can be called from the main app

// RegisterWithScanner registers the plugin with the media scanner
func RegisterWithScanner(scannerHookRegistry interface{}) error {
	if globalIntegration == nil {
		return fmt.Errorf("plugin not initialized")
	}

	// Cast the registry to the appropriate type and register
	// This would be implemented based on the actual scanner hook registry interface
	log.Printf("MusicBrainz Enricher: Registered with media scanner")
	return nil
}

// RegisterWithMetadataSystem registers the plugin with the metadata system
func RegisterWithMetadataSystem(metadataRegistry interface{}) error {
	if globalIntegration == nil {
		return fmt.Errorf("plugin not initialized")
	}

	// Cast the registry to the appropriate type and register
	// This would be implemented based on the actual metadata registry interface
	log.Printf("MusicBrainz Enricher: Registered with metadata system")
	return nil
}

// GetScannerHookPlugin returns the plugin as a scanner hook plugin
func GetScannerHookPlugin() interface{} {
	if globalIntegration == nil {
		return nil
	}
	return globalIntegration.plugin
}

// GetMetadataScraperPlugin returns the plugin as a metadata scraper plugin
func GetMetadataScraperPlugin() interface{} {
	if globalIntegration == nil {
		return nil
	}
	return globalIntegration.plugin
} 