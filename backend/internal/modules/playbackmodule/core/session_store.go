package core

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/pkg/plugins"
	"gorm.io/gorm"
)

// SessionStore provides unified session management for all transcoding providers
type SessionStore struct {
	db     *gorm.DB
	logger hclog.Logger
}

// NewSessionStore creates a new session store
func NewSessionStore(db *gorm.DB, logger hclog.Logger) *SessionStore {
	return &SessionStore{
		db:     db,
		logger: logger.Named("session-store"),
	}
}

// CreateSession creates a new transcoding session
func (s *SessionStore) CreateSession(provider string, req *plugins.TranscodeRequest) (*database.TranscodeSession, error) {
	session := &database.TranscodeSession{
		ID:           s.generateSessionID(),
		Provider:     provider,
		Status:       database.TranscodeStatusQueued,
		Request:      req,
		StartTime:    time.Now(),
		LastAccessed: time.Now(),
	}

	// Generate directory path
	container := req.Container
	if container == "" {
		container = "mp4" // default
	}
	session.DirectoryPath = s.generateSessionDirectory(container, provider, session.ID)

	if err := s.db.Create(session).Error; err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	s.logger.Info("created transcoding session",
		"session_id", session.ID,
		"provider", provider,
		"directory", session.DirectoryPath)

	return session, nil
}

// generateSessionID creates a unique session ID
func (s *SessionStore) generateSessionID() string {
	return uuid.New().String()
}

// generateSessionDirectory creates the directory path for a session
func (s *SessionStore) generateSessionDirectory(container, provider, sessionID string) string {
	// Format: [container]_[provider]_[sessionID]
	return fmt.Sprintf("/app/viewra-data/transcoding/%s_%s_%s", container, provider, sessionID)
}

// GetSession retrieves a session by ID
func (s *SessionStore) GetSession(sessionID string) (*database.TranscodeSession, error) {
	var session database.TranscodeSession
	if err := s.db.Where("id = ?", sessionID).First(&session).Error; err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	// Update last accessed time
	s.db.Model(&session).Update("last_accessed", time.Now())

	return &session, nil
}

// UpdateProgress updates session progress
func (s *SessionStore) UpdateProgress(sessionID string, progress *plugins.TranscodingProgress) error {
	updates := map[string]interface{}{
		"progress":      progress,
		"status":        database.TranscodeStatusRunning,
		"last_accessed": time.Now(),
	}

	if err := s.db.Model(&database.TranscodeSession{}).Where("id = ?", sessionID).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update progress: %w", err)
	}

	return nil
}

// CompleteSession marks a session as completed
func (s *SessionStore) CompleteSession(sessionID string, result *plugins.TranscodeResult) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":        database.TranscodeStatusCompleted,
		"result":        result,
		"end_time":      &now,
		"last_accessed": now,
	}

	if err := s.db.Model(&database.TranscodeSession{}).Where("id = ?", sessionID).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to complete session: %w", err)
	}

	s.logger.Info("session completed", "session_id", sessionID)
	return nil
}

// FailSession marks a session as failed
func (s *SessionStore) FailSession(sessionID string, err error) error {
	now := time.Now()
	result := &plugins.TranscodeResult{
		Success: false,
		Error:   err.Error(),
	}

	updates := map[string]interface{}{
		"status":        database.TranscodeStatusFailed,
		"result":        result,
		"end_time":      &now,
		"last_accessed": now,
	}

	if dbErr := s.db.Model(&database.TranscodeSession{}).Where("id = ?", sessionID).Updates(updates).Error; dbErr != nil {
		return fmt.Errorf("failed to update session: %w", dbErr)
	}

	s.logger.Error("session failed", "session_id", sessionID, "error", err)
	return nil
}

