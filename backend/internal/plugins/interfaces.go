// Package plugins provides the core plugin system for Viewra.
// This enables extensible functionality through a well-defined plugin architecture.
package plugins

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

// PluginType represents the type of plugin
type PluginType string

const (
	PluginTypeMetadataScraper PluginType = "metadata_scraper"
	PluginTypeAdminPage       PluginType = "admin_page"
	PluginTypeUIComponent     PluginType = "ui_component"
	PluginTypeScanner         PluginType = "scanner"
	PluginTypeAnalyzer        PluginType = "analyzer"
	PluginTypeNotification    PluginType = "notification"
	PluginTypeTranscoder      PluginType = "transcoder"
	PluginTypeExternal        PluginType = "external"
)

// PluginStatus represents the current status of a plugin
type PluginStatus string

const (
	PluginStatusEnabled    PluginStatus = "enabled"
	PluginStatusDisabled   PluginStatus = "disabled"
	PluginStatusInstalling PluginStatus = "installing"
	PluginStatusError      PluginStatus = "error"
	PluginStatusUpdating   PluginStatus = "updating"
)

// PluginInfo contains metadata about a plugin
type PluginInfo struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Author      string            `json:"author"`
	Website     string            `json:"website,omitempty"`
	Repository  string            `json:"repository,omitempty"`
	License     string            `json:"license,omitempty"`
	Type        PluginType        `json:"type"`
	Tags        []string          `json:"tags,omitempty"`
	Manifest    *PluginManifest   `json:"manifest,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Status      PluginStatus      `json:"status"`
	Error       string            `json:"error,omitempty"`
	InstallPath string            `json:"install_path,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// PluginManifest defines the structure of plugin.yml/plugin.yaml
type PluginManifest struct {
	SchemaVersion string                 `yaml:"schema_version"`
	ID            string                 `yaml:"id"`
	Name          string                 `yaml:"name"`
	Version       string                 `yaml:"version"`
	Description   string                 `yaml:"description"`
	Author        string                 `yaml:"author"`
	Website       string                 `yaml:"website,omitempty"`
	Repository    string                 `yaml:"repository,omitempty"`
	License       string                 `yaml:"license,omitempty"`
	Type          PluginType             `yaml:"type"`
	Tags          []string               `yaml:"tags,omitempty"`
	
	// Plugin capabilities
	Capabilities  PluginCapabilities     `yaml:"capabilities"`
	
	// Dependencies
	Dependencies  PluginDependencies     `yaml:"dependencies,omitempty"`
	
	// Configuration schema
	ConfigSchema  map[string]interface{} `yaml:"config_schema,omitempty"`
	
	// Entry points
	EntryPoints   PluginEntryPoints      `yaml:"entry_points"`
	
	// UI components (for frontend plugins)
	UI            *PluginUI              `yaml:"ui,omitempty"`
	
	// Permissions required
	Permissions   []string               `yaml:"permissions,omitempty"`
}

// PluginCapabilities defines what the plugin can do
type PluginCapabilities struct {
	MetadataExtraction bool `yaml:"metadata_extraction,omitempty"`
	AdminPages         bool `yaml:"admin_pages,omitempty"`
	UIComponents       bool `yaml:"ui_components,omitempty"`
	APIEndpoints       bool `yaml:"api_endpoints,omitempty"`
	BackgroundTasks    bool `yaml:"background_tasks,omitempty"`
	FileTranscoding    bool `yaml:"file_transcoding,omitempty"`
	Notifications      bool `yaml:"notifications,omitempty"`
	DatabaseAccess     bool `yaml:"database_access,omitempty"`
	ExternalServices   bool `yaml:"external_services,omitempty"`
}

// PluginDependencies defines plugin dependencies
type PluginDependencies struct {
	ViewraVersion string            `yaml:"viewra_version,omitempty"`
	Plugins       map[string]string `yaml:"plugins,omitempty"`
	System        []string          `yaml:"system,omitempty"`
}

// PluginEntryPoints defines how to run the plugin
type PluginEntryPoints struct {
	Main      string            `yaml:"main,omitempty"`       // Main executable/script
	Setup     string            `yaml:"setup,omitempty"`      // Setup script
	Teardown  string            `yaml:"teardown,omitempty"`   // Cleanup script
	WebServer string            `yaml:"web_server,omitempty"` // Plugin web server
	Workers   map[string]string `yaml:"workers,omitempty"`    // Background workers
}

// PluginUI defines frontend UI components
type PluginUI struct {
	AdminPages []AdminPageConfig `yaml:"admin_pages,omitempty"`
	Components []UIComponentConfig `yaml:"components,omitempty"`
	Assets     PluginAssets      `yaml:"assets,omitempty"`
}

// AdminPageConfig defines an admin page provided by the plugin
type AdminPageConfig struct {
	ID       string `yaml:"id"`
	Title    string `yaml:"title"`
	Path     string `yaml:"path"`
	Icon     string `yaml:"icon,omitempty"`
	Category string `yaml:"category,omitempty"`
	URL      string `yaml:"url"`      // URL to load (iframe or module)
	Type     string `yaml:"type"`     // "iframe", "module", "component"
}

// UIComponentConfig defines a UI component provided by the plugin
type UIComponentConfig struct {
	ID       string            `yaml:"id"`
	Name     string            `yaml:"name"`
	Type     string            `yaml:"type"` // "widget", "modal", "page"
	Props    map[string]string `yaml:"props,omitempty"`
	URL      string            `yaml:"url"`
}

