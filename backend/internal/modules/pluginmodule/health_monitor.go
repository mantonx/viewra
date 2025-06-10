package pluginmodule

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/pkg/plugins"
	"gorm.io/gorm"
)

// Enhanced PluginHealthMonitor with comprehensive health tracking
type PluginHealthMonitor struct {
	logger        hclog.Logger
	db            *gorm.DB
	plugins       map[string]*PluginHealthState
	mutex         sync.RWMutex
	checkInterval time.Duration
	done          chan struct{}
	wg            sync.WaitGroup

	// Health thresholds
	globalThresholds *plugins.HealthThresholds

	// Metrics aggregation
	metricsHistory map[string][]*plugins.PluginMetrics
	maxHistorySize int
}

// PluginHealthState tracks comprehensive health information for a plugin
type PluginHealthState struct {
	PluginID      string                       `json:"plugin_id"`
	Status        string                       `json:"status"`
	LastCheck     time.Time                    `json:"last_check"`
	StartTime     time.Time                    `json:"start_time"`
	HealthService plugins.HealthMonitorService `json:"-"`

	// Current metrics
	CurrentHealth  *plugins.HealthStatus  `json:"current_health"`
	CurrentMetrics *plugins.PluginMetrics `json:"current_metrics"`

	// Health history for trend analysis
	HealthHistory  []*plugins.HealthStatus  `json:"health_history"`
	MetricsHistory []*plugins.PluginMetrics `json:"metrics_history"`

	// Failure tracking
	ConsecutiveFailures int    `json:"consecutive_failures"`
	LastError           string `json:"last_error"`

	// Performance trends
	PerformanceTrend string  `json:"performance_trend"` // "improving", "stable", "degrading"
	TrendConfidence  float64 `json:"trend_confidence"`
}

// NewPluginHealthMonitor creates an enhanced health monitor
func NewPluginHealthMonitor(logger hclog.Logger, db *gorm.DB) *PluginHealthMonitor {
	return &PluginHealthMonitor{
		logger:         logger.Named("health-monitor"),
		db:             db,
		plugins:        make(map[string]*PluginHealthState),
		checkInterval:  30 * time.Second,
		done:           make(chan struct{}),
		metricsHistory: make(map[string][]*plugins.PluginMetrics),
		maxHistorySize: 100, // Keep last 100 health checks
		globalThresholds: &plugins.HealthThresholds{
			MaxMemoryUsage:      512 * 1024 * 1024, // 512MB
			MaxCPUUsage:         80.0,              // 80%
			MaxErrorRate:        5.0,               // 5%
			MaxResponseTime:     10 * time.Second,
			HealthCheckInterval: 30 * time.Second,
		},
	}
}

// RegisterPlugin registers a plugin for health monitoring
func (h *PluginHealthMonitor) RegisterPlugin(pluginID string, healthService plugins.HealthMonitorService) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.logger.Info("registering plugin for health monitoring", "plugin_id", pluginID)

	state := &PluginHealthState{
		PluginID:            pluginID,
		Status:              "starting",
		StartTime:           time.Now(),
		LastCheck:           time.Now(),
		HealthService:       healthService,
		HealthHistory:       make([]*plugins.HealthStatus, 0),
		MetricsHistory:      make([]*plugins.PluginMetrics, 0),
		ConsecutiveFailures: 0,
		PerformanceTrend:    "stable",
		TrendConfidence:     0.0,
	}

	h.plugins[pluginID] = state
	h.metricsHistory[pluginID] = make([]*plugins.PluginMetrics, 0)

	// Set plugin-specific thresholds if the service supports it
	if err := healthService.SetHealthThresholds(h.globalThresholds); err != nil {
		h.logger.Warn("failed to set health thresholds for plugin", "plugin_id", pluginID, "error", err)
	}
}

// UnregisterPlugin removes a plugin from health monitoring
func (h *PluginHealthMonitor) UnregisterPlugin(pluginID string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.logger.Info("unregistering plugin from health monitoring", "plugin_id", pluginID)
	delete(h.plugins, pluginID)
	delete(h.metricsHistory, pluginID)
}

// Start begins the health monitoring process
func (h *PluginHealthMonitor) Start() {
	h.logger.Info("starting plugin health monitor", "check_interval", h.checkInterval)

	h.wg.Add(1)
	go h.monitoringLoop()
}

// Stop gracefully stops the health monitoring
func (h *PluginHealthMonitor) Stop() {
	h.logger.Info("stopping plugin health monitor")
	close(h.done)
	h.wg.Wait()
}

// monitoringLoop runs the continuous health checking
func (h *PluginHealthMonitor) monitoringLoop() {
	defer h.wg.Done()

	ticker := time.NewTicker(h.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-h.done:
			return
		case <-ticker.C:
			h.checkAllPlugins()
		}
	}
}

