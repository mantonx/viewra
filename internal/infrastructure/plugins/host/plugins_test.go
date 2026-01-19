package host

import (
	"context"
	"log/slog"
	"os"
	"testing"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/types"
)

// mockPluginLookup is a mock implementation of PluginLookup for testing.
type mockPluginLookup struct {
	plugins        map[string]*types.Instance
	enabledPlugins map[string]bool
}

func newMockPluginLookup() *mockPluginLookup {
	return &mockPluginLookup{
		plugins:        make(map[string]*types.Instance),
		enabledPlugins: make(map[string]bool),
	}
}

func (m *mockPluginLookup) GetPlugin(id string) (*types.Instance, bool) {
	p, ok := m.plugins[id]
	return p, ok
}

func (m *mockPluginLookup) IsPluginEnabled(id string) bool {
	return m.enabledPlugins[id]
}

func (m *mockPluginLookup) AddPlugin(id string, instance *types.Instance, enabled bool) {
	m.plugins[id] = instance
	m.enabledPlugins[id] = enabled
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNewPluginsServer(t *testing.T) {
	lookup := newMockPluginLookup()
	logger := testLogger()

	server := NewPluginsServer(lookup, logger)

	if server == nil {
		t.Fatal("expected non-nil server")
	}
	if server.capabilities == nil {
		t.Error("expected capabilities map to be initialized")
	}
	if server.preferences == nil {
		t.Error("expected preferences map to be initialized")
	}
	if server.pluginLookup != lookup {
		t.Error("expected pluginLookup to be set")
	}
}

func TestRegisterCapability(t *testing.T) {
	lookup := newMockPluginLookup()
	server := NewPluginsServer(lookup, testLogger())

	// Register a capability
	server.RegisterCapability("plugin1", "Plugin One", "embedding")

	// Verify registration
	server.mu.RLock()
	providers := server.capabilities["embedding"]
	server.mu.RUnlock()

	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}
	if providers[0].PluginID != "plugin1" {
		t.Errorf("expected plugin ID 'plugin1', got '%s'", providers[0].PluginID)
	}
	if providers[0].PluginName != "Plugin One" {
		t.Errorf("expected plugin name 'Plugin One', got '%s'", providers[0].PluginName)
	}
	if !providers[0].Enabled {
		t.Error("expected provider to be enabled")
	}
	if providers[0].Configured {
		t.Error("expected provider to NOT be configured by default (must be set via Configure)")
	}
}

func TestRegisterCapability_UpdateExisting(t *testing.T) {
	lookup := newMockPluginLookup()
	server := NewPluginsServer(lookup, testLogger())

	// Register a capability
	server.RegisterCapability("plugin1", "Plugin One", "embedding")

	// Register same plugin with different name
	server.RegisterCapability("plugin1", "Plugin One Updated", "embedding")

	// Verify only one provider exists with updated name
	server.mu.RLock()
	providers := server.capabilities["embedding"]
	server.mu.RUnlock()

	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}
	if providers[0].PluginName != "Plugin One Updated" {
		t.Errorf("expected plugin name 'Plugin One Updated', got '%s'", providers[0].PluginName)
	}
}

func TestRegisterCapability_MultipleProviders(t *testing.T) {
	lookup := newMockPluginLookup()
	server := NewPluginsServer(lookup, testLogger())

	// Register multiple providers for same capability
	server.RegisterCapability("plugin1", "Plugin One", "embedding")
	server.RegisterCapability("plugin2", "Plugin Two", "embedding")

	// Verify both registered
	server.mu.RLock()
	providers := server.capabilities["embedding"]
	server.mu.RUnlock()

	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}
}

