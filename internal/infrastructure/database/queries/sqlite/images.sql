-- name: CreateImage :one
INSERT OR REPLACE INTO media_images (
    media_id,
    media_type,
    entity_id,
    image_type,
    source_type,
    file_path,
    external_url,
    local_cache_path,
    width,
    height,
    file_size_bytes,
    mime_type,
    file_hash,
    language,
    priority
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
) RETURNING *;

-- name: GetImageByID :one
SELECT * FROM media_images
WHERE id = ?
LIMIT 1;

-- name: ListImagesByMediaID :many
SELECT * FROM media_images
WHERE media_id = ?
ORDER BY image_type, priority ASC;

-- name: ListImagesByEntity :many
SELECT * FROM media_images
WHERE media_type = ? AND entity_id = ?
ORDER BY image_type, priority ASC;

-- name: GetImageByTypeAndEntity :one
SELECT * FROM media_images
WHERE media_type = ? AND entity_id = ? AND image_type = ?
ORDER BY priority ASC
LIMIT 1;

-- name: GetImageByTypeAndMediaID :one
SELECT * FROM media_images
WHERE media_id = ? AND image_type = ?
ORDER BY priority ASC
LIMIT 1;

-- name: UpdateImage :exec
UPDATE media_images
SET
    media_id = ?,
    media_type = ?,
    entity_id = ?,
    image_type = ?,
    source_type = ?,
    file_path = ?,
    external_url = ?,
    local_cache_path = ?,
    width = ?,
    height = ?,
    file_size_bytes = ?,
    mime_type = ?,
    file_hash = ?,
    language = ?,
    priority = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteImage :exec
DELETE FROM media_images
WHERE id = ?;

-- name: DeleteImagesByMediaID :exec
DELETE FROM media_images
WHERE media_id = ?;

-- name: DeleteImagesByEntity :exec
DELETE FROM media_images
WHERE media_type = ? AND entity_id = ?;

-- name: ListImagesBySource :many
SELECT * FROM media_images
WHERE source_type = ?
ORDER BY created_at DESC;

-- name: ListOrphanImages :many
-- This query returns all images for validation in application code
-- The application will check if files actually exist
SELECT * FROM media_images
WHERE source_type = 'local' AND file_path IS NOT NULL
ORDER BY created_at DESC;

-- name: CountImagesByMediaID :one
SELECT COUNT(*) FROM media_images
WHERE media_id = ?;

-- name: CountImagesByEntity :one
SELECT COUNT(*) FROM media_images
WHERE media_type = ? AND entity_id = ?;

-- name: GetImagesByHash :many
SELECT * FROM media_images
WHERE file_hash = ?;

-- name: DeleteImagesByHash :exec
DELETE FROM media_images
WHERE file_hash = ?;

-- name: GetImageByFilePath :one
SELECT * FROM media_images
WHERE file_path = ?
LIMIT 1;

-- name: GetAllFileHashes :many
SELECT DISTINCT file_hash FROM media_images
WHERE file_hash IS NOT NULL
ORDER BY file_hash;
