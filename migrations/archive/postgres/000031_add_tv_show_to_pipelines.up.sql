-- Add tv_show as a valid media_type for enrichment pipelines
-- TV shows are parent entities that need their own enrichment (NFO parsing, images)

-- Drop the existing CHECK constraint
ALTER TABLE enrichment_pipelines DROP CONSTRAINT IF EXISTS enrichment_pipelines_media_type_check;

-- Add new CHECK constraint with tv_show
ALTER TABLE enrichment_pipelines ADD CONSTRAINT enrichment_pipelines_media_type_check
    CHECK(media_type IN ('movie', 'tv', 'tv_show', 'music'));

-- Add tv_show pipeline configuration
INSERT INTO enrichment_pipelines (media_type, plugin_id, stage_name, position, enabled) VALUES
    ('tv_show', 'builtin:nfo', 'nfo', 1, true),
    ('tv_show', 'builtin:local-images', 'local-images', 2, true);
