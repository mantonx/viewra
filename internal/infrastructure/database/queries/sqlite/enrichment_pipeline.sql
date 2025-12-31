-- name: CreatePipelineStage :one
INSERT INTO enrichment_pipelines (
    media_type,
    plugin_id,
    stage_name,
    position,
    enabled,
    config_json,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
RETURNING *;

-- name: GetPipelineStage :one
SELECT * FROM enrichment_pipelines WHERE id = ?;

-- name: GetPipelineStageByPlugin :one
SELECT * FROM enrichment_pipelines
WHERE media_type = ? AND plugin_id = ?;

-- name: GetPipelineStageByName :one
SELECT * FROM enrichment_pipelines
WHERE media_type = ? AND stage_name = ?;

-- name: GetEnabledPipelineStages :many
SELECT * FROM enrichment_pipelines
WHERE media_type = ? AND enabled = 1
ORDER BY position ASC;

-- name: GetAllPipelineStages :many
SELECT * FROM enrichment_pipelines
WHERE media_type = ?
ORDER BY position ASC;

-- name: UpdatePipelineStage :exec
UPDATE enrichment_pipelines
SET
    stage_name = ?,
    position = ?,
    enabled = ?,
    config_json = ?,
    updated_at = datetime('now')
WHERE id = ?;

-- name: UpdatePipelineStagePosition :exec
UPDATE enrichment_pipelines
SET
    position = ?,
    updated_at = datetime('now')
WHERE id = ?;

-- name: EnablePipelineStage :exec
UPDATE enrichment_pipelines
SET
    enabled = 1,
    updated_at = datetime('now')
WHERE id = ?;

-- name: DisablePipelineStage :exec
UPDATE enrichment_pipelines
SET
    enabled = 0,
    updated_at = datetime('now')
WHERE id = ?;

-- name: DeletePipelineStage :exec
DELETE FROM enrichment_pipelines WHERE id = ?;

-- name: DeletePipelineStagesByMediaType :exec
DELETE FROM enrichment_pipelines WHERE media_type = ?;

-- name: GetNextPipelinePosition :one
SELECT COALESCE(MAX(position), 0) + 1 as next_position
FROM enrichment_pipelines
WHERE media_type = ?;

-- name: GetFirstPipelineStage :one
SELECT * FROM enrichment_pipelines
WHERE media_type = ? AND enabled = 1
ORDER BY position ASC
LIMIT 1;

-- name: GetNextPipelineStage :one
SELECT * FROM enrichment_pipelines
WHERE media_type = ? AND enabled = 1 AND position > ?
ORDER BY position ASC
LIMIT 1;

-- name: ShiftPipelinePositions :exec
-- Shift all stages at or above target position up by 1 to make room for a new stage.
-- Used when inserting builtin enrichers at specific positions.
UPDATE enrichment_pipelines
SET position = position + 1, updated_at = datetime('now')
WHERE media_type = ? AND position >= ?;
