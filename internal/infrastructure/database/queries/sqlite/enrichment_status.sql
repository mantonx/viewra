-- name: UpsertEnrichmentStatus :exec
INSERT INTO enrichment_status (
    media_type,
    media_id,
    stage,
    status,
    plugin_id,
    completed_at,
    error_message,
    metadata_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(media_type, media_id, stage) DO UPDATE SET
    status = excluded.status,
    plugin_id = excluded.plugin_id,
    completed_at = excluded.completed_at,
    error_message = excluded.error_message,
    metadata_json = excluded.metadata_json;

-- name: GetEnrichmentStatus :one
SELECT * FROM enrichment_status
WHERE media_type = ? AND media_id = ? AND stage = ?;

-- name: GetEnrichmentStatusByMedia :many
SELECT * FROM enrichment_status
WHERE media_type = ? AND media_id = ?
ORDER BY stage;

-- name: GetEnrichmentStatusByStage :many
SELECT * FROM enrichment_status
WHERE stage = ? AND status = ?
ORDER BY media_type, media_id
LIMIT ? OFFSET ?;

-- name: CountEnrichmentStatusByStage :one
SELECT
    SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending_count,
    SUM(CASE WHEN status = 'processing' THEN 1 ELSE 0 END) as processing_count,
    SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed_count,
    SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_count,
    SUM(CASE WHEN status = 'skipped' THEN 1 ELSE 0 END) as skipped_count,
    COUNT(*) as total_count
FROM enrichment_status
WHERE stage = ?;

-- name: MarkEnrichmentComplete :exec
UPDATE enrichment_status
SET
    status = 'completed',
    plugin_id = ?,
    completed_at = datetime('now'),
    error_message = NULL,
    metadata_json = ?
WHERE media_type = ? AND media_id = ? AND stage = ?;

-- name: MarkEnrichmentFailed :exec
UPDATE enrichment_status
SET
    status = 'failed',
    plugin_id = ?,
    error_message = ?
WHERE media_type = ? AND media_id = ? AND stage = ?;

-- name: MarkEnrichmentSkipped :exec
UPDATE enrichment_status
SET
    status = 'skipped',
    plugin_id = ?,
    completed_at = datetime('now')
WHERE media_type = ? AND media_id = ? AND stage = ?;

-- name: DeleteEnrichmentStatusByMedia :exec
DELETE FROM enrichment_status WHERE media_type = ? AND media_id = ?;

-- name: GetLibraryEnrichmentProgress :many
SELECT
    es.stage,
    SUM(CASE WHEN es.status = 'completed' THEN 1 ELSE 0 END) as completed_count,
    SUM(CASE WHEN es.status = 'pending' THEN 1 ELSE 0 END) as pending_count,
    SUM(CASE WHEN es.status = 'processing' THEN 1 ELSE 0 END) as processing_count,
    SUM(CASE WHEN es.status = 'failed' THEN 1 ELSE 0 END) as failed_count,
    SUM(CASE WHEN es.status = 'skipped' THEN 1 ELSE 0 END) as skipped_count,
    COUNT(*) as total_count
FROM enrichment_status es
WHERE (
    (es.media_type IN ('movie', 'tv') AND EXISTS (SELECT 1 FROM media m WHERE m.id = es.media_id AND m.library_id = ?))
    OR (es.media_type = 'tv_show' AND EXISTS (SELECT 1 FROM tv_shows ts WHERE ts.id = es.media_id AND ts.library_id = ?))
    OR (es.media_type = 'tv_season' AND EXISTS (SELECT 1 FROM tv_seasons tsn JOIN tv_shows ts ON ts.id = tsn.tv_show_id WHERE tsn.id = es.media_id AND ts.library_id = ?))
)
GROUP BY es.stage;

-- name: ResetStuckEnrichmentStatus :execrows
-- Reset all 'processing' status records to 'pending'.
-- Called at startup to recover from crashed workers.
UPDATE enrichment_status
SET status = 'pending'
WHERE status = 'processing';
