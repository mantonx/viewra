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

// PluginManifest defines the structure of plugin.json
type PluginManifest struct {
	SchemaVersion string                 `json:"schema_version"`
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Version       string                 `json:"version"`
	Description   string                 `json:"description"`
	Author        string                 `json:"author"`
	Website       string                 `json:"website,omitempty"`
	Repository    string                 `json:"repository,omitempty"`
	License       string                 `json:"license,omitempty"`
	Type          PluginType             `json:"type"`
	Tags          []string               `json:"tags,omitempty"`
	
	// Plugin capabilities
	Capabilities  PluginCapabilities     `json:"capabilities"`
	
	// Dependencies
	Dependencies  PluginDependencies     `json:"dependencies,omitempty"`
	
	// Configuration schema
	ConfigSchema  map[string]interface{} `json:"config_schema,omitempty"`
	
	// Entry points
	EntryPoints   PluginEntryPoints      `json:"entry_points"`
	
	// UI components (for frontend plugins)
	UI            *PluginUI              `json:"ui,omitempty"`
	
	// Permissions required
	Permissions   []string               `json:"permissions,omitempty"`
}

// PluginCapabilities defines what the plugin can do
type PluginCapabilities struct {
	MetadataExtraction bool `json:"metadata_extraction,omitempty"`
	AdminPages         bool `json:"admin_pages,omitempty"`
	UIComponents       bool `json:"ui_components,omitempty"`
	APIEndpoints       bool `json:"api_endpoints,omitempty"`
	BackgroundTasks    bool `json:"background_tasks,omitempty"`
	FileTranscoding    bool `json:"file_transcoding,omitempty"`
	Notifications      bool `json:"notifications,omitempty"`
	DatabaseAccess     bool `json:"database_access,omitempty"`
	ExternalServices   bool `json:"external_services,omitempty"`
}

// PluginDependencies defines plugin dependencies
type PluginDependencies struct {
	ViewraVersion string            `json:"viewra_version,omitempty"`
	Plugins       map[string]string `json:"plugins,omitempty"`
	System        []string          `json:"system,omitempty"`
}

// PluginEntryPoints defines how to run the plugin
type PluginEntryPoints struct {
	Main      string            `json:"main,omitempty"`       // Main executable/script
	Setup     string            `json:"setup,omitempty"`      // Setup script
	Teardown  string            `json:"teardown,omitempty"`   // Cleanup script
	WebServer string            `json:"web_server,omitempty"` // Plugin web server
	Workers   map[string]string `json:"workers,omitempty"`    // Background workers
}

// PluginUI defines frontend UI components
type PluginUI struct {
	AdminPages []AdminPageConfig `json:"admin_pages,omitempty"`
	Components []UIComponentConfig `json:"components,omitempty"`
	Assets     PluginAssets      `json:"assets,omitempty"`
}

// AdminPageConfig defines an admin page provided by the plugin
type AdminPageConfig struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Path     string `json:"path"`
	Icon     string `json:"icon,omitempty"`
	Category string `json:"category,omitempty"`
	URL      string `json:"url"`      // URL to load (iframe or module)
	Type     string `json:"type"`     // "iframe", "module", "component"
}

// UIComponentConfig defines a UI component provided by the plugin
type UIComponentConfig struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Type     string            `json:"type"` // "widget", "modal", "page"
	Props    map[string]string `json:"props,omitempty"`
	URL      string            `json:"url"`
}

// PluginAssets defines static assets
type PluginAssets struct {
	CSS        []string `json:"css,omitempty"`
	JavaScript []string `json:"javascript,omitempty"`
	Images     []string `json:"images,omitempty"`
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
