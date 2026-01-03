// Package host provides gRPC server implementations for host services exposed to plugins.
package host

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"google.golang.org/grpc"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/types"
)

// PluginsServer implements the HostPlugins gRPC service.
// This allows plugins to discover and invoke methods on other plugins that provide
// specific capabilities (e.g., "embedding", "chat").
//
// The capability broker enables a plugin architecture where:
//   - Provider plugins declare capabilities they provide (e.g., ai-local provides "embedding", "chat")
//   - Consumer plugins can invoke methods on capability providers via the host
//   - The host manages capability resolution and proxies requests to providers
//   - Configuration plugins can set preferences to route capabilities to specific providers
//
// Key design: The host acts as a "dumb pipe" that routes requests to providers.
// Provider plugins dispatch method calls via a generic Invoke pattern. Consumer plugins
// use typed protos for requests/responses, while the transport uses generic bytes.
type PluginsServer struct {
	pluginv1.UnimplementedHostPluginsServer

	// mu protects concurrent access to capabilities and preferences
	mu sync.RWMutex

	// capabilities maps capability name to list of providers
	capabilities map[string][]*CapabilityProvider

	// preferences maps capability name to preferred plugin ID
	// Set by configuration plugins (e.g., ai-local) to route capabilities
	preferences map[string]string

	// pluginLookup provides access to plugin instances for invoking methods
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
}

// PluginLookup provides access to running plugin instances.
// Used to check plugin status and invoke methods on providers.
type PluginLookup interface {
	// GetPlugin returns a running plugin instance by ID.
	GetPlugin(id string) (*types.Instance, bool)

	// IsPluginEnabled returns true if the plugin is enabled and healthy.
	IsPluginEnabled(id string) bool
}

// NewPluginsServer creates a new PluginsServer.
func NewPluginsServer(lookup PluginLookup, logger *slog.Logger) *PluginsServer {
	return &PluginsServer{
		capabilities: make(map[string][]*CapabilityProvider),
		preferences:  make(map[string]string),
		pluginLookup: lookup,
		logger:       logger,
	}
}

