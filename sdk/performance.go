package plugins

import (
	"fmt"
	"sync"
	"time"
)

// BasePerformanceMonitor provides reusable performance monitoring for plugins
type BasePerformanceMonitor struct {
	mutex      sync.RWMutex
	startTime  time.Time
	pluginName string

	// Operation metrics
	operations map[string]*OperationMetrics

	// Global metrics
	totalOperations    int64
	successfulOps      int64
	failedOps          int64
	totalExecutionTime time.Duration

	// Error tracking
	errorsByType    map[string]int64
	recentErrors    []ErrorEvent
	maxErrorHistory int

	// Custom metrics for plugin-specific data
	customCounters map[string]int64
	customGauges   map[string]float64
	customTimers   map[string]time.Duration
}

// OperationMetrics tracks metrics for a specific operation type
type OperationMetrics struct {
	Name            string        `json:"name"`
	TotalCalls      int64         `json:"total_calls"`
	SuccessfulCalls int64         `json:"successful_calls"`
	FailedCalls     int64         `json:"failed_calls"`
	TotalTime       time.Duration `json:"total_time"`
	MinTime         time.Duration `json:"min_time"`
	MaxTime         time.Duration `json:"max_time"`
	LastCall        time.Time     `json:"last_call"`
}

// ErrorEvent represents an error occurrence
type ErrorEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Context   string    `json:"context"`
	Operation string    `json:"operation"`
}

// PerformanceSnapshot represents a comprehensive performance snapshot
type PerformanceSnapshot struct {
	Timestamp  time.Time     `json:"timestamp"`
	PluginName string        `json:"plugin_name"`
	Uptime     time.Duration `json:"uptime"`

	// Global metrics
	TotalOperations      int64         `json:"total_operations"`
	SuccessfulOperations int64         `json:"successful_operations"`
	FailedOperations     int64         `json:"failed_operations"`
	OverallSuccessRate   float64       `json:"overall_success_rate"`
	AverageExecutionTime time.Duration `json:"average_execution_time"`
	OperationsPerSecond  float64       `json:"operations_per_second"`

	// Operation-specific metrics
	Operations map[string]*OperationSnapshot `json:"operations"`

	// Error metrics
	TotalErrors  int64            `json:"total_errors"`
	ErrorsByType map[string]int64 `json:"errors_by_type"`
	RecentErrors []ErrorEvent     `json:"recent_errors"`
	ErrorRate    float64          `json:"error_rate"`

	// Custom metrics
	CustomCounters map[string]int64         `json:"custom_counters"`
	CustomGauges   map[string]float64       `json:"custom_gauges"`
	CustomTimers   map[string]time.Duration `json:"custom_timers"`
}

// OperationSnapshot represents metrics for a specific operation
type OperationSnapshot struct {
	Name           string        `json:"name"`
	TotalCalls     int64         `json:"total_calls"`
	SuccessRate    float64       `json:"success_rate"`
	AverageTime    time.Duration `json:"average_time"`
	MinTime        time.Duration `json:"min_time"`
	MaxTime        time.Duration `json:"max_time"`
	CallsPerSecond float64       `json:"calls_per_second"`
	LastCall       time.Time     `json:"last_call"`
}

// NewBasePerformanceMonitor creates a new base performance monitor
func NewBasePerformanceMonitor(pluginName string) *BasePerformanceMonitor {
	return &BasePerformanceMonitor{
		startTime:       time.Now(),
		pluginName:      pluginName,
		operations:      make(map[string]*OperationMetrics),
		errorsByType:    make(map[string]int64),
		recentErrors:    make([]ErrorEvent, 0),
		maxErrorHistory: 50,
		customCounters:  make(map[string]int64),
		customGauges:    make(map[string]float64),
		customTimers:    make(map[string]time.Duration),
	}
}

