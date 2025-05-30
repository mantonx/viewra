package scanner

import (
	"fmt"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/events"
)

// ScanTelemetry handles events, logs, and metrics for scanner operations
type ScanTelemetry struct {
	eventBus events.EventBus
	jobID    uint
	mu       sync.RWMutex
	metrics  map[string]interface{}
	logs     []LogEntry
	maxLogs  int
}

// LogEntry represents a log entry
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields"`
	Error     string                 `json:"error,omitempty"`
}

// NewScanTelemetry creates a new scan telemetry collector
func NewScanTelemetry(eventBus events.EventBus, jobID uint) *ScanTelemetry {
	return &ScanTelemetry{
		eventBus: eventBus,
		jobID:    jobID,
		metrics:  make(map[string]interface{}),
		logs:     make([]LogEntry, 0),
		maxLogs:  1000, // Keep last 1000 log entries
	}
}

// EmitEvent publishes an event to the event bus
func (st *ScanTelemetry) EmitEvent(event events.Event) {
	if st.eventBus != nil {
		st.eventBus.PublishAsync(event)
	}
}

// EmitScanStarted emits a scan started event
func (st *ScanTelemetry) EmitScanStarted(libraryID uint, path string) {
	event := events.Event{
		Type:    "scan.started",
		Source:  "scanner",
		Title:   "Scan Started",
		Message: fmt.Sprintf("Started scanning library %d", libraryID),
		Data: map[string]interface{}{
			"job_id":     st.jobID,
			"library_id": libraryID,
			"path":       path,
		},
	}
	st.EmitEvent(event)
}

// EmitScanResumed emits a scan resumed event
func (st *ScanTelemetry) EmitScanResumed(libraryID uint, path string) {
	event := events.Event{
		Type:    "scan.resumed",
		Source:  "scanner",
		Title:   "Scan Resumed",
		Message: fmt.Sprintf("Resumed scanning library %d", libraryID),
		Data: map[string]interface{}{
			"job_id":     st.jobID,
			"library_id": libraryID,
			"path":       path,
		},
	}
	st.EmitEvent(event)
}

// EmitScanPaused emits a scan paused event
func (st *ScanTelemetry) EmitScanPaused(filesProcessed, filesSkipped, errorsCount int64) {
	event := events.Event{
		Type:    "scan.paused",
		Source:  "scanner",
		Title:   "Scan Paused",
		Message: fmt.Sprintf("Paused scanning job %d", st.jobID),
		Data: map[string]interface{}{
			"job_id":          st.jobID,
			"files_processed": filesProcessed,
			"files_skipped":   filesSkipped,
			"errors_count":    errorsCount,
		},
	}
	st.EmitEvent(event)
}

// EmitScanCompleted emits a scan completed event
func (st *ScanTelemetry) EmitScanCompleted(filesProcessed, filesSkipped, errorsCount int64, duration time.Duration) {
	event := events.Event{
		Type:    "scan.completed",
		Source:  "scanner",
		Title:   "Scan Completed",
		Message: fmt.Sprintf("Completed scanning job %d", st.jobID),
		Data: map[string]interface{}{
			"job_id":          st.jobID,
			"files_processed": filesProcessed,
			"files_skipped":   filesSkipped,
			"errors_count":    errorsCount,
			"duration_ms":     duration.Milliseconds(),
			"throughput_fps":  float64(filesProcessed) / duration.Seconds(),
		},
	}
	st.EmitEvent(event)
}

// EmitScanProgress emits a scan progress event
func (st *ScanTelemetry) EmitScanProgress(data map[string]interface{}) {
	// Ensure job_id is included
	data["job_id"] = st.jobID
	
	event := events.Event{
		Type:    "scan.progress",
		Source:  "scanner",
		Title:   "Scan Progress",
		Message: fmt.Sprintf("Scanning progress: %v files processed", data["files_processed"]),
		Data:    data,
	}
	st.EmitEvent(event)
}

// LogInfo logs an informational message
func (st *ScanTelemetry) LogInfo(message string, fields map[string]interface{}) {
	st.addLogEntry("info", message, fields, "")
}

// LogWarning logs a warning message
func (st *ScanTelemetry) LogWarning(message string, fields map[string]interface{}) {
	st.addLogEntry("warning", message, fields, "")
}

// LogError logs an error message
func (st *ScanTelemetry) LogError(message string, err error, fields map[string]interface{}) {
	errorMsg := ""
	if err != nil {
		errorMsg = err.Error()
	}
	st.addLogEntry("error", message, fields, errorMsg)
}

// LogDebug logs a debug message
func (st *ScanTelemetry) LogDebug(message string, fields map[string]interface{}) {
	st.addLogEntry("debug", message, fields, "")
}

