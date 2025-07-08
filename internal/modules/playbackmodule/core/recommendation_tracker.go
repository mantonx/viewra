// Package core provides recommendation tracking functionality for the playback module.
package core

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

// RecommendationTracker tracks user interactions for building recommendations
type RecommendationTracker struct {
	logger hclog.Logger
	db     *gorm.DB
}

// NewRecommendationTracker creates a new recommendation tracker
func NewRecommendationTracker(logger hclog.Logger, db *gorm.DB) *RecommendationTracker {
	return &RecommendationTracker{
		logger: logger,
		db:     db,
	}
}

// RecordInteraction records user interactions for recommendation engine
func (rt *RecommendationTracker) RecordInteraction(userID, mediaFileID, interactionType string, value float64, metadata map[string]interface{}) error {
	return rt.TrackInteraction(userID, mediaFileID, interactionType, value, metadata)
}

// TrackInteraction records user interactions for recommendation engine
func (rt *RecommendationTracker) TrackInteraction(userID, mediaFileID, interactionType string, value float64, metadata map[string]interface{}) error {
	// Create interaction record
	var metadataJSON string
	if len(metadata) > 0 {
		if jsonBytes, err := json.Marshal(metadata); err == nil {
			metadataJSON = string(jsonBytes)
		}
	}

	interaction := models.MediaInteraction{
		ID:              utils.GenerateUUID(),
		UserID:          userID,
		MediaFileID:     mediaFileID,
		InteractionType: interactionType,
		Score:           float32(value),
		Context:         metadataJSON,
		InteractionTime: time.Now(),
	}

	if err := rt.db.Create(&interaction).Error; err != nil {
		return fmt.Errorf("failed to track interaction: %w", err)
	}

	// Update media features for this content
	go rt.updateMediaFeatures(mediaFileID)

	rt.logger.Debug("Tracked user interaction",
		"userID", userID,
		"mediaFileID", mediaFileID,
		"type", interactionType,
		"value", value)

	return nil
}

// TrackPlaybackEvent tracks specific playback events for recommendations
func (rt *RecommendationTracker) TrackPlaybackEvent(sessionID, userID, mediaFileID, eventType string, position int64, metadata map[string]interface{}) error {
	// Common playback events for recommendations:
	// - "play" - user started playing
	// - "pause" - user paused
	// - "resume" - user resumed after pause
	// - "skip" - user skipped/scrubbed
	// - "stop" - user stopped before end
	// - "complete" - user watched to completion

	eventMetadata := map[string]interface{}{
		"session_id": sessionID,
		"position":   position,
	}

	// Merge additional metadata
	for k, v := range metadata {
		eventMetadata[k] = v
	}

	// Calculate interaction value based on event type
	var value float64
	switch eventType {
	case "play":
		value = 1.0 // Starting to watch is positive
	case "pause":
		value = 0.1 // Pausing is neutral
	case "resume":
		value = 0.8 // Resuming shows continued interest
	case "skip":
		value = -0.2 // Skipping might indicate less interest
	case "stop":
		value = -0.5 // Stopping early is negative
	case "complete":
		value = 2.0 // Completing is very positive
	default:
		value = 0.0
	}

	return rt.TrackInteraction(userID, mediaFileID, "playback_"+eventType, value, eventMetadata)
}

// updateMediaFeatures updates the media features for recommendation
func (rt *RecommendationTracker) updateMediaFeatures(mediaFileID string) {
	// Get media file information
	var mediaFile database.MediaFile
	if err := rt.db.First(&mediaFile, "id = ?", mediaFileID).Error; err != nil {
		rt.logger.Error("Failed to get media file for features update", "error", err)
		return
	}

	// Get or create media features
	var features models.MediaFeatures
	err := rt.db.FirstOrCreate(&features, models.MediaFeatures{MediaFileID: mediaFileID}).Error
	if err != nil {
		rt.logger.Error("Failed to get media features", "error", err)
		return
	}

	// Update basic features from media file
	rt.updateBasicFeatures(&features, &mediaFile)

	// Update interaction-based features
	rt.updateInteractionFeatures(&features, mediaFileID)

	// Save updated features
	if err := rt.db.Save(&features).Error; err != nil {
		rt.logger.Error("Failed to save media features", "error", err)
	}
}

