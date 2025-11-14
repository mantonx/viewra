-- Remove indexes
DROP INDEX IF EXISTS idx_transcode_jobs_last_accessed;
DROP INDEX IF EXISTS idx_transcode_jobs_file_size;

-- Remove columns (SQLite doesn't support DROP COLUMN directly, so we need to recreate)
-- For now, we'll just mark this as a warning - full rollback would require table recreation
-- In production, you'd want to recreate the table without these columns

-- Note: SQLite doesn't support DROP COLUMN, so this is a simplified down migration
-- To fully rollback, you would need to:
-- 1. Create new table without these columns
-- 2. Copy data from old table
-- 3. Drop old table
-- 4. Rename new table
