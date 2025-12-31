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
    CAST(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) AS INTEGER) as pending_count,
    CAST(SUM(CASE WHEN status = 'processing' THEN 1 ELSE 0 END) AS INTEGER) as processing_count,
    CAST(SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) AS INTEGER) as completed_count,
    CAST(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) AS INTEGER) as failed_count,
    CAST(SUM(CASE WHEN status = 'skipped' THEN 1 ELSE 0 END) AS INTEGER) as skipped_count,
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
    CAST(SUM(CASE WHEN es.status = 'completed' THEN 1 ELSE 0 END) AS INTEGER) as completed_count,
    CAST(SUM(CASE WHEN es.status = 'pending' THEN 1 ELSE 0 END) AS INTEGER) as pending_count,
    CAST(SUM(CASE WHEN es.status = 'processing' THEN 1 ELSE 0 END) AS INTEGER) as processing_count,
    CAST(SUM(CASE WHEN es.status = 'failed' THEN 1 ELSE 0 END) AS INTEGER) as failed_count,
    CAST(SUM(CASE WHEN es.status = 'skipped' THEN 1 ELSE 0 END) AS INTEGER) as skipped_count,
    COUNT(*) as total_count
FROM enrichment_status es
WHERE (
    (es.media_type IN ('movie', 'tv', 'music') AND EXISTS (SELECT 1 FROM media m WHERE m.id = es.media_id AND m.library_id = sqlc.arg('library_id')))
    OR (es.media_type = 'tv_show' AND EXISTS (SELECT 1 FROM tv_shows ts WHERE ts.id = es.media_id AND ts.library_id = sqlc.arg('library_id')))
    OR (es.media_type = 'tv_season' AND EXISTS (SELECT 1 FROM tv_seasons tsn JOIN tv_shows ts ON ts.id = tsn.show_id WHERE tsn.id = es.media_id AND ts.library_id = sqlc.arg('library_id')))
    OR (es.media_type = 'music_album' AND EXISTS (SELECT 1 FROM music_albums ma WHERE ma.id = es.media_id AND ma.library_id = sqlc.arg('library_id')))
    OR (es.media_type = 'music_artist' AND EXISTS (SELECT 1 FROM music_artists mart WHERE mart.id = es.media_id AND mart.library_id = sqlc.arg('library_id')))
)
GROUP BY es.stage;

-- name: GetLibraryEnrichmentOverallProgress :one
-- Get overall enrichment progress for a library.
-- Returns: items that completed all stages / total unique items that entered enrichment.
-- "Fully enriched" means an item has completed the last stage in the pipeline.
SELECT
    -- Total unique media items in enrichment for this library
    (SELECT COUNT(DISTINCT eq.media_id || ':' || eq.media_type)
     FROM enrichment_queue eq
     WHERE eq.library_id = sqlc.narg('library_id')) as total_items,
    -- Items still pending/processing (not fully done)
    (SELECT COUNT(DISTINCT eq.media_id || ':' || eq.media_type)
     FROM enrichment_queue eq
     WHERE eq.library_id = sqlc.narg('library_id')
       AND eq.status IN ('pending', 'processing')) as remaining_items

;

-- name: ResetStuckEnrichmentStatus :execrows
-- Reset all 'processing' status records to 'pending'.
-- Called at startup to recover from crashed workers.
UPDATE enrichment_status
SET status = 'pending'
WHERE status = 'processing';
