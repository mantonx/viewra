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
	TranscodingProvider() TranscodingProvider
	EnhancedAdminPageService() EnhancedAdminPageService
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

// Enhanced admin page service with dynamic content support
type EnhancedAdminPageService interface {
	AdminPageService
	// GetPageStatus returns real-time status data for a specific admin page
	GetPageStatus(ctx context.Context, pageID string) (*AdminPageStatus, error)
	// ExecutePageAction executes an action on a specific admin page
	ExecutePageAction(ctx context.Context, pageID, actionID string, payload map[string]interface{}) (*AdminPageActionResult, error)
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
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	URL         string                 `json:"url"`
	Icon        string                 `json:"icon"`
	Category    string                 `json:"category"`
	Type        string                 `json:"type"` // Type of admin page: configuration, dashboard, status, external
	Description string                 `json:"description,omitempty"`
	StatusAPI   *AdminPageStatusAPI    `json:"status_api,omitempty"`
	Actions     []*AdminPageAction     `json:"actions,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type AdminPageStatusAPI struct {
	Endpoint        string        `json:"endpoint"`         // API endpoint to get status data
	RefreshInterval time.Duration `json:"refresh_interval"` // How often to refresh data
	StatusIndicator string        `json:"status_indicator"` // How to determine status color
}

type AdminPageAction struct {
	ID         string                 `json:"id"`
	Label      string                 `json:"label"`
	Type       string                 `json:"type"`       // "button", "toggle", "dropdown"
	Style      string                 `json:"style"`      // "primary", "secondary", "danger", etc.
	Endpoint   string                 `json:"endpoint"`   // API endpoint to call
	Method     string                 `json:"method"`     // HTTP method
	Confirm    bool                   `json:"confirm"`    // Whether to show confirmation dialog
	Payload    map[string]interface{} `json:"payload"`    // Optional payload data
	Conditions map[string]interface{} `json:"conditions"` // When to show/enable this action
}

type AdminPageStatus struct {
	Status      string                 `json:"status"`       // "active", "idle", "error", etc.
	Color       string                 `json:"color"`        // "green", "yellow", "red", "blue", etc.
	Message     string                 `json:"message"`      // Status message
	Indicators  []*StatusIndicator     `json:"indicators"`   // Key-value status indicators
	Progress    *ProgressInfo          `json:"progress"`     // Progress information if applicable
	LastUpdated time.Time              `json:"last_updated"` // When status was last updated
	Metadata    map[string]interface{} `json:"metadata"`     // Additional status data
}

type StatusIndicator struct {
	Key     string `json:"key"`     // e.g., "Active Sessions"
	Value   string `json:"value"`   // e.g., "3 jobs"
	Color   string `json:"color"`   // Optional color for the value
	Tooltip string `json:"tooltip"` // Optional tooltip
}

type ProgressInfo struct {
	Current    int64  `json:"current"`
	Total      int64  `json:"total"`
	Percentage int    `json:"percentage"`
	Label      string `json:"label"`
}

type AdminPageActionResult struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data"`
	Error   string                 `json:"error"`
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

// Dashboard section interfaces for plugin-defined admin UI

// DashboardSectionProvider defines how plugins can provide dashboard sections
type DashboardSectionProvider interface {
	// GetDashboardSections returns the dashboard sections this plugin provides
	GetDashboardSections(ctx context.Context) ([]DashboardSection, error)
}

// DashboardSection represents a plugin-defined section of the admin dashboard
type DashboardSection struct {
	ID          string                 `json:"id"`
	PluginID    string                 `json:"plugin_id"`
	Type        string                 `json:"type"` // "transcoder", "metadata", "storage", etc.
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Icon        string                 `json:"icon"`
	Priority    int                    `json:"priority"` // Display order
	Config      DashboardSectionConfig `json:"config"`
	Manifest    DashboardManifest      `json:"manifest"`
}

// DashboardSectionConfig defines section behavior
type DashboardSectionConfig struct {
	RefreshInterval  int  `json:"refresh_interval"`  // Seconds
	SupportsRealtime bool `json:"supports_realtime"` // WebSocket support
	HasNerdPanel     bool `json:"has_nerd_panel"`    // Advanced metrics toggle
	RequiresAuth     bool `json:"requires_auth"`
	MinRefreshRate   int  `json:"min_refresh_rate"` // Rate limiting
	MaxDataPoints    int  `json:"max_data_points"`  // History limit
}

// DashboardManifest describes the UI components and data endpoints
type DashboardManifest struct {
	ComponentType string                  `json:"component_type"` // "builtin", "custom", "iframe"
	DataEndpoints map[string]DataEndpoint `json:"data_endpoints"`
	Actions       []DashboardAction       `json:"actions"`
	UISchema      map[string]interface{}  `json:"ui_schema"` // For builtin components
}

// DataEndpoint describes how to fetch section data
type DataEndpoint struct {
	Path        string            `json:"path"`
	Method      string            `json:"method"`
	DataType    string            `json:"data_type"` // "main", "nerd", "metrics"
	CacheKey    string            `json:"cache_key"`
	Headers     map[string]string `json:"headers"`
	Description string            `json:"description"`
}

// DashboardAction represents user actions within a section
type DashboardAction struct {
	ID       string                 `json:"id"`
	Label    string                 `json:"label"`
	Icon     string                 `json:"icon"`
	Style    string                 `json:"style"` // "primary", "danger", etc.
	Endpoint string                 `json:"endpoint"`
	Method   string                 `json:"method"`
	Confirm  bool                   `json:"confirm"`
	Payload  map[string]interface{} `json:"payload"`
	Shortcut string                 `json:"shortcut"` // Keyboard shortcut
}

// Real-time data interfaces

// DashboardDataProvider provides real-time dashboard data
type DashboardDataProvider interface {
	// GetMainData returns the primary dashboard data
	GetMainData(ctx context.Context, sectionID string) (interface{}, error)

	// GetNerdData returns advanced/detailed metrics
	GetNerdData(ctx context.Context, sectionID string) (interface{}, error)

	// GetMetrics returns time-series metrics data
	GetMetrics(ctx context.Context, sectionID string, timeRange TimeRange) ([]MetricPoint, error)

	// StreamData provides real-time updates via channel
	StreamData(ctx context.Context, sectionID string) (<-chan DashboardUpdate, error)
}

// TimeRange for metrics queries
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Step  string    `json:"step"` // "1m", "5m", "1h", etc.
}

