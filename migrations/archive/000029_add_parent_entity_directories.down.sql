-- Remove directory columns from parent entities

DROP INDEX IF EXISTS idx_tv_shows_directory;
ALTER TABLE tv_shows DROP COLUMN directory;

DROP INDEX IF EXISTS idx_music_albums_directory;
ALTER TABLE music_albums DROP COLUMN directory;

DROP INDEX IF EXISTS idx_music_artists_directory;
ALTER TABLE music_artists DROP COLUMN directory;
