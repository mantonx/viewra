package registry

import (
	"sync"
)

// CapabilityAliases maps well-known capabilities to stable URL paths.
// These paths are created as aliases to plugin routes when a plugin
// provides the corresponding capability.
var CapabilityAliases = map[string]string{
	"semantic_search": "/api/search",
	"similar_items":   "/api/similar",
	"recommendations": "/api/recommendations",
	"chat":            "/api/chat",
}

// CapabilityMapping tracks which plugin provides a capability and the plugin's route path.
type CapabilityMapping struct {
	PluginID   string // Plugin that provides this capability
	PluginPath string // Plugin's route path (e.g., "/search")
}

// CapabilityRegistry tracks which plugin provides each well-known capability.
// Only one plugin can provide each capability at a time (first wins).
type CapabilityRegistry struct {
	mu       sync.RWMutex
	mappings map[string]*CapabilityMapping // capability -> mapping
}

// NewCapabilityRegistry creates a new capability registry.
func NewCapabilityRegistry() *CapabilityRegistry {
	return &CapabilityRegistry{
		mappings: make(map[string]*CapabilityMapping),
	}
}

// Register registers a plugin as providing a capability.
// Returns true if registered, false if capability already provided by another plugin.
func (r *CapabilityRegistry) Register(pluginID, capability, pluginPath string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if already registered by another plugin
	if existing, ok := r.mappings[capability]; ok && existing.PluginID != pluginID {
		return false
	}

	r.mappings[capability] = &CapabilityMapping{
		PluginID:   pluginID,
		PluginPath: pluginPath,
	}
	return true
}

// Unregister removes all capabilities for a plugin.
func (r *CapabilityRegistry) Unregister(pluginID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for capability, mapping := range r.mappings {
		if mapping.PluginID == pluginID {
			delete(r.mappings, capability)
		}
	}
}

// Resolve returns the plugin and path for a capability.
// Returns nil if no plugin provides the capability.
func (r *CapabilityRegistry) Resolve(capability string) *CapabilityMapping {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.mappings[capability]
}

// GetCapabilitiesForPlugin returns all capabilities provided by a plugin.
func (r *CapabilityRegistry) GetCapabilitiesForPlugin(pluginID string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var caps []string
	for capability, mapping := range r.mappings {
		if mapping.PluginID == pluginID {
			caps = append(caps, capability)
		}
	}
	return caps
}

// ListAll returns all registered capabilities.
func (r *CapabilityRegistry) ListAll() map[string]*CapabilityMapping {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*CapabilityMapping, len(r.mappings))
	for k, v := range r.mappings {
		result[k] = v
	}
	return result
}
