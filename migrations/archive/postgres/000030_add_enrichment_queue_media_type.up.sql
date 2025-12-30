-- Add media_type column to enrichment_queue to explicitly track entity type
-- This fixes the ID collision issue where different entity types (movie, tv_show, etc.)
-- could have overlapping IDs since they're stored in separate tables with independent sequences

-- Add media_type column
ALTER TABLE enrichment_queue ADD COLUMN media_type TEXT;

-- Update existing rows to have media_type based on lookup
-- All existing jobs reference the media table, so we can determine type from there
UPDATE enrichment_queue eq
SET media_type = m.type
FROM media m
WHERE m.id = eq.media_id;

-- For any remaining NULL rows (shouldn't happen, but safety), default to 'movie'
UPDATE enrichment_queue SET media_type = 'movie' WHERE media_type IS NULL;

-- Make column NOT NULL and add constraint
ALTER TABLE enrichment_queue ALTER COLUMN media_type SET NOT NULL;
ALTER TABLE enrichment_queue ADD CONSTRAINT enrichment_queue_media_type_check
    CHECK(media_type IN ('movie', 'tv', 'tv_show', 'music'));

-- Drop old unique constraint and create new one with media_type
ALTER TABLE enrichment_queue DROP CONSTRAINT enrichment_queue_media_id_stage_key;
ALTER TABLE enrichment_queue ADD CONSTRAINT enrichment_queue_media_id_media_type_stage_key
    UNIQUE(media_id, media_type, stage);

-- Remove FK constraint since we now support parent entities that aren't in the media table
ALTER TABLE enrichment_queue DROP CONSTRAINT enrichment_queue_media_id_fkey;

-- Add index for media_type filtering
CREATE INDEX idx_enrichment_queue_media_type ON enrichment_queue(media_type, status);
