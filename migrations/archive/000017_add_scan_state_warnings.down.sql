-- Migration rollback: Remove warning and error tracking from scan_state table (SQLite)

-- Drop the indexes
DROP INDEX IF EXISTS idx_scan_state_warnings;
DROP INDEX IF EXISTS idx_scan_state_errors;

-- Note: SQLite doesn't support DROP COLUMN in older versions
-- For production, you may need to recreate the table without these columns
-- For development, these columns can remain (they won't cause issues)

-- If you need to fully rollback, uncomment and run this:
-- CREATE TABLE scan_state_backup AS SELECT
--     id, library_id, file_path, file_size, file_mtime, file_hash,
--     media_id, last_scanned_at, scan_job_id, created_at
-- FROM scan_state;
-- DROP TABLE scan_state;
-- ALTER TABLE scan_state_backup RENAME TO scan_state;
-- CREATE UNIQUE INDEX idx_scan_state_library_path ON scan_state(library_id, file_path);
-- CREATE INDEX idx_scan_state_library_mtime ON scan_state(library_id, file_mtime);
-- CREATE INDEX idx_scan_state_media_id ON scan_state(media_id);
-- CREATE INDEX idx_scan_state_library_id ON scan_state(library_id);
