-- name: CreateTranscodeJob :one
INSERT INTO transcode_jobs (
    media_id,
    quality,
    status,
    progress,
    created_at
) VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateTranscodeJob :one
UPDATE transcode_jobs
SET status = ?,
    progress = ?,
    error = ?,
    started_at = ?,
    completed_at = ?
WHERE id = ?
RETURNING *;

-- name: GetTranscodeJobByID :one
SELECT * FROM transcode_jobs
WHERE id = ?;

-- name: GetTranscodeJobByMediaIDAndQuality :one
SELECT * FROM transcode_jobs
WHERE media_id = ? AND quality = ?;

-- name: ListTranscodeJobsByMediaID :many
SELECT * FROM transcode_jobs
WHERE media_id = ?
ORDER BY created_at DESC;

-- name: ListTranscodeJobsByStatus :many
SELECT * FROM transcode_jobs
WHERE status = ?
ORDER BY created_at ASC
LIMIT ? OFFSET ?;

-- name: ListQueuedTranscodeJobs :many
SELECT * FROM transcode_jobs
WHERE status = 'queued'
ORDER BY created_at ASC
LIMIT ?;

-- name: ListProcessingTranscodeJobs :many
SELECT * FROM transcode_jobs
WHERE status = 'processing'
ORDER BY started_at ASC;

-- name: DeleteTranscodeJob :exec
DELETE FROM transcode_jobs
WHERE id = ?;

-- name: DeleteTranscodeJobsByMediaID :exec
DELETE FROM transcode_jobs
WHERE media_id = ?;

-- name: CountTranscodeJobsByStatus :one
SELECT COUNT(*) FROM transcode_jobs
WHERE status = ?;
