// Package core provides the core functionality for the playback module.
package core

import (
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/types"
	"github.com/mantonx/viewra/internal/utils"
	"gorm.io/gorm"
)

// PlaybackSession represents an active playback session with analytics tracking
type PlaybackSession struct {
	ID           string    `json:"id"`
	MediaFileID  string    `json:"media_file_id"`
	UserID       string    `json:"user_id,omitempty"`
	DeviceID     string    `json:"device_id"`
	Method       string    `json:"method"` // direct, remux, transcode
	TranscodeID  string    `json:"transcode_id,omitempty"`
	StartTime    time.Time `json:"start_time"`
	LastActivity time.Time `json:"last_activity"`
	Position     int64     `json:"position"` // Current playback position in seconds
	Duration     int64     `json:"duration"` // Total duration in seconds
	State        string    `json:"state"`    // playing, paused, stopped
	StreamURL    string    `json:"stream_url"`
	DirectURL    string    `json:"direct_url,omitempty"`

	// Analytics and device tracking fields
	IPAddress     string                 `json:"ip_address,omitempty"`
	Location      string                 `json:"location,omitempty"` // City, Country
	UserAgent     string                 `json:"user_agent,omitempty"`
	DeviceName    string                 `json:"device_name,omitempty"`    // e.g., "Chrome on Windows"
	DeviceType    string                 `json:"device_type,omitempty"`    // desktop, mobile, tablet, tv
	Browser       string                 `json:"browser,omitempty"`        // Chrome, Firefox, Safari, etc.
	OS            string                 `json:"os,omitempty"`             // Windows, macOS, Linux, iOS, Android
	Capabilities  map[string]bool        `json:"capabilities,omitempty"`   // codec support, etc.
	QualityPlayed string                 `json:"quality_played,omitempty"` // 1080p, 720p, etc.
	Bandwidth     int64                  `json:"bandwidth,omitempty"`      // Estimated bandwidth in bps
	DebugInfo     map[string]interface{} `json:"debug_info,omitempty"`     // Additional debug data

	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// SessionManager manages active playback sessions
type SessionManager struct {
	logger   hclog.Logger
	db       *gorm.DB
	sessions map[string]*PlaybackSession
	mu       sync.RWMutex
}

// NewSessionManager creates a new session manager
func NewSessionManager(logger hclog.Logger, db *gorm.DB) *SessionManager {
	sm := &SessionManager{
		logger:   logger,
		db:       db,
		sessions: make(map[string]*PlaybackSession),
	}

	// Start cleanup routine
	go sm.cleanupRoutine()

	return sm
}

// CreateSession creates a new playback session
func (sm *SessionManager) CreateSession(mediaFileID, userID, deviceID, method string) (*PlaybackSession, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Generate session ID
	sessionID := utils.GenerateUUID()

	session := &PlaybackSession{
		ID:           sessionID,
		MediaFileID:  mediaFileID,
		UserID:       userID,
		DeviceID:     deviceID,
		Method:       method,
		StartTime:    time.Now(),
		LastActivity: time.Now(),
		State:        "playing",
		Position:     0,
		Capabilities: make(map[string]bool),
		DebugInfo:    make(map[string]interface{}),
		Metadata:     make(map[string]interface{}),
	}

	sm.sessions[sessionID] = session

	sm.logger.Info("Created playback session",
		"sessionID", sessionID,
		"mediaFileID", mediaFileID,
		"method", method)

	// Update play statistics in database
	go sm.updatePlayStatistics(mediaFileID)

	return session, nil
}

// CreateSessionWithAnalytics creates a new playback session with device analytics
func (sm *SessionManager) CreateSessionWithAnalytics(mediaFileID, userID, deviceID, method string, analytics *types.DeviceAnalytics) (*PlaybackSession, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Generate session ID
	sessionID := utils.GenerateUUID()

	session := &PlaybackSession{
		ID:           sessionID,
		MediaFileID:  mediaFileID,
		UserID:       userID,
		DeviceID:     deviceID,
		Method:       method,
		StartTime:    time.Now(),
		LastActivity: time.Now(),
		State:        "playing",
		Position:     0,
		Capabilities: make(map[string]bool),
		DebugInfo:    make(map[string]interface{}),
		Metadata:     make(map[string]interface{}),
	}

	// Apply analytics data if provided
	if analytics != nil {
		session.IPAddress = analytics.IPAddress
		session.Location = analytics.Location
		session.UserAgent = analytics.UserAgent
		session.DeviceName = analytics.DeviceName
		session.DeviceType = analytics.DeviceType
		session.Browser = analytics.Browser
		session.OS = analytics.OS

		if analytics.Capabilities != nil {
			session.Capabilities = analytics.Capabilities
		}

		session.QualityPlayed = analytics.QualityPlayed
		session.Bandwidth = analytics.Bandwidth

		if analytics.DebugInfo != nil {
			session.DebugInfo = analytics.DebugInfo
		}
	}

	sm.sessions[sessionID] = session

	sm.logger.Info("Created playback session with analytics",
		"sessionID", sessionID,
		"mediaFileID", mediaFileID,
		"method", method,
		"deviceType", session.DeviceType,
		"browser", session.Browser)

	// Update play statistics in database
	go sm.updatePlayStatistics(mediaFileID)

	// Store analytics data for future dashboard
	go sm.storeAnalytics(session)

	return session, nil
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(sessionID string) (*PlaybackSession, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

// UpdateSession updates session information
func (sm *SessionManager) UpdateSession(sessionID string, updates map[string]interface{}) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Update last activity
	session.LastActivity = time.Now()

	// Apply updates
	for key, value := range updates {
		switch key {
		case "position":
			if pos, ok := value.(int64); ok {
				session.Position = pos
			}
		case "state":
			if state, ok := value.(string); ok {
				session.State = state
			}
		case "stream_url":
			if url, ok := value.(string); ok {
				session.StreamURL = url
			}
		case "direct_url":
			if url, ok := value.(string); ok {
				session.DirectURL = url
			}
		case "transcode_id":
			if id, ok := value.(string); ok {
				session.TranscodeID = id
			}
		case "duration":
			if dur, ok := value.(int64); ok {
				session.Duration = dur
			}
		default:
			// Store in metadata
			session.Metadata[key] = value
		}
	}

	sm.logger.Debug("Updated session",
		"sessionID", sessionID,
		"updates", updates)

	return nil
}

// EndSession ends a playback session
func (sm *SessionManager) EndSession(sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Update completion status based on media type
	if session.Position > 0 && session.Duration > 0 {
		// Use media-aware completion logic
		go sm.markAsCompletedIfEligible(session.MediaFileID, session.UserID, session.Position, session.Duration)
	}

	// Update last position for resume
	go sm.savePlaybackPosition(session.MediaFileID, session.UserID, session.Position)

	delete(sm.sessions, sessionID)

	sm.logger.Info("Ended playback session",
		"sessionID", sessionID,
		"duration", time.Since(session.StartTime))

	return nil
}

// GetActiveSessions returns all active sessions
func (sm *SessionManager) GetActiveSessions() []*PlaybackSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make([]*PlaybackSession, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		sessions = append(sessions, session)
	}

	return sessions
}

// GetSessionsByMediaFile returns all sessions for a specific media file
func (sm *SessionManager) GetSessionsByMediaFile(mediaFileID string) []*PlaybackSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var sessions []*PlaybackSession
	for _, session := range sm.sessions {
		if session.MediaFileID == mediaFileID {
			sessions = append(sessions, session)
		}
	}

	return sessions
}

