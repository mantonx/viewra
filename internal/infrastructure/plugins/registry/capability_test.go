package registry

import (
	"testing"
)

func TestCapabilityRegistry_RegisterAndResolve(t *testing.T) {
	registry := NewCapabilityRegistry()

	// Register a capability
	ok := registry.Register("plugin-a", "semantic_search", "/search")
	if !ok {
		t.Error("first registration should succeed")
	}

	// Try to register same capability from another plugin
	ok = registry.Register("plugin-b", "semantic_search", "/my-search")
	if ok {
		t.Error("duplicate capability registration should fail")
	}

	// Same plugin can re-register
	ok = registry.Register("plugin-a", "semantic_search", "/new-search")
	if !ok {
		t.Error("same plugin should be able to update capability")
	}

	// Resolve the capability
	mapping := registry.Resolve("semantic_search")
	if mapping == nil {
		t.Fatal("capability should be resolvable")
	}
	if mapping.PluginID != "plugin-a" {
		t.Errorf("PluginID = %s, want plugin-a", mapping.PluginID)
	}
	if mapping.PluginPath != "/new-search" {
		t.Errorf("PluginPath = %s, want /new-search", mapping.PluginPath)
	}

	// Unregister and verify
	registry.Unregister("plugin-a")
	if registry.Resolve("semantic_search") != nil {
		t.Error("capability should be nil after unregister")
	}
}

func TestCapabilityRegistry_GetCapabilitiesForPlugin(t *testing.T) {
	registry := NewCapabilityRegistry()

	// Register multiple capabilities for a plugin
	registry.Register("plugin-a", "semantic_search", "/search")
	registry.Register("plugin-a", "similar_items", "/similar")
	registry.Register("plugin-b", "chat", "/chat")

	caps := registry.GetCapabilitiesForPlugin("plugin-a")
	if len(caps) != 2 {
		t.Errorf("GetCapabilitiesForPlugin() returned %d capabilities, want 2", len(caps))
	}

	// Check both capabilities are present
	found := make(map[string]bool)
	for _, cap := range caps {
		found[cap] = true
	}
	if !found["semantic_search"] {
		t.Error("missing semantic_search capability")
	}
	if !found["similar_items"] {
		t.Error("missing similar_items capability")
	}
}

func TestCapabilityRegistry_ListAll(t *testing.T) {
	registry := NewCapabilityRegistry()

	registry.Register("plugin-a", "semantic_search", "/search")
	registry.Register("plugin-b", "chat", "/chat")

	all := registry.ListAll()
	if len(all) != 2 {
		t.Errorf("ListAll() returned %d mappings, want 2", len(all))
	}

	if _, ok := all["semantic_search"]; !ok {
		t.Error("ListAll() missing semantic_search")
	}
	if _, ok := all["chat"]; !ok {
		t.Error("ListAll() missing chat")
	}
}

func TestCapabilityRegistry_ResolveNonExistent(t *testing.T) {
	registry := NewCapabilityRegistry()

	mapping := registry.Resolve("non_existent")
	if mapping != nil {
		t.Error("Resolve() should return nil for non-existent capability")
	}
}

func TestCapabilityAliases(t *testing.T) {
	// Verify the well-known capability aliases are defined
	expectedAliases := map[string]string{
		"semantic_search": "/api/search",
		"similar_items":   "/api/similar",
		"recommendations": "/api/recommendations",
		"chat":            "/api/chat",
	}

	for cap, path := range expectedAliases {
		if CapabilityAliases[cap] != path {
			t.Errorf("CapabilityAliases[%s] = %s, want %s", cap, CapabilityAliases[cap], path)
		}
	}
}
