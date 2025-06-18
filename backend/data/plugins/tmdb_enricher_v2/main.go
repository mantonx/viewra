package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mantonx/viewra/pkg/plugins"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/mantonx/viewra/data/plugins/tmdb_enricher_v2/internal/cache"
	"github.com/mantonx/viewra/data/plugins/tmdb_enricher_v2/internal/config"
	"github.com/mantonx/viewra/data/plugins/tmdb_enricher_v2/internal/models"
	"github.com/mantonx/viewra/data/plugins/tmdb_enricher_v2/internal/services"
)

// TMDbEnricherV2 represents the main plugin implementation
type TMDbEnricherV2 struct {
	// Core services
	db       *gorm.DB
	logger   plugins.Logger
	basePath string
	context  *plugins.PluginContext

	// Plugin services
	enricher     *services.EnrichmentService
	matcher      *services.MatchingService
	cacheManager *cache.CacheManager
	artwork      *services.ArtworkService

	// SDK services
	healthService      *plugins.BaseHealthService
	configService      *config.TMDbConfigurationService
	performanceMonitor *plugins.BasePerformanceMonitor

	// Host service connections
	unifiedClient *plugins.UnifiedServiceClient
}

// Plugin lifecycle methods
func (t *TMDbEnricherV2) Initialize(ctx *plugins.PluginContext) error {
	// Basic nil checks to prevent segmentation fault
	if ctx == nil {
		return fmt.Errorf("plugin context is nil")
	}

	if ctx.Logger == nil {
		return fmt.Errorf("logger in plugin context is nil")
	}

	t.logger = ctx.Logger
	t.basePath = ctx.BasePath
	t.context = ctx

	t.logger.Info("TMDb Enricher v2 initializing", "base_path", t.basePath, "plugin_base_path", ctx.PluginBasePath)

	// Initialize database connection
	if ctx.PluginBasePath == "" {
		t.logger.Error("PluginBasePath is empty")
		return fmt.Errorf("PluginBasePath is empty")
	}

	dbPath := filepath.Join(ctx.PluginBasePath, "tmdb_enricher.db")
	t.logger.Info("Opening database", "db_path", dbPath)
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.logger.Error("Failed to open database", "error", err, "db_path", dbPath)
		return fmt.Errorf("failed to open database: %w", err)
	}
	t.db = db
	t.logger.Info("Database opened successfully")

	// Auto-migrate the database
	t.logger.Info("Starting database migration")
	if err := t.db.AutoMigrate(&models.TMDbCache{}, &models.TMDbEnrichment{}, &models.TMDbArtwork{}); err != nil {
		t.logger.Error("Database migration failed", "error", err)
		return fmt.Errorf("failed to migrate database: %w", err)
	}
	t.logger.Info("Database migration completed")

	// Initialize unified client for host services
	if ctx.HostServiceAddr != "" {
		t.logger.Info("Connecting to host services", "addr", ctx.HostServiceAddr)
		client, err := plugins.NewUnifiedServiceClient(ctx.HostServiceAddr)
		if err != nil {
			t.logger.Warn("failed to connect to host services", "error", err)
		} else {
			t.unifiedClient = client
			t.logger.Info("Connected to host services successfully")
		}
	} else {
		t.logger.Info("No host service address provided")
	}

	// Initialize services with dependency injection
	t.logger.Info("Initializing plugin services")
	if err := t.initializeServices(); err != nil {
		t.logger.Error("Failed to initialize services", "error", err)
		return fmt.Errorf("failed to initialize services: %w", err)
	}
	t.logger.Info("Plugin services initialized successfully")

	t.logger.Info("TMDb Enricher v2 initialized successfully")
	return nil
}

func (t *TMDbEnricherV2) Start() error {
	t.logger.Info("TMDb Enricher v2 started")

	// Start background tasks
	if t.cacheManager != nil {
		go t.cacheManager.StartCleanupRoutine(context.Background())
	}

	return nil
}

