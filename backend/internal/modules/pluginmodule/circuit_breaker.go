package pluginmodule

import (
	"sync"
	"time"

	"github.com/mantonx/viewra/pkg/plugins"
)

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	CircuitBreakerClosed CircuitBreakerState = iota
	CircuitBreakerOpen
	CircuitBreakerHalfOpen
)

// CircuitBreakerConfig holds configuration for circuit breaker behavior
type CircuitBreakerConfig struct {
	FailureThreshold  int           // Number of failures before opening circuit
	RecoveryTimeout   time.Duration // Time to wait before trying half-open
	SuccessThreshold  int           // Number of successes needed to close circuit in half-open state
	RequestTimeout    time.Duration // Timeout for individual requests
	SlidingWindowSize int           // Size of sliding window for failure counting
	MinRequestsNeeded int           // Minimum requests before circuit can open
}

// DefaultCircuitBreakerConfig returns sensible defaults
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		FailureThreshold:  5,
		RecoveryTimeout:   30 * time.Second,
		SuccessThreshold:  3,
		RequestTimeout:    10 * time.Second,
		SlidingWindowSize: 20,
		MinRequestsNeeded: 10,
	}
}

// CircuitBreakerMetrics tracks request metrics for circuit breaker decisions
type CircuitBreakerMetrics struct {
	TotalRequests        int64
	SuccessfulRequests   int64
	FailedRequests       int64
	ConsecutiveFailures  int
	ConsecutiveSuccesses int
	LastFailureTime      time.Time
	LastSuccessTime      time.Time

	// Sliding window for recent requests
	RecentRequests []RequestResult
	WindowIndex    int
}

// RequestResult represents the result of a single request
type RequestResult struct {
	Success      bool
	Timestamp    time.Time
	ResponseTime time.Duration
	Error        string
}

// PluginCircuitBreaker manages circuit breaker state for a specific plugin
type PluginCircuitBreaker struct {
	pluginID    string
	state       CircuitBreakerState
	config      *CircuitBreakerConfig
	metrics     *CircuitBreakerMetrics
	stateChange time.Time
	mutex       sync.RWMutex
}

// NewPluginCircuitBreaker creates a new circuit breaker for a plugin
func NewPluginCircuitBreaker(pluginID string, config *CircuitBreakerConfig) *PluginCircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}

	return &PluginCircuitBreaker{
		pluginID:    pluginID,
		state:       CircuitBreakerClosed,
		config:      config,
		stateChange: time.Now(),
		metrics: &CircuitBreakerMetrics{
			RecentRequests: make([]RequestResult, config.SlidingWindowSize),
		},
	}
}

// ShouldAllowRequest determines if a request should be allowed based on circuit breaker state
func (cb *PluginCircuitBreaker) ShouldAllowRequest() bool {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	switch cb.state {
	case CircuitBreakerClosed:
		return true
	case CircuitBreakerOpen:
		// Check if enough time has passed to try half-open
		if time.Since(cb.stateChange) >= cb.config.RecoveryTimeout {
			cb.mutex.RUnlock()
			cb.mutex.Lock()
			// Double-check pattern
			if cb.state == CircuitBreakerOpen && time.Since(cb.stateChange) >= cb.config.RecoveryTimeout {
				cb.state = CircuitBreakerHalfOpen
				cb.stateChange = time.Now()
				cb.metrics.ConsecutiveSuccesses = 0
			}
			cb.mutex.Unlock()
			cb.mutex.RLock()
			return true
		}
		return false
	case CircuitBreakerHalfOpen:
		// Allow limited requests to test if service has recovered
		return cb.metrics.ConsecutiveSuccesses < cb.config.SuccessThreshold
	default:
		return false
	}
}

// RecordRequest records the result of a request and updates circuit breaker state
func (cb *PluginCircuitBreaker) RecordRequest(success bool, responseTime time.Duration, err error) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	// Record metrics
	cb.metrics.TotalRequests++
	result := RequestResult{
		Success:      success,
		Timestamp:    time.Now(),
		ResponseTime: responseTime,
	}
	if err != nil {
		result.Error = err.Error()
	}

	// Add to sliding window
	cb.metrics.RecentRequests[cb.metrics.WindowIndex] = result
	cb.metrics.WindowIndex = (cb.metrics.WindowIndex + 1) % cb.config.SlidingWindowSize

	if success {
		cb.metrics.SuccessfulRequests++
		cb.metrics.ConsecutiveFailures = 0
		cb.metrics.ConsecutiveSuccesses++
		cb.metrics.LastSuccessTime = time.Now()

		// If in half-open state and enough successes, close circuit
		if cb.state == CircuitBreakerHalfOpen && cb.metrics.ConsecutiveSuccesses >= cb.config.SuccessThreshold {
			cb.state = CircuitBreakerClosed
			cb.stateChange = time.Now()
		}
	} else {
		cb.metrics.FailedRequests++
		cb.metrics.ConsecutiveFailures++
		cb.metrics.ConsecutiveSuccesses = 0
		cb.metrics.LastFailureTime = time.Now()

		// Check if we should open the circuit
		if cb.state == CircuitBreakerClosed || cb.state == CircuitBreakerHalfOpen {
			if cb.shouldOpenCircuit() {
				cb.state = CircuitBreakerOpen
				cb.stateChange = time.Now()
			}
		}
	}
}

