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
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, CURRENT_TIMESTAMP)
ON CONFLICT(library_id, file_path) DO UPDATE SET
    file_size = EXCLUDED.file_size,
    file_mtime = EXCLUDED.file_mtime,
    file_hash = EXCLUDED.file_hash,
    media_id = EXCLUDED.media_id,
    last_scanned_at = EXCLUDED.last_scanned_at,
    scan_job_id = EXCLUDED.scan_job_id;

-- name: GetScanStateByPath :one
SELECT * FROM scan_state
WHERE library_id = $1 AND file_path = $2;

-- name: GetLibraryScanState :many
SELECT * FROM scan_state
WHERE library_id = $1
ORDER BY file_path ASC;

-- name: CountLibraryScanState :one
SELECT COUNT(*) FROM scan_state
WHERE library_id = $1;

-- name: DeleteScanStateByPath :exec
DELETE FROM scan_state
WHERE library_id = $1 AND file_path = $2;

-- name: DeleteScanStateByLibrary :exec
DELETE FROM scan_state
WHERE library_id = $1;

-- name: GetScanStateModifiedSince :many
SELECT * FROM scan_state
WHERE library_id = $1 AND file_mtime > $2
ORDER BY file_mtime DESC;
