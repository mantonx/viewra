-- name: GetPlaybackPreferences :one
SELECT * FROM playback_preferences
WHERE user_id = $1 AND media_id = $2 AND device_profile = $3
LIMIT 1;

-- name: UpsertPlaybackPreferences :one
INSERT INTO playback_preferences (
    user_id,
    media_id,
    device_profile,
    selected_quality,
    selected_audio_track,
    selected_subtitle_track,
    created_at,
    updated_at
) VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT(user_id, media_id, device_profile)
DO UPDATE SET
    selected_quality = COALESCE(EXCLUDED.selected_quality, playback_preferences.selected_quality),
    selected_audio_track = COALESCE(EXCLUDED.selected_audio_track, playback_preferences.selected_audio_track),
    selected_subtitle_track = COALESCE(EXCLUDED.selected_subtitle_track, playback_preferences.selected_subtitle_track),
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: DeletePlaybackPreferences :exec
DELETE FROM playback_preferences
WHERE user_id = $1 AND media_id = $2 AND device_profile = $3;

-- name: DeletePlaybackPreferencesByMediaID :exec
DELETE FROM playback_preferences
WHERE media_id = $1;

-- name: DeletePlaybackPreferencesByUserID :exec
DELETE FROM playback_preferences
WHERE user_id = $1;
