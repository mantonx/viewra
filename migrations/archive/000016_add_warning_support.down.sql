-- Remove warning support from scan system

-- Step 1: Remove warning_count from scan_jobs
-- SQLite doesn't support DROP COLUMN before version 3.35.0
-- We need to recreate the table
CREATE TABLE scan_jobs_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    library_id INTEGER NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('pending', 'running', 'completed', 'failed', 'cancelled')),
    progress REAL DEFAULT 0,
    files_found INTEGER,
    files_processed INTEGER,
    bytes_processed INTEGER,
    error_count INTEGER DEFAULT 0,
    started_at DATETIME,
    completed_at DATETIME,
    error_message TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    phase TEXT CHECK(phase IN ('discovering', 'processing', 'completed')),
    estimated_total INTEGER,
    discovery_done BOOLEAN DEFAULT 0,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE CASCADE
);

INSERT INTO scan_jobs_new
SELECT id, library_id, status, progress, files_found, files_processed, bytes_processed,
       error_count, started_at, completed_at, error_message, created_at, updated_at,
       phase, estimated_total, discovery_done
FROM scan_jobs;

DROP TABLE scan_jobs;
ALTER TABLE scan_jobs_new RENAME TO scan_jobs;

CREATE INDEX idx_scan_jobs_library ON scan_jobs(library_id);
CREATE INDEX idx_scan_jobs_status ON scan_jobs(status);

-- Step 2: Recreate scan_checkpoints without 'warning' status
-- Any warnings will be converted to 'completed' status
CREATE TABLE scan_checkpoints_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    scan_job_id INTEGER NOT NULL,
    file_path TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('pending', 'processing', 'completed', 'failed')),
    file_size INTEGER,
    file_hash TEXT,
    error_message TEXT,
    error_category TEXT CHECK(error_category IN ('parsing', 'ffmpeg', 'database', 'filesystem', 'metadata')),
    processed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    retry_count INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (scan_job_id) REFERENCES scan_jobs(id) ON DELETE CASCADE
);

-- Convert warning status to completed when rolling back
INSERT INTO scan_checkpoints_new
SELECT id, scan_job_id, file_path,
       CASE WHEN status = 'warning' THEN 'completed' ELSE status END as status,
       file_size, file_hash, error_message, error_category, processed_at, created_at, retry_count
FROM scan_checkpoints;

DROP TABLE scan_checkpoints;
ALTER TABLE scan_checkpoints_new RENAME TO scan_checkpoints;

CREATE UNIQUE INDEX idx_scan_checkpoints_job_path ON scan_checkpoints(scan_job_id, file_path);
CREATE INDEX idx_scan_checkpoints_status ON scan_checkpoints(scan_job_id, status);
CREATE INDEX idx_scan_checkpoints_failed ON scan_checkpoints(scan_job_id, status) WHERE status = 'failed';