func TestUnregisterPlugin(t *testing.T) {
	lookup := newMockPluginLookup()
	server := NewPluginsServer(lookup, testLogger())

	// Register capabilities
	server.RegisterCapability("plugin1", "Plugin One", "embedding")
	server.RegisterCapability("plugin1", "Plugin One", "chat")
	server.RegisterCapability("plugin2", "Plugin Two", "embedding")

	// Unregister plugin1
	server.UnregisterPlugin("plugin1")

	// Verify plugin1 capabilities removed
	server.mu.RLock()
	embeddingProviders := server.capabilities["embedding"]
	_, chatExists := server.capabilities["chat"]
	server.mu.RUnlock()

	if len(embeddingProviders) != 1 {
		t.Errorf("expected 1 embedding provider, got %d", len(embeddingProviders))
	}
	if embeddingProviders[0].PluginID != "plugin2" {
		t.Errorf("expected plugin2 to remain, got '%s'", embeddingProviders[0].PluginID)
	}
	if chatExists {
		t.Error("expected chat capability to be removed (no providers left)")
	}
}

func TestUpdatePluginStatus(t *testing.T) {
	lookup := newMockPluginLookup()
	server := NewPluginsServer(lookup, testLogger())

	// Register a capability
	server.RegisterCapability("plugin1", "Plugin One", "embedding")

	// Update status
	server.UpdatePluginStatus("plugin1", false, false)

	// Verify status updated
	server.mu.RLock()
	providers := server.capabilities["embedding"]
	server.mu.RUnlock()

	if providers[0].Enabled {
		t.Error("expected provider to be disabled")
	}
	if providers[0].Configured {
		t.Error("expected provider to be unconfigured")
	}
}

func TestHasCapability(t *testing.T) {
	lookup := newMockPluginLookup()
	lookup.AddPlugin("plugin1", &types.Instance{ID: "plugin1"}, true)
	server := NewPluginsServer(lookup, testLogger())

	// No capabilities registered
	if server.HasCapability("embedding") {
		t.Error("expected HasCapability to return false when no providers")
	}

	// Register a capability with enabled plugin
	server.RegisterCapability("plugin1", "Plugin One", "embedding")

	// Now should have capability (provider is Enabled=true by default when registered)
	if !server.HasCapability("embedding") {
		t.Error("expected HasCapability to return true when enabled provider exists")
	}

	// Disable the provider via UpdatePluginStatus
	server.UpdatePluginStatus("plugin1", false, true)

	// Should not have capability when provider disabled
	if server.HasCapability("embedding") {
		t.Error("expected HasCapability to return false when provider disabled")
	}
}

func TestListCapabilities(t *testing.T) {
	lookup := newMockPluginLookup()
	lookup.AddPlugin("plugin1", &types.Instance{ID: "plugin1"}, true)
	lookup.AddPlugin("plugin2", &types.Instance{ID: "plugin2"}, true)
	server := NewPluginsServer(lookup, testLogger())

	// Register capabilities
	server.RegisterCapability("plugin1", "Plugin One", "embedding")
	server.RegisterCapability("plugin2", "Plugin Two", "embedding")
	server.RegisterCapability("plugin1", "Plugin One", "chat")

	// List capabilities
	resp, err := server.ListCapabilities(context.Background(), &pluginv1.Empty{})
	if err != nil {
		t.Fatalf("ListCapabilities failed: %v", err)
	}

	if len(resp.Capabilities) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(resp.Capabilities))
	}

	// Find embedding capability
	var embeddingCap *pluginv1.CapabilityInfo
	for _, cap := range resp.Capabilities {
		if cap.Name == "embedding" {
			embeddingCap = cap
			break
		}
	}

	if embeddingCap == nil {
		t.Fatal("expected to find embedding capability")
	}
	if len(embeddingCap.Providers) != 2 {
		t.Errorf("expected 2 embedding providers, got %d", len(embeddingCap.Providers))
	}
}

func TestListProviders(t *testing.T) {
	lookup := newMockPluginLookup()
	lookup.AddPlugin("plugin1", &types.Instance{ID: "plugin1"}, true)
	lookup.AddPlugin("plugin2", &types.Instance{ID: "plugin2"}, false)
	server := NewPluginsServer(lookup, testLogger())

	// Register capabilities
	server.RegisterCapability("plugin1", "Plugin One", "embedding")
	server.RegisterCapability("plugin2", "Plugin Two", "embedding")

	// Disable plugin2 via UpdatePluginStatus (RegisterCapability sets Enabled=true by default)
	server.UpdatePluginStatus("plugin2", false, true)

	// List providers for embedding
	resp, err := server.ListProviders(context.Background(), &pluginv1.CapabilityRequest{
		Capability: "embedding",
	})
	if err != nil {
		t.Fatalf("ListProviders failed: %v", err)
	}

	if len(resp.Providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(resp.Providers))
	}

	// Check enabled status (Enabled field indicates if plugin is enabled)
	for _, p := range resp.Providers {
		if p.PluginId == "plugin1" && !p.Enabled {
			t.Error("expected plugin1 to be enabled")
		}
		if p.PluginId == "plugin2" && p.Enabled {
			t.Error("expected plugin2 to be disabled")
		}
	}
}

