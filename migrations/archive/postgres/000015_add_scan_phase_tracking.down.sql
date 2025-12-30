-- Revert phase tracking fields

ALTER TABLE scan_jobs DROP COLUMN IF EXISTS phase;
ALTER TABLE scan_jobs DROP COLUMN IF EXISTS estimated_total;
ALTER TABLE scan_jobs DROP COLUMN IF EXISTS discovery_done;
