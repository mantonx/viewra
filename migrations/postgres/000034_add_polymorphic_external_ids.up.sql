-- Add polymorphic support to media_external_ids table
-- This allows storing external IDs for parent entities (tv_show, tv_season, music_album, music_artist)
-- that don't have entries in the media table.

-- Create media type enum if not exists
DO $$ BEGIN
    CREATE TYPE external_id_media_type AS ENUM (
        'movie', 'tv_show', 'tv_season', 'tv_episode',
        'music_artist', 'music_album', 'music_track'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Create new table with polymorphic support
CREATE TABLE IF NOT EXISTS media_external_ids_new (
    id SERIAL PRIMARY KEY,

    -- Polymorphic association: either media_id OR (media_type + entity_id)
    media_id INTEGER,                       -- FK to media.id (for movies, episodes, tracks with media files)
    media_type external_id_media_type NOT NULL,
    entity_id INTEGER NOT NULL,             -- ID in specific table (tv_shows.id, albums, etc.)

    -- External ID data
    provider TEXT NOT NULL,
    external_id TEXT NOT NULL,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Foreign key for media table entries
    CONSTRAINT fk_media_external_ids_media FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

-- Migrate existing data
INSERT INTO media_external_ids_new (media_id, media_type, entity_id, provider, external_id, created_at, updated_at)
SELECT
    e.media_id,
    CASE m.type
        WHEN 'movie' THEN 'movie'::external_id_media_type
        WHEN 'tv' THEN 'tv_episode'::external_id_media_type
        WHEN 'music' THEN 'music_track'::external_id_media_type
        ELSE 'movie'::external_id_media_type
    END,
    e.media_id,
    e.provider,
    e.external_id,
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