func (t *TMDbEnricherV2) Stop() error {
	t.logger.Info("TMDb Enricher v2 stopping")

	// Cleanup resources
	if t.db != nil {
		if sqlDB, err := t.db.DB(); err == nil {
			sqlDB.Close()
		}
	}

	if t.unifiedClient != nil {
		t.unifiedClient.Close()
	}

	t.logger.Info("TMDb Enricher v2 stopped")
	return nil
}

func (t *TMDbEnricherV2) Info() (*plugins.PluginInfo, error) {
	version := GetVersion()
	return &plugins.PluginInfo{
		ID:          "tmdb_enricher_v2",
		Name:        "TMDb Metadata Enricher v2",
		Version:     version.Version,
		Type:        "metadata_scraper",
		Description: "Modern TMDb enrichment plugin with service-oriented architecture, health monitoring, and comprehensive configuration management",
		Author:      "Viewra Team",
	}, nil
}

// CheckHealth returns nil if the plugin is healthy
func (t *TMDbEnricherV2) Health() error {
	// Check database connection
	if sqlDB, err := t.db.DB(); err != nil {
		return fmt.Errorf("database error: %w", err)
	} else if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Check if unified client is available
	if t.unifiedClient == nil {
		return fmt.Errorf("unified client not available")
	}

	return nil
}

// Database service implementation
func (t *TMDbEnricherV2) GetModels() []string {
	return []string{
		"TMDbCache",
		"TMDbEnrichment",
		"TMDbArtwork",
	}
}

