-- Settings queries for SQLite

-- name: GetSystemSetting :one
SELECT * FROM system_settings
WHERE key = ?
LIMIT 1;

-- name: GetSystemSettingsByCategory :many
SELECT * FROM system_settings
WHERE category = ?
ORDER BY key;

-- name: GetAllSystemSettings :many
SELECT * FROM system_settings
ORDER BY category, key;

-- name: UpsertSystemSetting :exec
INSERT INTO system_settings (key, value, value_type, category, description, updated_at, updated_by)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(key) DO UPDATE SET
    value = excluded.value,
    updated_at = excluded.updated_at,
    updated_by = excluded.updated_by;

-- name: DeleteSystemSetting :exec
DELETE FROM system_settings
WHERE key = ?;

-- name: GetUserSetting :one
SELECT * FROM user_settings
WHERE user_id = ? AND key = ?
LIMIT 1;

-- name: GetAllUserSettings :many
SELECT * FROM user_settings
WHERE user_id = ?
ORDER BY key;

-- name: UpsertUserSetting :exec
INSERT INTO user_settings (user_id, key, value, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(user_id, key) DO UPDATE SET
    value = excluded.value,
    updated_at = excluded.updated_at;

-- name: DeleteUserSetting :exec
DELETE FROM user_settings
WHERE user_id = ? AND key = ?;

-- name: DeleteAllUserSettings :exec
DELETE FROM user_settings
WHERE user_id = ?;
