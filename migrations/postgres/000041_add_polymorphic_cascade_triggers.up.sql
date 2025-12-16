-- Add triggers for polymorphic relationships that can't use foreign keys.
-- These ensure orphaned data is cleaned up when media items are deleted.

-- Clean up any existing orphaned data first
DELETE FROM credits WHERE media_type = 'movie' AND entity_id NOT IN (SELECT id FROM media);
DELETE FROM media_studios WHERE media_type = 'movie' AND entity_id NOT IN (SELECT id FROM media);

-- Function to clean up credits when media (movie) is deleted
CREATE OR REPLACE FUNCTION fn_media_cleanup_credits()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM credits WHERE media_type = 'movie' AND entity_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

-- Function to clean up media_studios when media is deleted
CREATE OR REPLACE FUNCTION fn_media_cleanup_media_studios()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM media_studios WHERE media_type = 'movie' AND entity_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

-- Function to clean up credits when tv_show is deleted
CREATE OR REPLACE FUNCTION fn_tv_shows_cleanup_credits()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM credits WHERE media_type = 'tv_show' AND entity_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

-- Function to clean up media_studios when tv_show is deleted
CREATE OR REPLACE FUNCTION fn_tv_shows_cleanup_media_studios()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM media_studios WHERE media_type = 'tv_show' AND entity_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

-- Create triggers on media table
CREATE TRIGGER tr_media_delete_credits
AFTER DELETE ON media
FOR EACH ROW
EXECUTE FUNCTION fn_media_cleanup_credits();

CREATE TRIGGER tr_media_delete_media_studios
AFTER DELETE ON media
FOR EACH ROW
EXECUTE FUNCTION fn_media_cleanup_media_studios();

-- Create triggers on tv_shows table
CREATE TRIGGER tr_tv_shows_delete_credits
AFTER DELETE ON tv_shows
FOR EACH ROW
EXECUTE FUNCTION fn_tv_shows_cleanup_credits();

CREATE TRIGGER tr_tv_shows_delete_media_studios
AFTER DELETE ON tv_shows
FOR EACH ROW
EXECUTE FUNCTION fn_tv_shows_cleanup_media_studios();

-- Function to clean up credits when tv_episode is deleted
-- Note: tv_episodes uses media_id as its primary key, not id
CREATE OR REPLACE FUNCTION fn_tv_episodes_cleanup_credits()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM credits WHERE media_type = 'tv_episode' AND entity_id = OLD.media_id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

-- Create trigger on tv_episodes table
CREATE TRIGGER tr_tv_episodes_delete_credits
AFTER DELETE ON tv_episodes
FOR EACH ROW
EXECUTE FUNCTION fn_tv_episodes_cleanup_credits();

-- Clean up orphaned people and studios
DELETE FROM people WHERE id NOT IN (SELECT DISTINCT person_id FROM credits);
DELETE FROM studios WHERE id NOT IN (SELECT DISTINCT studio_id FROM media_studios);

-- Function to clean up orphaned people when credits are deleted
CREATE OR REPLACE FUNCTION fn_credits_cleanup_orphan_people()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM people WHERE id = OLD.person_id
        AND NOT EXISTS (SELECT 1 FROM credits WHERE person_id = OLD.person_id);
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

-- Function to clean up orphaned studios when media_studios are deleted
CREATE OR REPLACE FUNCTION fn_media_studios_cleanup_orphan_studios()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM studios WHERE id = OLD.studio_id
        AND NOT EXISTS (SELECT 1 FROM media_studios WHERE studio_id = OLD.studio_id);
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

-- Create triggers for orphan cleanup
CREATE TRIGGER tr_credits_cleanup_orphan_people
AFTER DELETE ON credits
FOR EACH ROW
EXECUTE FUNCTION fn_credits_cleanup_orphan_people();

CREATE TRIGGER tr_media_studios_cleanup_orphan_studios
AFTER DELETE ON media_studios
FOR EACH ROW
EXECUTE FUNCTION fn_media_studios_cleanup_orphan_studios();
