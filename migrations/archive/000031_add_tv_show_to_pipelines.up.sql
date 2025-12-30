-- Add tv_show as a valid media_type for enrichment pipelines
-- TV shows are parent entities that need their own enrichment (NFO parsing, images)

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

-- Add tv_show pipeline configuration
INSERT INTO enrichment_pipelines (media_type, plugin_id, stage_name, position, enabled) VALUES
    ('tv_show', 'builtin:nfo', 'nfo', 1, 1),
    ('tv_show', 'builtin:local-images', 'local-images', 2, 1);
