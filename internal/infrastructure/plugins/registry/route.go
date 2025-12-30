// Package registry provides route, provider, and capability registries for plugins.
package registry

import (
	"regexp"
	"strings"
	"sync"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

// RegisteredRoute represents a route registered by a plugin.
type RegisteredRoute struct {
	PluginID    string
	Path        string   // Original path pattern (e.g., "/items/:id")
	FullPath    string   // Full path with plugin prefix (e.g., "/api/plugins/semantic-search/items/:id")
	Methods     []string // HTTP methods
	AdminOnly   bool
	Description string
	Capability  string // Capability name this route provides (e.g., "semantic_search")
	AliasPath   string // Stable URL alias (e.g., "/api/search"), empty if no alias
	Streaming   bool
	RateLimit   *pluginv1.PluginRateLimit

	// Compiled pattern for matching
	pathRegex  *regexp.Regexp
	paramNames []string
}

// Match checks if a request path matches this route and extracts path parameters.
// Returns the extracted params and true if matched, nil and false otherwise.
func (r *RegisteredRoute) Match(path string) (map[string]string, bool) {
	if r.pathRegex == nil {
		return nil, false
	}

	matches := r.pathRegex.FindStringSubmatch(path)
	if matches == nil {
		return nil, false
	}

	params := make(map[string]string)
	for i, name := range r.paramNames {
		if i+1 < len(matches) {
			params[name] = matches[i+1]
		}
	}
	return params, true
}

// HasMethod returns true if this route handles the given HTTP method.
func (r *RegisteredRoute) HasMethod(method string) bool {
	for _, m := range r.Methods {
		if strings.EqualFold(m, method) {
			return true
		}
	}
	return false
}

// RouteRegistry tracks all routes registered by plugins.
type RouteRegistry struct {
	mu     sync.RWMutex
	routes map[string][]*RegisteredRoute // pluginID -> routes
}

// NewRouteRegistry creates a new route registry.
func NewRouteRegistry() *RouteRegistry {
	return &RouteRegistry{
		routes: make(map[string][]*RegisteredRoute),
	}
}

// RegisterRoutes registers routes for a plugin.
// The path pattern is compiled for matching and parameter extraction.
func (r *RouteRegistry) RegisterRoutes(pluginID string, protoRoutes []*pluginv1.PluginRoute) {
	r.mu.Lock()
	defer r.mu.Unlock()

	routes := make([]*RegisteredRoute, 0, len(protoRoutes))
	for _, pr := range protoRoutes {
		route := &RegisteredRoute{
			PluginID:    pluginID,
			Path:        pr.Path,
			FullPath:    "/api/plugins/" + pluginID + pr.Path,
			Methods:     pr.Methods,
			AdminOnly:   pr.AdminOnly,
			Description: pr.Description,
			Capability:  pr.Capability,
			AliasPath:   pr.GetAliasPath(),
			Streaming:   pr.Streaming,
			RateLimit:   pr.RateLimit,
		}

		// Compile path pattern
		route.pathRegex, route.paramNames = compilePathPattern(pr.Path)

		routes = append(routes, route)
	}

	r.routes[pluginID] = routes
}

// UnregisterRoutes removes all routes for a plugin.
func (r *RouteRegistry) UnregisterRoutes(pluginID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.routes, pluginID)
}

// GetRoutes returns all routes for a plugin.
func (r *RouteRegistry) GetRoutes(pluginID string) []*RegisteredRoute {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.routes[pluginID]
}

// GetAllRoutes returns all registered routes across all plugins.
func (r *RouteRegistry) GetAllRoutes() []*RegisteredRoute {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var all []*RegisteredRoute
	for _, routes := range r.routes {
		all = append(all, routes...)
	}
	return all
}

// FindRoute finds a route matching the given plugin ID, path, and method.
// Returns the route, extracted path params, and true if found.
func (r *RouteRegistry) FindRoute(pluginID, path, method string) (*RegisteredRoute, map[string]string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	routes, ok := r.routes[pluginID]
	if !ok {
		return nil, nil, false
	}

	for _, route := range routes {
		if !route.HasMethod(method) {
			continue
		}
		if params, ok := route.Match(path); ok {
			return route, params, true
		}
	}

	return nil, nil, false
}

// FindRouteByCapability finds a route by capability.
func (r *RouteRegistry) FindRouteByCapability(capability string) (*RegisteredRoute, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, routes := range r.routes {
		for _, route := range routes {
			if route.Capability == capability {
				return route, true
			}
		}
	}
	return nil, false
}

// compilePathPattern converts a path pattern like "/items/:id" into a regex
// and extracts parameter names.
func compilePathPattern(pattern string) (*regexp.Regexp, []string) {
	var paramNames []string
	regexPattern := "^"

	parts := strings.Split(pattern, "/")
	for i, part := range parts {
		if i > 0 {
			regexPattern += "/"
		}
		switch {
		case strings.HasPrefix(part, ":"):
			// This is a parameter
			paramName := strings.TrimPrefix(part, ":")
			paramNames = append(paramNames, paramName)
			regexPattern += "([^/]+)"
		case part == "*":
			// Wildcard - matches everything
			regexPattern += "(.*)"
			paramNames = append(paramNames, "*")
		default:
			// Literal path segment
			regexPattern += regexp.QuoteMeta(part)
		}
	}

	regexPattern += "$"

	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return nil, nil
	}

	return re, paramNames
}
