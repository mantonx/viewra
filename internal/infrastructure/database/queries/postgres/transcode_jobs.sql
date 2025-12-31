-- name: CreateTranscodeJob :one
INSERT INTO transcode_jobs (
    media_id,
    quality,
    type,
    status,
    progress,
    created_at
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateTranscodeJob :exec
UPDATE transcode_jobs
SET status = $2,
    progress = $3,
    error = $4,
    started_at = $5,
    completed_at = $6,
    file_path = $7,
    file_size_bytes = $8,
    start_position = $9
WHERE id = $1;

-- name: GetTranscodeJobByID :one
SELECT * FROM transcode_jobs
WHERE id = $1;

-- name: GetTranscodeJobByMediaIDAndQuality :one
SELECT * FROM transcode_jobs
WHERE media_id = $1 AND quality = $2;

-- name: ListTranscodeJobsByMediaID :many
SELECT * FROM transcode_jobs
WHERE media_id = $1
ORDER BY created_at DESC;

-- name: ListTranscodeJobsByStatus :many
SELECT * FROM transcode_jobs
WHERE status = $1
ORDER BY created_at ASC;

-- name: ListQueuedTranscodeJobs :many
SELECT * FROM transcode_jobs
WHERE status = 'queued'
ORDER BY created_at ASC
LIMIT sqlc.arg('limit')::bigint;

-- name: ListProcessingTranscodeJobs :many
SELECT * FROM transcode_jobs
WHERE status = 'processing'
ORDER BY started_at ASC;

-- name: DeleteTranscodeJob :exec
DELETE FROM transcode_jobs
WHERE id = $1;

-- name: DeleteTranscodeJobsByMediaID :exec
DELETE FROM transcode_jobs
WHERE media_id = $1;

-- name: CountTranscodeJobsByStatus :one
SELECT COUNT(*) FROM transcode_jobs
WHERE status = $1;

-- name: ListAllTranscodeJobs :many
SELECT * FROM transcode_jobs
ORDER BY created_at DESC;

-- name: UpdateTranscodeJobAccess :exec
UPDATE transcode_jobs
SET last_accessed_at = $2,
    access_count = access_count + 1
WHERE id = $1;

-- name: UpdateTranscodeJobAccessByMediaAndQuality :exec
UPDATE transcode_jobs
SET last_accessed_at = $3,
    access_count = access_count + 1
WHERE media_id = $1 AND quality = $2;

-- name: ListTranscodeJobsByLRU :many
SELECT * FROM transcode_jobs
WHERE status = 'completed'
  AND last_accessed_at IS NOT NULL
ORDER BY last_accessed_at ASC
LIMIT sqlc.arg('limit')::bigint;

-- name: GetTotalTranscodeSize :one
SELECT COALESCE(SUM(file_size_bytes), 0) as total_size
FROM transcode_jobs
WHERE status = 'completed';
