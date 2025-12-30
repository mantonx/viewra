-- Rollback: Remove scan_state table
DROP INDEX IF EXISTS idx_scan_state_library_id;
DROP INDEX IF EXISTS idx_scan_state_media_id;
DROP INDEX IF EXISTS idx_scan_state_library_mtime;
DROP INDEX IF EXISTS idx_scan_state_library_path;
DROP TABLE IF EXISTS scan_state;
