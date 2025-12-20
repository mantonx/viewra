-- Embeddings table for semantic search (SQLite version)
-- Vectors are stored as BLOB (serialized float32 array)
-- For sqlite-vss, a virtual table will be created at runtime
CREATE TABLE IF NOT EXISTS embeddings (
    id INTEGER PRIMARY KEY,
    entity_type TEXT NOT NULL,  -- 'movie', 'tv_show', 'tv_episode', 'music_artist', 'music_album', 'music_track'
    entity_id INTEGER NOT NULL,
    vector BLOB NOT NULL,       -- Serialized float32 array
    text TEXT,                  -- Original text that was embedded
    dimensions INTEGER NOT NULL DEFAULT 768,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(entity_type, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_embeddings_entity ON embeddings(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_embeddings_type ON embeddings(entity_type);
