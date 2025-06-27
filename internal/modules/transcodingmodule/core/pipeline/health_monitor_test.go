package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamingHealthMonitor_NewStreamingHealthMonitor(t *testing.T) {
	logger := hclog.NewNullLogger()
	monitor := NewStreamingHealthMonitor(logger)

	assert.NotNil(t, monitor)
	assert.Equal(t, HealthStatusUnknown, monitor.status)
	assert.NotNil(t, monitor.metrics)
	assert.NotNil(t, monitor.activeSessions)
	assert.Equal(t, 30*time.Second, monitor.healthCheckInterval)
}

func TestStreamingHealthMonitor_StartStop(t *testing.T) {
	logger := hclog.NewNullLogger()
	monitor := NewStreamingHealthMonitor(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test starting
	err := monitor.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, monitor.running)

	// Test starting again (should fail)
	err = monitor.Start(ctx)
	assert.Error(t, err)

	// Test stopping
	err = monitor.Stop()
	assert.NoError(t, err)
	assert.False(t, monitor.running)

	// Test stopping again (should succeed)
	err = monitor.Stop()
	assert.NoError(t, err)
}

func TestStreamingHealthMonitor_SessionManagement(t *testing.T) {
	logger := hclog.NewNullLogger()
	monitor := NewStreamingHealthMonitor(logger)

	sessionID := "test-session-1"
	contentHash := "abc123"

	// Register session
	monitor.RegisterSession(sessionID, contentHash)

	// Check session exists
	session, err := monitor.GetSessionHealth(sessionID)
	assert.NoError(t, err)
	assert.NotNil(t, session)
	assert.Equal(t, sessionID, session.SessionID)
	assert.Equal(t, contentHash, session.ContentHash)
	assert.Equal(t, HealthStatusHealthy, session.Status)

	// Check metrics updated
	metrics := monitor.GetMetrics()
	assert.Equal(t, int64(1), metrics.TotalSessions)
	assert.Equal(t, int64(1), metrics.ActiveSessions)

	// Unregister session
	monitor.UnregisterSession(sessionID)

	// Check session no longer exists
	_, err = monitor.GetSessionHealth(sessionID)
	assert.Error(t, err)

	// Check metrics updated
	metrics = monitor.GetMetrics()
	assert.Equal(t, int64(1), metrics.TotalSessions)  // Total doesn't decrease
	assert.Equal(t, int64(0), metrics.ActiveSessions) // Active decreases
}

func TestStreamingHealthMonitor_RecordSegmentProduced(t *testing.T) {
	logger := hclog.NewNullLogger()
	monitor := NewStreamingHealthMonitor(logger)

	sessionID := "test-session-1"
	monitor.RegisterSession(sessionID, "abc123")

	// Record successful segment
	encodeTime := 2 * time.Second
	segmentSize := int64(1024 * 1024) // 1MB

	monitor.RecordSegmentProduced(sessionID, 0, encodeTime, segmentSize)

	// Check session health
	session, err := monitor.GetSessionHealth(sessionID)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), session.SegmentsProduced)
	assert.Equal(t, int64(0), session.SegmentsFailed)
	assert.Equal(t, 0, session.ConsecutiveErrors)
	assert.Equal(t, HealthStatusHealthy, session.Status)
	assert.Equal(t, encodeTime, session.AverageEncodeTime)
	assert.Equal(t, segmentSize, session.BytesProcessed)

	// Check global metrics
	metrics := monitor.GetMetrics()
	assert.Equal(t, int64(1), metrics.TotalSegments)
	assert.Equal(t, int64(0), metrics.FailedSegments)
	assert.Equal(t, segmentSize, metrics.TotalBytesProcessed)
	assert.Equal(t, encodeTime, metrics.AverageEncodeTime)
}

