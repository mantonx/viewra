-- Add tv_season, music_album, music_artist as valid media_types for enrichment_queue
-- These are parent entities that need their own enrichment jobs

-- Drop the existing CHECK constraint
ALTER TABLE enrichment_queue DROP CONSTRAINT IF EXISTS enrichment_queue_media_type_check;

-- Add new CHECK constraint with parent entity types
ALTER TABLE enrichment_queue ADD CONSTRAINT enrichment_queue_media_type_check
    CHECK(media_type IN ('movie', 'tv', 'tv_show', 'tv_season', 'music', 'music_album', 'music_artist'));
