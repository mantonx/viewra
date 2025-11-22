-- Remove artist_id foreign keys from music_tracks and music_albums
DROP INDEX IF EXISTS idx_music_tracks_artist_id;
DROP INDEX IF EXISTS idx_music_albums_artist_id;

-- Note: SQLite doesn't support DROP COLUMN directly in older versions
-- For production, you'd need to recreate the tables
-- For development, we'll just drop the indices

-- Drop music_artists table
DROP TABLE IF EXISTS music_artists;
