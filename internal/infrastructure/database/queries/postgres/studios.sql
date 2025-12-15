-- name: GetStudioByID :one
SELECT * FROM studios WHERE id = $1;

-- name: GetStudioByName :one
SELECT * FROM studios WHERE name = $1;

-- name: GetStudioByTMDbID :one
SELECT * FROM studios WHERE tmdb_id = $1;

-- name: CreateStudio :one
INSERT INTO studios (name, logo_path, tmdb_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateStudio :exec
UPDATE studios
SET name = $1, logo_path = $2, tmdb_id = $3
WHERE id = $4;

-- name: DeleteStudio :exec
DELETE FROM studios WHERE id = $1;

-- name: ListStudios :many
SELECT * FROM studios ORDER BY name;

-- name: GetStudiosForEntity :many
SELECT s.*
FROM studios s
JOIN media_studios ms ON s.id = ms.studio_id
WHERE ms.media_type = $1 AND ms.entity_id = $2
ORDER BY s.name;

-- name: AddMediaStudio :exec
INSERT INTO media_studios (media_type, entity_id, studio_id)
VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING;

-- name: RemoveMediaStudio :exec
DELETE FROM media_studios
WHERE media_type = $1 AND entity_id = $2 AND studio_id = $3;

-- name: ClearStudiosForEntity :exec
DELETE FROM media_studios
WHERE media_type = $1 AND entity_id = $2;
