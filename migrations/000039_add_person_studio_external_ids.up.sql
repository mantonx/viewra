-- Add person and studio to media_external_ids media_type CHECK constraint
-- This enables storing external IDs (TMDb, IMDb, etc.) for people and studios

-- SQLite doesn't support ALTER TABLE to modify CHECK constraints directly
-- We need to recreate the table with the new constraint

-- Create new table with expanded media_type constraint
CREATE TABLE IF NOT EXISTS media_external_ids_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    -- Polymorphic association: either media_id OR (media_type + entity_id)
    media_id INTEGER,                       -- FK to media.id (for movies, episodes, tracks with media files)
    media_type TEXT NOT NULL CHECK(media_type IN (
        'movie', 'tv_show', 'tv_season', 'tv_episode',
        'music_artist', 'music_album', 'music_track',
        'person', 'studio'
    )),
    entity_id INTEGER NOT NULL,             -- ID in specific table (people.id, studios.id, etc.)

    -- External ID data
    provider TEXT NOT NULL,
    external_id TEXT NOT NULL,

    -- Timestamps
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),

    -- Foreign key for media table entries (movies, episodes, tracks)
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

-- Migrate existing data
INSERT INTO media_external_ids_new (id, media_id, media_type, entity_id, provider, external_id, created_at, updated_at)
SELECT id, media_id, media_type, entity_id, provider, external_id, created_at, updated_at
FROM media_external_ids;

-- Drop old table and rename
DROP TABLE media_external_ids;
ALTER TABLE media_external_ids_new RENAME TO media_external_ids;

-- Recreate indexes
CREATE INDEX idx_media_external_ids_media_id ON media_external_ids(media_id) WHERE media_id IS NOT NULL;
CREATE INDEX idx_media_external_ids_entity ON media_external_ids(media_type, entity_id);
CREATE INDEX idx_media_external_ids_lookup ON media_external_ids(provider, external_id);
CREATE UNIQUE INDEX idx_media_external_ids_unique ON media_external_ids(media_type, entity_id, provider);

-- Migrate existing tmdb_id/imdb_id from people table to external IDs
INSERT OR IGNORE INTO media_external_ids (media_type, entity_id, provider, external_id)
SELECT 'person', id, 'tmdb', CAST(tmdb_id AS TEXT)
FROM people
WHERE tmdb_id IS NOT NULL;

INSERT OR IGNORE INTO media_external_ids (media_type, entity_id, provider, external_id)
SELECT 'person', id, 'imdb', imdb_id
FROM people
WHERE imdb_id IS NOT NULL AND imdb_id != '';

-- Migrate existing tmdb_id from studios table to external IDs
INSERT OR IGNORE INTO media_external_ids (media_type, entity_id, provider, external_id)
SELECT 'studio', id, 'tmdb', CAST(tmdb_id AS TEXT)
FROM studios
WHERE tmdb_id IS NOT NULL;