func TestStreamingHealthMonitor_RecordSegmentFailed(t *testing.T) {
	logger := hclog.NewNullLogger()
	monitor := NewStreamingHealthMonitor(logger)

	sessionID := "test-session-1"
	monitor.RegisterSession(sessionID, "abc123")

	// Record failed segment
	testError := assert.AnError
	monitor.RecordSegmentFailed(sessionID, 0, testError)

	// Check session health
	session, err := monitor.GetSessionHealth(sessionID)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), session.SegmentsProduced)
	assert.Equal(t, int64(1), session.SegmentsFailed)
	assert.Equal(t, 1, session.ConsecutiveErrors)
	assert.Equal(t, testError.Error(), session.LastError)

	// Check global metrics
	metrics := monitor.GetMetrics()
	assert.Equal(t, int64(0), metrics.TotalSegments)
	assert.Equal(t, int64(1), metrics.FailedSegments)
}

func TestStreamingHealthMonitor_RecordFFmpegProgress(t *testing.T) {
	logger := hclog.NewNullLogger()
	monitor := NewStreamingHealthMonitor(logger)

	sessionID := "test-session-1"
	monitor.RegisterSession(sessionID, "abc123")

	// Record FFmpeg progress
	progress := FFmpegProgress{
		Frame:    100,
		FPS:      30.0,
		Time:     5 * time.Second,
		Speed:    1.2,
		Progress: "continue",
	}

	monitor.RecordFFmpegProgress(sessionID, progress)

	// Check session health
	session, err := monitor.GetSessionHealth(sessionID)
	assert.NoError(t, err)
	assert.Equal(t, progress.FPS, session.CurrentFPS)
	assert.Equal(t, progress.Speed, session.CurrentSpeed)

	// Check global metrics
	metrics := monitor.GetMetrics()
	assert.Equal(t, progress.FPS, metrics.CurrentFPS)
}

func TestStreamingHealthMonitor_RecordError(t *testing.T) {
	logger := hclog.NewNullLogger()
	monitor := NewStreamingHealthMonitor(logger)

	sessionID := "test-session-1"
	monitor.RegisterSession(sessionID, "abc123")

	// Record different types of errors
	monitor.RecordError(sessionID, "ffmpeg", assert.AnError)
	monitor.RecordError(sessionID, "network", assert.AnError)
	monitor.RecordError(sessionID, "storage", assert.AnError)

	// Check metrics
	metrics := monitor.GetMetrics()
	assert.Equal(t, int64(1), metrics.FFmpegErrors)
	assert.Equal(t, int64(1), metrics.NetworkErrors)
	assert.Equal(t, int64(1), metrics.StorageErrors)
}

func TestStreamingHealthMonitor_HealthStatusTransitions(t *testing.T) {
	logger := hclog.NewNullLogger()
	monitor := NewStreamingHealthMonitor(logger)

	sessionID := "test-session-1"
	monitor.RegisterSession(sessionID, "abc123")

	// Initially healthy
	session, _ := monitor.GetSessionHealth(sessionID)
	assert.Equal(t, HealthStatusHealthy, session.Status)

	// Record some errors to transition to degraded
	monitor.RecordSegmentFailed(sessionID, 0, assert.AnError)
	monitor.RecordSegmentFailed(sessionID, 1, assert.AnError)
	monitor.RecordSegmentFailed(sessionID, 2, assert.AnError)

	session, _ = monitor.GetSessionHealth(sessionID)
	assert.Equal(t, HealthStatusDegraded, session.Status)
	assert.Equal(t, 3, session.ConsecutiveErrors)

	// Record more errors to transition to unhealthy
	monitor.RecordSegmentFailed(sessionID, 3, assert.AnError)
	monitor.RecordSegmentFailed(sessionID, 4, assert.AnError)

	session, _ = monitor.GetSessionHealth(sessionID)
	assert.Equal(t, HealthStatusUnhealthy, session.Status)
	assert.Equal(t, 5, session.ConsecutiveErrors)

	// Record success to reset error count
	monitor.RecordSegmentProduced(sessionID, 5, 2*time.Second, 1024)

	session, _ = monitor.GetSessionHealth(sessionID)
	assert.Equal(t, HealthStatusHealthy, session.Status)
	assert.Equal(t, 0, session.ConsecutiveErrors)
}

