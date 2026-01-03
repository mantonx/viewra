-- name: GetUserRating :one
SELECT id, user_id, entity_type, entity_id, rating, created_at, updated_at
FROM user_ratings
WHERE user_id = ? AND entity_type = ? AND entity_id = ?
LIMIT 1;

-- name: ListUserRatings :many
SELECT id, user_id, entity_type, entity_id, rating, created_at, updated_at
FROM user_ratings
WHERE user_id = ?
ORDER BY updated_at DESC;

-- name: ListUserRatingsByType :many
SELECT id, user_id, entity_type, entity_id, rating, created_at, updated_at
FROM user_ratings
WHERE user_id = ? AND entity_type = ?
ORDER BY updated_at DESC;

-- name: ListUserRatingsByRating :many
SELECT id, user_id, entity_type, entity_id, rating, created_at, updated_at
FROM user_ratings
WHERE user_id = ? AND rating = ?
ORDER BY updated_at DESC;

-- name: ListUserRatingsByTypeAndRating :many
SELECT id, user_id, entity_type, entity_id, rating, created_at, updated_at
FROM user_ratings
WHERE user_id = ? AND entity_type = ? AND rating = ?
ORDER BY updated_at DESC;

-- name: ListEntityIDsByRating :many
SELECT entity_id
FROM user_ratings
WHERE user_id = ? AND rating = ?
ORDER BY updated_at DESC
LIMIT sqlc.arg('limit');

-- name: ListEntityIDsByTypeAndRating :many
SELECT entity_id
FROM user_ratings
WHERE user_id = ? AND entity_type = ? AND rating = ?
ORDER BY updated_at DESC
LIMIT sqlc.arg('limit');

-- name: ListEntityIDsByPositiveRating :many
SELECT entity_id
FROM user_ratings
WHERE user_id = ? AND rating IN ('favorite', 'up')
ORDER BY updated_at DESC
LIMIT sqlc.arg('limit');

-- name: ListEntityIDsByTypeAndPositiveRating :many
SELECT entity_id
FROM user_ratings
WHERE user_id = ? AND entity_type = ? AND rating IN ('favorite', 'up')
ORDER BY updated_at DESC
LIMIT sqlc.arg('limit');

-- name: UpsertUserRating :one
INSERT INTO user_ratings (user_id, entity_type, entity_id, rating, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(user_id, entity_type, entity_id) DO UPDATE SET
    rating = excluded.rating,
    updated_at = excluded.updated_at
RETURNING *;

-- name: DeleteUserRating :exec
DELETE FROM user_ratings
WHERE user_id = ? AND entity_type = ? AND entity_id = ?;

-- name: DeleteAllUserRatings :exec
DELETE FROM user_ratings
WHERE user_id = ?;

-- name: HasUserRatings :one
SELECT EXISTS(SELECT 1 FROM user_ratings WHERE user_id = ?) AS has_ratings;

-- name: CountUserRatingsByRating :one
SELECT COUNT(*) AS count
FROM user_ratings
WHERE user_id = ? AND rating = ?;