// ListProviderSessions lists all sessions for a provider
func (s *SessionStore) ListProviderSessions(provider string, filter SessionFilter) ([]*database.TranscodeSession, error) {
	query := s.db.Where("provider = ?", provider)

	// Apply filters
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if !filter.Since.IsZero() {
		query = query.Where("start_time >= ?", filter.Since)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}

	// Default ordering
	query = query.Order("start_time DESC")

	var sessions []*database.TranscodeSession
	if err := query.Find(&sessions).Error; err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	return sessions, nil
}

// GetProviderStats returns statistics for a provider
func (s *SessionStore) GetProviderStats(provider string) (*ProviderStats, error) {
	stats := &ProviderStats{
		Provider: provider,
	}

	// Count active sessions
	s.db.Model(&database.TranscodeSession{}).
		Where("provider = ? AND status IN ?", provider, []string{"queued", "running"}).
		Count(&stats.ActiveSessions)

	// Count completed sessions
	s.db.Model(&database.TranscodeSession{}).
		Where("provider = ? AND status = ?", provider, "completed").
		Count(&stats.CompletedSessions)

	// Count failed sessions
	s.db.Model(&database.TranscodeSession{}).
		Where("provider = ? AND status = ?", provider, "failed").
		Count(&stats.FailedSessions)

	// Get total bytes processed
	var result struct {
		TotalBytes int64
	}
	s.db.Model(&database.TranscodeSession{}).
		Select("SUM((result->>'bytes_written')::bigint) as total_bytes").
		Where("provider = ? AND status = ? AND result IS NOT NULL", provider, "completed").
		Scan(&result)
	stats.TotalBytesProcessed = result.TotalBytes

	return stats, nil
}

// CleanupExpiredSessions removes expired sessions based on retention policy
func (s *SessionStore) CleanupExpiredSessions(policy RetentionPolicy) (int, error) {
	cutoffTime := time.Now().Add(-time.Duration(policy.RetentionHours) * time.Hour)

	// Find sessions to cleanup
	var sessions []*database.TranscodeSession
	if err := s.db.Where("last_accessed < ? AND status IN ?", cutoffTime, []string{"completed", "failed"}).
		Find(&sessions).Error; err != nil {
		return 0, fmt.Errorf("failed to find expired sessions: %w", err)
	}

	// Delete expired sessions
	if len(sessions) > 0 {
		sessionIDs := make([]string, len(sessions))
		for i, session := range sessions {
			sessionIDs[i] = session.ID
		}

		if err := s.db.Where("id IN ?", sessionIDs).Delete(&database.TranscodeSession{}).Error; err != nil {
			return 0, fmt.Errorf("failed to delete sessions: %w", err)
		}

		s.logger.Info("cleaned up expired sessions", "count", len(sessions))
	}

	return len(sessions), nil
}

// RemoveProviderSessions removes all sessions for a provider
func (s *SessionStore) RemoveProviderSessions(provider string) error {
	if err := s.db.Where("provider = ?", provider).Delete(&database.TranscodeSession{}).Error; err != nil {
		return fmt.Errorf("failed to remove provider sessions: %w", err)
	}

	s.logger.Info("removed all sessions for provider", "provider", provider)
	return nil
}

// GetActiveSessions returns all active sessions across all providers
func (s *SessionStore) GetActiveSessions() ([]*database.TranscodeSession, error) {
	var sessions []*database.TranscodeSession
	if err := s.db.Where("status IN ?", []string{"queued", "running"}).
		Order("start_time DESC").
		Find(&sessions).Error; err != nil {
		return nil, fmt.Errorf("failed to get active sessions: %w", err)
	}

	return sessions, nil
}

// Types for session management

type SessionFilter struct {
	Status string
	Since  time.Time
	Limit  int
}

type ProviderStats struct {
	Provider            string
	ActiveSessions      int64
	CompletedSessions   int64
	FailedSessions      int64
	TotalBytesProcessed int64
}

type RetentionPolicy struct {
	RetentionHours     int   // Default retention in hours
	ExtendedHours      int   // Extended retention for smaller files
	MaxTotalSizeGB     int64 // Maximum total size in GB
	LargeFileThreshold int64 // Threshold for "large" files in bytes
}
