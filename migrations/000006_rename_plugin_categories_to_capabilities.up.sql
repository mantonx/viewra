-- Rename 'categories' column to 'capabilities' in plugins table
-- This reflects that the column stores plugin capabilities, not UI display categories

ALTER TABLE plugins RENAME COLUMN categories TO capabilities;

-- Recreate index with new column name
DROP INDEX IF EXISTS idx_plugins_categories;
CREATE INDEX idx_plugins_capabilities ON plugins(capabilities);
