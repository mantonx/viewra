// Package musicbrainz_enricher provides metadata enrichment for music files using MusicBrainz.
package musicbrainz_enricher

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"musicbrainz_enricher/config"
)

const (
	// PluginID is the unique identifier for this plugin
	PluginID = "musicbrainz_enricher"
	
	// PluginName is the human-readable name
	PluginName = "MusicBrainz Metadata Enricher"
	
	// PluginVersion follows semantic versioning
	PluginVersion = "1.0.0"
	
	// PluginDescription describes what this plugin does
	PluginDescription = "Enriches music metadata and artwork using the MusicBrainz database"
	
	// PluginAuthor identifies the plugin author
	PluginAuthor = "Viewra Team"
)

// PluginInfo contains metadata about this plugin
type PluginInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Description string    `json:"description"`
	Author      string    `json:"author"`
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Plugin represents the MusicBrainz enricher plugin instance
type Plugin struct {
	db       *gorm.DB
	config   *config.Config
	enricher *Enricher
	info     *PluginInfo
}

// NewPlugin creates a new instance of the MusicBrainz enricher plugin
func NewPlugin(db *gorm.DB, cfg *config.Config) *Plugin {
	info := &PluginInfo{
		ID:          PluginID,
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Author:      PluginAuthor,
		Type:        "metadata_scraper",
		Status:      "enabled",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	enricher := NewEnricher(db, cfg)

	return &Plugin{
		db:       db,
		config:   cfg,
		enricher: enricher,
		info:     info,
	}
}

// Initialize sets up the plugin and creates necessary database tables
func (p *Plugin) Initialize(ctx interface{}) error {
	if err := p.enricher.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize enricher: %w", err)
	}
	
	fmt.Printf("MusicBrainz Enricher Plugin v%s initialized successfully\n", PluginVersion)
	return nil
}

// Start begins plugin operation
func (p *Plugin) Start(ctx context.Context) error {
	fmt.Printf("MusicBrainz Enricher Plugin started\n")
	return nil
}

// Stop gracefully shuts down the plugin
func (p *Plugin) Stop(ctx context.Context) error {
	fmt.Printf("MusicBrainz Enricher Plugin stopped\n")
	return nil
}

// Info returns plugin metadata
func (p *Plugin) Info() *PluginInfo {
	return p.info
}

// Health performs a health check on the plugin
func (p *Plugin) Health() error {
	return p.enricher.Health()
}

// RegisterRoutes registers HTTP endpoints for this plugin
func (p *Plugin) RegisterRoutes(router *gin.RouterGroup) {
	p.enricher.RegisterRoutes(router)
}

// Scanner Hook Interface Implementation

// OnMediaFileScanned is called when a media file is scanned and processed
func (p *Plugin) OnMediaFileScanned(mediaFileID uint, filePath string, metadata map[string]interface{}) error {
	return p.enricher.OnMediaFileScanned(mediaFileID, filePath, metadata)
}

// OnScanStarted is called when a scan job starts
func (p *Plugin) OnScanStarted(scanJobID uint, libraryID uint, libraryPath string) error {
	return p.enricher.OnScanStarted(scanJobID, libraryID, libraryPath)
}

// OnScanCompleted is called when a scan job completes
func (p *Plugin) OnScanCompleted(scanJobID uint, libraryID uint, stats map[string]interface{}) error {
	return p.enricher.OnScanCompleted(scanJobID, libraryID, stats)
}

// Metadata Scraper Interface Implementation

// CanHandle checks if this plugin can handle the given file
func (p *Plugin) CanHandle(filePath string, mimeType string) bool {
	return p.enricher.CanHandle(filePath, mimeType)
}

// ExtractMetadata extracts metadata from a file (delegates to enricher)
func (p *Plugin) ExtractMetadata(ctx context.Context, filePath string) (map[string]interface{}, error) {
	return p.enricher.ExtractMetadata(ctx, filePath)
}

// SupportedTypes returns the file types this plugin supports
func (p *Plugin) SupportedTypes() []string {
	return p.enricher.SupportedTypes()
}

// LoadFromConfig creates a plugin instance from configuration data
func LoadFromConfig(db *gorm.DB, configData []byte) (*Plugin, error) {
	cfg, err := config.LoadFromYAML(configData)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	
	plugin := NewPlugin(db, cfg)
	
	if err := plugin.Initialize(nil); err != nil {
		return nil, fmt.Errorf("failed to initialize plugin: %w", err)
	}
	
	return plugin, nil
}

 