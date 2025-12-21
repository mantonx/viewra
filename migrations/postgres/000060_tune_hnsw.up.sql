-- Tune HNSW index parameters for better recall/performance balance
--
-- HNSW Parameters:
-- - m: Number of connections per node (default 16, good for high-dimensional vectors)
-- - ef_construction: Build-time search depth (higher = better recall, slower build)
--
-- Runtime parameter hnsw.ef_search should be set per-session for query-time tuning

-- Drop existing index and recreate with tuned parameters
DROP INDEX IF EXISTS idx_embeddings_vector;

-- Recreate HNSW index with optimized parameters for 768-dimensional embeddings
-- m=16: Good balance for high-dim vectors (768)
-- ef_construction=64: Higher than default (40) for better recall at build time
CREATE INDEX idx_embeddings_vector ON embeddings 
    USING hnsw (vector vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);

-- Add comment documenting the runtime parameter
COMMENT ON INDEX idx_embeddings_vector IS 
    'HNSW vector index for semantic search. For optimal recall, set hnsw.ef_search=40 at session level.';
