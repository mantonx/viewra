-- Remove vec0 virtual table and triggers

DROP TRIGGER IF EXISTS vec_embeddings_delete;
DROP TRIGGER IF EXISTS vec_embeddings_update;
DROP TRIGGER IF EXISTS vec_embeddings_insert;
DROP TABLE IF EXISTS vec_embeddings;
