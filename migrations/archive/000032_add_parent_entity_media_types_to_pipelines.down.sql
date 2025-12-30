-- Remove tv_season, music_album, music_artist media types from enrichment pipelines

-- Delete pipeline configurations for parent entity types
DELETE FROM enrichment_pipelines WHERE media_type IN ('tv_season', 'music_album', 'music_artist');

-- SQLite doesn't support ALTER CHECK constraint, so we need to recreate the table
CREATE TABLE enrichment_pipelines_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_type TEXT NOT NULL CHECK(media_type IN ('movie', 'tv', 'tv_show', 'music')),
    plugin_id TEXT NOT NULL,
    stage_name TEXT NOT NULL,
    position INTEGER NOT NULL,
    enabled INTEGER DEFAULT 1,
    config_json TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    UNIQUE(media_type, position),
    UNIQUE(media_type, plugin_id)
);

-- Copy existing data (excluding new types which should already be deleted)
INSERT INTO enrichment_pipelines_new
    (id, media_type, plugin_id, stage_name, position, enabled, config_json, created_at, updated_at)
SELECT id, media_type, plugin_id, stage_name, position, enabled, config_json, created_at, updated_at
FROM enrichment_pipelines;

-- Drop old table and rename new
DROP TABLE enrichment_pipelines;
ALTER TABLE enrichment_pipelines_new RENAME TO enrichment_pipelines;

-- Recreate index
CREATE INDEX idx_enrichment_pipelines_order ON enrichment_pipelines(media_type, enabled, position);
