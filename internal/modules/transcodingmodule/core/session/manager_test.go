package session

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/database"
	plugins "github.com/mantonx/viewra/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Migrate the schema
	err = db.AutoMigrate(&database.TranscodeSession{})
	require.NoError(t, err)

	return db
}

func TestSessionManager_CreateSession(t *testing.T) {
	db := setupTestDB(t)
	logger := hclog.NewNullLogger()
	config := DefaultConfig()
	config.MaxConcurrentSessions = 5

	manager := NewSessionManager(db, logger, config)

	ctx := context.Background()
	req := &plugins.TranscodeRequest{
		SessionID:  "test-session-1",
		MediaID:    "media-123",
		Container:  "mp4",
		VideoCodec: "libx264",
		AudioCodec: "aac",
		Quality:    23,
	}

	// Test successful session creation
	session, err := manager.CreateSession(ctx, "test-provider", req)
	assert.NoError(t, err)
	assert.NotNil(t, session)
	assert.Equal(t, "test-session-1", session.ID)
	assert.Equal(t, StatusPending, session.Status)
	assert.Equal(t, float64(0), session.Progress)

	// Test duplicate session creation fails
	_, err = manager.CreateSession(ctx, "test-provider", req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session already exists")
}

func TestSessionManager_StateTransitions(t *testing.T) {
	db := setupTestDB(t)
	logger := hclog.NewNullLogger()
	config := DefaultConfig()

	manager := NewSessionManager(db, logger, config)

	ctx := context.Background()
	req := &plugins.TranscodeRequest{
		SessionID:  "test-session-2",
		MediaID:    "media-123",
		Container:  "mp4",
		VideoCodec: "libx264",
		AudioCodec: "aac",
		Quality:    23,
	}

	// Create session
	_, err := manager.CreateSession(ctx, "test-provider", req)
	require.NoError(t, err)

	// Test valid state transition: Pending -> Starting
	err = manager.TransitionSessionState("test-session-2", StatusStarting, nil)
	assert.NoError(t, err)

	// Verify state changed
	session, err := manager.GetSession("test-session-2")
	assert.NoError(t, err)
	assert.Equal(t, StatusStarting, session.Status)

	// Test valid state transition: Starting -> Running
	err = manager.TransitionSessionState("test-session-2", StatusRunning, nil)
	assert.NoError(t, err)

	// Test valid state transition: Running -> Complete
	result := &plugins.TranscodeResult{
		Success:     true,
		OutputPath:  "/path/to/output",
		ManifestURL: "/path/to/manifest.mpd",
	}
	err = manager.TransitionSessionState("test-session-2", StatusComplete, result)
	assert.NoError(t, err)

	// Verify session is removed from active sessions after completion
	_, err = manager.GetSession("test-session-2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestSessionManager_ConcurrencyLimits(t *testing.T) {
	db := setupTestDB(t)
	logger := hclog.NewNullLogger()
	config := DefaultConfig()
	config.MaxConcurrentSessions = 2 // Limit to 2 concurrent sessions

	manager := NewSessionManager(db, logger, config)

	ctx := context.Background()

	// Create first session
	req1 := &plugins.TranscodeRequest{
		SessionID: "test-session-1",
		MediaID:   "media-123",
		Container: "mp4",
	}
	_, err := manager.CreateSession(ctx, "test-provider", req1)
	assert.NoError(t, err)

	// Create second session
	req2 := &plugins.TranscodeRequest{
		SessionID: "test-session-2",
		MediaID:   "media-456",
		Container: "mp4",
	}
	_, err = manager.CreateSession(ctx, "test-provider", req2)
	assert.NoError(t, err)

	// Try to create third session - should fail due to limit
	req3 := &plugins.TranscodeRequest{
		SessionID: "test-session-3",
		MediaID:   "media-789",
		Container: "mp4",
	}
	_, err = manager.CreateSession(ctx, "test-provider", req3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum concurrent sessions reached")
}

func TestSessionManager_ProgressUpdates(t *testing.T) {
	db := setupTestDB(t)
	logger := hclog.NewNullLogger()
	config := DefaultConfig()

	manager := NewSessionManager(db, logger, config)

	ctx := context.Background()
	req := &plugins.TranscodeRequest{
		SessionID: "test-session-progress",
		MediaID:   "media-123",
		Container: "mp4",
	}

	// Create session
	session, err := manager.CreateSession(ctx, "test-provider", req)
	require.NoError(t, err)

	// Transition to running
	err = manager.TransitionSessionState("test-session-progress", StatusStarting, nil)
	require.NoError(t, err)
	err = manager.TransitionSessionState("test-session-progress", StatusRunning, nil)
	require.NoError(t, err)

	// Update progress
	progress := &plugins.TranscodingProgress{
		PercentComplete: 50.0,
		CurrentFrame:    1500,
		CurrentSpeed:    30.0,
		TimeElapsed:     50 * time.Second,
	}

	err = manager.UpdateSessionProgress("test-session-progress", progress)
	assert.NoError(t, err)

	// Verify progress was updated
	session, err = manager.GetSession("test-session-progress")
	assert.NoError(t, err)
	assert.Equal(t, 50.0, session.Progress)
}

func TestSessionManager_SessionStats(t *testing.T) {
	db := setupTestDB(t)
	logger := hclog.NewNullLogger()
	config := DefaultConfig()

	manager := NewSessionManager(db, logger, config)

	ctx := context.Background()

	// Create multiple sessions in different states
	for i := 0; i < 3; i++ {
		req := &plugins.TranscodeRequest{
			SessionID: fmt.Sprintf("test-session-%d", i),
			MediaID:   "media-123",
			Container: "mp4",
		}
		_, err := manager.CreateSession(ctx, "test-provider", req)
		require.NoError(t, err)
	}

	// Transition one to running
	err := manager.TransitionSessionState("test-session-0", StatusStarting, nil)
	require.NoError(t, err)
	err = manager.TransitionSessionState("test-session-0", StatusRunning, nil)
	require.NoError(t, err)

	// Get stats
	stats := manager.GetSessionStats()
	assert.Equal(t, 3, stats["active_sessions"])

	byStatus := stats["by_status"].(map[string]int)
	assert.Equal(t, 2, byStatus["pending"])
	assert.Equal(t, 1, byStatus["running"])

	byProvider := stats["by_provider"].(map[string]int)
	assert.Equal(t, 3, byProvider["test-provider"])
}

func TestSessionManager_CleanupStale(t *testing.T) {
	db := setupTestDB(t)
	logger := hclog.NewNullLogger()
	config := DefaultConfig()
	config.StaleSessionTimeout = 1 * time.Millisecond // Very short timeout for testing
	config.CleanupInterval = 0                        // Disable automatic cleanup

	manager := NewSessionManager(db, logger, config)

	ctx := context.Background()
	req := &plugins.TranscodeRequest{
		SessionID: "test-stale-session",
		MediaID:   "media-123",
		Container: "mp4",
	}

	// Create session
	session, err := manager.CreateSession(ctx, "test-provider", req)
	require.NoError(t, err)

	// Wait for timeout
	time.Sleep(10 * time.Millisecond)

	// Manually trigger cleanup
	manager.cleanupStaleSessions()

	// Session should be removed from active sessions
	_, err = manager.GetSession("test-stale-session")
	assert.Error(t, err)
}

func TestStateValidator_Transitions(t *testing.T) {
	validator := NewStateValidator()

	// Test valid transitions
	session := &Session{
		ID:     "test-session",
		Status: StatusPending,
	}

	err := validator.ValidateTransition(session, StatusStarting, nil)
	assert.NoError(t, err)

	session.Status = StatusStarting
	err = validator.ValidateTransition(session, StatusRunning, nil)
	assert.NoError(t, err)

	session.Status = StatusRunning
	err = validator.ValidateTransition(session, StatusComplete, nil)
	assert.NoError(t, err)

	// Test invalid transitions
	session.Status = StatusPending
	err = validator.ValidateTransition(session, StatusComplete, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transition not allowed")

	session.Status = StatusComplete
	err = validator.ValidateTransition(session, StatusRunning, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transition not allowed")
}

func TestStateValidator_SessionConsistency(t *testing.T) {
	validator := NewStateValidator()

	// Test valid session
	session := &Session{
		ID:        "test-session",
		Status:    StatusRunning,
		Progress:  50.0,
		StartTime: time.Now().Add(-1 * time.Minute),
		Handle:    &plugins.TranscodeHandle{},
		Process:   "dummy-process",
	}

	errors := validator.ValidateSessionConsistency(session)
	assert.Empty(t, errors)

	// Test invalid session - completed with incomplete progress
	session.Status = StatusComplete
	session.Progress = 50.0

	errors = validator.ValidateSessionConsistency(session)
	assert.NotEmpty(t, errors)
	assert.Contains(t, errors[0].Error(), "completed session should have 100% progress")

	// Test invalid session - failed without error
	session.Status = StatusFailed
	session.Error = nil

	errors = validator.ValidateSessionConsistency(session)
	assert.NotEmpty(t, errors)
	assert.Contains(t, errors[0].Error(), "failed session must have an error")
}
