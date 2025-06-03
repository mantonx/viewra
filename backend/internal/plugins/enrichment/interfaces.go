package enrichment

import "github.com/mantonx/viewra/internal/database"

// =============================================================================
// INTERNAL ENRICHMENT PLUGIN INTERFACES
// =============================================================================
// These interfaces define the contract for internal enrichment plugins that
// run within the main application process and communicate directly with the
// enrichment module.

// InternalEnrichmentPlugin defines the interface for core enrichment plugins
type InternalEnrichmentPlugin interface {
	// GetName returns the unique name of the plugin
	GetName() string

	// Initialize sets up the plugin (database migrations, etc.)
	Initialize() error

	// CanEnrich determines if this plugin can enrich the given media file
	CanEnrich(mediaFile *database.MediaFile) bool

	// EnrichMediaFile enriches a media file with metadata from this plugin's source
	EnrichMediaFile(mediaFile *database.MediaFile, existingMetadata map[string]string) error

	// OnMediaFileScanned is called when a media file is scanned (plugin hook)
	OnMediaFileScanned(mediaFile *database.MediaFile, metadata map[string]string) error
}

// PluginConfig defines common configuration options for internal plugins
type PluginConfig interface {
	// IsEnabled returns whether the plugin is enabled
	IsEnabled() bool

	// GetSource returns the source name for enrichment registration
	GetSource() string

	// GetPriority returns the plugin priority (lower = higher priority)
	GetPriority() int
}

// CacheablePlugin defines an interface for plugins that support caching
type CacheablePlugin interface {
	InternalEnrichmentPlugin

	// ClearCache clears the plugin's cache
	ClearCache() error

	// GetCacheStats returns cache statistics
	GetCacheStats() map[string]interface{}
}

// ConfigurablePlugin defines an interface for plugins with runtime configuration
type ConfigurablePlugin interface {
	InternalEnrichmentPlugin

	// UpdateConfig updates the plugin configuration
	UpdateConfig(config map[string]interface{}) error

	// GetConfig returns the current plugin configuration
	GetConfig() map[string]interface{}
}
