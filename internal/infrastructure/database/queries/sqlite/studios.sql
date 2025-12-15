-- name: GetStudioByID :one
SELECT * FROM studios WHERE id = ?;

-- name: GetStudioByName :one
SELECT * FROM studios WHERE name = ?;

-- name: GetStudioByTMDbID :one
SELECT * FROM studios WHERE tmdb_id = ?;

-- name: CreateStudio :one
INSERT INTO studios (name, logo_path, tmdb_id)
VALUES (?, ?, ?)
RETURNING *;

-- name: UpdateStudio :exec
UPDATE studios
SET name = ?, logo_path = ?, tmdb_id = ?
WHERE id = ?;

-- name: DeleteStudio :exec
DELETE FROM studios WHERE id = ?;

-- name: ListStudios :many
SELECT * FROM studios ORDER BY name;

-- name: GetStudiosForEntity :many
SELECT s.*
FROM studios s
JOIN media_studios ms ON s.id = ms.studio_id
WHERE ms.media_type = ? AND ms.entity_id = ?
ORDER BY s.name;

-- name: AddMediaStudio :exec
INSERT OR IGNORE INTO media_studios (media_type, entity_id, studio_id)
VALUES (?, ?, ?);

-- name: RemoveMediaStudio :exec
DELETE FROM media_studios
WHERE media_type = ? AND entity_id = ? AND studio_id = ?;

-- name: ClearStudiosForEntity :exec
DELETE FROM media_studios
WHERE media_type = ? AND entity_id = ?;
