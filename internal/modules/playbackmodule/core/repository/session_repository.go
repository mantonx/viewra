// Package repository provides data access layer for the playback module
package repository

import (
	"context"
	"time"

	"github.com/mantonx/viewra/internal/modules/playbackmodule/models"
	"gorm.io/gorm"
)

// SessionRepository handles session data access
type SessionRepository struct {
	db *gorm.DB
}

// NewSessionRepository creates a new session repository
func NewSessionRepository(db *gorm.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// Create creates a new playback session
func (r *SessionRepository) Create(ctx context.Context, session *models.PlaybackSession) error {
	return r.db.WithContext(ctx).Create(session).Error
}

// GetByID retrieves a session by ID
func (r *SessionRepository) GetByID(ctx context.Context, sessionID string) (*models.PlaybackSession, error) {
	var session models.PlaybackSession
	err := r.db.WithContext(ctx).Where("id = ?", sessionID).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// Update updates a session
func (r *SessionRepository) Update(ctx context.Context, session *models.PlaybackSession) error {
	return r.db.WithContext(ctx).Save(session).Error
}

// UpdateFields updates specific fields of a session
func (r *SessionRepository) UpdateFields(ctx context.Context, sessionID string, updates map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&models.PlaybackSession{}).
		Where("id = ?", sessionID).
		Updates(updates).Error
}

// Delete deletes a session
func (r *SessionRepository) Delete(ctx context.Context, sessionID string) error {
	return r.db.WithContext(ctx).Where("id = ?", sessionID).Delete(&models.PlaybackSession{}).Error
}

// GetActiveByUser retrieves active sessions for a user
func (r *SessionRepository) GetActiveByUser(ctx context.Context, userID string) ([]*models.PlaybackSession, error) {
	var sessions []*models.PlaybackSession
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND state IN ?", userID, []string{"playing", "paused"}).
		Order("last_activity DESC").
		Find(&sessions).Error
	return sessions, err
}

// GetByMediaFile retrieves sessions for a media file
func (r *SessionRepository) GetByMediaFile(ctx context.Context, mediaFileID string) ([]*models.PlaybackSession, error) {
	var sessions []*models.PlaybackSession
	err := r.db.WithContext(ctx).
		Where("media_file_id = ?", mediaFileID).
		Order("start_time DESC").
		Find(&sessions).Error
	return sessions, err
}

// CleanupStale removes stale sessions
func (r *SessionRepository) CleanupStale(ctx context.Context, staleDuration time.Duration) error {
	cutoff := time.Now().Add(-staleDuration)
	return r.db.WithContext(ctx).
		Where("last_activity < ? AND state IN ?", cutoff, []string{"playing", "paused"}).
		Update("state", "abandoned").Error
}

// GetRecentSessions retrieves recent sessions with optional filtering
func (r *SessionRepository) GetRecentSessions(ctx context.Context, userID string, limit int) ([]*models.PlaybackSession, error) {
	query := r.db.WithContext(ctx)
	
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	
	var sessions []*models.PlaybackSession
	err := query.Order("start_time DESC").Limit(limit).Find(&sessions).Error
	return sessions, err
}

// GetSessionStats retrieves session statistics
func (r *SessionRepository) GetSessionStats(ctx context.Context, userID string, since time.Time) (map[string]interface{}, error) {
	var stats struct {
		TotalSessions   int64
		TotalWatchTime  int64
		UniqueMediaFiles int64
	}
	
	query := r.db.WithContext(ctx).Model(&models.PlaybackSession{}).
		Where("user_id = ? AND start_time >= ?", userID, since)
	
	// Total sessions
	query.Count(&stats.TotalSessions)
	
	// Total watch time
	query.Select("COALESCE(SUM(duration), 0)").Scan(&stats.TotalWatchTime)
	
	// Unique media files
	query.Distinct("media_file_id").Count(&stats.UniqueMediaFiles)
	
	return map[string]interface{}{
		"total_sessions":    stats.TotalSessions,
		"total_watch_time":  stats.TotalWatchTime,
		"unique_media_files": stats.UniqueMediaFiles,
	}, nil
}

// CreateEvent creates a session event
func (r *SessionRepository) CreateEvent(ctx context.Context, event *models.SessionEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

// GetSessionEvents retrieves events for a session
func (r *SessionRepository) GetSessionEvents(ctx context.Context, sessionID string) ([]*models.SessionEvent, error) {
	var events []*models.SessionEvent
	err := r.db.WithContext(ctx).
		Where("session_id = ?", sessionID).
		Order("timestamp ASC").
		Find(&events).Error
	return events, err
}