-- Remove retry_count column from scan_checkpoints
ALTER TABLE scan_checkpoints DROP COLUMN retry_count;