func TestSetCapabilityPreference(t *testing.T) {
	lookup := newMockPluginLookup()
	server := NewPluginsServer(lookup, testLogger())

	// Register capability
	server.RegisterCapability("plugin1", "Plugin One", "embedding")

	// Set preference
	_, err := server.SetCapabilityPreference(context.Background(), &pluginv1.CapabilityPreferenceRequest{
		Capability: "embedding",
		PluginId:   "plugin1",
	})
	if err != nil {
		t.Fatalf("SetCapabilityPreference failed: %v", err)
	}

	// Verify preference set
	server.mu.RLock()
	pref := server.preferences["embedding"]
	server.mu.RUnlock()

	if pref != "plugin1" {
		t.Errorf("expected preference 'plugin1', got '%s'", pref)
	}
}

func TestSetCapabilityPreference_MissingFields(t *testing.T) {
	lookup := newMockPluginLookup()
	server := NewPluginsServer(lookup, testLogger())

	// Try to set preference with empty capability
	_, err := server.SetCapabilityPreference(context.Background(), &pluginv1.CapabilityPreferenceRequest{
		Capability: "",
		PluginId:   "plugin1",
	})
	if err == nil {
		t.Error("expected error for empty capability")
	}

	// Try to set preference with empty plugin_id
	_, err = server.SetCapabilityPreference(context.Background(), &pluginv1.CapabilityPreferenceRequest{
		Capability: "embedding",
		PluginId:   "",
	})
	if err == nil {
		t.Error("expected error for empty plugin_id")
	}
}

func TestClearCapabilityPreference(t *testing.T) {
	lookup := newMockPluginLookup()
	server := NewPluginsServer(lookup, testLogger())

	// Register and set preference
	server.RegisterCapability("plugin1", "Plugin One", "embedding")
	server.SetCapabilityPreference(context.Background(), &pluginv1.CapabilityPreferenceRequest{
		Capability: "embedding",
		PluginId:   "plugin1",
	})

	// Clear preference
	_, err := server.ClearCapabilityPreference(context.Background(), &pluginv1.CapabilityPreferenceRequest{
		Capability: "embedding",
	})
	if err != nil {
		t.Fatalf("ClearCapabilityPreference failed: %v", err)
	}

	// Verify preference cleared
	server.mu.RLock()
	_, exists := server.preferences["embedding"]
	server.mu.RUnlock()

	if exists {
		t.Error("expected preference to be cleared")
	}
}

func TestGetCapabilityPreferences(t *testing.T) {
	lookup := newMockPluginLookup()
	server := NewPluginsServer(lookup, testLogger())

	// Register and set preferences
	server.RegisterCapability("plugin1", "Plugin One", "embedding")
	server.RegisterCapability("plugin2", "Plugin Two", "chat")
	server.SetCapabilityPreference(context.Background(), &pluginv1.CapabilityPreferenceRequest{
		Capability: "embedding",
		PluginId:   "plugin1",
	})
	server.SetCapabilityPreference(context.Background(), &pluginv1.CapabilityPreferenceRequest{
		Capability: "chat",
		PluginId:   "plugin2",
	})

	// Get preferences
	resp, err := server.GetCapabilityPreferences(context.Background(), &pluginv1.Empty{})
	if err != nil {
		t.Fatalf("GetCapabilityPreferences failed: %v", err)
	}

	if len(resp.Preferences) != 2 {
		t.Errorf("expected 2 preferences, got %d", len(resp.Preferences))
	}
	if resp.Preferences["embedding"] != "plugin1" {
		t.Errorf("expected embedding preference 'plugin1', got '%s'", resp.Preferences["embedding"])
	}
	if resp.Preferences["chat"] != "plugin2" {
		t.Errorf("expected chat preference 'plugin2', got '%s'", resp.Preferences["chat"])
	}
}

