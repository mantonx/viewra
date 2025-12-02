-- Session queries for SQLite

-- name: CreateSession :one
INSERT INTO sessions (public_id, user_id, refresh_token_hash, user_agent, ip_address, created_at, last_used_at, expires_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetSessionByID :one
SELECT * FROM sessions
WHERE id = ?
LIMIT 1;

-- name: GetSessionByPublicID :one
SELECT * FROM sessions
WHERE public_id = ?
LIMIT 1;

-- name: GetSessionByTokenHash :one
SELECT * FROM sessions
WHERE refresh_token_hash = ?
LIMIT 1;

-- name: GetSessionsByUserID :many
SELECT * FROM sessions
WHERE user_id = ?
ORDER BY last_used_at DESC;

-- name: UpdateSessionLastUsed :exec
UPDATE sessions
SET last_used_at = ?
WHERE id = ?;

-- name: DeleteSession :exec
DELETE FROM sessions
WHERE id = ?;

-- name: DeleteSessionsByUserID :exec
DELETE FROM sessions
WHERE user_id = ?;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions
WHERE expires_at < ?;

-- name: CountSessionsByUserID :one
SELECT COUNT(*) FROM sessions
WHERE user_id = ?;
