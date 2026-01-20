-- Store similar/recommended titles for movies and TV shows (from TMDb recommendations)
CREATE TABLE IF NOT EXISTS media_similar_titles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    media_type TEXT NOT NULL,  -- "movie" or "tv_show"
    similar_title TEXT NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,  -- Order from TMDb (lower = more relevant)
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(media_id, media_type, similar_title)
);

-- Index for efficient lookup by media
CREATE INDEX IF NOT EXISTS idx_media_similar_titles_media ON media_similar_titles(media_id, media_type);