// mockProviderClient is a minimal mock for ProviderClient
type mockProviderClient struct {
	pluginv1.PluginProviderClient
}

func TestResolveProvider(t *testing.T) {
	lookup := newMockPluginLookup()
	// Need to set ProviderClient for resolveProvider to succeed
	lookup.AddPlugin("plugin1", &types.Instance{
		ID:             "plugin1",
		ProviderClient: &mockProviderClient{},
	}, true)
	lookup.AddPlugin("plugin2", &types.Instance{
		ID:             "plugin2",
		ProviderClient: &mockProviderClient{},
	}, true)
	server := NewPluginsServer(lookup, testLogger())

	// Register capabilities
	server.RegisterCapability("plugin1", "Plugin One", "embedding")
	server.RegisterCapability("plugin2", "Plugin Two", "embedding")

	// Mark plugins as configured (default is false after registration)
	server.UpdatePluginStatus("plugin1", true, true)
	server.UpdatePluginStatus("plugin2", true, true)

	// Test: resolve with no preference (should return first available)
	provider, instance, err := server.resolveProvider("embedding", "")
	if err != nil {
		t.Fatalf("resolveProvider failed: %v", err)
	}
	if provider == nil || instance == nil {
		t.Fatal("expected non-nil provider and instance")
	}

	// Test: resolve with explicit preferred plugin
	provider, instance, err = server.resolveProvider("embedding", "plugin2")
	if err != nil {
		t.Fatalf("resolveProvider with preference failed: %v", err)
	}
	if provider.PluginID != "plugin2" {
		t.Errorf("expected plugin2, got %s", provider.PluginID)
	}

	// Test: resolve with configured preference
	server.SetCapabilityPreference(context.Background(), &pluginv1.CapabilityPreferenceRequest{
		Capability: "embedding",
		PluginId:   "plugin2",
	})
	provider, _, err = server.resolveProvider("embedding", "")
	if err != nil {
		t.Fatalf("resolveProvider with configured preference failed: %v", err)
	}
	if provider.PluginID != "plugin2" {
		t.Errorf("expected plugin2 from preference, got %s", provider.PluginID)
	}
}

func TestResolveProvider_NoCapability(t *testing.T) {
	lookup := newMockPluginLookup()
	server := NewPluginsServer(lookup, testLogger())

	_, _, err := server.resolveProvider("nonexistent", "")
	if err == nil {
		t.Error("expected error for non-existent capability")
	}
}

func TestResolveProvider_NoAvailableProvider(t *testing.T) {
	lookup := newMockPluginLookup()
	lookup.AddPlugin("plugin1", &types.Instance{ID: "plugin1"}, false) // disabled
	server := NewPluginsServer(lookup, testLogger())

	server.RegisterCapability("plugin1", "Plugin One", "embedding")

	_, _, err := server.resolveProvider("embedding", "")
	if err == nil {
		t.Error("expected error when no available provider")
	}
}

func TestMapProviderErrorCode(t *testing.T) {
	tests := []struct {
		code     string
		expected pluginv1.CapabilityErrorCode
	}{
		{"METHOD_NOT_FOUND", pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_METHOD_NOT_FOUND},
		{"INVALID_REQUEST", pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_INVALID_REQUEST},
		{"PROVIDER_ERROR", pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_PROVIDER_ERROR},
		{"TIMEOUT", pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_TIMEOUT},
		{"RATE_LIMITED", pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_RATE_LIMITED},
		{"NOT_CONFIGURED", pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_NOT_CONFIGURED},
		{"UNKNOWN", pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_PROVIDER_ERROR},
		{"", pluginv1.CapabilityErrorCode_CAPABILITY_ERROR_PROVIDER_ERROR},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			result := mapProviderErrorCode(tt.code)
			if result != tt.expected {
				t.Errorf("mapProviderErrorCode(%q) = %v, want %v", tt.code, result, tt.expected)
			}
		})
	}
}
