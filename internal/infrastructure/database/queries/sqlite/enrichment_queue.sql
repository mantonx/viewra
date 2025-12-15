-- name: EnqueueEnrichmentJob :one
INSERT INTO enrichment_queue (
    media_id,
    media_type,
    stage,
    priority,
    status,
    attempts,
    max_attempts,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?, 'pending', 0, ?, datetime('now'), datetime('now'))
ON CONFLICT(media_id, media_type, stage) DO UPDATE SET
    priority = CASE WHEN enrichment_queue.status IN ('completed', 'skipped') THEN excluded.priority ELSE enrichment_queue.priority END,
    status = CASE WHEN enrichment_queue.status IN ('completed', 'skipped') THEN 'pending' ELSE enrichment_queue.status END,
    updated_at = datetime('now')
WHERE enrichment_queue.status IN ('completed', 'skipped', 'failed')
RETURNING *;

-- name: GetEnrichmentJob :one
SELECT * FROM enrichment_queue WHERE id = ?;

-- name: GetEnrichmentJobByMediaAndStage :one
SELECT * FROM enrichment_queue WHERE media_id = ? AND media_type = ? AND stage = ?;

-- name: ClaimEnrichmentJobs :many
UPDATE enrichment_queue
SET
    status = 'processing',
    locked_by = ?,
    locked_at = datetime('now'),
    updated_at = datetime('now')
WHERE id IN (
    SELECT eq.id FROM enrichment_queue eq
    WHERE eq.stage = ? AND eq.status = 'pending'
    ORDER BY eq.priority DESC, eq.created_at ASC
    LIMIT ?
)
RETURNING *;

-- name: CompleteEnrichmentJob :exec
UPDATE enrichment_queue
SET
    status = 'completed',
    locked_by = NULL,
    locked_at = NULL,
    updated_at = datetime('now')
WHERE id = ?;

-- name: FailEnrichmentJob :exec
UPDATE enrichment_queue
SET
    status = CASE WHEN attempts + 1 >= max_attempts THEN 'failed' ELSE 'pending' END,
    attempts = attempts + 1,
    error_message = ?,
    error_category = ?,
    next_retry_at = ?,
    locked_by = NULL,
    locked_at = NULL,
    updated_at = datetime('now')
WHERE id = ?;

-- name: SkipEnrichmentJob :exec
UPDATE enrichment_queue
SET
    status = 'skipped',
    locked_by = NULL,
    locked_at = NULL,
    updated_at = datetime('now')
WHERE id = ?;

-- name: GetEnrichmentQueueStats :one
SELECT
    stage,
    SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending_count,
    SUM(CASE WHEN status = 'processing' THEN 1 ELSE 0 END) as processing_count,
    SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed_count,
    SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_count,
    SUM(CASE WHEN status = 'skipped' THEN 1 ELSE 0 END) as skipped_count,
    COUNT(*) as total_count
FROM enrichment_queue
WHERE stage = ?
GROUP BY stage;

-- name: GetEnrichmentQueueStatsByMedia :many
SELECT
    stage,
    status,
    attempts,
    error_message,
    created_at,
    updated_at
FROM enrichment_queue
WHERE media_id = ?
ORDER BY created_at;

-- name: ReleaseStuckEnrichmentJobs :exec
UPDATE enrichment_queue
SET
    status = 'pending',
    locked_by = NULL,
    locked_at = NULL,
    updated_at = datetime('now')
WHERE status = 'processing'
  AND locked_at < datetime('now', ? || ' seconds');

-- name: DeleteEnrichmentJobsByMedia :exec
DELETE FROM enrichment_queue WHERE media_id = ? AND media_type = ?;

-- name: GetRetryableEnrichmentJobs :many
SELECT * FROM enrichment_queue
WHERE status = 'failed'
  AND next_retry_at IS NOT NULL
  AND next_retry_at <= datetime('now')
  AND stage = ?
ORDER BY next_retry_at ASC
LIMIT ?;

-- name: ResetEnrichmentJobForRetry :exec
UPDATE enrichment_queue
SET
    status = 'pending',
    error_message = NULL,
    error_category = NULL,
    next_retry_at = NULL,
    updated_at = datetime('now')
WHERE id = ?;
