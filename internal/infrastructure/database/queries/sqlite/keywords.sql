-- name: InsertKeyword :exec
INSERT INTO media_keywords (media_type, entity_id, keyword_id, keyword, is_location)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (media_type, entity_id, keyword_id) DO UPDATE SET
    keyword = excluded.keyword,
    is_location = excluded.is_location;

-- name: GetKeywordsByEntity :many
SELECT keyword_id, keyword, is_location
FROM media_keywords
WHERE media_type = ? AND entity_id = ?
ORDER BY keyword;

-- name: GetLocationKeywordsByEntity :many
SELECT keyword_id, keyword
FROM media_keywords
WHERE media_type = ? AND entity_id = ? AND is_location = TRUE
ORDER BY keyword;

-- name: GetThemeKeywordsByEntity :many
SELECT keyword_id, keyword
FROM media_keywords
WHERE media_type = ? AND entity_id = ? AND is_location = FALSE
ORDER BY keyword;

-- name: DeleteKeywordsByEntity :exec
DELETE FROM media_keywords
WHERE media_type = ? AND entity_id = ?;

-- name: SearchByKeyword :many
SELECT DISTINCT media_type, entity_id
FROM media_keywords
WHERE keyword LIKE ?
ORDER BY entity_id;

-- name: SearchByLocationKeyword :many
SELECT DISTINCT media_type, entity_id
FROM media_keywords
WHERE keyword LIKE ? AND is_location = TRUE
ORDER BY entity_id;
