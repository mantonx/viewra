-- Add tv_season, music_album, music_artist as valid media_types for enrichment pipelines
-- These are parent entities that need their own enrichment (images, metadata)

-- SQLite doesn't support ALTER CHECK constraint, so we need to recreate the table
CREATE TABLE enrichment_pipelines_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_type TEXT NOT NULL CHECK(media_type IN ('movie', 'tv', 'tv_show', 'tv_season', 'music', 'music_album', 'music_artist')),
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

-- Copy existing data
INSERT INTO enrichment_pipelines_new
    (id, media_type, plugin_id, stage_name, position, enabled, config_json, created_at, updated_at)
SELECT id, media_type, plugin_id, stage_name, position, enabled, config_json, created_at, updated_at
FROM enrichment_pipelines;

-- Drop old table and rename new
DROP TABLE enrichment_pipelines;
ALTER TABLE enrichment_pipelines_new RENAME TO enrichment_pipelines;

-- Recreate index
CREATE INDEX idx_enrichment_pipelines_order ON enrichment_pipelines(media_type, enabled, position);

-- Add tv_season pipeline configuration (local images only - seasons get metadata from show)
INSERT INTO enrichment_pipelines (media_type, plugin_id, stage_name, position, enabled) VALUES
    ('tv_season', 'builtin:local-images', 'local-images', 1, 1);

-- Add music_album pipeline configuration
INSERT INTO enrichment_pipelines (media_type, plugin_id, stage_name, position, enabled) VALUES
    ('music_album', 'builtin:local-images', 'local-images', 1, 1);

-- Add music_artist pipeline configuration
INSERT INTO enrichment_pipelines (media_type, plugin_id, stage_name, position, enabled) VALUES
    ('music_artist', 'builtin:local-images', 'local-images', 1, 1);
