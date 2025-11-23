-- Migration rollback: Remove warning and error tracking from scan_state table (PostgreSQL)

-- Drop the indexes
DROP INDEX IF EXISTS idx_scan_state_warnings;
DROP INDEX IF EXISTS idx_scan_state_errors;

-- Drop the warning columns
ALTER TABLE scan_state DROP COLUMN IF EXISTS has_warning;
ALTER TABLE scan_state DROP COLUMN IF EXISTS warning_message;
ALTER TABLE scan_state DROP COLUMN IF EXISTS warning_category;

-- Drop the error columns
ALTER TABLE scan_state DROP COLUMN IF EXISTS has_error;
ALTER TABLE scan_state DROP COLUMN IF EXISTS error_message;
ALTER TABLE scan_state DROP COLUMN IF EXISTS error_category;
