-- Rollback: Remove scan checkpoints system (SQLite)

-- Drop checkpoint table
DROP TABLE IF EXISTS scan_checkpoints;

-- SQLite doesn't support DROP COLUMN in all versions
-- For backwards compatibility, we leave the columns in place
-- They will be ignored by the application code
-- A full schema rebuild would be needed to remove them completely
-- (This is acceptable as they're nullable and default-valued)
