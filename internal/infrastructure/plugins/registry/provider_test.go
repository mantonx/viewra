package registry

import (
	"context"
	"testing"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"google.golang.org/grpc"
)

// mockProviderClient implements PluginProviderClient for testing.
type mockProviderClient struct {
	pluginv1.PluginProviderClient
	capabilities *pluginv1.ProviderCapabilities
	err          error
}

func (m *mockProviderClient) GetCapabilities(ctx context.Context, in *pluginv1.Empty, opts ...grpc.CallOption) (*pluginv1.ProviderCapabilities, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.capabilities, nil
}

// mockCoreClient implements PluginCoreClient for testing.
type mockCoreClient struct {
	pluginv1.PluginCoreClient
}

func TestProviderRegistry_RegisterAndGet(t *testing.T) {
	registry := NewProviderRegistry()
	ctx := context.Background()

	// Create a mock client
	client := &mockProviderClient{
		capabilities: &pluginv1.ProviderCapabilities{
			ProviderId:         "ollama",
			DisplayName:        "Ollama (Local)",
			Description:        "Local AI inference",
			SupportsChat:       true,
			SupportsEmbeddings: true,
			SupportsStreaming:  true,
			RequiresApiKey:     false,
			RequiresUrl:        true,
			IsLocal:            true,
		},
	}

	// Register the provider
	coreClient := &mockCoreClient{}
	caps, err := registry.Register(ctx, "provider-ollama", client, coreClient)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if caps.ProviderId != "ollama" {
		t.Errorf("Register() returned caps.ProviderId = %v, want ollama", caps.ProviderId)
	}

	// Get the provider
	provider := registry.Get("ollama")
	if provider == nil {
		t.Fatal("Get() returned nil, want provider")
	}

	if provider.PluginID != "provider-ollama" {
		t.Errorf("provider.PluginID = %v, want provider-ollama", provider.PluginID)
	}

	if provider.ProviderID != "ollama" {
		t.Errorf("provider.ProviderID = %v, want ollama", provider.ProviderID)
	}

	if !provider.Capabilities.SupportsChat {
		t.Error("provider.Capabilities.SupportsChat = false, want true")
	}
}

func TestProviderRegistry_List(t *testing.T) {
	registry := NewProviderRegistry()
	ctx := context.Background()

	// Register multiple providers
	providers := []struct {
		pluginID   string
		providerID string
	}{
		{"provider-ollama", "ollama"},
		{"provider-openai", "openai"},
		{"provider-anthropic", "anthropic"},
	}

	for _, p := range providers {
		client := &mockProviderClient{
			capabilities: &pluginv1.ProviderCapabilities{
				ProviderId:  p.providerID,
				DisplayName: p.providerID,
			},
		}
		coreClient := &mockCoreClient{}
		_, err := registry.Register(ctx, p.pluginID, client, coreClient)
		if err != nil {
			t.Fatalf("Register(%s) error = %v", p.pluginID, err)
		}
	}

	// List all providers
	list := registry.List()
	if len(list) != 3 {
		t.Errorf("List() returned %d providers, want 3", len(list))
	}

	// Check all providers are present
	found := make(map[string]bool)
	for _, p := range list {
		found[p.ProviderID] = true
	}

	for _, p := range providers {
		if !found[p.providerID] {
			t.Errorf("List() missing provider %s", p.providerID)
		}
	}
}

func TestProviderRegistry_Unregister(t *testing.T) {
	registry := NewProviderRegistry()
	ctx := context.Background()

	// Register a provider
	client := &mockProviderClient{
		capabilities: &pluginv1.ProviderCapabilities{
			ProviderId:  "ollama",
			DisplayName: "Ollama",
		},
	}
	coreClient := &mockCoreClient{}
	_, err := registry.Register(ctx, "provider-ollama", client, coreClient)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Verify it exists
	if registry.Get("ollama") == nil {
		t.Fatal("Get(ollama) returned nil after register")
	}

	// Unregister
	registry.Unregister("provider-ollama")

	// Verify it's gone
	if registry.Get("ollama") != nil {
		t.Error("Get(ollama) returned non-nil after unregister")
	}

	// List should be empty
	if len(registry.List()) != 0 {
		t.Errorf("List() returned %d providers, want 0", len(registry.List()))
	}
}

func TestProviderRegistry_DuplicateRegistration(t *testing.T) {
	registry := NewProviderRegistry()
	ctx := context.Background()

	client := &mockProviderClient{
		capabilities: &pluginv1.ProviderCapabilities{
			ProviderId:  "ollama",
			DisplayName: "Ollama",
		},
	}
	coreClient := &mockCoreClient{}

	// First registration should succeed
	_, err := registry.Register(ctx, "provider-ollama-1", client, coreClient)
	if err != nil {
		t.Fatalf("First Register() error = %v", err)
	}

	// Second registration with same provider ID should fail
	_, err = registry.Register(ctx, "provider-ollama-2", client, coreClient)
	if err == nil {
		t.Error("Second Register() should have failed for duplicate provider ID")
	}
}

func TestProviderRegistry_EmptyProviderID(t *testing.T) {
	registry := NewProviderRegistry()
	ctx := context.Background()

	client := &mockProviderClient{
		capabilities: &pluginv1.ProviderCapabilities{
			ProviderId:  "", // Empty provider ID
			DisplayName: "Test",
		},
	}
	coreClient := &mockCoreClient{}

	_, err := registry.Register(ctx, "test-plugin", client, coreClient)
	if err == nil {
		t.Error("Register() should fail for empty provider ID")
	}
}

func TestProviderRegistry_GetNonExistent(t *testing.T) {
	registry := NewProviderRegistry()

	provider := registry.Get("non-existent")
	if provider != nil {
		t.Error("Get(non-existent) should return nil")
	}
}

func TestProviderRegistry_GetByPluginID(t *testing.T) {
	registry := NewProviderRegistry()
	ctx := context.Background()

	client := &mockProviderClient{
		capabilities: &pluginv1.ProviderCapabilities{
			ProviderId:  "ollama",
			DisplayName: "Ollama",
		},
	}
	coreClient := &mockCoreClient{}
	_, err := registry.Register(ctx, "provider-ollama", client, coreClient)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Get provider ID by plugin ID
	providerID := registry.GetByPluginID("provider-ollama")
	if providerID != "ollama" {
		t.Errorf("GetByPluginID() = %s, want ollama", providerID)
	}

	// Non-existent plugin
	providerID = registry.GetByPluginID("non-existent")
	if providerID != "" {
		t.Errorf("GetByPluginID(non-existent) = %s, want empty string", providerID)
	}
}
