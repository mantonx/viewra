-- Add directory column to parent entities for enrichment pipeline support
-- Parent entities (TV shows, music albums, music artists) don't have file paths
-- like their children (episodes, tracks), but need directory paths for:
-- - Finding NFO/metadata files
-- - Locating artwork (posters, fanart, covers)
-- - Supporting the async enrichment pipeline

-- TV Shows: stores the show directory (e.g., /media/tv/Breaking Bad)
ALTER TABLE tv_shows ADD COLUMN directory TEXT;
CREATE INDEX idx_tv_shows_directory ON tv_shows(directory);

-- Music Albums: stores the album directory (e.g., /media/music/Artist/Album)
ALTER TABLE music_albums ADD COLUMN directory TEXT;
CREATE INDEX idx_music_albums_directory ON music_albums(directory);

-- Music Artists: stores the artist directory (e.g., /media/music/Artist)
ALTER TABLE music_artists ADD COLUMN directory TEXT;
CREATE INDEX idx_music_artists_directory ON music_artists(directory);
