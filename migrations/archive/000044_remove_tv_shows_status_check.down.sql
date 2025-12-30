-- Restore CHECK constraint on tv_shows.status
-- Note: This will fail if there are rows with status values not in the CHECK list

CREATE TABLE tv_shows_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    library_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    original_title TEXT,
    sort_title TEXT,
    year INTEGER,
    first_air_date DATE,
    last_air_date DATE,
    genre TEXT,
    plot TEXT,
    status TEXT CHECK(status IN ('Returning Series', 'Planned', 'In Production', 'Ended', 'Cancelled', 'Pilot')),
    content_rating TEXT,
    maturity_rating INTEGER,
    network TEXT,
    original_language TEXT,
    country_of_origin TEXT,
    imdb_id TEXT,
    tmdb_id INTEGER,
    tvdb_id INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    directory TEXT,
    rating REAL,
    rating_votes INTEGER,
    tagline TEXT,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE CASCADE
);

INSERT INTO tv_shows_new SELECT * FROM tv_shows;
DROP TABLE tv_shows;
ALTER TABLE tv_shows_new RENAME TO tv_shows;

CREATE INDEX idx_tv_shows_library_id ON tv_shows(library_id);
CREATE INDEX idx_tv_shows_title ON tv_shows(title);
CREATE INDEX idx_tv_shows_sort_title ON tv_shows(sort_title);
CREATE INDEX idx_tv_shows_tvdb_id ON tv_shows(tvdb_id);
CREATE INDEX idx_tv_shows_tmdb_id ON tv_shows(tmdb_id);
CREATE INDEX idx_tv_shows_imdb_id ON tv_shows(imdb_id);
CREATE INDEX idx_tv_shows_status ON tv_shows(status);
CREATE INDEX idx_tv_shows_directory ON tv_shows(directory);

CREATE TRIGGER IF NOT EXISTS tr_tv_shows_delete_credits
AFTER DELETE ON tv_shows
FOR EACH ROW
BEGIN
    DELETE FROM credits WHERE media_type = 'tv_show' AND entity_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_tv_shows_delete_media_studios
AFTER DELETE ON tv_shows
FOR EACH ROW
BEGIN
    DELETE FROM media_studios WHERE media_type = 'tv_show' AND entity_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_tv_shows_delete_enrichment_queue
AFTER DELETE ON tv_shows
FOR EACH ROW
BEGIN
    DELETE FROM enrichment_queue WHERE media_type = 'tv_show' AND media_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_tv_shows_delete_external_ids
AFTER DELETE ON tv_shows
FOR EACH ROW
BEGIN
    DELETE FROM media_external_ids WHERE media_type = 'tv_show' AND entity_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_tv_shows_delete_images
AFTER DELETE ON tv_shows
FOR EACH ROW
BEGIN
    DELETE FROM media_images WHERE media_type = 'tv_show' AND entity_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_tv_shows_delete_enrichment_status
AFTER DELETE ON tv_shows
FOR EACH ROW
BEGIN
    DELETE FROM enrichment_status WHERE media_type = 'tv_show' AND media_id = OLD.id;
END;
