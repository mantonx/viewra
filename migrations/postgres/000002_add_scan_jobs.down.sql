-- Rollback scan_jobs table (PostgreSQL version)
DROP INDEX IF EXISTS idx_scan_jobs_started_at;
DROP INDEX IF EXISTS idx_scan_jobs_created_at;
DROP INDEX IF EXISTS idx_scan_jobs_status;
DROP INDEX IF EXISTS idx_scan_jobs_library_id;
DROP TABLE IF EXISTS scan_jobs;
