-- Remove marketplace-related columns from plugins table
DROP INDEX IF EXISTS idx_plugins_source;
ALTER TABLE plugins DROP COLUMN IF EXISTS source;
ALTER TABLE plugins DROP COLUMN IF EXISTS source_url;
ALTER TABLE plugins DROP COLUMN IF EXISTS checksum;
