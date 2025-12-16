-- Remove library_id from enrichment_queue
DROP INDEX IF EXISTS idx_enrichment_queue_library;

-- SQLite doesn't support DROP COLUMN directly, but since this is just
-- removing a nullable column, we can create a new table without it
-- For simplicity in rollback, we just drop the index (column will remain but be unused)
-- In production, a full table rebuild would be needed to truly remove the column
