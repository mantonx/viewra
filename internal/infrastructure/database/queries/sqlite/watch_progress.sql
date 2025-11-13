-- name: CreateWatchProgress :one
INSERT INTO watch_progress (
    media_id,
    user_id,
    position,
    duration,
    watched,
    last_watched,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateWatchProgress :one
UPDATE watch_progress
SET position = ?,
    duration = ?,
    watched = ?,
    last_watched = ?,
    updated_at = ?
WHERE id = ?
RETURNING *;

-- name: GetWatchProgressByID :one
SELECT * FROM watch_progress
WHERE id = ?;

-- name: GetWatchProgressByMediaID :one
SELECT * FROM watch_progress
WHERE media_id = ?
LIMIT 1;

-- name: GetWatchProgressByMediaIDAndUserID :one
SELECT * FROM watch_progress
WHERE media_id = ? AND user_id = ?
LIMIT 1;

-- name: ListWatchProgressByUserID :many
SELECT * FROM watch_progress
WHERE user_id = ?
ORDER BY last_watched DESC
LIMIT ? OFFSET ?;

-- name: ListWatchedByUserID :many
SELECT * FROM watch_progress
WHERE user_id = ?
  AND watched = TRUE
ORDER BY last_watched DESC
LIMIT ? OFFSET ?;

-- name: ListInProgressByUserID :many
SELECT * FROM watch_progress
WHERE user_id = ?
  AND watched = FALSE
  AND position > 0
ORDER BY last_watched DESC
LIMIT ? OFFSET ?;

-- name: DeleteWatchProgress :exec
DELETE FROM watch_progress
WHERE id = ?;

-- name: DeleteWatchProgressByMediaID :exec
DELETE FROM watch_progress
WHERE media_id = ?;

-- name: UpsertWatchProgress :one
INSERT INTO watch_progress (
    media_id,
    user_id,
    position,
    duration,
    watched,
    last_watched,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(media_id, user_id)
DO UPDATE SET
    position = excluded.position,
    duration = excluded.duration,
    watched = excluded.watched,
    last_watched = excluded.last_watched,
    updated_at = excluded.updated_at
RETURNING *;
