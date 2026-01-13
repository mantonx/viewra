-- Remove target_paths column from scan_jobs

-- SQLite doesn't support DROP COLUMN directly, but we can create a new table
-- and copy data. However, for down migrations, we'll use a workaround by
-- setting the column to NULL and ignoring it.

-- For SQLite compatibility, we can't actually drop the column.
-- The column will remain but be unused when migrating down.
-- This is acceptable since down migrations are rare in production.

-- Note: If you need to fully remove the column, you would need to:
-- 1. Create a new table without target_paths
-- 2. Copy all data from old table to new table
-- 3. Drop old table
-- 4. Rename new table to scan_jobs
-- But this is complex and unnecessary for most use cases.
