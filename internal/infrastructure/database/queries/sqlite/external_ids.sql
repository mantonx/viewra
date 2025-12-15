-- name: UpsertExternalID :exec
INSERT INTO media_external_ids (
    media_id,
    provider,
    external_id,
    created_at,
    updated_at
) VALUES (?, ?, ?, datetime('now'), datetime('now'))
ON CONFLICT(media_id, provider) DO UPDATE SET
    external_id = excluded.external_id,
    updated_at = datetime('now');

-- name: GetExternalID :one
SELECT * FROM media_external_ids
WHERE media_id = ? AND provider = ?;

-- name: GetExternalIDsByMedia :many
SELECT * FROM media_external_ids
WHERE media_id = ?
ORDER BY provider;

-- name: GetMediaByExternalID :one
SELECT media_id FROM media_external_ids
WHERE provider = ? AND external_id = ?;

-- name: DeleteExternalID :exec
DELETE FROM media_external_ids
WHERE media_id = ? AND provider = ?;

-- name: DeleteExternalIDsByMedia :exec
DELETE FROM media_external_ids WHERE media_id = ?;

-- name: UpsertMetadataSource :exec
INSERT INTO media_metadata_sources (
    media_id,
    field_name,
    plugin_id,
    raw_value,
    updated_at
) VALUES (?, ?, ?, ?, datetime('now'))
ON CONFLICT(media_id, field_name, plugin_id) DO UPDATE SET
    raw_value = excluded.raw_value,
    updated_at = datetime('now');

-- name: GetMetadataSource :one
SELECT * FROM media_metadata_sources
WHERE media_id = ? AND field_name = ? AND plugin_id = ?;

-- name: GetMetadataSourcesByMedia :many
SELECT * FROM media_metadata_sources
WHERE media_id = ?
ORDER BY field_name, plugin_id;

-- name: GetMetadataSourcesByField :many
SELECT * FROM media_metadata_sources
WHERE media_id = ? AND field_name = ?
ORDER BY plugin_id;

-- name: DeleteMetadataSource :exec
DELETE FROM media_metadata_sources
WHERE media_id = ? AND field_name = ? AND plugin_id = ?;

-- name: DeleteMetadataSourcesByMedia :exec
DELETE FROM media_metadata_sources WHERE media_id = ?;

-- name: DeleteMetadataSourcesByPlugin :exec
DELETE FROM media_metadata_sources WHERE plugin_id = ?;
