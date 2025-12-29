-- Rollback: Scheduler Redesign
-- Restores the old task_executions table from the new scheduler_executions data

-- Recreate the old task_executions table structure
CREATE TABLE IF NOT EXISTS task_executions_restore (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT NOT NULL,
    started_at DATETIME NOT NULL,
    ended_at DATETIME NOT NULL,
    duration_ms INTEGER NOT NULL,
    success BOOLEAN NOT NULL DEFAULT 0,
    error TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Migrate data back from scheduler_executions to old format
INSERT INTO task_executions_restore (task_id, started_at, ended_at, duration_ms, success, error, created_at)
SELECT 
    CASE task_id
        WHEN 'internal:cleanup:scan-jobs' THEN 'scan-job-cleanup'
        WHEN 'internal:cleanup:image-cache' THEN 'image-cache-cleanup'
        WHEN 'internal:transcode:policy-cleanup' THEN 'transcode-cleanup-policy'
        WHEN 'internal:transcode:disk-monitor' THEN 'transcode-cleanup-disk-check'
        WHEN 'internal:auth:session-cleanup' THEN 'session-cleanup'
        ELSE task_id
    END,
    COALESCE(started_at, created_at),
    COALESCE(ended_at, created_at),
    COALESCE(duration_ms, 0),
    CASE WHEN status = 'completed' THEN 1 ELSE 0 END,
    error,
    created_at
FROM scheduler_executions
WHERE started_at IS NOT NULL AND ended_at IS NOT NULL;

-- Drop new tables and indexes
DROP INDEX IF EXISTS idx_scheduler_locks_expires;
DROP TABLE IF EXISTS scheduler_locks;

DROP INDEX IF EXISTS idx_scheduler_executions_running;
DROP INDEX IF EXISTS idx_scheduler_executions_started;
DROP INDEX IF EXISTS idx_scheduler_executions_status;
DROP INDEX IF EXISTS idx_scheduler_executions_task_id;
DROP TABLE IF EXISTS scheduler_executions;

DROP TABLE IF EXISTS scheduled_tasks;

-- Rename restore table to original name if task_executions doesn't exist
-- (it was not dropped by the up migration, just left as-is)
DROP TABLE IF EXISTS task_executions;
ALTER TABLE task_executions_restore RENAME TO task_executions;

-- Recreate original indexes
CREATE INDEX IF NOT EXISTS idx_task_executions_task_id ON task_executions(task_id);
CREATE INDEX IF NOT EXISTS idx_task_executions_started_at ON task_executions(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_task_executions_task_time ON task_executions(task_id, started_at DESC);