// RegisterCapability registers a plugin as providing a capability.
// Called when a plugin is loaded and declares capabilities in its manifest.
func (s *PluginsServer) RegisterCapability(pluginID, pluginName, capability string) {
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
func (s *PluginsServer) UnregisterPlugin(pluginID string) {
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
func (s *PluginsServer) UpdatePluginStatus(pluginID string, enabled, configured bool) {
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

// resolveProvider finds the best available provider for a capability.
// Resolution order:
//  1. preferred_plugin parameter (explicit request)
//  2. Configured preference (set by configuration plugin like ai-local)
//  3. First available enabled provider
func (s *PluginsServer) resolveProvider(capability, preferredPlugin string) (*CapabilityProvider, *types.Instance, error) {
	s.mu.RLock()
	providers := s.capabilities[capability]
	configuredPreference := s.preferences[capability]
	s.mu.RUnlock()

	if len(providers) == 0 {
		return nil, nil, fmt.Errorf("no provider found for capability: %s", capability)
	}

	// 1. If preferred plugin is specified in request, try it first
	if preferredPlugin != "" {
		for _, p := range providers {
			if p.PluginID == preferredPlugin && p.Enabled && p.Configured {
				instance, ok := s.pluginLookup.GetPlugin(p.PluginID)
				if ok && instance.ProviderClient != nil {
					return p, instance, nil
				}
			}
		}
		// Fall through if preferred plugin not available
	}

	// 2. Check configured preference (set by configuration plugin like ai-local)
	if configuredPreference != "" {
		for _, p := range providers {
			if p.PluginID == configuredPreference && p.Enabled && p.Configured {
				instance, ok := s.pluginLookup.GetPlugin(p.PluginID)
				if ok && instance.ProviderClient != nil {
					s.logger.Debug("using configured preference",
						"capability", capability,
						"preferred_plugin", configuredPreference)
					return p, instance, nil
				}
			}
		}
		s.logger.Debug("configured preference not available, falling back",
			"capability", capability,
			"preferred_plugin", configuredPreference)
	}

	// 3. Find the first enabled and configured provider
	for _, p := range providers {
		if p.Enabled && p.Configured {
			instance, ok := s.pluginLookup.GetPlugin(p.PluginID)
			if ok && instance.ProviderClient != nil {
				return p, instance, nil
			}
		}
	}

	return nil, nil, fmt.Errorf("no enabled provider with ProviderClient for capability: %s", capability)
}

// HasCapability returns true if any enabled plugin provides the capability.
func (s *PluginsServer) HasCapability(capability string) bool {
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
func (s *PluginsServer) ListCapabilities(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.CapabilityListResponse, error) {
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
func (s *PluginsServer) ListProviders(ctx context.Context, req *pluginv1.CapabilityRequest) (*pluginv1.ProviderListResponse, error) {
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
// The preference is used when InvokeCapability is called without a preferred_plugin.
func (s *PluginsServer) SetCapabilityPreference(ctx context.Context, req *pluginv1.CapabilityPreferenceRequest) (*pluginv1.Empty, error) {
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
// After clearing, InvokeCapability falls back to first available provider.
func (s *PluginsServer) ClearCapabilityPreference(ctx context.Context, req *pluginv1.CapabilityPreferenceRequest) (*pluginv1.Empty, error) {
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
func (s *PluginsServer) GetCapabilityPreferences(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.CapabilityPreferencesResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Copy preferences to avoid holding lock during response serialization
	prefs := make(map[string]string, len(s.preferences))
	for k, v := range s.preferences {
		prefs[k] = v
	}

	return &pluginv1.CapabilityPreferencesResponse{Preferences: prefs}, nil
}

// InvokeCapability forwards a request to a capability provider.
// The host resolves the capability to an available provider and proxies the request.
// This is the core of the cross-plugin RPC system.
func (s *PluginsServer) InvokeCapability(ctx context.Context, req *pluginv1.CapabilityInvokeRequest) (*pluginv1.CapabilityInvokeResponse, error) {
	// Resolve provider
	provider, instance, err := s.resolveProvider(req.Capability, req.PreferredPlugin)
	if err != nil {
		return &pluginv1.CapabilityInvokeResponse{
			Error: &pluginv1.CapabilityError{
				Code:    pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_NOT_FOUND,
				Message: err.Error(),
			},
		}, nil
	}

	s.logger.Debug("invoking capability",
		"capability", req.Capability,
		"method", req.Method,
		"provider", provider.PluginID)

	// Forward request to provider
	providerReq := &pluginv1.ProviderInvokeRequest{
		Method:     req.Method,
		Payload:    req.RequestPayload,
		ApiVersion: req.ApiVersion,
		Metadata:   req.Metadata,
	}

	providerResp, err := instance.ProviderClient.Invoke(ctx, providerReq)
	if err != nil {
		s.logger.Error("provider invoke failed",
			"capability", req.Capability,
			"method", req.Method,
			"provider", provider.PluginID,
			"error", err)
		return &pluginv1.CapabilityInvokeResponse{
			Error: &pluginv1.CapabilityError{
				Code:      pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_PROVIDER_ERROR,
				Message:   err.Error(),
				Retryable: true,
			},
		}, nil
	}

	// Convert provider error to capability error if present
	if providerResp.Error != nil {
		return &pluginv1.CapabilityInvokeResponse{
			Error: &pluginv1.CapabilityError{
				Code:      mapProviderErrorCode(providerResp.Error.Code),
				Message:   providerResp.Error.Message,
				Retryable: providerResp.Error.Retryable,
			},
		}, nil
	}

	// Add provider info to metadata
	metadata := providerResp.Metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata["provider_plugin_id"] = provider.PluginID
	metadata["provider_plugin_name"] = provider.PluginName

	return &pluginv1.CapabilityInvokeResponse{
		ResponsePayload: providerResp.Payload,
		Metadata:        metadata,
		ProviderPlugin:  provider.PluginID,
	}, nil
}

// InvokeCapabilityStream forwards a streaming request to a capability provider.
// Used for server-streaming methods like ChatStream.
func (s *PluginsServer) InvokeCapabilityStream(req *pluginv1.CapabilityInvokeRequest, stream grpc.ServerStreamingServer[pluginv1.CapabilityInvokeResponse]) error {
	ctx := stream.Context()

	// Resolve provider
	provider, instance, err := s.resolveProvider(req.Capability, req.PreferredPlugin)
	if err != nil {
		return stream.Send(&pluginv1.CapabilityInvokeResponse{
			Error: &pluginv1.CapabilityError{
				Code:    pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_NOT_FOUND,
				Message: err.Error(),
			},
		})
	}

	s.logger.Debug("invoking capability stream",
		"capability", req.Capability,
		"method", req.Method,
		"provider", provider.PluginID)

	// Forward request to provider's streaming method
	providerReq := &pluginv1.ProviderInvokeRequest{
		Method:     req.Method,
		Payload:    req.RequestPayload,
		ApiVersion: req.ApiVersion,
		Metadata:   req.Metadata,
	}

	providerStream, err := instance.ProviderClient.InvokeStream(ctx, providerReq)
	if err != nil {
		s.logger.Error("provider invoke stream failed",
			"capability", req.Capability,
			"method", req.Method,
			"provider", provider.PluginID,
			"error", err)
		return stream.Send(&pluginv1.CapabilityInvokeResponse{
			Error: &pluginv1.CapabilityError{
				Code:      pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_PROVIDER_ERROR,
				Message:   err.Error(),
				Retryable: true,
			},
		})
	}

	// Forward all chunks from provider to consumer
	for {
		providerResp, err := providerStream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			s.logger.Error("provider stream recv failed",
				"capability", req.Capability,
				"method", req.Method,
				"provider", provider.PluginID,
				"error", err)
			return stream.Send(&pluginv1.CapabilityInvokeResponse{
				Error: &pluginv1.CapabilityError{
					Code:      pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_PROVIDER_ERROR,
					Message:   err.Error(),
					Retryable: true,
				},
			})
		}

		// Convert provider error to capability error if present
		if providerResp.Error != nil {
			if err := stream.Send(&pluginv1.CapabilityInvokeResponse{
				Error: &pluginv1.CapabilityError{
					Code:      mapProviderErrorCode(providerResp.Error.Code),
					Message:   providerResp.Error.Message,
					Retryable: providerResp.Error.Retryable,
				},
			}); err != nil {
				return err
			}
			continue
		}

		// Add provider info to metadata
		metadata := providerResp.Metadata
		if metadata == nil {
			metadata = make(map[string]string)
		}
		metadata["provider_plugin_id"] = provider.PluginID
		metadata["provider_plugin_name"] = provider.PluginName

		if err := stream.Send(&pluginv1.CapabilityInvokeResponse{
			ResponsePayload: providerResp.Payload,
			Metadata:        metadata,
			ProviderPlugin:  provider.PluginID,
		}); err != nil {
			return err
		}
	}
}

// DescribeCapability returns metadata about a capability's available methods.
// Useful for discovering what methods a capability supports.
func (s *PluginsServer) DescribeCapability(ctx context.Context, req *pluginv1.DescribeCapabilityRequest) (*pluginv1.DescribeCapabilityResponse, error) {
	s.mu.RLock()
	providers := s.capabilities[req.Capability]
	s.mu.RUnlock()

	if len(providers) == 0 {
		return nil, fmt.Errorf("no provider found for capability: %s", req.Capability)
	}

	// Build provider info list
	var providerInfos []*pluginv1.DescribeCapabilityProviderInfo
	var methods []*pluginv1.CapabilityMethodInfo
	var apiVersion string
	var supportedVersions []string

	for _, p := range providers {
		if !p.Enabled || !p.Configured {
			providerInfos = append(providerInfos, &pluginv1.DescribeCapabilityProviderInfo{
				PluginId:   p.PluginID,
				PluginName: p.PluginName,
				Enabled:    false,
			})
			continue
		}

		instance, ok := s.pluginLookup.GetPlugin(p.PluginID)
		if !ok || instance.ProviderClient == nil {
			providerInfos = append(providerInfos, &pluginv1.DescribeCapabilityProviderInfo{
				PluginId:   p.PluginID,
				PluginName: p.PluginName,
				Enabled:    false,
			})
			continue
		}

		// Get methods from provider
		methodsResp, err := instance.ProviderClient.DescribeMethods(ctx, &pluginv1.Empty{})
		if err != nil {
			s.logger.Warn("failed to get provider methods",
				"provider", p.PluginID,
				"error", err)
			providerInfos = append(providerInfos, &pluginv1.DescribeCapabilityProviderInfo{
				PluginId:   p.PluginID,
				PluginName: p.PluginName,
				Enabled:    false,
			})
			continue
		}

		// Use first available provider's methods as canonical
		if len(methods) == 0 && methodsResp != nil {
			apiVersion = methodsResp.ApiVersion
			supportedVersions = methodsResp.SupportedVersions
			for _, m := range methodsResp.Methods {
				methods = append(methods, &pluginv1.CapabilityMethodInfo{
					Name:         m.Name,
					Description:  m.Description,
					IsStreaming:  m.IsStreaming,
					RequestType:  m.RequestType,
					ResponseType: m.ResponseType,
				})
			}
		}

		providerInfos = append(providerInfos, &pluginv1.DescribeCapabilityProviderInfo{
			PluginId:   p.PluginID,
			PluginName: p.PluginName,
			Enabled:    true,
			Configured: true,
		})
	}

	return &pluginv1.DescribeCapabilityResponse{
		Capability:        req.Capability,
		ApiVersion:        apiVersion,
		SupportedVersions: supportedVersions,
		Methods:           methods,
		Providers:         providerInfos,
	}, nil
}

// InvokeVectorSearch forwards a vector search request to a plugin providing vector_search capability.
// This enables plugin-to-plugin semantic search operations (e.g., recommendations → semantic-search).
func (s *PluginsServer) InvokeVectorSearch(ctx context.Context, req *pluginv1.VectorSearchInvokeRequest) (*pluginv1.VectorSearchInvokeResponse, error) {
	// Resolve provider for vector_search capability
	provider, instance, err := s.resolveVectorSearchProvider(req.PreferredPlugin)
	if err != nil {
		return &pluginv1.VectorSearchInvokeResponse{
			Error: &pluginv1.CapabilityError{
				Code:    pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_NOT_FOUND,
				Message: err.Error(),
			},
		}, nil
	}

	s.logger.Debug("invoking vector search",
		"method", req.Method,
		"provider", provider.PluginID)

	var resp *pluginv1.SemanticSearchResponse

	switch req.Method {
	case "FindSimilar":
		if req.FindSimilar == nil {
			return &pluginv1.VectorSearchInvokeResponse{
				Error: &pluginv1.CapabilityError{
					Code:    pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_INVALID_REQUEST,
					Message: "FindSimilar request is required for method FindSimilar",
				},
			}, nil
		}
		resp, err = instance.VectorSearchClient.FindSimilar(ctx, req.FindSimilar)

	case "Search":
		if req.Search == nil {
			return &pluginv1.VectorSearchInvokeResponse{
				Error: &pluginv1.CapabilityError{
					Code:    pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_INVALID_REQUEST,
					Message: "Search request is required for method Search",
				},
			}, nil
		}
		resp, err = instance.VectorSearchClient.Search(ctx, req.Search)

	default:
		return &pluginv1.VectorSearchInvokeResponse{
			Error: &pluginv1.CapabilityError{
				Code:    pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_METHOD_NOT_FOUND,
				Message: fmt.Sprintf("unknown vector search method: %s", req.Method),
			},
		}, nil
	}

	if err != nil {
		s.logger.Error("vector search failed",
			"method", req.Method,
			"provider", provider.PluginID,
			"error", err)
		return &pluginv1.VectorSearchInvokeResponse{
			Error: &pluginv1.CapabilityError{
				Code:      pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_PROVIDER_ERROR,
				Message:   err.Error(),
				Retryable: true,
			},
		}, nil
	}

	return &pluginv1.VectorSearchInvokeResponse{
		ProviderPlugin: provider.PluginID,
		Response:       resp,
	}, nil
}

// resolveVectorSearchProvider finds an available provider for vector_search capability.
// Unlike resolveProvider, this specifically looks for plugins with VectorSearchClient.
func (s *PluginsServer) resolveVectorSearchProvider(preferredPlugin string) (*CapabilityProvider, *types.Instance, error) {
	const capability = "vector_search"

	s.mu.RLock()
	providers := s.capabilities[capability]
	configuredPreference := s.preferences[capability]
	s.mu.RUnlock()

	if len(providers) == 0 {
		return nil, nil, fmt.Errorf("no provider found for capability: %s", capability)
	}

	// 1. If preferred plugin is specified in request, try it first
	if preferredPlugin != "" {
		for _, p := range providers {
			if p.PluginID == preferredPlugin && p.Enabled && p.Configured {
				instance, ok := s.pluginLookup.GetPlugin(p.PluginID)
				if ok && instance.VectorSearchClient != nil {
					return p, instance, nil
				}
			}
		}
		// Fall through if preferred plugin not available
	}

	// 2. Check configured preference
	if configuredPreference != "" {
		for _, p := range providers {
			if p.PluginID == configuredPreference && p.Enabled && p.Configured {
				instance, ok := s.pluginLookup.GetPlugin(p.PluginID)
				if ok && instance.VectorSearchClient != nil {
					s.logger.Debug("using configured preference for vector_search",
						"preferred_plugin", configuredPreference)
					return p, instance, nil
				}
			}
		}
		s.logger.Debug("configured preference not available for vector_search, falling back",
			"preferred_plugin", configuredPreference)
	}

	// 3. Find the first enabled and configured provider with VectorSearchClient
	for _, p := range providers {
		if p.Enabled && p.Configured {
			instance, ok := s.pluginLookup.GetPlugin(p.PluginID)
			if ok && instance.VectorSearchClient != nil {
				return p, instance, nil
			}
		}
	}

	return nil, nil, fmt.Errorf("no enabled provider with VectorSearchClient for capability: %s", capability)
}

// mapProviderErrorCode converts a provider error code string to a CapabilityErrorCode.
func mapProviderErrorCode(code string) pluginv1.CapabilityErrorCode {
	switch code {
	case "METHOD_NOT_FOUND":
		return pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_METHOD_NOT_FOUND
	case "INVALID_REQUEST":
		return pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_INVALID_REQUEST
	case "PROVIDER_ERROR":
		return pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_PROVIDER_ERROR
	case "TIMEOUT":
		return pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_TIMEOUT
	case "RATE_LIMITED":
		return pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_RATE_LIMITED
	case "NOT_CONFIGURED":
		return pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_NOT_CONFIGURED
	default:
		return pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_PROVIDER_ERROR
	}
}
