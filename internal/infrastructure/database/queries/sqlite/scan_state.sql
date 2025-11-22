-- name: UpsertScanState :exec
INSERT INTO scan_state (
    library_id,
    file_path,
    file_size,
    file_mtime,
    file_hash,
    media_id,
    last_scanned_at,
    scan_job_id,
    created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(library_id, file_path) DO UPDATE SET
    file_size = excluded.file_size,
    file_mtime = excluded.file_mtime,
    file_hash = excluded.file_hash,
    media_id = excluded.media_id,
    last_scanned_at = excluded.last_scanned_at,
    scan_job_id = excluded.scan_job_id;

-- name: GetScanStateByPath :one
SELECT * FROM scan_state
WHERE library_id = ? AND file_path = ?;

-- name: GetLibraryScanState :many
SELECT * FROM scan_state
WHERE library_id = ?
ORDER BY file_path ASC;

-- name: CountLibraryScanState :one
SELECT COUNT(*) FROM scan_state
WHERE library_id = ?;

-- name: DeleteScanStateByPath :exec
DELETE FROM scan_state
WHERE library_id = ? AND file_path = ?;

-- name: DeleteScanStateByLibrary :exec
DELETE FROM scan_state
WHERE library_id = ?;

-- name: GetScanStateModifiedSince :many
SELECT * FROM scan_state
WHERE library_id = ? AND file_mtime > ?
ORDER BY file_mtime DESC;
