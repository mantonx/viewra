-- Recreate old AI tables (data will be lost)
-- These tables are now managed by plugins, so this is just for rollback compatibility

-- Recreate mood_tags table
CREATE TABLE IF NOT EXISTS mood_tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_type TEXT NOT NULL CHECK(entity_type IN (
        'movie', 'tv_show', 'tv_episode',
        'music_artist', 'music_album', 'music_track'
    )),
    entity_id INTEGER NOT NULL,
    tag TEXT NOT NULL,
    confidence REAL DEFAULT 1.0,
    created_at TEXT DEFAULT (datetime('now')),
    UNIQUE(entity_type, entity_id, tag)
);

CREATE INDEX IF NOT EXISTS idx_mood_tags_entity ON mood_tags(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_mood_tags_tag ON mood_tags(tag);

-- Recreate embeddings table
CREATE TABLE IF NOT EXISTS embeddings (
    id INTEGER PRIMARY KEY,
    entity_type TEXT NOT NULL,
    entity_id INTEGER NOT NULL,
    vector BLOB NOT NULL,
    text TEXT,
    dimensions INTEGER NOT NULL DEFAULT 768,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(entity_type, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_embeddings_entity ON embeddings(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_embeddings_type ON embeddings(entity_type);

-- Note: vec0 virtual table recreation requires sqlite-vec extension
-- This is handled by the application on startup if needed
