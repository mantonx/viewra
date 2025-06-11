package config

import "time"

// PluginReliabilityConfig contains reliability settings for external plugins
type PluginReliabilityConfig struct {
	// Timeout settings
	RequestTimeout     time.Duration `json:"request_timeout" default:"60s"`
	HealthCheckTimeout time.Duration `json:"health_check_timeout" default:"10s"`
	StartupTimeout     time.Duration `json:"startup_timeout" default:"30s"`
	ShutdownTimeout    time.Duration `json:"shutdown_timeout" default:"10s"`

	// Retry configuration
	MaxRetries        int           `json:"max_retries" default:"3"`
	InitialRetryDelay time.Duration `json:"initial_retry_delay" default:"1s"`
	MaxRetryDelay     time.Duration `json:"max_retry_delay" default:"30s"`
	BackoffMultiplier float64       `json:"backoff_multiplier" default:"2.0"`

	// Circuit breaker settings
	CircuitBreakerEnabled bool          `json:"circuit_breaker_enabled" default:"true"`
	FailureThreshold      int           `json:"failure_threshold" default:"5"`
	CircuitResetTimeout   time.Duration `json:"circuit_reset_timeout" default:"60s"`
	HalfOpenMaxCalls      int           `json:"half_open_max_calls" default:"3"`

	// Health monitoring
	HealthCheckInterval    time.Duration `json:"health_check_interval" default:"30s"`
	DegradedErrorRate      float64       `json:"degraded_error_rate" default:"0.1"`
	UnhealthyErrorRate     float64       `json:"unhealthy_error_rate" default:"0.25"`
	MaxConsecutiveFailures int           `json:"max_consecutive_failures" default:"5"`

	// Rate limiting for external APIs
	GlobalRateLimit    float64 `json:"global_rate_limit" default:"10.0"` // requests per second
	PerPluginRateLimit float64 `json:"per_plugin_rate_limit" default:"2.0"`
	BurstSize          int     `json:"burst_size" default:"5"`

	// Resource limits
	MaxMemoryUsage int64   `json:"max_memory_usage" default:"536870912"` // 512MB
	MaxCPUPercent  float64 `json:"max_cpu_percent" default:"50.0"`
	MaxDiskUsage   int64   `json:"max_disk_usage" default:"1073741824"` // 1GB

	// Fallback and degradation
	EnableGracefulDegradation bool          `json:"enable_graceful_degradation" default:"true"`
	FallbackCacheEnabled      bool          `json:"fallback_cache_enabled" default:"true"`
	FallbackCacheDuration     time.Duration `json:"fallback_cache_duration" default:"24h"`

	// Monitoring and alerting
	EnableDetailedMetrics      bool          `json:"enable_detailed_metrics" default:"true"`
	MetricsRetentionPeriod     time.Duration `json:"metrics_retention_period" default:"7d"`
	AlertOnDegradedPerformance bool          `json:"alert_on_degraded_performance" default:"true"`
	AlertOnPluginFailure       bool          `json:"alert_on_plugin_failure" default:"true"`

	// Auto-recovery settings
	AutoRestartEnabled    bool          `json:"auto_restart_enabled" default:"true"`
	MaxRestartAttempts    int           `json:"max_restart_attempts" default:"3"`
	RestartCooldownPeriod time.Duration `json:"restart_cooldown_period" default:"5m"`

	// Network resilience
	DNSCacheEnabled    bool          `json:"dns_cache_enabled" default:"true"`
	DNSCacheDuration   time.Duration `json:"dns_cache_duration" default:"5m"`
	ConnectionPoolSize int           `json:"connection_pool_size" default:"10"`
	KeepAliveTimeout   time.Duration `json:"keep_alive_timeout" default:"30s"`

	// Plugin-specific overrides
	PluginOverrides map[string]PluginSpecificReliabilityConfig `json:"plugin_overrides"`
}

