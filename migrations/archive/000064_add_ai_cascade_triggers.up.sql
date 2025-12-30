-- Add cascade triggers for polymorphic tables that were missing them:
-- - embeddings, mood_tags, media_keywords (AI-related)
-- - media_external_ids, media_images (missing triggers for movies)
-- These ensure orphaned data is cleaned up when media items are deleted.

-- =============================================================================
-- CLEANUP EXISTING ORPHANED DATA
-- =============================================================================

-- Embeddings
DELETE FROM embeddings WHERE entity_type = 'movie' 
    AND entity_id NOT IN (SELECT id FROM media WHERE type = 'movie');
DELETE FROM embeddings WHERE entity_type = 'tv_show' 
    AND entity_id NOT IN (SELECT id FROM tv_shows);
DELETE FROM embeddings WHERE entity_type = 'tv_episode' 
    AND entity_id NOT IN (SELECT media_id FROM tv_episodes);
DELETE FROM embeddings WHERE entity_type = 'music_artist' 
    AND entity_id NOT IN (SELECT id FROM music_artists);
DELETE FROM embeddings WHERE entity_type = 'music_album' 
    AND entity_id NOT IN (SELECT id FROM music_albums);
DELETE FROM embeddings WHERE entity_type = 'music_track' 
    AND entity_id NOT IN (SELECT id FROM music_tracks);

-- Mood tags
DELETE FROM mood_tags WHERE entity_type = 'movie' 
    AND entity_id NOT IN (SELECT id FROM media WHERE type = 'movie');
DELETE FROM mood_tags WHERE entity_type = 'tv_show' 
    AND entity_id NOT IN (SELECT id FROM tv_shows);
DELETE FROM mood_tags WHERE entity_type = 'tv_episode' 
    AND entity_id NOT IN (SELECT media_id FROM tv_episodes);
DELETE FROM mood_tags WHERE entity_type = 'music_artist' 
    AND entity_id NOT IN (SELECT id FROM music_artists);
DELETE FROM mood_tags WHERE entity_type = 'music_album' 
    AND entity_id NOT IN (SELECT id FROM music_albums);
DELETE FROM mood_tags WHERE entity_type = 'music_track' 
    AND entity_id NOT IN (SELECT id FROM music_tracks);

-- Media keywords (only movies and tv_shows supported)
DELETE FROM media_keywords WHERE media_type = 'movie' 
    AND entity_id NOT IN (SELECT id FROM media WHERE type = 'movie');
DELETE FROM media_keywords WHERE media_type = 'tv_show' 
    AND entity_id NOT IN (SELECT id FROM tv_shows);

-- Media external IDs (missing trigger for movies)
DELETE FROM media_external_ids WHERE media_type = 'movie' 
    AND entity_id NOT IN (SELECT id FROM media WHERE type = 'movie');

-- Media images (missing trigger for movies)
DELETE FROM media_images WHERE media_type = 'movie' 
    AND entity_id NOT IN (SELECT id FROM media WHERE type = 'movie');

-- Enrichment status
DELETE FROM enrichment_status WHERE media_type = 'movie' 
    AND media_id NOT IN (SELECT id FROM media WHERE type = 'movie');
DELETE FROM enrichment_status WHERE media_type = 'tv_show' 
    AND media_id NOT IN (SELECT id FROM tv_shows);
DELETE FROM enrichment_status WHERE media_type = 'tv_season' 
    AND media_id NOT IN (SELECT id FROM tv_seasons);

-- =============================================================================
-- TRIGGERS FOR MEDIA TABLE (movies)
-- =============================================================================

CREATE TRIGGER IF NOT EXISTS tr_media_delete_embeddings
AFTER DELETE ON media
FOR EACH ROW
BEGIN
    DELETE FROM embeddings WHERE entity_type = 'movie' AND entity_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_media_delete_mood_tags
AFTER DELETE ON media
FOR EACH ROW
BEGIN
    DELETE FROM mood_tags WHERE entity_type = 'movie' AND entity_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_media_delete_media_keywords
AFTER DELETE ON media
FOR EACH ROW
BEGIN
    DELETE FROM media_keywords WHERE media_type = 'movie' AND entity_id = OLD.id;
END;

-- These were missing from earlier migrations
CREATE TRIGGER IF NOT EXISTS tr_media_delete_external_ids
AFTER DELETE ON media
FOR EACH ROW
BEGIN
    DELETE FROM media_external_ids WHERE media_type = 'movie' AND entity_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_media_delete_images
AFTER DELETE ON media
FOR EACH ROW
BEGIN
    DELETE FROM media_images WHERE media_type = 'movie' AND entity_id = OLD.id;
END;

-- =============================================================================
-- TRIGGERS FOR TV_SHOWS TABLE
-- =============================================================================

