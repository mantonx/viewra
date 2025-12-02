-- Session queries for PostgreSQL

-- name: CreateSession :one
INSERT INTO sessions (public_id, user_id, refresh_token_hash, user_agent, ip_address, created_at, last_used_at, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetSessionByID :one
SELECT * FROM sessions
WHERE id = $1
LIMIT 1;

-- name: GetSessionByPublicID :one
SELECT * FROM sessions
WHERE public_id = $1
LIMIT 1;

-- name: GetSessionByTokenHash :one
SELECT * FROM sessions
WHERE refresh_token_hash = $1
LIMIT 1;

-- name: GetSessionsByUserID :many
SELECT * FROM sessions
WHERE user_id = $1
ORDER BY last_used_at DESC;

-- name: UpdateSessionLastUsed :exec
UPDATE sessions
SET last_used_at = $1
WHERE id = $2;

-- name: DeleteSession :exec
DELETE FROM sessions
WHERE id = $1;

-- name: DeleteSessionsByUserID :exec
DELETE FROM sessions
WHERE user_id = $1;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions
WHERE expires_at < $1;

-- name: CountSessionsByUserID :one
SELECT COUNT(*) FROM sessions
WHERE user_id = $1;
