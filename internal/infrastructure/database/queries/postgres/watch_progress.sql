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
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateWatchProgress :one
UPDATE watch_progress
SET position = $1,
    duration = $2,
    watched = $3,
    last_watched = $4,
    updated_at = $5
WHERE id = $6
RETURNING *;

-- name: GetWatchProgressByID :one
SELECT * FROM watch_progress
WHERE id = $1;

-- name: GetWatchProgressByMediaID :one
SELECT * FROM watch_progress
WHERE media_id = $1
LIMIT 1;

-- name: GetWatchProgressByMediaIDAndUserID :one
SELECT * FROM watch_progress
WHERE media_id = $1 AND user_id = $2
LIMIT 1;

-- name: ListWatchProgressByUserID :many
SELECT * FROM watch_progress
WHERE user_id = $1
ORDER BY last_watched DESC
LIMIT $2 OFFSET $3;

-- name: ListWatchedByUserID :many
SELECT * FROM watch_progress
WHERE user_id = $1
  AND watched = TRUE
ORDER BY last_watched DESC
LIMIT $2 OFFSET $3;

-- name: ListInProgressByUserID :many
SELECT * FROM watch_progress
WHERE user_id = $1
  AND watched = FALSE
  AND position > 0
ORDER BY last_watched DESC
LIMIT $2 OFFSET $3;

-- name: DeleteWatchProgress :exec
DELETE FROM watch_progress
WHERE id = $1;

-- name: DeleteWatchProgressByMediaID :exec
DELETE FROM watch_progress
WHERE media_id = $1;

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
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT(media_id, user_id)
DO UPDATE SET
    position = EXCLUDED.position,
    duration = EXCLUDED.duration,
    watched = EXCLUDED.watched,
    last_watched = EXCLUDED.last_watched,
    updated_at = EXCLUDED.updated_at
RETURNING *;
