-- Add people, credits, and studios tables for comprehensive metadata support
-- This enables storing cast/crew with roles, photos, and external IDs

-- People table: normalized storage for cast, directors, writers, creators
CREATE TABLE people (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    sort_name TEXT,
    photo_path TEXT,           -- Local cached path
    photo_url TEXT,            -- Original remote URL
    imdb_id TEXT,
    tmdb_id INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_people_name ON people(name);
CREATE UNIQUE INDEX idx_people_tmdb_id ON people(tmdb_id) WHERE tmdb_id IS NOT NULL;
CREATE UNIQUE INDEX idx_people_imdb_id ON people(imdb_id) WHERE imdb_id IS NOT NULL;

-- Credits table: links people to media with role information
CREATE TABLE credits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    person_id INTEGER NOT NULL,
    media_type TEXT NOT NULL CHECK(media_type IN (
        'movie', 'tv_show', 'tv_episode', 'music_track'
    )),
    entity_id INTEGER NOT NULL,
    credit_type TEXT NOT NULL CHECK(credit_type IN (
        'cast', 'director', 'writer', 'creator', 'guest', 'composer', 'producer'
    )),
    character_name TEXT,       -- For cast: "Tony Stark"
    department TEXT,           -- For crew: "Directing", "Writing", "Production"
    job TEXT,                  -- For crew: "Director", "Screenplay", "Executive Producer"
    billing_order INTEGER,     -- Cast ordering
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (person_id) REFERENCES people(id) ON DELETE CASCADE
);

CREATE INDEX idx_credits_person_id ON credits(person_id);
CREATE INDEX idx_credits_entity ON credits(media_type, entity_id);
CREATE INDEX idx_credits_type ON credits(credit_type);
CREATE INDEX idx_credits_billing ON credits(media_type, entity_id, billing_order);

-- Studios table: production companies
CREATE TABLE studios (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    logo_path TEXT,
    tmdb_id INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_studios_tmdb_id ON studios(tmdb_id) WHERE tmdb_id IS NOT NULL;

-- Media-studios junction table
CREATE TABLE media_studios (
    media_type TEXT NOT NULL CHECK(media_type IN ('movie', 'tv_show')),
    entity_id INTEGER NOT NULL,
    studio_id INTEGER NOT NULL,
    PRIMARY KEY (media_type, entity_id, studio_id),
    FOREIGN KEY (studio_id) REFERENCES studios(id) ON DELETE CASCADE
);

CREATE INDEX idx_media_studios_studio ON media_studios(studio_id);
