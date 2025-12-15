-- name: CreateImage :one
INSERT INTO media_images (
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
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
)
ON CONFLICT (media_type, entity_id, image_type, file_path, COALESCE(file_hash, ''))
DO UPDATE SET
    media_id = EXCLUDED.media_id,
    source_type = EXCLUDED.source_type,
    external_url = EXCLUDED.external_url,
    local_cache_path = EXCLUDED.local_cache_path,
    width = EXCLUDED.width,
    height = EXCLUDED.height,
    file_size_bytes = EXCLUDED.file_size_bytes,
    mime_type = EXCLUDED.mime_type,
    language = EXCLUDED.language,
    priority = EXCLUDED.priority,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetImageByID :one
SELECT * FROM media_images
WHERE id = $1
LIMIT 1;

-- name: ListImagesByMediaID :many
SELECT * FROM media_images
WHERE media_id = $1
ORDER BY image_type, priority ASC;

-- name: ListImagesByEntity :many
SELECT * FROM media_images
WHERE media_type = $1 AND entity_id = $2
ORDER BY image_type, priority ASC;

-- name: GetImageByTypeAndEntity :one
SELECT * FROM media_images
WHERE media_type = $1 AND entity_id = $2 AND image_type = $3
ORDER BY priority ASC
LIMIT 1;

-- name: GetImageByTypeAndMediaID :one
SELECT * FROM media_images
WHERE media_id = $1 AND image_type = $2
ORDER BY priority ASC
LIMIT 1;

-- name: UpdateImage :exec
UPDATE media_images
SET
    media_id = $1,
    media_type = $2,
    entity_id = $3,
    image_type = $4,
    source_type = $5,
    file_path = $6,
    external_url = $7,
    local_cache_path = $8,
    width = $9,
    height = $10,
    file_size_bytes = $11,
    mime_type = $12,
    file_hash = $13,
    language = $14,
    priority = $15,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $16;

-- name: DeleteImage :exec
DELETE FROM media_images
WHERE id = $1;

-- name: DeleteImagesByMediaID :exec
DELETE FROM media_images
WHERE media_id = $1;

-- name: DeleteImagesByEntity :exec
DELETE FROM media_images
WHERE media_type = $1 AND entity_id = $2;

-- name: ListImagesBySource :many
SELECT * FROM media_images
WHERE source_type = $1
ORDER BY created_at DESC;

-- name: ListOrphanImages :many
-- This query returns all images for validation in application code
-- The application will check if files actually exist
SELECT * FROM media_images
WHERE source_type = 'local' AND file_path IS NOT NULL
ORDER BY created_at DESC;

-- name: CountImagesByMediaID :one
SELECT COUNT(*) FROM media_images
WHERE media_id = $1;

-- name: CountImagesByEntity :one
SELECT COUNT(*) FROM media_images
WHERE media_type = $1 AND entity_id = $2;

-- name: GetImagesByHash :many
SELECT * FROM media_images
WHERE file_hash = $1;

-- name: DeleteImagesByHash :exec
DELETE FROM media_images
WHERE file_hash = $1;

-- name: GetImageByFilePath :one
SELECT * FROM media_images
WHERE file_path = $1
LIMIT 1;

-- name: GetAllFileHashes :many
SELECT DISTINCT file_hash FROM media_images
WHERE file_hash IS NOT NULL
ORDER BY file_hash;

-- name: GetImageByExternalURL :one
SELECT * FROM media_images
WHERE external_url = $1 AND media_id = $2
LIMIT 1;

-- name: DeleteOrphanedEntityImages :execresult
-- Delete images for entities that no longer exist (tv_show, tv_season, music_album, music_artist).
-- This handles the polymorphic entity_id references that don't have foreign key constraints.
DELETE FROM media_images
WHERE
    (media_type = 'tv_show' AND entity_id NOT IN (SELECT id FROM tv_shows))
    OR (media_type = 'tv_season' AND entity_id NOT IN (SELECT id FROM tv_seasons))
    OR (media_type = 'music_album' AND entity_id NOT IN (SELECT id FROM music_albums))
    OR (media_type = 'music_artist' AND entity_id NOT IN (SELECT id FROM music_artists));

-- name: CountOrphanedEntityImages :one
-- Count images for entities that no longer exist (for reporting before cleanup).
SELECT COUNT(*) FROM media_images
WHERE
    (media_type = 'tv_show' AND entity_id NOT IN (SELECT id FROM tv_shows))
    OR (media_type = 'tv_season' AND entity_id NOT IN (SELECT id FROM tv_seasons))
    OR (media_type = 'music_album' AND entity_id NOT IN (SELECT id FROM music_albums))
    OR (media_type = 'music_artist' AND entity_id NOT IN (SELECT id FROM music_artists));