// RecordOperation records an operation execution
func (pm *BasePerformanceMonitor) RecordOperation(operationName string, duration time.Duration, success bool, context string) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Update global metrics
	pm.totalOperations++
	pm.totalExecutionTime += duration

	if success {
		pm.successfulOps++
	} else {
		pm.failedOps++
		pm.recordError(operationName, "Operation failed", context, operationName)
	}

	// Update operation-specific metrics
	op, exists := pm.operations[operationName]
	if !exists {
		op = &OperationMetrics{
			Name:    operationName,
			MinTime: duration,
			MaxTime: duration,
		}
		pm.operations[operationName] = op
	}

	op.TotalCalls++
	op.TotalTime += duration
	op.LastCall = time.Now()

	if success {
		op.SuccessfulCalls++
	} else {
		op.FailedCalls++
	}

	// Update min/max times
	if duration < op.MinTime {
		op.MinTime = duration
	}
	if duration > op.MaxTime {
		op.MaxTime = duration
	}
}

// RecordError records an error event
func (pm *BasePerformanceMonitor) RecordError(errorType, message, context, operation string) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	pm.recordError(errorType, message, context, operation)
}

// recordError records an error event (internal, not thread-safe)
func (pm *BasePerformanceMonitor) recordError(errorType, message, context, operation string) {
	pm.errorsByType[errorType]++

	errorEvent := ErrorEvent{
		Timestamp: time.Now(),
		Type:      errorType,
		Message:   message,
		Context:   context,
		Operation: operation,
	}

	pm.recentErrors = append(pm.recentErrors, errorEvent)

	// Keep only recent errors
	if len(pm.recentErrors) > pm.maxErrorHistory {
		pm.recentErrors = pm.recentErrors[1:]
	}
}

// Custom metrics methods (similar to health service)
func (pm *BasePerformanceMonitor) IncrementCounter(name string) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	pm.customCounters[name]++
}

func (pm *BasePerformanceMonitor) AddToCounter(name string, value int64) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	pm.customCounters[name] += value
}

func (pm *BasePerformanceMonitor) SetGauge(name string, value float64) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	pm.customGauges[name] = value
}

func (pm *BasePerformanceMonitor) RecordTimer(name string, duration time.Duration) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	pm.customTimers[name] = duration
}

// GetSnapshot returns a comprehensive performance snapshot
func (pm *BasePerformanceMonitor) GetSnapshot() *PerformanceSnapshot {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	now := time.Now()
	uptime := now.Sub(pm.startTime)

	// Calculate global metrics
	var overallSuccessRate float64
	if pm.totalOperations > 0 {
		overallSuccessRate = float64(pm.successfulOps) / float64(pm.totalOperations) * 100
	}

	var averageExecutionTime time.Duration
	if pm.successfulOps > 0 {
		averageExecutionTime = pm.totalExecutionTime / time.Duration(pm.successfulOps)
	}

	var operationsPerSecond float64
	if uptime.Seconds() > 0 {
		operationsPerSecond = float64(pm.totalOperations) / uptime.Seconds()
	}

	// Calculate operation snapshots
	operations := make(map[string]*OperationSnapshot)
	for name, op := range pm.operations {
		var successRate float64
		if op.TotalCalls > 0 {
			successRate = float64(op.SuccessfulCalls) / float64(op.TotalCalls) * 100
		}

		var averageTime time.Duration
		if op.SuccessfulCalls > 0 {
			averageTime = op.TotalTime / time.Duration(op.SuccessfulCalls)
		}

		var callsPerSecond float64
		if uptime.Seconds() > 0 {
			callsPerSecond = float64(op.TotalCalls) / uptime.Seconds()
		}

		operations[name] = &OperationSnapshot{
			Name:           name,
			TotalCalls:     op.TotalCalls,
			SuccessRate:    successRate,
			AverageTime:    averageTime,
			MinTime:        op.MinTime,
			MaxTime:        op.MaxTime,
			CallsPerSecond: callsPerSecond,
			LastCall:       op.LastCall,
		}
	}

	// Calculate error rate
	totalErrors := int64(0)
	for _, count := range pm.errorsByType {
		totalErrors += count
	}

	var errorRate float64
	if pm.totalOperations > 0 {
		errorRate = float64(totalErrors) / float64(pm.totalOperations) * 100
	}

	// Copy maps to prevent data races
	errorsByType := make(map[string]int64)
	for k, v := range pm.errorsByType {
		errorsByType[k] = v
	}

	customCounters := make(map[string]int64)
	for k, v := range pm.customCounters {
		customCounters[k] = v
	}

	customGauges := make(map[string]float64)
	for k, v := range pm.customGauges {
		customGauges[k] = v
	}

	customTimers := make(map[string]time.Duration)
	for k, v := range pm.customTimers {
		customTimers[k] = v
	}

	recentErrors := make([]ErrorEvent, len(pm.recentErrors))
	copy(recentErrors, pm.recentErrors)

	return &PerformanceSnapshot{
		Timestamp:            now,
		PluginName:           pm.pluginName,
		Uptime:               uptime,
		TotalOperations:      pm.totalOperations,
		SuccessfulOperations: pm.successfulOps,
		FailedOperations:     pm.failedOps,
		OverallSuccessRate:   overallSuccessRate,
		AverageExecutionTime: averageExecutionTime,
		OperationsPerSecond:  operationsPerSecond,
		Operations:           operations,
		TotalErrors:          totalErrors,
		ErrorsByType:         errorsByType,
		RecentErrors:         recentErrors,
		ErrorRate:            errorRate,
		CustomCounters:       customCounters,
		CustomGauges:         customGauges,
		CustomTimers:         customTimers,
	}
}

