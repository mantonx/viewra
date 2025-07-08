// Package models provides database models for the playback module.
package models

import (
	"time"

	"gorm.io/gorm"
)

// PlaybackSession represents a playback session in the database
type PlaybackSession struct {
	ID             string     `gorm:"primaryKey;size:36" json:"id"`
	MediaFileID    string     `gorm:"size:36;index" json:"media_file_id"`
	UserID         string     `gorm:"size:36;index;index:idx_user_time" json:"user_id"`
	DeviceID       string     `gorm:"size:36;index" json:"device_id"`
	Method         string     `gorm:"size:20" json:"method"` // direct, remux, transcode
	TranscodeID    string     `gorm:"size:36;index" json:"transcode_id,omitempty"`
	StartTime      time.Time  `gorm:"index;index:idx_user_time" json:"start_time"`
	EndTime        *time.Time `json:"end_time,omitempty"`
	LastActivity   time.Time  `gorm:"index" json:"last_activity"`
	Position       int64      `json:"position"`                   // Current playback position in seconds
	Duration       int64      `json:"duration"`                   // Total duration in seconds
	State          string     `gorm:"size:20;index" json:"state"` // playing, paused, stopped, ended
	PlayedDuration int64      `json:"played_duration"`            // Total seconds actually played
	Completed      bool       `gorm:"index" json:"completed"`     // Whether user completed playback

	// Analytics fields
	IPAddress     string `gorm:"size:45" json:"ip_address,omitempty"`
	Location      string `gorm:"size:100" json:"location,omitempty"`
	UserAgent     string `gorm:"size:500" json:"user_agent,omitempty"`
	DeviceName    string `gorm:"size:100" json:"device_name,omitempty"`
	DeviceType    string `gorm:"size:50" json:"device_type,omitempty"`
	Browser       string `gorm:"size:50" json:"browser,omitempty"`
	OS            string `gorm:"size:50" json:"os,omitempty"`
	QualityPlayed string `gorm:"size:20" json:"quality_played,omitempty"`
	Bandwidth     int64  `json:"bandwidth,omitempty"`

	// Metadata stored as JSON
	Capabilities string `gorm:"type:json" json:"capabilities,omitempty"`
	DebugInfo    string `gorm:"type:json" json:"debug_info,omitempty"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// UserMediaProgress tracks user's progress for resume functionality
type UserMediaProgress struct {
	ID          string `gorm:"primaryKey;size:36"`
	UserID      string `gorm:"size:36;index"`
	MediaFileID string `gorm:"size:36;index"`
	Position    int64  // Last playback position in seconds
	Duration    int64  // Total duration in seconds
	Completed   bool   // Whether user has completed >90% of the content
	UpdatedAt   time.Time

	// Create unique index on user_id + media_file_id
	gorm.Model
}

// PlaybackAnalytics stores aggregated analytics data
type PlaybackAnalytics struct {
	ID           string    `gorm:"primaryKey;size:36"`
	Date         time.Time `gorm:"type:date;index"`
	MediaFileID  string    `gorm:"size:36;index"`
	MediaType    string    `gorm:"size:20"` // video, audio
	PlayCount    int64
	UniqueUsers  int64
	TotalMinutes int64

	// Device breakdown (stored as JSON)
	DeviceStats  string `gorm:"type:json"`
	QualityStats string `gorm:"type:json"`
	MethodStats  string `gorm:"type:json"` // direct, remux, transcode counts

	CreatedAt time.Time
	UpdatedAt time.Time
}

// TranscodeCleanupTask tracks transcodes that need cleanup
type TranscodeCleanupTask struct {
	ID          string `gorm:"primaryKey;size:36"`
	TranscodeID string `gorm:"size:36;unique"`
	FilePath    string `gorm:"size:500"`
	FileSize    int64
	CreatedAt   time.Time
	LastUsed    time.Time `gorm:"index"`
	CleanupAt   time.Time `gorm:"index"`
	Status      string    `gorm:"size:20"` // pending, cleaned, failed
}

// SessionEvent represents events that occur during a playback session
type SessionEvent struct {
	ID        string    `gorm:"primaryKey;size:36"`
	SessionID string    `gorm:"size:36;index"`
	EventType string    `gorm:"size:50;index"` // play, pause, seek, buffer, error, quality_change
	EventTime time.Time `gorm:"index"`
	Position  int64     // Position in seconds when event occurred
	Data      string    `gorm:"type:json"` // Additional event data as JSON
	CreatedAt time.Time
}

// WatchHistory provides a denormalized view for quick history queries
type PlaybackHistory struct {
	ID            string    `gorm:"primaryKey;size:36"`
	UserID        string    `gorm:"size:36;index;index:idx_user_play_time"`
	MediaFileID   string    `gorm:"size:36;index"`
	MediaTitle    string    `gorm:"size:500"` // Denormalized for performance
	MediaType     string    `gorm:"size:20"`  // video, audio
	ThumbnailPath string    `gorm:"size:500"` // For quick display
	PlayedAt      time.Time `gorm:"index;index:idx_user_play_time"`
	Duration      int64     // Total duration of media
	PlayedSeconds int64     // How many seconds were played
	Completed     bool      `gorm:"index"`
	LastPosition  int64     // For resume functionality
	DeviceType    string    `gorm:"size:50"`
	Quality       string    `gorm:"size:20"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// UserPlaybackStats aggregates user playback statistics
type UserPlaybackStats struct {
	ID              string `gorm:"primaryKey;size:36"`
	UserID          string `gorm:"size:36;unique"`
	TotalPlayTime   int64  // Total seconds played
	ItemsCompleted  int64  // Total completed items (videos + audio)
	AudiosPlayed    int64
	FavoriteGenres  string `gorm:"type:json"` // JSON array of genres with play counts
	DeviceBreakdown string `gorm:"type:json"` // JSON object of device usage
	PeakPlayHour    int    // 0-23, hour of day user plays most
	LastActive      time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// UserPreferences tracks user preferences for recommendations
type UserPreferences struct {
	ID                 string `gorm:"primaryKey;size:36"`
	UserID             string `gorm:"size:36;unique"`
	PreferredGenres    string `gorm:"type:json"` // JSON array of genre preferences with weights
	PreferredActors    string `gorm:"type:json"` // JSON array of actor preferences
	PreferredDirectors string `gorm:"type:json"` // JSON array of director preferences
	ContentRatings     string `gorm:"type:json"` // Preferred content ratings (PG, R, etc)
	LanguagePrefs      string `gorm:"type:json"` // Preferred languages
	AvoidGenres        string `gorm:"type:json"` // Genres to avoid
	PreferredDuration  string `gorm:"size:50"`   // short (<30min), medium (30-90min), long (>90min)
	AutoSkipIntro      bool   // Whether user typically skips intros
	AutoSkipCredits    bool   // Whether user typically skips credits
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// MediaInteraction tracks detailed user interactions with media
type MediaInteraction struct {
	ID              string    `gorm:"primaryKey;size:36"`
	UserID          string    `gorm:"size:36;index;index:idx_user_media_time"`
	MediaFileID     string    `gorm:"size:36;index;index:idx_user_media_time"`
	InteractionType string    `gorm:"size:50;index"` // completed, liked, disliked, skipped, replayed
	InteractionTime time.Time `gorm:"index;index:idx_user_media_time"`
	Score           float32   // Interaction score for recommendations (0-1)
	Context         string    `gorm:"type:json"` // Additional context (time of day, device, etc)
	CreatedAt       time.Time
}

// MediaFeatures stores extracted features for recommendation engine
type MediaFeatures struct {
	ID              string `gorm:"primaryKey;size:36"`
	MediaFileID     string `gorm:"size:36;unique"`
	Genres          string `gorm:"type:json"` // JSON array of genres
	Actors          string `gorm:"type:json"` // JSON array of actors
	Directors       string `gorm:"type:json"` // JSON array of directors
	Tags            string `gorm:"type:json"` // JSON array of tags/keywords
	Mood            string `gorm:"type:json"` // JSON array of moods (happy, sad, action, etc)
	Pace            string `gorm:"size:20"`   // slow, medium, fast
	ContentRating   string `gorm:"size:20"`   // PG, PG-13, R, etc
	Language        string `gorm:"size:10"`   // Primary language
	ReleaseYear     int
	PopularityScore float32 // External popularity score (0-1)
	QualityScore    float32 // Quality score based on resolution, bitrate, etc (0-1)
	EngagementScore float32 // Based on completion rates (0-1)
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// UserVector stores user preference vectors for similarity calculations
type UserVector struct {
	ID             string `gorm:"primaryKey;size:36"`
	UserID         string `gorm:"size:36;unique"`
	GenreVector    string `gorm:"type:json"` // JSON array of float values
	ActorVector    string `gorm:"type:json"` // JSON array of float values
	DirectorVector string `gorm:"type:json"` // JSON array of float values
	MoodVector     string `gorm:"type:json"` // JSON array of float values
	TimeVector     string `gorm:"type:json"` // Viewing time preferences
	LastCalculated time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// RecommendationCache stores pre-calculated recommendations
type RecommendationCache struct {
	ID                 string    `gorm:"primaryKey;size:36"`
	UserID             string    `gorm:"size:36;index"`
	RecommendationType string    `gorm:"size:50;index"` // similar, trending, personalized, etc
	MediaFileID        string    `gorm:"size:36"`
	Score              float32   // Recommendation score (0-1)
	Reason             string    `gorm:"type:json"` // JSON explaining why recommended
	Position           int       // Order in recommendation list
	ExpiresAt          time.Time `gorm:"index"`
	CreatedAt          time.Time
}

// UserInteraction tracks user interactions for recommendation engine
type UserInteraction struct {
	ID              string    `gorm:"primaryKey;size:36" json:"id"`
	UserID          string    `gorm:"size:36;index" json:"user_id"`
	MediaFileID     string    `gorm:"size:36;index" json:"media_file_id"`
	InteractionType string    `gorm:"size:50;index" json:"interaction_type"` // playback_play, playback_pause, etc.
	Value           float64   `json:"value"`                                 // Interaction weight/score
	Metadata        string    `gorm:"type:json" json:"metadata,omitempty"`   // Additional context as JSON
	InteractionTime time.Time `gorm:"index" json:"interaction_time"`
	CreatedAt       time.Time `json:"created_at"`
}

// TranscodeCache stores transcode results for deduplication
type TranscodeCache struct {
	ID               string     `gorm:"primaryKey;size:36"`
	TranscodeID      string     `gorm:"size:36;unique;index"`
	TranscodeHash    string     `gorm:"size:64;index"` // Hash of transcode parameters
	MediaFileID      string     `gorm:"size:36;index"`
	InputPath        string     `gorm:"size:500"`
	OutputPath       string     `gorm:"size:500"`
	Container        string     `gorm:"size:20"`
	VideoCodec       string     `gorm:"size:50"`
	AudioCodec       string     `gorm:"size:50"`
	ResolutionWidth  int        `json:"resolution_width"`
	ResolutionHeight int        `json:"resolution_height"`
	VideoBitrate     int        `json:"video_bitrate"`
	AudioBitrate     int        `json:"audio_bitrate"`
	FileSize         int64      `json:"file_size"`
	Duration         int64      `json:"duration"`
	Status           string     `gorm:"size:20;index"` // pending, in_progress, completed, failed
	RequestTime      time.Time  `gorm:"index"`
	CompletedAt      *time.Time `json:"completed_at"`
	LastUsed         time.Time  `gorm:"index"` // For cache management
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
