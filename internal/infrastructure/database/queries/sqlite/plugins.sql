-- name: GetPlugin :one
SELECT id, name, version, description, author, license, homepage,
       categories, is_builtin, enabled, path,
       health_status, last_heartbeat, restart_count,
       installed_at, updated_at
FROM plugins
WHERE id = ?;

-- name: ListPlugins :many
SELECT id, name, version, description, author, license, homepage,
       categories, is_builtin, enabled, path,
       health_status, last_heartbeat, restart_count,
       installed_at, updated_at
FROM plugins
ORDER BY is_builtin DESC, name;

-- name: ListEnabledPlugins :many
SELECT id, name, version, description, author, license, homepage,
       categories, is_builtin, enabled, path,
       health_status, last_heartbeat, restart_count,
       installed_at, updated_at
FROM plugins
WHERE enabled = 1
ORDER BY is_builtin DESC, name;

-- name: ListPluginsByCategory :many
SELECT id, name, version, description, author, license, homepage,
       categories, is_builtin, enabled, path,
       health_status, last_heartbeat, restart_count,
       installed_at, updated_at
FROM plugins
WHERE categories LIKE '%' || ? || '%'
ORDER BY is_builtin DESC, name;

-- name: CreatePlugin :exec
INSERT INTO plugins (
    id, name, version, description, author, license, homepage,
    categories, is_builtin, enabled, path,
    health_status, last_heartbeat, restart_count,
    installed_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'));

-- name: UpdatePlugin :exec
UPDATE plugins SET
    name = ?,
    version = ?,
    description = ?,
    author = ?,
    license = ?,
    homepage = ?,
    categories = ?,
    path = ?,
    updated_at = datetime('now')
WHERE id = ?;

-- name: DeletePlugin :exec
DELETE FROM plugins WHERE id = ? AND is_builtin = 0;

-- name: EnablePlugin :exec
UPDATE plugins SET enabled = 1, updated_at = datetime('now') WHERE id = ?;

-- name: DisablePlugin :exec
UPDATE plugins SET enabled = 0, updated_at = datetime('now') WHERE id = ?;

-- name: UpdatePluginHealth :exec
UPDATE plugins SET
    health_status = ?,
    last_heartbeat = datetime('now'),
    updated_at = datetime('now')
WHERE id = ?;

-- name: IncrementPluginRestartCount :exec
UPDATE plugins SET
    restart_count = restart_count + 1,
    updated_at = datetime('now')
WHERE id = ?;

-- name: ResetPluginRestartCount :exec
UPDATE plugins SET
    restart_count = 0,
    updated_at = datetime('now')
WHERE id = ?;

-- name: PluginExists :one
SELECT EXISTS(SELECT 1 FROM plugins WHERE id = ?) as plugin_exists;
