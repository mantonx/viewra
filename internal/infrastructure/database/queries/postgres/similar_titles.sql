-- name: InsertSimilarTitle :exec
INSERT INTO media_similar_titles (media_id, media_type, similar_title, position)
VALUES ($1, $2, $3, $4)
ON CONFLICT (media_id, media_type, similar_title) DO UPDATE SET
    position = excluded.position;

-- name: GetSimilarTitlesByEntity :many
SELECT similar_title
FROM media_similar_titles
WHERE media_id = $1 AND media_type = $2
ORDER BY position ASC;

-- name: DeleteSimilarTitlesByEntity :exec
DELETE FROM media_similar_titles
WHERE media_id = $1 AND media_type = $2;
