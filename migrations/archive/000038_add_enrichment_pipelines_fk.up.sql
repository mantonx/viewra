-- Add foreign key constraint from enrichment_pipelines.plugin_id to plugins.id
-- This ensures pipeline entries reference valid plugins and are cleaned up on plugin deletion

-- SQLite doesn't support ALTER TABLE ADD CONSTRAINT, so we recreate the table
CREATE TABLE enrichment_pipelines_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_type TEXT NOT NULL CHECK(media_type IN ('movie', 'tv', 'tv_show', 'tv_season', 'music', 'music_album', 'music_artist')),
    plugin_id TEXT NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
    stage_name TEXT NOT NULL,
    position INTEGER NOT NULL,
    enabled INTEGER DEFAULT 1,
    config_json TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    UNIQUE(media_type, position),
    UNIQUE(media_type, plugin_id)
);

-- Copy existing data (only rows with valid plugin_ids)
INSERT INTO enrichment_pipelines_new
    (id, media_type, plugin_id, stage_name, position, enabled, config_json, created_at, updated_at)
SELECT ep.id, ep.media_type, ep.plugin_id, ep.stage_name, ep.position, ep.enabled, ep.config_json, ep.created_at, ep.updated_at
FROM enrichment_pipelines ep
WHERE EXISTS (SELECT 1 FROM plugins p WHERE p.id = ep.plugin_id);

-- Drop old table and rename new
DROP TABLE enrichment_pipelines;
ALTER TABLE enrichment_pipelines_new RENAME TO enrichment_pipelines;

-- Recreate index
CREATE INDEX idx_enrichment_pipelines_order ON enrichment_pipelines(media_type, enabled, position);
