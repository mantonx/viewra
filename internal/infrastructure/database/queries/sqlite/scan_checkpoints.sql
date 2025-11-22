-- name: CreateScanCheckpoint :one
INSERT INTO scan_checkpoints (
    scan_job_id,
    file_path,
    status,
    file_size,
    file_hash,
    created_at
) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
RETURNING *;

-- name: CreateScanCheckpointBatch :exec
INSERT INTO scan_checkpoints (
    scan_job_id,
    file_path,
    status,
    file_size,
    file_hash,
    created_at
) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP);

-- name: GetScanCheckpointByID :one
SELECT * FROM scan_checkpoints
WHERE id = ?;

-- name: GetScanCheckpointByPath :one
SELECT * FROM scan_checkpoints
WHERE scan_job_id = ? AND file_path = ?;

-- name: GetPendingScanCheckpoints :many
SELECT * FROM scan_checkpoints
WHERE scan_job_id = ? AND status = 'pending'
ORDER BY id ASC
LIMIT ?;

-- name: UpdateScanCheckpointStatus :exec
UPDATE scan_checkpoints
SET
    status = ?,
    error_message = ?,
    error_category = ?,
    processed_at = ?
WHERE id = ?;

-- name: UpdateScanCheckpointRetryCount :exec
UPDATE scan_checkpoints
SET retry_count = ?
WHERE id = ?;

-- name: GetScanCheckpointStats :one
SELECT
    COUNT(*) as total_files,
    SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending_files,
    SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed_files,
    SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_files,
    SUM(CASE WHEN status IN ('completed', 'failed') THEN 1 ELSE 0 END) as processed_files
FROM scan_checkpoints
WHERE scan_job_id = ?;

-- name: GetScanCheckpointErrorsByCategory :many
SELECT
    error_category,
    COUNT(*) as error_count
FROM scan_checkpoints
WHERE scan_job_id = ? AND status = 'failed' AND error_category IS NOT NULL
GROUP BY error_category;

-- name: ListFailedScanCheckpoints :many
SELECT * FROM scan_checkpoints
WHERE scan_job_id = ? AND status = 'failed'
ORDER BY processed_at DESC
LIMIT ?;

-- name: ResetFailedScanCheckpoints :exec
UPDATE scan_checkpoints
SET
    status = 'pending',
    error_message = NULL,
    error_category = NULL,
    processed_at = NULL
WHERE scan_job_id = ? AND status = 'failed';

-- name: CountFailedScanCheckpoints :one
SELECT COUNT(*) FROM scan_checkpoints
WHERE scan_job_id = ? AND status = 'failed';

-- name: DeleteScanCheckpointsByJobID :exec
DELETE FROM scan_checkpoints
WHERE scan_job_id = ?;

-- name: GetScanCheckpointProgress :one
SELECT
    COUNT(*) as total,
    SUM(CASE WHEN status IN ('completed', 'failed') THEN 1 ELSE 0 END) as processed
FROM scan_checkpoints
WHERE scan_job_id = ?;
