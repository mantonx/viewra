-- Remove foreign key constraint from enrichment_pipelines.plugin_id
ALTER TABLE enrichment_pipelines
    DROP CONSTRAINT IF EXISTS fk_enrichment_pipelines_plugin;
