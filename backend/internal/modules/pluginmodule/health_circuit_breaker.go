package pluginmodule

import (
	"sync"
	"time"
)

// HealthMonitorCircuitBreakerManager integrates circuit breaker functionality with health monitoring
type HealthMonitorCircuitBreakerManager struct {
	circuitBreakers map[string]*PluginCircuitBreaker
	mutex           sync.RWMutex
	config          *CircuitBreakerConfig
}

// NewHealthMonitorCircuitBreakerManager creates a new circuit breaker manager
func NewHealthMonitorCircuitBreakerManager() *HealthMonitorCircuitBreakerManager {
	return &HealthMonitorCircuitBreakerManager{
		circuitBreakers: make(map[string]*PluginCircuitBreaker),
		config:          DefaultCircuitBreakerConfig(),
	}
}

// GetOrCreateCircuitBreaker returns existing circuit breaker or creates a new one
func (cm *HealthMonitorCircuitBreakerManager) GetOrCreateCircuitBreaker(pluginID string) *PluginCircuitBreaker {
	cm.mutex.RLock()
	if cb, exists := cm.circuitBreakers[pluginID]; exists {
		cm.mutex.RUnlock()
		return cb
	}
	cm.mutex.RUnlock()

	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// Double-check pattern
	if cb, exists := cm.circuitBreakers[pluginID]; exists {
		return cb
	}

	cb := NewPluginCircuitBreaker(pluginID, cm.config)
	cm.circuitBreakers[pluginID] = cb
	return cb
}

// RemoveCircuitBreaker removes a circuit breaker for a plugin
func (cm *HealthMonitorCircuitBreakerManager) RemoveCircuitBreaker(pluginID string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	delete(cm.circuitBreakers, pluginID)
}

// ShouldAllowRequest checks if a request should be allowed for a plugin
func (cm *HealthMonitorCircuitBreakerManager) ShouldAllowRequest(pluginID string) bool {
	cb := cm.GetOrCreateCircuitBreaker(pluginID)
	return cb.ShouldAllowRequest()
}

// RecordRequest records a request result for a plugin
func (cm *HealthMonitorCircuitBreakerManager) RecordRequest(pluginID string, success bool, responseTime time.Duration, err error) {
	cb := cm.GetOrCreateCircuitBreaker(pluginID)
	cb.RecordRequest(success, responseTime, err)
}

// GetCircuitBreakerState returns the current state of a circuit breaker
func (cm *HealthMonitorCircuitBreakerManager) GetCircuitBreakerState(pluginID string) CircuitBreakerState {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	if cb, exists := cm.circuitBreakers[pluginID]; exists {
		return cb.GetState()
	}
	return CircuitBreakerClosed // Default state if no circuit breaker exists
}

// GetCircuitBreakerMetrics returns metrics for a circuit breaker
func (cm *HealthMonitorCircuitBreakerManager) GetCircuitBreakerMetrics(pluginID string) *CircuitBreakerMetrics {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	if cb, exists := cm.circuitBreakers[pluginID]; exists {
		return cb.GetMetrics()
	}
	return nil
}

// ResetCircuitBreaker resets a circuit breaker to initial state
func (cm *HealthMonitorCircuitBreakerManager) ResetCircuitBreaker(pluginID string) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	if cb, exists := cm.circuitBreakers[pluginID]; exists {
		cb.Reset()
	}
}

// GetAllCircuitBreakerStates returns the states of all circuit breakers
func (cm *HealthMonitorCircuitBreakerManager) GetAllCircuitBreakerStates() map[string]CircuitBreakerState {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	states := make(map[string]CircuitBreakerState)
	for pluginID, cb := range cm.circuitBreakers {
		states[pluginID] = cb.GetState()
	}
	return states
}

// Integration methods for PluginHealthMonitor

// Add circuit breaker manager to PluginHealthMonitor (extend the existing struct)
func (h *PluginHealthMonitor) InitializeCircuitBreakers() {
	// This would ideally be added to the PluginHealthMonitor struct definition
	// For now, we provide the functionality through the existing extension methods
}

// Enhanced ShouldAllowRequest with circuit breaker integration
func (h *PluginHealthMonitor) ShouldAllowRequestWithCircuitBreaker(pluginID string) bool {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if state, exists := h.plugins[pluginID]; exists {
		// Check consecutive failures first (simple circuit breaker)
		if state.ConsecutiveFailures >= 5 {
			// Check if enough time has passed for recovery attempt
			timeSinceLastFailure := time.Since(state.LastCheck)
			if timeSinceLastFailure < 30*time.Second {
				return false // Circuit is "open"
			}
			// Allow one request to test recovery (half-open state)
		}

		// Additional checks based on error rate
		if state.CurrentHealth != nil && state.CurrentHealth.ErrorRate > 50.0 {
			return false
		}

		return true
	}
	return true
}