// cleanupRoutine periodically cleans up inactive sessions
func (sm *SessionManager) cleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		sm.cleanupInactiveSessions()
	}
}

// cleanupInactiveSessions removes sessions that have been inactive
func (sm *SessionManager) cleanupInactiveSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	inactiveThreshold := 30 * time.Minute
	now := time.Now()

	for sessionID, session := range sm.sessions {
		if now.Sub(session.LastActivity) > inactiveThreshold {
			sm.logger.Info("Cleaning up inactive session",
				"sessionID", sessionID,
				"lastActivity", session.LastActivity)

			// Save position before cleanup
			if session.Position > 0 {
				go sm.savePlaybackPosition(session.MediaFileID, session.UserID, session.Position)
			}

			delete(sm.sessions, sessionID)
		}
	}
}

// CleanupSessions cleans up all sessions (called on shutdown)
func (sm *SessionManager) CleanupSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, session := range sm.sessions {
		// Save playback positions
		if session.Position > 0 {
			sm.savePlaybackPosition(session.MediaFileID, session.UserID, session.Position)
		}
	}

	sm.sessions = make(map[string]*PlaybackSession)
	sm.logger.Info("Cleaned up all sessions")
}

// updatePlayStatistics updates play count for a media file
func (sm *SessionManager) updatePlayStatistics(mediaFileID string) {
	if sm.db == nil {
		return
	}

	err := sm.db.Model(&database.MediaFile{}).
		Where("id = ?", mediaFileID).
		Updates(map[string]interface{}{
			"play_count":  gorm.Expr("play_count + ?", 1),
			"last_played": time.Now(),
		}).Error

	if err != nil {
		sm.logger.Error("Failed to update play statistics",
			"mediaFileID", mediaFileID,
			"error", err)
	}
}

