-- Add marketplace-related columns to plugins table
ALTER TABLE plugins ADD COLUMN source TEXT NOT NULL DEFAULT 'local';
ALTER TABLE plugins ADD COLUMN source_url TEXT;
ALTER TABLE plugins ADD COLUMN checksum TEXT;

-- Create index for filtering by source
CREATE INDEX IF NOT EXISTS idx_plugins_source ON plugins(source);
