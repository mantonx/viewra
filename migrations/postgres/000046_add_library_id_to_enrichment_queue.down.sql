-- Remove library_id from enrichment_queue
DROP INDEX IF EXISTS idx_enrichment_queue_library;
ALTER TABLE enrichment_queue DROP COLUMN IF EXISTS library_id;
