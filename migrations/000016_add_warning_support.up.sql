-- Add 'warning' status to scan_checkpoints table
-- This allows us to track files that were processed with warnings (e.g., FFmpeg metadata extraction failures)
-- without marking them as completely failed

-- Step 1: Drop the existing CHECK constraint
-- SQLite doesn't support ALTER TABLE DROP CONSTRAINT, so we need to recreate the table
CREATE TABLE scan_checkpoints_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    scan_job_id INTEGER NOT NULL,
    file_path TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('pending', 'processing', 'completed', 'failed', 'warning')),
    file_size INTEGER,
    file_hash TEXT,
    error_message TEXT,
    error_category TEXT CHECK(error_category IN ('parsing', 'ffmpeg', 'database', 'filesystem', 'metadata')),
    processed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    retry_count INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (scan_job_id) REFERENCES scan_jobs(id) ON DELETE CASCADE
);

-- Step 2: Copy data from old table to new table
INSERT INTO scan_checkpoints_new SELECT * FROM scan_checkpoints;

-- Step 3: Drop old table
DROP TABLE scan_checkpoints;

-- Step 4: Rename new table
ALTER TABLE scan_checkpoints_new RENAME TO scan_checkpoints;

-- Step 5: Recreate indexes
CREATE UNIQUE INDEX idx_scan_checkpoints_job_path ON scan_checkpoints(scan_job_id, file_path);
CREATE INDEX idx_scan_checkpoints_status ON scan_checkpoints(scan_job_id, status);
CREATE INDEX idx_scan_checkpoints_failed ON scan_checkpoints(scan_job_id, status) WHERE status = 'failed';
CREATE INDEX idx_scan_checkpoints_warning ON scan_checkpoints(scan_job_id, status) WHERE status = 'warning';

-- Step 6: Add warning_count to scan_jobs table
ALTER TABLE scan_jobs ADD COLUMN warning_count INTEGER DEFAULT 0;
