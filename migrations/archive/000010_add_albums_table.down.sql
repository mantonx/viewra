-- Remove album_id from music_tracks
DROP INDEX IF EXISTS idx_music_tracks_album_id;
ALTER TABLE music_tracks DROP COLUMN album_id;

-- Remove music_albums table and indices
DROP INDEX IF EXISTS idx_music_albums_musicbrainz_album_id;
DROP INDEX IF EXISTS idx_music_albums_year;
DROP INDEX IF EXISTS idx_music_albums_artist;
DROP INDEX IF EXISTS idx_music_albums_album_artist;
DROP INDEX IF EXISTS idx_music_albums_library_id;
DROP TABLE IF EXISTS music_albums;
