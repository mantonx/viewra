package pluginmodule

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// RegisterHealthRoutes registers HTTP routes for plugin health monitoring
func (m *ExternalPluginManager) RegisterHealthRoutes(r *gin.Engine) {
	api := r.Group("/api")
	pluginHealth := api.Group("/plugins/health")
	{
		pluginHealth.GET("", m.GetAllPluginHealthHandler)
		pluginHealth.GET("/:plugin_id", m.GetPluginHealthHandler)
		pluginHealth.GET("/:plugin_id/status", m.GetPluginReliabilityStatusHandler)
		pluginHealth.POST("/:plugin_id/reset", m.ResetPluginHealthHandler)
	}

	m.logger.Info("registered plugin health monitoring HTTP routes")
}

// GetAllPluginHealthHandler returns health status for all plugins
func (m *ExternalPluginManager) GetAllPluginHealthHandler(c *gin.Context) {
	healthData := m.GetAllPluginHealth()

	response := make(map[string]interface{})
	for pluginID, health := range healthData {
		response[pluginID] = map[string]interface{}{
			"plugin_id":             health.PluginID,
			"status":                health.Status,
			"error_rate":            health.GetErrorRate(),
			"total_requests":        health.GetTotalRequests(),
			"successful_requests":   health.GetSuccessfulRequests(),
			"failed_requests":       health.GetFailedRequests(),
			"consecutive_failures":  health.ConsecutiveFailures,
			"average_response_time": health.GetAverageResponseTime().String(),
			"uptime":                health.GetUptime().String(),
			"last_error":            health.LastError,
			"last_check_time":       health.GetLastCheckTime(),
			"start_time":            health.StartTime,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"plugins": response,
		"summary": map[string]interface{}{
			"total_plugins":     len(healthData),
			"healthy_plugins":   countHealthyPlugins(healthData),
			"degraded_plugins":  countDegradedPlugins(healthData),
			"unhealthy_plugins": countUnhealthyPlugins(healthData),
			"check_time":        time.Now(),
		},
	})
}

// GetPluginHealthHandler returns health status for a specific plugin
func (m *ExternalPluginManager) GetPluginHealthHandler(c *gin.Context) {
	pluginID := c.Param("plugin_id")
	if pluginID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Plugin ID is required",
		})
		return
	}

	health, err := m.GetPluginHealth(pluginID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":     "Plugin health data not found",
			"plugin_id": pluginID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"plugin_id":             health.PluginID,
		"status":                health.Status,
		"error_rate":            health.GetErrorRate(),
		"total_requests":        health.GetTotalRequests(),
		"successful_requests":   health.GetSuccessfulRequests(),
		"failed_requests":       health.GetFailedRequests(),
		"consecutive_failures":  health.ConsecutiveFailures,
		"average_response_time": health.GetAverageResponseTime().String(),
		"uptime":                health.GetUptime().String(),
		"last_error":            health.LastError,
		"last_check_time":       health.GetLastCheckTime(),
		"start_time":            health.StartTime,
	})
}

// GetPluginReliabilityStatusHandler returns comprehensive reliability status
func (m *ExternalPluginManager) GetPluginReliabilityStatusHandler(c *gin.Context) {
	pluginID := c.Param("plugin_id")
	if pluginID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Plugin ID is required",
		})
		return
	}

	status := m.GetPluginReliabilityStatus(pluginID)
	c.JSON(http.StatusOK, status)
}

// ResetPluginHealthHandler resets health metrics for a plugin
func (m *ExternalPluginManager) ResetPluginHealthHandler(c *gin.Context) {
	pluginID := c.Param("plugin_id")
	if pluginID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Plugin ID is required",
		})
		return
	}

	// Unregister and re-register the plugin to reset its health metrics
	m.healthMonitor.UnregisterPlugin(pluginID)

	// Get the plugin interface to re-register it
	if pluginInterface, exists := m.GetRunningPluginInterface(pluginID); exists {
		m.registerPluginHealth(pluginID, pluginInterface)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Plugin health metrics reset successfully",
		"plugin_id":  pluginID,
		"reset_time": time.Now(),
	})
}

// Helper functions for summary statistics
func countHealthyPlugins(healthData map[string]*PluginHealthState) int {
	count := 0
	for _, health := range healthData {
		if health.Status == "healthy" {
			count++
		}
	}
	return count
}

func countDegradedPlugins(healthData map[string]*PluginHealthState) int {
	count := 0
	for _, health := range healthData {
		if health.Status == "degraded" {
			count++
		}
	}
	return count
}

func countUnhealthyPlugins(healthData map[string]*PluginHealthState) int {
	count := 0
	for _, health := range healthData {
		if health.Status == "unhealthy" {
			count++
		}
	}
	return count
}
