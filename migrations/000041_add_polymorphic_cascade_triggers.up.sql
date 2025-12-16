-- Add triggers for polymorphic relationships that can't use foreign keys.
-- These ensure orphaned data is cleaned up when media items are deleted.

-- Clean up any existing orphaned data first
DELETE FROM credits WHERE media_type = 'movie' AND entity_id NOT IN (SELECT id FROM media);
DELETE FROM media_studios WHERE media_type = 'movie' AND entity_id NOT IN (SELECT id FROM media);

-- Trigger to delete credits when a movie is deleted
CREATE TRIGGER IF NOT EXISTS tr_media_delete_credits
AFTER DELETE ON media
FOR EACH ROW
BEGIN
    DELETE FROM credits WHERE media_type = 'movie' AND entity_id = OLD.id;
END;

-- Trigger to delete media_studios when a movie is deleted
CREATE TRIGGER IF NOT EXISTS tr_media_delete_media_studios
AFTER DELETE ON media
FOR EACH ROW
BEGIN
    DELETE FROM media_studios WHERE media_type = 'movie' AND entity_id = OLD.id;
END;

-- Trigger to delete credits when a tv_show is deleted
CREATE TRIGGER IF NOT EXISTS tr_tv_shows_delete_credits
AFTER DELETE ON tv_shows
FOR EACH ROW
BEGIN
    DELETE FROM credits WHERE media_type = 'tv_show' AND entity_id = OLD.id;
END;

-- Trigger to delete media_studios when a tv_show is deleted
CREATE TRIGGER IF NOT EXISTS tr_tv_shows_delete_media_studios
AFTER DELETE ON tv_shows
FOR EACH ROW
BEGIN
    DELETE FROM media_studios WHERE media_type = 'tv_show' AND entity_id = OLD.id;
END;

-- Trigger to delete credits when a tv_episode is deleted
-- Note: tv_episodes uses media_id as its primary key, not id
CREATE TRIGGER IF NOT EXISTS tr_tv_episodes_delete_credits
AFTER DELETE ON tv_episodes
FOR EACH ROW
BEGIN
    DELETE FROM credits WHERE media_type = 'tv_episode' AND entity_id = OLD.media_id;
END;

-- Clean up orphaned people and studios
DELETE FROM people WHERE id NOT IN (SELECT DISTINCT person_id FROM credits);
DELETE FROM studios WHERE id NOT IN (SELECT DISTINCT studio_id FROM media_studios);

-- Trigger to clean up orphaned people when credits are deleted
CREATE TRIGGER IF NOT EXISTS tr_credits_cleanup_orphan_people
AFTER DELETE ON credits
FOR EACH ROW
BEGIN
    DELETE FROM people WHERE id = OLD.person_id
        AND NOT EXISTS (SELECT 1 FROM credits WHERE person_id = OLD.person_id);
END;

-- Trigger to clean up orphaned studios when media_studios are deleted
CREATE TRIGGER IF NOT EXISTS tr_media_studios_cleanup_orphan_studios
AFTER DELETE ON media_studios
FOR EACH ROW
BEGIN
    DELETE FROM studios WHERE id = OLD.studio_id
        AND NOT EXISTS (SELECT 1 FROM media_studios WHERE studio_id = OLD.studio_id);
END;