// addLogEntry adds a log entry to the internal log buffer
func (st *ScanTelemetry) addLogEntry(level, message string, fields map[string]interface{}, errorMsg string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Fields:    fields,
		Error:     errorMsg,
	}
	
	st.logs = append(st.logs, entry)
	
	// Trim logs if we exceed the maximum
	if len(st.logs) > st.maxLogs {
		// Keep the last maxLogs entries - ensure we don't get negative indices
		startIndex := len(st.logs) - st.maxLogs
		if startIndex < 0 {
			startIndex = 0
		}
		st.logs = st.logs[startIndex:]
	}
	
	// Also log to console for debugging
	fieldsStr := ""
	if fields != nil {
		fieldsStr = fmt.Sprintf(" %+v", fields)
	}
	if errorMsg != "" {
		fmt.Printf("[%s] %s: %s (error: %s)%s\n", level, time.Now().Format("15:04:05"), message, errorMsg, fieldsStr)
	} else {
		fmt.Printf("[%s] %s: %s%s\n", level, time.Now().Format("15:04:05"), message, fieldsStr)
	}
}

// RecordMetric records a metric value
func (st *ScanTelemetry) RecordMetric(name string, value float64, tags map[string]string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	
	metricData := map[string]interface{}{
		"value":     value,
		"tags":      tags,
		"timestamp": time.Now(),
	}
	
	st.metrics[name] = metricData
}

// RecordCounter increments a counter metric
func (st *ScanTelemetry) RecordCounter(name string, tags map[string]string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	
	// Get existing counter value or start at 0
	counter := float64(0)
	if existing, ok := st.metrics[name]; ok {
		if existingData, ok := existing.(map[string]interface{}); ok {
			if existingValue, ok := existingData["value"].(float64); ok {
				counter = existingValue
			}
		}
	}
	
	counter++
	
	metricData := map[string]interface{}{
		"value":     counter,
		"tags":      tags,
		"timestamp": time.Now(),
	}
	
	st.metrics[name] = metricData
}

// RecordTiming records a timing metric
func (st *ScanTelemetry) RecordTiming(name string, duration time.Duration, tags map[string]string) {
	st.RecordMetric(name, float64(duration.Milliseconds()), tags)
}

// GetMetrics returns all recorded metrics
func (st *ScanTelemetry) GetMetrics() map[string]interface{} {
	st.mu.RLock()
	defer st.mu.RUnlock()
	
	// Create a copy to avoid race conditions
	result := make(map[string]interface{})
	for k, v := range st.metrics {
		result[k] = v
	}
	return result
}

// GetLogs returns all log entries
func (st *ScanTelemetry) GetLogs() []LogEntry {
	st.mu.RLock()
	defer st.mu.RUnlock()
	
	// Create a copy to avoid race conditions
	result := make([]LogEntry, len(st.logs))
	copy(result, st.logs)
	return result
}

// GetLogsSince returns log entries since a given time
func (st *ScanTelemetry) GetLogsSince(since time.Time) []LogEntry {
	st.mu.RLock()
	defer st.mu.RUnlock()
	
	var result []LogEntry
	for _, entry := range st.logs {
		if entry.Timestamp.After(since) {
			result = append(result, entry)
		}
	}
	return result
}

// GetLogsWithLevel returns log entries with a specific level
func (st *ScanTelemetry) GetLogsWithLevel(level string) []LogEntry {
	st.mu.RLock()
	defer st.mu.RUnlock()
	
	var result []LogEntry
	for _, entry := range st.logs {
		if entry.Level == level {
			result = append(result, entry)
		}
	}
	return result
}

// ClearLogs clears all log entries
func (st *ScanTelemetry) ClearLogs() {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.logs = st.logs[:0]
}

// ClearMetrics clears all metrics
func (st *ScanTelemetry) ClearMetrics() {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.metrics = make(map[string]interface{})
}

// GetSummary returns a summary of telemetry data
func (st *ScanTelemetry) GetSummary() map[string]interface{} {
	st.mu.RLock()
	defer st.mu.RUnlock()
	
	errorCount := 0
	warningCount := 0
	infoCount := 0
	debugCount := 0
	
	for _, entry := range st.logs {
		switch entry.Level {
		case "error":
			errorCount++
		case "warning":
			warningCount++
		case "info":
			infoCount++
		case "debug":
			debugCount++
		}
	}
	
	return map[string]interface{}{
		"job_id":        st.jobID,
		"total_logs":    len(st.logs),
		"error_count":   errorCount,
		"warning_count": warningCount,
		"info_count":    infoCount,
		"debug_count":   debugCount,
		"metrics_count": len(st.metrics),
		"last_updated":  time.Now(),
	}
} 