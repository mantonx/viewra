// Package core provides media-aware history functionality for video and audio playback.
package history

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/models"
	"github.com/mantonx/viewra/internal/utils"
	"gorm.io/gorm"
)

// MediaHistoryManager manages user playback history for both video and audio content
type MediaHistoryManager struct {
	logger hclog.Logger
	db     *gorm.DB
}

// NewMediaHistoryManager creates a new media-aware history manager
func NewMediaHistoryManager(logger hclog.Logger, db *gorm.DB) *MediaHistoryManager {
	return &MediaHistoryManager{
		logger: logger,
		db:     db,
	}
}

// RecordPlaybackHistory creates a history entry when a session ends (video or audio)
func (mhm *MediaHistoryManager) RecordPlaybackHistory(sessionID, userID, mediaFileID string, playedDuration, totalDuration int64) error {
	// Calculate play percentage
	playPercentage := float64(0)
	if totalDuration > 0 {
		playPercentage = (float64(playedDuration) / float64(totalDuration)) * 100
	}

	// Get media file info for the history entry
	var mediaFile database.MediaFile
	if err := mhm.db.First(&mediaFile, "id = ?", mediaFileID).Error; err != nil {
		mhm.logger.Error("Failed to get media file for history", "mediaFileID", mediaFileID, "error", err)
		// Continue anyway with basic info
	}

	// Determine completion based on media type
	isCompleted := mhm.isPlaybackCompleted(mediaFile.MediaType, playPercentage, playedDuration, totalDuration)

	// Create playback history entry
	history := models.PlaybackHistory{
		ID:            utils.GenerateUUID(),
		UserID:        userID,
		MediaFileID:   mediaFileID,
		MediaType:     string(mediaFile.MediaType),
		PlayedAt:      time.Now(),
		Duration:      totalDuration,
		PlayedSeconds: playedDuration,
		Completed:     isCompleted,
		LastPosition:  playedDuration,
		Quality:       mhm.getQualityString(&mediaFile),
	}

	if err := mhm.db.Create(&history).Error; err != nil {
		return fmt.Errorf("failed to record playback history: %w", err)
	}

	// Update user preferences based on this playback
	go mhm.updateUserPreferences(userID, &mediaFile, playPercentage, playedDuration)

	mhm.logger.Info("Recorded playback history",
		"userID", userID,
		"mediaFileID", mediaFileID,
		"mediaType", string(mediaFile.MediaType),
		"playPercentage", playPercentage,
		"completed", isCompleted)

	return nil
}

// isPlaybackCompleted determines if playback is considered "completed" based on media type
func (mhm *MediaHistoryManager) isPlaybackCompleted(mediaType database.MediaType, playPercentage float64, playedDuration, totalDuration int64) bool {
	switch mediaType {
	case database.MediaTypeTrack:
		// For music, consider completed if:
		// 1. Played > 70% of the song, OR
		// 2. Played at least 30 seconds and > 50% (for short songs), OR
		// 3. Played at least 90 seconds (for very long songs/podcasts)
		if playPercentage >= 70.0 {
			return true
		}
		if playedDuration >= 30 && playPercentage >= 50.0 {
			return true
		}
		if playedDuration >= 90 {
			return true
		}
		return false

	case database.MediaTypeMovie, database.MediaTypeEpisode:
		// For video content, use traditional 90% rule
		return playPercentage >= 90.0

	default:
		// Default to 80% for unknown types
		return playPercentage >= 80.0
	}
}

// getQualityString returns appropriate quality description based on media type
func (mhm *MediaHistoryManager) getQualityString(mediaFile *database.MediaFile) string {
	switch mediaFile.MediaType {
	case database.MediaTypeTrack:
		// For audio, focus on audio quality
		if mediaFile.AudioCodec == "flac" || mediaFile.AudioCodec == "alac" {
			return "Lossless"
		}
		if mediaFile.AudioSampleRate >= 96000 {
			return "Hi-Res"
		}
		if mediaFile.AudioSampleRate >= 48000 {
			return "CD Quality"
		}
		if mediaFile.BitrateKbps >= 320 {
			return "High"
		}
		if mediaFile.BitrateKbps >= 192 {
			return "Medium"
		}
		return "Standard"

	case database.MediaTypeMovie, database.MediaTypeEpisode:
		// For video, use resolution
		if mediaFile.Resolution != "" {
			return mediaFile.Resolution
		}
		if mediaFile.VideoHeight >= 2160 {
			return "4K"
		}
		if mediaFile.VideoHeight >= 1080 {
			return "1080p"
		}
		if mediaFile.VideoHeight >= 720 {
			return "720p"
		}
		return "SD"

	default:
		return mediaFile.Resolution
	}
}

