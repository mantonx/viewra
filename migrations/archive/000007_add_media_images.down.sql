-- Migration Rollback: Remove media_images table
-- Created: 2025-11-15

-- Drop indexes first
DROP INDEX IF EXISTS idx_media_images_entity_unique;
DROP INDEX IF EXISTS idx_media_images_unique;
DROP INDEX IF EXISTS idx_media_images_hash;
DROP INDEX IF EXISTS idx_media_images_source;
DROP INDEX IF EXISTS idx_media_images_type;
DROP INDEX IF EXISTS idx_media_images_entity;
DROP INDEX IF EXISTS idx_media_images_media_id;

-- Drop table
DROP TABLE IF EXISTS media_images;
