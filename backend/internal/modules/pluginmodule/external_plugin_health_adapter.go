package pluginmodule

import (
	"context"
	"time"

	"github.com/mantonx/viewra/pkg/plugins"
)

// ExternalPluginHealthAdapter adapts external plugins to HealthMonitorService interface
type ExternalPluginHealthAdapter struct {
	pluginID string
	plugin   ExternalPluginInterface
}

// NewExternalPluginHealthAdapter creates a health adapter for external plugins
func NewExternalPluginHealthAdapter(pluginID string, plugin ExternalPluginInterface) *ExternalPluginHealthAdapter {
	return &ExternalPluginHealthAdapter{
		pluginID: pluginID,
		plugin:   plugin,
	}
}

// GetHealthStatus implements HealthMonitorService interface
func (a *ExternalPluginHealthAdapter) GetHealthStatus(ctx context.Context) (*plugins.HealthStatus, error) {
	// Try to get health from the plugin
	err := a.plugin.Health()
	status := "healthy"
	message := "Plugin is healthy"

	if err != nil {
		status = "unhealthy"
		message = err.Error()
	}

	return &plugins.HealthStatus{
		Status:       status,
		Message:      message,
		LastCheck:    time.Now(),
		Uptime:       time.Since(time.Now()), // Placeholder - would need actual start time
		MemoryUsage:  0,                      // Not available from external plugin
		CPUUsage:     0,                      // Not available from external plugin
		ErrorRate:    0,                      // Not available from external plugin
		ResponseTime: 0,                      // Not available from external plugin
		Details:      make(map[string]string),
	}, nil
}

// GetMetrics implements HealthMonitorService interface
func (a *ExternalPluginHealthAdapter) GetMetrics(ctx context.Context) (*plugins.PluginMetrics, error) {
	// External plugins don't provide detailed metrics, return basic metrics
	return &plugins.PluginMetrics{
		ExecutionCount:  0,
		SuccessCount:    0,
		ErrorCount:      0,
		AverageExecTime: 0,
		LastExecution:   time.Now(),
		BytesProcessed:  0,
		ItemsProcessed:  0,
		CacheHitRate:    0,
		CustomMetrics:   make(map[string]interface{}),
	}, nil
}

// SetHealthThresholds implements HealthMonitorService interface
func (a *ExternalPluginHealthAdapter) SetHealthThresholds(thresholds *plugins.HealthThresholds) error {
	// External plugins can't configure thresholds, so this is a no-op
	return nil
}

// Helper method to register external plugin with health monitor
func (m *ExternalPluginManager) registerPluginHealth(pluginID string, pluginInterface ExternalPluginInterface) {
	// Create health adapter for external plugin
	healthAdapter := NewExternalPluginHealthAdapter(pluginID, pluginInterface)

	// Register with health monitor
	m.healthMonitor.RegisterPlugin(pluginID, healthAdapter)
}