// updateUserPreferences updates user preferences based on playback behavior
func (mhm *MediaHistoryManager) updateUserPreferences(userID string, mediaFile *database.MediaFile, playPercentage float64, playedDuration int64) {
	if mediaFile == nil {
		return
	}

	// Get or create user preferences
	var prefs models.UserPreferences
	err := mhm.db.FirstOrCreate(&prefs, models.UserPreferences{UserID: userID}).Error
	if err != nil {
		mhm.logger.Error("Failed to get user preferences", "error", err)
		return
	}

	// Store viewing/listening behavior based on media type
	var behaviorKey string
	switch mediaFile.MediaType {
	case database.MediaTypeTrack:
		behaviorKey = "listening_behavior"
	case database.MediaTypeMovie, database.MediaTypeEpisode:
		behaviorKey = "viewing_behavior"
	default:
		behaviorKey = "playback_behavior"
	}

	behavior := map[string]interface{}{
		"last_played_at":          time.Now(),
		"total_play_time_seconds": int64(playPercentage / 100.0 * float64(mediaFile.Duration)),
		"media_type":              string(mediaFile.MediaType),
		"completion_rate":         playPercentage,
		"played_duration":         playedDuration,
		"behavior_type":           behaviorKey,
	}

	// Update quality preferences based on media type
	mhm.updateQualityPreferenceByType(&prefs, mediaFile, playPercentage)

	// Update format preferences
	mhm.updateFormatPreference(&prefs, mediaFile.Container, playPercentage)

	// Store behavior in ContentRatings field as JSON
	if behaviorJSON, err := json.Marshal(behavior); err == nil {
		prefs.ContentRatings = string(behaviorJSON)
	}

	// Save updated preferences
	if err := mhm.db.Save(&prefs).Error; err != nil {
		mhm.logger.Error("Failed to save user preferences", "error", err)
	}
}

// updateQualityPreferenceByType updates quality preferences based on media type
func (mhm *MediaHistoryManager) updateQualityPreferenceByType(prefs *models.UserPreferences, mediaFile *database.MediaFile, playPercentage float64) {
	// Parse existing preferences from PreferredGenres field
	var qualityPrefs map[string]float64
	if prefs.PreferredGenres != "" {
		json.Unmarshal([]byte(prefs.PreferredGenres), &qualityPrefs)
	}
	if qualityPrefs == nil {
		qualityPrefs = make(map[string]float64)
	}

	// Weight preference based on completion rate
	weight := playPercentage / 100.0

	var qualityKey string
	switch mediaFile.MediaType {
	case database.MediaTypeTrack:
		// For audio, track codec and sample rate preferences
		qualityKey = fmt.Sprintf("audio_%s_%dkHz", mediaFile.AudioCodec, mediaFile.AudioSampleRate/1000)
		if mediaFile.AudioBitDepth > 0 {
			qualityKey += fmt.Sprintf("_%dbit", mediaFile.AudioBitDepth)
		}

	case database.MediaTypeMovie, database.MediaTypeEpisode:
		// For video, track resolution and codec preferences
		qualityKey = fmt.Sprintf("video_%s_%s", mediaFile.Resolution, mediaFile.VideoCodec)
		if mediaFile.HDRFormat != "" {
			qualityKey += "_hdr"
		}

	default:
		qualityKey = fmt.Sprintf("quality_%s", mediaFile.Resolution)
	}

	qualityPrefs[qualityKey] = qualityPrefs[qualityKey]*0.8 + weight*0.2

	// Save back to preferences
	if qualityJSON, err := json.Marshal(qualityPrefs); err == nil {
		prefs.PreferredGenres = string(qualityJSON)
	}
}

