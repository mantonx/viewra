-- Add polymorphic support to media_external_ids table
-- This allows storing external IDs for parent entities (tv_show, tv_season, music_album, music_artist)
-- that don't have entries in the media table.

-- Create new table with polymorphic support (matching media_images pattern)
CREATE TABLE IF NOT EXISTS media_external_ids_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    -- Polymorphic association: either media_id OR (media_type + entity_id)
    media_id INTEGER,                       -- FK to media.id (for movies, episodes, tracks with media files)
    media_type TEXT NOT NULL CHECK(media_type IN (
        'movie', 'tv_show', 'tv_season', 'tv_episode',
        'music_artist', 'music_album', 'music_track'
    )),
    entity_id INTEGER NOT NULL,             -- ID in specific table (tv_shows.id, albums, etc.)

    -- External ID data
    provider TEXT NOT NULL,
    external_id TEXT NOT NULL,

    -- Timestamps
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),

    -- Foreign key for media table entries (movies, episodes, tracks)
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

-- Migrate existing data (all existing entries are for media table entities)
INSERT INTO media_external_ids_new (media_id, media_type, entity_id, provider, external_id, created_at, updated_at)
SELECT
    media_id,
    CASE
        WHEN m.type = 'movie' THEN 'movie'
        WHEN m.type = 'tv' THEN 'tv_episode'
        WHEN m.type = 'music' THEN 'music_track'
        ELSE 'movie'  -- fallback
    END as media_type,
    media_id as entity_id,
    provider,
    external_id,
    e.created_at,
    e.updated_at
FROM media_external_ids e
JOIN media m ON e.media_id = m.id;

-- Drop old table and rename
DROP TABLE media_external_ids;
ALTER TABLE media_external_ids_new RENAME TO media_external_ids;

-- Create indexes
CREATE INDEX idx_media_external_ids_media_id ON media_external_ids(media_id) WHERE media_id IS NOT NULL;
CREATE INDEX idx_media_external_ids_entity ON media_external_ids(media_type, entity_id);
CREATE INDEX idx_media_external_ids_lookup ON media_external_ids(provider, external_id);

-- Unique constraint: one external ID per provider per entity
CREATE UNIQUE INDEX idx_media_external_ids_unique ON media_external_ids(media_type, entity_id, provider);