// MetricPoint represents a timestamped metric value
type MetricPoint struct {
	Timestamp time.Time              `json:"timestamp"`
	Value     float64                `json:"value"`
	Labels    map[string]string      `json:"labels"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// DashboardUpdate for real-time streaming
type DashboardUpdate struct {
	SectionID string      `json:"section_id"`
	DataType  string      `json:"data_type"` // "main", "nerd", "metrics"
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
	EventType string      `json:"event_type"` // "update", "add", "remove"
}

// RealtimeDataStreamer provides WebSocket streaming capabilities
type RealtimeDataStreamer interface {
	// StartStreaming starts a WebSocket stream for real-time updates
	StartStreaming(ctx context.Context, sectionID string, clientID string) (<-chan StreamUpdate, error)

	// StopStreaming stops the stream for a specific client
	StopStreaming(sectionID string, clientID string) error

	// GetStreamingClients returns active streaming clients
	GetStreamingClients(sectionID string) []string
}

// StreamUpdate represents a real-time update sent via WebSocket
type StreamUpdate struct {
	Type      string                 `json:"type"` // "session_update", "metric_update", "status_change"
	SectionID string                 `json:"section_id"`
	DataType  string                 `json:"data_type"` // "main", "nerd", "metrics"
	Data      interface{}            `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
	ClientID  string                 `json:"client_id"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// Transcoder-specific dashboard data structures

// TranscoderMainData represents the main view for transcoding plugins
type TranscoderMainData struct {
	ActiveSessions []TranscodeSessionSummary `json:"active_sessions"`
	QueuedSessions []TranscodeSessionSummary `json:"queued_sessions"`
	RecentSessions []TranscodeSessionSummary `json:"recent_sessions"`
	EngineStatus   TranscoderEngineStatus    `json:"engine_status"`
	QuickStats     TranscoderQuickStats      `json:"quick_stats"`
}

// TranscoderNerdData represents advanced metrics for transcoding plugins
type TranscoderNerdData struct {
	EncoderQueues      []EncoderQueueInfo `json:"encoder_queues"`
	HardwareStatus     HardwareStatusInfo `json:"hardware_status"`
	PerformanceMetrics PerformanceMetrics `json:"performance_metrics"`
	ConfigDiagnostics  []ConfigDiagnostic `json:"config_diagnostics"`
	SystemResources    SystemResourceInfo `json:"system_resources"`
}

// TranscodeSessionSummary for dashboard display
type TranscodeSessionSummary struct {
	ID                string    `json:"id"`
	InputFilename     string    `json:"input_filename"`
	InputResolution   string    `json:"input_resolution"`
	OutputResolution  string    `json:"output_resolution"`
	InputCodec        string    `json:"input_codec"`
	OutputCodec       string    `json:"output_codec"`
	Bitrate           string    `json:"bitrate"`
	Duration          string    `json:"duration"`
	Progress          float64   `json:"progress"`
	TranscoderType    string    `json:"transcoder_type"` // "software", "nvenc", "vaapi", etc.
	ClientIP          string    `json:"client_ip"`
	ClientDevice      string    `json:"client_device"`
	StartTime         time.Time `json:"start_time"`
	Status            string    `json:"status"`
	EstimatedTimeLeft string    `json:"estimated_time_left"`
	ThroughputFPS     float64   `json:"throughput_fps"`
}

// TranscoderEngineStatus shows the overall health of the transcoding engine
type TranscoderEngineStatus struct {
	Type            string    `json:"type"`
	Status          string    `json:"status"`
	Version         string    `json:"version"`
	MaxConcurrent   int       `json:"max_concurrent"`
	ActiveSessions  int       `json:"active_sessions"`
	QueuedSessions  int       `json:"queued_sessions"`
	LastHealthCheck time.Time `json:"last_health_check"`
	Capabilities    []string  `json:"capabilities"`
}

// TranscoderQuickStats for at-a-glance metrics
type TranscoderQuickStats struct {
	SessionsToday     int     `json:"sessions_today"`
	TotalHoursToday   float64 `json:"total_hours_today"`
	AverageSpeed      float64 `json:"average_speed"`
	ErrorRate         float64 `json:"error_rate"`
	CurrentThroughput string  `json:"current_throughput"`
	PeakConcurrent    int     `json:"peak_concurrent"`
}

// EncoderQueueInfo for advanced diagnostics
type EncoderQueueInfo struct {
	QueueID     string `json:"queue_id"`
	Type        string `json:"type"`
	Pending     int    `json:"pending"`
	Processing  int    `json:"processing"`
	MaxSlots    int    `json:"max_slots"`
	AvgWaitTime string `json:"avg_wait_time"`
}

// HardwareStatusInfo for hardware-accelerated transcoders
type HardwareStatusInfo struct {
	GPU            GPUInfo       `json:"gpu"`
	Encoders       []EncoderInfo `json:"encoders"`
	Memory         MemoryInfo    `json:"memory"`
	Temperature    int           `json:"temperature"`
	PowerDraw      float64       `json:"power_draw"`
	UtilizationPct float64       `json:"utilization_pct"`
}

// GPUInfo represents GPU hardware status
type GPUInfo struct {
	Name        string `json:"name"`
	Driver      string `json:"driver"`
	VRAMTotal   int64  `json:"vram_total"`
	VRAMUsed    int64  `json:"vram_used"`
	CoreClock   int    `json:"core_clock"`
	MemoryClock int    `json:"memory_clock"`
	FanSpeed    int    `json:"fan_speed"`
}

// EncoderInfo represents individual encoder status
type EncoderInfo struct {
	ID           string  `json:"id"`
	Type         string  `json:"type"`
	Status       string  `json:"status"`
	CurrentLoad  float64 `json:"current_load"`
	SessionCount int     `json:"session_count"`
	MaxSessions  int     `json:"max_sessions"`
}

// MemoryInfo represents memory usage
type MemoryInfo struct {
	System SystemMemory `json:"system"`
	GPU    GPUMemory    `json:"gpu"`
}

// SystemMemory represents system RAM usage
type SystemMemory struct {
	Total  int64 `json:"total"`
	Used   int64 `json:"used"`
	Cached int64 `json:"cached"`
}

// GPUMemory represents GPU memory usage
type GPUMemory struct {
	Total int64 `json:"total"`
	Used  int64 `json:"used"`
	Free  int64 `json:"free"`
}

// PerformanceMetrics for detailed performance analysis
type PerformanceMetrics struct {
	EncodingSpeed    float64 `json:"encoding_speed"`
	QualityScore     float64 `json:"quality_score"`
	CompressionRatio float64 `json:"compression_ratio"`
	ErrorCount       int     `json:"error_count"`
	RestartCount     int     `json:"restart_count"`
	UptimeSeconds    int64   `json:"uptime_seconds"`
}

// ConfigDiagnostic for configuration validation
type ConfigDiagnostic struct {
	Category   string `json:"category"`
	Level      string `json:"level"` // "info", "warning", "error"
	Message    string `json:"message"`
	Setting    string `json:"setting"`
	Value      string `json:"value"`
	Suggestion string `json:"suggestion"`
}

// SystemResourceInfo for system resource monitoring
type SystemResourceInfo struct {
	CPU     CPUInfo     `json:"cpu"`
	Memory  MemoryInfo  `json:"memory"`
	Disk    DiskInfo    `json:"disk"`
	Network NetworkInfo `json:"network"`
}

// CPUInfo represents CPU usage
type CPUInfo struct {
	Usage     float64 `json:"usage"`
	Cores     int     `json:"cores"`
	Threads   int     `json:"threads"`
	Frequency int     `json:"frequency"`
}

// DiskInfo represents disk usage
type DiskInfo struct {
	TotalSpace int64   `json:"total_space"`
	UsedSpace  int64   `json:"used_space"`
	IOReads    int64   `json:"io_reads"`
	IOWrites   int64   `json:"io_writes"`
	IOUtil     float64 `json:"io_util"`
}

// NetworkInfo represents network usage
type NetworkInfo struct {
	BytesReceived int64   `json:"bytes_received"`
	BytesSent     int64   `json:"bytes_sent"`
	PacketsRx     int64   `json:"packets_rx"`
	PacketsTx     int64   `json:"packets_tx"`
	Bandwidth     float64 `json:"bandwidth"`
}
