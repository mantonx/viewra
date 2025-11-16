-- Migration: Add scheduler execution history tables
-- Created: 2025-11-16
-- Description: Task execution tracking for the unified scheduler

-- Task execution history
CREATE TABLE IF NOT EXISTS task_executions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT NOT NULL,                -- Task identifier (e.g., "transcode-cleanup")
    started_at DATETIME NOT NULL,         -- When execution started
    ended_at DATETIME NOT NULL,           -- When execution completed
    duration_ms INTEGER NOT NULL,         -- Execution duration in milliseconds
    success BOOLEAN NOT NULL DEFAULT 0,   -- Whether execution succeeded
    error TEXT,                           -- Error message if failed
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Index for querying by task_id (most common query pattern)
CREATE INDEX IF NOT EXISTS idx_task_executions_task_id ON task_executions(task_id);

-- Index for querying recent executions
CREATE INDEX IF NOT EXISTS idx_task_executions_started_at ON task_executions(started_at DESC);

-- Composite index for task history queries (task_id + time)
CREATE INDEX IF NOT EXISTS idx_task_executions_task_time ON task_executions(task_id, started_at DESC);
