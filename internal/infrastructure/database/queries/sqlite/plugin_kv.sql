-- name: PluginKVGet :one
SELECT value, expires_at
FROM plugin_kv
WHERE plugin_id = ? AND key = ?
  AND (expires_at IS NULL OR expires_at > datetime('now'));

-- name: PluginKVSet :exec
INSERT INTO plugin_kv (plugin_id, key, value, expires_at, created_at, updated_at)
VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))
ON CONFLICT(plugin_id, key) DO UPDATE SET
    value = excluded.value,
    expires_at = excluded.expires_at,
    updated_at = datetime('now');

-- name: PluginKVDelete :exec
DELETE FROM plugin_kv WHERE plugin_id = ? AND key = ?;

-- name: PluginKVList :many
SELECT key FROM plugin_kv
WHERE plugin_id = ?
  AND (? = '' OR key LIKE ? || '%')
  AND (expires_at IS NULL OR expires_at > datetime('now'))
ORDER BY key
LIMIT ?;

-- name: PluginKVDeleteExpired :exec
DELETE FROM plugin_kv WHERE expires_at IS NOT NULL AND expires_at <= datetime('now');

-- name: PluginKVDeleteByPlugin :exec
DELETE FROM plugin_kv WHERE plugin_id = ?;

-- name: PluginKVCount :one
SELECT COUNT(*) as count FROM plugin_kv
WHERE plugin_id = ?
  AND (expires_at IS NULL OR expires_at > datetime('now'));

-- name: PluginKVTotalSize :one
SELECT COALESCE(SUM(LENGTH(value)), 0) as total_bytes
FROM plugin_kv
WHERE plugin_id = ?
  AND (expires_at IS NULL OR expires_at > datetime('now'));
