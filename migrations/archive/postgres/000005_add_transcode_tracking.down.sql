-- Remove indexes
DROP INDEX IF EXISTS idx_transcode_jobs_file_size;
DROP INDEX IF EXISTS idx_transcode_jobs_last_accessed;

-- Remove columns
ALTER TABLE transcode_jobs DROP COLUMN IF EXISTS access_count;
ALTER TABLE transcode_jobs DROP COLUMN IF EXISTS last_accessed_at;
ALTER TABLE transcode_jobs DROP COLUMN IF EXISTS file_size_bytes;
ALTER TABLE transcode_jobs DROP COLUMN IF EXISTS file_path;
