-- Drop index first
DROP INDEX IF EXISTS idx_transcode_jobs_type;

-- PostgreSQL supports DROP COLUMN
ALTER TABLE transcode_jobs DROP COLUMN type;