// updateFormatPreference updates container format preferences
func (mhm *MediaHistoryManager) updateFormatPreference(prefs *models.UserPreferences, container string, playPercentage float64) {
	if container == "" {
		return
	}

	// Parse existing preferences from LanguagePrefs field
	var formatPrefs map[string]float64
	if prefs.LanguagePrefs != "" {
		json.Unmarshal([]byte(prefs.LanguagePrefs), &formatPrefs)
	}
	if formatPrefs == nil {
		formatPrefs = make(map[string]float64)
	}

	// Weight preference based on completion rate
	weight := playPercentage / 100.0
	formatPrefs["format_"+container] = formatPrefs["format_"+container]*0.8 + weight*0.2

	// Save back to preferences
	if formatJSON, err := json.Marshal(formatPrefs); err == nil {
		prefs.LanguagePrefs = string(formatJSON)
	}
}

// GetPlaybackHistory returns playback history for a user (both video and audio)
func (mhm *MediaHistoryManager) GetPlaybackHistory(userID string, mediaType string, limit int) ([]models.PlaybackHistory, error) {
	var history []models.PlaybackHistory

	query := mhm.db.Where("user_id = ?", userID)

	// Filter by media type if specified
	if mediaType != "" {
		query = query.Where("media_type = ?", mediaType)
	}

	query = query.Order("played_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&history).Error; err != nil {
		return nil, fmt.Errorf("failed to get playback history: %w", err)
	}

	return history, nil
}

// GetRecentlyPlayed returns recently played content for a user
func (mhm *MediaHistoryManager) GetRecentlyPlayed(userID string, mediaType string, limit int) ([]models.PlaybackHistory, error) {
	var history []models.PlaybackHistory

	// Get recent plays from the last 30 days
	cutoff := time.Now().AddDate(0, 0, -30)

	query := mhm.db.Where("user_id = ? AND played_at > ?", userID, cutoff)

	if mediaType != "" {
		query = query.Where("media_type = ?", mediaType)
	}

	query = query.Order("played_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&history).Error; err != nil {
		return nil, fmt.Errorf("failed to get recently played: %w", err)
	}

	return history, nil
}

// GetIncompleteVideos returns video content that user started but didn't finish
func (mhm *MediaHistoryManager) GetIncompleteVideos(userID string, limit int) ([]models.PlaybackHistory, error) {
	var history []models.PlaybackHistory

	// Only for video content, get plays that are between 10% and 80% complete
	query := mhm.db.Where("user_id = ? AND media_type IN (?, ?) AND NOT completed",
		userID, database.MediaTypeMovie, database.MediaTypeEpisode).
		Where("(played_seconds * 100.0 / duration) BETWEEN ? AND ?", 10.0, 80.0).
		Order("played_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&history).Error; err != nil {
		return nil, fmt.Errorf("failed to get incomplete videos: %w", err)
	}

	return history, nil
}

// GetMostPlayedMusic returns most played music tracks for a user
func (mhm *MediaHistoryManager) GetMostPlayedMusic(userID string, limit int) ([]models.PlaybackHistory, error) {
	var history []models.PlaybackHistory

	// Get music tracks ordered by play count
	query := mhm.db.Raw(`
	SELECT media_file_id, media_type, COUNT(*) as play_count,
	MAX(played_at) as last_played,
	AVG(played_seconds) as avg_listen_time,
	quality, duration
	FROM playback_histories 
	WHERE user_id = ? AND media_type = ?
	GROUP BY media_file_id 
	ORDER BY play_count DESC, last_played DESC
	`, userID, database.MediaTypeTrack)

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&history).Error; err != nil {
		return nil, fmt.Errorf("failed to get most played music: %w", err)
	}

	return history, nil
}

// GetUserPreferences returns user preferences for recommendations
func (mhm *MediaHistoryManager) GetUserPreferences(userID string) (*models.UserPreferences, error) {
	var prefs models.UserPreferences

	err := mhm.db.FirstOrCreate(&prefs, models.UserPreferences{UserID: userID}).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get user preferences: %w", err)
	}

	return &prefs, nil
}
