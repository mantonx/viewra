-- Revert HNSW index to default parameters

DROP INDEX IF EXISTS idx_embeddings_vector;

-- Recreate with default parameters
CREATE INDEX idx_embeddings_vector ON embeddings 
    USING hnsw (vector vector_cosine_ops);
