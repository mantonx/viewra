// Package plugins provides the application layer service for managing plugins.
package plugins

import (
	"encoding/json"
	"time"
)

// PluginSummary contains basic plugin information for list views.
type PluginSummary struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	Description string         `json:"description"`
	Author      string         `json:"author"`
	Enabled     bool           `json:"enabled"`
	IsBuiltin   bool           `json:"is_builtin"`
	Health      string         `json:"health"`                // "healthy", "degraded", "unhealthy", "unknown"
	ProviderID  string         `json:"provider_id,omitempty"` // Provider ID if this is a provider plugin (e.g., "ollama")
	Meta        map[string]any `json:"meta,omitempty"`        // x-viewra-meta from settings schema

	// Settings availability
	HasSettings bool `json:"has_settings"` // Whether this plugin has configurable settings

	// Capabilities this plugin provides (e.g., "search", "embedding", "chat")
	Capabilities []string `json:"capabilities,omitempty"`

	// Dependencies this plugin requires but are not currently available
	// Used to show warnings in the UI about missing capabilities
	MissingDependencies []string `json:"missing_dependencies,omitempty"`

	// DisplayCategory is the computed category for UI grouping
	// Determined by: builtin → "Local", provider capability → "AI Providers",
	// search capability → "Search", enricher category → "Enrichers", else "Other"
	DisplayCategory string `json:"display_category"`
}

// PluginDetail contains full plugin information.
type PluginDetail struct {
	PluginSummary
	License       string    `json:"license"`
	Homepage      string    `json:"homepage"`
	InstalledAt   time.Time `json:"installed_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	RestartCount  int       `json:"restart_count"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	HealthMessage string    `json:"health_message,omitempty"`

	// EnricherCapabilities contains enricher-specific capabilities (nil for non-enrichers)
	EnricherCapabilities *EnricherCapabilities `json:"enricher_capabilities,omitempty"`
}

// EnricherCapabilities describes what an enricher plugin provides.
type EnricherCapabilities struct {
	MediaTypes []string `json:"media_types"`
	Provides   []string `json:"provides"`
	IsLocal    bool     `json:"is_local"`
	RateLimit  int      `json:"rate_limit"`
	Requires   []string `json:"requires,omitempty"`
}

// PluginSettings contains the settings schema and current values for a plugin.
type PluginSettings struct {
	PluginID string          `json:"plugin_id"`
	Schema   json.RawMessage `json:"schema"`
	Values   json.RawMessage `json:"values"`
}

// PluginHealthDetail contains detailed health information.
type PluginHealthDetail struct {
	Status        string    `json:"status"`
	Message       string    `json:"message"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	ErrorRate     float64   `json:"error_rate"`
	AvgLatencyMs  int64     `json:"avg_latency_ms"`
	Restarts      int       `json:"restarts"`
	UptimeSeconds int64     `json:"uptime_seconds"`
}

// LogEntry represents a single log entry from a plugin.
type LogEntry struct {
	Timestamp time.Time         `json:"timestamp"`
	Level     string            `json:"level"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
}

// LogOptions configures log retrieval.
type LogOptions struct {
	Limit int       // Max entries to return (default 100)
	Level string    // Filter by level (e.g., "error", "warn")
	Since time.Time // Only entries after this time
}

// FilterTab represents a filter tab for the plugins UI.
type FilterTab struct {
	ID    string `json:"id"`    // Tab identifier (e.g., "all", "enrichers")
	Label string `json:"label"` // Display label
	Count int    `json:"count"` // Number of plugins matching this filter
}

// ErrPluginNotFound is returned when a plugin doesn't exist.
type ErrPluginNotFound struct {
	PluginID string
}

func (e ErrPluginNotFound) Error() string {
	return "plugin not found: " + e.PluginID
}

// ErrBuiltinPlugin is returned when trying to disable a built-in plugin.
type ErrBuiltinPlugin struct {
	PluginID string
}

func (e ErrBuiltinPlugin) Error() string {
	return "cannot disable built-in plugin: " + e.PluginID
}
