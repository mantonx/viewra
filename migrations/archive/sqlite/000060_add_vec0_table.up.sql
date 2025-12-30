-- Add vec0 virtual table for fast KNN vector search (SQLite only)
-- PostgreSQL uses HNSW index on the existing embeddings table
--
-- The vec_embeddings table mirrors the embeddings table but uses sqlite-vec's
-- vec0 virtual table format for efficient approximate nearest neighbor search.
-- Triggers keep the two tables in sync automatically.

-- Create vec0 virtual table
CREATE VIRTUAL TABLE IF NOT EXISTS vec_embeddings USING vec0(
    embedding_id INTEGER PRIMARY KEY,
    entity_type TEXT,
    contents_embedding float[768] distance_metric=cosine
);

-- Populate from existing embeddings
INSERT INTO vec_embeddings(embedding_id, entity_type, contents_embedding)
SELECT id, entity_type, vector FROM embeddings;

-- Trigger: auto-insert into vec_embeddings when embedding is added
CREATE TRIGGER IF NOT EXISTS vec_embeddings_insert 
AFTER INSERT ON embeddings BEGIN
    INSERT INTO vec_embeddings(embedding_id, entity_type, contents_embedding)
    VALUES (NEW.id, NEW.entity_type, NEW.vector);
END;

-- Trigger: auto-update vec_embeddings when embedding is updated
-- vec0 doesn't support UPDATE, so we delete and re-insert
CREATE TRIGGER IF NOT EXISTS vec_embeddings_update 
AFTER UPDATE ON embeddings BEGIN
    DELETE FROM vec_embeddings WHERE embedding_id = OLD.id;
    INSERT INTO vec_embeddings(embedding_id, entity_type, contents_embedding)
    VALUES (NEW.id, NEW.entity_type, NEW.vector);
END;

-- Trigger: auto-delete from vec_embeddings when embedding is deleted
CREATE TRIGGER IF NOT EXISTS vec_embeddings_delete 
AFTER DELETE ON embeddings BEGIN
    DELETE FROM vec_embeddings WHERE embedding_id = OLD.id;
END;