// checkAllPlugins performs health checks on all registered plugins
func (h *PluginHealthMonitor) checkAllPlugins() {
	h.mutex.RLock()
	plugins := make(map[string]*PluginHealthState)
	for id, state := range h.plugins {
		plugins[id] = state
	}
	h.mutex.RUnlock()

	for pluginID, state := range plugins {
		h.checkPlugin(pluginID, state)
	}
}

// checkPlugin performs a comprehensive health check on a single plugin
func (h *PluginHealthMonitor) checkPlugin(pluginID string, state *PluginHealthState) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	startTime := time.Now()

	// Get health status
	healthStatus, healthErr := state.HealthService.GetHealthStatus(ctx)
	if healthErr != nil {
		h.handleHealthCheckError(pluginID, state, healthErr)
		return
	}

	// Get metrics
	metrics, metricsErr := state.HealthService.GetMetrics(ctx)
	if metricsErr != nil {
		h.logger.Warn("failed to get metrics for plugin", "plugin_id", pluginID, "error", metricsErr)
		// Continue with health status even if metrics fail
	}

	checkDuration := time.Since(startTime)

	h.mutex.Lock()
	defer h.mutex.Unlock()

	// Update state
	state.LastCheck = time.Now()
	state.CurrentHealth = healthStatus
	state.CurrentMetrics = metrics
	state.ConsecutiveFailures = 0 // Reset on successful check
	state.LastError = ""

	// Add to history
	h.addToHistory(state, healthStatus, metrics)

	// Analyze trends
	h.analyzeTrends(state)

	// Determine overall status
	overallStatus := h.determineOverallStatus(healthStatus, metrics)
	state.Status = overallStatus

	h.logger.Debug("health check completed",
		"plugin_id", pluginID,
		"status", overallStatus,
		"check_duration", checkDuration,
		"memory_usage", healthStatus.MemoryUsage,
		"cpu_usage", healthStatus.CPUUsage,
		"error_rate", healthStatus.ErrorRate)

	// Store health check result in database
	h.storeHealthCheckResult(pluginID, healthStatus, metrics, overallStatus)
}

// handleHealthCheckError handles errors during health checks
func (h *PluginHealthMonitor) handleHealthCheckError(pluginID string, state *PluginHealthState, err error) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	state.ConsecutiveFailures++
	state.LastError = err.Error()
	state.LastCheck = time.Now()

	// Determine status based on consecutive failures
	if state.ConsecutiveFailures >= 3 {
		state.Status = "unhealthy"
	} else if state.ConsecutiveFailures >= 1 {
		state.Status = "degraded"
	}

	h.logger.Error("health check failed for plugin",
		"plugin_id", pluginID,
		"consecutive_failures", state.ConsecutiveFailures,
		"error", err)
}

// addToHistory adds health status and metrics to the plugin's history
func (h *PluginHealthMonitor) addToHistory(state *PluginHealthState, health *plugins.HealthStatus, metrics *plugins.PluginMetrics) {
	// Add to health history
	state.HealthHistory = append(state.HealthHistory, health)
	if len(state.HealthHistory) > h.maxHistorySize {
		state.HealthHistory = state.HealthHistory[1:]
	}

	// Add to metrics history
	if metrics != nil {
		state.MetricsHistory = append(state.MetricsHistory, metrics)
		if len(state.MetricsHistory) > h.maxHistorySize {
			state.MetricsHistory = state.MetricsHistory[1:]
		}
	}
}

// analyzeTrends analyzes performance trends for the plugin
func (h *PluginHealthMonitor) analyzeTrends(state *PluginHealthState) {
	if len(state.MetricsHistory) < 5 {
		state.PerformanceTrend = "insufficient_data"
		state.TrendConfidence = 0.0
		return
	}

	// Analyze response time trend over last 10 checks
	recentMetrics := state.MetricsHistory
	if len(recentMetrics) > 10 {
		recentMetrics = recentMetrics[len(recentMetrics)-10:]
	}

	// Simple trend analysis based on average execution time
	if len(recentMetrics) >= 5 {
		firstHalf := recentMetrics[:len(recentMetrics)/2]
		secondHalf := recentMetrics[len(recentMetrics)/2:]

		firstAvg := h.calculateAverageExecTime(firstHalf)
		secondAvg := h.calculateAverageExecTime(secondHalf)

		// Use firstAvg as baseline for comparison
		if secondAvg < time.Duration(float64(firstAvg)*0.9) {
			state.PerformanceTrend = "improving"
			state.TrendConfidence = 0.8
		} else if secondAvg > time.Duration(float64(firstAvg)*1.1) {
			state.PerformanceTrend = "degrading"
			state.TrendConfidence = 0.8
		} else {
			state.PerformanceTrend = "stable"
			state.TrendConfidence = 0.9
		}
	}
}

