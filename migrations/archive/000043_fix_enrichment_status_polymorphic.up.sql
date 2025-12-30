-- Fix enrichment_status to support polymorphic media types (tv_show, tv_season, etc.)
-- The table currently has an FK to media(id), but enrichment runs for tv_shows/tv_seasons
-- which are in separate tables. Add media_type column and remove the FK constraint.

-- SQLite doesn't support DROP CONSTRAINT, so we need to recreate the table.

-- Step 1: Create new table with correct schema (no FK, with media_type)
CREATE TABLE enrichment_status_new (
    media_type TEXT NOT NULL DEFAULT 'movie' CHECK(media_type IN ('movie', 'tv', 'tv_show', 'tv_season')),
    media_id INTEGER NOT NULL,
    stage TEXT NOT NULL,
    status TEXT DEFAULT 'pending' CHECK(status IN ('pending', 'processing', 'completed', 'skipped', 'failed')),
    plugin_id TEXT,
    completed_at TEXT,
    error_message TEXT,
    metadata_json TEXT,
    PRIMARY KEY (media_type, media_id, stage)
);

-- Step 2: Copy existing data (all existing entries are for media table, use 'movie' or 'tv' based on type)
-- Note: media.type is 'movie', 'tv_episode', or 'music_track'. Map 'tv_episode' to 'tv' for enrichment_status.
INSERT INTO enrichment_status_new (media_type, media_id, stage, status, plugin_id, completed_at, error_message, metadata_json)
SELECT
    CASE WHEN m.type = 'tv_episode' THEN 'tv' ELSE COALESCE(m.type, 'movie') END as media_type,
    es.media_id,
    es.stage,
    es.status,
    es.plugin_id,
    es.completed_at,
    es.error_message,
    es.metadata_json
FROM enrichment_status es
LEFT JOIN media m ON es.media_id = m.id;

-- Step 3: Drop old table and rename new one
DROP TABLE enrichment_status;
ALTER TABLE enrichment_status_new RENAME TO enrichment_status;

-- Step 4: Recreate index
CREATE INDEX idx_enrichment_status_stage ON enrichment_status(stage, status);
CREATE INDEX idx_enrichment_status_media ON enrichment_status(media_type, media_id);

-- Step 5: Add triggers for cascade deletes
CREATE TRIGGER IF NOT EXISTS tr_media_delete_enrichment_status
AFTER DELETE ON media
FOR EACH ROW
BEGIN
    DELETE FROM enrichment_status WHERE media_type IN ('movie', 'tv') AND media_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_tv_shows_delete_enrichment_status
AFTER DELETE ON tv_shows
FOR EACH ROW
BEGIN
    DELETE FROM enrichment_status WHERE media_type = 'tv_show' AND media_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_tv_seasons_delete_enrichment_status
AFTER DELETE ON tv_seasons
FOR EACH ROW
BEGIN
    DELETE FROM enrichment_status WHERE media_type = 'tv_season' AND media_id = OLD.id;
END;
