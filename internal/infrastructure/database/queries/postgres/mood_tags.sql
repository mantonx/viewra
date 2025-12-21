-- name: GetMoodTagsByMediaID :many
SELECT tag, confidence
FROM media_mood_tags
WHERE media_id = $1
ORDER BY confidence DESC, tag;

-- name: InsertMoodTag :exec
INSERT INTO media_mood_tags (media_id, tag, confidence)
VALUES ($1, $2, $3);

-- name: DeleteMoodTagsByMediaID :exec
DELETE FROM media_mood_tags WHERE media_id = $1;
