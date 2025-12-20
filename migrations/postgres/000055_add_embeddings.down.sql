DROP INDEX IF EXISTS idx_embeddings_type;
DROP INDEX IF EXISTS idx_embeddings_entity;
DROP INDEX IF EXISTS idx_embeddings_vector;
DROP TABLE IF EXISTS embeddings;
-- Note: We don't drop the pgvector extension as other tables might use it
