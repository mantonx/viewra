-- name: GetPluginAPIKey :one
SELECT id, plugin_id, key_hash, permissions, expires_at, created_at
FROM plugin_api_keys
WHERE id = $1
  AND (expires_at IS NULL OR expires_at > NOW());

-- name: GetPluginAPIKeyByHash :one
SELECT id, plugin_id, key_hash, permissions, expires_at, created_at
FROM plugin_api_keys
WHERE key_hash = $1
  AND (expires_at IS NULL OR expires_at > NOW());

-- name: ListPluginAPIKeys :many
SELECT id, plugin_id, key_hash, permissions, expires_at, created_at
FROM plugin_api_keys
WHERE plugin_id = $1
ORDER BY created_at DESC;

-- name: CreatePluginAPIKey :exec
INSERT INTO plugin_api_keys (id, plugin_id, key_hash, permissions, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, NOW());

-- name: DeletePluginAPIKey :exec
DELETE FROM plugin_api_keys WHERE id = $1;

-- name: DeletePluginAPIKeysByPlugin :exec
DELETE FROM plugin_api_keys WHERE plugin_id = $1;

-- name: DeleteExpiredPluginAPIKeys :exec
DELETE FROM plugin_api_keys WHERE expires_at IS NOT NULL AND expires_at <= NOW();

-- name: CountPluginAPIKeys :one
SELECT COUNT(*) as count FROM plugin_api_keys
WHERE plugin_id = $1
  AND (expires_at IS NULL OR expires_at > NOW());