func (t *TMDbEnricherV2) Migrate(connectionString string) error {
	db, err := gorm.Open(sqlite.Open(connectionString), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate all models
	err = db.AutoMigrate(
		&models.TMDbCache{},
		&models.TMDbEnrichment{},
		&models.TMDbArtwork{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	t.logger.Info("database migration completed successfully")
	return nil
}

func (t *TMDbEnricherV2) Rollback(connectionString string) error {
	db, err := gorm.Open(sqlite.Open(connectionString), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	return db.Migrator().DropTable(&models.TMDbCache{}, &models.TMDbEnrichment{}, &models.TMDbArtwork{})
}

// Scanner hook service implementation
func (t *TMDbEnricherV2) OnMediaFileScanned(mediaFileID string, filePath string, metadata map[string]string) error {
	if !t.configService.GetTMDbConfig().Features.AutoEnrich {
		t.logger.Debug("auto-enrichment disabled, skipping", "file", filePath)
		return nil
	}

	// Use enrichment service to process the file
	if err := t.enricher.ProcessMediaFile(mediaFileID, filePath, metadata); err != nil {
		t.logger.Warn("enrichment failed", "error", err, "media_file_id", mediaFileID)
		return err
	}

	// Download artwork if enabled and enrichment was successful
	if t.configService.GetTMDbConfig().Features.EnableArtwork {
		// Get the enrichment data
		var enrichment models.TMDbEnrichment
		if err := t.db.Where("media_file_id = ?", mediaFileID).First(&enrichment).Error; err != nil {
			t.logger.Debug("no enrichment found for artwork download", "media_file_id", mediaFileID)
			return nil // Don't fail the overall process
		}

		// Download artwork in the background to avoid blocking scan
		go func() {
			if err := t.artwork.DownloadArtworkForEnrichment(mediaFileID, &enrichment); err != nil {
				t.logger.Warn("artwork download failed", "error", err, "media_file_id", mediaFileID)
			}
		}()
	}

	return nil
}

func (t *TMDbEnricherV2) OnScanStarted(scanJobID, libraryID uint32, libraryPath string) error {
	t.logger.Info("scan started", "scan_job_id", scanJobID, "library_id", libraryID, "path", libraryPath)

	// Prepare for scan - could pre-warm caches, etc.
	if t.cacheManager != nil {
		t.cacheManager.PrepareForScan()
	}

	return nil
}

func (t *TMDbEnricherV2) OnScanCompleted(scanJobID, libraryID uint32, stats map[string]string) error {
	t.logger.Info("scan completed", "scan_job_id", scanJobID, "library_id", libraryID, "stats", stats)

	// Post-scan cleanup or statistics
	return nil
}

// Metadata scraper service implementation
func (t *TMDbEnricherV2) CanHandle(filePath, mimeType string) bool {
	// Handle video files that could be movies or TV shows
	if strings.HasPrefix(mimeType, "video/") {
		return true
	}

	// Also handle files with video extensions even without MIME type
	ext := strings.ToLower(filepath.Ext(filePath))
	videoExtensions := []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".ts", ".mpg", ".mpeg"}

	for _, videoExt := range videoExtensions {
		if ext == videoExt {
			return true
		}
	}

	return false
}

func (t *TMDbEnricherV2) ExtractMetadata(filePath string) (map[string]string, error) {
	// This plugin doesn't extract metadata directly from files
	// It enriches existing metadata using TMDb API
	return map[string]string{
		"source": "tmdb_enricher_v2",
	}, nil
}

func (t *TMDbEnricherV2) GetSupportedTypes() []string {
	types := []string{}

	if t.configService.GetTMDbConfig().Features.EnableMovies {
		types = append(types, "movie")
	}

	if t.configService.GetTMDbConfig().Features.EnableTVShows {
		types = append(types, "tv", "episode")
	}

	return types
}

// Search service implementation (placeholder - to be implemented)
func (t *TMDbEnricherV2) Search(ctx context.Context, query map[string]string, limit, offset uint32) ([]*plugins.SearchResult, uint32, bool, error) {
	// Future implementation: Use enrichment service to search TMDb
	return nil, 0, false, fmt.Errorf("search service not yet implemented in v2 - use enrichment service directly")
}

func (t *TMDbEnricherV2) GetSearchCapabilities(ctx context.Context) ([]string, bool, uint32, error) {
	return []string{"title", "year", "tmdb_id"}, true, 100, nil
}

// Asset service implementation (placeholder - integrates with artwork service)
func (t *TMDbEnricherV2) SaveAsset(mediaFileID string, assetType, category, subtype string, data []byte, mimeType, sourceURL, pluginID string, metadata map[string]string) (uint32, string, string, error) {
	// Future implementation: Integrate with artwork service
	return 0, "", "", fmt.Errorf("asset service not yet implemented in v2 - use artwork service directly")
}

func (t *TMDbEnricherV2) AssetExists(mediaFileID string, assetType, category, subtype, hash string) (bool, uint32, string, error) {
	return false, 0, "", fmt.Errorf("asset check not implemented in v2 yet")
}

func (t *TMDbEnricherV2) RemoveAsset(assetID uint32) error {
	return fmt.Errorf("asset removal not implemented in v2 yet")
}

// API registration service implementation
func (t *TMDbEnricherV2) GetRegisteredRoutes(ctx context.Context) ([]*plugins.APIRoute, error) {
	routes := []*plugins.APIRoute{
		{
			Method:      "GET",
			Path:        "/tmdb/v2/health",
			Description: "Check TMDb enricher v2 health",
		},
		{
			Method:      "GET",
			Path:        "/tmdb/v2/cache/stats",
			Description: "Get cache statistics",
		},
		{
			Method:      "POST",
			Path:        "/tmdb/v2/cache/clear",
			Description: "Clear cache",
		},
	}

	return routes, nil
}

// Service interfaces implementation
func (t *TMDbEnricherV2) MetadataScraperService() plugins.MetadataScraperService {
	return t
}

func (t *TMDbEnricherV2) DatabaseService() plugins.DatabaseService {
	return t
}

func (t *TMDbEnricherV2) ScannerHookService() plugins.ScannerHookService {
	return t
}

func (t *TMDbEnricherV2) SearchService() plugins.SearchService {
	return t
}

func (t *TMDbEnricherV2) AssetService() plugins.AssetService {
	return t
}

func (t *TMDbEnricherV2) AdminPageService() plugins.AdminPageService {
	return &TMDbAdminPageService{
		plugin: t,
		logger: t.logger,
	}
}

func (t *TMDbEnricherV2) APIRegistrationService() plugins.APIRegistrationService {
	return t
}

// HealthMonitorService returns the health monitoring service
func (t *TMDbEnricherV2) HealthMonitorService() plugins.HealthMonitorService {
	return t.healthService
}

// ConfigurationService returns the configuration service
func (t *TMDbEnricherV2) ConfigurationService() plugins.ConfigurationService {
	return t.configService
}

// PerformanceMonitorService returns the performance monitoring service
func (t *TMDbEnricherV2) PerformanceMonitorService() plugins.PerformanceMonitorService {
	return &performanceServiceAdapter{monitor: t.performanceMonitor}
}

// TranscodingService returns nil since this is not a transcoding plugin
func (t *TMDbEnricherV2) TranscodingService() plugins.TranscodingService {
	return nil
}

// performanceServiceAdapter adapts BasePerformanceMonitor to PerformanceMonitorService interface
type performanceServiceAdapter struct {
	monitor *plugins.BasePerformanceMonitor
}

func (p *performanceServiceAdapter) GetPerformanceSnapshot(ctx context.Context) (*plugins.PerformanceSnapshot, error) {
	return p.monitor.GetSnapshot(), nil
}

func (p *performanceServiceAdapter) RecordOperation(operationName string, duration time.Duration, success bool, context string) {
	p.monitor.RecordOperation(operationName, duration, success, context)
}

func (p *performanceServiceAdapter) RecordError(errorType, message, context, operation string) {
	p.monitor.RecordError(errorType, message, context, operation)
}

func (p *performanceServiceAdapter) GetUptimeString() string {
	return p.monitor.GetUptimeString()
}

func (p *performanceServiceAdapter) Reset() {
	p.monitor.Reset()
}

// GetTMDbConfig returns the current TMDb configuration (plugin-specific method)
func (t *TMDbEnricherV2) GetTMDbConfig() *config.Config {
	return t.configService.GetTMDbConfig()
}

// UpdateTMDbConfig updates the TMDb configuration (plugin-specific method)
func (t *TMDbEnricherV2) UpdateTMDbConfig(newConfig *config.Config) error {
	return t.configService.UpdateTMDbConfig(newConfig)
}

// GetPerformanceMonitor returns the performance monitor for services to use
func (t *TMDbEnricherV2) GetPerformanceMonitor() *plugins.BasePerformanceMonitor {
	return t.performanceMonitor
}

// initializeServices initializes all plugin services
func (t *TMDbEnricherV2) initializeServices() error {
	t.logger.Info("Starting service initialization")

	// Initialize SDK services first
	t.logger.Info("Initializing health service")
	t.healthService = plugins.NewHealthServiceBuilder("TMDb Enricher").
		WithCustomCounter("tmdb_api_calls", 0).
		WithCustomCounter("cache_hits", 0).
		WithCustomCounter("cache_misses", 0).
		WithCustomGauge("cache_hit_rate", 0.0).
		Build()
	t.logger.Info("Health service initialized")

	// Initialize performance monitor
	t.logger.Info("Initializing performance monitor")
	t.performanceMonitor = plugins.NewPerformanceMonitorBuilder("TMDb Enricher").
		WithCustomCounter("api_calls", 0).
		WithCustomCounter("cache_operations", 0).
		WithCustomCounter("enrichments_completed", 0).
		WithCustomCounter("artwork_downloads", 0).
		WithCustomGauge("cache_hit_rate", 0.0).
		WithCustomGauge("api_rate_limit_remaining", 0.0).
		WithMaxErrorHistory(100).
		Build()
	t.logger.Info("Performance monitor initialized")

	// Load configuration from CUE file using the plugin SDK
	t.logger.Info("Loading configuration from plugin.cue")
	tmdbConfig := config.DefaultConfig()

	// Add debug logging for paths
	t.logger.Info("Plugin paths",
		"base_path", t.basePath,
		"plugin_base_path", t.context.PluginBasePath,
		"dir_of_base_path", filepath.Dir(t.basePath))

	// Add debug logging
	t.logger.Info("Default config before loading", "api_key", tmdbConfig.API.Key, "rate_limit", tmdbConfig.API.RateLimit)

	if err := plugins.LoadPluginConfig(t.context, tmdbConfig); err != nil {
		t.logger.Error("Failed to load configuration from CUE file", "error", err)
		return fmt.Errorf("failed to load TMDb configuration: %w", err)
	}

	// Debug the loaded config
	t.logger.Info("Configuration loaded successfully",
		"api_key_set", tmdbConfig.API.Key != "",
		"api_key_length", len(tmdbConfig.API.Key),
		"api_key_starts_with_eyJ", strings.HasPrefix(tmdbConfig.API.Key, "eyJ"),
		"api_key_dot_count", strings.Count(tmdbConfig.API.Key, "."),
		"auto_enrich", tmdbConfig.Features.AutoEnrich,
		"rate_limit", tmdbConfig.API.RateLimit)

	// Create TMDb configuration service for validation and management
	configPath := filepath.Join(t.basePath, "tmdb_config.json")
	t.logger.Info("Initializing configuration service", "config_path", configPath)
	t.configService = config.NewTMDbConfigurationService(configPath)

	// Set the loaded configuration instead of initializing from file
	if err := t.configService.UpdateTMDbConfig(tmdbConfig); err != nil {
		t.logger.Error("Configuration service validation failed", "error", err)
		return fmt.Errorf("failed to validate TMDb configuration: %w", err)
	}
	t.logger.Info("Configuration service initialized successfully")

	// Initialize cache manager with config and performance monitor
	t.logger.Info("Initializing cache manager")
	t.cacheManager = cache.NewCacheManager(t.db, tmdbConfig, t.logger)
	t.logger.Info("Cache manager initialized")

	// Initialize matching service with config
	t.logger.Info("Initializing matching service")
	t.matcher = services.NewMatchingService(tmdbConfig, t.logger)
	t.logger.Info("Matching service initialized")

	// Initialize enrichment service with config and performance monitor
	t.logger.Info("Initializing enrichment service")
	var err error
	t.enricher, err = services.NewEnrichmentService(t.db, tmdbConfig, t.unifiedClient, t.logger)
	if err != nil {
		t.logger.Error("Enrichment service initialization failed", "error", err)
		return fmt.Errorf("failed to initialize enrichment service: %w", err)
	}
	t.logger.Info("Enrichment service initialized")

	// Initialize artwork service with config and performance monitor
	t.logger.Info("Initializing artwork service")
	t.artwork = services.NewArtworkService(t.db, tmdbConfig, t.unifiedClient, t.logger)
	t.logger.Info("Artwork service initialized")

	// Add configuration change callback to update services when config changes
	t.logger.Info("Adding configuration callback")
	t.configService.AddConfigurationCallback(t.onConfigurationChanged)
	t.logger.Info("Configuration callback added")

	t.logger.Info("All services initialized successfully")
	return nil
}

// onConfigurationChanged handles configuration changes
func (t *TMDbEnricherV2) onConfigurationChanged(oldConfig, newConfig *plugins.PluginConfiguration) error {
	t.logger.Info("TMDb configuration changed, updating services")

	// Get the new TMDb configuration
	tmdbConfig := t.configService.GetTMDbConfig()

	// Update health service thresholds if they changed
	if newConfig.Thresholds != nil {
		if err := t.healthService.SetHealthThresholds(newConfig.Thresholds); err != nil {
			t.logger.Warn("failed to update health thresholds", "error", err)
		}
	}

	// Update cache manager with new settings
	if t.cacheManager != nil {
		t.logger.Debug("updating cache manager configuration",
			"old_duration", oldConfig.Settings["cache_duration_hours"],
			"new_duration", tmdbConfig.Cache.DurationHours)

		// Cache manager should reinitialize with new settings
		t.cacheManager.UpdateConfiguration(tmdbConfig)
	}

	// Update enrichment service configuration
	if t.enricher != nil {
		t.logger.Debug("updating enrichment service configuration")
		t.enricher.UpdateConfiguration(tmdbConfig)
	}

	// Update matching service configuration
	if t.matcher != nil {
		t.logger.Debug("updating matching service configuration")
		t.matcher.UpdateConfiguration(tmdbConfig)
	}

	// Update artwork service configuration
	if t.artwork != nil {
		t.logger.Debug("updating artwork service configuration")
		t.artwork.UpdateConfiguration(tmdbConfig)
	}

	// Update health service metrics based on configuration changes
	if t.healthService != nil {
		// Update API rate limit metric if it changed
		if oldAPISettings, exists := oldConfig.Settings["api"]; exists {
			if newAPISettings, exists := newConfig.Settings["api"]; exists {
				if oldAPIMap, ok := oldAPISettings.(map[string]interface{}); ok {
					if newAPIMap, ok := newAPISettings.(map[string]interface{}); ok {
						if oldRate, ok := oldAPIMap["rate_limit"].(float64); ok {
							if newRate, ok := newAPIMap["rate_limit"].(float64); ok && oldRate != newRate {
								t.healthService.SetGauge("config_api_rate_limit", newRate)
								t.logger.Info("updated API rate limit", "old", oldRate, "new", newRate)
							}
						}
					}
				}
			}
		}

		// Update feature flags
		for feature, enabled := range newConfig.Features {
			if oldEnabled, exists := oldConfig.Features[feature]; !exists || oldEnabled != enabled {
				t.healthService.SetGauge(fmt.Sprintf("feature_%s_enabled", feature), boolToFloat(enabled))
				t.logger.Info("updated feature flag", "feature", feature, "enabled", enabled)
			}
		}
	}

	t.logger.Info("TMDb configuration update completed",
		"api_rate_limit", tmdbConfig.API.RateLimit,
		"cache_duration_hours", tmdbConfig.Cache.DurationHours,
		"features_auto_enrich", tmdbConfig.Features.AutoEnrich)
	return nil
}

// Helper function to convert bool to float for gauge metrics
func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

// TMDbAdminPageService implements the AdminPageService for TMDb enricher
type TMDbAdminPageService struct {
	plugin *TMDbEnricherV2
	logger plugins.Logger
}

// GetAdminPages returns the admin pages provided by this plugin
func (a *TMDbAdminPageService) GetAdminPages() []*plugins.AdminPageConfig {
	return []*plugins.AdminPageConfig{
		{
			ID:       "tmdb_config",
			Title:    "TMDb Configuration",
			URL:      "/admin/plugins/tmdb_enricher_v2/config",
			Icon:     "database",
			Category: "enrichment",
			Type:     "configuration",
		},
		{
			ID:       "tmdb_api_status",
			Title:    "API Status & Limits",
			URL:      "/admin/plugins/tmdb_enricher_v2/api",
			Icon:     "server",
			Category: "enrichment",
			Type:     "status",
		},
		{
			ID:       "tmdb_cache",
			Title:    "Cache Management",
			URL:      "/admin/plugins/tmdb_enricher_v2/cache",
			Icon:     "hard-drive",
			Category: "enrichment",
			Type:     "dashboard",
		},
		{
			ID:       "tmdb_matching",
			Title:    "Matching Rules",
			URL:      "/admin/plugins/tmdb_enricher_v2/matching",
			Icon:     "search",
			Category: "enrichment",
			Type:     "configuration",
		},
		{
			ID:       "tmdb_artwork",
			Title:    "Artwork Settings",
			URL:      "/admin/plugins/tmdb_enricher_v2/artwork",
			Icon:     "image",
			Category: "enrichment",
			Type:     "configuration",
		},
	}
}

// RegisterRoutes registers the admin page routes for this plugin
func (a *TMDbAdminPageService) RegisterRoutes(basePath string) error {
	a.logger.Info("Registering TMDb enricher admin routes", "base_path", basePath)

	// Here we would register HTTP routes for the admin pages
	// The actual route registration would be handled by the host application
	// We just need to define what routes this plugin provides

	return nil
}

func main() {
	plugin := &TMDbEnricherV2{}
	plugins.StartPlugin(plugin)
}
