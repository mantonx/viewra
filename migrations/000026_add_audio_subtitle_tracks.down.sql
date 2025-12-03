-- Remove audio and subtitle track tables

DROP INDEX IF EXISTS idx_subtitle_tracks_language;
DROP INDEX IF EXISTS idx_subtitle_tracks_media_id;
DROP INDEX IF EXISTS idx_audio_tracks_language;
DROP INDEX IF EXISTS idx_audio_tracks_media_id;

DROP TABLE IF EXISTS media_subtitle_tracks;
DROP TABLE IF EXISTS media_audio_tracks;

-- Note: SQLite doesn't support DROP COLUMN directly
-- Library columns (preferred_audio_lang, preferred_subtitle_lang, auto_enable_subtitles)
-- will remain but be unused after downgrade
