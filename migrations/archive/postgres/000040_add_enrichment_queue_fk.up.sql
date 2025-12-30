-- Add triggers for enrichment_queue cascade deletes when entities are removed.
-- enrichment_queue is polymorphic (media_type + media_id), so it can't use FKs.
-- Instead, we use triggers to clean up orphaned records.

-- First, clean up any orphaned records
DELETE FROM enrichment_queue WHERE media_type IN ('movie', 'tv') AND media_id NOT IN (SELECT id FROM media);
DELETE FROM enrichment_queue WHERE media_type = 'tv_show' AND media_id NOT IN (SELECT id FROM tv_shows);
DELETE FROM enrichment_queue WHERE media_type = 'tv_season' AND media_id NOT IN (SELECT id FROM tv_seasons);

-- Function to clean up enrichment_queue when media (movie or tv episode) is deleted
CREATE OR REPLACE FUNCTION fn_media_cleanup_enrichment_queue()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM enrichment_queue WHERE media_type IN ('movie', 'tv') AND media_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

-- Function to clean up enrichment_queue when tv_show is deleted
CREATE OR REPLACE FUNCTION fn_tv_shows_cleanup_enrichment_queue()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM enrichment_queue WHERE media_type = 'tv_show' AND media_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

-- Function to clean up enrichment_queue when tv_season is deleted
CREATE OR REPLACE FUNCTION fn_tv_seasons_cleanup_enrichment_queue()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM enrichment_queue WHERE media_type = 'tv_season' AND media_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

-- Create triggers
CREATE TRIGGER tr_media_delete_enrichment_queue
AFTER DELETE ON media
FOR EACH ROW
EXECUTE FUNCTION fn_media_cleanup_enrichment_queue();

CREATE TRIGGER tr_tv_shows_delete_enrichment_queue
AFTER DELETE ON tv_shows
FOR EACH ROW
EXECUTE FUNCTION fn_tv_shows_cleanup_enrichment_queue();

CREATE TRIGGER tr_tv_seasons_delete_enrichment_queue
AFTER DELETE ON tv_seasons
FOR EACH ROW
EXECUTE FUNCTION fn_tv_seasons_cleanup_enrichment_queue();
