-- Drop all tables and functions in reverse dependency order

-- Drop triggers first
DROP TRIGGER IF EXISTS tr_studios_delete_external_ids ON studios;
DROP TRIGGER IF EXISTS tr_people_delete_external_ids ON people;
DROP TRIGGER IF EXISTS tr_music_tracks_delete_images ON music_tracks;
DROP TRIGGER IF EXISTS tr_music_albums_delete_images ON music_albums;
DROP TRIGGER IF EXISTS tr_music_artists_delete_images ON music_artists;
DROP TRIGGER IF EXISTS tr_tv_episodes_delete_images ON tv_episodes;
DROP TRIGGER IF EXISTS tr_tv_seasons_delete_images ON tv_seasons;
DROP TRIGGER IF EXISTS tr_tv_shows_delete_images ON tv_shows;
DROP TRIGGER IF EXISTS tr_movies_delete_images ON movies;
DROP TRIGGER IF EXISTS tr_tv_episodes_delete_credits ON tv_episodes;
DROP TRIGGER IF EXISTS tr_tv_shows_delete_credits ON tv_shows;
DROP TRIGGER IF EXISTS tr_movies_delete_credits ON movies;
DROP TRIGGER IF EXISTS tr_tv_shows_delete_mood_tags ON tv_shows;
DROP TRIGGER IF EXISTS tr_movies_delete_mood_tags ON movies;

-- Drop functions
DROP FUNCTION IF EXISTS delete_external_ids_on_studio_delete();
DROP FUNCTION IF EXISTS delete_external_ids_on_person_delete();
DROP FUNCTION IF EXISTS delete_images_on_music_track_delete();
DROP FUNCTION IF EXISTS delete_images_on_music_album_delete();
DROP FUNCTION IF EXISTS delete_images_on_music_artist_delete();
DROP FUNCTION IF EXISTS delete_images_on_tv_episode_delete();
DROP FUNCTION IF EXISTS delete_images_on_tv_season_delete();
DROP FUNCTION IF EXISTS delete_images_on_tv_show_delete();
DROP FUNCTION IF EXISTS delete_images_on_movie_delete();
DROP FUNCTION IF EXISTS delete_credits_on_tv_episode_delete();
DROP FUNCTION IF EXISTS delete_credits_on_tv_show_delete();
DROP FUNCTION IF EXISTS delete_credits_on_movie_delete();
DROP FUNCTION IF EXISTS delete_mood_tags_on_tv_show_delete();
DROP FUNCTION IF EXISTS delete_mood_tags_on_movie_delete();

-- Drop tables
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

DROP TABLE IF EXISTS mood_tags;
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

-- Drop embeddings table and extension if they exist
-- DROP TABLE IF EXISTS embeddings;
-- DROP EXTENSION IF EXISTS vector;
