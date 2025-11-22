-- Remove artist_id columns
ALTER TABLE music_tracks DROP COLUMN IF EXISTS artist_id;
ALTER TABLE music_albums DROP COLUMN IF EXISTS artist_id;

-- Drop music_artists table
DROP TABLE IF EXISTS music_artists;
