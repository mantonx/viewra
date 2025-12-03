-- Settings queries for PostgreSQL

-- name: GetSystemSetting :one
SELECT * FROM system_settings
WHERE key = $1
LIMIT 1;

-- name: GetSystemSettingsByCategory :many
SELECT * FROM system_settings
WHERE category = $1
ORDER BY key;

-- name: GetAllSystemSettings :many
SELECT * FROM system_settings
ORDER BY category, key;

-- name: UpsertSystemSetting :exec
INSERT INTO system_settings (key, value, value_type, category, description, updated_at, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT(key) DO UPDATE SET
    value = EXCLUDED.value,
    updated_at = EXCLUDED.updated_at,
    updated_by = EXCLUDED.updated_by;

-- name: DeleteSystemSetting :exec
DELETE FROM system_settings
WHERE key = $1;

-- name: GetUserSetting :one
SELECT * FROM user_settings
WHERE user_id = $1 AND key = $2
LIMIT 1;

-- name: GetAllUserSettings :many
SELECT * FROM user_settings
WHERE user_id = $1
ORDER BY key;

-- name: UpsertUserSetting :exec
INSERT INTO user_settings (user_id, key, value, updated_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT(user_id, key) DO UPDATE SET
    value = EXCLUDED.value,
    updated_at = EXCLUDED.updated_at;

-- name: DeleteUserSetting :exec
DELETE FROM user_settings
WHERE user_id = $1 AND key = $2;

-- name: DeleteAllUserSettings :exec
DELETE FROM user_settings
WHERE user_id = $1;
