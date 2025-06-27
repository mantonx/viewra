// Package pipeline provides comprehensive health monitoring and metrics for streaming
package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
)

// HealthStatus represents the overall health state
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// StreamingHealthMonitor provides comprehensive monitoring for streaming pipeline
type StreamingHealthMonitor struct {
	logger hclog.Logger

	// Health tracking
	status          HealthStatus
	lastHealthCheck time.Time
	healthMutex     sync.RWMutex

	// Metrics collection
	metrics      *StreamingMetrics
	metricsMutex sync.RWMutex

	// Session tracking
	activeSessions map[string]*SessionHealth
	sessionsMutex  sync.RWMutex

	// Configuration
	healthCheckInterval time.Duration
	alertThresholds     AlertThresholds

	// Monitoring lifecycle
	ctx        context.Context
	cancelFunc context.CancelFunc
	running    bool
	runMutex   sync.RWMutex
}

// StreamingMetrics contains performance and health metrics
type StreamingMetrics struct {
	// System metrics
	TotalSessions  int64 `json:"total_sessions"`
	ActiveSessions int64 `json:"active_sessions"`
	FailedSessions int64 `json:"failed_sessions"`
	TotalSegments  int64 `json:"total_segments"`
	FailedSegments int64 `json:"failed_segments"`

	// Performance metrics
	AverageEncodeTime   time.Duration `json:"average_encode_time"`
	AverageSegmentSize  int64         `json:"average_segment_size"`
	TotalBytesProcessed int64         `json:"total_bytes_processed"`

	// Real-time metrics
	CurrentFPS            float64 `json:"current_fps"`
	CurrentBitrate        int64   `json:"current_bitrate"`
	SegmentProductionRate float64 `json:"segment_production_rate"` // segments per second

	// Error metrics
	FFmpegErrors  int64 `json:"ffmpeg_errors"`
	NetworkErrors int64 `json:"network_errors"`
	StorageErrors int64 `json:"storage_errors"`

	// Timestamps
	LastUpdated      time.Time `json:"last_updated"`
	MetricsStartTime time.Time `json:"metrics_start_time"`
}

// SessionHealth tracks health of individual streaming sessions
type SessionHealth struct {
	SessionID    string       `json:"session_id"`
	ContentHash  string       `json:"content_hash"`
	Status       HealthStatus `json:"status"`
	StartTime    time.Time    `json:"start_time"`
	LastActivity time.Time    `json:"last_activity"`

	// Session-specific metrics
	SegmentsProduced int64   `json:"segments_produced"`
	SegmentsFailed   int64   `json:"segments_failed"`
	CurrentFPS       float64 `json:"current_fps"`
	CurrentSpeed     float64 `json:"current_speed"`
	BytesProcessed   int64   `json:"bytes_processed"`

	// Health indicators
	LastError         string    `json:"last_error,omitempty"`
	ConsecutiveErrors int       `json:"consecutive_errors"`
	StallCount        int       `json:"stall_count"`
	LastStallTime     time.Time `json:"last_stall_time,omitempty"`

	// Performance indicators
	AverageEncodeTime time.Duration   `json:"average_encode_time"`
	EncodeTimeHistory []time.Duration `json:"-"` // Keep last 10 encode times
}

// AlertThresholds defines when to trigger health alerts
type AlertThresholds struct {
	MaxConsecutiveErrors  int           `json:"max_consecutive_errors"`
	MaxStallDuration      time.Duration `json:"max_stall_duration"`
	MinFPS                float64       `json:"min_fps"`
	MaxEncodeTime         time.Duration `json:"max_encode_time"`
	MaxFailureRate        float64       `json:"max_failure_rate"` // 0.0-1.0
	SessionTimeoutMinutes int           `json:"session_timeout_minutes"`
}

// HealthReport provides comprehensive health status
type HealthReport struct {
	OverallStatus  HealthStatus              `json:"overall_status"`
	Timestamp      time.Time                 `json:"timestamp"`
	Metrics        *StreamingMetrics         `json:"metrics"`
	SessionsHealth map[string]*SessionHealth `json:"sessions_health"`
	ActiveAlerts   []HealthAlert             `json:"active_alerts"`
	SystemInfo     map[string]interface{}    `json:"system_info"`
}

