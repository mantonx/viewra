-- name: InsertKeyword :exec
INSERT INTO media_keywords (media_type, entity_id, keyword_id, keyword, is_location)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (media_type, entity_id, keyword_id) DO UPDATE SET
    keyword = excluded.keyword,
    is_location = excluded.is_location;

-- name: GetKeywordsByEntity :many
SELECT keyword_id, keyword, is_location
FROM media_keywords
WHERE media_type = $1 AND entity_id = $2
ORDER BY keyword;

-- name: GetLocationKeywordsByEntity :many
SELECT keyword_id, keyword
FROM media_keywords
WHERE media_type = $1 AND entity_id = $2 AND is_location = TRUE
ORDER BY keyword;

-- name: GetThemeKeywordsByEntity :many
SELECT keyword_id, keyword
FROM media_keywords
WHERE media_type = $1 AND entity_id = $2 AND is_location = FALSE
ORDER BY keyword;

-- name: DeleteKeywordsByEntity :exec
DELETE FROM media_keywords
WHERE media_type = $1 AND entity_id = $2;

-- name: SearchByKeyword :many
SELECT DISTINCT media_type, entity_id
FROM media_keywords
WHERE keyword LIKE $1
ORDER BY entity_id;

-- name: SearchByLocationKeyword :many
SELECT DISTINCT media_type, entity_id
FROM media_keywords
WHERE keyword LIKE $1 AND is_location = TRUE
ORDER BY entity_id;
