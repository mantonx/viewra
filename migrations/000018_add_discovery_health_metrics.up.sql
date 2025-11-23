-- Add discovery health metrics to scan_jobs table
-- These fields track errors and issues during file discovery phase

ALTER TABLE scan_jobs ADD COLUMN discovery_errors INTEGER DEFAULT 0;
ALTER TABLE scan_jobs ADD COLUMN discovery_warnings INTEGER DEFAULT 0;
ALTER TABLE scan_jobs ADD COLUMN dirs_scanned INTEGER DEFAULT 0;
ALTER TABLE scan_jobs ADD COLUMN dirs_skipped INTEGER DEFAULT 0;
ALTER TABLE scan_jobs ADD COLUMN files_skipped INTEGER DEFAULT 0;