// HealthAlert represents a health issue
type HealthAlert struct {
	ID         string        `json:"id"`
	Type       AlertType     `json:"type"`
	Severity   AlertSeverity `json:"severity"`
	Message    string        `json:"message"`
	SessionID  string        `json:"session_id,omitempty"`
	Timestamp  time.Time     `json:"timestamp"`
	Resolved   bool          `json:"resolved"`
	ResolvedAt time.Time     `json:"resolved_at,omitempty"`
}

type AlertType string
type AlertSeverity string

const (
	AlertTypePerformance AlertType = "performance"
	AlertTypeError       AlertType = "error"
	AlertTypeResource    AlertType = "resource"
	AlertTypeTimeout     AlertType = "timeout"

	AlertSeverityLow      AlertSeverity = "low"
	AlertSeverityMedium   AlertSeverity = "medium"
	AlertSeverityHigh     AlertSeverity = "high"
	AlertSeverityCritical AlertSeverity = "critical"
)

// NewStreamingHealthMonitor creates a new health monitor
func NewStreamingHealthMonitor(logger hclog.Logger) *StreamingHealthMonitor {
	return &StreamingHealthMonitor{
		logger: logger,
		status: HealthStatusUnknown,

		metrics: &StreamingMetrics{
			MetricsStartTime: time.Now(),
			LastUpdated:      time.Now(),
		},

		activeSessions:      make(map[string]*SessionHealth),
		healthCheckInterval: 30 * time.Second,

		alertThresholds: AlertThresholds{
			MaxConsecutiveErrors:  5,
			MaxStallDuration:      10 * time.Second,
			MinFPS:                15.0,
			MaxEncodeTime:         30 * time.Second,
			MaxFailureRate:        0.1, // 10% failure rate
			SessionTimeoutMinutes: 60,
		},
	}
}

// Start begins health monitoring
func (hm *StreamingHealthMonitor) Start(ctx context.Context) error {
	hm.runMutex.Lock()
	defer hm.runMutex.Unlock()

	if hm.running {
		return fmt.Errorf("health monitor already running")
	}

	hm.ctx, hm.cancelFunc = context.WithCancel(ctx)
	hm.running = true

	// Start periodic health checks
	go hm.healthCheckLoop()

	hm.logger.Info("Streaming health monitor started",
		"check_interval", hm.healthCheckInterval)

	return nil
}

// Stop stops health monitoring
func (hm *StreamingHealthMonitor) Stop() error {
	hm.runMutex.Lock()
	defer hm.runMutex.Unlock()

	if !hm.running {
		return nil
	}

	if hm.cancelFunc != nil {
		hm.cancelFunc()
	}

	hm.running = false

	hm.logger.Info("Streaming health monitor stopped")
	return nil
}

// RegisterSession registers a new streaming session for monitoring
func (hm *StreamingHealthMonitor) RegisterSession(sessionID, contentHash string) {
	hm.sessionsMutex.Lock()
	defer hm.sessionsMutex.Unlock()

	sessionHealth := &SessionHealth{
		SessionID:         sessionID,
		ContentHash:       contentHash,
		Status:            HealthStatusHealthy,
		StartTime:         time.Now(),
		LastActivity:      time.Now(),
		EncodeTimeHistory: make([]time.Duration, 0, 10),
	}

	hm.activeSessions[sessionID] = sessionHealth

	hm.metricsMutex.Lock()
	hm.metrics.TotalSessions++
	hm.metrics.ActiveSessions++
	hm.metricsMutex.Unlock()

	hm.logger.Debug("Registered session for monitoring",
		"session_id", sessionID,
		"content_hash", contentHash)
}

// UnregisterSession removes a session from monitoring
func (hm *StreamingHealthMonitor) UnregisterSession(sessionID string) {
	hm.sessionsMutex.Lock()
	defer hm.sessionsMutex.Unlock()

	if _, exists := hm.activeSessions[sessionID]; exists {
		delete(hm.activeSessions, sessionID)

		hm.metricsMutex.Lock()
		hm.metrics.ActiveSessions--
		hm.metricsMutex.Unlock()

		hm.logger.Debug("Unregistered session from monitoring", "session_id", sessionID)
	}
}

