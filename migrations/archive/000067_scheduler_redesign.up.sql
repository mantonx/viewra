-- Scheduler Redesign: Task definitions and execution history
-- Replaces the old task_executions table with a more comprehensive schema

-- Task definitions (persisted)
CREATE TABLE IF NOT EXISTS scheduled_tasks (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    schedule TEXT,  -- Cron expression (NULL/empty = manual or dependency-only)
    enabled INTEGER NOT NULL DEFAULT 1,
    source TEXT NOT NULL CHECK (source IN ('internal', 'plugin')),
    source_id TEXT,
    depends_on TEXT,  -- JSON array of task IDs
    timeout_seconds INTEGER DEFAULT 300,
    retry_count INTEGER DEFAULT 0,
    retry_delay_seconds INTEGER DEFAULT 60,
    concurrency_key TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Execution history (new table, more comprehensive than old task_executions)
CREATE TABLE IF NOT EXISTS scheduler_executions (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES scheduled_tasks(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled', 'skipped', 'interrupted')),
    scheduled_at DATETIME,
    started_at DATETIME,
    ended_at DATETIME,
    duration_ms INTEGER,
    success INTEGER,
    error TEXT,
    logs TEXT,  -- Max 64KB, truncated by application
    attempt INTEGER DEFAULT 1,
    parent_execution_id TEXT REFERENCES scheduler_executions(id),
    triggered_by TEXT NOT NULL CHECK (triggered_by IN ('schedule', 'manual', 'retry', 'dependency')),
    dependency_exec_id TEXT,
    resumable INTEGER DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_scheduler_executions_task_id ON scheduler_executions(task_id);
CREATE INDEX IF NOT EXISTS idx_scheduler_executions_status ON scheduler_executions(status);
CREATE INDEX IF NOT EXISTS idx_scheduler_executions_started ON scheduler_executions(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_scheduler_executions_running ON scheduler_executions(status) WHERE status IN ('pending', 'running');

-- Concurrency locks for preventing duplicate task runs
CREATE TABLE IF NOT EXISTS scheduler_locks (
    lock_key TEXT PRIMARY KEY,
    execution_id TEXT NOT NULL,
    acquired_at DATETIME NOT NULL,
    expires_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_scheduler_locks_expires ON scheduler_locks(expires_at);

-- Migrate existing task definitions
-- Map old task IDs to new format and seed the scheduled_tasks table
INSERT OR IGNORE INTO scheduled_tasks (id, name, description, schedule, enabled, source, source_id, timeout_seconds, created_at, updated_at)
VALUES 
    ('internal:cleanup:scan-jobs', 'Scan Job Cleanup', 'Delete old scan jobs and checkpoints based on retention policy', '*/30 * * * *', 1, 'internal', 'cleanup', 300, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('internal:cleanup:image-cache', 'Image Cache Cleanup', 'Remove orphaned image cache files that are no longer referenced in the database', '0 3 * * *', 1, 'internal', 'cleanup', 300, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('internal:transcode:policy-cleanup', 'Transcode Policy Cleanup', 'Clean failed/old/idle/orphaned transcodes based on policy rules', '0 */6 * * *', 1, 'internal', 'transcode', 300, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('internal:transcode:disk-monitor', 'Transcode Disk Monitor', 'Monitor disk usage and perform LRU cleanup if threshold exceeded', '*/30 * * * *', 1, 'internal', 'transcode', 300, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('internal:auth:session-cleanup', 'Session Cleanup', 'Remove expired user sessions from the database', '0 * * * *', 1, 'internal', 'auth', 300, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

-- Migrate old execution history if table exists (only for known task IDs)
INSERT OR IGNORE INTO scheduler_executions (id, task_id, status, started_at, ended_at, duration_ms, success, error, triggered_by, created_at)
SELECT 
    lower(hex(randomblob(16))),
    CASE task_id
        WHEN 'scan-job-cleanup' THEN 'internal:cleanup:scan-jobs'
        WHEN 'image-cache-cleanup' THEN 'internal:cleanup:image-cache'
        WHEN 'transcode-cleanup-policy' THEN 'internal:transcode:policy-cleanup'
        WHEN 'transcode-cleanup-disk-check' THEN 'internal:transcode:disk-monitor'
        WHEN 'session-cleanup' THEN 'internal:auth:session-cleanup'
    END,
    CASE WHEN success THEN 'completed' ELSE 'failed' END,
    started_at,
    ended_at,
    duration_ms,
    success,
    error,
    'schedule',
    created_at
FROM task_executions
WHERE EXISTS (SELECT 1 FROM sqlite_master WHERE type='table' AND name='task_executions')
  AND task_id IN ('scan-job-cleanup', 'image-cache-cleanup', 'transcode-cleanup-policy', 'transcode-cleanup-disk-check', 'session-cleanup');
