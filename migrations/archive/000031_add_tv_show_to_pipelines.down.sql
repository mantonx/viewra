-- Remove tv_show from enrichment pipelines

-- Delete tv_show entries
DELETE FROM enrichment_pipelines WHERE media_type = 'tv_show';

-- Recreate table without tv_show in CHECK constraint
CREATE TABLE enrichment_pipelines_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_type TEXT NOT NULL CHECK(media_type IN ('movie', 'tv', 'music')),
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

INSERT INTO enrichment_pipelines_new
    (id, media_type, plugin_id, stage_name, position, enabled, config_json, created_at, updated_at)
SELECT id, media_type, plugin_id, stage_name, position, enabled, config_json, created_at, updated_at
FROM enrichment_pipelines;

DROP TABLE enrichment_pipelines;
ALTER TABLE enrichment_pipelines_new RENAME TO enrichment_pipelines;

CREATE INDEX idx_enrichment_pipelines_order ON enrichment_pipelines(media_type, enabled, position);
