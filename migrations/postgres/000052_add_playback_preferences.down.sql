-- PostgreSQL supports DROP COLUMN directly
ALTER TABLE watch_progress DROP COLUMN IF EXISTS selected_quality;
ALTER TABLE watch_progress DROP COLUMN IF EXISTS selected_audio_track;
ALTER TABLE watch_progress DROP COLUMN IF EXISTS selected_subtitle_track;
