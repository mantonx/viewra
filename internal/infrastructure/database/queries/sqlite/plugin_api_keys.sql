-- name: GetPluginAPIKey :one
SELECT id, plugin_id, key_hash, permissions, expires_at, created_at
FROM plugin_api_keys
WHERE id = ?
  AND (expires_at IS NULL OR expires_at > datetime('now'));

-- name: GetPluginAPIKeyByHash :one
SELECT id, plugin_id, key_hash, permissions, expires_at, created_at
FROM plugin_api_keys
WHERE key_hash = ?
  AND (expires_at IS NULL OR expires_at > datetime('now'));

-- name: ListPluginAPIKeys :many
SELECT id, plugin_id, key_hash, permissions, expires_at, created_at
FROM plugin_api_keys
WHERE plugin_id = ?
ORDER BY created_at DESC;

-- name: CreatePluginAPIKey :exec
INSERT INTO plugin_api_keys (id, plugin_id, key_hash, permissions, expires_at, created_at)
VALUES (?, ?, ?, ?, ?, datetime('now'));

-- name: DeletePluginAPIKey :exec
DELETE FROM plugin_api_keys WHERE id = ?;

-- name: DeletePluginAPIKeysByPlugin :exec
DELETE FROM plugin_api_keys WHERE plugin_id = ?;

-- name: DeleteExpiredPluginAPIKeys :exec
DELETE FROM plugin_api_keys WHERE expires_at IS NOT NULL AND expires_at <= datetime('now');

-- name: CountPluginAPIKeys :one
SELECT COUNT(*) as count FROM plugin_api_keys
WHERE plugin_id = ?
  AND (expires_at IS NULL OR expires_at > datetime('now'));
