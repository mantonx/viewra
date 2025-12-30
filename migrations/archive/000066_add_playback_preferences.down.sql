-- Remove playback preferences table
DROP INDEX IF EXISTS idx_playback_preferences_device;
DROP INDEX IF EXISTS idx_playback_preferences_user_media;
DROP TABLE IF EXISTS playback_preferences;
