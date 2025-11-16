-- name: LogTaskExecution :one
INSERT INTO task_executions (
    task_id,
    started_at,
    ended_at,
    duration_ms,
    success,
    error
) VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetTaskExecutionHistory :many
SELECT *
FROM task_executions
WHERE task_id = ?
ORDER BY started_at DESC
LIMIT ?;

-- name: GetAllRecentExecutions :many
SELECT *
FROM task_executions
ORDER BY started_at DESC
LIMIT ?;

-- name: GetTaskExecutionStats :one
SELECT
    COUNT(*) as total_executions,
    SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) as successful_executions,
    SUM(CASE WHEN success = 0 THEN 1 ELSE 0 END) as failed_executions,
    AVG(duration_ms) as avg_duration_ms,
    MAX(started_at) as last_execution
FROM task_executions
WHERE task_id = ?;

-- name: DeleteOldTaskExecutions :exec
DELETE FROM task_executions
WHERE started_at < ?;

-- name: GetLastTaskExecution :one
SELECT *
FROM task_executions
WHERE task_id = ?
ORDER BY started_at DESC
LIMIT 1;
