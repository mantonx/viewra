-- name: PluginKVGet :one
SELECT value, expires_at
FROM plugin_kv
WHERE plugin_id = $1 AND key = $2
  AND (expires_at IS NULL OR expires_at > NOW());

-- name: PluginKVSet :exec
INSERT INTO plugin_kv (plugin_id, key, value, expires_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, NOW(), NOW())
ON CONFLICT(plugin_id, key) DO UPDATE SET
    value = EXCLUDED.value,
    expires_at = EXCLUDED.expires_at,
    updated_at = NOW();

-- name: PluginKVDelete :exec
DELETE FROM plugin_kv WHERE plugin_id = $1 AND key = $2;

-- name: PluginKVList :many
SELECT key FROM plugin_kv
WHERE plugin_id = $1
  AND ($2 = '' OR key LIKE $3 || '%')
  AND (expires_at IS NULL OR expires_at > NOW())
ORDER BY key
LIMIT $4;

-- name: PluginKVDeleteExpired :exec
DELETE FROM plugin_kv WHERE expires_at IS NOT NULL AND expires_at <= NOW();

-- name: PluginKVDeleteByPlugin :exec
DELETE FROM plugin_kv WHERE plugin_id = $1;

-- name: PluginKVCount :one
SELECT COUNT(*) as count FROM plugin_kv
WHERE plugin_id = $1
  AND (expires_at IS NULL OR expires_at > NOW());

-- name: PluginKVTotalSize :one
SELECT COALESCE(SUM(LENGTH(value)), 0)::bigint as total_bytes
FROM plugin_kv
WHERE plugin_id = $1
  AND (expires_at IS NULL OR expires_at > NOW());
