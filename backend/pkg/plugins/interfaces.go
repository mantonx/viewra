// Package plugins provides interfaces and types for developing Viewra plugins.
// This package is designed to be imported by external plugins without creating
// dependencies on the main Viewra application.
package plugins

import (
	"context"
	"time"

	"github.com/hashicorp/go-hclog"
)

// Plugin constants
const (
	PluginTypeMetadataScraper = "metadata_scraper"
	PluginTypeScannerHook     = "scanner_hook"
	PluginTypeAdminPage       = "admin_page"
	PluginTypeGeneric         = "generic"
)

// Implementation interface that all plugins must implement
type Implementation interface {
	// Core plugin methods
	Initialize(ctx *PluginContext) error
	Start() error
	Stop() error
	Info() (*PluginInfo, error)
	Health() error

	// Optional service implementations (return nil if not supported)
	MetadataScraperService() MetadataScraperService
	ScannerHookService() ScannerHookService
	AssetService() AssetService
	DatabaseService() DatabaseService
	AdminPageService() AdminPageService
	APIRegistrationService() APIRegistrationService
	SearchService() SearchService

	// New enhanced service interfaces (return nil if not supported)
	HealthMonitorService() HealthMonitorService
	ConfigurationService() ConfigurationService
	PerformanceMonitorService() PerformanceMonitorService
}

// Service interfaces
type MetadataScraperService interface {
	CanHandle(filePath, mimeType string) bool
	ExtractMetadata(filePath string) (map[string]string, error)
	GetSupportedTypes() []string
}

type ScannerHookService interface {
	OnMediaFileScanned(mediaFileID string, filePath string, metadata map[string]string) error
	OnScanStarted(scanJobID, libraryID uint32, libraryPath string) error
	OnScanCompleted(scanJobID, libraryID uint32, stats map[string]string) error
}

type AssetService interface {
	SaveAsset(mediaFileID string, assetType, category, subtype string, data []byte, mimeType, sourceURL, pluginID string, metadata map[string]string) (uint32, string, string, error)
	AssetExists(mediaFileID string, assetType, category, subtype, hash string) (bool, uint32, string, error)
	RemoveAsset(assetID uint32) error
}

type DatabaseService interface {
	GetModels() []string
	Migrate(connectionString string) error
	Rollback(connectionString string) error
}

type AdminPageService interface {
	GetAdminPages() []*AdminPageConfig
	RegisterRoutes(basePath string) error
}

type APIRegistrationService interface {
	GetRegisteredRoutes(ctx context.Context) ([]*APIRoute, error)
}

type SearchService interface {
	Search(ctx context.Context, query map[string]string, limit, offset uint32) ([]*SearchResult, uint32, bool, error)
	GetSearchCapabilities(ctx context.Context) ([]string, bool, uint32, error)
}

// Client interfaces for communicating with host services
type AssetServiceClient interface {
	SaveAsset(ctx context.Context, req *SaveAssetRequest) (*SaveAssetResponse, error)
	AssetExists(ctx context.Context, req *AssetExistsRequest) (*AssetExistsResponse, error)
	RemoveAsset(ctx context.Context, req *RemoveAssetRequest) (*RemoveAssetResponse, error)
}

type EnrichmentServiceClient interface {
	RegisterEnrichment(ctx context.Context, req *RegisterEnrichmentRequest) (*RegisterEnrichmentResponse, error)
}

// Data structures
type PluginContext struct {
	PluginID        string `json:"plugin_id"` // Plugin identifier passed from manager
	DatabaseURL     string `json:"database_url"`
	HostServiceAddr string `json:"host_service_addr"`
	PluginBasePath  string `json:"plugin_base_path"`
	LogLevel        string `json:"log_level"`
	BasePath        string `json:"base_path"`
	Logger          Logger `json:"-"`
}

type PluginInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Author      string `json:"author"`
}

type AdminPageConfig struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Icon     string `json:"icon"`
	Category string `json:"category"`
}

type APIRoute struct {
	Path        string `json:"path"`
	Method      string `json:"method"`
	Description string `json:"description"`
	Public      bool   `json:"public"`
}

type SearchResult struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Title    string            `json:"title"`
	Subtitle string            `json:"subtitle"`
	URL      string            `json:"url"`
	Metadata map[string]string `json:"metadata"`
}

// Asset service request/response types
type SaveAssetRequest struct {
	MediaFileID string            `json:"media_file_id"`
	AssetType   string            `json:"asset_type"`
	Category    string            `json:"category"`
	Subtype     string            `json:"subtype"`
	Data        []byte            `json:"data"`
	MimeType    string            `json:"mime_type"`
	SourceURL   string            `json:"source_url"`
	PluginID    string            `json:"plugin_id,omitempty"` // Plugin identifier for asset tracking
	Metadata    map[string]string `json:"metadata"`
}

type SaveAssetResponse struct {
	Success      bool   `json:"success"`
	Error        string `json:"error"`
	AssetID      uint32 `json:"asset_id"`
	Hash         string `json:"hash"`
	RelativePath string `json:"relative_path"`
}

