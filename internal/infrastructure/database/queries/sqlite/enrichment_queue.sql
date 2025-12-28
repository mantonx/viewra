-- name: EnqueueEnrichmentJob :one
-- Enqueue a job for enrichment processing. On conflict:
-- - If status is completed/skipped/failed: reset to pending for re-processing
-- - If status is pending/processing: keep existing state (idempotent)
-- Always returns the row (new or existing) to avoid "no rows" errors.
INSERT INTO enrichment_queue (
    media_id,
    library_id,
    media_type,
    stage,
    priority,
    status,
    attempts,
    max_attempts,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?, ?, 'pending', 0, ?, datetime('now'), datetime('now'))
ON CONFLICT(media_id, media_type, stage) DO UPDATE SET
    library_id = COALESCE(excluded.library_id, enrichment_queue.library_id),
    priority = CASE WHEN enrichment_queue.status IN ('completed', 'skipped', 'failed') THEN excluded.priority ELSE enrichment_queue.priority END,
    status = CASE WHEN enrichment_queue.status IN ('completed', 'skipped', 'failed') THEN 'pending' ELSE enrichment_queue.status END,
    attempts = CASE WHEN enrichment_queue.status IN ('completed', 'skipped', 'failed') THEN 0 ELSE enrichment_queue.attempts END,
    error_message = CASE WHEN enrichment_queue.status IN ('completed', 'skipped', 'failed') THEN NULL ELSE enrichment_queue.error_message END,
    error_category = CASE WHEN enrichment_queue.status IN ('completed', 'skipped', 'failed') THEN NULL ELSE enrichment_queue.error_category END,
    updated_at = datetime('now')
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

-- name: RequeueEnrichmentJob :exec
-- Re-queue a job for retry without incrementing the attempt count.
-- Used for transient infrastructure errors (plugin restarts, connection issues).
UPDATE enrichment_queue
SET
    status = 'pending',
    error_message = ?,
    error_category = ?,
    next_retry_at = ?,
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

-- name: GetCurrentEnrichmentItem :one
-- Get the currently processing enrichment item with its title for a library.
-- Joins with media/tv_shows/tv_seasons/music tables to get the title.
-- Returns the first processing item (by locked_at) for the library.
SELECT
    eq.id,
    eq.media_id,
    eq.library_id,
    eq.media_type,
    eq.stage,
    COALESCE(
        m.title,
        ts.title,
        tsn.name,
        ma.title,
        mart.name,
        ''
    ) as title
FROM enrichment_queue eq
LEFT JOIN media m ON eq.media_type IN ('movie', 'tv', 'music') AND eq.media_id = m.id
LEFT JOIN tv_shows ts ON eq.media_type = 'tv_show' AND eq.media_id = ts.id
LEFT JOIN tv_seasons tsn ON eq.media_type = 'tv_season' AND eq.media_id = tsn.id
LEFT JOIN music_albums ma ON eq.media_type = 'music_album' AND eq.media_id = ma.id
LEFT JOIN music_artists mart ON eq.media_type = 'music_artist' AND eq.media_id = mart.id
WHERE eq.library_id = ?
  AND eq.status = 'processing'
ORDER BY eq.locked_at ASC
LIMIT 1;

-- name: UpdatePriorityByMedia :exec
-- Updates priority for all pending/processing jobs for a media item.
-- Used to boost priority after enrichment discovers actual release date.
-- Only upgrades priority (higher values), never downgrades - preserves "added recently" boost.
UPDATE enrichment_queue
SET priority = ?, updated_at = datetime('now')
WHERE media_id = ? AND media_type = ? AND status IN ('pending', 'processing') AND priority < ?;

-- name: BoostPriority :execrows
-- Boosts priority for pending/processing jobs and returns the number of affected rows.
-- Used for interactive priority boost when user views an item.
UPDATE enrichment_queue
SET priority = ?, updated_at = datetime('now')
WHERE media_id = ? AND media_type = ? AND status IN ('pending', 'processing');

-- name: GetLibraryEnrichmentFailures :many
-- Get failed enrichment jobs for a library with titles for display.
-- Joins with media/tv_shows/tv_seasons/music tables to get the title.
SELECT
    eq.id,
    eq.media_id,
    eq.library_id,
    eq.media_type,
    eq.stage,
    eq.attempts,
    eq.max_attempts,
    eq.error_message,
    eq.error_category,
    eq.updated_at as last_attempt_at,
    COALESCE(
        m.title,
        ts.title,
        tsn.name,
        ma.title,
        mart.name,
        ''
    ) as title
FROM enrichment_queue eq
LEFT JOIN media m ON eq.media_type IN ('movie', 'tv', 'music') AND eq.media_id = m.id
LEFT JOIN tv_shows ts ON eq.media_type = 'tv_show' AND eq.media_id = ts.id
LEFT JOIN tv_seasons tsn ON eq.media_type = 'tv_season' AND eq.media_id = tsn.id
LEFT JOIN music_albums ma ON eq.media_type = 'music_album' AND eq.media_id = ma.id
LEFT JOIN music_artists mart ON eq.media_type = 'music_artist' AND eq.media_id = mart.id
WHERE eq.library_id = ?
  AND eq.status = 'failed'
ORDER BY eq.updated_at DESC
LIMIT ? OFFSET ?;

-- name: CountLibraryEnrichmentFailures :one
-- Count total failed enrichment jobs for a library.
SELECT COUNT(*) as total
FROM enrichment_queue
WHERE library_id = ? AND status = 'failed';

-- name: RetryEnrichmentJobsByLibrary :execrows
-- Reset all failed jobs for a library to pending for retry.
UPDATE enrichment_queue
SET
    status = 'pending',
    attempts = 0,
    error_message = NULL,
    error_category = NULL,
    next_retry_at = NULL,
    updated_at = datetime('now')
WHERE library_id = ? AND status = 'failed';

-- name: RetryEnrichmentJob :exec
-- Reset a single failed job to pending for retry.
UPDATE enrichment_queue
SET
    status = 'pending',
    attempts = 0,
    error_message = NULL,
    error_category = NULL,
    next_retry_at = NULL,
    updated_at = datetime('now')
WHERE id = ? AND status = 'failed';

-- name: GetOrphanedPipelineStates :many
-- Find enrichment statuses where a stage completed but the next stage was never enqueued.
-- This happens when the server crashes between marking a stage complete and enqueuing the next.
-- Returns the media items that need their next stage enqueued.
SELECT
    es.media_type,
    es.media_id,
    es.stage as completed_stage,
    next_ep.stage_name as next_stage
FROM enrichment_status es
JOIN enrichment_pipelines ep
    ON ep.media_type = es.media_type
    AND ep.stage_name = es.stage
    AND ep.enabled = 1
JOIN enrichment_pipelines next_ep
    ON next_ep.media_type = es.media_type
    AND next_ep.enabled = 1
    AND next_ep.position = (
        SELECT MIN(ep2.position)
        FROM enrichment_pipelines ep2
        WHERE ep2.media_type = es.media_type
        AND ep2.position > ep.position
        AND ep2.enabled = 1
    )
WHERE es.status = 'completed'
AND NOT EXISTS (
    SELECT 1 FROM enrichment_status es2
    WHERE es2.media_type = es.media_type
    AND es2.media_id = es.media_id
    AND es2.stage = next_ep.stage_name
)
AND NOT EXISTS (
    SELECT 1 FROM enrichment_queue eq
    WHERE eq.media_type = es.media_type
    AND eq.media_id = es.media_id
    AND eq.stage = next_ep.stage_name
    AND eq.status IN ('pending', 'processing')
);
