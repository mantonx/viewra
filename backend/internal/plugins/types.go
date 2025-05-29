package plugins

import (
	"context"
	"io/fs"
	"time"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/plugins/proto"
)

const (
	PluginTypeMetadataScraper = "metadata_scraper"
	PluginTypeScannerHook     = "scanner_hook"
	PluginTypeAdminPage       = "admin_page"
	PluginTypeGeneric         = "generic"
)

// Plugin represents a loaded plugin instance
type Plugin struct {
	// Metadata
	ID          string
	Name        string
	Version     string
	Type        string
	Description string
	Author      string
	
	// Paths
	BinaryPath string
	ConfigPath string
	BasePath   string
	
	// Runtime state
	Running      bool
	LastStarted  time.Time
	LastStopped  time.Time
	RestartCount int
	Error        error
	
	// Plugin process management
	Client    *goplugin.Client
	GRPCClient *GRPCClient
	
	// Service clients (based on plugin capabilities)
	PluginService          proto.PluginServiceClient
	MetadataScraperService proto.MetadataScraperServiceClient
	ScannerHookService     proto.ScannerHookServiceClient
	DatabaseService        proto.DatabaseServiceClient
	AdminPageService       proto.AdminPageServiceClient
	APIRegistrationService proto.APIRegistrationServiceClient
	SearchService          proto.SearchServiceClient
	
	// Add other service clients as needed
}

// Config represents plugin configuration loaded from CueLang
type Config struct {
	// Schema metadata
	SchemaVersion string `cue:"schema_version"`
	
	// Plugin identification
	ID          string `cue:"id"`
	Name        string `cue:"name"`
	Version     string `cue:"version"`
	Description string `cue:"description"`
	Author      string `cue:"author"`
	Website     string `cue:"website"`
	Repository  string `cue:"repository"`
	License     string `cue:"license"`
	Type        string `cue:"type"`
	Tags        []string `cue:"tags"`
	Help        string   `cue:"help"`
	
	// Plugin behavior
	EnabledByDefault bool `cue:"enabled_by_default"`
	
	// Plugin capabilities
	Capabilities struct {
		MetadataExtraction bool `cue:"metadata_extraction"`
		APIEndpoints      bool `cue:"api_endpoints"`
		BackgroundTasks   bool `cue:"background_tasks"`
		DatabaseAccess    bool `cue:"database_access"`
		ExternalServices  bool `cue:"external_services"`
	} `cue:"capabilities"`
	
	// Entry points
	EntryPoints PluginEntryPoints `cue:"entry_points"`
	
	// Permissions
	Permissions []string `cue:"permissions"`
	
	// Plugin-specific configuration (flexible map)
	Settings map[string]interface{} `cue:"settings"`
}

// PluginEntryPoints defines the entry points for a plugin
type PluginEntryPoints struct {
	Main string `cue:"main"`
}

// Manager manages the plugin lifecycle
type Manager interface {
	// Lifecycle
	Initialize(ctx context.Context) error
	Shutdown(ctx context.Context) error
	
	// Plugin management
	LoadPlugin(ctx context.Context, pluginID string) error
	UnloadPlugin(ctx context.Context, pluginID string) error
	RestartPlugin(ctx context.Context, pluginID string) error
	
	// Discovery and querying
	DiscoverPlugins() error
	ListPlugins() map[string]*Plugin
	GetPlugin(pluginID string) (*Plugin, bool)
	
	// Service accessors
	GetMetadataScrapers() []proto.MetadataScraperServiceClient
	GetScannerHooks() []proto.ScannerHookServiceClient
	GetDatabases() []proto.DatabaseServiceClient
	GetAdminPages() []proto.AdminPageServiceClient
}

