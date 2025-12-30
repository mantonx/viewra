-- Add music_albums table for proper music album entity support
-- This replaces the workaround of using track IDs as album representatives

CREATE TABLE music_albums (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    library_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    album_artist TEXT,
    artist TEXT,  -- Fallback if album_artist is not set
    year INTEGER,
    release_date DATE,
    genre TEXT,
    total_tracks INTEGER,
    total_discs INTEGER,
    record_label TEXT,
    release_type TEXT CHECK(release_type IN ('album', 'single', 'ep', 'compilation', 'live', 'remix', 'soundtrack')),
    compilation BOOLEAN DEFAULT 0,
    musicbrainz_album_id TEXT UNIQUE,
    cover_art_path TEXT,
    sort_title TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE CASCADE,
    UNIQUE(library_id, title, album_artist)
);

CREATE INDEX idx_music_albums_library_id ON music_albums(library_id);
CREATE INDEX idx_music_albums_album_artist ON music_albums(album_artist);
CREATE INDEX idx_music_albums_artist ON music_albums(artist);
CREATE INDEX idx_music_albums_year ON music_albums(year);
CREATE INDEX idx_music_albums_musicbrainz_album_id ON music_albums(musicbrainz_album_id);

-- Add album_id foreign key to music_tracks
ALTER TABLE music_tracks ADD COLUMN album_id INTEGER REFERENCES music_albums(id) ON DELETE SET NULL;

CREATE INDEX idx_music_tracks_album_id ON music_tracks(album_id);
