-- name: CreateScanJob :one
INSERT INTO scan_jobs (
    library_id,
    status,
    progress,
    files_found,
    files_processed,
    bytes_processed,
    error_count,
    started_at,
    phase,
    estimated_total,
    discovery_done,
    created_at,
    updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
RETURNING *;

-- name: GetScanJob :one
SELECT * FROM scan_jobs
WHERE id = $1;

-- name: GetLatestScanJobByLibrary :one
SELECT * FROM scan_jobs
WHERE library_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: ListScanJobsByLibrary :many
SELECT * FROM scan_jobs
WHERE library_id = $1
ORDER BY created_at DESC
LIMIT sqlc.arg('limit')::bigint;

-- name: ListRunningScanJobs :many
SELECT * FROM scan_jobs
WHERE status = 'running'
ORDER BY started_at ASC;

-- name: UpdateScanJobProgress :exec
UPDATE scan_jobs
SET
    progress = $1,
    files_found = $2,
    files_processed = $3,
    bytes_processed = $4,
    error_count = $5,
    warning_count = $6,
    phase = $7,
    estimated_total = $8,
    discovery_done = $9,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $10;

-- name: UpdateScanJobStatus :exec
UPDATE scan_jobs
SET
    status = $1,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $2;

-- name: CompleteScanJob :exec
UPDATE scan_jobs
SET
    status = $1,
    progress = $2,
    files_found = $3,
    files_processed = $4,
    bytes_processed = $5,
    error_count = $6,
    warning_count = $7,
    completed_at = $8,
    error_message = $9,
    phase = $10,
    discovery_done = $11,
    discovery_errors = $12,
    discovery_warnings = $13,
    dirs_scanned = $14,
    dirs_skipped = $15,
    files_skipped = $16,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $17;

-- name: DeleteScanJob :exec
DELETE FROM scan_jobs
WHERE id = $1;

-- name: DeleteOldScanJobs :exec
-- sqlc.arg(retention_days): the number of days to retain completed/failed jobs
DELETE FROM scan_jobs
WHERE library_id = sqlc.arg(library_id)
  AND status IN ('completed', 'failed')
  AND created_at < (CURRENT_TIMESTAMP - (sqlc.arg(retention_days)::text || ' days')::interval);

-- name: CountScanJobsByLibrary :one
SELECT COUNT(*) FROM scan_jobs
WHERE library_id = $1;

-- name: GetScanJobStats :one
SELECT
    COUNT(*) as total_jobs,
    SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END) as running_jobs,
    SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed_jobs,
    SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_jobs,
    SUM(files_processed) as total_files_processed,
    SUM(bytes_processed) as total_bytes_processed
FROM scan_jobs
WHERE library_id = $1;
