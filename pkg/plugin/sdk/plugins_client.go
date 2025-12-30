// PluginsClient provides access to other plugins via the capability broker.
//
// This allows plugins to discover and connect to other plugins that provide
// specific capabilities like "embedding", "chat", or custom plugin-defined
// capabilities.
//
// # Usage
//
// The PluginsClient is obtained from the HostServices:
//
//	plugins := hostServices.Plugins
//	if plugins.IsAvailable(ctx, "embedding") {
//	    conn, err := plugins.GetConnection(ctx, "embedding")
//	    // Use conn to call the embedding provider
//	}
//
// # Capabilities
//
// Capabilities are declared in plugin manifests via the "provides" field.
// Common capabilities include:
//   - "embedding" - Vector embedding generation
//   - "chat" - Chat/completion generation
//   - "semantic_search" - Semantic search over media
//
// Plugins can also define custom capabilities for inter-plugin communication.
package sdk

import (
	"context"
	"fmt"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"google.golang.org/grpc"
)

// PluginsClient provides capability-based access to other plugins.
type PluginsClient struct {
	client pluginv1.HostPluginsClient
	broker GRPCBroker
}

// GRPCBroker is the interface for the go-plugin broker that establishes
// plugin-to-plugin connections.
type GRPCBroker interface {
	// Dial creates a gRPC connection to a broker ID returned by the host.
	Dial(id uint32) (*grpc.ClientConn, error)
}

// NewPluginsClient creates a new plugins client for capability discovery.
func NewPluginsClient(conn *grpc.ClientConn, broker GRPCBroker) *PluginsClient {
	return &PluginsClient{
		client: pluginv1.NewHostPluginsClient(conn),
		broker: broker,
	}
}

// GetConnection returns a gRPC connection to a plugin providing the capability.
// The host resolves the capability to an available, enabled plugin.
//
// Example:
//
//	conn, err := plugins.GetConnection(ctx, "embedding")
//	if err != nil {
//	    return err
//	}
//	defer conn.Close()
//	embeddingClient := embedpb.NewEmbeddingServiceClient(conn)
func (c *PluginsClient) GetConnection(ctx context.Context, capability string) (*grpc.ClientConn, error) {
	return c.GetConnectionPreferred(ctx, capability, "")
}

// GetConnectionPreferred returns a connection to a specific plugin providing the capability.
// If preferredPlugin is empty or unavailable, falls back to any available provider.
//
// Example:
//
//	// Prefer ollama for embeddings, fall back to openai if unavailable
//	conn, err := plugins.GetConnectionPreferred(ctx, "embedding", "provider-ollama")
func (c *PluginsClient) GetConnectionPreferred(ctx context.Context, capability, preferredPlugin string) (*grpc.ClientConn, error) {
	resp, err := c.client.GetCapabilityProvider(ctx, &pluginv1.CapabilityRequest{
		Capability:      capability,
		PreferredPlugin: preferredPlugin,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get capability provider: %w", err)
	}

	if !resp.Available {
		if resp.Error != "" {
			return nil, fmt.Errorf("capability %q not available: %s", capability, resp.Error)
		}
		return nil, fmt.Errorf("capability %q not available: no provider found", capability)
	}

	// Use the broker to establish a connection to the provider plugin
	conn, err := c.broker.Dial(resp.BrokerId)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to provider %q: %w", resp.PluginId, err)
	}

	return conn, nil
}

// IsAvailable checks if any plugin provides the specified capability.
//
// Example:
//
//	if plugins.IsAvailable(ctx, "embedding") {
//	    // Embedding capability is available
//	}
func (c *PluginsClient) IsAvailable(ctx context.Context, capability string) bool {
	providers, err := c.ListProviders(ctx, capability)
	if err != nil {
		return false
	}
	for _, p := range providers {
		if p.Enabled && p.Configured {
			return true
		}
	}
	return false
}

