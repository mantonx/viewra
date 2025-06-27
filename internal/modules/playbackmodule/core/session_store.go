package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/database"
	plugins "github.com/mantonx/viewra/sdk"
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
	// Serialize request to JSON
	requestJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize request: %w", err)
	}

	session := &database.TranscodeSession{
		ID:           s.generateSessionID(),
		Provider:     provider,
		Status:       database.TranscodeStatusQueued,
		Request:      string(requestJSON),
		StartTime:    time.Now(),
		LastAccessed: time.Now(),
	}

	// Generate directory path
	container := req.Container
	if container == "" {
		container = "mp4" // default
	}
	session.DirectoryPath = s.generateSessionDirectory(container, provider, session.ID)

	// Generate content hash based on transcoding parameters
	// This ensures content-addressable URLs are available immediately
	session.ContentHash = s.generateContentHash(req)

	if err := s.db.Create(session).Error; err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	s.logger.Info("created transcoding session",
		"session_id", session.ID,
		"provider", provider,
		"directory", session.DirectoryPath,
		"content_hash", session.ContentHash)

	return session, nil
}

// generateSessionID creates a unique session ID
func (s *SessionStore) generateSessionID() string {
	return uuid.New().String()
}

// generateSessionDirectory creates the directory path for a session
func (s *SessionStore) generateSessionDirectory(container, provider, sessionID string) string {
	// Format: [container]_[provider]_[sessionID]
	// This is just the directory name, not the full path
	// The actual path will be set when the directory is created
	return fmt.Sprintf("%s_%s_%s", container, provider, sessionID)
}

// generateContentHash generates a deterministic content hash based on transcoding parameters
func (s *SessionStore) generateContentHash(req *plugins.TranscodeRequest) string {
	// Create a deterministic hash based on transcoding parameters
	// This ensures the same content parameters always generate the same hash
	// for content deduplication and CDN caching

	// Build hash input with all relevant parameters that affect output
	hashInput := fmt.Sprintf("%s_%s_%s_%s_%d_%d",
		req.MediaID,       // Media identifier
		req.Container,     // Output format (dash, hls, mp4)
		req.VideoCodec,    // Video codec
		req.AudioCodec,    // Audio codec
		req.Quality,       // Quality level
		req.SpeedPriority, // Speed/quality tradeoff
	)

	// Add resolution if specified
	if req.Resolution != nil {
		hashInput += fmt.Sprintf("_%dx%d", req.Resolution.Width, req.Resolution.Height)
	}

	// Add ABR flag
	if req.EnableABR {
		hashInput += "_abr"
	}

	// Add bitrate constraints if specified
	if req.VideoBitrate > 0 {
		hashInput += fmt.Sprintf("_vb%d", req.VideoBitrate)
	}
	if req.AudioBitrate > 0 {
		hashInput += fmt.Sprintf("_ab%d", req.AudioBitrate)
	}

	// Generate SHA256 hash
	hash := sha256.Sum256([]byte(hashInput))
	// Return full 64-character SHA256 hash for content-addressable storage
	return hex.EncodeToString(hash[:])
}

// GetSession retrieves a session by ID
func (s *SessionStore) GetSession(sessionID string) (*database.TranscodeSession, error) {
	var session database.TranscodeSession
	if err := s.db.Where("id = ?", sessionID).First(&session).Error; err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	// Don't update last_accessed automatically - only update for active operations
	// This was preventing cleanup of old sessions

	return &session, nil
}

// UpdateProgress updates session progress
func (s *SessionStore) UpdateProgress(sessionID string, progress *plugins.TranscodingProgress) error {
	// Serialize progress to JSON
	progressJSON, err := json.Marshal(progress)
	if err != nil {
		return fmt.Errorf("failed to serialize progress: %w", err)
	}

	updates := map[string]interface{}{
		"progress":      string(progressJSON),
		"status":        database.TranscodeStatusRunning,
		"last_accessed": time.Now(),
		"updated_at":    time.Now(),
	}

	if err := s.db.Model(&database.TranscodeSession{}).Where("id = ?", sessionID).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update progress: %w", err)
	}

	return nil
}

// CompleteSession marks a session as completed
func (s *SessionStore) CompleteSession(sessionID string, result *plugins.TranscodeResult) error {
	// Serialize result to JSON
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to serialize result: %w", err)
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":   database.TranscodeStatusCompleted,
		"end_time": &now,
		"result":   string(resultJSON),
	}

	if err := s.db.Model(&database.TranscodeSession{}).Where("id = ?", sessionID).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to complete session: %w", err)
	}

	s.logger.Info("completed session", "session_id", sessionID)
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

	// Use database-agnostic approach for SQLite compatibility
	// SQLite doesn't support ::bigint casting or complex JSON operations
	if s.db.Dialector.Name() == "sqlite" {
		// For SQLite, we'll skip the complex JSON query
		// This is acceptable for testing
		result.TotalBytes = 0
	} else {
		// PostgreSQL query with JSON operations
		s.db.Model(&database.TranscodeSession{}).
			Select("SUM((result->>'bytes_written')::bigint) as total_bytes").
			Where("provider = ? AND status = ? AND result IS NOT NULL", provider, "completed").
			Scan(&result)
	}
	stats.TotalBytesProcessed = result.TotalBytes

	return stats, nil
}

