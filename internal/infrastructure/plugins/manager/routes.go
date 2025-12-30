package manager

import (
	"context"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/types"
)

// registerPluginRoutes fetches and registers routes for a plugin.
func (m *Manager) registerPluginRoutes(ctx context.Context, plugin *types.Instance) error {
	if plugin.CoreClient == nil {
		return nil
	}

	// Get routes from plugin
	routes, err := plugin.CoreClient.GetRoutes(ctx, &pluginv1.Empty{})
	if err != nil {
		return err
	}

	if routes == nil || len(routes.Routes) == 0 {
		m.logger.Debug("plugin has no routes", "plugin", plugin.ID)
		return nil
	}

	// Register routes
	m.routeRegistry.RegisterRoutes(plugin.ID, routes.Routes)

	m.logger.Debug("registered plugin routes",
		"plugin", plugin.ID,
		"count", len(routes.Routes))

	// Register capabilities
	for _, route := range routes.Routes {
		if route.Capability != "" {
			if m.capabilityRegistry.Register(plugin.ID, route.Capability, route.Path) {
				m.logger.Debug("registered capability",
					"plugin", plugin.ID,
					"capability", route.Capability,
					"path", route.Path)
			} else {
				m.logger.Warn("capability already registered by another plugin",
					"capability", route.Capability,
					"plugin", plugin.ID)
			}
		}
	}

	return nil
}
