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

-- Functions for media_external_ids cleanup

CREATE OR REPLACE FUNCTION fn_tv_shows_cleanup_external_ids()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM media_external_ids WHERE media_type = 'tv_show' AND entity_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION fn_tv_seasons_cleanup_external_ids()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM media_external_ids WHERE media_type = 'tv_season' AND entity_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION fn_tv_episodes_cleanup_external_ids()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM media_external_ids WHERE media_type = 'tv_episode' AND entity_id = OLD.media_id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION fn_people_cleanup_external_ids()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM media_external_ids WHERE media_type = 'person' AND entity_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION fn_studios_cleanup_external_ids()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM media_external_ids WHERE media_type = 'studio' AND entity_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

-- Triggers for media_external_ids

CREATE TRIGGER tr_tv_shows_delete_external_ids
AFTER DELETE ON tv_shows
FOR EACH ROW
EXECUTE FUNCTION fn_tv_shows_cleanup_external_ids();

CREATE TRIGGER tr_tv_seasons_delete_external_ids
AFTER DELETE ON tv_seasons
FOR EACH ROW
EXECUTE FUNCTION fn_tv_seasons_cleanup_external_ids();

CREATE TRIGGER tr_tv_episodes_delete_external_ids
AFTER DELETE ON tv_episodes
FOR EACH ROW
EXECUTE FUNCTION fn_tv_episodes_cleanup_external_ids();

CREATE TRIGGER tr_people_delete_external_ids
AFTER DELETE ON people
FOR EACH ROW
EXECUTE FUNCTION fn_people_cleanup_external_ids();

CREATE TRIGGER tr_studios_delete_external_ids
AFTER DELETE ON studios
FOR EACH ROW
EXECUTE FUNCTION fn_studios_cleanup_external_ids();

-- Functions for media_images cleanup

CREATE OR REPLACE FUNCTION fn_tv_shows_cleanup_images()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM media_images WHERE media_type = 'tv_show' AND entity_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION fn_tv_seasons_cleanup_images()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM media_images WHERE media_type = 'tv_season' AND entity_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION fn_tv_episodes_cleanup_images()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM media_images WHERE media_type = 'tv_episode' AND entity_id = OLD.media_id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

-- Triggers for media_images

CREATE TRIGGER tr_tv_shows_delete_images
AFTER DELETE ON tv_shows
FOR EACH ROW
EXECUTE FUNCTION fn_tv_shows_cleanup_images();

CREATE TRIGGER tr_tv_seasons_delete_images
AFTER DELETE ON tv_seasons
FOR EACH ROW
EXECUTE FUNCTION fn_tv_seasons_cleanup_images();

CREATE TRIGGER tr_tv_episodes_delete_images
AFTER DELETE ON tv_episodes
FOR EACH ROW
EXECUTE FUNCTION fn_tv_episodes_cleanup_images();
