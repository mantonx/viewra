-- Remove audio and subtitle track tables

DROP INDEX IF EXISTS idx_subtitle_tracks_language;
DROP INDEX IF EXISTS idx_subtitle_tracks_media_id;
DROP INDEX IF EXISTS idx_audio_tracks_language;
DROP INDEX IF EXISTS idx_audio_tracks_media_id;

DROP TABLE IF EXISTS media_subtitle_tracks;
DROP TABLE IF EXISTS media_audio_tracks;

ALTER TABLE libraries DROP COLUMN IF EXISTS preferred_audio_lang;
ALTER TABLE libraries DROP COLUMN IF EXISTS preferred_subtitle_lang;
ALTER TABLE libraries DROP COLUMN IF EXISTS auto_enable_subtitles;
