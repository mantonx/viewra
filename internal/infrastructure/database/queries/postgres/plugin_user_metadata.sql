-- name: GetPluginUserMetadata :one
SELECT value, created_at, updated_at
FROM plugin_user_metadata
WHERE plugin_id = $1 AND user_id = $2 AND key = $3;

-- name: SetPluginUserMetadata :exec
INSERT INTO plugin_user_metadata (plugin_id, user_id, key, value, created_at, updated_at)
VALUES ($1, $2, $3, $4, NOW(), NOW())
ON CONFLICT(plugin_id, user_id, key) DO UPDATE SET
    value = excluded.value,
    updated_at = NOW();

-- name: DeletePluginUserMetadata :exec
DELETE FROM plugin_user_metadata WHERE plugin_id = $1 AND user_id = $2 AND key = $3;

-- name: ListPluginUserMetadataKeys :many
SELECT key FROM plugin_user_metadata
WHERE plugin_id = $1 AND user_id = $2
ORDER BY key;

-- name: DeletePluginUserMetadataByPlugin :exec
DELETE FROM plugin_user_metadata WHERE plugin_id = $1;

-- name: DeletePluginUserMetadataByUser :exec
DELETE FROM plugin_user_metadata WHERE user_id = $1;

-- name: CountPluginUserMetadata :one
SELECT COUNT(*) as count FROM plugin_user_metadata
WHERE plugin_id = $1 AND user_id = $2;
