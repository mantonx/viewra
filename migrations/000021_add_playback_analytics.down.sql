-- Drop playback analytics tables

DROP INDEX IF EXISTS idx_quality_switch_events_timestamp;
DROP INDEX IF EXISTS idx_quality_switch_events_media_id;
DROP INDEX IF EXISTS idx_quality_switch_events_session_id;
DROP INDEX IF EXISTS idx_playback_sessions_start_time;
DROP INDEX IF EXISTS idx_playback_sessions_media_id;

DROP TABLE IF EXISTS quality_switch_events;
DROP TABLE IF EXISTS playback_sessions;
