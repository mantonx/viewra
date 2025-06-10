package pluginmodule

import (
	"time"
)

// Computed properties for PluginHealthState to provide easy access to commonly used metrics
// This fixes the field access issues by providing the correct hierarchy access

// GetTotalRequests returns the total number of requests from current metrics
func (state *PluginHealthState) GetTotalRequests() int64 {
	if state.CurrentMetrics != nil {
		return state.CurrentMetrics.ExecutionCount
	}
	return 0
}

// GetSuccessfulRequests returns the number of successful requests
func (state *PluginHealthState) GetSuccessfulRequests() int64 {
	if state.CurrentMetrics != nil {
		return state.CurrentMetrics.SuccessCount
	}
	return 0
}

// GetFailedRequests returns the number of failed requests
func (state *PluginHealthState) GetFailedRequests() int64 {
	if state.CurrentMetrics != nil {
		return state.CurrentMetrics.ErrorCount
	}
	return 0
}

// GetErrorRate returns the current error rate percentage
func (state *PluginHealthState) GetErrorRate() float64 {
	if state.CurrentHealth != nil {
		return state.CurrentHealth.ErrorRate
	}
	return 0.0
}

// GetAverageResponseTime returns the average response time
func (state *PluginHealthState) GetAverageResponseTime() time.Duration {
	if state.CurrentHealth != nil {
		return state.CurrentHealth.ResponseTime
	}
	if state.CurrentMetrics != nil {
		return state.CurrentMetrics.AverageExecTime
	}
	return 0
}

// GetLastCheckTime returns the last health check time
func (state *PluginHealthState) GetLastCheckTime() time.Time {
	return state.LastCheck
}

// GetUptime returns the uptime duration
func (state *PluginHealthState) GetUptime() time.Duration {
	if state.CurrentHealth != nil {
		return state.CurrentHealth.Uptime
	}
	return time.Since(state.StartTime)
}

// GetMemoryUsage returns the current memory usage
func (state *PluginHealthState) GetMemoryUsage() int64 {
	if state.CurrentHealth != nil {
		return state.CurrentHealth.MemoryUsage
	}
	return 0
}

// GetCPUUsage returns the current CPU usage percentage
func (state *PluginHealthState) GetCPUUsage() float64 {
	if state.CurrentHealth != nil {
		return state.CurrentHealth.CPUUsage
	}
	return 0.0
}

// GetSuccessRate calculates the success rate as a percentage
func (state *PluginHealthState) GetSuccessRate() float64 {
	totalRequests := state.GetTotalRequests()
	if totalRequests == 0 {
		return 100.0 // No requests = 100% success rate
	}
	successfulRequests := state.GetSuccessfulRequests()
	return (float64(successfulRequests) / float64(totalRequests)) * 100.0
}

// IsHealthy returns true if the plugin is in a healthy state
func (state *PluginHealthState) IsHealthy() bool {
	return state.Status == "healthy"
}

// IsDegraded returns true if the plugin is in a degraded state
func (state *PluginHealthState) IsDegraded() bool {
	return state.Status == "degraded"
}

// IsUnhealthy returns true if the plugin is in an unhealthy state
func (state *PluginHealthState) IsUnhealthy() bool {
	return state.Status == "unhealthy"
}

// GetStatusSummary returns a human-readable status summary
func (state *PluginHealthState) GetStatusSummary() map[string]interface{} {
	return map[string]interface{}{
		"plugin_id":             state.PluginID,
		"status":                state.Status,
		"consecutive_failures":  state.ConsecutiveFailures,
		"total_requests":        state.GetTotalRequests(),
		"successful_requests":   state.GetSuccessfulRequests(),
		"failed_requests":       state.GetFailedRequests(),
		"success_rate":          state.GetSuccessRate(),
		"error_rate":            state.GetErrorRate(),
		"average_response_time": state.GetAverageResponseTime().String(),
		"last_check_time":       state.GetLastCheckTime(),
		"uptime":                state.GetUptime().String(),
		"memory_usage":          state.GetMemoryUsage(),
		"cpu_usage":             state.GetCPUUsage(),
		"last_error":            state.LastError,
		"performance_trend":     state.PerformanceTrend,
		"trend_confidence":      state.TrendConfidence,
	}
}
