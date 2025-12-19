-- Add startup_time_ms to playback_sessions table
-- Tracks time from play request to first frame rendered (in milliseconds)
ALTER TABLE playback_sessions ADD COLUMN startup_time_ms INTEGER;
