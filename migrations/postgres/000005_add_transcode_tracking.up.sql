-- Add access tracking and file metadata to transcode_jobs table
ALTER TABLE transcode_jobs ADD COLUMN file_path TEXT;
ALTER TABLE transcode_jobs ADD COLUMN file_size_bytes BIGINT DEFAULT 0;
ALTER TABLE transcode_jobs ADD COLUMN last_accessed_at TIMESTAMP;
ALTER TABLE transcode_jobs ADD COLUMN access_count INTEGER DEFAULT 0;

-- Create index on last_accessed_at for LRU cleanup
CREATE INDEX idx_transcode_jobs_last_accessed ON transcode_jobs(last_accessed_at);

-- Create index on file_size_bytes for size-based queries
CREATE INDEX idx_transcode_jobs_file_size ON transcode_jobs(file_size_bytes);
