package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mantonx/viewra/pkg/plugins"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TemplatePlugin demonstrates how to implement a Viewra plugin using the modern SDK
type TemplatePlugin struct {
	logger   plugins.Logger
	config   *Config
	db       *gorm.DB
	basePath string
}

// Config defines the plugin configuration structure
type Config struct {
	Enabled     bool   `json:"enabled" default:"true"`
	APIKey      string `json:"api_key"`
	UserAgent   string `json:"user_agent" default:"Viewra-Template/1.0"`
	MaxResults  int    `json:"max_results" default:"10"`
	CacheHours  int    `json:"cache_hours" default:"24"`
	Debug       bool   `json:"debug" default:"false"`
}

// TemplateData represents sample database model
type TemplateData struct {
	ID          uint32    `gorm:"primaryKey"`
	MediaFileID uint32    `gorm:"not null;index"`
	Title       string    `gorm:"not null"`
	Description string
	ExtraData   string `gorm:"type:text"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Plugin lifecycle methods
func (t *TemplatePlugin) Initialize(ctx *plugins.PluginContext) error {
	t.logger = ctx.Logger
	t.basePath = ctx.BasePath

	// Initialize with default configuration
	t.config = &Config{
		Enabled:    true,
		UserAgent:  "Viewra-Template/1.0",
		MaxResults: 10,
		CacheHours: 24,
		Debug:      false,
	}

	// Initialize database if needed
	if ctx.DatabaseURL != "" {
		if err := t.initDatabase(ctx.DatabaseURL); err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
	}

	t.logger.Info("Template plugin initialized", "base_path", t.basePath)
	return nil
}

func (t *TemplatePlugin) Start() error {
	t.logger.Info("Template plugin started", "version", "1.0.0")
	return nil
}

func (t *TemplatePlugin) Stop() error {
	if t.db != nil {
		if sqlDB, err := t.db.DB(); err == nil {
			sqlDB.Close()
		}
	}
	t.logger.Info("Template plugin stopped")
	return nil
}

func (t *TemplatePlugin) Info() (*plugins.PluginInfo, error) {
	return &plugins.PluginInfo{
		ID:          "template_plugin",
		Name:        "Template Plugin",
		Version:     "1.0.0",
		Type:        "metadata_scraper",
		Description: "Template plugin demonstrating all service interfaces",
		Author:      "Viewra Team",
	}, nil
}

func (t *TemplatePlugin) Health() error {
	if !t.config.Enabled {
		return fmt.Errorf("plugin is disabled")
	}
	
	// Test database connection if available
	if t.db != nil {
		sqlDB, err := t.db.DB()
		if err != nil {
			return fmt.Errorf("failed to get database instance: %w", err)
		}
		if err := sqlDB.Ping(); err != nil {
			return fmt.Errorf("database ping failed: %w", err)
		}
	}

	return nil
}

// Service interface implementations
func (t *TemplatePlugin) MetadataScraperService() plugins.MetadataScraperService {
	return t
}

func (t *TemplatePlugin) ScannerHookService() plugins.ScannerHookService {
	return t
}

func (t *TemplatePlugin) SearchService() plugins.SearchService {
	return t
}

func (t *TemplatePlugin) DatabaseService() plugins.DatabaseService {
	return t
}

func (t *TemplatePlugin) APIRegistrationService() plugins.APIRegistrationService {
	return t
}

func (t *TemplatePlugin) AssetService() plugins.AssetService {
	return nil // Not implemented in template
}

func (t *TemplatePlugin) AdminPageService() plugins.AdminPageService {
	return nil // Not implemented in template
}

// MetadataScraperService implementation
func (t *TemplatePlugin) CanHandle(filePath, mimeType string) bool {
	if !t.config.Enabled {
		return false
	}

	// Handle common audio file types
	supportedMimeTypes := []string{
		"audio/mpeg",
		"audio/mp4",
		"audio/m4a",
		"audio/flac",
		"audio/ogg",
		"audio/wav",
	}

	for _, supportedType := range supportedMimeTypes {
		if mimeType == supportedType {
			return true
		}
	}

	// Also check by file extension
	supportedExtensions := []string{".mp3", ".m4a", ".flac", ".ogg", ".wav"}
	filePath = strings.ToLower(filePath)
	for _, ext := range supportedExtensions {
		if strings.HasSuffix(filePath, ext) {
			return true
		}
	}

	return false
}

func (t *TemplatePlugin) ExtractMetadata(filePath string) (map[string]string, error) {
	t.logger.Debug("Extracting metadata", "file", filePath)

	// This is a template - in a real plugin, you would use a library
	// like taglib-go, id3-go, or similar to extract metadata
	metadata := map[string]string{
		"title":       "Template Title",
		"artist":      "Template Artist", 
		"album":       "Template Album",
		"year":        "2024",
		"genre":       "Template Genre",
		"track":       "1",
		"albumartist": "Template Album Artist",
		"comment":     "Extracted by Template Plugin",
	}

	t.logger.Info("Metadata extracted", "file", filePath, "fields", len(metadata))
	return metadata, nil
}

func (t *TemplatePlugin) GetSupportedTypes() []string {
	return []string{
		"audio/mpeg",
		"audio/mp4",
		"audio/m4a", 
		"audio/flac",
		"audio/ogg",
		"audio/wav",
	}
}

// ScannerHookService implementation
func (t *TemplatePlugin) OnMediaFileScanned(mediaFileID uint32, filePath string, metadata map[string]string) error {
	if !t.config.Enabled {
		return nil
	}

	t.logger.Debug("Media file scanned", "id", mediaFileID, "file", filePath)

	// Example: Store processed data in database
	if t.db != nil {
		enrichment := &TemplateData{
			MediaFileID: uint32(mediaFileID),
			Title:       metadata["title"],
			Description: metadata["description"],
			ExtraData:   "Additional processing data here",
		}

		if err := t.db.Create(enrichment).Error; err != nil {
			t.logger.Error("Failed to save template data", "error", err)
			return err
		}

		t.logger.Info("Template data saved", "mediaFileID", mediaFileID)
	}

	return nil
}

func (t *TemplatePlugin) OnScanStarted(scanJobID, libraryID uint32, libraryPath string) error {
	t.logger.Info("Scan started", "scanJobID", scanJobID, "libraryID", libraryID, "path", libraryPath)
	return nil
}

func (t *TemplatePlugin) OnScanCompleted(scanJobID, libraryID uint32, stats map[string]string) error {
	t.logger.Info("Scan completed", "scanJobID", scanJobID, "libraryID", libraryID, "stats", stats)
	return nil
}

// SearchService implementation
func (t *TemplatePlugin) Search(ctx context.Context, query map[string]string, limit, offset uint32) ([]*plugins.SearchResult, uint32, bool, error) {
	if !t.config.Enabled {
		return nil, 0, false, fmt.Errorf("plugin is disabled")
	}

	title := query["title"]
	artist := query["artist"]

	t.logger.Debug("Searching", "title", title, "artist", artist, "limit", limit, "offset", offset)

	// Example search results - in a real plugin, you would query an external API
	results := []*plugins.SearchResult{
		{
			ID:    "template_1",
			Title: fmt.Sprintf("Template Match for '%s'", title),
			Type:  "track",
			Metadata: map[string]string{
				"artist":      artist,
				"album":       "Template Album",
				"year":        "2024",
				"genre":       "Template",
				"source":      "template_plugin",
				"confidence":  "0.95",
			},
		},
		{
			ID:    "template_2",
			Title: fmt.Sprintf("Alternative Match for '%s'", title),
			Type:  "track",
			Metadata: map[string]string{
				"artist":      artist,
				"album":       "Another Template Album",
				"year":        "2023",
				"genre":       "Alternative Template",
				"source":      "template_plugin",
				"confidence":  "0.80",
			},
		},
	}

	// Apply limit
	if limit > 0 && len(results) > int(limit) {
		results = results[:limit]
	}

	total := uint32(len(results))
	hasMore := false

	return results, total, hasMore, nil
}

func (t *TemplatePlugin) GetSearchCapabilities(ctx context.Context) ([]string, bool, uint32, error) {
	capabilities := []string{"title", "artist", "album", "genre"}
	supportsFullText := true
	maxResults := uint32(t.config.MaxResults)

	return capabilities, supportsFullText, maxResults, nil
}

// DatabaseService implementation
func (t *TemplatePlugin) GetModels() []string {
	return []string{"TemplateData"}
}

func (t *TemplatePlugin) Migrate(connectionString string) error {
	return t.initDatabase(connectionString)
}

func (t *TemplatePlugin) Rollback(connectionString string) error {
	// Example rollback - drop template tables
	if t.db != nil {
		return t.db.Migrator().DropTable(&TemplateData{})
	}
	return fmt.Errorf("database not initialized")
}

// APIRegistrationService implementation
func (t *TemplatePlugin) GetRegisteredRoutes(ctx context.Context) ([]*plugins.APIRoute, error) {
	routes := []*plugins.APIRoute{
		{
			Method:      "GET",
			Path:        "/api/plugins/template/info",
			Description: "Get template plugin information",
		},
		{
			Method:      "GET", 
			Path:        "/api/plugins/template/search",
			Description: "Search using template plugin",
		},
		{
			Method:      "POST",
			Path:        "/api/plugins/template/process",
			Description: "Process media file with template plugin",
		},
	}

	return routes, nil
}

// Helper methods
func (t *TemplatePlugin) initDatabase(connectionString string) error {
	db, err := gorm.Open(sqlite.Open(connectionString), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	t.db = db

	// Auto-migrate tables
	if err := db.AutoMigrate(&TemplateData{}); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	t.logger.Info("Template plugin database initialized successfully")
	return nil
}

func main() {
	plugin := &TemplatePlugin{}
	plugins.StartPlugin(plugin)
} 