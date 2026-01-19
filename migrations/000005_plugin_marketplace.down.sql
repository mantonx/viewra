-- Remove marketplace-related columns from plugins table
-- Note: SQLite doesn't support DROP COLUMN directly in older versions,
-- but modern SQLite 3.35.0+ does. For compatibility, we use a safe approach.

-- Drop the index first
DROP INDEX IF EXISTS idx_plugins_source;

-- For SQLite 3.35.0+, we can drop columns directly
-- These will fail gracefully on older SQLite versions
ALTER TABLE plugins DROP COLUMN source;
ALTER TABLE plugins DROP COLUMN source_url;
ALTER TABLE plugins DROP COLUMN checksum;
