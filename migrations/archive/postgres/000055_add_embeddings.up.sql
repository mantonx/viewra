-- Enable pgvector extension (requires superuser or extension already installed)
-- If this fails, the extension needs to be installed by a database admin:
-- CREATE EXTENSION IF NOT EXISTS vector;
DO $$
BEGIN
    CREATE EXTENSION IF NOT EXISTS vector;
EXCEPTION
    WHEN insufficient_privilege THEN
        RAISE NOTICE 'pgvector extension not installed. Please run: CREATE EXTENSION vector;';
    WHEN undefined_file THEN
        RAISE NOTICE 'pgvector extension not available. Please install pgvector first.';
END $$;

-- Embeddings table for semantic search (PostgreSQL with pgvector)
-- Uses vector(768) type for efficient similarity search
CREATE TABLE IF NOT EXISTS embeddings (
    id SERIAL PRIMARY KEY,
    entity_type TEXT NOT NULL,  -- 'movie', 'tv_show', 'tv_episode', 'music_artist', 'music_album', 'music_track'
    entity_id INTEGER NOT NULL,
    vector vector(768) NOT NULL,  -- pgvector type with 768 dimensions
    text TEXT,                     -- Original text that was embedded
    dimensions INTEGER NOT NULL DEFAULT 768,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(entity_type, entity_id)
);

-- Create HNSW index for fast approximate nearest neighbor search
-- HNSW is faster than IVFFlat for most use cases
CREATE INDEX IF NOT EXISTS idx_embeddings_vector ON embeddings 
    USING hnsw (vector vector_cosine_ops);

CREATE INDEX IF NOT EXISTS idx_embeddings_entity ON embeddings(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_embeddings_type ON embeddings(entity_type);
