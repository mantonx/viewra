-- Add triggers for media_external_ids and media_images polymorphic cascade deletes.
-- These tables use polymorphic relationships (media_type + entity_id) that can't use FKs.

-- Clean up any existing orphaned data
DELETE FROM media_external_ids WHERE media_type = 'tv_show' AND entity_id NOT IN (SELECT id FROM tv_shows);
DELETE FROM media_external_ids WHERE media_type = 'tv_season' AND entity_id NOT IN (SELECT id FROM tv_seasons);
DELETE FROM media_external_ids WHERE media_type = 'tv_episode' AND entity_id NOT IN (SELECT media_id FROM tv_episodes);
DELETE FROM media_external_ids WHERE media_type = 'person' AND entity_id NOT IN (SELECT id FROM people);
DELETE FROM media_external_ids WHERE media_type = 'studio' AND entity_id NOT IN (SELECT id FROM studios);

DELETE FROM media_images WHERE media_type = 'tv_show' AND entity_id NOT IN (SELECT id FROM tv_shows);
DELETE FROM media_images WHERE media_type = 'tv_season' AND entity_id NOT IN (SELECT id FROM tv_seasons);
DELETE FROM media_images WHERE media_type = 'tv_episode' AND entity_id NOT IN (SELECT media_id FROM tv_episodes);

-- Triggers for media_external_ids cleanup

CREATE TRIGGER IF NOT EXISTS tr_tv_shows_delete_external_ids
AFTER DELETE ON tv_shows
FOR EACH ROW
BEGIN
    DELETE FROM media_external_ids WHERE media_type = 'tv_show' AND entity_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_tv_seasons_delete_external_ids
AFTER DELETE ON tv_seasons
FOR EACH ROW
BEGIN
    DELETE FROM media_external_ids WHERE media_type = 'tv_season' AND entity_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_tv_episodes_delete_external_ids
AFTER DELETE ON tv_episodes
FOR EACH ROW
BEGIN
    DELETE FROM media_external_ids WHERE media_type = 'tv_episode' AND entity_id = OLD.media_id;
END;

CREATE TRIGGER IF NOT EXISTS tr_people_delete_external_ids
AFTER DELETE ON people
FOR EACH ROW
BEGIN
    DELETE FROM media_external_ids WHERE media_type = 'person' AND entity_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_studios_delete_external_ids
AFTER DELETE ON studios
FOR EACH ROW
BEGIN
    DELETE FROM media_external_ids WHERE media_type = 'studio' AND entity_id = OLD.id;
END;

-- Triggers for media_images cleanup

CREATE TRIGGER IF NOT EXISTS tr_tv_shows_delete_images
AFTER DELETE ON tv_shows
FOR EACH ROW
BEGIN
    DELETE FROM media_images WHERE media_type = 'tv_show' AND entity_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_tv_seasons_delete_images
AFTER DELETE ON tv_seasons
FOR EACH ROW
BEGIN
    DELETE FROM media_images WHERE media_type = 'tv_season' AND entity_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS tr_tv_episodes_delete_images
AFTER DELETE ON tv_episodes
FOR EACH ROW
BEGIN
    DELETE FROM media_images WHERE media_type = 'tv_episode' AND entity_id = OLD.media_id;
END;
