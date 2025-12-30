-- Drop old AI tables that are now managed by the semantic-search plugin
-- The plugin creates its own namespaced tables: plugin_semantic_search_*

-- Note: We cannot drop vec_embeddings (vec0 virtual table) here because
-- golang-migrate uses its own sqlite3 driver without sqlite-vec extension.
-- The vec_embeddings table will remain orphaned but harmless.
-- If cleanup is needed, use the app with sqlite-vec loaded.

-- Drop triggers that reference the old tables
DROP TRIGGER IF EXISTS vec_embeddings_delete;
DROP TRIGGER IF EXISTS vec_embeddings_update;
DROP TRIGGER IF EXISTS vec_embeddings_insert;

-- Drop old embeddings table
DROP TABLE IF EXISTS embeddings;

-- Drop old mood_tags table (now managed by semantic-search plugin)
DROP TABLE IF EXISTS mood_tags;
