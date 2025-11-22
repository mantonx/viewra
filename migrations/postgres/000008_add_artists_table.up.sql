-- Create music_artists table for proper artist entities
CREATE TABLE music_artists (
    id SERIAL PRIMARY KEY,
    library_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    sort_name TEXT,
    musicbrainz_artist_id TEXT UNIQUE,
    bio TEXT,
    country TEXT,
    formed_year INTEGER,
    genre TEXT,
    image_path TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE CASCADE,
    UNIQUE(library_id, name)
);

CREATE INDEX idx_music_artists_library_id ON music_artists(library_id);
CREATE INDEX idx_music_artists_name ON music_artists(name);
CREATE INDEX idx_music_artists_sort_name ON music_artists(sort_name);
CREATE INDEX idx_music_artists_musicbrainz_artist_id ON music_artists(musicbrainz_artist_id);

-- Add artist_id foreign key to music_albums table
ALTER TABLE music_albums ADD COLUMN artist_id INTEGER REFERENCES music_artists(id) ON DELETE SET NULL;
CREATE INDEX idx_music_albums_artist_id ON music_albums(artist_id);

-- Add artist_id foreign key to music_tracks table
ALTER TABLE music_tracks ADD COLUMN artist_id INTEGER REFERENCES music_artists(id) ON DELETE SET NULL;
CREATE INDEX idx_music_tracks_artist_id ON music_tracks(artist_id);
