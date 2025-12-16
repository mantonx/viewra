-- Revert: Remove person and studio from media_external_ids

-- Delete migrated person/studio external IDs
DELETE FROM media_external_ids WHERE media_type IN ('person', 'studio');

-- Recreate table with original constraint (without person/studio)
CREATE TABLE IF NOT EXISTS media_external_ids_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER,
    media_type TEXT NOT NULL CHECK(media_type IN (
        'movie', 'tv_show', 'tv_season', 'tv_episode',
        'music_artist', 'music_album', 'music_track'
    )),
    entity_id INTEGER NOT NULL,
    provider TEXT NOT NULL,
    external_id TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

-- Migrate remaining data
INSERT INTO media_external_ids_old (id, media_id, media_type, entity_id, provider, external_id, created_at, updated_at)
SELECT id, media_id, media_type, entity_id, provider, external_id, created_at, updated_at
FROM media_external_ids
WHERE media_type NOT IN ('person', 'studio');

-- Drop and rename
DROP TABLE media_external_ids;
ALTER TABLE media_external_ids_old RENAME TO media_external_ids;

-- Recreate indexes
CREATE INDEX idx_media_external_ids_media_id ON media_external_ids(media_id) WHERE media_id IS NOT NULL;
CREATE INDEX idx_media_external_ids_entity ON media_external_ids(media_type, entity_id);
CREATE INDEX idx_media_external_ids_lookup ON media_external_ids(provider, external_id);
CREATE UNIQUE INDEX idx_media_external_ids_unique ON media_external_ids(media_type, entity_id, provider);
