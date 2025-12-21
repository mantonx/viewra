-- name: GetMoodTagsByEntity :many
SELECT tag, confidence
FROM mood_tags
WHERE entity_type = ? AND entity_id = ?
ORDER BY confidence DESC, tag;

-- name: InsertMoodTag :exec
INSERT INTO mood_tags (entity_type, entity_id, tag, confidence)
VALUES (?, ?, ?, ?)
ON CONFLICT(entity_type, entity_id, tag) DO UPDATE SET confidence = excluded.confidence;

-- name: DeleteMoodTagsByEntity :exec
DELETE FROM mood_tags WHERE entity_type = ? AND entity_id = ?;

-- name: GetMoodTagsByMediaID :many
-- Deprecated: Use GetMoodTagsByEntity instead. Kept for backward compatibility.
SELECT tag, confidence
FROM mood_tags
WHERE entity_type = 'movie' AND entity_id = ?
ORDER BY confidence DESC, tag;

-- name: DeleteMoodTagsByMediaID :exec
-- Deprecated: Use DeleteMoodTagsByEntity instead. Kept for backward compatibility.
DELETE FROM mood_tags WHERE entity_type = 'movie' AND entity_id = ?;
