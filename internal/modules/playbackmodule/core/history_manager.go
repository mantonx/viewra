// Package core provides the core functionality for the playback module.
package core

import (
	"fmt"

	"github.com/hashicorp/go-hclog"
	"gorm.io/gorm"
)

// HistoryManager coordinates session tracking, media history, and recommendations
type HistoryManager struct {
	logger                hclog.Logger
	db                    *gorm.DB
	sessionTracker        *SessionTracker
	mediaHistoryManager   *MediaHistoryManager // Replaces watchHistoryManager for better music support
	recommendationTracker *RecommendationTracker
}

// NewHistoryManager creates a new history manager
func NewHistoryManager(logger hclog.Logger, db *gorm.DB) *HistoryManager {
	return &HistoryManager{
		logger:                logger,
		db:                    db,
		sessionTracker:        NewSessionTracker(logger.Named("session-tracker"), db),
		mediaHistoryManager:   NewMediaHistoryManager(logger.Named("media-history"), db),
		recommendationTracker: NewRecommendationTracker(logger.Named("recommendation-tracker"), db),
	}
}

// RecordSessionStart records the start of a playback session
func (hm *HistoryManager) RecordSessionStart(session *PlaybackSession) error {
	if err := hm.sessionTracker.RecordSessionStart(session); err != nil {
		return fmt.Errorf("failed to record session start: %w", err)
	}

	// Track play interaction for recommendations
	go hm.recommendationTracker.TrackPlaybackEvent(
		session.ID, session.UserID, session.MediaFileID, "play", 0,
		map[string]interface{}{
			"method":      session.Method,
			"device_type": session.DeviceType,
		},
	)

	return nil
}

// UpdateSessionProgress updates session progress and tracks interaction
func (hm *HistoryManager) UpdateSessionProgress(sessionID, userID, mediaFileID string, position int64, state string) error {
	if err := hm.sessionTracker.UpdateSessionProgress(sessionID, position, state); err != nil {
		return fmt.Errorf("failed to update session progress: %w", err)
	}

	// Track interaction based on state change
	if state == "paused" {
		go hm.recommendationTracker.TrackPlaybackEvent(sessionID, userID, mediaFileID, "pause", position, nil)
	} else if state == "playing" {
		go hm.recommendationTracker.TrackPlaybackEvent(sessionID, userID, mediaFileID, "resume", position, nil)
	}

	return nil
}

// RecordSessionEnd records the end of a session and creates playback history
func (hm *HistoryManager) RecordSessionEnd(sessionID, userID, mediaFileID string, finalPosition, totalDuration int64, endReason string) error {
	// Record session end
	if err := hm.sessionTracker.RecordSessionEnd(sessionID, finalPosition, endReason); err != nil {
		return fmt.Errorf("failed to record session end: %w", err)
	}

	// Create media history entry (supports both video and audio)
	if err := hm.mediaHistoryManager.RecordPlaybackHistory(sessionID, userID, mediaFileID, finalPosition, totalDuration); err != nil {
		hm.logger.Error("Failed to record playback history", "error", err)
		// Don't return error as session end is more critical
	}

	// Track completion or stop event
	watchPercentage := float64(0)
	if totalDuration > 0 {
		watchPercentage = (float64(finalPosition) / float64(totalDuration)) * 100
	}

	eventType := "stop"
	if watchPercentage >= 90.0 {
		eventType = "complete"
	}

	go hm.recommendationTracker.TrackPlaybackEvent(
		sessionID, userID, mediaFileID, eventType, finalPosition,
		map[string]interface{}{
			"watch_percentage": watchPercentage,
			"end_reason":       endReason,
		},
	)

	return nil
}

// TrackInteraction tracks a user interaction for recommendations
func (hm *HistoryManager) TrackInteraction(userID, mediaFileID, interactionType string, value float64, metadata map[string]interface{}) error {
	return hm.recommendationTracker.TrackInteraction(userID, mediaFileID, interactionType, value, metadata)
}

// GetSessionTracker returns the session tracker component
func (hm *HistoryManager) GetSessionTracker() *SessionTracker {
	return hm.sessionTracker
}

// GetMediaHistoryManager returns the media history manager component
func (hm *HistoryManager) GetMediaHistoryManager() *MediaHistoryManager {
	return hm.mediaHistoryManager
}

// GetRecommendationTracker returns the recommendation tracker component
func (hm *HistoryManager) GetRecommendationTracker() *RecommendationTracker {
	return hm.recommendationTracker
}