// updateBasicFeatures updates basic media features
func (rt *RecommendationTracker) updateBasicFeatures(features *models.MediaFeatures, mediaFile *database.MediaFile) {
	// Use the existing MediaFeatures fields and add custom fields as JSON
	features.Language = mediaFile.Language
	features.ReleaseYear = 0 // Will need to extract from metadata
	features.QualityScore = float32(rt.calculateQualityScore(mediaFile))

	// Store technical features as JSON in existing fields
	technicalFeatures := map[string]interface{}{
		"media_type":     string(mediaFile.MediaType),
		"container":      mediaFile.Container,
		"resolution":     mediaFile.Resolution,
		"duration":       mediaFile.Duration,
		"video_codec":    mediaFile.VideoCodec,
		"audio_codec":    mediaFile.AudioCodec,
		"has_subtitles":  mediaFile.SubtitleStreams != "",
		"is_hdr":         mediaFile.HDRFormat != "",
		"audio_channels": mediaFile.AudioChannels,
		"video_width":    mediaFile.VideoWidth,
		"video_height":   mediaFile.VideoHeight,
	}

	if techJSON, err := json.Marshal(technicalFeatures); err == nil {
		features.Tags = string(techJSON) // Store in Tags field as JSON
	}
}

// updateInteractionFeatures updates features based on user interactions
func (rt *RecommendationTracker) updateInteractionFeatures(features *models.MediaFeatures, mediaFileID string) {
	// Calculate popularity score based on interactions
	var totalInteractions int64
	var avgRating float64

	rt.db.Model(&models.MediaInteraction{}).
		Where("media_file_id = ?", mediaFileID).
		Count(&totalInteractions)

	rt.db.Model(&models.MediaInteraction{}).
		Where("media_file_id = ? AND interaction_type LIKE 'playback_%'", mediaFileID).
		Select("AVG(score)").
		Scan(&avgRating)

	features.PopularityScore = float32(float64(totalInteractions) * (1.0 + avgRating))

	// Calculate completion rate
	var completions, starts int64
	rt.db.Model(&models.MediaInteraction{}).
		Where("media_file_id = ? AND interaction_type = 'playback_complete'", mediaFileID).
		Count(&completions)

	rt.db.Model(&models.MediaInteraction{}).
		Where("media_file_id = ? AND interaction_type = 'playback_play'", mediaFileID).
		Count(&starts)

	completionRate := float32(0.0)
	if starts > 0 {
		completionRate = float32(float64(completions) / float64(starts))
	}
	features.EngagementScore = completionRate // Store completion rate as engagement score
}

// calculateQualityScore calculates a quality score for the media
func (rt *RecommendationTracker) calculateQualityScore(mediaFile *database.MediaFile) float64 {
	score := 1.0

	// Resolution score
	switch mediaFile.Resolution {
	case "4K", "2160p":
		score += 3.0
	case "1080p":
		score += 2.0
	case "720p":
		score += 1.0
	}

	// Codec score
	if mediaFile.VideoCodec == "h265" || mediaFile.VideoCodec == "hevc" {
		score += 1.0
	}

	// Audio quality score
	if mediaFile.AudioChannels > 2 {
		score += 0.5
	}

	// HDR support
	if mediaFile.HDRFormat != "" {
		score += 1.0
	}

	return score
}

// GetUserInteractionHistory returns interaction history for a user
func (rt *RecommendationTracker) GetUserInteractionHistory(userID string, limit int) ([]models.MediaInteraction, error) {
	var interactions []models.MediaInteraction

	query := rt.db.Where("user_id = ?", userID).
		Order("interaction_time DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&interactions).Error; err != nil {
		return nil, fmt.Errorf("failed to get interaction history: %w", err)
	}

	return interactions, nil
}

// GetPopularContent returns popular content based on interactions
func (rt *RecommendationTracker) GetPopularContent(mediaType string, limit int) ([]models.MediaFeatures, error) {
	var features []models.MediaFeatures

	query := rt.db.Where("popularity_score > 0").
		Order("popularity_score DESC")

	// Note: MediaType field not directly available, would need to parse from Tags JSON
	// For now, skip media type filtering

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&features).Error; err != nil {
		return nil, fmt.Errorf("failed to get popular content: %w", err)
	}

	return features, nil
}

// GetSimilarContent returns content similar to a given media file
func (rt *RecommendationTracker) GetSimilarContent(mediaFileID string, limit int) ([]models.MediaFeatures, error) {
	// Get features of the reference media
	var refFeatures models.MediaFeatures
	if err := rt.db.First(&refFeatures, "media_file_id = ?", mediaFileID).Error; err != nil {
		return nil, fmt.Errorf("failed to get reference features: %w", err)
	}

	// Find similar content based on available features
	var similar []models.MediaFeatures
	query := rt.db.Where("media_file_id != ?", mediaFileID)

	// Use existing fields for similarity matching
	if refFeatures.Language != "" {
		query = query.Where("language = ?", refFeatures.Language)
	}

	if refFeatures.ReleaseYear > 0 {
		// Find content with similar release year (±5 years)
		minYear := refFeatures.ReleaseYear - 5
		maxYear := refFeatures.ReleaseYear + 5
		query = query.Where("release_year BETWEEN ? AND ?", minYear, maxYear)
	}

	// Order by popularity and quality
	query = query.Order("popularity_score DESC, quality_score DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&similar).Error; err != nil {
		return nil, fmt.Errorf("failed to get similar content: %w", err)
	}

	return similar, nil
}
