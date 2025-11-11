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
SELECT COUNT(*) > 0 as exists
FROM libraries
WHERE path = $1;

-- name: LibraryExistsByID :one
SELECT COUNT(*) > 0 as exists
FROM libraries
WHERE id = $1;

-- name: CountLibraries :one
SELECT COUNT(*) FROM libraries;

-- name: CountLibrariesByType :one
SELECT COUNT(*) FROM libraries
WHERE type = $1;
