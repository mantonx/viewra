-- Library queries for SQLite

-- name: CreateLibrary :one
INSERT INTO libraries (name, path, type, created_at, updated_at)
VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
RETURNING *;

-- name: GetLibraryByID :one
SELECT * FROM libraries
WHERE id = ?
LIMIT 1;

-- name: GetLibraryByPath :one
SELECT * FROM libraries
WHERE path = ?
LIMIT 1;

-- name: ListLibraries :many
SELECT * FROM libraries
ORDER BY name;

-- name: ListLibrariesByType :many
SELECT * FROM libraries
WHERE type = ?
ORDER BY name;

-- name: UpdateLibrary :one
UPDATE libraries
SET name = ?,
    path = ?,
    type = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DeleteLibrary :exec
DELETE FROM libraries
WHERE id = ?;

-- name: LibraryExistsByPath :one
SELECT COUNT(*) 
FROM libraries
WHERE path = ?;

-- name: LibraryExistsByID :one
SELECT COUNT(*)
FROM libraries
WHERE id = ?;

-- name: CountLibraries :one
SELECT COUNT(*) FROM libraries;

-- name: CountLibrariesByType :one
SELECT COUNT(*) FROM libraries
WHERE type = ?;

-- name: UpdateLibraryMonitoring :one
UPDATE libraries
SET monitoring_enabled = ?,
    monitoring_config = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: ListMonitoredLibraries :many
SELECT * FROM libraries
WHERE monitoring_enabled = 1
ORDER BY name;
