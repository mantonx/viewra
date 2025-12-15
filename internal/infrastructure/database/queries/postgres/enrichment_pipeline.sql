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
) VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
RETURNING *;

-- name: GetPipelineStage :one
SELECT * FROM enrichment_pipelines WHERE id = $1;

-- name: GetPipelineStageByPlugin :one
SELECT * FROM enrichment_pipelines
WHERE media_type = $1 AND plugin_id = $2;

-- name: GetPipelineStageByName :one
SELECT * FROM enrichment_pipelines
WHERE media_type = $1 AND stage_name = $2;

-- name: GetEnabledPipelineStages :many
SELECT * FROM enrichment_pipelines
WHERE media_type = $1 AND enabled = TRUE
ORDER BY position ASC;

-- name: GetAllPipelineStages :many
SELECT * FROM enrichment_pipelines
WHERE media_type = $1
ORDER BY position ASC;

-- name: UpdatePipelineStage :exec
UPDATE enrichment_pipelines
SET
    stage_name = $1,
    position = $2,
    enabled = $3,
    config_json = $4,
    updated_at = NOW()
WHERE id = $5;

-- name: UpdatePipelineStagePosition :exec
UPDATE enrichment_pipelines
SET
    position = $1,
    updated_at = NOW()
WHERE id = $2;

-- name: EnablePipelineStage :exec
UPDATE enrichment_pipelines
SET
    enabled = TRUE,
    updated_at = NOW()
WHERE id = $1;

-- name: DisablePipelineStage :exec
UPDATE enrichment_pipelines
SET
    enabled = FALSE,
    updated_at = NOW()
WHERE id = $1;

-- name: DeletePipelineStage :exec
DELETE FROM enrichment_pipelines WHERE id = $1;

-- name: DeletePipelineStagesByMediaType :exec
DELETE FROM enrichment_pipelines WHERE media_type = $1;

-- name: GetNextPipelinePosition :one
SELECT COALESCE(MAX(position), 0) + 1 as next_position
FROM enrichment_pipelines
WHERE media_type = $1;

-- name: GetFirstPipelineStage :one
SELECT * FROM enrichment_pipelines
WHERE media_type = $1 AND enabled = TRUE
ORDER BY position ASC
LIMIT 1;

-- name: GetNextPipelineStage :one
SELECT * FROM enrichment_pipelines
WHERE media_type = $1 AND enabled = TRUE AND position > $2
ORDER BY position ASC
LIMIT 1;
