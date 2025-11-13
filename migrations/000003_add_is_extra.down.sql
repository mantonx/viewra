-- Remove is_extra field from media table
-- Phase 1.14.1 - Extras Support

DROP INDEX IF EXISTS idx_media_is_extra;

ALTER TABLE media DROP COLUMN is_extra;
