package pluginmodule

import (
	"net/http"
	"time"
)

// HTTP Status Codes
const (
	StatusOK                  = http.StatusOK
	StatusBadRequest          = http.StatusBadRequest
	StatusNotFound            = http.StatusNotFound
	StatusInternalServerError = http.StatusInternalServerError
)

// Default Configuration Values
const (
	DefaultHostServiceAddr = "localhost:50051"
	DefaultLogLevel        = "debug"
	DefaultPluginDir       = "./viewra-data/plugins"
	DefaultFilePerm        = 0755
)

// Environment Variable Names
const (
	EnvPluginID        = "VIEWRA_PLUGIN_ID"
	EnvDatabaseURL     = "VIEWRA_DATABASE_URL"
	EnvHostServiceAddr = "VIEWRA_HOST_SERVICE_ADDR"
	EnvLogLevel        = "VIEWRA_LOG_LEVEL"
	EnvBasePath        = "VIEWRA_BASE_PATH"
)

// Error Messages
const (
	ErrPluginIDRequired   = "Plugin ID is required"
	ErrPluginNameRequired = "Plugin name is required"
	ErrPluginNotFound     = "Plugin not found"
	ErrPluginInterface    = "plugin does not implement required interface"
)

// Health Monitoring Constants
const (
	DefaultMaxHistorySize = 100
	DefaultMemoryLimit    = 512 * 1024 * 1024 // 512MB
	PercentMultiplier     = 100.0
	WarningThreshold      = 0.8 // 80% of maximum
)

// Timeouts and Intervals
const (
	DefaultHealthCheckInterval = 30 * time.Second
	DefaultRequestTimeout      = 10 * time.Second
	ProcessCheckInterval       = 5 * time.Second
	RetryDelay                 = 1 * time.Second
	MaxRetries                 = 3
)

// Cache and Storage Limits
const (
	DefaultMaxCacheSize = 100 * 1024 * 1024       // 100MB
	DefaultMaxFileSize  = 10 * 1024 * 1024 * 1024 // 10GB
)

// Plugin Status Values
const (
	StatusStarting = "starting"
	StatusRunning  = "running"
	StatusStopped  = "stopped"
	StatusError    = "error"
	StatusLoading  = "loading"
	StatusEnabled  = "enabled"
	StatusDisabled = "disabled"
)

// Health Status Values
const (
	HealthStatusHealthy   = "healthy"
	HealthStatusDegraded  = "degraded"
	HealthStatusUnhealthy = "unhealthy"
	HealthStatusUnknown   = "unknown"
)
