-- Add triggers for enrichment_queue cascade deletes when entities are removed.
-- enrichment_queue is polymorphic (media_type + media_id), so it can't use FKs.
-- Instead, we use triggers to clean up orphaned records.

-- First, clean up any orphaned records
DELETE FROM enrichment_queue WHERE media_type IN ('movie', 'tv') AND media_id NOT IN (SELECT id FROM media);
DELETE FROM enrichment_queue WHERE media_type = 'tv_show' AND media_id NOT IN (SELECT id FROM tv_shows);
DELETE FROM enrichment_queue WHERE media_type = 'tv_season' AND media_id NOT IN (SELECT id FROM tv_seasons);

-- Trigger for media deletes (handles both 'movie' and 'tv' media types)
CREATE TRIGGER IF NOT EXISTS tr_media_delete_enrichment_queue
AFTER DELETE ON media
FOR EACH ROW
BEGIN
    DELETE FROM enrichment_queue WHERE media_type IN ('movie', 'tv') AND media_id = OLD.id;
END;

-- Trigger for tv_show deletes
CREATE TRIGGER IF NOT EXISTS tr_tv_shows_delete_enrichment_queue
AFTER DELETE ON tv_shows
FOR EACH ROW
BEGIN
    DELETE FROM enrichment_queue WHERE media_type = 'tv_show' AND media_id = OLD.id;
END;

-- Trigger for tv_season deletes
CREATE TRIGGER IF NOT EXISTS tr_tv_seasons_delete_enrichment_queue
AFTER DELETE ON tv_seasons
FOR EACH ROW
BEGIN
    DELETE FROM enrichment_queue WHERE media_type = 'tv_season' AND media_id = OLD.id;
END;
