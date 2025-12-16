-- Remove the unique constraint on tv_shows
DROP INDEX IF EXISTS idx_tv_shows_library_title_unique;
