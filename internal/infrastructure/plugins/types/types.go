// Package types provides shared types for the plugins infrastructure.
// These types are used across multiple sub-packages to avoid circular dependencies.
package types

import (
	"sync"
	"time"

	"github.com/hashicorp/go-plugin"
	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/manifest"
)

// Category defines the type of functionality a plugin provides.
type Category string

const (
	CategoryEnricher         Category = "enricher"
	CategoryNotificationSink Category = "notification_sink"
	CategoryProvider         Category = "provider"
)

// Handshake is the shared handshake config for all plugins.
// Both host and plugin must agree on these values.
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "VIEWRA_PLUGIN",
	MagicCookieValue: "viewra-plugin-v1",
}

// Health represents the current health status of a plugin.
type Health struct {
	Status        pluginv1.HealthStatus_Status
	Message       string
	LastHeartbeat time.Time
	ErrorRate     float64       // Errors per minute (rolling window)
	AvgLatency    time.Duration // Average response latency
	Restarts      int           // Number of times this plugin has been restarted
}

// Instance represents a loaded and running plugin.
type Instance struct {
	// ID is the unique identifier for this plugin.
	ID string

	// Manifest contains the plugin's static metadata from plugin.yml.
	Manifest *manifest.Manifest

	// Path is the filesystem path to the plugin binary.
	Path string

	// BuildTime is when the plugin binary was last modified (used as build time proxy).
	BuildTime time.Time

	// Client is the go-plugin client managing the plugin process.
	Client *plugin.Client

	// CoreClient provides access to the plugin's core RPC methods.
	CoreClient pluginv1.PluginCoreClient

	// EnricherClient provides access to enricher-specific methods (if applicable).
	EnricherClient pluginv1.EnricherClient

	// ProviderClient provides access to provider methods (if applicable).
	// This is set for plugins with category "provider".
	ProviderClient pluginv1.PluginProviderClient

	// Health tracks the plugin's current health status.
	Health Health

	// Categories lists which interfaces this plugin implements.
	Categories []Category

	// mu protects health updates.
	mu sync.RWMutex
}

// UpdateHealth updates the plugin's health status.
func (p *Instance) UpdateHealth(status pluginv1.HealthStatus_Status, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Health.Status = status
	p.Health.Message = message
	p.Health.LastHeartbeat = time.Now()
}

// IsHealthy returns true if the plugin is in a healthy state.
func (p *Instance) IsHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Health.Status == pluginv1.HealthStatus_HEALTHY
}

// GetHealthStatus returns the current health status.
func (p *Instance) GetHealthStatus() pluginv1.HealthStatus_Status {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Health.Status
}

// HasCategory returns true if the instance has the specified category.
func (p *Instance) HasCategory(cat Category) bool {
	for _, c := range p.Categories {
		if c == cat {
			return true
		}
	}
	return false
}

// ParseCategories converts string slice to Category slice.
func ParseCategories(categories []string) []Category {
	result := make([]Category, len(categories))
	for i, c := range categories {
		result[i] = Category(c)
	}
	return result
}

// HasCategory checks if a category slice contains a target category.
func HasCategoryIn(categories []Category, target Category) bool {
	for _, c := range categories {
		if c == target {
			return true
		}
	}
	return false
}
