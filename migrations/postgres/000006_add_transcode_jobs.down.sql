-- Rollback transcode_jobs table (PostgreSQL version)
DROP INDEX IF EXISTS idx_transcode_jobs_created_at;
DROP INDEX IF EXISTS idx_transcode_jobs_status;
DROP INDEX IF EXISTS idx_transcode_jobs_media_id;
DROP TABLE IF EXISTS transcode_jobs;
