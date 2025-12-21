-- User queries for SQLite

-- name: CreateUser :one
INSERT INTO users (public_id, username, display_name, password_hash, is_admin, is_disabled, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = ?
LIMIT 1;

-- name: GetUserByPublicID :one
SELECT * FROM users
WHERE public_id = ?
LIMIT 1;

-- name: GetUserByUsername :one
SELECT * FROM users
WHERE LOWER(username) = LOWER(?)
LIMIT 1;

-- name: UpdateUser :one
UPDATE users
SET display_name = ?,
    is_admin = ?,
    is_disabled = ?,
    updated_at = ?
WHERE id = ?
RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = ?,
    updated_at = ?
WHERE id = ?;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = ?;

-- name: ListUsers :many
SELECT * FROM users
ORDER BY username
LIMIT ? OFFSET ?;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: ExistsAnyUser :one
SELECT EXISTS(SELECT 1 FROM users);

-- Location Preferences (stored in users table)

-- name: GetUserLocation :one
SELECT id, location_latitude, location_longitude, location_timezone, location_enabled, location_name
FROM users
WHERE id = ?;

-- name: UpdateUserLocation :exec
UPDATE users
SET location_latitude = ?,
    location_longitude = ?,
    location_timezone = ?,
    location_enabled = ?,
    location_name = ?,
    updated_at = ?
WHERE id = ?;
