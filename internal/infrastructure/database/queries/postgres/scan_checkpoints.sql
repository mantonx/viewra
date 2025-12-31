-- name: CreateScanCheckpoint :one
INSERT INTO scan_checkpoints (
    scan_job_id,
    file_path,
    status,
    file_size,
    file_hash,
    created_at
) VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
RETURNING *;

-- name: CreateScanCheckpointBatch :exec
INSERT INTO scan_checkpoints (
    scan_job_id,
    file_path,
    status,
    file_size,
    file_hash,
    created_at
) VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP);

-- name: GetScanCheckpointByID :one
SELECT * FROM scan_checkpoints
WHERE id = $1;

-- name: GetScanCheckpointByPath :one
SELECT * FROM scan_checkpoints
WHERE scan_job_id = $1 AND file_path = $2;

-- name: GetPendingScanCheckpoints :many
SELECT * FROM scan_checkpoints
WHERE scan_job_id = $1 AND status = 'pending'
ORDER BY id ASC
LIMIT sqlc.arg('limit')::bigint;

-- name: UpdateScanCheckpointStatus :exec
UPDATE scan_checkpoints
SET
    status = $1,
    error_message = $2,
    error_category = $3,
    processed_at = $4
WHERE id = $5;

-- name: UpdateScanCheckpointRetryCount :exec
UPDATE scan_checkpoints
SET retry_count = $1
WHERE id = $2;

-- name: GetScanCheckpointStats :one
SELECT
    COUNT(*) as total_files,
    SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending_files,
    SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed_files,
    SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_files,
    SUM(CASE WHEN status = 'warning' THEN 1 ELSE 0 END) as warning_files,
    SUM(CASE WHEN status IN ('completed', 'failed', 'warning') THEN 1 ELSE 0 END) as processed_files,
    MIN(CASE WHEN status IN ('completed', 'failed', 'warning') THEN processed_at END) as first_processed_at
FROM scan_checkpoints
WHERE scan_job_id = $1;

-- name: GetScanCheckpointErrorsByCategory :many
SELECT
    error_category,
    COUNT(*) as error_count
FROM scan_checkpoints
WHERE scan_job_id = $1 AND status IN ('failed', 'warning') AND error_category IS NOT NULL
GROUP BY error_category;

-- name: ListFailedScanCheckpoints :many
SELECT * FROM scan_checkpoints
WHERE scan_job_id = $1 AND status IN ('failed', 'warning')
ORDER BY
    CASE
        WHEN status = 'failed' THEN 1
        WHEN status = 'warning' THEN 2
    END,
    processed_at DESC
LIMIT sqlc.arg('limit')::bigint;

-- name: ResetFailedScanCheckpoints :exec
UPDATE scan_checkpoints
SET
    status = 'pending',
    error_message = NULL,
    error_category = NULL,
    processed_at = NULL
WHERE scan_job_id = $1 AND status = 'failed';

-- name: CountFailedScanCheckpoints :one
SELECT COUNT(*) FROM scan_checkpoints
WHERE scan_job_id = $1 AND status = 'failed';

-- name: DeleteScanCheckpointsByJobID :exec
DELETE FROM scan_checkpoints
WHERE scan_job_id = $1;

-- name: GetScanCheckpointProgress :one
SELECT
    COUNT(*) as total,
    SUM(CASE WHEN status IN ('completed', 'failed', 'warning') THEN 1 ELSE 0 END) as processed
FROM scan_checkpoints
WHERE scan_job_id = $1;
