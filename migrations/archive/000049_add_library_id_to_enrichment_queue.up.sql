-- Add library_id to enrichment_queue for SSE event filtering
-- This allows clients to subscribe to enrichment progress for a specific library
ALTER TABLE enrichment_queue ADD COLUMN library_id INTEGER REFERENCES libraries(id) ON DELETE CASCADE;

-- Backfill library_id from the media table for existing records
-- This updates existing queue entries to have the correct library association
UPDATE enrichment_queue
SET library_id = (
    SELECT m.library_id FROM media m WHERE m.id = enrichment_queue.media_id
)
WHERE library_id IS NULL;

-- Create index for filtering enrichment jobs by library
CREATE INDEX idx_enrichment_queue_library ON enrichment_queue(library_id);
