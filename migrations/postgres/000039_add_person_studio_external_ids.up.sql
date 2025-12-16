-- Add person and studio to media_external_ids media_type CHECK constraint
-- This enables storing external IDs (TMDb, IMDb, etc.) for people and studios

-- PostgreSQL supports ALTER TABLE to modify CHECK constraints
-- Drop the old constraint and add new one

-- First, drop the existing check constraint
ALTER TABLE media_external_ids DROP CONSTRAINT IF EXISTS media_external_ids_media_type_check;

-- Add new constraint with person and studio
ALTER TABLE media_external_ids ADD CONSTRAINT media_external_ids_media_type_check
    CHECK (media_type IN (
        'movie', 'tv_show', 'tv_season', 'tv_episode',
        'music_artist', 'music_album', 'music_track',
        'person', 'studio'
    ));

-- Migrate existing tmdb_id/imdb_id from people table to external IDs
INSERT INTO media_external_ids (media_type, entity_id, provider, external_id)
SELECT 'person', id, 'tmdb', tmdb_id::TEXT
FROM people
WHERE tmdb_id IS NOT NULL
ON CONFLICT (media_type, entity_id, provider) DO NOTHING;

INSERT INTO media_external_ids (media_type, entity_id, provider, external_id)
SELECT 'person', id, 'imdb', imdb_id
FROM people
WHERE imdb_id IS NOT NULL AND imdb_id != ''
ON CONFLICT (media_type, entity_id, provider) DO NOTHING;

-- Migrate existing tmdb_id from studios table to external IDs
INSERT INTO media_external_ids (media_type, entity_id, provider, external_id)
SELECT 'studio', id, 'tmdb', tmdb_id::TEXT
FROM studios
WHERE tmdb_id IS NOT NULL
ON CONFLICT (media_type, entity_id, provider) DO NOTHING;
