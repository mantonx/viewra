-- Add tv_season, music_album, music_artist as valid media_types for enrichment pipelines
-- These are parent entities that need their own enrichment (images, metadata)

-- Drop the existing CHECK constraint
ALTER TABLE enrichment_pipelines DROP CONSTRAINT IF EXISTS enrichment_pipelines_media_type_check;

-- Add new CHECK constraint with parent entity types
ALTER TABLE enrichment_pipelines ADD CONSTRAINT enrichment_pipelines_media_type_check
    CHECK(media_type IN ('movie', 'tv', 'tv_show', 'tv_season', 'music', 'music_album', 'music_artist'));

-- Add tv_season pipeline configuration (local images only - seasons get metadata from show)
INSERT INTO enrichment_pipelines (media_type, plugin_id, stage_name, position, enabled) VALUES
    ('tv_season', 'builtin:local-images', 'local-images', 1, true);

-- Add music_album pipeline configuration
INSERT INTO enrichment_pipelines (media_type, plugin_id, stage_name, position, enabled) VALUES
    ('music_album', 'builtin:local-images', 'local-images', 1, true);

-- Add music_artist pipeline configuration
INSERT INTO enrichment_pipelines (media_type, plugin_id, stage_name, position, enabled) VALUES
    ('music_artist', 'builtin:local-images', 'local-images', 1, true);
