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
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT(media_type, media_id, stage) DO UPDATE SET
    status = EXCLUDED.status,
    plugin_id = EXCLUDED.plugin_id,
    completed_at = EXCLUDED.completed_at,
    error_message = EXCLUDED.error_message,
    metadata_json = EXCLUDED.metadata_json;

-- name: GetEnrichmentStatus :one
SELECT * FROM enrichment_status
WHERE media_type = $1 AND media_id = $2 AND stage = $3;

-- name: GetEnrichmentStatusByMedia :many
SELECT * FROM enrichment_status
WHERE media_type = $1 AND media_id = $2
ORDER BY stage;

-- name: GetEnrichmentStatusByStage :many
SELECT * FROM enrichment_status
WHERE stage = $1 AND status = $2
ORDER BY media_type, media_id
LIMIT $3 OFFSET $4;

-- name: CountEnrichmentStatusByStage :one
SELECT
    SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END)::bigint as pending_count,
    SUM(CASE WHEN status = 'processing' THEN 1 ELSE 0 END)::bigint as processing_count,
    SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END)::bigint as completed_count,
    SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END)::bigint as failed_count,
    SUM(CASE WHEN status = 'skipped' THEN 1 ELSE 0 END)::bigint as skipped_count,
    COUNT(*)::bigint as total_count
FROM enrichment_status
WHERE stage = $1;

-- name: MarkEnrichmentComplete :exec
UPDATE enrichment_status
SET
    status = 'completed',
    plugin_id = $1,
    completed_at = NOW(),
    error_message = NULL,
    metadata_json = $2
WHERE media_type = $3 AND media_id = $4 AND stage = $5;

-- name: MarkEnrichmentFailed :exec
UPDATE enrichment_status
SET
    status = 'failed',
    plugin_id = $1,
    error_message = $2
WHERE media_type = $3 AND media_id = $4 AND stage = $5;

-- name: MarkEnrichmentSkipped :exec
UPDATE enrichment_status
SET
    status = 'skipped',
    plugin_id = $1,
    completed_at = NOW()
WHERE media_type = $2 AND media_id = $3 AND stage = $4;

-- name: DeleteEnrichmentStatusByMedia :exec
DELETE FROM enrichment_status WHERE media_type = $1 AND media_id = $2;

-- name: GetLibraryEnrichmentProgress :many
SELECT
    es.stage,
    SUM(CASE WHEN es.status = 'completed' THEN 1 ELSE 0 END)::bigint as completed_count,
    SUM(CASE WHEN es.status = 'pending' THEN 1 ELSE 0 END)::bigint as pending_count,
    SUM(CASE WHEN es.status = 'processing' THEN 1 ELSE 0 END)::bigint as processing_count,
    SUM(CASE WHEN es.status = 'failed' THEN 1 ELSE 0 END)::bigint as failed_count,
    SUM(CASE WHEN es.status = 'skipped' THEN 1 ELSE 0 END)::bigint as skipped_count,
    COUNT(*)::bigint as total_count
FROM enrichment_status es
WHERE (
    (es.media_type IN ('movie', 'tv') AND EXISTS (SELECT 1 FROM media m WHERE m.id = es.media_id AND m.library_id = $1))
    OR (es.media_type = 'tv_show' AND EXISTS (SELECT 1 FROM tv_shows ts WHERE ts.id = es.media_id AND ts.library_id = $1))
    OR (es.media_type = 'tv_season' AND EXISTS (SELECT 1 FROM tv_seasons tsn JOIN tv_shows ts ON ts.id = tsn.tv_show_id WHERE tsn.id = es.media_id AND ts.library_id = $1))
)
GROUP BY es.stage;

-- name: ResetStuckEnrichmentStatus :execrows
-- Reset all 'processing' status records to 'pending'.
-- Called at startup to recover from crashed workers.
UPDATE enrichment_status
SET status = 'pending'
WHERE status = 'processing';
