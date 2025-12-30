-- Add playback preference columns to watch_progress
-- These store user's last selected quality, audio track, and subtitle track per video
-- NULL means no preference saved (use defaults)

ALTER TABLE watch_progress ADD COLUMN selected_quality TEXT;
ALTER TABLE watch_progress ADD COLUMN selected_audio_track INTEGER;
ALTER TABLE watch_progress ADD COLUMN selected_subtitle_track INTEGER;
