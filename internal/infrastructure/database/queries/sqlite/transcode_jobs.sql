-- name: CreateTranscodeJob :one
INSERT INTO transcode_jobs (
    media_id,
    quality,
    status,
    progress,
    created_at
) VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateTranscodeJob :exec
UPDATE transcode_jobs
SET status = ?,
    progress = ?,
    error = ?,
    started_at = ?,
    completed_at = ?,
    file_path = ?,
    file_size_bytes = ?
WHERE id = ?;

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
ORDER BY created_at ASC;

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

-- name: ListAllTranscodeJobs :many
SELECT * FROM transcode_jobs
ORDER BY created_at DESC;

-- name: UpdateTranscodeJobAccess :exec
UPDATE transcode_jobs
SET last_accessed_at = ?,
    access_count = access_count + 1
WHERE id = ?;

-- name: UpdateTranscodeJobAccessByMediaAndQuality :exec
UPDATE transcode_jobs
SET last_accessed_at = ?,
    access_count = access_count + 1
WHERE media_id = ? AND quality = ?;

-- name: ListTranscodeJobsByLRU :many
SELECT * FROM transcode_jobs
WHERE status = 'completed'
  AND last_accessed_at IS NOT NULL
ORDER BY last_accessed_at ASC
LIMIT ?;

-- name: GetTotalTranscodeSize :one
SELECT COALESCE(SUM(file_size_bytes), 0) as total_size
FROM transcode_jobs
WHERE status = 'completed';
