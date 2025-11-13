-- name: CreateTranscodeJob :one
INSERT INTO transcode_jobs (
    media_id,
    quality,
    status,
    progress,
    created_at
) VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateTranscodeJob :exec
UPDATE transcode_jobs
SET status = $2,
    progress = $3,
    error = $4,
    started_at = $5,
    completed_at = $6
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
LIMIT $1;

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
