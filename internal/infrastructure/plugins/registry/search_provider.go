package registry

import (
	"sort"
	"sync"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// RegisteredSearchProvider represents a registered search provider.
type RegisteredSearchProvider struct {
	PluginID string
	Info     *sdk.SearchProviderInfo
	Enabled  bool
}

// SearchProviderRegistry manages multiple search providers with priority-based selection.
// The highest priority enabled provider is used as the default.
type SearchProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]*RegisteredSearchProvider // pluginID -> provider
}

// NewSearchProviderRegistry creates a new search provider registry.
func NewSearchProviderRegistry() *SearchProviderRegistry {
	return &SearchProviderRegistry{
		providers: make(map[string]*RegisteredSearchProvider),
	}
}

// Register registers a plugin as a search provider.
// If already registered, updates the info.
func (r *SearchProviderRegistry) Register(pluginID string, info *sdk.SearchProviderInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.providers[pluginID]; ok {
		// Update existing
		existing.Info = info
		return
	}

	r.providers[pluginID] = &RegisteredSearchProvider{
		PluginID: pluginID,
		Info:     info,
		Enabled:  true, // Default to enabled
	}
}

// Unregister removes a search provider.
func (r *SearchProviderRegistry) Unregister(pluginID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.providers, pluginID)
}

// SetEnabled sets whether a search provider is enabled.
func (r *SearchProviderRegistry) SetEnabled(pluginID string, enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if provider, ok := r.providers[pluginID]; ok {
		provider.Enabled = enabled
	}
}

// GetDefault returns the highest priority enabled search provider.
// Returns nil if no enabled providers exist.
func (r *SearchProviderRegistry) GetDefault() *RegisteredSearchProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var best *RegisteredSearchProvider
	for _, provider := range r.providers {
		if !provider.Enabled {
			continue
		}
		if best == nil || provider.Info.Priority > best.Info.Priority {
			best = provider
		}
	}
	return best
}

// Get returns a specific search provider by plugin ID.
// Returns nil if not found.
func (r *SearchProviderRegistry) Get(pluginID string) *RegisteredSearchProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.providers[pluginID]
}

// GetAll returns all registered search providers, sorted by priority (highest first).
func (r *SearchProviderRegistry) GetAll() []*RegisteredSearchProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*RegisteredSearchProvider, 0, len(r.providers))
	for _, provider := range r.providers {
		result = append(result, provider)
	}

	// Sort by priority (highest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Info.Priority > result[j].Info.Priority
	})

	return result
}

// GetEnabled returns all enabled search providers, sorted by priority.
func (r *SearchProviderRegistry) GetEnabled() []*RegisteredSearchProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*RegisteredSearchProvider, 0)
	for _, provider := range r.providers {
		if provider.Enabled {
			result = append(result, provider)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Info.Priority > result[j].Info.Priority
	})

	return result
}

// Count returns the number of registered providers.
func (r *SearchProviderRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.providers)
}

// HasEnabled returns true if at least one provider is enabled.
func (r *SearchProviderRegistry) HasEnabled() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, provider := range r.providers {
		if provider.Enabled {
			return true
		}
	}
	return false
}
