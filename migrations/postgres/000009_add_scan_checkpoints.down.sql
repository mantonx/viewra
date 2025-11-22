-- Rollback: Remove scan checkpoints system

-- Drop checkpoint table (cascades will handle dependencies)
DROP TABLE IF EXISTS scan_checkpoints;

-- Remove columns from scan_jobs
ALTER TABLE scan_jobs DROP COLUMN IF EXISTS last_checkpoint_at;
ALTER TABLE scan_jobs DROP COLUMN IF EXISTS resume_count;
