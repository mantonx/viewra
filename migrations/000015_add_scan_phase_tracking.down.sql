-- Revert phase tracking fields

-- Note: SQLite doesn't support DROP COLUMN directly, so we need to recreate the table
-- This is a destructive operation that will lose scan job data

CREATE TABLE scan_jobs_backup (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    library_id INTEGER NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('pending', 'running', 'paused', 'completed', 'failed')),
    progress REAL DEFAULT 0.0,
    files_found INTEGER DEFAULT 0,
    files_processed INTEGER DEFAULT 0,
    bytes_processed INTEGER DEFAULT 0,
    error_count INTEGER DEFAULT 0,
    started_at DATETIME,
    completed_at DATETIME,
    error_message TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE CASCADE
);

INSERT INTO scan_jobs_backup SELECT
    id, library_id, status, progress, files_found, files_processed,
    bytes_processed, error_count, started_at, completed_at, error_message,
    created_at, updated_at
FROM scan_jobs;

DROP TABLE scan_jobs;
ALTER TABLE scan_jobs_backup RENAME TO scan_jobs;

CREATE INDEX idx_scan_jobs_library_id ON scan_jobs(library_id);
CREATE INDEX idx_scan_jobs_status ON scan_jobs(status);
CREATE INDEX idx_scan_jobs_created_at ON scan_jobs(created_at);
CREATE INDEX idx_scan_jobs_started_at ON scan_jobs(started_at);
