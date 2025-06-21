package apiroutes

// "log" // Keep commented out or remove if not used elsewhere
import (
	"sync"
)

// APIRoute defines the structure for an API route entry.
type APIRoute struct {
	Path        string `json:"path"`
	Method      string `json:"method"`
	Description string `json:"description"`
	// Future: Add PluginID string `json:"plugin_id,omitempty"`
}

var (
	routeRegistry = make([]APIRoute, 0)
	registryMu    sync.RWMutex
)

// Register adds a new route to the API registry.
func Register(path, method, description string) {
	registryMu.Lock()
	defer registryMu.Unlock()
	routeRegistry = append(routeRegistry, APIRoute{
		Path:        path,
		Method:      method,
		Description: description,
	})
	// log.Printf("[DEBUG][apiroutes.Register] Registered: %s %s. Registry length: %d", method, path, len(routeRegistry))
}

// Get retrieves a copy of the current API route registry.
func Get() []APIRoute {
	registryMu.RLock()
	defer registryMu.RUnlock()
	// log.Printf("[DEBUG][apiroutes.Get] Current registry length: %d", len(routeRegistry))
	// Return a copy to prevent external modification
	registryCopy := make([]APIRoute, len(routeRegistry))
	copy(registryCopy, routeRegistry)
	return registryCopy
}

// ClearForTesting removes all registered routes. For use in tests only.
func ClearForTesting() {
	registryMu.Lock()
	defer registryMu.Unlock()
	routeRegistry = make([]APIRoute, 0)
}
