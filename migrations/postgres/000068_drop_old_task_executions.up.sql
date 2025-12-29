-- Drop old task_executions table (data was migrated to scheduler_executions in migration 67)
DROP INDEX IF EXISTS idx_task_executions_task_time;
DROP INDEX IF EXISTS idx_task_executions_started_at;
DROP INDEX IF EXISTS idx_task_executions_task_id;
DROP TABLE IF EXISTS task_executions;
