package plugins

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

// HostPluginsServer implements the HostPlugins gRPC service.
// This allows plugins to discover and connect to other plugins that provide
// specific capabilities (e.g., "embedding", "chat").
//
// The capability broker enables a plugin architecture where:
//   - Provider plugins declare capabilities they provide (e.g., ai-local provides "embedding", "chat")
//   - Consumer plugins can request connections to capability providers
//   - The host manages capability resolution and plugin-to-plugin connections
//   - Configuration plugins can set preferences to route capabilities to specific providers
type HostPluginsServer struct {
	pluginv1.UnimplementedHostPluginsServer

	// mu protects concurrent access to capabilities and preferences
	mu sync.RWMutex

	// capabilities maps capability name to list of providers
	capabilities map[string][]*CapabilityProvider

	// preferences maps capability name to preferred plugin ID
	// Set by configuration plugins (e.g., ai-local) to route capabilities
	preferences map[string]string

	// pluginLookup provides access to plugin instances for broker connection
	pluginLookup PluginLookup

	// logger for debugging capability resolution
	logger *slog.Logger
}

// CapabilityProvider describes a plugin that provides a specific capability.
type CapabilityProvider struct {
	PluginID   string // Unique plugin identifier
	PluginName string // Human-readable name
	Enabled    bool   // Whether the plugin is currently enabled
	Configured bool   // Whether the plugin is properly configured
	BrokerID   uint32 // gRPC broker ID for plugin-to-plugin connection (0 if not available)
}

// PluginLookup provides access to running plugin instances.
// Used to check plugin status and get broker connections.
type PluginLookup interface {
	// GetPlugin returns a running plugin instance by ID.
	GetPlugin(id string) (*PluginInstance, bool)

	// IsPluginEnabled returns true if the plugin is enabled and healthy.
	IsPluginEnabled(id string) bool
}

// NewHostPluginsServer creates a new HostPluginsServer.
func NewHostPluginsServer(lookup PluginLookup, logger *slog.Logger) *HostPluginsServer {
	return &HostPluginsServer{
		capabilities: make(map[string][]*CapabilityProvider),
		preferences:  make(map[string]string),
		pluginLookup: lookup,
		logger:       logger,
	}
}

// RegisterCapability registers a plugin as providing a capability.
// Called when a plugin is loaded and declares capabilities in its manifest.
func (s *HostPluginsServer) RegisterCapability(pluginID, pluginName, capability string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if this plugin is already registered for this capability
	providers := s.capabilities[capability]
	for _, p := range providers {
		if p.PluginID == pluginID {
			// Already registered, update name
			p.PluginName = pluginName
			return
		}
	}

	// Add new provider
	s.capabilities[capability] = append(providers, &CapabilityProvider{
		PluginID:   pluginID,
		PluginName: pluginName,
		Enabled:    true, // Assume enabled when registering
		Configured: true, // Assume configured when registering
	})

	s.logger.Debug("registered capability provider",
		"capability", capability,
		"plugin_id", pluginID,
		"plugin_name", pluginName)
}

// UnregisterPlugin removes all capabilities for a plugin.
// Called when a plugin is unloaded.
func (s *HostPluginsServer) UnregisterPlugin(pluginID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for capability, providers := range s.capabilities {
		var remaining []*CapabilityProvider
		for _, p := range providers {
			if p.PluginID != pluginID {
				remaining = append(remaining, p)
			}
		}
		if len(remaining) == 0 {
			delete(s.capabilities, capability)
		} else {
			s.capabilities[capability] = remaining
		}
	}

	s.logger.Debug("unregistered plugin capabilities", "plugin_id", pluginID)
}

// UpdatePluginStatus updates the enabled/configured status for a plugin.
func (s *HostPluginsServer) UpdatePluginStatus(pluginID string, enabled, configured bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, providers := range s.capabilities {
		for _, p := range providers {
			if p.PluginID == pluginID {
				p.Enabled = enabled
				p.Configured = configured
			}
		}
	}
}

// GetCapabilityProvider returns connection info for a plugin providing the capability.
// This is the core of the capability broker - it finds a provider plugin and asks it
// to expose its service, returning the broker ID so the consumer can connect.
//
// Resolution order:
//  1. preferred_plugin parameter (explicit request)
//  2. Configured preference (set by configuration plugin like ai-local)
//  3. First available enabled provider
func (s *HostPluginsServer) GetCapabilityProvider(ctx context.Context, req *pluginv1.CapabilityRequest) (*pluginv1.CapabilityProviderResponse, error) {
	s.mu.RLock()
	providers := s.capabilities[req.Capability]
	preferredPluginID := s.preferences[req.Capability]
	s.mu.RUnlock()

	if len(providers) == 0 {
		return &pluginv1.CapabilityProviderResponse{
			Available: false,
			Error:     "no provider found for capability: " + req.Capability,
		}, nil
	}

	// 1. If preferred plugin is specified in request, try it first
	if req.PreferredPlugin != "" {
		for _, p := range providers {
			if p.PluginID == req.PreferredPlugin && p.Enabled && p.Configured {
				return s.requestServiceExposure(ctx, p, req.Capability, "")
			}
		}
		// Preferred plugin not available, fall through
	}

	// 2. Check configured preference (set by configuration plugin like ai-local)
	if preferredPluginID != "" {
		for _, p := range providers {
			if p.PluginID == preferredPluginID && p.Enabled && p.Configured {
				s.logger.Debug("using configured preference",
					"capability", req.Capability,
					"preferred_plugin", preferredPluginID)
				return s.requestServiceExposure(ctx, p, req.Capability, "")
			}
		}
		// Configured preference not available, fall through
		s.logger.Debug("configured preference not available, falling back",
			"capability", req.Capability,
			"preferred_plugin", preferredPluginID)
	}

	// 3. Find the first enabled and configured provider
	for _, p := range providers {
		if p.Enabled && p.Configured {
			return s.requestServiceExposure(ctx, p, req.Capability, "")
		}
	}

	// No enabled provider found
	return &pluginv1.CapabilityProviderResponse{
		Available: false,
		Error:     "no enabled provider for capability: " + req.Capability,
	}, nil
}

