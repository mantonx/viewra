// PluginsClient provides access to other plugins via the host-proxied capability invoke.
//
// This allows plugins to invoke methods on other plugins that provide specific
// capabilities like "embedding", "chat", or custom plugin-defined capabilities.
// The host acts as a proxy, routing requests to the appropriate provider.
//
// # Usage
//
// The PluginsClient is obtained from the HostServices:
//
//	plugins := hostServices.Plugins
//	if plugins.IsAvailable(ctx, "embedding") {
//	    respBytes, meta, err := plugins.Invoke(ctx, "embedding", "GenerateEmbedding", req)
//	    // Unmarshal respBytes to get the typed response
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
	"io"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// PluginsClient provides capability-based access to other plugins.
type PluginsClient struct {
	client pluginv1.HostPluginsClient
}

// NewPluginsClient creates a new plugins client for capability discovery and invocation.
func NewPluginsClient(conn *grpc.ClientConn) *PluginsClient {
	return &PluginsClient{
		client: pluginv1.NewHostPluginsClient(conn),
	}
}

// Invoke calls a method on a capability provider and returns the response payload.
// The caller is responsible for marshaling the request and unmarshaling the response.
//
// Example:
//
//	req := &pluginv1.ProviderEmbeddingRequest{Text: "hello"}
//	respBytes, meta, err := plugins.Invoke(ctx, "embedding", "GenerateEmbedding", req)
//	if err != nil {
//	    return err
//	}
//	var resp pluginv1.ProviderEmbeddingResponse
//	proto.Unmarshal(respBytes, &resp)
func (c *PluginsClient) Invoke(ctx context.Context, capability, method string, req proto.Message, opts ...InvokeOption) ([]byte, *InvokeMetadata, error) {
	options := &invokeOptions{}
	for _, opt := range opts {
		opt(options)
	}

	payload, err := proto.Marshal(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.client.InvokeCapability(ctx, &pluginv1.CapabilityInvokeRequest{
		Capability:      capability,
		Method:          method,
		RequestPayload:  payload,
		PreferredPlugin: options.preferredPlugin,
		ApiVersion:      options.apiVersion,
		Metadata:        options.metadata,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("invoke failed: %w", err)
	}

	if resp.Error != nil {
		return nil, nil, &CapabilityError{
			Code:      resp.Error.Code,
			Message:   resp.Error.Message,
			Details:   resp.Error.Details,
			Retryable: resp.Error.Retryable,
		}
	}

	meta := &InvokeMetadata{
		ProviderPlugin: resp.ProviderPlugin,
		LatencyMs:      resp.LatencyMs,
		Metadata:       resp.Metadata,
	}

	return resp.ResponsePayload, meta, nil
}

// InvokeStream calls a streaming method on a capability provider.
// Returns a channel that receives response chunks until closed.
//
// Example:
//
//	req := &pluginv1.ProviderChatRequest{...}
//	chunks, err := plugins.InvokeStream(ctx, "chat", "ChatStream", req)
//	if err != nil {
//	    return err
//	}
//	for chunk := range chunks {
//	    if chunk.Error != nil {
//	        return chunk.Error
//	    }
//	    var resp pluginv1.ProviderChatStreamChunk
//	    proto.Unmarshal(chunk.Payload, &resp)
//	    // Process chunk
//	}
func (c *PluginsClient) InvokeStream(ctx context.Context, capability, method string, req proto.Message, opts ...InvokeOption) (<-chan *StreamChunk, error) {
	options := &invokeOptions{}
	for _, opt := range opts {
		opt(options)
	}

	payload, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	stream, err := c.client.InvokeCapabilityStream(ctx, &pluginv1.CapabilityInvokeRequest{
		Capability:      capability,
		Method:          method,
		RequestPayload:  payload,
		PreferredPlugin: options.preferredPlugin,
		ApiVersion:      options.apiVersion,
		Metadata:        options.metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("invoke stream failed: %w", err)
	}

	ch := make(chan *StreamChunk)
	go func() {
		defer close(ch)
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				ch <- &StreamChunk{Error: err}
				return
			}
			if resp.Error != nil {
				ch <- &StreamChunk{Error: &CapabilityError{
					Code:      resp.Error.Code,
					Message:   resp.Error.Message,
					Details:   resp.Error.Details,
					Retryable: resp.Error.Retryable,
				}}
				return
			}
			ch <- &StreamChunk{
				Payload:        resp.ResponsePayload,
				ProviderPlugin: resp.ProviderPlugin,
				Metadata:       resp.Metadata,
			}
		}
	}()

	return ch, nil
}

// Describe returns metadata about a capability's available methods.
//
// Example:
//
//	desc, err := plugins.Describe(ctx, "embedding")
//	for _, method := range desc.Methods {
//	    fmt.Printf("Method: %s (streaming: %v)\n", method.Name, method.IsStreaming)
//	}
func (c *PluginsClient) Describe(ctx context.Context, capability string) (*CapabilityDescription, error) {
	resp, err := c.client.DescribeCapability(ctx, &pluginv1.DescribeCapabilityRequest{
		Capability: capability,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe capability: %w", err)
	}

	methods := make([]CapabilityMethod, len(resp.Methods))
	for i, m := range resp.Methods {
		methods[i] = CapabilityMethod{
			Name:         m.Name,
			Description:  m.Description,
			IsStreaming:  m.IsStreaming,
			RequestType:  m.RequestType,
			ResponseType: m.ResponseType,
		}
	}

	providers := make([]PluginProvider, len(resp.Providers))
	for i, p := range resp.Providers {
		providers[i] = PluginProvider{
			ID:          p.PluginId,
			Name:        p.PluginName,
			Enabled:     p.Enabled,
			Configured:  p.Configured,
			IsPreferred: p.IsPreferred,
		}
	}

	return &CapabilityDescription{
		Capability:        resp.Capability,
		APIVersion:        resp.ApiVersion,
		SupportedVersions: resp.SupportedVersions,
		Methods:           methods,
		Providers:         providers,
	}, nil
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

// SetCapabilityPreference sets the preferred plugin for a capability.
// Used by configuration plugins (e.g., ai-local) to route capabilities to specific providers.
// When other plugins invoke this capability, the preferred plugin will be used.
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
// After clearing, Invoke falls back to the first available provider.
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

// --- Types ---

// InvokeOption configures an Invoke call.
type InvokeOption func(*invokeOptions)

type invokeOptions struct {
	preferredPlugin string
	apiVersion      string
	metadata        map[string]string
}

// WithPreferredPlugin sets the preferred provider plugin for this invoke.
func WithPreferredPlugin(pluginID string) InvokeOption {
	return func(o *invokeOptions) { o.preferredPlugin = pluginID }
}

// WithAPIVersion sets the API version for this invoke.
func WithAPIVersion(version string) InvokeOption {
	return func(o *invokeOptions) { o.apiVersion = version }
}

// WithMetadata sets request metadata for this invoke.
func WithMetadata(meta map[string]string) InvokeOption {
	return func(o *invokeOptions) { o.metadata = meta }
}

// InvokeMetadata contains metadata about an invoke call.
type InvokeMetadata struct {
	ProviderPlugin string            // Which plugin handled the request
	LatencyMs      int64             // How long the provider took
	Metadata       map[string]string // Response metadata from provider
}

// StreamChunk represents a chunk from a streaming response.
type StreamChunk struct {
	Payload        []byte            // Serialized response proto
	ProviderPlugin string            // Which plugin is streaming
	Metadata       map[string]string // Chunk metadata
	Error          error             // Error if chunk failed
}

// CapabilityError represents a structured error from a capability call.
type CapabilityError struct {
	Code      pluginv1.CapabilityErrorCode
	Message   string
	Details   string
	Retryable bool
}

func (e *CapabilityError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// CapabilityDescription describes a capability and its available methods.
type CapabilityDescription struct {
	Capability        string
	APIVersion        string
	SupportedVersions []string
	Methods           []CapabilityMethod
	Providers         []PluginProvider
}

// CapabilityMethod describes a method available on a capability.
type CapabilityMethod struct {
	Name         string
	Description  string
	IsStreaming  bool
	RequestType  string
	ResponseType string
}

// PluginProvider describes a plugin that provides a capability.
type PluginProvider struct {
	ID          string // Plugin ID (e.g., "provider-ollama")
	Name        string // Human-readable name (e.g., "Ollama Provider")
	Enabled     bool   // Whether the plugin is enabled
	Configured  bool   // Whether the plugin is properly configured
	IsPreferred bool   // Whether this is the preferred provider
}

// Capability describes an available capability and its providers.
type Capability struct {
	Name      string           // Capability name (e.g., "embedding")
	Providers []PluginProvider // Plugins providing this capability
}
