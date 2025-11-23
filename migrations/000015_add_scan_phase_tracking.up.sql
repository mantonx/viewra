-- Add phase tracking fields to scan_jobs for better progress reporting
-- Phase: discovering -> processing -> completed

ALTER TABLE scan_jobs ADD COLUMN phase TEXT DEFAULT 'processing' CHECK(phase IN ('discovering', 'processing', 'completed'));
ALTER TABLE scan_jobs ADD COLUMN estimated_total INTEGER DEFAULT 0;
ALTER TABLE scan_jobs ADD COLUMN discovery_done BOOLEAN DEFAULT 1;
