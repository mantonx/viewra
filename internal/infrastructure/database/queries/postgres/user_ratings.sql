-- name: GetUserRating :one
SELECT id, user_id, entity_type, entity_id, rating, created_at, updated_at
FROM user_ratings
WHERE user_id = $1 AND entity_type = $2 AND entity_id = $3
LIMIT 1;

-- name: ListUserRatings :many
SELECT id, user_id, entity_type, entity_id, rating, created_at, updated_at
FROM user_ratings
WHERE user_id = $1
ORDER BY updated_at DESC;

-- name: ListUserRatingsByType :many
SELECT id, user_id, entity_type, entity_id, rating, created_at, updated_at
FROM user_ratings
WHERE user_id = $1 AND entity_type = $2
ORDER BY updated_at DESC;

-- name: ListUserRatingsByRating :many
SELECT id, user_id, entity_type, entity_id, rating, created_at, updated_at
FROM user_ratings
WHERE user_id = $1 AND rating = $2
ORDER BY updated_at DESC;

-- name: ListUserRatingsByTypeAndRating :many
SELECT id, user_id, entity_type, entity_id, rating, created_at, updated_at
FROM user_ratings
WHERE user_id = $1 AND entity_type = $2 AND rating = $3
ORDER BY updated_at DESC;

-- name: ListEntityIDsByRating :many
SELECT entity_id
FROM user_ratings
WHERE user_id = $1 AND rating = $2
ORDER BY updated_at DESC
LIMIT sqlc.arg('limit')::bigint;

-- name: ListEntityIDsByTypeAndRating :many
SELECT entity_id
FROM user_ratings
WHERE user_id = $1 AND entity_type = $2 AND rating = $3
ORDER BY updated_at DESC
LIMIT sqlc.arg('limit')::bigint;

-- name: ListEntityIDsByPositiveRating :many
SELECT entity_id
FROM user_ratings
WHERE user_id = $1 AND rating IN ('favorite', 'up')
ORDER BY updated_at DESC
LIMIT sqlc.arg('limit')::bigint;

-- name: ListEntityIDsByTypeAndPositiveRating :many
SELECT entity_id
FROM user_ratings
WHERE user_id = $1 AND entity_type = $2 AND rating IN ('favorite', 'up')
ORDER BY updated_at DESC
LIMIT sqlc.arg('limit')::bigint;

-- name: UpsertUserRating :one
INSERT INTO user_ratings (user_id, entity_type, entity_id, rating, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT(user_id, entity_type, entity_id) DO UPDATE SET
    rating = EXCLUDED.rating,
    updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: DeleteUserRating :exec
DELETE FROM user_ratings
WHERE user_id = $1 AND entity_type = $2 AND entity_id = $3;

-- name: DeleteAllUserRatings :exec
DELETE FROM user_ratings
WHERE user_id = $1;

-- name: HasUserRatings :one
SELECT CASE WHEN EXISTS(SELECT 1 FROM user_ratings WHERE user_id = $1) THEN 1::bigint ELSE 0::bigint END AS has_ratings;

-- name: CountUserRatingsByRating :one
SELECT COUNT(*) AS count
FROM user_ratings
WHERE user_id = $1 AND rating = $2;
