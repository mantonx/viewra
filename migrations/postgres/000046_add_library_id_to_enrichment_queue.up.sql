-- Add library_id to enrichment_queue for SSE event filtering
-- This allows clients to subscribe to enrichment progress for a specific library
ALTER TABLE enrichment_queue ADD COLUMN library_id INTEGER REFERENCES libraries(id) ON DELETE CASCADE;

-- Backfill library_id from the media table for existing records
UPDATE enrichment_queue eq
SET library_id = m.library_id
FROM media m
WHERE eq.media_id = m.id
  AND eq.library_id IS NULL;

-- Create index for filtering enrichment jobs by library
CREATE INDEX idx_enrichment_queue_library ON enrichment_queue(library_id);
