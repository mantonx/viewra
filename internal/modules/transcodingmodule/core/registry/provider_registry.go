// Package registry provides a centralized provider management system for the transcoding module.
// It encapsulates provider registration, discovery, and selection logic.
package registry

import (
	"sync"

	"github.com/hashicorp/go-hclog"
	tErrors "github.com/mantonx/viewra/internal/modules/transcodingmodule/errors"
	plugins "github.com/mantonx/viewra/sdk"
)

// ProviderRegistry manages transcoding provider registration and selection.
// It provides a clean interface for provider management and eliminates the need
// for direct provider map manipulation in the main Manager.
type ProviderRegistry struct {
	providers map[string]plugins.TranscodingProvider
	mutex     sync.RWMutex
	logger    hclog.Logger
}

// NewProviderRegistry creates a new provider registry.
func NewProviderRegistry(logger hclog.Logger) *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]plugins.TranscodingProvider),
		logger:    logger.Named("provider-registry"),
	}
}

// Register adds a provider to the registry.
// It logs the registration and handles thread-safe access to the provider map.
func (pr *ProviderRegistry) Register(provider plugins.TranscodingProvider) {
	pr.mutex.Lock()
	defer pr.mutex.Unlock()

	info := provider.GetInfo()
	pr.providers[info.ID] = provider
	
	pr.logger.Info("registered transcoding provider",
		"id", info.ID,
		"name", info.Name,
		"priority", info.Priority)
}

// GetProvider returns a specific provider by ID.
func (pr *ProviderRegistry) GetProvider(providerID string) (plugins.TranscodingProvider, error) {
	pr.mutex.RLock()
	defer pr.mutex.RUnlock()

	provider, exists := pr.providers[providerID]
	if !exists {
		return nil, tErrors.ProviderError("get_provider", tErrors.ErrProviderNotFound).
			WithDetail("provider_id", providerID)
	}

	return provider, nil
}

// GetProviders returns information about all registered providers.
func (pr *ProviderRegistry) GetProviders() []plugins.ProviderInfo {
	pr.mutex.RLock()
	defer pr.mutex.RUnlock()

	providers := make([]plugins.ProviderInfo, 0, len(pr.providers))
	for _, provider := range pr.providers {
		providers = append(providers, provider.GetInfo())
	}

	return providers
}

// SelectProvider chooses the best provider for a given request.
// It prioritizes providers based on format support and priority.
func (pr *ProviderRegistry) SelectProvider(req plugins.TranscodeRequest) (plugins.TranscodingProvider, error) {
	pr.mutex.RLock()
	defer pr.mutex.RUnlock()

	// Find all providers that support the requested format
	var candidates []plugins.TranscodingProvider
	for _, provider := range pr.providers {
		formats := provider.GetSupportedFormats()
		for _, format := range formats {
			if format.Format == req.Container {
				candidates = append(candidates, provider)
				break
			}
		}
	}

	if len(candidates) == 0 {
		return nil, tErrors.ProviderError("select_provider", tErrors.ErrNoProvidersAvailable).
			WithDetail("format", req.Container)
	}

	// Select provider with highest priority
	var selected plugins.TranscodingProvider
	highestPriority := -1

	for _, provider := range candidates {
		info := provider.GetInfo()
		if info.Priority > highestPriority {
			selected = provider
			highestPriority = info.Priority
		}
	}

	return selected, nil
}

// Count returns the number of registered providers.
func (pr *ProviderRegistry) Count() int {
	pr.mutex.RLock()
	defer pr.mutex.RUnlock()
	return len(pr.providers)
}

// Clear removes all providers from the registry.
// This is primarily useful for testing.
func (pr *ProviderRegistry) Clear() {
	pr.mutex.Lock()
	defer pr.mutex.Unlock()
	pr.providers = make(map[string]plugins.TranscodingProvider)
	pr.logger.Debug("cleared all providers from registry")
}