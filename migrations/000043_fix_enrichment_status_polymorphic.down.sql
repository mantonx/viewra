-- Revert enrichment_status to original schema with FK constraint
-- Note: This will lose the media_type column and any non-media entries

-- Drop triggers
DROP TRIGGER IF EXISTS tr_media_delete_enrichment_status;
DROP TRIGGER IF EXISTS tr_tv_shows_delete_enrichment_status;
DROP TRIGGER IF EXISTS tr_tv_seasons_delete_enrichment_status;

-- Recreate original table with FK
CREATE TABLE enrichment_status_old (
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    stage TEXT NOT NULL,
    status TEXT DEFAULT 'pending' CHECK(status IN ('pending', 'processing', 'completed', 'skipped', 'failed')),
    plugin_id TEXT,
    completed_at TEXT,
    error_message TEXT,
    metadata_json TEXT,
    PRIMARY KEY (media_id, stage)
);

-- Copy only movie/tv data that exists in media table
INSERT INTO enrichment_status_old (media_id, stage, status, plugin_id, completed_at, error_message, metadata_json)
SELECT es.media_id, es.stage, es.status, es.plugin_id, es.completed_at, es.error_message, es.metadata_json
FROM enrichment_status es
WHERE es.media_type IN ('movie', 'tv') AND EXISTS (SELECT 1 FROM media m WHERE m.id = es.media_id);

-- Swap tables
DROP TABLE enrichment_status;
ALTER TABLE enrichment_status_old RENAME TO enrichment_status;

-- Recreate original index
CREATE INDEX idx_enrichment_status_stage ON enrichment_status(stage, status);
