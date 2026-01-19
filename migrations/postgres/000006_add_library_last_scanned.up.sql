-- Add last_scanned_at column if it doesn't exist (fixes DBs created before this was in init)
ALTER TABLE libraries ADD COLUMN last_scanned_at DATETIME;