// PluginSpecificReliabilityConfig allows per-plugin reliability configuration
type PluginSpecificReliabilityConfig struct {
	RequestTimeout      *time.Duration `json:"request_timeout,omitempty"`
	MaxRetries          *int           `json:"max_retries,omitempty"`
	RateLimit           *float64       `json:"rate_limit,omitempty"`
	FailureThreshold    *int           `json:"failure_threshold,omitempty"`
	HealthCheckInterval *time.Duration `json:"health_check_interval,omitempty"`

	// External API specific settings
	APIKeyRotationEnabled bool     `json:"api_key_rotation_enabled"`
	APIKeys               []string `json:"api_keys,omitempty"`
	UserAgents            []string `json:"user_agents,omitempty"`

	// Caching overrides
	CacheEnabled  *bool          `json:"cache_enabled,omitempty"`
	CacheDuration *time.Duration `json:"cache_duration,omitempty"`
	CacheMaxSize  *int64         `json:"cache_max_size,omitempty"`
}

// GetPluginConfig returns the effective configuration for a specific plugin
func (c *PluginReliabilityConfig) GetPluginConfig(pluginID string) PluginEffectiveConfig {
	base := PluginEffectiveConfig{
		RequestTimeout:      c.RequestTimeout,
		MaxRetries:          c.MaxRetries,
		InitialRetryDelay:   c.InitialRetryDelay,
		MaxRetryDelay:       c.MaxRetryDelay,
		BackoffMultiplier:   c.BackoffMultiplier,
		RateLimit:           c.PerPluginRateLimit,
		FailureThreshold:    c.FailureThreshold,
		HealthCheckInterval: c.HealthCheckInterval,
	}

	// Apply plugin-specific overrides
	if override, exists := c.PluginOverrides[pluginID]; exists {
		if override.RequestTimeout != nil {
			base.RequestTimeout = *override.RequestTimeout
		}
		if override.MaxRetries != nil {
			base.MaxRetries = *override.MaxRetries
		}
		if override.RateLimit != nil {
			base.RateLimit = *override.RateLimit
		}
		if override.FailureThreshold != nil {
			base.FailureThreshold = *override.FailureThreshold
		}
		if override.HealthCheckInterval != nil {
			base.HealthCheckInterval = *override.HealthCheckInterval
		}
	}

	return base
}

// PluginEffectiveConfig represents the final configuration after applying overrides
type PluginEffectiveConfig struct {
	RequestTimeout      time.Duration
	MaxRetries          int
	InitialRetryDelay   time.Duration
	MaxRetryDelay       time.Duration
	BackoffMultiplier   float64
	RateLimit           float64
	FailureThreshold    int
	HealthCheckInterval time.Duration
}

// DefaultPluginReliabilityConfig returns default reliability configuration
func DefaultPluginReliabilityConfig() *PluginReliabilityConfig {
	return &PluginReliabilityConfig{
		RequestTimeout:             60 * time.Second,
		HealthCheckTimeout:         10 * time.Second,
		StartupTimeout:             30 * time.Second,
		ShutdownTimeout:            10 * time.Second,
		MaxRetries:                 3,
		InitialRetryDelay:          1 * time.Second,
		MaxRetryDelay:              30 * time.Second,
		BackoffMultiplier:          2.0,
		CircuitBreakerEnabled:      true,
		FailureThreshold:           5,
		CircuitResetTimeout:        60 * time.Second,
		HalfOpenMaxCalls:           3,
		HealthCheckInterval:        30 * time.Second,
		DegradedErrorRate:          0.1,
		UnhealthyErrorRate:         0.25,
		MaxConsecutiveFailures:     5,
		GlobalRateLimit:            10.0,
		PerPluginRateLimit:         2.0,
		BurstSize:                  5,
		MaxMemoryUsage:             512 * 1024 * 1024, // 512MB
		MaxCPUPercent:              50.0,
		MaxDiskUsage:               1024 * 1024 * 1024, // 1GB
		EnableGracefulDegradation:  true,
		FallbackCacheEnabled:       true,
		FallbackCacheDuration:      24 * time.Hour,
		EnableDetailedMetrics:      true,
		MetricsRetentionPeriod:     7 * 24 * time.Hour,
		AlertOnDegradedPerformance: true,
		AlertOnPluginFailure:       true,
		AutoRestartEnabled:         true,
		MaxRestartAttempts:         3,
		RestartCooldownPeriod:      5 * time.Minute,
		DNSCacheEnabled:            true,
		DNSCacheDuration:           5 * time.Minute,
		ConnectionPoolSize:         10,
		KeepAliveTimeout:           30 * time.Second,
		PluginOverrides:            make(map[string]PluginSpecificReliabilityConfig),
	}
}