// RecordSegmentProduced records successful segment production
func (hm *StreamingHealthMonitor) RecordSegmentProduced(sessionID string, segmentIndex int, encodeTime time.Duration, segmentSize int64) {
	hm.sessionsMutex.Lock()
	session, exists := hm.activeSessions[sessionID]
	if exists {
		session.SegmentsProduced++
		session.LastActivity = time.Now()
		session.Status = HealthStatusHealthy
		session.ConsecutiveErrors = 0 // Reset error count on success
		session.BytesProcessed += segmentSize

		// Update encode time history
		session.EncodeTimeHistory = append(session.EncodeTimeHistory, encodeTime)
		if len(session.EncodeTimeHistory) > 10 {
			session.EncodeTimeHistory = session.EncodeTimeHistory[1:] // Keep last 10
		}

		// Calculate average encode time
		if len(session.EncodeTimeHistory) > 0 {
			var total time.Duration
			for _, t := range session.EncodeTimeHistory {
				total += t
			}
			session.AverageEncodeTime = total / time.Duration(len(session.EncodeTimeHistory))
		}
	}
	hm.sessionsMutex.Unlock()

	// Update global metrics
	hm.metricsMutex.Lock()
	hm.metrics.TotalSegments++
	hm.metrics.TotalBytesProcessed += segmentSize

	// Update average encode time globally
	if hm.metrics.TotalSegments > 0 {
		totalTime := time.Duration(hm.metrics.TotalSegments) * hm.metrics.AverageEncodeTime
		hm.metrics.AverageEncodeTime = (totalTime + encodeTime) / time.Duration(hm.metrics.TotalSegments)
	} else {
		hm.metrics.AverageEncodeTime = encodeTime
	}

	hm.metrics.LastUpdated = time.Now()
	hm.metricsMutex.Unlock()

	hm.logger.Debug("Recorded segment production",
		"session_id", sessionID,
		"segment_index", segmentIndex,
		"encode_time", encodeTime,
		"segment_size", segmentSize)
}

// RecordSegmentFailed records failed segment production
func (hm *StreamingHealthMonitor) RecordSegmentFailed(sessionID string, segmentIndex int, err error) {
	hm.sessionsMutex.Lock()
	session, exists := hm.activeSessions[sessionID]
	if exists {
		session.SegmentsFailed++
		session.LastActivity = time.Now()
		session.ConsecutiveErrors++
		session.LastError = err.Error()

		// Update session health status based on consecutive errors
		if session.ConsecutiveErrors >= hm.alertThresholds.MaxConsecutiveErrors {
			session.Status = HealthStatusUnhealthy
		} else if session.ConsecutiveErrors > 2 {
			session.Status = HealthStatusDegraded
		}
	}
	hm.sessionsMutex.Unlock()

	// Update global metrics
	hm.metricsMutex.Lock()
	hm.metrics.FailedSegments++
	hm.metrics.LastUpdated = time.Now()
	hm.metricsMutex.Unlock()

	hm.logger.Warn("Recorded segment failure",
		"session_id", sessionID,
		"segment_index", segmentIndex,
		"error", err)
}

// RecordFFmpegProgress updates real-time FFmpeg metrics
func (hm *StreamingHealthMonitor) RecordFFmpegProgress(sessionID string, progress FFmpegProgress) {
	hm.sessionsMutex.Lock()
	session, exists := hm.activeSessions[sessionID]
	if exists {
		session.CurrentFPS = progress.FPS
		session.CurrentSpeed = progress.Speed
		session.LastActivity = time.Now()

		// Check for stalls
		if progress.FPS < hm.alertThresholds.MinFPS && progress.FPS > 0 {
			session.StallCount++
			session.LastStallTime = time.Now()

			if session.Status == HealthStatusHealthy {
				session.Status = HealthStatusDegraded
			}
		}
	}
	hm.sessionsMutex.Unlock()

	// Update global real-time metrics
	hm.metricsMutex.Lock()
	hm.metrics.CurrentFPS = progress.FPS
	hm.metrics.LastUpdated = time.Now()
	hm.metricsMutex.Unlock()
}

