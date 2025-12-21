-- name: GetMoodTagsByMediaID :many
SELECT tag, confidence
FROM media_mood_tags
WHERE media_id = ?
ORDER BY confidence DESC, tag;

-- name: InsertMoodTag :exec
INSERT INTO media_mood_tags (media_id, tag, confidence)
VALUES (?, ?, ?);

-- name: DeleteMoodTagsByMediaID :exec
DELETE FROM media_mood_tags WHERE media_id = ?;
