-- Recreate old task_executions table for rollback
CREATE TABLE IF NOT EXISTS task_executions (
    id SERIAL PRIMARY KEY,
    task_id TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    duration_ms BIGINT,
    success BOOLEAN NOT NULL,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_task_executions_task_id ON task_executions(task_id);
CREATE INDEX IF NOT EXISTS idx_task_executions_started_at ON task_executions(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_task_executions_task_time ON task_executions(task_id, started_at DESC);

-- Migrate data back from scheduler_executions (best effort)
INSERT INTO task_executions (task_id, started_at, ended_at, duration_ms, success, error, created_at)
SELECT 
    CASE task_id
        WHEN 'internal:cleanup:scan-jobs' THEN 'scan-job-cleanup'
        WHEN 'internal:cleanup:image-cache' THEN 'image-cache-cleanup'
        WHEN 'internal:transcode:policy-cleanup' THEN 'transcode-cleanup-policy'
        WHEN 'internal:transcode:disk-monitor' THEN 'transcode-cleanup-disk-check'
        WHEN 'internal:auth:session-cleanup' THEN 'session-cleanup'
        ELSE task_id
    END,
    started_at,
    ended_at,
    duration_ms,
    success,
    error,
    created_at
FROM scheduler_executions
WHERE task_id IN ('internal:cleanup:scan-jobs', 'internal:cleanup:image-cache', 'internal:transcode:policy-cleanup', 'internal:transcode:disk-monitor', 'internal:auth:session-cleanup');
