-- name: GetPluginUserMetadata :one
SELECT value, created_at, updated_at
FROM plugin_user_metadata
WHERE plugin_id = ? AND user_id = ? AND key = ?;

-- name: SetPluginUserMetadata :exec
INSERT INTO plugin_user_metadata (plugin_id, user_id, key, value, created_at, updated_at)
VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))
ON CONFLICT(plugin_id, user_id, key) DO UPDATE SET
    value = excluded.value,
    updated_at = datetime('now');

-- name: DeletePluginUserMetadata :exec
DELETE FROM plugin_user_metadata WHERE plugin_id = ? AND user_id = ? AND key = ?;

-- name: ListPluginUserMetadataKeys :many
SELECT key FROM plugin_user_metadata
WHERE plugin_id = ? AND user_id = ?
ORDER BY key;

-- name: DeletePluginUserMetadataByPlugin :exec
DELETE FROM plugin_user_metadata WHERE plugin_id = ?;

-- name: DeletePluginUserMetadataByUser :exec
DELETE FROM plugin_user_metadata WHERE user_id = ?;

-- name: CountPluginUserMetadata :one
SELECT COUNT(*) as count FROM plugin_user_metadata
WHERE plugin_id = ? AND user_id = ?;
