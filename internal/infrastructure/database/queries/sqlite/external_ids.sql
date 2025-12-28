-- name: UpsertExternalID :exec
-- Upserts an external ID for any entity type (movies, tv_shows, tv_seasons, etc.)
-- Uses media_type + entity_id for polymorphic lookup
INSERT INTO media_external_ids (
    media_id,
    media_type,
    entity_id,
    provider,
    external_id,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))
ON CONFLICT(media_type, entity_id, provider) DO UPDATE SET
    external_id = excluded.external_id,
    updated_at = datetime('now');

-- name: GetExternalID :one
SELECT * FROM media_external_ids
WHERE media_type = ? AND entity_id = ? AND provider = ?;

-- name: GetExternalIDsByMedia :many
-- Gets external IDs by media_type and entity_id (polymorphic)
SELECT * FROM media_external_ids
WHERE media_type = ? AND entity_id = ?
ORDER BY provider;

-- name: GetExternalIDsByMediaID :many
-- Legacy query: gets external IDs by media table ID (for backward compatibility)
SELECT * FROM media_external_ids
WHERE media_id = ?
ORDER BY provider;

-- name: GetExternalIDsByMediaIDBatch :many
-- Batch fetch: gets external IDs for multiple media IDs
-- Note: sqlc doesn't support array params in SQLite, so this is implemented
-- in the repository using a dynamic query or multiple calls
SELECT * FROM media_external_ids
WHERE media_id IN (sqlc.slice('media_ids'))
ORDER BY media_id, provider;

-- name: GetMediaByExternalID :one
-- Returns entity_id for the given provider/external_id combination
SELECT entity_id FROM media_external_ids
WHERE provider = ? AND external_id = ?;

-- name: GetEntityByExternalID :one
-- Returns full entity info for external ID lookup
SELECT media_type, entity_id FROM media_external_ids
WHERE provider = ? AND external_id = ?;

-- name: DeleteExternalID :exec
DELETE FROM media_external_ids
WHERE media_type = ? AND entity_id = ? AND provider = ?;

-- name: DeleteExternalIDsByMedia :exec
-- Deletes by media_type and entity_id
DELETE FROM media_external_ids
WHERE media_type = ? AND entity_id = ?;

-- name: DeleteExternalIDsByMediaID :exec
-- Legacy: deletes by media table ID
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

-- name: GetPersonByExternalID :one
-- Looks up a person by external ID (e.g., TMDb person ID)
SELECT entity_id FROM media_external_ids
WHERE media_type = 'person' AND provider = ? AND external_id = ?;

-- name: GetStudioByExternalID :one
-- Looks up a studio by external ID (e.g., TMDb company ID)
SELECT entity_id FROM media_external_ids
WHERE media_type = 'studio' AND provider = ? AND external_id = ?;

-- name: GetPersonExternalIDs :many
-- Gets all external IDs for a person
SELECT provider, external_id FROM media_external_ids
WHERE media_type = 'person' AND entity_id = ?
ORDER BY provider;

-- name: GetStudioExternalIDs :many
-- Gets all external IDs for a studio
SELECT provider, external_id FROM media_external_ids
WHERE media_type = 'studio' AND entity_id = ?
ORDER BY provider;