type AssetExistsRequest struct {
	MediaFileID string `json:"media_file_id"`
	AssetType   string `json:"asset_type"`
	Category    string `json:"category"`
	Subtype     string `json:"subtype"`
	Hash        string `json:"hash"`
}

type AssetExistsResponse struct {
	Exists       bool   `json:"exists"`
	AssetID      uint32 `json:"asset_id"`
	RelativePath string `json:"relative_path"`
}

type RemoveAssetRequest struct {
	AssetID uint32 `json:"asset_id"`
}

type RemoveAssetResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// Enrichment service request/response types
type RegisterEnrichmentRequest struct {
	MediaFileID     string            `json:"media_file_id"`
	SourceName      string            `json:"source_name"`
	Enrichments     map[string]string `json:"enrichments"`
	ConfidenceScore float64           `json:"confidence_score"`
	MatchMetadata   map[string]string `json:"match_metadata"`
}

type RegisterEnrichmentResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	JobID   string `json:"job_id"`
}

// Logger interface for plugin logging
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	With(args ...interface{}) hclog.Logger
}

type EnrichmentService interface {
	RegisterEnrichment(mediaFileID, sourceName string, enrichments map[string]string, confidenceScore float64, metadata map[string]string) (string, error)
}

// Health monitoring interface for plugins
type HealthMonitorService interface {
	// GetHealthStatus returns the current health status of the plugin
	GetHealthStatus(ctx context.Context) (*HealthStatus, error)

	// GetMetrics returns performance metrics for the plugin
	GetMetrics(ctx context.Context) (*PluginMetrics, error)

	// SetHealthThresholds configures health check thresholds
	SetHealthThresholds(thresholds *HealthThresholds) error
}

// Plugin metrics and monitoring types
type HealthStatus struct {
	Status       string            `json:"status"` // "healthy", "degraded", "unhealthy"
	Message      string            `json:"message"`
	LastCheck    time.Time         `json:"last_check"`
	Uptime       time.Duration     `json:"uptime"`
	MemoryUsage  int64             `json:"memory_usage"`  // bytes
	CPUUsage     float64           `json:"cpu_usage"`     // percentage
	ErrorRate    float64           `json:"error_rate"`    // percentage
	ResponseTime time.Duration     `json:"response_time"` // average response time
	Details      map[string]string `json:"details"`
}

type PluginMetrics struct {
	ExecutionCount  int64                  `json:"execution_count"`
	SuccessCount    int64                  `json:"success_count"`
	ErrorCount      int64                  `json:"error_count"`
	AverageExecTime time.Duration          `json:"average_exec_time"`
	LastExecution   time.Time              `json:"last_execution"`
	BytesProcessed  int64                  `json:"bytes_processed"`
	ItemsProcessed  int64                  `json:"items_processed"`
	CacheHitRate    float64                `json:"cache_hit_rate"`
	CustomMetrics   map[string]interface{} `json:"custom_metrics"`
}

type HealthThresholds struct {
	MaxMemoryUsage      int64         `json:"max_memory_usage"` // bytes
	MaxCPUUsage         float64       `json:"max_cpu_usage"`    // percentage
	MaxErrorRate        float64       `json:"max_error_rate"`   // percentage
	MaxResponseTime     time.Duration `json:"max_response_time"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
}

// Performance monitoring interface for plugins
type PerformanceMonitorService interface {
	// GetPerformanceSnapshot returns current performance metrics
	GetPerformanceSnapshot(ctx context.Context) (*PerformanceSnapshot, error)

	// RecordOperation records an operation execution
	RecordOperation(operationName string, duration time.Duration, success bool, context string)

	// RecordError records an error event
	RecordError(errorType, message, context, operation string)

	// GetUptimeString returns human-readable uptime
	GetUptimeString() string

	// Reset resets all performance metrics
	Reset()
}

// Advanced configuration management interface
type ConfigurationService interface {
	// GetConfiguration returns the current plugin configuration
	GetConfiguration(ctx context.Context) (*PluginConfiguration, error)

	// UpdateConfiguration updates plugin configuration at runtime
	UpdateConfiguration(ctx context.Context, config *PluginConfiguration) error

	// ReloadConfiguration reloads configuration from source
	ReloadConfiguration(ctx context.Context) error

	// ValidateConfiguration validates a configuration before applying
	ValidateConfiguration(config *PluginConfiguration) (*ValidationResult, error)

	// GetConfigurationSchema returns the JSON schema for this plugin's configuration
	GetConfigurationSchema() (*ConfigurationSchema, error)
}

// Plugin configuration types
type PluginConfiguration struct {
	Version      string                 `json:"version"`
	Enabled      bool                   `json:"enabled"`
	Settings     map[string]interface{} `json:"settings"`
	Features     map[string]bool        `json:"features"`
	Thresholds   *HealthThresholds      `json:"thresholds,omitempty"`
	LastModified time.Time              `json:"last_modified"`
	ModifiedBy   string                 `json:"modified_by"`
}

type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

type ConfigurationSchema struct {
	Schema   map[string]interface{} `json:"schema"`   // JSON Schema
	Examples map[string]interface{} `json:"examples"` // Example configurations
	Defaults map[string]interface{} `json:"defaults"` // Default values
}

// Note: PerformanceSnapshot is defined in performance.go to avoid duplication