// requestServiceExposure asks a provider plugin to expose its service for the given capability.
// It calls the provider's ExposeService RPC to get a broker ID that consumers can dial.
func (s *HostPluginsServer) requestServiceExposure(ctx context.Context, p *CapabilityProvider, capability, requestingPluginID string) (*pluginv1.CapabilityProviderResponse, error) {
	// Get the plugin instance to access its CoreClient
	instance, ok := s.pluginLookup.GetPlugin(p.PluginID)
	if !ok {
		return &pluginv1.CapabilityProviderResponse{
			Available: false,
			Error:     "provider plugin not found: " + p.PluginID,
		}, nil
	}

	if instance.CoreClient == nil {
		return &pluginv1.CapabilityProviderResponse{
			Available: false,
			Error:     "provider plugin has no core client: " + p.PluginID,
		}, nil
	}

	// Ask the provider to expose its service
	resp, err := instance.CoreClient.ExposeService(ctx, &pluginv1.ExposeServiceRequest{
		Capability:         capability,
		RequestingPluginId: requestingPluginID,
	})
	if err != nil {
		s.logger.Error("failed to request service exposure",
			"provider", p.PluginID,
			"capability", capability,
			"error", err)
		return &pluginv1.CapabilityProviderResponse{
			Available: false,
			Error:     "failed to expose service: " + err.Error(),
		}, nil
	}

	if !resp.Success {
		return &pluginv1.CapabilityProviderResponse{
			Available: false,
			Error:     resp.Error,
		}, nil
	}

	s.logger.Debug("service exposed successfully",
		"provider", p.PluginID,
		"capability", capability,
		"broker_id", resp.BrokerId,
		"service_name", resp.ServiceName)

	return &pluginv1.CapabilityProviderResponse{
		Available:  true,
		BrokerId:   resp.BrokerId,
		PluginId:   p.PluginID,
		PluginName: p.PluginName,
	}, nil
}

// HasCapability returns true if any enabled plugin provides the capability.
func (s *HostPluginsServer) HasCapability(capability string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	providers := s.capabilities[capability]
	for _, p := range providers {
		if p.Enabled {
			return true
		}
	}
	return false
}

// ListCapabilities returns all available capabilities and their providers.
func (s *HostPluginsServer) ListCapabilities(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.CapabilityListResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var caps []*pluginv1.CapabilityInfo
	for name, providers := range s.capabilities {
		var providerInfos []*pluginv1.PluginProviderInfo
		for _, p := range providers {
			providerInfos = append(providerInfos, &pluginv1.PluginProviderInfo{
				PluginId:   p.PluginID,
				PluginName: p.PluginName,
				Enabled:    p.Enabled,
				Configured: p.Configured,
			})
		}
		caps = append(caps, &pluginv1.CapabilityInfo{
			Name:      name,
			Providers: providerInfos,
		})
	}

	return &pluginv1.CapabilityListResponse{Capabilities: caps}, nil
}

// ListProviders returns all plugins providing a specific capability.
func (s *HostPluginsServer) ListProviders(ctx context.Context, req *pluginv1.CapabilityRequest) (*pluginv1.ProviderListResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	providers := s.capabilities[req.Capability]
	var providerInfos []*pluginv1.PluginProviderInfo
	for _, p := range providers {
		providerInfos = append(providerInfos, &pluginv1.PluginProviderInfo{
			PluginId:   p.PluginID,
			PluginName: p.PluginName,
			Enabled:    p.Enabled,
			Configured: p.Configured,
		})
	}

	return &pluginv1.ProviderListResponse{Providers: providerInfos}, nil
}

// SetCapabilityPreference sets the preferred plugin for a capability.
// Used by configuration plugins (e.g., ai-local) to route capabilities to specific providers.
// The preference is used when GetCapabilityProvider is called without a preferred_plugin.
func (s *HostPluginsServer) SetCapabilityPreference(ctx context.Context, req *pluginv1.CapabilityPreferenceRequest) (*pluginv1.Empty, error) {
	if req.Capability == "" {
		return nil, fmt.Errorf("capability is required")
	}
	if req.PluginId == "" {
		return nil, fmt.Errorf("plugin_id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.preferences[req.Capability] = req.PluginId

	s.logger.Info("capability preference set",
		"capability", req.Capability,
		"plugin_id", req.PluginId)

	return &pluginv1.Empty{}, nil
}

// ClearCapabilityPreference removes the preference for a capability.
// After clearing, GetCapabilityProvider falls back to first available provider.
func (s *HostPluginsServer) ClearCapabilityPreference(ctx context.Context, req *pluginv1.CapabilityPreferenceRequest) (*pluginv1.Empty, error) {
	if req.Capability == "" {
		return nil, fmt.Errorf("capability is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.preferences, req.Capability)

	s.logger.Info("capability preference cleared",
		"capability", req.Capability)

	return &pluginv1.Empty{}, nil
}

// GetCapabilityPreferences returns all configured capability preferences.
func (s *HostPluginsServer) GetCapabilityPreferences(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.CapabilityPreferencesResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Copy preferences to avoid holding lock during response serialization
	prefs := make(map[string]string, len(s.preferences))
	for k, v := range s.preferences {
		prefs[k] = v
	}

	return &pluginv1.CapabilityPreferencesResponse{Preferences: prefs}, nil
}