// CleanupExpiredSessions removes expired sessions based on retention policy
func (s *SessionStore) CleanupExpiredSessions(policy RetentionPolicy) (int, error) {
	cutoffTime := time.Now().Add(-time.Duration(policy.RetentionHours) * time.Hour)

	// Find sessions to cleanup
	var sessions []*database.TranscodeSession
	if err := s.db.Where("last_accessed < ? AND status IN ?", cutoffTime, []string{"completed", "failed", "cancelled"}).
		Find(&sessions).Error; err != nil {
		return 0, fmt.Errorf("failed to find expired sessions: %w", err)
	}

	// Delete expired sessions and their directories
	if len(sessions) > 0 {
		sessionIDs := make([]string, len(sessions))
		for i, session := range sessions {
			sessionIDs[i] = session.ID

			// Also remove the directory if it exists
			if session.DirectoryPath != "" {
				if err := os.RemoveAll(session.DirectoryPath); err != nil {
					s.logger.Warn("failed to remove session directory",
						"session_id", session.ID,
						"path", session.DirectoryPath,
						"error", err)
				} else {
					s.logger.Debug("removed session directory",
						"session_id", session.ID,
						"path", session.DirectoryPath)
				}
			}
		}

		if err := s.db.Where("id IN ?", sessionIDs).Delete(&database.TranscodeSession{}).Error; err != nil {
			return 0, fmt.Errorf("failed to delete sessions: %w", err)
		}

		s.logger.Info("cleaned up expired sessions", "count", len(sessions))

		// Log details about what was cleaned
		for _, session := range sessions {
			s.logger.Debug("cleaned session details",
				"session_id", session.ID,
				"status", session.Status,
				"directory_path", session.DirectoryPath,
				"has_dir", session.DirectoryPath != "")
		}
	}

	return len(sessions), nil
}

// CleanupStaleSessions marks running/queued sessions as failed if they've been stuck for too long
func (s *SessionStore) CleanupStaleSessions(maxAge time.Duration) (int, error) {
	cutoffTime := time.Now().Add(-maxAge)

	// Find stale running/queued sessions
	var staleSessions []*database.TranscodeSession
	if err := s.db.Where("last_accessed < ? AND status IN ?", cutoffTime, []string{"running", "queued"}).
		Find(&staleSessions).Error; err != nil {
		return 0, fmt.Errorf("failed to find stale sessions: %w", err)
	}

	// Mark them as failed
	if len(staleSessions) > 0 {
		sessionIDs := make([]string, len(staleSessions))
		for i, session := range staleSessions {
			sessionIDs[i] = session.ID
		}

		// Update status to failed with explanation
		updates := map[string]interface{}{
			"status":   "failed",
			"result":   `{"error": "Session timed out - no activity for too long"}`,
			"end_time": time.Now(),
		}

		if err := s.db.Model(&database.TranscodeSession{}).
			Where("id IN ?", sessionIDs).
			Updates(updates).Error; err != nil {
			return 0, fmt.Errorf("failed to update stale sessions: %w", err)
		}

		s.logger.Warn("marked stale sessions as failed", "count", len(staleSessions), "max_age", maxAge)
	}

	return len(staleSessions), nil
}

// UpdateSessionStatus updates the status of a session
func (s *SessionStore) UpdateSessionStatus(sessionID, status, result string) error {
	updates := map[string]interface{}{
		"status":        status,
		"last_accessed": time.Now(),
		"updated_at":    time.Now(),
	}

	// Only set result and end_time if result is provided
	if result != "" {
		updates["result"] = result
		updates["end_time"] = time.Now()
	}

	if err := s.db.Model(&database.TranscodeSession{}).
		Where("id = ?", sessionID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update session status: %w", err)
	}

	s.logger.Info("updated session status", "session_id", sessionID, "status", status)
	return nil
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

// ListActiveSessionsByContentHash returns active sessions with the specified content hash
func (s *SessionStore) ListActiveSessionsByContentHash(contentHash string) ([]*database.TranscodeSession, error) {
	var sessions []*database.TranscodeSession
	if err := s.db.Where("content_hash = ? AND status IN ?", contentHash, []string{"queued", "running"}).
		Order("start_time DESC").
		Find(&sessions).Error; err != nil {
		return nil, fmt.Errorf("failed to get active sessions by content hash: %w", err)
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
