package musicbrainz_enricher

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"

	"musicbrainz_enricher/config"
)

// PluginBridge adapts our plugin to work with the main Viewra plugin system
type PluginBridge struct {
	plugin *Plugin
	info   *MainAppPluginInfo
}

// MainAppPluginInfo matches the main app's PluginInfo structure
type MainAppPluginInfo struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	Author      string                 `json:"author"`
	Website     string                 `json:"website,omitempty"`
	Repository  string                 `json:"repository,omitempty"`
	License     string                 `json:"license,omitempty"`
	Type        string                 `json:"type"` // PluginType from main app
	Tags        []string               `json:"tags,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Status      string                 `json:"status"` // PluginStatus from main app
	Error       string                 `json:"error,omitempty"`
	InstallPath string                 `json:"install_path,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// PluginContext matches the main app's PluginContext structure
type MainAppPluginContext struct {
	PluginID   string
	Logger     PluginLogger
	Database   interface{} // Will be *gorm.DB
	Config     interface{} // Plugin config interface
	HTTPClient interface{} // HTTP client interface
	FileSystem interface{} // File system interface
	Events     interface{} // Event bus interface
	Hooks      interface{} // Hook registry interface
}

// NewPluginBridge creates a bridge between our plugin and the main app
func NewPluginBridge(db *gorm.DB, configData []byte) (*PluginBridge, error) {
	// Parse configuration
	cfg, err := config.LoadFromYAML(configData)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create our plugin instance
	plugin := NewPlugin(db, cfg)

	// Create main app compatible info
	var configMap map[string]interface{}
	if err := yaml.Unmarshal(configData, &configMap); err == nil {
		if configSection, ok := configMap["config"].(map[string]interface{}); ok {
			configMap = configSection
		}
	}

	info := &MainAppPluginInfo{
		ID:          PluginID,
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Author:      PluginAuthor,
		Website:     "https://github.com/mantonx/viewra",
		Repository:  "https://github.com/mantonx/viewra",
		License:     "MIT",
		Type:        "metadata_scraper",
		Tags:        []string{"music", "metadata", "enrichment", "musicbrainz", "artwork"},
		Config:      configMap,
		Status:      "enabled",
		InstallPath: "",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	return &PluginBridge{
		plugin: plugin,
		info:   info,
	}, nil
}

// Main App Plugin Interface Implementation

// Initialize implements the main app's Plugin.Initialize
func (b *PluginBridge) Initialize(ctx *MainAppPluginContext) error {
	return b.plugin.Initialize(ctx)
}

// Start implements the main app's Plugin.Start
func (b *PluginBridge) Start(ctx context.Context) error {
	return b.plugin.Start(ctx)
}

// Stop implements the main app's Plugin.Stop
func (b *PluginBridge) Stop(ctx context.Context) error {
	return b.plugin.Stop(ctx)
}

// Info implements the main app's Plugin.Info
func (b *PluginBridge) Info() *MainAppPluginInfo {
	return b.info
}

// Health implements the main app's Plugin.Health
func (b *PluginBridge) Health() error {
	return b.plugin.Health()
}

// MetadataScraperPlugin Interface Implementation

// CanHandle implements MetadataScraperPlugin.CanHandle
func (b *PluginBridge) CanHandle(filePath string, mimeType string) bool {
	return b.plugin.CanHandle(filePath, mimeType)
}

// ExtractMetadata implements MetadataScraperPlugin.ExtractMetadata
func (b *PluginBridge) ExtractMetadata(ctx context.Context, filePath string) (map[string]interface{}, error) {
	return b.plugin.ExtractMetadata(ctx, filePath)
}

// SupportedTypes implements MetadataScraperPlugin.SupportedTypes
func (b *PluginBridge) SupportedTypes() []string {
	return b.plugin.SupportedTypes()
}

// ScannerHookPlugin Interface Implementation

// OnMediaFileScanned implements ScannerHookPlugin.OnMediaFileScanned
func (b *PluginBridge) OnMediaFileScanned(mediaFileID uint, filePath string, metadata map[string]interface{}) error {
	return b.plugin.OnMediaFileScanned(mediaFileID, filePath, metadata)
}

// OnScanStarted implements ScannerHookPlugin.OnScanStarted
func (b *PluginBridge) OnScanStarted(scanJobID uint, libraryID uint, libraryPath string) error {
	return b.plugin.OnScanStarted(scanJobID, libraryID, libraryPath)
}

// OnScanCompleted implements ScannerHookPlugin.OnScanCompleted
func (b *PluginBridge) OnScanCompleted(scanJobID uint, libraryID uint, stats map[string]interface{}) error {
	return b.plugin.OnScanCompleted(scanJobID, libraryID, stats)
}

// AdminPagePlugin Interface Implementation (optional)

// RegisterRoutes implements AdminPagePlugin.RegisterRoutes
func (b *PluginBridge) RegisterRoutes(router *gin.RouterGroup) error {
	b.plugin.RegisterRoutes(router)
	return nil
}

// GetAdminPages implements AdminPagePlugin.GetAdminPages
func (b *PluginBridge) GetAdminPages() []interface{} {
	// Return empty for now - admin pages will be added later
	return []interface{}{}
}

// Factory function for the main app plugin manager
func CreatePluginInstance(db *gorm.DB, configData []byte) (interface{}, error) {
	return NewPluginBridge(db, configData)
} 