-- Library queries for PostgreSQL

-- name: CreateLibrary :one
INSERT INTO libraries (name, path, type, created_at, updated_at)
VALUES ($1, $2, $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
RETURNING *;

-- name: GetLibraryByID :one
SELECT * FROM libraries
WHERE id = $1
LIMIT 1;

-- name: GetLibraryByPath :one
SELECT * FROM libraries
WHERE path = $1
LIMIT 1;

-- name: ListLibraries :many
SELECT * FROM libraries
ORDER BY name;

-- name: ListLibrariesByType :many
SELECT * FROM libraries
WHERE type = $1
ORDER BY name;

-- name: UpdateLibrary :one
UPDATE libraries
SET name = $1,
    path = $2,
    type = $3,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $4
RETURNING *;

-- name: DeleteLibrary :exec
DELETE FROM libraries
WHERE id = $1;

-- name: LibraryExistsByPath :one
SELECT COUNT(*)
FROM libraries
WHERE path = $1;

-- name: LibraryExistsByID :one
SELECT COUNT(*)
FROM libraries
WHERE id = $1;

-- name: CountLibraries :one
SELECT COUNT(*) FROM libraries;

-- name: CountLibrariesByType :one
SELECT COUNT(*) FROM libraries
WHERE type = $1;

-- name: UpdateLibraryMonitoring :one
UPDATE libraries
SET monitoring_enabled = $1,
    monitoring_config = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $3
RETURNING *;

-- name: ListMonitoredLibraries :many
SELECT * FROM libraries
WHERE monitoring_enabled = 1
ORDER BY name;

-- name: UpdateLibraryLastScannedAt :exec
UPDATE libraries
SET last_scanned_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1;
