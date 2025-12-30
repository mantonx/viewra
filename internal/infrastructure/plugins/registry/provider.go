package registry

import (
	"context"
	"fmt"
	"sync"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

// RegisteredProvider holds a provider plugin's client and capabilities.
type RegisteredProvider struct {
	// PluginID is the ID of the plugin providing this provider.
	PluginID string

	// ProviderID is the provider identifier (e.g., "ollama", "openai").
	ProviderID string

	// Client is the gRPC client for the provider.
	Client pluginv1.PluginProviderClient

	// CoreClient is the gRPC client for core plugin operations (settings, lifecycle).
	CoreClient pluginv1.PluginCoreClient

	// Capabilities describes what this provider supports.
	Capabilities *pluginv1.ProviderCapabilities
}

// ProviderRegistry manages AI provider plugins.
// It tracks which plugins provide AI capabilities (embeddings, chat)
// and routes requests to the appropriate plugin.
type ProviderRegistry struct {
	mu sync.RWMutex

	// providers maps provider ID (e.g., "ollama") to registered provider
	providers map[string]*RegisteredProvider

	// pluginProviders maps plugin ID to provider IDs it provides
	// (for cleanup when a plugin is unloaded)
	pluginProviders map[string][]string
}

// NewProviderRegistry creates a new provider registry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers:       make(map[string]*RegisteredProvider),
		pluginProviders: make(map[string][]string),
	}
}

// Register registers a provider plugin.
// Called when a plugin with category "provider" is loaded.
// Returns the capabilities of the registered provider.
func (r *ProviderRegistry) Register(
	ctx context.Context,
	pluginID string,
	client pluginv1.PluginProviderClient,
	coreClient pluginv1.PluginCoreClient,
) (*pluginv1.ProviderCapabilities, error) {
	// Get capabilities from the plugin
	caps, err := client.GetCapabilities(ctx, &pluginv1.Empty{})
	if err != nil {
		return nil, fmt.Errorf("failed to get provider capabilities: %w", err)
	}

	if caps.ProviderId == "" {
		return nil, fmt.Errorf("provider plugin %s did not return a provider_id", pluginID)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if this provider ID is already registered
	if existing, ok := r.providers[caps.ProviderId]; ok {
		return nil, fmt.Errorf("provider %s already registered by plugin %s", caps.ProviderId, existing.PluginID)
	}

	// Register the provider
	r.providers[caps.ProviderId] = &RegisteredProvider{
		PluginID:     pluginID,
		ProviderID:   caps.ProviderId,
		Client:       client,
		CoreClient:   coreClient,
		Capabilities: caps,
	}

	// Track for cleanup
	r.pluginProviders[pluginID] = append(r.pluginProviders[pluginID], caps.ProviderId)

	return caps, nil
}

// Unregister removes all providers from a plugin.
// Called when a plugin is unloaded.
func (r *ProviderRegistry) Unregister(pluginID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove all providers from this plugin
	for _, providerID := range r.pluginProviders[pluginID] {
		delete(r.providers, providerID)
	}
	delete(r.pluginProviders, pluginID)
}

// Get returns a provider by ID.
// Returns nil if no provider is registered with that ID.
func (r *ProviderRegistry) Get(providerID string) *RegisteredProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.providers[providerID]
}

// List returns all registered providers.
func (r *ProviderRegistry) List() []*RegisteredProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*RegisteredProvider, 0, len(r.providers))
	for _, p := range r.providers {
		result = append(result, p)
	}
	return result
}

// GetByPluginID returns the provider ID for a plugin.
// Returns empty string if the plugin is not a provider.
func (r *ProviderRegistry) GetByPluginID(pluginID string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providerIDs := r.pluginProviders[pluginID]
	if len(providerIDs) > 0 {
		return providerIDs[0] // Return first provider ID (plugins typically provide one)
	}
	return ""
}
