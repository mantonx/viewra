package database

import (
	"context"
	"database/sql"

	"github.com/mantonx/viewra/internal/domain/progress"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
)

// PlaybackPreferencesRepository implements progress.PlaybackPreferencesRepository using SQLite.
type PlaybackPreferencesRepository struct {
	queries *sqlc_sqlite.Queries
}

// NewPlaybackPreferencesRepository creates a new PlaybackPreferencesRepository.
func NewPlaybackPreferencesRepository(db *sql.DB) *PlaybackPreferencesRepository {
	return &PlaybackPreferencesRepository{
		queries: sqlc_sqlite.New(db),
	}
}

// Get retrieves preferences for a specific user, media, and device profile.
func (r *PlaybackPreferencesRepository) Get(ctx context.Context, userID, mediaID int64, deviceProfile string) (*progress.PlaybackPreferences, error) {
	row, err := r.queries.GetPlaybackPreferences(ctx, sqlc_sqlite.GetPlaybackPreferencesParams{
		UserID:        userID,
		MediaID:       mediaID,
		DeviceProfile: deviceProfile,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found is not an error
		}
		return nil, err
	}

	return r.toDomain(row), nil
}

// Upsert creates or updates preferences.
func (r *PlaybackPreferencesRepository) Upsert(ctx context.Context, prefs *progress.PlaybackPreferences) error {
	_, err := r.queries.UpsertPlaybackPreferences(ctx, sqlc_sqlite.UpsertPlaybackPreferencesParams{
		UserID:                prefs.UserID,
		MediaID:               prefs.MediaID,
		DeviceProfile:         prefs.DeviceProfile,
		SelectedQuality:       toNullString(prefs.SelectedQuality),
		SelectedAudioTrack:    toNullInt64FromIntPtr(prefs.SelectedAudioTrack),
		SelectedSubtitleTrack: toNullInt64FromIntPtr(prefs.SelectedSubtitleTrack),
	})
	return err
}

// Delete removes preferences for a specific user, media, and device profile.
func (r *PlaybackPreferencesRepository) Delete(ctx context.Context, userID, mediaID int64, deviceProfile string) error {
	return r.queries.DeletePlaybackPreferences(ctx, sqlc_sqlite.DeletePlaybackPreferencesParams{
		UserID:        userID,
		MediaID:       mediaID,
		DeviceProfile: deviceProfile,
	})
}

// DeleteByMediaID removes all preferences for a media item.
func (r *PlaybackPreferencesRepository) DeleteByMediaID(ctx context.Context, mediaID int64) error {
	return r.queries.DeletePlaybackPreferencesByMediaID(ctx, mediaID)
}

// DeleteByUserID removes all preferences for a user.
func (r *PlaybackPreferencesRepository) DeleteByUserID(ctx context.Context, userID int64) error {
	return r.queries.DeletePlaybackPreferencesByUserID(ctx, userID)
}

// toDomain converts a database row to a domain entity.
func (r *PlaybackPreferencesRepository) toDomain(row sqlc_sqlite.PlaybackPreference) *progress.PlaybackPreferences {
	return &progress.PlaybackPreferences{
		ID:                    row.ID,
		UserID:                row.UserID,
		MediaID:               row.MediaID,
		DeviceProfile:         row.DeviceProfile,
		SelectedQuality:       fromNullString(row.SelectedQuality),
		SelectedAudioTrack:    fromNullInt64ToIntPtr(row.SelectedAudioTrack),
		SelectedSubtitleTrack: fromNullInt64ToIntPtr(row.SelectedSubtitleTrack),
		CreatedAt:             row.CreatedAt.Time,
		UpdatedAt:             row.UpdatedAt.Time,
	}
}

// Helper functions for null conversions
func toNullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *s, Valid: true}
}

func toNullInt64FromIntPtr(i *int) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: int64(*i), Valid: true}
}

func fromNullString(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

func fromNullInt64ToIntPtr(ni sql.NullInt64) *int {
	if !ni.Valid {
		return nil
	}
	i := int(ni.Int64)
	return &i
}
