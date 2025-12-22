-- Remove cascade triggers for polymorphic tables

-- Media (movies)
DROP TRIGGER IF EXISTS tr_media_delete_embeddings ON media;
DROP TRIGGER IF EXISTS tr_media_delete_mood_tags ON media;
DROP TRIGGER IF EXISTS tr_media_delete_media_keywords ON media;
DROP TRIGGER IF EXISTS tr_media_delete_external_ids ON media;
DROP TRIGGER IF EXISTS tr_media_delete_images ON media;

-- TV Shows
DROP TRIGGER IF EXISTS tr_tv_shows_delete_embeddings ON tv_shows;
DROP TRIGGER IF EXISTS tr_tv_shows_delete_mood_tags ON tv_shows;
DROP TRIGGER IF EXISTS tr_tv_shows_delete_media_keywords ON tv_shows;

-- TV Seasons
DROP TRIGGER IF EXISTS tr_tv_seasons_delete_embeddings ON tv_seasons;
DROP TRIGGER IF EXISTS tr_tv_seasons_delete_mood_tags ON tv_seasons;

-- TV Episodes
DROP TRIGGER IF EXISTS tr_tv_episodes_delete_embeddings ON tv_episodes;
DROP TRIGGER IF EXISTS tr_tv_episodes_delete_mood_tags ON tv_episodes;

-- Music Artists
DROP TRIGGER IF EXISTS tr_music_artists_delete_embeddings ON music_artists;
DROP TRIGGER IF EXISTS tr_music_artists_delete_mood_tags ON music_artists;
DROP TRIGGER IF EXISTS tr_music_artists_delete_external_ids ON music_artists;
DROP TRIGGER IF EXISTS tr_music_artists_delete_images ON music_artists;

-- Music Albums
DROP TRIGGER IF EXISTS tr_music_albums_delete_embeddings ON music_albums;
DROP TRIGGER IF EXISTS tr_music_albums_delete_mood_tags ON music_albums;
DROP TRIGGER IF EXISTS tr_music_albums_delete_external_ids ON music_albums;
DROP TRIGGER IF EXISTS tr_music_albums_delete_images ON music_albums;

-- Music Tracks
DROP TRIGGER IF EXISTS tr_music_tracks_delete_embeddings ON music_tracks;
DROP TRIGGER IF EXISTS tr_music_tracks_delete_mood_tags ON music_tracks;
DROP TRIGGER IF EXISTS tr_music_tracks_delete_external_ids ON music_tracks;

-- Drop functions
DROP FUNCTION IF EXISTS fn_media_cleanup_embeddings();
DROP FUNCTION IF EXISTS fn_media_cleanup_mood_tags();
DROP FUNCTION IF EXISTS fn_media_cleanup_media_keywords();
DROP FUNCTION IF EXISTS fn_media_cleanup_external_ids();
DROP FUNCTION IF EXISTS fn_media_cleanup_images();
DROP FUNCTION IF EXISTS fn_tv_shows_cleanup_embeddings();
DROP FUNCTION IF EXISTS fn_tv_shows_cleanup_mood_tags();
DROP FUNCTION IF EXISTS fn_tv_shows_cleanup_media_keywords();
DROP FUNCTION IF EXISTS fn_tv_seasons_cleanup_embeddings();
DROP FUNCTION IF EXISTS fn_tv_seasons_cleanup_mood_tags();
DROP FUNCTION IF EXISTS fn_tv_episodes_cleanup_embeddings();
DROP FUNCTION IF EXISTS fn_tv_episodes_cleanup_mood_tags();
DROP FUNCTION IF EXISTS fn_music_artists_cleanup_embeddings();
DROP FUNCTION IF EXISTS fn_music_artists_cleanup_mood_tags();
DROP FUNCTION IF EXISTS fn_music_artists_cleanup_external_ids();
DROP FUNCTION IF EXISTS fn_music_artists_cleanup_images();
DROP FUNCTION IF EXISTS fn_music_albums_cleanup_embeddings();
DROP FUNCTION IF EXISTS fn_music_albums_cleanup_mood_tags();
DROP FUNCTION IF EXISTS fn_music_albums_cleanup_external_ids();
DROP FUNCTION IF EXISTS fn_music_albums_cleanup_images();
DROP FUNCTION IF EXISTS fn_music_tracks_cleanup_embeddings();
DROP FUNCTION IF EXISTS fn_music_tracks_cleanup_mood_tags();
DROP FUNCTION IF EXISTS fn_music_tracks_cleanup_external_ids();
