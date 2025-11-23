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
    has_warning,
    warning_message,
    warning_category,
    has_error,
    error_message,
    error_category,
    created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(library_id, file_path) DO UPDATE SET
    file_size = excluded.file_size,
    file_mtime = excluded.file_mtime,
    file_hash = excluded.file_hash,
    media_id = excluded.media_id,
    last_scanned_at = excluded.last_scanned_at,
    scan_job_id = excluded.scan_job_id,
    has_warning = excluded.has_warning,
    warning_message = excluded.warning_message,
    warning_category = excluded.warning_category,
    has_error = excluded.has_error,
    error_message = excluded.error_message,
    error_category = excluded.error_category;

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

-- name: GetLibraryWarnings :many
SELECT * FROM scan_state
WHERE library_id = ? AND has_warning = TRUE
ORDER BY file_path ASC;

-- name: CountLibraryWarnings :one
SELECT COUNT(*) FROM scan_state
WHERE library_id = ? AND has_warning = TRUE;

-- name: SetScanStateWarning :exec
UPDATE scan_state
SET has_warning = ?,
    warning_message = ?,
    warning_category = ?
WHERE library_id = ? AND file_path = ?;

-- name: ClearScanStateWarning :exec
UPDATE scan_state
SET has_warning = FALSE,
    warning_message = NULL,
    warning_category = NULL
WHERE library_id = ? AND file_path = ?;

-- name: GetLibraryErrors :many
SELECT * FROM scan_state
WHERE library_id = ? AND has_error = TRUE
ORDER BY file_path ASC;

-- name: CountLibraryErrors :one
SELECT COUNT(*) FROM scan_state
WHERE library_id = ? AND has_error = TRUE;

-- name: SetScanStateError :exec
UPDATE scan_state
SET has_error = ?,
    error_message = ?,
    error_category = ?
WHERE library_id = ? AND file_path = ?;

-- name: ClearScanStateError :exec
UPDATE scan_state
SET has_error = FALSE,
    error_message = NULL,
    error_category = NULL
WHERE library_id = ? AND file_path = ?;

-- name: GetLibraryIssues :many
SELECT * FROM scan_state
WHERE library_id = ? AND (has_warning = TRUE OR has_error = TRUE)
ORDER BY has_error DESC, file_path ASC;

-- name: CountLibraryIssues :one
SELECT
    COUNT(CASE WHEN has_error = TRUE THEN 1 END) as error_count,
    COUNT(CASE WHEN has_warning = TRUE THEN 1 END) as warning_count
FROM scan_state
WHERE library_id = ?;
