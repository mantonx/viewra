-- name: EnqueueEnrichmentJob :one
-- Enqueue a job for enrichment processing. On conflict:
-- - If status is completed/skipped/failed: reset to pending for re-processing
-- - If status is pending/processing: keep existing state (idempotent)
-- Always returns the row (new or existing) to avoid "no rows" errors.
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
) VALUES ($1, $2, $3, $4, 'pending', 0, $5, NOW(), NOW())
ON CONFLICT(media_id, media_type, stage) DO UPDATE SET
    priority = CASE WHEN enrichment_queue.status IN ('completed', 'skipped', 'failed') THEN EXCLUDED.priority ELSE enrichment_queue.priority END,
    status = CASE WHEN enrichment_queue.status IN ('completed', 'skipped', 'failed') THEN 'pending' ELSE enrichment_queue.status END,
    attempts = CASE WHEN enrichment_queue.status IN ('completed', 'skipped', 'failed') THEN 0 ELSE enrichment_queue.attempts END,
    error_message = CASE WHEN enrichment_queue.status IN ('completed', 'skipped', 'failed') THEN NULL ELSE enrichment_queue.error_message END,
    error_category = CASE WHEN enrichment_queue.status IN ('completed', 'skipped', 'failed') THEN NULL ELSE enrichment_queue.error_category END,
    updated_at = NOW()
RETURNING *;

-- name: GetEnrichmentJob :one
SELECT * FROM enrichment_queue WHERE id = $1;

-- name: GetEnrichmentJobByMediaAndStage :one
SELECT * FROM enrichment_queue WHERE media_id = $1 AND media_type = $2 AND stage = $3;

-- name: ClaimEnrichmentJobs :many
UPDATE enrichment_queue
SET
    status = 'processing',
    locked_by = $1,
    locked_at = NOW(),
    updated_at = NOW()
WHERE id IN (
    SELECT eq.id FROM enrichment_queue eq
    WHERE eq.stage = $2 AND eq.status = 'pending'
    ORDER BY eq.priority DESC, eq.created_at ASC
    LIMIT $3
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: CompleteEnrichmentJob :exec
UPDATE enrichment_queue
SET
    status = 'completed',
    locked_by = NULL,
    locked_at = NULL,
    updated_at = NOW()
WHERE id = $1;

-- name: FailEnrichmentJob :exec
UPDATE enrichment_queue
SET
    status = CASE WHEN attempts + 1 >= max_attempts THEN 'failed' ELSE 'pending' END,
    attempts = attempts + 1,
    error_message = $1,
    error_category = $2,
    next_retry_at = $3,
    locked_by = NULL,
    locked_at = NULL,
    updated_at = NOW()
WHERE id = $4;

-- name: SkipEnrichmentJob :exec
UPDATE enrichment_queue
SET
    status = 'skipped',
    locked_by = NULL,
    locked_at = NULL,
    updated_at = NOW()
WHERE id = $1;

-- name: GetEnrichmentQueueStats :one
SELECT
    stage,
    SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END)::bigint as pending_count,
    SUM(CASE WHEN status = 'processing' THEN 1 ELSE 0 END)::bigint as processing_count,
    SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END)::bigint as completed_count,
    SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END)::bigint as failed_count,
    SUM(CASE WHEN status = 'skipped' THEN 1 ELSE 0 END)::bigint as skipped_count,
    COUNT(*)::bigint as total_count
FROM enrichment_queue
WHERE stage = $1
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
WHERE media_id = $1
ORDER BY created_at;

-- name: ReleaseStuckEnrichmentJobs :exec
UPDATE enrichment_queue
SET
    status = 'pending',
    locked_by = NULL,
    locked_at = NULL,
    updated_at = NOW()
WHERE status = 'processing'
  AND locked_at < NOW() - ($1 || ' seconds')::interval;

-- name: DeleteEnrichmentJobsByMedia :exec
DELETE FROM enrichment_queue WHERE media_id = $1 AND media_type = $2;

-- name: GetRetryableEnrichmentJobs :many
SELECT * FROM enrichment_queue
WHERE status = 'failed'
  AND next_retry_at IS NOT NULL
  AND next_retry_at <= NOW()
  AND stage = $1
ORDER BY next_retry_at ASC
LIMIT $2;

-- name: ResetEnrichmentJobForRetry :exec
UPDATE enrichment_queue
SET
    status = 'pending',
    error_message = NULL,
    error_category = NULL,
    next_retry_at = NULL,
    updated_at = NOW()
WHERE id = $1;
