-- name: UpsertPlaybackSession :one
INSERT INTO playback_sessions (
    session_id,
    media_id,
    start_time,
    end_time,
    total_play_time_ms,
    total_buffer_time_ms,
    stall_count,
    quality_switch_count,
    average_quality,
    device_type,
    connection_type
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT(session_id)
DO UPDATE SET
    end_time = COALESCE(EXCLUDED.end_time, playback_sessions.end_time),
    total_play_time_ms = EXCLUDED.total_play_time_ms,
    total_buffer_time_ms = EXCLUDED.total_buffer_time_ms,
    stall_count = EXCLUDED.stall_count,
    quality_switch_count = EXCLUDED.quality_switch_count,
    average_quality = COALESCE(EXCLUDED.average_quality, playback_sessions.average_quality)
RETURNING *;

-- name: CreateQualitySwitchEvent :one
INSERT INTO quality_switch_events (
    session_id,
    media_id,
    from_quality,
    to_quality,
    switch_reason,
    position_seconds,
    network_speed_mbps,
    buffer_seconds,
    caused_stall,
    device_type,
    connection_type,
    timestamp
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetPlaybackSessionByID :one
SELECT * FROM playback_sessions
WHERE session_id = $1;

-- name: ListPlaybackSessionsByMediaID :many
SELECT * FROM playback_sessions
WHERE media_id = $1
ORDER BY start_time DESC
LIMIT $2 OFFSET $3;

-- name: ListQualitySwitchEventsBySessionID :many
SELECT * FROM quality_switch_events
WHERE session_id = $1
ORDER BY timestamp ASC;

-- name: GetQualitySwitchStats :one
SELECT
    COUNT(*) as total_switches,
    SUM(CASE WHEN caused_stall = true THEN 1 ELSE 0 END) as switches_with_stall,
    COUNT(DISTINCT session_id) as unique_sessions
FROM quality_switch_events
WHERE media_id = $1;

-- name: DeleteOldPlaybackSessions :exec
DELETE FROM playback_sessions
WHERE start_time < $1;

-- name: DeleteOldQualitySwitchEvents :exec
DELETE FROM quality_switch_events
WHERE timestamp < $1;
