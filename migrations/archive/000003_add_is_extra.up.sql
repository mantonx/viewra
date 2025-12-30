-- Add is_extra field to distinguish main features from extras (trailers, deleted scenes, etc.)
-- Phase 1.14.1 - Extras Support

ALTER TABLE media ADD COLUMN is_extra BOOLEAN DEFAULT 0 NOT NULL;

-- Add index for efficient filtering of main features vs extras
CREATE INDEX idx_media_is_extra ON media(is_extra);
