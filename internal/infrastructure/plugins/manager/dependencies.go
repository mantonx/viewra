package manager

import (
	"fmt"

	"github.com/mantonx/viewra/internal/infrastructure/plugins/manifest"
)

// pluginLoadInfo holds manifest and path info for dependency resolution.
type pluginLoadInfo struct {
	manifest *manifest.Manifest
	path     string
}

// resolveLoadOrder performs topological sort based on plugin dependencies.
// Returns the ordered list of plugin IDs to load and a map of skipped plugins with reasons.
func (m *Manager) resolveLoadOrder(
	pluginInfoMap map[string]*pluginLoadInfo,
	capabilityProviders map[string][]string,
) ([]string, map[string]string) {
	// Build adjacency list: plugin -> plugins it depends on
	dependencies := make(map[string][]string)
	skipped := make(map[string]string)

	for pluginID, info := range pluginInfoMap {
		var deps []string

		// Process Requires field (all entries are required)
		for _, capability := range info.manifest.Requires {
			providers := capabilityProviders[capability]
			if len(providers) == 0 {
				skipped[pluginID] = fmt.Sprintf("required capability '%s' not provided by any plugin", capability)
				break
			}
			deps = append(deps, providers...)
		}

		if _, wasSkipped := skipped[pluginID]; !wasSkipped {
			dependencies[pluginID] = deps
		}
	}

	// Remove skipped plugins from consideration
	for pluginID := range skipped {
		delete(dependencies, pluginID)
	}

	// Kahn's algorithm for topological sort
	inDegree := make(map[string]int)
	for pluginID := range dependencies {
		if _, exists := inDegree[pluginID]; !exists {
			inDegree[pluginID] = 0
		}
		for _, dep := range dependencies[pluginID] {
			// Only count dependencies that are in our plugin set
			if _, exists := dependencies[dep]; exists {
				inDegree[pluginID]++
			}
		}
	}

	// Start with plugins that have no dependencies
	var queue []string
	for pluginID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, pluginID)
		}
	}

	var result []string
	for len(queue) > 0 {
		// Pop from queue
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// Reduce in-degree for plugins that depend on current
		for pluginID, deps := range dependencies {
			for _, dep := range deps {
				if dep == current {
					inDegree[pluginID]--
					if inDegree[pluginID] == 0 {
						queue = append(queue, pluginID)
					}
				}
			}
		}
	}

	// Check for circular dependencies
	if len(result) != len(dependencies) {
		// Find plugins involved in cycle
		for pluginID := range dependencies {
			found := false
			for _, r := range result {
				if r == pluginID {
					found = true
					break
				}
			}
			if !found {
				skipped[pluginID] = "circular dependency detected"
			}
		}
	}

	m.logger.Debug("resolved plugin load order",
		"order", result,
		"skipped_count", len(skipped))

	return result, skipped
}
