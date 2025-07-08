// Package repository provides data access layer for the playback module
package repository

import (
	"context"
	"time"

	"github.com/mantonx/viewra/internal/modules/playbackmodule/models"
	"gorm.io/gorm"
)

// HistoryRepository handles playback history data access
type HistoryRepository struct {
	db *gorm.DB
}

// NewHistoryRepository creates a new history repository
func NewHistoryRepository(db *gorm.DB) *HistoryRepository {
	return &HistoryRepository{db: db}
}

// CreateHistory creates a new playback history entry
func (r *HistoryRepository) CreateHistory(ctx context.Context, history *models.PlaybackHistory) error {
	return r.db.WithContext(ctx).Create(history).Error
}

// GetUserHistory retrieves playback history for a user
func (r *HistoryRepository) GetUserHistory(ctx context.Context, userID string, limit, offset int) ([]*models.PlaybackHistory, error) {
	var history []*models.PlaybackHistory
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("played_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&history).Error
	return history, err
}

// GetMediaHistory retrieves playback history for a media file
func (r *HistoryRepository) GetMediaHistory(ctx context.Context, mediaFileID string) ([]*models.PlaybackHistory, error) {
	var history []*models.PlaybackHistory
	err := r.db.WithContext(ctx).
		Where("media_file_id = ?", mediaFileID).
		Order("played_at DESC").
		Find(&history).Error
	return history, err
}

// GetRecentHistory retrieves recent playback history
func (r *HistoryRepository) GetRecentHistory(ctx context.Context, userID string, since time.Time) ([]*models.PlaybackHistory, error) {
	var history []*models.PlaybackHistory
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND played_at >= ?", userID, since).
		Order("played_at DESC").
		Find(&history).Error
	return history, err
}

// UpdateProgress updates user media progress
func (r *HistoryRepository) UpdateProgress(ctx context.Context, progress *models.UserMediaProgress) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND media_file_id = ?", progress.UserID, progress.MediaFileID).
		Assign(progress).
		FirstOrCreate(progress).Error
}

// GetProgress retrieves user media progress
func (r *HistoryRepository) GetProgress(ctx context.Context, userID, mediaFileID string) (*models.UserMediaProgress, error) {
	var progress models.UserMediaProgress
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND media_file_id = ?", userID, mediaFileID).
		First(&progress).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &progress, err
}

// GetUserStats retrieves user playback statistics
func (r *HistoryRepository) GetUserStats(ctx context.Context, userID string) (*models.UserPlaybackStats, error) {
	var stats models.UserPlaybackStats
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		First(&stats).Error
	if err == gorm.ErrRecordNotFound {
		// Create new stats if not found
		stats = models.UserPlaybackStats{
			UserID: userID,
		}
		err = r.db.WithContext(ctx).Create(&stats).Error
	}
	return &stats, err
}

// UpdateUserStats updates user playback statistics
func (r *HistoryRepository) UpdateUserStats(ctx context.Context, stats *models.UserPlaybackStats) error {
	return r.db.WithContext(ctx).Save(stats).Error
}

// CreateAnalytics creates a new analytics entry
func (r *HistoryRepository) CreateAnalytics(ctx context.Context, analytics *models.PlaybackAnalytics) error {
	return r.db.WithContext(ctx).Create(analytics).Error
}

// GetAnalytics retrieves analytics for a time range
func (r *HistoryRepository) GetAnalytics(ctx context.Context, start, end time.Time) ([]*models.PlaybackAnalytics, error) {
	var analytics []*models.PlaybackAnalytics
	err := r.db.WithContext(ctx).
		Where("timestamp BETWEEN ? AND ?", start, end).
		Order("timestamp DESC").
		Find(&analytics).Error
	return analytics, err
}

// CreateInteraction creates a media interaction entry
func (r *HistoryRepository) CreateInteraction(ctx context.Context, interaction *models.MediaInteraction) error {
	return r.db.WithContext(ctx).Create(interaction).Error
}

// GetUserInteractions retrieves user interactions
func (r *HistoryRepository) GetUserInteractions(ctx context.Context, userID string, limit int) ([]*models.MediaInteraction, error) {
	var interactions []*models.MediaInteraction
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("interaction_time DESC").
		Limit(limit).
		Find(&interactions).Error
	return interactions, err
}

// GetUserPreferences retrieves user preferences
func (r *HistoryRepository) GetUserPreferences(ctx context.Context, userID string) (*models.UserPreferences, error) {
	var prefs models.UserPreferences
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		First(&prefs).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &prefs, err
}

// UpdateUserPreferences updates user preferences
func (r *HistoryRepository) UpdateUserPreferences(ctx context.Context, prefs *models.UserPreferences) error {
	return r.db.WithContext(ctx).
		Where("user_id = ?", prefs.UserID).
		Assign(prefs).
		FirstOrCreate(prefs).Error
}