// calculateAverageExecTime calculates average execution time from metrics slice
func (h *PluginHealthMonitor) calculateAverageExecTime(metrics []*plugins.PluginMetrics) time.Duration {
	if len(metrics) == 0 {
		return 0
	}

	var total time.Duration
	for _, m := range metrics {
		total += m.AverageExecTime
	}
	return total / time.Duration(len(metrics))
}

// determineOverallStatus determines the overall health status based on health and metrics
func (h *PluginHealthMonitor) determineOverallStatus(health *plugins.HealthStatus, metrics *plugins.PluginMetrics) string {
	// Check critical thresholds
	if health.MemoryUsage > h.globalThresholds.MaxMemoryUsage {
		return "unhealthy"
	}

	if health.CPUUsage > h.globalThresholds.MaxCPUUsage {
		return "unhealthy"
	}

	if health.ErrorRate > h.globalThresholds.MaxErrorRate {
		return "unhealthy"
	}

	if health.ResponseTime > h.globalThresholds.MaxResponseTime {
		return "degraded"
	}

	// Check for warning thresholds (80% of max)
	warningMemory := int64(float64(h.globalThresholds.MaxMemoryUsage) * 0.8)
	warningCPU := h.globalThresholds.MaxCPUUsage * 0.8
	warningErrorRate := h.globalThresholds.MaxErrorRate * 0.8

	if health.MemoryUsage > warningMemory ||
		health.CPUUsage > warningCPU ||
		health.ErrorRate > warningErrorRate {
		return "degraded"
	}

	return "healthy"
}

// storeHealthCheckResult stores the health check result in the database
func (h *PluginHealthMonitor) storeHealthCheckResult(pluginID string, health *plugins.HealthStatus, metrics *plugins.PluginMetrics, overallStatus string) {
	// Create a database record for this health check
	// This would typically be stored in a plugin_health_checks table
	h.logger.Debug("storing health check result",
		"plugin_id", pluginID,
		"status", overallStatus,
		"memory_usage", health.MemoryUsage,
		"cpu_usage", health.CPUUsage)
}

// GetPluginHealth returns the current health state for a plugin
func (h *PluginHealthMonitor) GetPluginHealth(pluginID string) (*PluginHealthState, error) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	state, exists := h.plugins[pluginID]
	if !exists {
		return nil, fmt.Errorf("plugin %s not found in health monitor", pluginID)
	}

	// Return a copy to avoid race conditions
	stateCopy := *state
	return &stateCopy, nil
}

// GetAllPluginHealth returns health state for all monitored plugins
func (h *PluginHealthMonitor) GetAllPluginHealth() map[string]*PluginHealthState {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	result := make(map[string]*PluginHealthState)
	for id, state := range h.plugins {
		stateCopy := *state
		result[id] = &stateCopy
	}

	return result
}

// GetSystemHealth returns overall system health based on all plugins
func (h *PluginHealthMonitor) GetSystemHealth() *SystemHealthStatus {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	status := &SystemHealthStatus{
		Timestamp:      time.Now(),
		TotalPlugins:   len(h.plugins),
		HealthyCount:   0,
		DegradedCount:  0,
		UnhealthyCount: 0,
		PluginDetails:  make(map[string]string),
	}

	for pluginID, state := range h.plugins {
		status.PluginDetails[pluginID] = state.Status

		switch state.Status {
		case "healthy":
			status.HealthyCount++
		case "degraded":
			status.DegradedCount++
		case "unhealthy":
			status.UnhealthyCount++
		}
	}

	// Determine overall system status
	if status.UnhealthyCount > 0 {
		status.OverallStatus = "unhealthy"
	} else if status.DegradedCount > 0 {
		status.OverallStatus = "degraded"
	} else {
		status.OverallStatus = "healthy"
	}

	return status
}

// SystemHealthStatus represents the overall health of the plugin system
type SystemHealthStatus struct {
	Timestamp      time.Time         `json:"timestamp"`
	OverallStatus  string            `json:"overall_status"`
	TotalPlugins   int               `json:"total_plugins"`
	HealthyCount   int               `json:"healthy_count"`
	DegradedCount  int               `json:"degraded_count"`
	UnhealthyCount int               `json:"unhealthy_count"`
	PluginDetails  map[string]string `json:"plugin_details"`
}

// UpdateGlobalThresholds updates the global health thresholds
func (h *PluginHealthMonitor) UpdateGlobalThresholds(thresholds *plugins.HealthThresholds) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.globalThresholds = thresholds
	h.checkInterval = thresholds.HealthCheckInterval

	// Update thresholds for all registered plugins
	for pluginID, state := range h.plugins {
		if err := state.HealthService.SetHealthThresholds(thresholds); err != nil {
			h.logger.Warn("failed to update thresholds for plugin", "plugin_id", pluginID, "error", err)
		}
	}

	h.logger.Info("updated global health thresholds", "new_interval", h.checkInterval)
	return nil
}
