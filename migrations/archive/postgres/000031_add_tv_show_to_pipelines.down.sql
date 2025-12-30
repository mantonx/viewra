-- Remove tv_show from enrichment pipelines

-- Delete tv_show entries
DELETE FROM enrichment_pipelines WHERE media_type = 'tv_show';

-- Drop the current CHECK constraint
ALTER TABLE enrichment_pipelines DROP CONSTRAINT IF EXISTS enrichment_pipelines_media_type_check;

-- Add back original CHECK constraint without tv_show
ALTER TABLE enrichment_pipelines ADD CONSTRAINT enrichment_pipelines_media_type_check
    CHECK(media_type IN ('movie', 'tv', 'music'));
