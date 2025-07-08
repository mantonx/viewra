// Package repository provides data access layer for the transcoding module
package repository

import (
	"context"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"gorm.io/gorm"
)

// TranscodeRepository handles transcode session data access
type TranscodeRepository struct {
	db *gorm.DB
}

// NewTranscodeRepository creates a new transcode repository
func NewTranscodeRepository(db *gorm.DB) *TranscodeRepository {
	return &TranscodeRepository{db: db}
}

// Create creates a new transcode session record
func (r *TranscodeRepository) Create(ctx context.Context, session *database.TranscodeSession) error {
	return r.db.WithContext(ctx).Create(session).Error
}

// GetByID retrieves a session by ID
func (r *TranscodeRepository) GetByID(ctx context.Context, sessionID string) (*database.TranscodeSession, error) {
	var session database.TranscodeSession
	err := r.db.WithContext(ctx).Where("id = ?", sessionID).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// GetByContentHash retrieves sessions by content hash
func (r *TranscodeRepository) GetByContentHash(ctx context.Context, contentHash string) ([]*database.TranscodeSession, error) {
	var sessions []*database.TranscodeSession
	err := r.db.WithContext(ctx).
		Where("content_hash = ?", contentHash).
		Order("created_at DESC").
		Find(&sessions).Error
	return sessions, err
}

// Update updates a transcode session
func (r *TranscodeRepository) Update(ctx context.Context, session *database.TranscodeSession) error {
	return r.db.WithContext(ctx).Save(session).Error
}

// UpdateFields updates specific fields of a session
func (r *TranscodeRepository) UpdateFields(ctx context.Context, sessionID string, updates map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&database.TranscodeSession{}).
		Where("id = ?", sessionID).
		Updates(updates).Error
}

// Delete deletes a session
func (r *TranscodeRepository) Delete(ctx context.Context, sessionID string) error {
	return r.db.WithContext(ctx).Where("id = ?", sessionID).Delete(&database.TranscodeSession{}).Error
}

// GetActiveSessions retrieves all active sessions
func (r *TranscodeRepository) GetActiveSessions(ctx context.Context) ([]*database.TranscodeSession, error) {
	var sessions []*database.TranscodeSession
	err := r.db.WithContext(ctx).
		Where("status IN ?", []string{"pending", "processing"}).
		Order("created_at ASC").
		Find(&sessions).Error
	return sessions, err
}

// GetSessionsByMediaFile retrieves sessions for a media file
func (r *TranscodeRepository) GetSessionsByMediaFile(ctx context.Context, mediaFileID string) ([]*database.TranscodeSession, error) {
	var sessions []*database.TranscodeSession
	err := r.db.WithContext(ctx).
		Where("media_file_id = ?", mediaFileID).
		Order("created_at DESC").
		Find(&sessions).Error
	return sessions, err
}

// GetRecentSessions retrieves recent sessions
func (r *TranscodeRepository) GetRecentSessions(ctx context.Context, limit int) ([]*database.TranscodeSession, error) {
	var sessions []*database.TranscodeSession
	err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Limit(limit).
		Find(&sessions).Error
	return sessions, err
}

// CleanupOldSessions removes sessions older than the specified duration
func (r *TranscodeRepository) CleanupOldSessions(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	return r.db.WithContext(ctx).
		Where("created_at < ? AND status IN ?", cutoff, []string{"completed", "failed"}).
		Delete(&database.TranscodeSession{}).Error
}

// GetSessionStats retrieves session statistics
func (r *TranscodeRepository) GetSessionStats(ctx context.Context) (map[string]interface{}, error) {
	var stats struct {
		TotalSessions      int64
		ActiveSessions     int64
		CompletedSessions  int64
		FailedSessions     int64
	}
	
	// Total sessions
	r.db.WithContext(ctx).Model(&database.TranscodeSession{}).Count(&stats.TotalSessions)
	
	// Active sessions
	r.db.WithContext(ctx).Model(&database.TranscodeSession{}).
		Where("status IN ?", []string{"pending", "processing"}).
		Count(&stats.ActiveSessions)
	
	// Completed sessions
	r.db.WithContext(ctx).Model(&database.TranscodeSession{}).
		Where("status = ?", "completed").
		Count(&stats.CompletedSessions)
	
	// Failed sessions
	r.db.WithContext(ctx).Model(&database.TranscodeSession{}).
		Where("status = ?", "failed").
		Count(&stats.FailedSessions)
	
	return map[string]interface{}{
		"total_sessions":     stats.TotalSessions,
		"active_sessions":    stats.ActiveSessions,
		"completed_sessions": stats.CompletedSessions,
		"failed_sessions":    stats.FailedSessions,
	}, nil
}