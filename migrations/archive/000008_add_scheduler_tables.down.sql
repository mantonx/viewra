-- Rollback: Remove scheduler tables
DROP INDEX IF EXISTS idx_task_executions_task_time;
DROP INDEX IF EXISTS idx_task_executions_started_at;
DROP INDEX IF EXISTS idx_task_executions_task_id;
DROP TABLE IF EXISTS task_executions;
