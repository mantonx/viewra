-- Migration: Add scan_state table for incremental scanning (PostgreSQL)
-- This enables the scanner to detect which files have changed since last scan

-- Persistent file state for incremental scanning
CREATE TABLE scan_state (
    id SERIAL PRIMARY KEY,
    library_id INTEGER NOT NULL,
    file_path TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    file_mtime TIMESTAMP NOT NULL,
    file_hash TEXT, -- Optional: NULL for mtime+size-only comparison (faster for large files)
    media_id INTEGER,
    last_scanned_at TIMESTAMP NOT NULL,
    scan_job_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE CASCADE,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE SET NULL,
    FOREIGN KEY (scan_job_id) REFERENCES scan_jobs(id)
);

-- Unique constraint ensures one state record per file per library
CREATE UNIQUE INDEX idx_scan_state_library_path ON scan_state(library_id, file_path);

-- Index for efficient mtime queries (find files modified since timestamp)
CREATE INDEX idx_scan_state_library_mtime ON scan_state(library_id, file_mtime);

-- Index for media lookups (cleanup when media deleted)
CREATE INDEX idx_scan_state_media_id ON scan_state(media_id);

-- Index for library-wide queries
CREATE INDEX idx_scan_state_library_id ON scan_state(library_id);