CREATE TRIGGER IF NOT EXISTS tr_tv_shows_delete_embeddings
AFTER DELETE ON tv_shows
FOR EACH ROW
BEGIN
    DELETE FROM embeddings WHERE entity_type = 'tv_show' AND entity_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_tv_shows_delete_mood_tags
AFTER DELETE ON tv_shows
FOR EACH ROW
BEGIN
    DELETE FROM mood_tags WHERE entity_type = 'tv_show' AND entity_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_tv_shows_delete_media_keywords
AFTER DELETE ON tv_shows
FOR EACH ROW
BEGIN
    DELETE FROM media_keywords WHERE media_type = 'tv_show' AND entity_id = OLD.id;
END;

-- =============================================================================
-- TRIGGERS FOR TV_SEASONS TABLE
-- =============================================================================

CREATE TRIGGER IF NOT EXISTS tr_tv_seasons_delete_embeddings
AFTER DELETE ON tv_seasons
FOR EACH ROW
BEGIN
    DELETE FROM embeddings WHERE entity_type = 'tv_season' AND entity_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_tv_seasons_delete_mood_tags
AFTER DELETE ON tv_seasons
FOR EACH ROW
BEGIN
    DELETE FROM mood_tags WHERE entity_type = 'tv_season' AND entity_id = OLD.id;
END;

-- =============================================================================
-- TRIGGERS FOR TV_EPISODES TABLE
-- Note: tv_episodes uses media_id as its primary key, not id
-- =============================================================================

CREATE TRIGGER IF NOT EXISTS tr_tv_episodes_delete_embeddings
AFTER DELETE ON tv_episodes
FOR EACH ROW
BEGIN
    DELETE FROM embeddings WHERE entity_type = 'tv_episode' AND entity_id = OLD.media_id;
END;

CREATE TRIGGER IF NOT EXISTS tr_tv_episodes_delete_mood_tags
AFTER DELETE ON tv_episodes
FOR EACH ROW
BEGIN
    DELETE FROM mood_tags WHERE entity_type = 'tv_episode' AND entity_id = OLD.media_id;
END;

-- =============================================================================
-- TRIGGERS FOR MUSIC TABLES
-- =============================================================================

-- Music artists
CREATE TRIGGER IF NOT EXISTS tr_music_artists_delete_embeddings
AFTER DELETE ON music_artists
FOR EACH ROW
BEGIN
    DELETE FROM embeddings WHERE entity_type = 'music_artist' AND entity_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_music_artists_delete_mood_tags
AFTER DELETE ON music_artists
FOR EACH ROW
BEGIN
    DELETE FROM mood_tags WHERE entity_type = 'music_artist' AND entity_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_music_artists_delete_external_ids
AFTER DELETE ON music_artists
FOR EACH ROW
BEGIN
    DELETE FROM media_external_ids WHERE media_type = 'music_artist' AND entity_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_music_artists_delete_images
AFTER DELETE ON music_artists
FOR EACH ROW
BEGIN
    DELETE FROM media_images WHERE media_type = 'music_artist' AND entity_id = OLD.id;
END;

-- Music albums
CREATE TRIGGER IF NOT EXISTS tr_music_albums_delete_embeddings
AFTER DELETE ON music_albums
FOR EACH ROW
BEGIN
    DELETE FROM embeddings WHERE entity_type = 'music_album' AND entity_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_music_albums_delete_mood_tags
AFTER DELETE ON music_albums
FOR EACH ROW
BEGIN
    DELETE FROM mood_tags WHERE entity_type = 'music_album' AND entity_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_music_albums_delete_external_ids
AFTER DELETE ON music_albums
FOR EACH ROW
BEGIN
    DELETE FROM media_external_ids WHERE media_type = 'music_album' AND entity_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_music_albums_delete_images
AFTER DELETE ON music_albums
FOR EACH ROW
BEGIN
    DELETE FROM media_images WHERE media_type = 'music_album' AND entity_id = OLD.id;
END;

-- Music tracks
CREATE TRIGGER IF NOT EXISTS tr_music_tracks_delete_embeddings
AFTER DELETE ON music_tracks
FOR EACH ROW
BEGIN
    DELETE FROM embeddings WHERE entity_type = 'music_track' AND entity_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_music_tracks_delete_mood_tags
AFTER DELETE ON music_tracks
FOR EACH ROW
BEGIN
    DELETE FROM mood_tags WHERE entity_type = 'music_track' AND entity_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_music_tracks_delete_external_ids
AFTER DELETE ON music_tracks
FOR EACH ROW
BEGIN
    DELETE FROM media_external_ids WHERE media_type = 'music_track' AND entity_id = OLD.id;
END;
