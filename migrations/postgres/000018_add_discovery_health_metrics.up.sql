-- Add discovery health metrics to scan_jobs table (PostgreSQL)
-- These fields track errors and issues during file discovery phase

ALTER TABLE scan_jobs ADD COLUMN IF NOT EXISTS discovery_errors INTEGER DEFAULT 0;
ALTER TABLE scan_jobs ADD COLUMN IF NOT EXISTS discovery_warnings INTEGER DEFAULT 0;
ALTER TABLE scan_jobs ADD COLUMN IF NOT EXISTS dirs_scanned INTEGER DEFAULT 0;
ALTER TABLE scan_jobs ADD COLUMN IF NOT EXISTS dirs_skipped INTEGER DEFAULT 0;
ALTER TABLE scan_jobs ADD COLUMN IF NOT EXISTS files_skipped INTEGER DEFAULT 0;
