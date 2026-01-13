-- Add target_paths column to scan_jobs for targeted scanning
-- When set, the scanner will only process files within these paths (relative to library root)
-- Stored as JSON array of strings, e.g., ["Movies/Action", "Movies/Drama/The Godfather.mkv"]

ALTER TABLE scan_jobs ADD COLUMN target_paths TEXT;