// RecordError records various types of errors
func (hm *StreamingHealthMonitor) RecordError(sessionID string, errorType string, err error) {
	hm.metricsMutex.Lock()
	switch errorType {
	case "ffmpeg":
		hm.metrics.FFmpegErrors++
	case "network":
		hm.metrics.NetworkErrors++
	case "storage":
		hm.metrics.StorageErrors++
	}
	hm.metrics.LastUpdated = time.Now()
	hm.metricsMutex.Unlock()

	hm.logger.Error("Recorded error",
		"session_id", sessionID,
		"error_type", errorType,
		"error", err)
}

// GetHealthReport generates a comprehensive health report
func (hm *StreamingHealthMonitor) GetHealthReport() *HealthReport {
	hm.healthMutex.RLock()
	status := hm.status
	hm.healthMutex.RUnlock()

	hm.metricsMutex.RLock()
	metrics := *hm.metrics // Copy metrics
	hm.metricsMutex.RUnlock()

	hm.sessionsMutex.RLock()
	sessionsHealth := make(map[string]*SessionHealth)
	for id, session := range hm.activeSessions {
		sessionCopy := *session // Copy session data
		sessionsHealth[id] = &sessionCopy
	}
	hm.sessionsMutex.RUnlock()

	// Generate alerts
	alerts := hm.generateAlerts(sessionsHealth)

	// Collect system info
	systemInfo := map[string]interface{}{
		"active_sessions_count": len(sessionsHealth),
		"monitor_uptime":        time.Since(metrics.MetricsStartTime),
		"health_check_interval": hm.healthCheckInterval,
		"alert_thresholds":      hm.alertThresholds,
	}

	return &HealthReport{
		OverallStatus:  status,
		Timestamp:      time.Now(),
		Metrics:        &metrics,
		SessionsHealth: sessionsHealth,
		ActiveAlerts:   alerts,
		SystemInfo:     systemInfo,
	}
}

// generateAlerts creates alerts based on current health status
func (hm *StreamingHealthMonitor) generateAlerts(sessions map[string]*SessionHealth) []HealthAlert {
	var alerts []HealthAlert
	now := time.Now()

	for sessionID, session := range sessions {
		// Check for consecutive errors
		if session.ConsecutiveErrors >= hm.alertThresholds.MaxConsecutiveErrors {
			alerts = append(alerts, HealthAlert{
				ID:        fmt.Sprintf("errors_%s_%d", sessionID, now.Unix()),
				Type:      AlertTypeError,
				Severity:  AlertSeverityCritical,
				Message:   fmt.Sprintf("Session has %d consecutive errors", session.ConsecutiveErrors),
				SessionID: sessionID,
				Timestamp: now,
			})
		}

		// Check for stalls
		if !session.LastStallTime.IsZero() && now.Sub(session.LastStallTime) < hm.alertThresholds.MaxStallDuration {
			alerts = append(alerts, HealthAlert{
				ID:        fmt.Sprintf("stall_%s_%d", sessionID, now.Unix()),
				Type:      AlertTypePerformance,
				Severity:  AlertSeverityHigh,
				Message:   fmt.Sprintf("Session is stalled (FPS: %.1f)", session.CurrentFPS),
				SessionID: sessionID,
				Timestamp: now,
			})
		}

		// Check for slow encoding
		if session.AverageEncodeTime > hm.alertThresholds.MaxEncodeTime {
			alerts = append(alerts, HealthAlert{
				ID:        fmt.Sprintf("slow_%s_%d", sessionID, now.Unix()),
				Type:      AlertTypePerformance,
				Severity:  AlertSeverityMedium,
				Message:   fmt.Sprintf("Slow encoding detected (avg: %v)", session.AverageEncodeTime),
				SessionID: sessionID,
				Timestamp: now,
			})
		}

		// Check for session timeout
		sessionAge := now.Sub(session.StartTime)
		timeoutDuration := time.Duration(hm.alertThresholds.SessionTimeoutMinutes) * time.Minute
		if sessionAge > timeoutDuration && now.Sub(session.LastActivity) > 5*time.Minute {
			alerts = append(alerts, HealthAlert{
				ID:        fmt.Sprintf("timeout_%s_%d", sessionID, now.Unix()),
				Type:      AlertTypeTimeout,
				Severity:  AlertSeverityMedium,
				Message:   fmt.Sprintf("Session inactive for %v", now.Sub(session.LastActivity)),
				SessionID: sessionID,
				Timestamp: now,
			})
		}
	}

	return alerts
}

