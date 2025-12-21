package plugins

import (
	"testing"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

func TestRouteRegistry_RegisterAndFind(t *testing.T) {
	registry := NewRouteRegistry()

	// Register routes for a plugin
	registry.RegisterRoutes("test-plugin", []*pluginv1.PluginRoute{
		{
			Path:        "/search",
			Methods:     []string{"GET", "POST"},
			Description: "Search endpoint",
		},
		{
			Path:        "/items/:id",
			Methods:     []string{"GET"},
			Description: "Get item by ID",
		},
		{
			Path:        "/items/:id/similar",
			Methods:     []string{"GET"},
			Description: "Get similar items",
		},
	})

	tests := []struct {
		name       string
		pluginID   string
		path       string
		method     string
		wantFound  bool
		wantParams map[string]string
	}{
		{
			name:      "exact match GET",
			pluginID:  "test-plugin",
			path:      "/search",
			method:    "GET",
			wantFound: true,
		},
		{
			name:      "exact match POST",
			pluginID:  "test-plugin",
			path:      "/search",
			method:    "POST",
			wantFound: true,
		},
		{
			name:      "method not allowed",
			pluginID:  "test-plugin",
			path:      "/search",
			method:    "DELETE",
			wantFound: false,
		},
		{
			name:       "path param extraction",
			pluginID:   "test-plugin",
			path:       "/items/123",
			method:     "GET",
			wantFound:  true,
			wantParams: map[string]string{"id": "123"},
		},
		{
			name:       "nested path with param",
			pluginID:   "test-plugin",
			path:       "/items/456/similar",
			method:     "GET",
			wantFound:  true,
			wantParams: map[string]string{"id": "456"},
		},
		{
			name:      "unknown plugin",
			pluginID:  "unknown",
			path:      "/search",
			method:    "GET",
			wantFound: false,
		},
		{
			name:      "unknown path",
			pluginID:  "test-plugin",
			path:      "/unknown",
			method:    "GET",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route, params, found := registry.FindRoute(tt.pluginID, tt.path, tt.method)

			if found != tt.wantFound {
				t.Errorf("FindRoute() found = %v, want %v", found, tt.wantFound)
			}

			if tt.wantParams != nil && found {
				for key, wantVal := range tt.wantParams {
					if gotVal, ok := params[key]; !ok || gotVal != wantVal {
						t.Errorf("FindRoute() param[%s] = %v, want %v", key, gotVal, wantVal)
					}
				}
			}

			if found && route == nil {
				t.Error("FindRoute() returned found=true but route is nil")
			}
		})
	}
}

func TestRouteRegistry_Unregister(t *testing.T) {
	registry := NewRouteRegistry()

	registry.RegisterRoutes("plugin-a", []*pluginv1.PluginRoute{
		{Path: "/route-a", Methods: []string{"GET"}},
	})
	registry.RegisterRoutes("plugin-b", []*pluginv1.PluginRoute{
		{Path: "/route-b", Methods: []string{"GET"}},
	})

	// Both should be found
	_, _, foundA := registry.FindRoute("plugin-a", "/route-a", "GET")
	_, _, foundB := registry.FindRoute("plugin-b", "/route-b", "GET")
	if !foundA || !foundB {
		t.Fatal("routes should be found after registration")
	}

	// Unregister plugin-a
	registry.UnregisterRoutes("plugin-a")

	// plugin-a route should not be found, plugin-b should still work
	_, _, foundA = registry.FindRoute("plugin-a", "/route-a", "GET")
	_, _, foundB = registry.FindRoute("plugin-b", "/route-b", "GET")
	if foundA {
		t.Error("plugin-a route should not be found after unregister")
	}
	if !foundB {
		t.Error("plugin-b route should still be found")
	}
}

func TestCapabilityRegistry(t *testing.T) {
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