// ListProviders returns all plugins providing a specific capability.
// Includes both enabled and disabled plugins for UI display purposes.
//
// Example:
//
//	providers, err := plugins.ListProviders(ctx, "embedding")
//	for _, p := range providers {
//	    fmt.Printf("Provider: %s (enabled: %v)\n", p.Name, p.Enabled)
//	}
func (c *PluginsClient) ListProviders(ctx context.Context, capability string) ([]PluginProvider, error) {
	resp, err := c.client.ListProviders(ctx, &pluginv1.CapabilityRequest{
		Capability: capability,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list providers: %w", err)
	}

	providers := make([]PluginProvider, len(resp.Providers))
	for i, p := range resp.Providers {
		providers[i] = PluginProvider{
			ID:         p.PluginId,
			Name:       p.PluginName,
			Enabled:    p.Enabled,
			Configured: p.Configured,
		}
	}
	return providers, nil
}

// ListCapabilities returns all available capabilities and their providers.
// Useful for discovering what capabilities are available in the system.
//
// Example:
//
//	caps, err := plugins.ListCapabilities(ctx)
//	for _, cap := range caps {
//	    fmt.Printf("Capability: %s, Providers: %d\n", cap.Name, len(cap.Providers))
//	}
func (c *PluginsClient) ListCapabilities(ctx context.Context) ([]Capability, error) {
	resp, err := c.client.ListCapabilities(ctx, &pluginv1.Empty{})
	if err != nil {
		return nil, fmt.Errorf("failed to list capabilities: %w", err)
	}

	caps := make([]Capability, len(resp.Capabilities))
	for i, cap := range resp.Capabilities {
		providers := make([]PluginProvider, len(cap.Providers))
		for j, p := range cap.Providers {
			providers[j] = PluginProvider{
				ID:         p.PluginId,
				Name:       p.PluginName,
				Enabled:    p.Enabled,
				Configured: p.Configured,
			}
		}
		caps[i] = Capability{
			Name:      cap.Name,
			Providers: providers,
		}
	}
	return caps, nil
}

// PluginProvider describes a plugin that provides a capability.
type PluginProvider struct {
	ID         string // Plugin ID (e.g., "provider-ollama")
	Name       string // Human-readable name (e.g., "Ollama Provider")
	Enabled    bool   // Whether the plugin is enabled
	Configured bool   // Whether the plugin is properly configured
}

// Capability describes an available capability and its providers.
type Capability struct {
	Name      string           // Capability name (e.g., "embedding")
	Providers []PluginProvider // Plugins providing this capability
}

// SetCapabilityPreference sets the preferred plugin for a capability.
// Used by configuration plugins (e.g., ai-local) to route capabilities to specific providers.
// When other plugins request this capability, the preferred plugin will be used.
//
// Example:
//
//	// Route all embedding requests to OpenAI
//	err := plugins.SetCapabilityPreference(ctx, "embedding", "provider-openai")
func (c *PluginsClient) SetCapabilityPreference(ctx context.Context, capability, pluginID string) error {
	_, err := c.client.SetCapabilityPreference(ctx, &pluginv1.CapabilityPreferenceRequest{
		Capability: capability,
		PluginId:   pluginID,
	})
	if err != nil {
		return fmt.Errorf("failed to set capability preference: %w", err)
	}
	return nil
}

// ClearCapabilityPreference removes the preference for a capability.
// After clearing, GetConnection falls back to the first available provider.
//
// Example:
//
//	err := plugins.ClearCapabilityPreference(ctx, "embedding")
func (c *PluginsClient) ClearCapabilityPreference(ctx context.Context, capability string) error {
	_, err := c.client.ClearCapabilityPreference(ctx, &pluginv1.CapabilityPreferenceRequest{
		Capability: capability,
	})
	if err != nil {
		return fmt.Errorf("failed to clear capability preference: %w", err)
	}
	return nil
}

// GetCapabilityPreferences returns all configured capability preferences.
// Returns a map of capability name to preferred plugin ID.
//
// Example:
//
//	prefs, err := plugins.GetCapabilityPreferences(ctx)
//	if embeddingPlugin, ok := prefs["embedding"]; ok {
//	    fmt.Printf("Embedding provider: %s\n", embeddingPlugin)
//	}
func (c *PluginsClient) GetCapabilityPreferences(ctx context.Context) (map[string]string, error) {
	resp, err := c.client.GetCapabilityPreferences(ctx, &pluginv1.Empty{})
	if err != nil {
		return nil, fmt.Errorf("failed to get capability preferences: %w", err)
	}
	return resp.Preferences, nil
}
