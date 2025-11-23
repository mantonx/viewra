-- Remove discovery health metrics from scan_jobs table (PostgreSQL)

ALTER TABLE scan_jobs DROP COLUMN IF EXISTS files_skipped;
ALTER TABLE scan_jobs DROP COLUMN IF EXISTS dirs_skipped;
ALTER TABLE scan_jobs DROP COLUMN IF EXISTS dirs_scanned;
ALTER TABLE scan_jobs DROP COLUMN IF EXISTS discovery_warnings;
ALTER TABLE scan_jobs DROP COLUMN IF EXISTS discovery_errors;
