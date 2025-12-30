-- Remove tv_season, music_album, music_artist media types from enrichment_queue

-- Delete queue entries for parent entity types
DELETE FROM enrichment_queue WHERE media_type IN ('tv_season', 'music_album', 'music_artist');

-- Drop the existing CHECK constraint
ALTER TABLE enrichment_queue DROP CONSTRAINT IF EXISTS enrichment_queue_media_type_check;

-- Restore previous CHECK constraint without parent entity types
ALTER TABLE enrichment_queue ADD CONSTRAINT enrichment_queue_media_type_check
    CHECK(media_type IN ('movie', 'tv', 'tv_show', 'music'));
