package registry

import (
	"sort"
	"sync"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// RegisteredWidget represents a widget registered by a plugin.
type RegisteredWidget struct {
	// PluginID is the plugin that provides this widget.
	PluginID string

	// Widget is the widget definition.
	Widget sdk.Widget

	// Enabled indicates if the widget is currently enabled.
	// Can be toggled by admin or based on plugin settings.
	Enabled bool
}

// WidgetRegistry manages widgets registered by plugins.
// Widgets are grouped by location and sorted by priority within each location.
type WidgetRegistry struct {
	mu      sync.RWMutex
	widgets map[string]*RegisteredWidget // widgetKey (pluginID:widgetID) -> widget
}

// NewWidgetRegistry creates a new widget registry.
func NewWidgetRegistry() *WidgetRegistry {
	return &WidgetRegistry{
		widgets: make(map[string]*RegisteredWidget),
	}
}

// widgetKey creates a unique key for a widget.
func widgetKey(pluginID, widgetID string) string {
	return pluginID + ":" + widgetID
}

// Register registers a widget from a plugin.
// If the widget already exists, it is updated.
func (r *WidgetRegistry) Register(pluginID string, widget sdk.Widget) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := widgetKey(pluginID, widget.ID)
	r.widgets[key] = &RegisteredWidget{
		PluginID: pluginID,
		Widget:   widget,
		Enabled:  true,
	}
}

// RegisterAll registers multiple widgets from a plugin.
func (r *WidgetRegistry) RegisterAll(pluginID string, widgets []sdk.Widget) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, w := range widgets {
		key := widgetKey(pluginID, w.ID)
		r.widgets[key] = &RegisteredWidget{
			PluginID: pluginID,
			Widget:   w,
			Enabled:  true,
		}
	}
}

// Unregister removes a specific widget.
func (r *WidgetRegistry) Unregister(pluginID, widgetID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.widgets, widgetKey(pluginID, widgetID))
}

// UnregisterPlugin removes all widgets from a plugin.
func (r *WidgetRegistry) UnregisterPlugin(pluginID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for key, widget := range r.widgets {
		if widget.PluginID == pluginID {
			delete(r.widgets, key)
		}
	}
}

// SetEnabled sets whether a widget is enabled.
func (r *WidgetRegistry) SetEnabled(pluginID, widgetID string, enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := widgetKey(pluginID, widgetID)
	if widget, ok := r.widgets[key]; ok {
		widget.Enabled = enabled
	}
}

// Get returns a specific widget.
// Returns nil if not found.
func (r *WidgetRegistry) Get(pluginID, widgetID string) *RegisteredWidget {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.widgets[widgetKey(pluginID, widgetID)]
}

// GetByID returns a widget by its ID (without plugin prefix).
// If multiple plugins provide the same widget ID, returns the first match.
// Returns nil if not found.
func (r *WidgetRegistry) GetByID(widgetID string) *RegisteredWidget {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, widget := range r.widgets {
		if widget.Widget.ID == widgetID {
			return widget
		}
	}
	return nil
}

// GetAll returns all registered widgets, sorted by priority (highest first).
func (r *WidgetRegistry) GetAll() []*RegisteredWidget {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*RegisteredWidget, 0, len(r.widgets))
	for _, widget := range r.widgets {
		result = append(result, widget)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Widget.Priority > result[j].Widget.Priority
	})

	return result
}

// GetEnabled returns all enabled widgets, sorted by priority.
func (r *WidgetRegistry) GetEnabled() []*RegisteredWidget {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*RegisteredWidget, 0)
	for _, widget := range r.widgets {
		if widget.Enabled {
			result = append(result, widget)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Widget.Priority > result[j].Widget.Priority
	})

	return result
}

// GetByLocation returns all enabled widgets for a specific location, sorted by priority.
func (r *WidgetRegistry) GetByLocation(location string) []*RegisteredWidget {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*RegisteredWidget, 0)
	for _, widget := range r.widgets {
		if widget.Enabled && widget.Widget.Location == location {
			result = append(result, widget)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Widget.Priority > result[j].Widget.Priority
	})

	return result
}

// GetByClientType returns all enabled widgets that match a client type, sorted by priority.
// Widgets with ClientTypeAll match all client types.
func (r *WidgetRegistry) GetByClientType(clientType string) []*RegisteredWidget {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*RegisteredWidget, 0)
	for _, widget := range r.widgets {
		if !widget.Enabled {
			continue
		}
		if matchesClientType(widget.Widget.ClientTypes, clientType) {
			result = append(result, widget)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Widget.Priority > result[j].Widget.Priority
	})

	return result
}

// GetByLocationAndClientType returns enabled widgets for a location and client type.
func (r *WidgetRegistry) GetByLocationAndClientType(location, clientType string) []*RegisteredWidget {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*RegisteredWidget, 0)
	for _, widget := range r.widgets {
		if !widget.Enabled {
			continue
		}
		if widget.Widget.Location != location {
			continue
		}
		if !matchesClientType(widget.Widget.ClientTypes, clientType) {
			continue
		}
		result = append(result, widget)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Widget.Priority > result[j].Widget.Priority
	})

	return result
}

// GetForPlugin returns all widgets registered by a plugin.
func (r *WidgetRegistry) GetForPlugin(pluginID string) []*RegisteredWidget {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*RegisteredWidget, 0)
	for _, widget := range r.widgets {
		if widget.PluginID == pluginID {
			result = append(result, widget)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Widget.Priority > result[j].Widget.Priority
	})

	return result
}

// Count returns the total number of registered widgets.
func (r *WidgetRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.widgets)
}

// CountEnabled returns the number of enabled widgets.
func (r *WidgetRegistry) CountEnabled() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, widget := range r.widgets {
		if widget.Enabled {
			count++
		}
	}
	return count
}

// matchesClientType checks if a widget's client types include the given client type.
func matchesClientType(widgetClientTypes []string, clientType string) bool {
	if len(widgetClientTypes) == 0 {
		return true // No restriction means all clients
	}

	for _, ct := range widgetClientTypes {
		if ct == sdk.ClientTypeAll || ct == clientType {
			return true
		}
	}
	return false
}
