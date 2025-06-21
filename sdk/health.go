package plugins

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// BaseHealthService provides a standard implementation of HealthMonitorService
// that can be embedded or used directly by plugins
type BaseHealthService struct {
	startTime  time.Time
	mutex      sync.RWMutex
	thresholds *HealthThresholds
	pluginName string

	// Metrics tracking
	totalRequests      int64
	successfulRequests int64
	errorCount         int64
	totalResponseTime  time.Duration

	// Performance metrics
	lastError       string
	lastRequestTime time.Time
	memoryPeakUsage int64

	// Custom metrics for plugin-specific data
	customCounters map[string]int64
	customGauges   map[string]float64
	customTimers   map[string]time.Duration
}

// NewBaseHealthService creates a new base health monitoring service
func NewBaseHealthService(pluginName string) *BaseHealthService {
	return &BaseHealthService{
		startTime:      time.Now(),
		pluginName:     pluginName,
		customCounters: make(map[string]int64),
		customGauges:   make(map[string]float64),
		customTimers:   make(map[string]time.Duration),
		thresholds: &HealthThresholds{
			MaxMemoryUsage:      256 * 1024 * 1024, // 256MB default
			MaxCPUUsage:         70.0,              // 70% default
			MaxErrorRate:        3.0,               // 3% default
			MaxResponseTime:     5 * time.Second,   // 5s default
			HealthCheckInterval: 30 * time.Second,  // 30s default
		},
	}
}

// GetHealthStatus returns the current health status
func (h *BaseHealthService) GetHealthStatus(ctx context.Context) (*HealthStatus, error) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	// Get current memory usage
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	currentMemory := int64(memStats.Alloc)

	// Update peak memory usage
	if currentMemory > h.memoryPeakUsage {
		h.memoryPeakUsage = currentMemory
	}

	// Calculate error rate
	var errorRate float64
	if h.totalRequests > 0 {
		errorRate = (float64(h.errorCount) / float64(h.totalRequests)) * 100
	}

	// Calculate average response time
	var avgResponseTime time.Duration
	if h.successfulRequests > 0 {
		avgResponseTime = h.totalResponseTime / time.Duration(h.successfulRequests)
	}

	// Determine overall status
	status := "healthy"
	message := fmt.Sprintf("%s plugin operating normally", h.pluginName)

	if currentMemory > h.thresholds.MaxMemoryUsage {
		status = "unhealthy"
		message = "Memory usage exceeds threshold"
	} else if errorRate > h.thresholds.MaxErrorRate {
		status = "unhealthy"
		message = "Error rate exceeds threshold"
	} else if avgResponseTime > h.thresholds.MaxResponseTime {
		status = "degraded"
		message = "Response time above threshold"
	} else if currentMemory > int64(float64(h.thresholds.MaxMemoryUsage)*0.8) {
		status = "degraded"
		message = "Memory usage approaching threshold"
	} else if errorRate > h.thresholds.MaxErrorRate*0.8 {
		status = "degraded"
		message = "Error rate approaching threshold"
	}

	// Build details map with standard metrics plus custom metrics
	details := map[string]string{
		"plugin_name":         h.pluginName,
		"peak_memory_usage":   fmt.Sprintf("%d", h.memoryPeakUsage),
		"last_request_time":   h.lastRequestTime.Format(time.RFC3339),
		"last_error":          h.lastError,
		"total_requests":      fmt.Sprintf("%d", h.totalRequests),
		"successful_requests": fmt.Sprintf("%d", h.successfulRequests),
	}

	// Add custom counters to details
	for key, value := range h.customCounters {
		details["custom_"+key] = fmt.Sprintf("%d", value)
	}

	// Add custom gauges to details
	for key, value := range h.customGauges {
		details["custom_"+key] = fmt.Sprintf("%.2f", value)
	}

	return &HealthStatus{
		Status:       status,
		Message:      message,
		LastCheck:    time.Now(),
		Uptime:       time.Since(h.startTime),
		MemoryUsage:  currentMemory,
		CPUUsage:     h.getCurrentCPUUsage(),
		ErrorRate:    errorRate,
		ResponseTime: avgResponseTime,
		Details:      details,
	}, nil
}

