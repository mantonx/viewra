-- Migration: Add scan checkpoints for resumable scans
-- This enables the scanner to resume from where it left off after interruption

-- Checkpoint state for resumable scans
CREATE TABLE scan_checkpoints (
    id SERIAL PRIMARY KEY,
    scan_job_id INTEGER NOT NULL,
    file_path TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('pending', 'processing', 'completed', 'failed')),
    file_size BIGINT,
    file_hash TEXT,
    error_message TEXT,
    error_category TEXT CHECK(error_category IN ('parsing', 'ffmpeg', 'database', 'filesystem', 'metadata')),
    processed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (scan_job_id) REFERENCES scan_jobs(id) ON DELETE CASCADE
);

-- Unique constraint ensures no duplicate checkpoints for same file in same scan
CREATE UNIQUE INDEX idx_scan_checkpoints_job_path ON scan_checkpoints(scan_job_id, file_path);

-- Index for efficient status queries (get pending files, count failed, etc.)
CREATE INDEX idx_scan_checkpoints_status ON scan_checkpoints(scan_job_id, status);

-- Index specifically for failed checkpoints (for error reporting)
CREATE INDEX idx_scan_checkpoints_failed ON scan_checkpoints(scan_job_id, status) WHERE status = 'failed';

-- Extend scan_jobs table with checkpoint metadata
ALTER TABLE scan_jobs ADD COLUMN last_checkpoint_at TIMESTAMP;
ALTER TABLE scan_jobs ADD COLUMN resume_count INTEGER DEFAULT 0;

-- Add comment for documentation
COMMENT ON TABLE scan_checkpoints IS 'Tracks individual file processing state for resumable scans';
COMMENT ON COLUMN scan_checkpoints.status IS 'pending: not yet processed, processing: currently being processed, completed: successfully processed, failed: processing failed';
COMMENT ON COLUMN scan_checkpoints.error_category IS 'Category of error for failed files: parsing, ffmpeg, database, filesystem, metadata';
COMMENT ON COLUMN scan_jobs.last_checkpoint_at IS 'Timestamp of last checkpoint update (for resume detection)';
COMMENT ON COLUMN scan_jobs.resume_count IS 'Number of times this scan has been resumed';
