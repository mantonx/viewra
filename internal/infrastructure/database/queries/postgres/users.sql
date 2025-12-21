-- User queries for PostgreSQL

-- name: CreateUser :one
INSERT INTO users (public_id, username, display_name, password_hash, is_admin, is_disabled, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1
LIMIT 1;

-- name: GetUserByPublicID :one
SELECT * FROM users
WHERE public_id = $1
LIMIT 1;

-- name: GetUserByUsername :one
SELECT * FROM users
WHERE LOWER(username) = LOWER($1)
LIMIT 1;

-- name: UpdateUser :one
UPDATE users
SET display_name = $1,
    is_admin = $2,
    is_disabled = $3,
    updated_at = $4
WHERE id = $5
RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = $1,
    updated_at = $2
WHERE id = $3;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;

-- name: ListUsers :many
SELECT * FROM users
ORDER BY username
LIMIT $1 OFFSET $2;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: ExistsAnyUser :one
SELECT EXISTS(SELECT 1 FROM users);

-- Location Preferences (stored in users table)

-- name: GetUserLocation :one
SELECT id, location_latitude, location_longitude, location_timezone, location_enabled, location_name
FROM users
WHERE id = $1;

-- name: UpdateUserLocation :exec
UPDATE users
SET location_latitude = $1,
    location_longitude = $2,
    location_timezone = $3,
    location_enabled = $4,
    location_name = $5,
    updated_at = $6
WHERE id = $7;
