-- Remove tv_season, music_album, music_artist media types from enrichment pipelines

-- Delete pipeline configurations for parent entity types
DELETE FROM enrichment_pipelines WHERE media_type IN ('tv_season', 'music_album', 'music_artist');

-- Drop the existing CHECK constraint
ALTER TABLE enrichment_pipelines DROP CONSTRAINT IF EXISTS enrichment_pipelines_media_type_check;

-- Restore previous CHECK constraint without parent entity types
ALTER TABLE enrichment_pipelines ADD CONSTRAINT enrichment_pipelines_media_type_check
    CHECK(media_type IN ('movie', 'tv', 'tv_show', 'music'));
