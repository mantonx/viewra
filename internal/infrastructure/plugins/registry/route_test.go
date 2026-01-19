package registry

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

func TestRouteRegistry_GetAllRoutes(t *testing.T) {
	registry := NewRouteRegistry()

	registry.RegisterRoutes("plugin-a", []*pluginv1.PluginRoute{
		{Path: "/route-a1", Methods: []string{"GET"}},
		{Path: "/route-a2", Methods: []string{"POST"}},
	})
	registry.RegisterRoutes("plugin-b", []*pluginv1.PluginRoute{
		{Path: "/route-b1", Methods: []string{"GET"}},
	})

	all := registry.GetAllRoutes()
	if len(all) != 3 {
		t.Errorf("GetAllRoutes() returned %d routes, want 3", len(all))
	}
}

func TestRouteRegistry_FindRouteByCapability(t *testing.T) {
	registry := NewRouteRegistry()

	registry.RegisterRoutes("search-plugin", []*pluginv1.PluginRoute{
		{
			Path:       "/search",
			Methods:    []string{"GET"},
			Capability: "search",
		},
	})

	route, found := registry.FindRouteByCapability("search")
	if !found {
		t.Fatal("FindRouteByCapability() should find the route")
	}
	if route.Capability != "search" {
		t.Errorf("route.Capability = %s, want search", route.Capability)
	}

	// Non-existent capability
	_, found = registry.FindRouteByCapability("non_existent")
	if found {
		t.Error("FindRouteByCapability() should not find non-existent capability")
	}
}

func TestRegisteredRoute_Match(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		path       string
		wantMatch  bool
		wantParams map[string]string
	}{
		{
			name:      "exact match",
			pattern:   "/search",
			path:      "/search",
			wantMatch: true,
		},
		{
			name:      "no match",
			pattern:   "/search",
			path:      "/other",
			wantMatch: false,
		},
		{
			name:       "single param",
			pattern:    "/items/:id",
			path:       "/items/123",
			wantMatch:  true,
			wantParams: map[string]string{"id": "123"},
		},
		{
			name:       "multiple params",
			pattern:    "/users/:userId/posts/:postId",
			path:       "/users/42/posts/99",
			wantMatch:  true,
			wantParams: map[string]string{"userId": "42", "postId": "99"},
		},
		{
			name:      "partial match fails",
			pattern:   "/items/:id/details",
			path:      "/items/123",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regex, paramNames := compilePathPattern(tt.pattern)
			route := &RegisteredRoute{
				pathRegex:  regex,
				paramNames: paramNames,
			}

			params, matched := route.Match(tt.path)
			if matched != tt.wantMatch {
				t.Errorf("Match() = %v, want %v", matched, tt.wantMatch)
			}

			if tt.wantParams != nil && matched {
				for key, want := range tt.wantParams {
					if got := params[key]; got != want {
						t.Errorf("params[%s] = %s, want %s", key, got, want)
					}
				}
			}
		})
	}
}

func TestRegisteredRoute_HasMethod(t *testing.T) {
	route := &RegisteredRoute{
		Methods: []string{"GET", "POST"},
	}

	tests := []struct {
		method string
		want   bool
	}{
		{"GET", true},
		{"get", true},
		{"POST", true},
		{"post", true},
		{"DELETE", false},
		{"PUT", false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			if got := route.HasMethod(tt.method); got != tt.want {
				t.Errorf("HasMethod(%s) = %v, want %v", tt.method, got, tt.want)
			}
		})
	}
}
