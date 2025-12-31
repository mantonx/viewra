-- Scheduled Tasks Queries

-- name: GetScheduledTask :one
SELECT id, name, description, schedule, enabled, source, source_id,
       depends_on, timeout_seconds, retry_count, retry_delay_seconds,
       concurrency_key, created_at, updated_at
FROM scheduled_tasks
WHERE id = ?;

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
WHERE enabled = 1
ORDER BY source, name;

-- name: ListScheduledTasksBySource :many
SELECT id, name, description, schedule, enabled, source, source_id,
       depends_on, timeout_seconds, retry_count, retry_delay_seconds,
       concurrency_key, created_at, updated_at
FROM scheduled_tasks
WHERE source = ?
ORDER BY name;

-- name: ListScheduledTasksBySourceID :many
SELECT id, name, description, schedule, enabled, source, source_id,
       depends_on, timeout_seconds, retry_count, retry_delay_seconds,
       concurrency_key, created_at, updated_at
FROM scheduled_tasks
WHERE source_id = ?
ORDER BY name;

-- name: UpsertScheduledTask :exec
INSERT INTO scheduled_tasks (
    id, name, description, schedule, enabled, source, source_id,
    depends_on, timeout_seconds, retry_count, retry_delay_seconds,
    concurrency_key, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
ON CONFLICT(id) DO UPDATE SET
    name = excluded.name,
    description = excluded.description,
    schedule = CASE WHEN excluded.source = 'plugin' THEN excluded.schedule ELSE scheduled_tasks.schedule END,
    enabled = scheduled_tasks.enabled,
    source = excluded.source,
    source_id = excluded.source_id,
    depends_on = excluded.depends_on,
    timeout_seconds = excluded.timeout_seconds,
    retry_count = excluded.retry_count,
    retry_delay_seconds = excluded.retry_delay_seconds,
    concurrency_key = excluded.concurrency_key,
    updated_at = datetime('now');

-- name: UpdateScheduledTask :exec
UPDATE scheduled_tasks SET
    schedule = COALESCE(sqlc.narg('schedule'), schedule),
    enabled = COALESCE(sqlc.narg('enabled'), enabled),
    updated_at = datetime('now')
WHERE id = sqlc.arg('id');

-- name: EnableScheduledTask :exec
UPDATE scheduled_tasks SET enabled = 1, updated_at = datetime('now') WHERE id = ?;

-- name: DisableScheduledTask :exec
UPDATE scheduled_tasks SET enabled = 0, updated_at = datetime('now') WHERE id = ?;

-- name: DeleteScheduledTask :exec
DELETE FROM scheduled_tasks WHERE id = ?;

-- name: DeleteScheduledTasksBySourceID :exec
DELETE FROM scheduled_tasks WHERE source_id = ?;

-- name: ScheduledTaskExists :one
SELECT EXISTS(SELECT 1 FROM scheduled_tasks WHERE id = ?) as task_exists;

-- Scheduler Executions Queries

-- name: CreateSchedulerExecution :exec
INSERT INTO scheduler_executions (
    id, task_id, status, scheduled_at, started_at, ended_at, duration_ms,
    success, error, logs, attempt, parent_execution_id, triggered_by,
    dependency_exec_id, resumable, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'));

-- name: GetSchedulerExecution :one
SELECT id, task_id, status, scheduled_at, started_at, ended_at, duration_ms,
       success, error, logs, attempt, parent_execution_id, triggered_by,
       dependency_exec_id, resumable, created_at
FROM scheduler_executions
WHERE id = ?;

-- name: UpdateSchedulerExecution :exec
UPDATE scheduler_executions SET
    status = ?,
    started_at = ?,
    ended_at = ?,
    duration_ms = ?,
    success = ?,
    error = ?,
    logs = ?,
    resumable = ?
WHERE id = ?;

-- name: ListSchedulerExecutions :many
SELECT id, task_id, status, scheduled_at, started_at, ended_at, duration_ms,
       success, error, logs, attempt, parent_execution_id, triggered_by,
       dependency_exec_id, resumable, created_at
FROM scheduler_executions
WHERE (CAST(sqlc.narg('task_id') AS TEXT) IS NULL OR task_id = sqlc.narg('task_id'))
  AND (CAST(sqlc.narg('status') AS TEXT) IS NULL OR status = sqlc.narg('status'))
ORDER BY created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: ListSchedulerExecutionsByTask :many
SELECT id, task_id, status, scheduled_at, started_at, ended_at, duration_ms,
       success, error, logs, attempt, parent_execution_id, triggered_by,
       dependency_exec_id, resumable, created_at
FROM scheduler_executions
WHERE task_id = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: GetLatestSchedulerExecution :one
SELECT id, task_id, status, scheduled_at, started_at, ended_at, duration_ms,
       success, error, logs, attempt, parent_execution_id, triggered_by,
       dependency_exec_id, resumable, created_at
FROM scheduler_executions
WHERE task_id = ?
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
WHERE status = 'interrupted' AND resumable = 1
ORDER BY created_at DESC;

-- name: CountSchedulerExecutionsByTask :one
SELECT COUNT(*) as count
FROM scheduler_executions
WHERE task_id = ?;

-- name: DeleteOldSchedulerExecutions :execrows
DELETE FROM scheduler_executions
WHERE created_at < ?;

-- name: MarkRunningAsInterrupted :execrows
UPDATE scheduler_executions
SET status = 'interrupted',
    ended_at = datetime('now'),
    resumable = ?
WHERE status IN ('pending', 'running');

-- name: GetSchedulerExecutionStats :one
SELECT
    COUNT(*) as total_executions,
    CAST(SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) AS INTEGER) as successful_executions,
    CAST(SUM(CASE WHEN success = 0 THEN 1 ELSE 0 END) AS INTEGER) as failed_executions,
    CAST(AVG(duration_ms) AS REAL) as avg_duration_ms,
    MAX(started_at) as last_execution
FROM scheduler_executions
WHERE task_id = ?;

-- Scheduler Locks Queries

-- name: TryAcquireSchedulerLock :exec
INSERT INTO scheduler_locks (lock_key, execution_id, acquired_at, expires_at)
VALUES (?, ?, datetime('now'), ?)
ON CONFLICT(lock_key) DO NOTHING;

-- name: GetSchedulerLock :one
SELECT lock_key, execution_id, acquired_at, expires_at
FROM scheduler_locks
WHERE lock_key = ?;

-- name: ReleaseSchedulerLock :exec
DELETE FROM scheduler_locks WHERE lock_key = ?;

-- name: RefreshSchedulerLock :exec
UPDATE scheduler_locks SET expires_at = ? WHERE lock_key = ?;

-- name: SchedulerLockExists :one
SELECT EXISTS(SELECT 1 FROM scheduler_locks WHERE lock_key = ? AND expires_at > datetime('now')) as lock_exists;

-- name: CleanExpiredSchedulerLocks :execrows
DELETE FROM scheduler_locks WHERE expires_at < datetime('now');