// healthCheckLoop runs periodic health checks
func (hm *StreamingHealthMonitor) healthCheckLoop() {
	ticker := time.NewTicker(hm.healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-hm.ctx.Done():
			return
		case <-ticker.C:
			hm.performHealthCheck()
		}
	}
}

// performHealthCheck evaluates overall system health
func (hm *StreamingHealthMonitor) performHealthCheck() {
	hm.healthMutex.Lock()
	defer hm.healthMutex.Unlock()

	hm.lastHealthCheck = time.Now()

	// Calculate overall health based on session health and metrics
	hm.sessionsMutex.RLock()
	totalSessions := len(hm.activeSessions)
	unhealthySessions := 0
	degradedSessions := 0

	for _, session := range hm.activeSessions {
		switch session.Status {
		case HealthStatusUnhealthy:
			unhealthySessions++
		case HealthStatusDegraded:
			degradedSessions++
		}
	}
	hm.sessionsMutex.RUnlock()

	// Determine overall status
	if totalSessions == 0 {
		hm.status = HealthStatusHealthy // No sessions = healthy
	} else {
		unhealthyRatio := float64(unhealthySessions) / float64(totalSessions)
		degradedRatio := float64(degradedSessions) / float64(totalSessions)

		if unhealthyRatio > 0.5 { // More than 50% unhealthy
			hm.status = HealthStatusUnhealthy
		} else if unhealthyRatio > 0.2 || degradedRatio > 0.5 { // More than 20% unhealthy or 50% degraded
			hm.status = HealthStatusDegraded
		} else {
			hm.status = HealthStatusHealthy
		}
	}

	hm.logger.Debug("Health check completed",
		"status", hm.status,
		"total_sessions", totalSessions,
		"unhealthy_sessions", unhealthySessions,
		"degraded_sessions", degradedSessions)
}

// GetMetrics returns current streaming metrics
func (hm *StreamingHealthMonitor) GetMetrics() *StreamingMetrics {
	hm.metricsMutex.RLock()
	defer hm.metricsMutex.RUnlock()

	metrics := *hm.metrics // Copy metrics
	return &metrics
}

// GetSessionHealth returns health status for a specific session
func (hm *StreamingHealthMonitor) GetSessionHealth(sessionID string) (*SessionHealth, error) {
	hm.sessionsMutex.RLock()
	defer hm.sessionsMutex.RUnlock()

	session, exists := hm.activeSessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	sessionCopy := *session // Copy session data
	return &sessionCopy, nil
}

// GetOverallHealth returns the current overall health status
func (hm *StreamingHealthMonitor) GetOverallHealth() HealthStatus {
	hm.healthMutex.RLock()
	defer hm.healthMutex.RUnlock()
	return hm.status
}

// IsHealthy returns true if the streaming system is healthy
func (hm *StreamingHealthMonitor) IsHealthy() bool {
	return hm.GetOverallHealth() == HealthStatusHealthy
}

// SetAlertThresholds updates alert threshold configuration
func (hm *StreamingHealthMonitor) SetAlertThresholds(thresholds AlertThresholds) {
	hm.alertThresholds = thresholds
	hm.logger.Debug("Updated alert thresholds", "thresholds", thresholds)
}

// GetAlertThresholds returns current alert thresholds
func (hm *StreamingHealthMonitor) GetAlertThresholds() AlertThresholds {
	return hm.alertThresholds
}
