package registry

import (
	"sort"
	"sync"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// RegisteredTrendingProvider represents a registered trending provider.
type RegisteredTrendingProvider struct {
	PluginID string
	Info     *sdk.TrendingProviderInfo
	Enabled  bool
}

// TrendingProviderRegistry manages multiple trending providers with priority-based selection.
// The first enabled provider is used as the default (currently no priority, first registered wins).
type TrendingProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]*RegisteredTrendingProvider // pluginID -> provider
}

// NewTrendingProviderRegistry creates a new trending provider registry.
func NewTrendingProviderRegistry() *TrendingProviderRegistry {
	return &TrendingProviderRegistry{
		providers: make(map[string]*RegisteredTrendingProvider),
	}
}

// Register registers a plugin as a trending provider.
// If already registered, updates the info.
func (r *TrendingProviderRegistry) Register(pluginID string, info *sdk.TrendingProviderInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.providers[pluginID]; ok {
		// Update existing
		existing.Info = info
		return
	}

	r.providers[pluginID] = &RegisteredTrendingProvider{
		PluginID: pluginID,
		Info:     info,
		Enabled:  true, // Default to enabled
	}
}

// Unregister removes a trending provider.
func (r *TrendingProviderRegistry) Unregister(pluginID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.providers, pluginID)
}

// SetEnabled sets whether a trending provider is enabled.
func (r *TrendingProviderRegistry) SetEnabled(pluginID string, enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if provider, ok := r.providers[pluginID]; ok {
		provider.Enabled = enabled
	}
}

// GetDefault returns the first enabled trending provider.
// Returns nil if no enabled providers exist.
// Note: Unlike search providers, trending doesn't use priority ordering currently.
func (r *TrendingProviderRegistry) GetDefault() *RegisteredTrendingProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, provider := range r.providers {
		if provider.Enabled {
			return provider
		}
	}
	return nil
}

// Get returns a specific trending provider by plugin ID.
// Returns nil if not found.
func (r *TrendingProviderRegistry) Get(pluginID string) *RegisteredTrendingProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.providers[pluginID]
}

// GetByProviderID returns a trending provider by its provider info ID.
// Returns nil if not found.
func (r *TrendingProviderRegistry) GetByProviderID(providerID string) *RegisteredTrendingProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, provider := range r.providers {
		if provider.Info != nil && provider.Info.ID == providerID {
			return provider
		}
	}
	return nil
}

// GetAll returns all registered trending providers, sorted by name.
func (r *TrendingProviderRegistry) GetAll() []*RegisteredTrendingProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*RegisteredTrendingProvider, 0, len(r.providers))
	for _, provider := range r.providers {
		result = append(result, provider)
	}

	// Sort by name for consistent ordering
	sort.Slice(result, func(i, j int) bool {
		return result[i].Info.Name < result[j].Info.Name
	})

	return result
}

// GetEnabled returns all enabled trending providers, sorted by name.
func (r *TrendingProviderRegistry) GetEnabled() []*RegisteredTrendingProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*RegisteredTrendingProvider, 0)
	for _, provider := range r.providers {
		if provider.Enabled {
			result = append(result, provider)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Info.Name < result[j].Info.Name
	})

	return result
}

// Count returns the number of registered providers.
func (r *TrendingProviderRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.providers)
}

// HasEnabled returns true if at least one provider is enabled.
func (r *TrendingProviderRegistry) HasEnabled() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, provider := range r.providers {
		if provider.Enabled {
			return true
		}
	}
	return false
}

// SupportsWindow checks if any enabled provider supports the given time window.
func (r *TrendingProviderRegistry) SupportsWindow(window string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, provider := range r.providers {
		if !provider.Enabled || provider.Info == nil {
			continue
		}
		for _, w := range provider.Info.Windows {
			if w == window {
				return true
			}
		}
	}
	return false
}

// SupportsMediaType checks if any enabled provider supports the given media type.
func (r *TrendingProviderRegistry) SupportsMediaType(mediaType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, provider := range r.providers {
		if !provider.Enabled || provider.Info == nil {
			continue
		}
		for _, mt := range provider.Info.MediaTypes {
			if mt == mediaType || mt == "all" {
				return true
			}
		}
	}
	return false
}
