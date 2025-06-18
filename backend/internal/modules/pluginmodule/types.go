package pluginmodule

import (
	"io/fs"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"gorm.io/gorm"
)

// Core plugin interfaces and types

// CorePlugin represents a built-in core plugin that runs within the main process
type CorePlugin interface {
	FileHandlerPlugin

	// IsEnabled returns whether this core plugin is enabled
	IsEnabled() bool

	// Enable enables the plugin
	Enable() error

	// Disable disables the plugin
	Disable() error

	// Initialize performs any setup needed for the plugin
	Initialize() error

	// Shutdown performs any cleanup needed when the plugin is disabled
	Shutdown() error

	// GetDisplayName returns a human-readable display name for the plugin
	GetDisplayName() string
}

// FileHandlerPlugin interface for plugins that handle files
type FileHandlerPlugin interface {
	// Match determines if this plugin can handle the given file
	Match(path string, info fs.FileInfo) bool

	// HandleFile processes the file and extracts metadata
	HandleFile(path string, ctx *MetadataContext) error

	// GetName returns the name of the plugin
	GetName() string

	// GetSupportedExtensions returns the file extensions this plugin supports
	GetSupportedExtensions() []string

	// GetPluginType returns the type of media this plugin handles (music, video, image, etc.)
	GetPluginType() string

	// GetType returns the plugin type (metadata, scanner, etc.)
	GetType() string
}

// MetadataContext provides context for plugin operations
type MetadataContext struct {
	DB        *gorm.DB
	MediaFile *database.MediaFile
	LibraryID uint
	EventBus  interface{} // Will be events.EventBus but kept as interface for flexibility
	PluginID  string      // Plugin ID for tracking which plugin created the data
}

// External plugin management types

// ExternalPlugin represents an external plugin instance
type ExternalPlugin struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Version     string    `json:"version"`
	Description string    `json:"description"`
	Running     bool      `json:"running"`
	Path        string    `json:"path"`
	LastStarted time.Time `json:"last_started"`
	LastStopped time.Time `json:"last_stopped"`
}

// PluginInfo represents information about a plugin for API responses
type PluginInfo struct {
	Name          string   `json:"name"`
	ID            string   `json:"id,omitempty"` // Only for external plugins
	Type          string   `json:"type"`
	Version       string   `json:"version"`
	Description   string   `json:"description"`
	SupportedExts []string `json:"supported_extensions"`
	Enabled       bool     `json:"enabled"`
	IsCore        bool     `json:"is_core"`
	Category      string   `json:"category,omitempty"` // core_metadata, core_scanner, external_enrichment, etc.
}

// HostServices manages external plugin host services
type HostServices struct {
	// Placeholder for host service implementation
	// This will be filled out when we move the full gRPC infrastructure
}

// Core plugin factory and registry

// CorePluginFactory is a function that creates a core plugin
type CorePluginFactory func() CorePlugin

// Plugin module configuration types

// PluginModuleConfig defines configuration for the plugin module
type PluginModuleConfig struct {
	PluginDir       string                           `json:"plugin_dir"`
	ConfigPath      string                           `json:"config_path"`
	EnabledCore     []string                         `json:"enabled_core"`
	EnabledExternal []string                         `json:"enabled_external"`
	LibraryConfigs  map[string]LibraryPluginSettings `json:"library_configs"`
	HostPort        string                           `json:"host_port"` // Port for external plugin communication
	EnableHotReload bool                             `json:"enable_hot_reload"`
	HotReload       PluginHotReloadConfig            `json:"hot_reload"`
}

// PluginHotReloadConfig configures hot reload behavior for the plugin module
type PluginHotReloadConfig struct {
	Enabled         bool     `json:"enabled"`
	DebounceDelayMs int      `json:"debounce_delay_ms"`
	WatchPatterns   []string `json:"watch_patterns"`
	ExcludePatterns []string `json:"exclude_patterns"`
	PreserveState   bool     `json:"preserve_state"`
	MaxRetries      int      `json:"max_retries"`
	RetryDelayMs    int      `json:"retry_delay_ms"`
}

// LibraryPluginSettings defines plugin settings for a specific library type
type LibraryPluginSettings struct {
	CorePlugins       []string `json:"core_plugins"`
	ExternalPlugins   []string `json:"external_plugins"`
	AllowedExtensions []string `json:"allowed_extensions"`
	RestrictToType    bool     `json:"restrict_to_type"`
}

// Library manager types

// LibraryManagerSettings defines configuration for library-specific plugin handling
type LibraryManagerSettings struct {
	LibraryID       uint                             `json:"library_id"`
	LibraryType     string                           `json:"library_type"` // "music", "movies", "tv"
	PluginSettings  map[string]LibraryPluginSettings `json:"plugin_settings"`
	DefaultSettings LibraryPluginSettings            `json:"default_settings"`
}

// Media manager types

// MediaAsset represents an asset extracted by a plugin
type MediaAsset struct {
	MediaFileID string            `json:"media_file_id"`
	Type        string            `json:"type"` // "artwork", "subtitle", "thumbnail"
	MimeType    string            `json:"mime_type"`
	Data        []byte            `json:"data"`
	PluginID    string            `json:"plugin_id"`
	Metadata    map[string]string `json:"metadata"`
}
