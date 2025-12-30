-- Drop old AI tables that are now managed by the semantic-search plugin
-- The plugin creates its own namespaced tables: plugin_semantic_search_*

-- Drop old embeddings table (with pgvector index)
DROP INDEX IF EXISTS idx_embeddings_vector;
DROP INDEX IF EXISTS idx_embeddings_type;
DROP INDEX IF EXISTS idx_embeddings_entity;
DROP TABLE IF EXISTS embeddings;

-- Drop old mood_tags table (now managed by semantic-search plugin)
DROP INDEX IF EXISTS idx_mood_tags_tag;
DROP INDEX IF EXISTS idx_mood_tags_entity;
DROP TABLE IF EXISTS mood_tags;
