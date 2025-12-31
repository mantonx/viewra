-- Scheduled Tasks Queries

-- name: GetScheduledTask :one
SELECT id, name, description, schedule, enabled, source, source_id,
       depends_on, timeout_seconds, retry_count, retry_delay_seconds,
       concurrency_key, created_at, updated_at
FROM scheduled_tasks
WHERE id = $1;

-- name: ListScheduledTasks :many
SELECT id, name, description, schedule, enabled, source, source_id,
       depends_on, timeout_seconds, retry_count, retry_delay_seconds,
       concurrency_key, created_at, updated_at
FROM scheduled_tasks
ORDER BY source, name;

-- name: ListEnabledScheduledTasks :many
SELECT id, name, description, schedule, enabled, source, source_id,
       depends_on, timeout_seconds, retry_count, retry_delay_seconds,
       concurrency_key, created_at, updated_at
FROM scheduled_tasks
WHERE enabled = TRUE
ORDER BY source, name;

-- name: ListScheduledTasksBySource :many
SELECT id, name, description, schedule, enabled, source, source_id,
       depends_on, timeout_seconds, retry_count, retry_delay_seconds,
       concurrency_key, created_at, updated_at
FROM scheduled_tasks
WHERE source = $1
ORDER BY name;

-- name: ListScheduledTasksBySourceID :many
SELECT id, name, description, schedule, enabled, source, source_id,
       depends_on, timeout_seconds, retry_count, retry_delay_seconds,
       concurrency_key, created_at, updated_at
FROM scheduled_tasks
WHERE source_id = $1
ORDER BY name;

-- name: UpsertScheduledTask :exec
INSERT INTO scheduled_tasks (
    id, name, description, schedule, enabled, source, source_id,
    depends_on, timeout_seconds, retry_count, retry_delay_seconds,
    concurrency_key, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW())
