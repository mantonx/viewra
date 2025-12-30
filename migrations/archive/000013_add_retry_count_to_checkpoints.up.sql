-- Add retry_count column to scan_checkpoints for transient error retry tracking
ALTER TABLE scan_checkpoints ADD COLUMN retry_count INTEGER NOT NULL DEFAULT 0;
