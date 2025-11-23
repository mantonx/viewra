-- Migration: Add warning and error tracking to scan_state table (PostgreSQL)
-- This enables persistent library-level issue tracking across scans

-- Add warning columns to scan_state
ALTER TABLE scan_state ADD COLUMN has_warning BOOLEAN DEFAULT FALSE;
ALTER TABLE scan_state ADD COLUMN warning_message TEXT;
ALTER TABLE scan_state ADD COLUMN warning_category TEXT;

-- Add error columns to scan_state
ALTER TABLE scan_state ADD COLUMN has_error BOOLEAN DEFAULT FALSE;
ALTER TABLE scan_state ADD COLUMN error_message TEXT;
ALTER TABLE scan_state ADD COLUMN error_category TEXT;

-- Create indexes for efficient warning/error queries (partial indexes for better performance)
CREATE INDEX idx_scan_state_warnings ON scan_state(library_id, has_warning) WHERE has_warning = TRUE;
CREATE INDEX idx_scan_state_errors ON scan_state(library_id, has_error) WHERE has_error = TRUE;
