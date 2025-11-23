-- Add 'warning' status to scan_checkpoints table (PostgreSQL version)

-- Step 1: Drop and recreate the CHECK constraint for status
ALTER TABLE scan_checkpoints DROP CONSTRAINT IF EXISTS scan_checkpoints_status_check;
ALTER TABLE scan_checkpoints ADD CONSTRAINT scan_checkpoints_status_check
    CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'warning'));

-- Step 2: Create index for warning status
CREATE INDEX idx_scan_checkpoints_warning ON scan_checkpoints(scan_job_id, status) WHERE status = 'warning';

-- Step 3: Add warning_count to scan_jobs table
ALTER TABLE scan_jobs ADD COLUMN warning_count INTEGER DEFAULT 0;