// markAsCompletedIfEligible marks a media file as completed based on media-aware logic
func (sm *SessionManager) markAsCompletedIfEligible(mediaFileID, userID string, position, duration int64) {
	if sm.db == nil {
		return
	}

	// Get media file to determine type
	var mediaFile database.MediaFile
	if err := sm.db.First(&mediaFile, "id = ?", mediaFileID).Error; err != nil {
		sm.logger.Error("Failed to get media file for completion check", "error", err)
		return
	}

	// Calculate completion percentage
	completionPercentage := float64(0)
	if duration > 0 {
		completionPercentage = (float64(position) / float64(duration)) * 100
	}

	// Use media-aware completion logic
	isCompleted := sm.isPlaybackCompleted(mediaFile.MediaType, completionPercentage, position, duration)

	if isCompleted {
		// This is a simplified version - in a real system you'd have a user_media_progress table
		err := sm.db.Model(&database.MediaFile{}).
			Where("id = ?", mediaFileID).
			Update("watched", true).Error

		if err != nil {
			sm.logger.Error("Failed to mark as completed",
				"mediaFileID", mediaFileID,
				"mediaType", string(mediaFile.MediaType),
				"completionPercentage", completionPercentage,
				"error", err)
		} else {
			sm.logger.Debug("Marked media as completed",
				"mediaFileID", mediaFileID,
				"mediaType", string(mediaFile.MediaType),
				"completionPercentage", completionPercentage)
		}
	}
}

// isPlaybackCompleted determines if playback is considered "completed" based on media type
func (sm *SessionManager) isPlaybackCompleted(mediaType database.MediaType, completionPercentage float64, position, duration int64) bool {
	switch mediaType {
	case database.MediaTypeTrack:
		// For music, consider completed if:
		// 1. Played > 70% of the song, OR
		// 2. Played at least 30 seconds and > 50% (for short songs), OR
		// 3. Played at least 90 seconds (for very long songs/podcasts)
		if completionPercentage >= 70.0 {
			return true
		}
		if position >= 30 && completionPercentage >= 50.0 {
			return true
		}
		if position >= 90 {
			return true
		}
		return false

	case database.MediaTypeMovie, database.MediaTypeEpisode:
		// For video content, use traditional 90% rule
		return completionPercentage >= 90.0

	default:
		// Default to 80% for unknown types
		return completionPercentage >= 80.0
	}
}

// savePlaybackPosition saves the current playback position for resume
func (sm *SessionManager) savePlaybackPosition(mediaFileID, userID string, position int64) {
	if sm.db == nil {
		return
	}

	// This is a simplified version - in a real system you'd have a user_media_progress table
	// TODO: Implement proper playback position storage
	sm.logger.Debug("Saving playback position",
		"mediaFileID", mediaFileID,
		"userID", userID,
		"position", position)
}

// storeAnalytics stores session analytics data for future dashboard use
func (sm *SessionManager) storeAnalytics(session *PlaybackSession) {
	if sm.db == nil {
		return
	}

	// Create analytics record
	// This would typically go to a dedicated analytics table
	// For now, we'll store it as metadata
	analytics := map[string]interface{}{
		"session_id":     session.ID,
		"media_file_id":  session.MediaFileID,
		"user_id":        session.UserID,
		"device_id":      session.DeviceID,
		"method":         session.Method,
		"start_time":     session.StartTime,
		"ip_address":     session.IPAddress,
		"location":       session.Location,
		"user_agent":     session.UserAgent,
		"device_name":    session.DeviceName,
		"device_type":    session.DeviceType,
		"browser":        session.Browser,
		"os":             session.OS,
		"quality_played": session.QualityPlayed,
		"bandwidth":      session.Bandwidth,
	}

	// Store capabilities as JSON
	if len(session.Capabilities) > 0 {
		analytics["capabilities"] = session.Capabilities
	}

	// Store debug info
	if len(session.DebugInfo) > 0 {
		analytics["debug_info"] = session.DebugInfo
	}

	sm.logger.Info("Storing playback analytics",
		"sessionID", session.ID,
		"deviceType", session.DeviceType,
		"location", session.Location)

	// TODO: Create dedicated analytics table and store this data there
	// For now, log it for visibility
}
