-- Remove target_paths column from scan_jobs

ALTER TABLE scan_jobs DROP COLUMN target_paths;
