-- Drop all tables in reverse dependency order

DROP TABLE IF EXISTS scheduler_locks;
DROP TABLE IF EXISTS scheduler_executions;
DROP TABLE IF EXISTS scheduled_tasks;

DROP TABLE IF EXISTS plugin_user_metadata;
DROP TABLE IF EXISTS plugin_kv;
DROP TABLE IF EXISTS plugin_api_keys;
DROP TABLE IF EXISTS plugins;

DROP TABLE IF EXISTS media_metadata_sources;
DROP TABLE IF EXISTS media_external_ids;
DROP TABLE IF EXISTS enrichment_pipelines;
DROP TABLE IF EXISTS enrichment_status;
DROP TABLE IF EXISTS enrichment_queue;

DROP TABLE IF EXISTS playback_quality_events;
DROP TABLE IF EXISTS quality_switch_events;
DROP TABLE IF EXISTS playback_sessions;
DROP TABLE IF EXISTS user_video_preferences;
DROP TABLE IF EXISTS transcode_analytics;
DROP TABLE IF EXISTS transcode_jobs;

DROP TABLE IF EXISTS scan_state;
DROP TABLE IF EXISTS scan_checkpoints;
DROP TABLE IF EXISTS scan_jobs;

DROP TABLE IF EXISTS playback_preferences;
DROP TABLE IF EXISTS watch_progress;

DROP TABLE IF EXISTS user_settings;
DROP TABLE IF EXISTS system_settings;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;

DROP TABLE IF EXISTS media_keywords;
DROP TABLE IF EXISTS media_subtitle_tracks;
DROP TABLE IF EXISTS media_audio_tracks;
DROP TABLE IF EXISTS media_images;
DROP TABLE IF EXISTS media_studios;
DROP TABLE IF EXISTS studios;
DROP TABLE IF EXISTS credits;
DROP TABLE IF EXISTS people;

DROP TABLE IF EXISTS music_tracks;
DROP TABLE IF EXISTS music_albums;
DROP TABLE IF EXISTS music_artists;

DROP TABLE IF EXISTS tv_episodes;
DROP TABLE IF EXISTS tv_seasons;
DROP TABLE IF EXISTS tv_shows;

DROP TABLE IF EXISTS movies;
DROP TABLE IF EXISTS media;
DROP TABLE IF EXISTS libraries;