ON CONFLICT(id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    schedule = CASE WHEN EXCLUDED.source = 'plugin' THEN EXCLUDED.schedule ELSE scheduled_tasks.schedule END,
    enabled = scheduled_tasks.enabled,
    source = EXCLUDED.source,
    source_id = EXCLUDED.source_id,
    depends_on = EXCLUDED.depends_on,
    timeout_seconds = EXCLUDED.timeout_seconds,
    retry_count = EXCLUDED.retry_count,
    retry_delay_seconds = EXCLUDED.retry_delay_seconds,
    concurrency_key = EXCLUDED.concurrency_key,
    updated_at = NOW();

-- name: UpdateScheduledTask :exec
UPDATE scheduled_tasks SET
    schedule = COALESCE(sqlc.narg('schedule'), schedule),
    enabled = COALESCE(sqlc.narg('enabled'), enabled),
    updated_at = NOW()
WHERE id = sqlc.arg('id');

-- name: EnableScheduledTask :exec
UPDATE scheduled_tasks SET enabled = TRUE, updated_at = NOW() WHERE id = $1;

-- name: DisableScheduledTask :exec
UPDATE scheduled_tasks SET enabled = FALSE, updated_at = NOW() WHERE id = $1;

-- name: DeleteScheduledTask :exec
DELETE FROM scheduled_tasks WHERE id = $1;

-- name: DeleteScheduledTasksBySourceID :exec
DELETE FROM scheduled_tasks WHERE source_id = $1;

-- name: ScheduledTaskExists :one
SELECT EXISTS(SELECT 1 FROM scheduled_tasks WHERE id = $1)::bigint as task_exists;

-- Scheduler Executions Queries

-- name: CreateSchedulerExecution :exec
INSERT INTO scheduler_executions (
    id, task_id, status, scheduled_at, started_at, ended_at, duration_ms,
    success, error, logs, attempt, parent_execution_id, triggered_by,
    dependency_exec_id, resumable, created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, NOW());

-- name: GetSchedulerExecution :one
SELECT id, task_id, status, scheduled_at, started_at, ended_at, duration_ms,
       success, error, logs, attempt, parent_execution_id, triggered_by,
       dependency_exec_id, resumable, created_at
FROM scheduler_executions
WHERE id = $1;

-- name: UpdateSchedulerExecution :exec
UPDATE scheduler_executions SET
    status = $1,
    started_at = $2,
    ended_at = $3,
    duration_ms = $4,
    success = $5,
    error = $6,
    logs = $7,
    resumable = $8
WHERE id = $9;

-- name: ListSchedulerExecutions :many
SELECT id, task_id, status, scheduled_at, started_at, ended_at, duration_ms,
       success, error, logs, attempt, parent_execution_id, triggered_by,
       dependency_exec_id, resumable, created_at
FROM scheduler_executions
WHERE (sqlc.narg('task_id')::TEXT IS NULL OR task_id = sqlc.narg('task_id'))
  AND (sqlc.narg('status')::TEXT IS NULL OR status = sqlc.narg('status'))
ORDER BY created_at DESC
LIMIT sqlc.arg('limit')::bigint OFFSET sqlc.arg('offset')::bigint;

-- name: ListSchedulerExecutionsByTask :many
SELECT id, task_id, status, scheduled_at, started_at, ended_at, duration_ms,
       success, error, logs, attempt, parent_execution_id, triggered_by,
       dependency_exec_id, resumable, created_at
FROM scheduler_executions
WHERE task_id = $1
ORDER BY created_at DESC
LIMIT sqlc.arg('limit')::bigint;

-- name: GetLatestSchedulerExecution :one
SELECT id, task_id, status, scheduled_at, started_at, ended_at, duration_ms,
       success, error, logs, attempt, parent_execution_id, triggered_by,
       dependency_exec_id, resumable, created_at
FROM scheduler_executions
WHERE task_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: GetRunningSchedulerExecutions :many
SELECT id, task_id, status, scheduled_at, started_at, ended_at, duration_ms,
       success, error, logs, attempt, parent_execution_id, triggered_by,
       dependency_exec_id, resumable, created_at
FROM scheduler_executions
WHERE status IN ('pending', 'running')
ORDER BY created_at DESC;

-- name: GetInterruptedSchedulerExecutions :many
SELECT id, task_id, status, scheduled_at, started_at, ended_at, duration_ms,
       success, error, logs, attempt, parent_execution_id, triggered_by,
       dependency_exec_id, resumable, created_at
FROM scheduler_executions
WHERE status = 'interrupted' AND resumable = TRUE
ORDER BY created_at DESC;

-- name: CountSchedulerExecutionsByTask :one
SELECT COUNT(*) as count
FROM scheduler_executions
WHERE task_id = $1;

-- name: DeleteOldSchedulerExecutions :execrows
DELETE FROM scheduler_executions
WHERE created_at < $1;

-- name: MarkRunningAsInterrupted :execrows
UPDATE scheduler_executions
SET status = 'interrupted',
    ended_at = NOW(),
    resumable = $1
WHERE status IN ('pending', 'running');

-- name: GetSchedulerExecutionStats :one
SELECT
    COUNT(*) as total_executions,
    SUM(CASE WHEN success = TRUE THEN 1 ELSE 0 END) as successful_executions,
    SUM(CASE WHEN success = FALSE THEN 1 ELSE 0 END) as failed_executions,
    AVG(duration_ms) as avg_duration_ms,
    MAX(started_at) as last_execution
FROM scheduler_executions
WHERE task_id = $1;

-- Scheduler Locks Queries

-- name: TryAcquireSchedulerLock :exec
INSERT INTO scheduler_locks (lock_key, execution_id, acquired_at, expires_at)
VALUES ($1, $2, NOW(), $3)
ON CONFLICT(lock_key) DO NOTHING;

-- name: GetSchedulerLock :one
SELECT lock_key, execution_id, acquired_at, expires_at
FROM scheduler_locks
WHERE lock_key = $1;

-- name: ReleaseSchedulerLock :exec
DELETE FROM scheduler_locks WHERE lock_key = $1;

-- name: RefreshSchedulerLock :exec
UPDATE scheduler_locks SET expires_at = $1 WHERE lock_key = $2;

-- name: SchedulerLockExists :one
SELECT EXISTS(SELECT 1 FROM scheduler_locks WHERE lock_key = $1 AND expires_at > NOW())::bigint as lock_exists;

-- name: CleanExpiredSchedulerLocks :execrows
DELETE FROM scheduler_locks WHERE expires_at < NOW();
