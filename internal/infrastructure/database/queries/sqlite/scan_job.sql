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
    target_paths
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
    warning_count = ?,
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
    warning_count = ?,
    completed_at = ?,
    error_message = ?,
    phase = ?,
    discovery_done = ?,
    discovery_errors = ?,
    discovery_warnings = ?,
    dirs_scanned = ?,
    dirs_skipped = ?,
    files_skipped = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteScanJob :exec
DELETE FROM scan_jobs
WHERE id = ?;

-- name: DeleteOldScanJobs :exec
DELETE FROM scan_jobs
WHERE library_id = sqlc.arg(library_id)
  AND status IN ('completed', 'failed')
  AND created_at < datetime('now', CAST(sqlc.arg(retention_days) AS TEXT));

-- name: CountScanJobsByLibrary :one
SELECT COUNT(*) FROM scan_jobs
WHERE library_id = ?;

-- name: GetScanJobStats :one
SELECT
    COUNT(*) as total_jobs,
    CAST(SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END) AS INTEGER) as running_jobs,
    CAST(SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) AS INTEGER) as completed_jobs,
    CAST(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) AS INTEGER) as failed_jobs,
    CAST(SUM(files_processed) AS INTEGER) as total_files_processed,
    CAST(SUM(bytes_processed) AS INTEGER) as total_bytes_processed
FROM scan_jobs
WHERE library_id = ?;
