-- Revert: Remove media_type column from enrichment_queue

-- Drop new index
DROP INDEX IF EXISTS idx_enrichment_queue_media_type;

-- Re-add FK constraint (only works for rows that exist in media table)
-- First, delete any rows that reference non-media entities
DELETE FROM enrichment_queue WHERE media_type IN ('tv_show');

-- Add back the FK constraint
ALTER TABLE enrichment_queue ADD CONSTRAINT enrichment_queue_media_id_fkey
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE;

-- Drop new unique constraint and recreate original
ALTER TABLE enrichment_queue DROP CONSTRAINT enrichment_queue_media_id_media_type_stage_key;
ALTER TABLE enrichment_queue ADD CONSTRAINT enrichment_queue_media_id_stage_key
    UNIQUE(media_id, stage);

-- Remove column and its constraint
ALTER TABLE enrichment_queue DROP CONSTRAINT enrichment_queue_media_type_check;
ALTER TABLE enrichment_queue DROP COLUMN media_type;