// Implementation interfaces that plugins must implement
type Implementation interface {
	// Core plugin methods
	Initialize(ctx *proto.PluginContext) error
	Start() error
	Stop() error
	Info() (*proto.PluginInfo, error)
	Health() error
	
	// Optional service implementations (return nil if not supported)
	MetadataScraperService() MetadataScraperService
	ScannerHookService() ScannerHookService
	DatabaseService() DatabaseService
	AdminPageService() AdminPageService
	APIRegistrationService() APIRegistrationService
	SearchService() SearchService
}

// Service interfaces
type MetadataScraperService interface {
	CanHandle(filePath, mimeType string) bool
	ExtractMetadata(filePath string) (map[string]string, error)
	GetSupportedTypes() []string
}

type ScannerHookService interface {
	OnMediaFileScanned(mediaFileID uint32, filePath string, metadata map[string]string) error
	OnScanStarted(scanJobID, libraryID uint32, libraryPath string) error
	OnScanCompleted(scanJobID, libraryID uint32, stats map[string]string) error
}

type DatabaseService interface {
	GetModels() []string
	Migrate(connectionString string) error
	Rollback(connectionString string) error
}

type AdminPageService interface {
	GetAdminPages() []*proto.AdminPageConfig
	RegisterRoutes(basePath string) error
}

// APIRegistrationService interface (to be implemented by plugins)
type APIRegistrationService interface {
	GetRegisteredRoutes(ctx context.Context) ([]*proto.APIRoute, error)
}

// SearchService interface (to be implemented by plugins)
type SearchService interface {
	Search(ctx context.Context, query map[string]string, limit, offset uint32) ([]*proto.SearchResult, uint32, bool, error)
	GetSearchCapabilities(ctx context.Context) ([]string, bool, uint32, error)
}

// Plugin registry for organizing plugins by type
type Registry struct {
	MetadataScrapers []string
	ScannerHooks     []string
	Databases        []string
	AdminPages       []string
}

// Logger interface for plugin logging
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	With(args ...interface{}) hclog.Logger
}

// MetadataContext provides context and utilities for file processing plugins
type MetadataContext struct {
	DB          interface{} // Will be *gorm.DB but kept as interface for flexibility
	MediaFile   *database.MediaFile
	LibraryID   uint
	EventBus    interface{} // Will be events.EventBus but kept as interface for flexibility
}

// FileHandlerPlugin defines the interface for file processing plugins
// These plugins handle metadata extraction from different media file types
type FileHandlerPlugin interface {
	// Match determines if this plugin can handle the given file
	Match(path string, info fs.FileInfo) bool
	
	// HandleFile processes the file and extracts metadata
	HandleFile(path string, ctx MetadataContext) error
	
	// GetName returns the name of the plugin
	GetName() string
	
	// GetSupportedExtensions returns the file extensions this plugin supports
	GetSupportedExtensions() []string
	
	// GetPluginType returns the type of media this plugin handles (music, video, image, etc.)
	GetPluginType() string
}

// ScannerPluginHook defines the interface for scanner plugin hooks
// These provide lifecycle callbacks during the scanning process
type ScannerPluginHook interface {
	OnScanStarted(jobID, libraryID uint, path string) error
	OnScanCompleted(jobID, libraryID uint, stats map[string]interface{}) error
	OnMediaFileScanned(mediaFile *database.MediaFile, metadata interface{}) error
}

// CorePlugin represents a built-in plugin that's always available
type CorePlugin interface {
	FileHandlerPlugin
	
	// IsEnabled returns whether this core plugin is enabled
	IsEnabled() bool
	
	// Initialize performs any setup needed for the plugin
	Initialize() error
	
	// Shutdown performs any cleanup needed when the plugin is disabled
	Shutdown() error
}

// PluginInfo contains metadata about a plugin
type PluginInfo struct {
	Name            string   `json:"name"`
	Type            string   `json:"type"`
	Version         string   `json:"version"`
	Description     string   `json:"description"`
	SupportedExts   []string `json:"supported_extensions"`
	Enabled         bool     `json:"enabled"`
	IsCore          bool     `json:"is_core"`
} 