// Package core provides session tracking functionality for the playback module.
package session

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/models"
	"github.com/mantonx/viewra/internal/utils"
	"gorm.io/gorm"
)

// SessionTracker handles recording and updating playback sessions
type SessionTracker struct {
	logger hclog.Logger
	db     *gorm.DB
}

// NewSessionTracker creates a new session tracker
func NewSessionTracker(logger hclog.Logger, db *gorm.DB) *SessionTracker {
	return &SessionTracker{
		logger: logger,
		db:     db,
	}
}

// RecordSessionStart records the start of a playback session
func (st *SessionTracker) RecordSessionStart(session *PlaybackSession) error {
	// Convert in-memory session to database model
	dbSession := models.PlaybackSession{
		ID:            session.ID,
		MediaFileID:   session.MediaFileID,
		UserID:        session.UserID,
		DeviceID:      session.DeviceID,
		Method:        session.Method,
		TranscodeID:   session.TranscodeID,
		StartTime:     session.StartTime,
		LastActivity:  session.LastActivity,
		Position:      session.Position,
		Duration:      session.Duration,
		State:         session.State,
		IPAddress:     session.IPAddress,
		Location:      session.Location,
		UserAgent:     session.UserAgent,
		DeviceName:    session.DeviceName,
		DeviceType:    session.DeviceType,
		Browser:       session.Browser,
		OS:            session.OS,
		QualityPlayed: session.QualityPlayed,
		Bandwidth:     session.Bandwidth,
	}

	// Convert capabilities and debug info to JSON
	if len(session.Capabilities) > 0 {
		capsJSON, _ := json.Marshal(session.Capabilities)
		dbSession.Capabilities = string(capsJSON)
	}

	if len(session.DebugInfo) > 0 {
		debugJSON, _ := json.Marshal(session.DebugInfo)
		dbSession.DebugInfo = string(debugJSON)
	}

	// Save to database
	if err := st.db.Create(&dbSession).Error; err != nil {
		return fmt.Errorf("failed to record session start: %w", err)
	}

	// Record session start event
	event := models.SessionEvent{
		ID:        utils.GenerateUUID(),
		SessionID: session.ID,
		EventType: "start",
		EventTime: time.Now(),
		Position:  0,
		Data:      fmt.Sprintf(`{"method":"%s","device":"%s"}`, session.Method, session.DeviceType),
	}

	if err := st.db.Create(&event).Error; err != nil {
		st.logger.Error("Failed to record session start event", "error", err)
	}

	return nil
}

// UpdateSessionProgress updates the progress of an active session
func (st *SessionTracker) UpdateSessionProgress(sessionID string, position int64, state string) error {
	// Update session
	updates := map[string]interface{}{
		"position":      position,
		"state":         state,
		"last_activity": time.Now(),
	}

	if err := st.db.Model(&models.PlaybackSession{}).
		Where("id = ?", sessionID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update session progress: %w", err)
	}

	// Record progress event (throttled to avoid spam)
	st.recordProgressEvent(sessionID, position, state)

	return nil
}

// RecordSessionEnd records the end of a playback session
func (st *SessionTracker) RecordSessionEnd(sessionID string, finalPosition int64, endReason string) error {
	now := time.Now()
	// Update session with end time
	updates := map[string]interface{}{
		"end_time":      &now,
		"position":      finalPosition,
		"state":         "ended",
		"last_activity": now,
	}

	if err := st.db.Model(&models.PlaybackSession{}).
		Where("id = ?", sessionID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to record session end: %w", err)
	}

	// Record end event
	event := models.SessionEvent{
		ID:        utils.GenerateUUID(),
		SessionID: sessionID,
		EventType: "end",
		EventTime: time.Now(),
		Position:  finalPosition,
		Data:      fmt.Sprintf(`{"reason":"%s"}`, endReason),
	}

	if err := st.db.Create(&event).Error; err != nil {
		st.logger.Error("Failed to record session end event", "error", err)
	}

	return nil
}

// recordProgressEvent records a progress event (throttled)
func (st *SessionTracker) recordProgressEvent(sessionID string, position int64, state string) {
	// Check if we should record this progress event (throttle to every 30 seconds)
	var lastEvent models.SessionEvent
	err := st.db.Where("session_id = ? AND event_type = ?", sessionID, "progress").
		Order("event_time DESC").
		First(&lastEvent).Error

	if err == nil && time.Since(lastEvent.EventTime) < 30*time.Second {
		return // Skip this event to avoid spam
	}

	// Record progress event
	event := models.SessionEvent{
		ID:        utils.GenerateUUID(),
		SessionID: sessionID,
		EventType: "progress",
		EventTime: time.Now(),
		Position:  position,
		Data:      fmt.Sprintf(`{"state":"%s"}`, state),
	}

	if err := st.db.Create(&event).Error; err != nil {
		st.logger.Error("Failed to record progress event", "error", err)
	}
}

// GetSessionHistory returns the session history for a user
func (st *SessionTracker) GetSessionHistory(userID string, limit int) ([]models.PlaybackSession, error) {
	var sessions []models.PlaybackSession

	query := st.db.Where("user_id = ?", userID).
		Order("start_time DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&sessions).Error; err != nil {
		return nil, fmt.Errorf("failed to get session history: %w", err)
	}

	return sessions, nil
}

// GetActiveSessionsForUser returns active sessions for a user
func (st *SessionTracker) GetActiveSessionsForUser(userID string) ([]models.PlaybackSession, error) {
	var sessions []models.PlaybackSession

	// Consider a session active if it was updated in the last 30 minutes
	cutoff := time.Now().Add(-30 * time.Minute)

	err := st.db.Where("user_id = ? AND last_activity > ? AND end_time IS NULL", userID, cutoff).
		Find(&sessions).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get active sessions: %w", err)
	}

	return sessions, nil
}