// Reset resets all performance metrics
func (pm *BasePerformanceMonitor) Reset() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	pm.startTime = time.Now()
	pm.totalOperations = 0
	pm.successfulOps = 0
	pm.failedOps = 0
	pm.totalExecutionTime = 0
	pm.operations = make(map[string]*OperationMetrics)
	pm.errorsByType = make(map[string]int64)
	pm.recentErrors = make([]ErrorEvent, 0)
	pm.customCounters = make(map[string]int64)
	pm.customGauges = make(map[string]float64)
	pm.customTimers = make(map[string]time.Duration)
}

// GetUptimeString returns a human-readable uptime string
func (pm *BasePerformanceMonitor) GetUptimeString() string {
	snapshot := pm.GetSnapshot()
	return formatDuration(snapshot.Uptime)
}

// GetErrorRate returns the current error rate as a percentage
func (pm *BasePerformanceMonitor) GetErrorRate() float64 {
	snapshot := pm.GetSnapshot()
	return snapshot.ErrorRate
}

// Helper function to format duration
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// PerformanceMonitorBuilder provides a builder pattern for creating customized performance monitors
type PerformanceMonitorBuilder struct {
	monitor *BasePerformanceMonitor
}

// NewPerformanceMonitorBuilder creates a new builder
func NewPerformanceMonitorBuilder(pluginName string) *PerformanceMonitorBuilder {
	return &PerformanceMonitorBuilder{
		monitor: NewBasePerformanceMonitor(pluginName),
	}
}

// WithCustomCounter initializes a custom counter
func (b *PerformanceMonitorBuilder) WithCustomCounter(name string, initialValue int64) *PerformanceMonitorBuilder {
	b.monitor.customCounters[name] = initialValue
	return b
}

// WithCustomGauge initializes a custom gauge
func (b *PerformanceMonitorBuilder) WithCustomGauge(name string, initialValue float64) *PerformanceMonitorBuilder {
	b.monitor.customGauges[name] = initialValue
	return b
}

// WithMaxErrorHistory sets the maximum number of errors to keep in history
func (b *PerformanceMonitorBuilder) WithMaxErrorHistory(maxErrors int) *PerformanceMonitorBuilder {
	b.monitor.maxErrorHistory = maxErrors
	return b
}

// Build returns the configured performance monitor
func (b *PerformanceMonitorBuilder) Build() *BasePerformanceMonitor {
	return b.monitor
}
