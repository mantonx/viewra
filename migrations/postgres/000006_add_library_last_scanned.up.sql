-- Add last_scanned_at column if it doesn't exist (fixes DBs created before this was in init)
-- Note: This is a no-op for new databases where init already has this column
ALTER TABLE libraries ADD COLUMN IF NOT EXISTS last_scanned_at TIMESTAMPTZ;