// shouldOpenCircuit determines if the circuit should be opened based on failure patterns
func (cb *PluginCircuitBreaker) shouldOpenCircuit() bool {
	// Need minimum requests before considering opening circuit
	if cb.metrics.TotalRequests < int64(cb.config.MinRequestsNeeded) {
		return false
	}

	// Check consecutive failures
	if cb.metrics.ConsecutiveFailures >= cb.config.FailureThreshold {
		return true
	}

	// Check failure rate in sliding window
	recentFailures := 0
	recentRequests := 0
	now := time.Now()
	windowDuration := 5 * time.Minute // Consider last 5 minutes

	for _, req := range cb.metrics.RecentRequests {
		if !req.Timestamp.IsZero() && now.Sub(req.Timestamp) <= windowDuration {
			recentRequests++
			if !req.Success {
				recentFailures++
			}
		}
	}

	if recentRequests >= cb.config.MinRequestsNeeded {
		failureRate := float64(recentFailures) / float64(recentRequests)
		return failureRate >= 0.5 // 50% failure rate threshold
	}

	return false
}

// GetState returns the current circuit breaker state
func (cb *PluginCircuitBreaker) GetState() CircuitBreakerState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// GetMetrics returns current metrics
func (cb *PluginCircuitBreaker) GetMetrics() *CircuitBreakerMetrics {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	// Return a copy to avoid race conditions
	metricsCopy := *cb.metrics
	return &metricsCopy
}

// Reset resets the circuit breaker to initial state
func (cb *PluginCircuitBreaker) Reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.state = CircuitBreakerClosed
	cb.stateChange = time.Now()
	cb.metrics = &CircuitBreakerMetrics{
		RecentRequests: make([]RequestResult, cb.config.SlidingWindowSize),
	}
}

// Add circuit breaker methods to PluginHealthMonitor via extension
func (h *PluginHealthMonitor) ShouldAllowRequest(pluginID string) bool {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if state, exists := h.plugins[pluginID]; exists {
		// Simple implementation - allow if consecutive failures < 5
		return state.ConsecutiveFailures < 5
	}
	return true
}

func (h *PluginHealthMonitor) RecordRequest(pluginID string, success bool, responseTime time.Duration, err error) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	state, exists := h.plugins[pluginID]
	if !exists {
		return
	}

	// Update the metrics in the current health state
	if state.CurrentMetrics == nil {
		state.CurrentMetrics = &plugins.PluginMetrics{
			CustomMetrics: make(map[string]interface{}),
		}
	}

	// Update execution metrics
	state.CurrentMetrics.ExecutionCount++
	state.CurrentMetrics.LastExecution = time.Now()

	// Update average execution time
	if state.CurrentMetrics.AverageExecTime == 0 {
		state.CurrentMetrics.AverageExecTime = responseTime
	} else {
		// Simple moving average
		state.CurrentMetrics.AverageExecTime = (state.CurrentMetrics.AverageExecTime + responseTime) / 2
	}

	if success {
		state.CurrentMetrics.SuccessCount++
		state.ConsecutiveFailures = 0
		state.LastError = ""
	} else {
		state.CurrentMetrics.ErrorCount++
		state.ConsecutiveFailures++
		if err != nil {
			state.LastError = err.Error()
		}
	}

	// Update health status metrics if they exist
	if state.CurrentHealth != nil {
		state.CurrentHealth.ResponseTime = state.CurrentMetrics.AverageExecTime

		// Calculate error rate
		if state.CurrentMetrics.ExecutionCount > 0 {
			state.CurrentHealth.ErrorRate = (float64(state.CurrentMetrics.ErrorCount) / float64(state.CurrentMetrics.ExecutionCount)) * 100.0
		}

		state.CurrentHealth.LastCheck = time.Now()
	}
}

func (h *PluginHealthMonitor) StartHealthChecks(ctx interface{}) {
	// Wrapper for the existing Start method
	h.Start()
}
