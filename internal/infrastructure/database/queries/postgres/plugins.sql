-- name: GetPlugin :one
SELECT id, name, version, description, author, license, homepage,
       categories, is_builtin, enabled, path,
       health_status, last_heartbeat, restart_count,
       settings, settings_schema,
       installed_at, updated_at
FROM plugins
WHERE id = $1;

-- name: ListPlugins :many
SELECT id, name, version, description, author, license, homepage,
       categories, is_builtin, enabled, path,
       health_status, last_heartbeat, restart_count,
       settings, settings_schema,
       installed_at, updated_at
FROM plugins
ORDER BY is_builtin DESC, name;

-- name: ListEnabledPlugins :many
SELECT id, name, version, description, author, license, homepage,
       categories, is_builtin, enabled, path,
       health_status, last_heartbeat, restart_count,
       settings, settings_schema,
       installed_at, updated_at
FROM plugins
WHERE enabled = TRUE
ORDER BY is_builtin DESC, name;

-- name: ListPluginsByCategory :many
SELECT id, name, version, description, author, license, homepage,
       categories, is_builtin, enabled, path,
       health_status, last_heartbeat, restart_count,
       settings, settings_schema,
       installed_at, updated_at
FROM plugins
WHERE categories ? $1
ORDER BY is_builtin DESC, name;

-- name: CreatePlugin :exec
INSERT INTO plugins (
    id, name, version, description, author, license, homepage,
    categories, is_builtin, enabled, path,
    health_status, last_heartbeat, restart_count,
    installed_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW(), NOW());

-- name: UpdatePlugin :exec
UPDATE plugins SET
    name = $1,
    version = $2,
    description = $3,
    author = $4,
    license = $5,
    homepage = $6,
    categories = $7,
    path = $8,
    updated_at = NOW()
WHERE id = $9;

-- name: DeletePlugin :exec
DELETE FROM plugins WHERE id = $1 AND is_builtin = FALSE;

-- name: EnablePlugin :exec
UPDATE plugins SET enabled = TRUE, updated_at = NOW() WHERE id = $1;

-- name: DisablePlugin :exec
UPDATE plugins SET enabled = FALSE, updated_at = NOW() WHERE id = $1;

-- name: UpdatePluginHealth :exec
UPDATE plugins SET
    health_status = $1,
    last_heartbeat = NOW(),
    updated_at = NOW()
WHERE id = $2;

-- name: IncrementPluginRestartCount :exec
UPDATE plugins SET
    restart_count = restart_count + 1,
    updated_at = NOW()
WHERE id = $1;

-- name: ResetPluginRestartCount :exec
UPDATE plugins SET
    restart_count = 0,
    updated_at = NOW()
WHERE id = $1;

-- name: PluginExists :one
SELECT EXISTS(SELECT 1 FROM plugins WHERE id = $1) as plugin_exists;

-- name: UpsertPlugin :exec
INSERT INTO plugins (
    id, name, version, description, author, license, homepage,
    categories, is_builtin, enabled, path,
    health_status, restart_count,
    installed_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, FALSE, TRUE, $9, 'unknown', 0, NOW(), NOW())
ON CONFLICT(id) DO UPDATE SET
    name = EXCLUDED.name,
    version = EXCLUDED.version,
    description = EXCLUDED.description,
    author = EXCLUDED.author,
    license = EXCLUDED.license,
    homepage = EXCLUDED.homepage,
    categories = EXCLUDED.categories,
    path = EXCLUDED.path,
    updated_at = NOW();

-- name: GetPluginSettings :one
SELECT settings FROM plugins WHERE id = $1;

-- name: UpdatePluginSettings :exec
UPDATE plugins SET
    settings = $1,
    updated_at = NOW()
WHERE id = $2;

-- name: UpdatePluginSettingsSchema :exec
UPDATE plugins SET
    settings_schema = $1,
    updated_at = NOW()
WHERE id = $2;
