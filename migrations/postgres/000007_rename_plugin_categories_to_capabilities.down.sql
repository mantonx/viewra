-- Revert 'capabilities' column back to 'categories'

ALTER TABLE plugins RENAME COLUMN capabilities TO categories;

-- Recreate index with original column name
DROP INDEX IF EXISTS idx_plugins_capabilities;
CREATE INDEX idx_plugins_categories ON plugins(categories);
