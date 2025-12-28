package progress

import (
	"context"
	"time"
)

// PlaybackPreferences represents device-specific playback preferences for a media item.
// Different devices (browsers) may have different optimal settings based on their capabilities.
type PlaybackPreferences struct {
	ID                    int64
	UserID                int64
	MediaID               int64
	DeviceProfile         string  // Hash of client capabilities (e.g., "chrome-h264-sdr", "firefox-h264h265-hdr")
	SelectedQuality       *string // Quality ID (e.g., "original", "1080p-10m")
	SelectedAudioTrack    *int    // Audio stream index
	SelectedSubtitleTrack *int    // Subtitle track ID (-1 = off)
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// PlaybackPreferencesRepository defines the interface for managing device-specific preferences.
type PlaybackPreferencesRepository interface {
	// Get retrieves preferences for a specific user, media, and device profile.
	// Returns nil, nil if no preferences exist (not an error).
	Get(ctx context.Context, userID, mediaID int64, deviceProfile string) (*PlaybackPreferences, error)

	// Upsert creates or updates preferences.
	// Uses COALESCE to preserve existing values when new values are nil.
	Upsert(ctx context.Context, prefs *PlaybackPreferences) error

	// Delete removes preferences for a specific user, media, and device profile.
	Delete(ctx context.Context, userID, mediaID int64, deviceProfile string) error

	// DeleteByMediaID removes all preferences for a media item (used when media is deleted).
	DeleteByMediaID(ctx context.Context, mediaID int64) error

	// DeleteByUserID removes all preferences for a user (used when user is deleted).
	DeleteByUserID(ctx context.Context, userID int64) error
}
