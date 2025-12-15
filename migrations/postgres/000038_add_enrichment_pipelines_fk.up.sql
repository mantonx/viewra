-- Add foreign key constraint from enrichment_pipelines.plugin_id to plugins.id
-- This ensures pipeline entries reference valid plugins and are cleaned up on plugin deletion

-- First, delete any orphaned pipeline entries (plugin_ids that don't exist in plugins)
DELETE FROM enrichment_pipelines
WHERE plugin_id NOT IN (SELECT id FROM plugins);

-- Add the foreign key constraint
ALTER TABLE enrichment_pipelines
    ADD CONSTRAINT fk_enrichment_pipelines_plugin
    FOREIGN KEY (plugin_id) REFERENCES plugins(id) ON DELETE CASCADE;
