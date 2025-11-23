-- Remove discovery health metrics from scan_jobs table

ALTER TABLE scan_jobs DROP COLUMN files_skipped;
ALTER TABLE scan_jobs DROP COLUMN dirs_skipped;
ALTER TABLE scan_jobs DROP COLUMN dirs_scanned;
ALTER TABLE scan_jobs DROP COLUMN discovery_warnings;
ALTER TABLE scan_jobs DROP COLUMN discovery_errors;
