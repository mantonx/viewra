-- Remove warning support from scan system (PostgreSQL version)

-- Step 1: Remove warning_count from scan_jobs
ALTER TABLE scan_jobs DROP COLUMN warning_count;

-- Step 2: Drop warning index
DROP INDEX IF EXISTS idx_scan_checkpoints_warning;

-- Step 3: Convert any warning status to completed
UPDATE scan_checkpoints SET status = 'completed' WHERE status = 'warning';

-- Step 4: Restore original CHECK constraint
ALTER TABLE scan_checkpoints DROP CONSTRAINT IF EXISTS scan_checkpoints_status_check;
ALTER TABLE scan_checkpoints ADD CONSTRAINT scan_checkpoints_status_check
    CHECK (status IN ('pending', 'processing', 'completed', 'failed'));
