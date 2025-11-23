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
    discovery_done
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetScanJob :one
SELECT * FROM scan_jobs
WHERE id = ?;

-- name: GetLatestScanJobByLibrary :one
SELECT * FROM scan_jobs
WHERE library_id = ?
ORDER BY created_at DESC
LIMIT 1;

-- name: ListScanJobsByLibrary :many
SELECT * FROM scan_jobs
WHERE library_id = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: ListRunningScanJobs :many
SELECT * FROM scan_jobs
WHERE status = 'running'
ORDER BY started_at ASC;

-- name: UpdateScanJobProgress :exec
UPDATE scan_jobs
SET
    progress = ?,
    files_found = ?,
    files_processed = ?,
    bytes_processed = ?,
    error_count = ?,
    phase = ?,
    estimated_total = ?,
    discovery_done = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateScanJobStatus :exec
UPDATE scan_jobs
SET
    status = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: CompleteScanJob :exec
UPDATE scan_jobs
SET
    status = ?,
    progress = ?,
    files_found = ?,
    files_processed = ?,
    bytes_processed = ?,
    error_count = ?,
    completed_at = ?,
    error_message = ?,
    phase = ?,
    discovery_done = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteScanJob :exec
DELETE FROM scan_jobs
WHERE id = ?;

-- name: DeleteOldScanJobs :exec
DELETE FROM scan_jobs
WHERE library_id = ?
  AND status IN ('completed', 'failed')
  AND created_at < datetime('now', ?);

-- name: CountScanJobsByLibrary :one
SELECT COUNT(*) FROM scan_jobs
WHERE library_id = ?;

-- name: GetScanJobStats :one
SELECT
    COUNT(*) as total_jobs,
    SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END) as running_jobs,
    SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed_jobs,
    SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_jobs,
    SUM(files_processed) as total_files_processed,
    SUM(bytes_processed) as total_bytes_processed
FROM scan_jobs
WHERE library_id = ?;