// GetMetrics returns performance metrics
func (h *BaseHealthService) GetMetrics(ctx context.Context) (*PluginMetrics, error) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	var avgExecTime time.Duration
	if h.successfulRequests > 0 {
		avgExecTime = h.totalResponseTime / time.Duration(h.successfulRequests)
	}

	// Build custom metrics map
	customMetrics := map[string]interface{}{
		"plugin_name":       h.pluginName,
		"peak_memory_usage": h.memoryPeakUsage,
		"success_rate":      h.getSuccessRate(),
	}

	// Add custom counters
	for key, value := range h.customCounters {
		customMetrics["counter_"+key] = value
	}

	// Add custom gauges
	for key, value := range h.customGauges {
		customMetrics["gauge_"+key] = value
	}

	// Add custom timers
	for key, value := range h.customTimers {
		customMetrics["timer_"+key] = value.Milliseconds()
	}

	return &PluginMetrics{
		ExecutionCount:  h.totalRequests,
		SuccessCount:    h.successfulRequests,
		ErrorCount:      h.errorCount,
		AverageExecTime: avgExecTime,
		LastExecution:   h.lastRequestTime,
		BytesProcessed:  0, // Can be overridden by specific plugins
		ItemsProcessed:  h.successfulRequests,
		CacheHitRate:    0, // Can be overridden by specific plugins
		CustomMetrics:   customMetrics,
	}, nil
}

// SetHealthThresholds configures health check thresholds
func (h *BaseHealthService) SetHealthThresholds(thresholds *HealthThresholds) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.thresholds = thresholds
	return nil
}

// Public methods for plugins to record metrics

// RecordRequest records a request attempt and its outcome
func (h *BaseHealthService) RecordRequest(success bool, responseTime time.Duration, err error) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.totalRequests++
	h.lastRequestTime = time.Now()

	if success {
		h.successfulRequests++
		h.totalResponseTime += responseTime
	} else {
		h.errorCount++
		if err != nil {
			h.lastError = err.Error()
		}
	}
}

// IncrementCounter increments a custom counter metric
func (h *BaseHealthService) IncrementCounter(name string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.customCounters[name]++
}

// AddToCounter adds a value to a custom counter metric
func (h *BaseHealthService) AddToCounter(name string, value int64) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.customCounters[name] += value
}

// SetGauge sets a custom gauge metric value
func (h *BaseHealthService) SetGauge(name string, value float64) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.customGauges[name] = value
}

// RecordTimer records a timing metric
func (h *BaseHealthService) RecordTimer(name string, duration time.Duration) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.customTimers[name] = duration
}

// GetCounter returns the current value of a counter
func (h *BaseHealthService) GetCounter(name string) int64 {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	return h.customCounters[name]
}

// GetGauge returns the current value of a gauge
func (h *BaseHealthService) GetGauge(name string) float64 {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	return h.customGauges[name]
}

// Reset resets all metrics (useful for testing or periodic cleanup)
func (h *BaseHealthService) Reset() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.totalRequests = 0
	h.successfulRequests = 0
	h.errorCount = 0
	h.totalResponseTime = 0
	h.lastError = ""
	h.memoryPeakUsage = 0

	// Clear custom metrics
	h.customCounters = make(map[string]int64)
	h.customGauges = make(map[string]float64)
	h.customTimers = make(map[string]time.Duration)
}

// Private helper methods

// getCurrentCPUUsage estimates current CPU usage (simplified)
func (h *BaseHealthService) getCurrentCPUUsage() float64 {
	// For a simple implementation, we can estimate based on goroutines
	// In a real implementation, you might use a more sophisticated CPU monitoring approach
	numGoroutines := float64(runtime.NumGoroutine())
	maxGoroutines := 100.0 // Assumed max for this plugin

	usage := (numGoroutines / maxGoroutines) * 100
	if usage > 100 {
		usage = 100
	}
	return usage
}

// getSuccessRate calculates the success rate percentage
func (h *BaseHealthService) getSuccessRate() float64 {
	if h.totalRequests == 0 {
		return 0.0
	}
	return (float64(h.successfulRequests) / float64(h.totalRequests)) * 100
}

// HealthServiceBuilder provides a builder pattern for creating customized health services
type HealthServiceBuilder struct {
	service *BaseHealthService
}

// NewHealthServiceBuilder creates a new builder for health services
func NewHealthServiceBuilder(pluginName string) *HealthServiceBuilder {
	return &HealthServiceBuilder{
		service: NewBaseHealthService(pluginName),
	}
}

// WithThresholds sets custom health thresholds
func (b *HealthServiceBuilder) WithThresholds(thresholds *HealthThresholds) *HealthServiceBuilder {
	b.service.thresholds = thresholds
	return b
}

// WithCustomCounter initializes a custom counter
func (b *HealthServiceBuilder) WithCustomCounter(name string, initialValue int64) *HealthServiceBuilder {
	b.service.customCounters[name] = initialValue
	return b
}

// WithCustomGauge initializes a custom gauge
func (b *HealthServiceBuilder) WithCustomGauge(name string, initialValue float64) *HealthServiceBuilder {
	b.service.customGauges[name] = initialValue
	return b
}

// Build returns the configured health service
func (b *HealthServiceBuilder) Build() *BaseHealthService {
	return b.service
}