func TestStreamingHealthMonitor_GetHealthReport(t *testing.T) {
	logger := hclog.NewNullLogger()
	monitor := NewStreamingHealthMonitor(logger)

	// Register multiple sessions
	monitor.RegisterSession("session-1", "hash-1")
	monitor.RegisterSession("session-2", "hash-2")

	// Record some activity
	monitor.RecordSegmentProduced("session-1", 0, 2*time.Second, 1024)
	monitor.RecordSegmentFailed("session-2", 0, assert.AnError)

	// Get health report
	report := monitor.GetHealthReport()

	assert.NotNil(t, report)
	assert.NotNil(t, report.Metrics)
	assert.Len(t, report.SessionsHealth, 2)
	assert.NotNil(t, report.SystemInfo)

	// Check session health in report
	session1 := report.SessionsHealth["session-1"]
	session2 := report.SessionsHealth["session-2"]

	assert.NotNil(t, session1)
	assert.NotNil(t, session2)
	assert.Equal(t, int64(1), session1.SegmentsProduced)
	assert.Equal(t, int64(1), session2.SegmentsFailed)
}

func TestStreamingHealthMonitor_AlertGeneration(t *testing.T) {
	logger := hclog.NewNullLogger()
	monitor := NewStreamingHealthMonitor(logger)

	// Set low thresholds for testing
	monitor.SetAlertThresholds(AlertThresholds{
		MaxConsecutiveErrors:  2,
		MaxStallDuration:      5 * time.Second,
		MinFPS:                20.0,
		MaxEncodeTime:         5 * time.Second,
		MaxFailureRate:        0.5,
		SessionTimeoutMinutes: 1,
	})

	sessionID := "test-session-1"
	monitor.RegisterSession(sessionID, "abc123")

	// Generate consecutive errors to trigger alert
	monitor.RecordSegmentFailed(sessionID, 0, assert.AnError)
	monitor.RecordSegmentFailed(sessionID, 1, assert.AnError)
	monitor.RecordSegmentFailed(sessionID, 2, assert.AnError)

	// Get health report to trigger alert generation
	report := monitor.GetHealthReport()

	// Should have alerts for consecutive errors
	assert.NotEmpty(t, report.ActiveAlerts)

	// Find the consecutive errors alert
	found := false
	for _, alert := range report.ActiveAlerts {
		if alert.Type == AlertTypeError && alert.SessionID == sessionID {
			found = true
			assert.Equal(t, AlertSeverityCritical, alert.Severity)
			assert.Contains(t, alert.Message, "consecutive errors")
			break
		}
	}
	assert.True(t, found, "Should have found consecutive errors alert")
}

func TestStreamingHealthMonitor_OverallHealthCalculation(t *testing.T) {
	logger := hclog.NewNullLogger()
	monitor := NewStreamingHealthMonitor(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start monitoring
	err := monitor.Start(ctx)
	require.NoError(t, err)

	// No sessions = healthy (after health check)
	monitor.performHealthCheck()
	assert.Equal(t, HealthStatusHealthy, monitor.GetOverallHealth())

	// Add healthy sessions
	monitor.RegisterSession("session-1", "hash-1")
	monitor.RegisterSession("session-2", "hash-2")

	// Trigger health check
	monitor.performHealthCheck()
	assert.Equal(t, HealthStatusHealthy, monitor.GetOverallHealth())

	// Make one session unhealthy
	for i := 0; i < 5; i++ {
		monitor.RecordSegmentFailed("session-1", i, assert.AnError)
	}

	// Trigger health check
	monitor.performHealthCheck()
	// Should still be healthy with only 25% unhealthy sessions (1 out of 4 sessions)
	// But we only have 2 sessions, so 50% unhealthy = degraded
	assert.Equal(t, HealthStatusDegraded, monitor.GetOverallHealth())

	// Make both sessions unhealthy
	for i := 0; i < 5; i++ {
		monitor.RecordSegmentFailed("session-2", i, assert.AnError)
	}

	// Trigger health check
	monitor.performHealthCheck()
	// Should be unhealthy with 100% unhealthy sessions
	assert.Equal(t, HealthStatusUnhealthy, monitor.GetOverallHealth())
}
