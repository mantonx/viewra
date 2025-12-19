-- name: CreateTranscodeAnalytics :exec
INSERT INTO transcode_analytics (
    session_id,
    media_id,
    quality_profile,
    strategy,
    hw_accel,
    status,
    created_at
) VALUES (?, ?, ?, ?, ?, 'started', ?);

-- name: UpdateTranscodeFirstFrame :exec
UPDATE transcode_analytics
SET first_frame_ms = ?
WHERE session_id = ?;

-- name: UpdateTranscodeFirstSegment :exec
UPDATE transcode_analytics
SET first_segment_ms = ?
WHERE session_id = ?;

-- name: UpdateTranscodeManifestReady :exec
UPDATE transcode_analytics
SET manifest_ready_ms = ?
WHERE session_id = ?;

-- name: UpdateTranscodeSegmentCount :exec
UPDATE transcode_analytics
SET segments_created = ?
WHERE session_id = ?;

-- name: CompleteTranscodeAnalytics :exec
UPDATE transcode_analytics
SET status = 'completed',
    total_duration_ms = ?,
    completed_at = ?
WHERE session_id = ?;

-- name: FailTranscodeAnalytics :exec
UPDATE transcode_analytics
SET status = 'failed',
    error_reason = ?,
    completed_at = ?
WHERE session_id = ?;

-- name: GetTranscodeAnalyticsBySessionID :one
SELECT * FROM transcode_analytics
WHERE session_id = ?;

-- name: ListTranscodeAnalyticsByMediaID :many
SELECT * FROM transcode_analytics
WHERE media_id = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: GetTranscodeSummaryByMediaID :one
SELECT
    COUNT(*) as total_sessions,
    AVG(manifest_ready_ms) as avg_manifest_ready_ms,
    AVG(first_frame_ms) as avg_first_frame_ms,
    AVG(first_segment_ms) as avg_first_segment_ms,
    MIN(manifest_ready_ms) as min_manifest_ready_ms,
    MAX(manifest_ready_ms) as max_manifest_ready_ms,
    SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_count
FROM transcode_analytics
WHERE media_id = ?;

-- name: GetOverallTranscodeSummary :one
SELECT
    COUNT(*) as total_sessions,
    COUNT(DISTINCT media_id) as unique_media,
    AVG(manifest_ready_ms) as avg_manifest_ready_ms,
    AVG(first_frame_ms) as avg_first_frame_ms,
    AVG(first_segment_ms) as avg_first_segment_ms,
    MIN(manifest_ready_ms) as min_manifest_ready_ms,
    MAX(manifest_ready_ms) as max_manifest_ready_ms,
    SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_count,
    SUM(CASE WHEN hw_accel IS NOT NULL AND hw_accel != '' THEN 1 ELSE 0 END) as hw_accel_count
FROM transcode_analytics;

-- name: GetCorrelatedAnalytics :many
SELECT
    p.session_id,
    p.media_id,
    p.startup_time_ms as frontend_startup_ms,
    p.total_play_time_ms,
    p.total_buffer_time_ms,
    p.stall_count,
    t.quality_profile,
    t.strategy,
    t.hw_accel,
    t.manifest_ready_ms as backend_startup_ms,
    t.first_frame_ms,
    t.first_segment_ms,
    t.status as transcode_status,
    t.segments_created
FROM playback_sessions p
LEFT JOIN transcode_analytics t ON p.session_id = t.session_id
WHERE p.media_id = ?
ORDER BY p.start_time DESC
LIMIT ? OFFSET ?;

-- name: GetCorrelatedAnalyticsAll :many
SELECT
    p.session_id,
    p.media_id,
    p.startup_time_ms as frontend_startup_ms,
    p.total_play_time_ms,
    p.total_buffer_time_ms,
    p.stall_count,
    t.quality_profile,
    t.strategy,
    t.hw_accel,
    t.manifest_ready_ms as backend_startup_ms,
    t.first_frame_ms,
    t.first_segment_ms,
    t.status as transcode_status,
    t.segments_created
FROM playback_sessions p
LEFT JOIN transcode_analytics t ON p.session_id = t.session_id
ORDER BY p.start_time DESC
LIMIT ? OFFSET ?;
