-- Remove cascade triggers for polymorphic tables

-- Media (movies)
DROP TRIGGER IF EXISTS tr_media_delete_embeddings;
DROP TRIGGER IF EXISTS tr_media_delete_mood_tags;
DROP TRIGGER IF EXISTS tr_media_delete_media_keywords;
DROP TRIGGER IF EXISTS tr_media_delete_external_ids;
DROP TRIGGER IF EXISTS tr_media_delete_images;

-- TV Shows
DROP TRIGGER IF EXISTS tr_tv_shows_delete_embeddings;
DROP TRIGGER IF EXISTS tr_tv_shows_delete_mood_tags;
DROP TRIGGER IF EXISTS tr_tv_shows_delete_media_keywords;

-- TV Seasons
DROP TRIGGER IF EXISTS tr_tv_seasons_delete_embeddings;
DROP TRIGGER IF EXISTS tr_tv_seasons_delete_mood_tags;

-- TV Episodes
DROP TRIGGER IF EXISTS tr_tv_episodes_delete_embeddings;
DROP TRIGGER IF EXISTS tr_tv_episodes_delete_mood_tags;

-- Music Artists
DROP TRIGGER IF EXISTS tr_music_artists_delete_embeddings;
DROP TRIGGER IF EXISTS tr_music_artists_delete_mood_tags;
DROP TRIGGER IF EXISTS tr_music_artists_delete_external_ids;
DROP TRIGGER IF EXISTS tr_music_artists_delete_images;

-- Music Albums
DROP TRIGGER IF EXISTS tr_music_albums_delete_embeddings;
DROP TRIGGER IF EXISTS tr_music_albums_delete_mood_tags;
DROP TRIGGER IF EXISTS tr_music_albums_delete_external_ids;
DROP TRIGGER IF EXISTS tr_music_albums_delete_images;

-- Music Tracks
DROP TRIGGER IF EXISTS tr_music_tracks_delete_embeddings;
DROP TRIGGER IF EXISTS tr_music_tracks_delete_mood_tags;
DROP TRIGGER IF EXISTS tr_music_tracks_delete_external_ids;