// PluginAssets defines static assets
type PluginAssets struct {
	CSS        []string `yaml:"css,omitempty"`
	JavaScript []string `yaml:"javascript,omitempty"`
	Images     []string `yaml:"images,omitempty"`
}

// PluginContext provides context and utilities for plugins
type PluginContext struct {
	PluginID    string
	Logger      PluginLogger
	Database    Database
	Config      PluginConfig
	HTTPClient  HTTPClient
	FileSystem  FileSystemAccess
	Events      EventBus
	Hooks       HookRegistry
}

// Plugin defines the core interface that all plugins must implement
type Plugin interface {
	// Initialize the plugin with context
	Initialize(ctx *PluginContext) error
	
	// Start the plugin (called when enabled)
	Start(ctx context.Context) error
	
	// Stop the plugin (called when disabled or shutting down)
	Stop(ctx context.Context) error
	
	// Get plugin information
	Info() *PluginInfo
	
	// Health check
	Health() error
}

// MetadataScraperPlugin interface for plugins that extract metadata
type MetadataScraperPlugin interface {
	Plugin
	
	// Check if this scraper can handle the given file
	CanHandle(filePath string, mimeType string) bool
	
	// Extract metadata from a file
	ExtractMetadata(ctx context.Context, filePath string) (map[string]interface{}, error)
	
	// Get supported file types/extensions
	SupportedTypes() []string
}

// AdminPagePlugin interface for plugins that provide admin pages
type AdminPagePlugin interface {
	Plugin
	
	// Register admin routes
	RegisterRoutes(router *gin.RouterGroup) error
	
	// Get admin page configurations
	GetAdminPages() []AdminPageConfig
}

// UIComponentPlugin interface for plugins that provide UI components
type UIComponentPlugin interface {
	Plugin
	
	// Get UI component configurations
	GetUIComponents() []UIComponentConfig
	
	// Serve component assets
	ServeAssets(router *gin.RouterGroup) error
}

// ScannerPlugin interface for custom file scanners
type ScannerPlugin interface {
	Plugin
	
	// Scan a directory for files
	ScanDirectory(ctx context.Context, path string) ([]string, error)
	
	// Check if file should be processed
	ShouldProcess(filePath string, info PluginFileInfo) bool
}

// ScannerHookPlugin interface for plugins that want to hook into the scanner
type ScannerHookPlugin interface {
	Plugin
	
	// OnMediaFileScanned is called when a media file is scanned and processed
	OnMediaFileScanned(mediaFileID uint, filePath string, metadata map[string]interface{}) error
	
	// OnScanStarted is called when a scan job starts
	OnScanStarted(scanJobID uint, libraryID uint, libraryPath string) error
	
	// OnScanCompleted is called when a scan job completes
	OnScanCompleted(scanJobID uint, libraryID uint, stats map[string]interface{}) error
}

// AnalyzerPlugin interface for file analysis
type AnalyzerPlugin interface {
	Plugin
	
	// Analyze a file and return insights
	AnalyzeFile(ctx context.Context, filePath string) (AnalysisResult, error)
	
	// Get analysis capabilities
	GetCapabilities() []string
}

// NotificationPlugin interface for sending notifications
type NotificationPlugin interface {
	Plugin
	
	// Send a notification
	SendNotification(ctx context.Context, notification Notification) error
	
	// Get notification channels supported
	GetChannels() []string
}

// =============================================================================
// DATABASE INTERFACE
// =============================================================================

// Database interface for plugin access to database operations
type Database interface {
	GetDB() interface{} // Returns *gorm.DB but kept as interface{} for flexibility
}

// =============================================================================
// CORE PLUGIN INTERFACES
// Supporting types for plugin interfaces
type PluginLogger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
}

type PluginConfig interface {
	Get(key string) interface{}
	Set(key string, value interface{}) error
	GetString(key string) string
	GetInt(key string) int
	GetBool(key string) bool
}

type HTTPClient interface {
	Get(url string) ([]byte, error)
	Post(url string, data []byte) ([]byte, error)
	Put(url string, data []byte) ([]byte, error)
	Delete(url string) ([]byte, error)
}

type FileSystemAccess interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte) error
	Exists(path string) bool
	ListFiles(dir string) ([]string, error)
	CreateDir(path string) error
}

type EventBus interface {
	Publish(event string, data interface{}) error
	Subscribe(event string, handler func(data interface{})) error
	Unsubscribe(event string, handler func(data interface{})) error
}

type HookRegistry interface {
	Register(hook string, handler func(data interface{}) interface{}) error
	Execute(hook string, data interface{}) interface{}
	Remove(hook string, handler func(data interface{}) interface{}) error
}

type PluginFileInfo struct {
	Path     string
	Size     int64
	ModTime  time.Time
	MimeType string
	Hash     string
}

type AnalysisResult struct {
	Type     string                 `json:"type"`
	Score    float64                `json:"score"`
	Metadata map[string]interface{} `json:"metadata"`
	Tags     []string               `json:"tags"`
}

type Notification struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Title    string                 `json:"title"`
	Message  string                 `json:"message"`
	Data     map[string]interface{} `json:"data,omitempty"`
	Priority int                    `json:"priority"`
}
