package musicbrainz_enricher

import (
	"context"
	"fmt"
	"log"

	"gorm.io/gorm"

	"musicbrainz_enricher/config"
)

// PluginRegistry provides access to the main Viewra plugin system
// This will be injected by the main application when the plugin is loaded
type PluginRegistry interface {
	// RegisterScannerHookPlugin registers a scanner hook plugin
	RegisterScannerHookPlugin(plugin ScannerHookPlugin) error
	
	// RegisterMetadataScraperPlugin registers a metadata scraper plugin
	RegisterMetadataScraperPlugin(plugin MetadataScraperPlugin) error
	
	// GetDatabase returns the main application database
	GetDatabase() *gorm.DB
	
	// GetLogger returns a logger for the plugin
	GetLogger(pluginID string) PluginLogger
}

// PluginLogger interface for logging (matches main app interface)
type PluginLogger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
}

// ScannerHookPlugin interface (matches main app interface)
type ScannerHookPlugin interface {
	// OnMediaFileScanned is called when a media file is scanned and processed
	OnMediaFileScanned(mediaFileID uint, filePath string, metadata map[string]interface{}) error
	
	// OnScanStarted is called when a scan job starts
	OnScanStarted(scanJobID uint, libraryID uint, libraryPath string) error
	
	// OnScanCompleted is called when a scan job completes
	OnScanCompleted(scanJobID uint, libraryID uint, stats map[string]interface{}) error
}

// MetadataScraperPlugin interface (matches main app interface)
type MetadataScraperPlugin interface {
	// Check if this scraper can handle the given file
	CanHandle(filePath string, mimeType string) bool
	
	// Extract metadata from a file
	ExtractMetadata(ctx context.Context, filePath string) (map[string]interface{}, error)
	
	// Get supported file types/extensions
	SupportedTypes() []string
}

// Global registry instance - will be set by the main application
var globalRegistry PluginRegistry

// SetGlobalRegistry sets the global plugin registry (called by main app)
func SetGlobalRegistry(registry PluginRegistry) {
	globalRegistry = registry
}

// GetGlobalRegistry returns the global plugin registry
func GetGlobalRegistry() PluginRegistry {
	return globalRegistry
}

// RegisterWithMainApp registers the plugin with the main Viewra application
// This function is called by the main app's plugin manager
func RegisterWithMainApp(db *gorm.DB, configData []byte, logger PluginLogger) error {
	// Load configuration
	cfg, err := config.LoadFromYAML(configData)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create plugin instance
	plugin := NewPlugin(db, cfg)
	
	// Initialize the plugin
	if err := plugin.Initialize(nil); err != nil {
		return fmt.Errorf("failed to initialize plugin: %w", err)
	}

	// Register as scanner hook plugin if registry is available
	if globalRegistry != nil {
		if err := globalRegistry.RegisterScannerHookPlugin(plugin); err != nil {
			return fmt.Errorf("failed to register scanner hook plugin: %w", err)
		}
		
		// Also register as metadata scraper if the plugin supports it
		if err := globalRegistry.RegisterMetadataScraperPlugin(plugin); err != nil {
			// This is optional, so just log the error
			if logger != nil {
				logger.Warn("Failed to register as metadata scraper plugin", "error", err)
			}
		}
	}

	if logger != nil {
		logger.Info("MusicBrainz Enricher Plugin registered successfully", "version", PluginVersion)
	} else {
		log.Printf("MusicBrainz Enricher Plugin v%s registered successfully\n", PluginVersion)
	}

	return nil
}

// InitPlugin is the main entry point called by the plugin manager
// This follows the standard plugin initialization pattern
func InitPlugin(db *gorm.DB, configData []byte, logger PluginLogger) error {
	return RegisterWithMainApp(db, configData, logger)
} 