package manager

import (
	"context"
	"fmt"
	"time"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	domainevents "github.com/mantonx/viewra/internal/domain/events"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/types"
)

// StartHealthMonitor starts a background goroutine that monitors plugin health.
func (m *Manager) StartHealthMonitor(ctx context.Context) {
	ticker := time.NewTicker(m.healthCheckInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				m.checkAllHealth(ctx)
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

func (m *Manager) checkAllHealth(ctx context.Context) {
	plugins := m.ListPlugins()
	for _, p := range plugins {
		m.checkPluginHealth(ctx, p)
	}
}

func (m *Manager) checkPluginHealth(ctx context.Context, p *types.Instance) {
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Capture previous status for change detection
	previousStatus := p.Health.Status

	health, err := p.CoreClient.HealthCheck(checkCtx, &pluginv1.Empty{})
	if err != nil {
		p.UpdateHealth(pluginv1.HealthStatus_UNHEALTHY, err.Error())
		m.logger.Warn("plugin health check failed", "plugin", p.ID, "error", err)

		// Publish health update event (status changed to unhealthy)
		if m.publisher != nil && previousStatus != pluginv1.HealthStatus_UNHEALTHY {
			m.publisher.Publish(domainevents.NewEvent(domainevents.EventPluginHealthUpdate, "plugin-manager").
				WithData("plugin_id", p.ID).
				WithData("status", "unhealthy").
				WithData("previous_status", previousStatus.String()).
				WithData("message", err.Error()).
				Build())
		}

		// Check if we should restart the plugin
		if p.Health.Restarts < m.maxRestarts {
			m.restartPlugin(ctx, p)
		}
		return
	}

	// Publish health update event only if status changed
	if m.publisher != nil && health.Status != previousStatus {
		m.publisher.Publish(domainevents.NewEvent(domainevents.EventPluginHealthUpdate, "plugin-manager").
			WithData("plugin_id", p.ID).
			WithData("status", health.Status.String()).
			WithData("previous_status", previousStatus.String()).
			WithData("message", health.Message).
			Build())
	}

	p.UpdateHealth(health.Status, health.Message)
}

func (m *Manager) restartPlugin(ctx context.Context, p *types.Instance) {
	m.logger.Info("restarting plugin", "plugin", p.ID, "restarts", p.Health.Restarts)

	willRestart := p.Health.Restarts < m.maxRestarts
	restartCount := p.Health.Restarts + 1

	// Publish plugin.crashed event before restart attempt
	if m.publisher != nil {
		m.publisher.Publish(domainevents.NewEvent(domainevents.EventPluginCrashed, "plugin-manager").
			WithData("plugin_id", p.ID).
			WithData("restart_count", restartCount).
			WithData("will_restart", willRestart).
			WithData("message", "health check failed").
			Build())
	}

	// Kill the old process
	p.Client.Kill()

	// Remove from registry
	m.mu.Lock()
	delete(m.plugins, p.ID)
	m.mu.Unlock()

	// Try to reload
	newInstance, err := m.LoadPlugin(ctx, p.Path)
	if err != nil {
		m.logger.Error("failed to restart plugin", "plugin", p.ID, "error", err)
		// Publish crash event for failed restart
		if m.publisher != nil {
			m.publisher.Publish(domainevents.NewEvent(domainevents.EventPluginCrashed, "plugin-manager").
				WithData("plugin_id", p.ID).
				WithData("restart_count", restartCount).
				WithData("will_restart", false).
				WithData("message", err.Error()).
				Build())
		}
		return
	}

	newInstance.Health.Restarts = restartCount

	// Publish plugin.loaded with is_restart=true (overrides the one from LoadPlugin)
	if m.publisher != nil {
		m.publisher.Publish(domainevents.NewEvent(domainevents.EventPluginLoaded, "plugin-manager").
			WithData("plugin_id", newInstance.ID).
			WithData("name", newInstance.Manifest.Name).
			WithData("version", newInstance.Manifest.Version).
			WithData("capabilities", newInstance.Manifest.Capabilities).
			WithData("is_restart", true).
			WithData("restart_count", restartCount).
			Build())
	}

	m.logger.Info("plugin restarted successfully", "plugin", p.ID, "restarts", newInstance.Health.Restarts)
}

// RestartPlugin restarts a plugin by ID. This is useful when the plugin binary
// has been rebuilt and we need to reload it without a full server restart.
func (m *Manager) RestartPlugin(ctx context.Context, pluginID string) error {
	m.mu.RLock()
	p, ok := m.plugins[pluginID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}

	m.logger.Info("restarting plugin by request", "plugin", pluginID)

	// Kill the old process
	p.Client.Kill()

	// Remove from registry
	m.mu.Lock()
	delete(m.plugins, pluginID)
	m.mu.Unlock()

	// Try to reload
	newInstance, err := m.LoadPlugin(ctx, p.Path)
	if err != nil {
		return fmt.Errorf("failed to restart plugin %s: %w", pluginID, err)
	}

	// Publish plugin.loaded event
	if m.publisher != nil {
		m.publisher.Publish(domainevents.NewEvent(domainevents.EventPluginLoaded, "plugin-manager").
			WithData("plugin_id", newInstance.ID).
			WithData("name", newInstance.Manifest.Name).
			WithData("version", newInstance.Manifest.Version).
			WithData("capabilities", newInstance.Manifest.Capabilities).
			WithData("is_restart", true).
			Build())
	}

	m.logger.Info("plugin restarted successfully by request", "plugin", pluginID)
	return nil
}
