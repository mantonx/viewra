-- Add unique constraint on tv_shows to prevent duplicate shows within a library
-- This enables the use of INSERT...ON CONFLICT for atomic upserts

-- Create a unique index on library_id and lowercase title
-- Using LOWER(title) ensures case-insensitive uniqueness
CREATE UNIQUE INDEX IF NOT EXISTS idx_tv_shows_library_title_unique
ON tv_shows(library_id, LOWER(title));
