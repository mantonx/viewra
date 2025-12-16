-- Fix enrichment_status to support polymorphic media types (tv_show, tv_season, etc.)
-- The table currently has an FK to media(id), but enrichment runs for tv_shows/tv_seasons
-- which are in separate tables. Add media_type column and remove the FK constraint.

-- Step 1: Add media_type column
ALTER TABLE enrichment_status ADD COLUMN media_type TEXT NOT NULL DEFAULT 'movie';

-- Step 2: Update existing entries to use correct media_type based on media table
-- Note: media.type is 'movie', 'tv_episode', or 'music_track'. Map 'tv_episode' to 'tv' for enrichment_status.
UPDATE enrichment_status es
SET media_type = CASE WHEN m.type = 'tv_episode' THEN 'tv' ELSE COALESCE(m.type, 'movie') END
FROM media m
WHERE es.media_id = m.id;

-- Step 3: Remove the FK constraint
ALTER TABLE enrichment_status DROP CONSTRAINT enrichment_status_media_id_fkey;

-- Step 4: Change primary key to include media_type
ALTER TABLE enrichment_status DROP CONSTRAINT enrichment_status_pkey;
ALTER TABLE enrichment_status ADD PRIMARY KEY (media_type, media_id, stage);

-- Step 5: Add check constraint for media_type
ALTER TABLE enrichment_status ADD CONSTRAINT enrichment_status_media_type_check
    CHECK (media_type IN ('movie', 'tv', 'tv_show', 'tv_season'));

-- Step 6: Add index for media lookups
CREATE INDEX idx_enrichment_status_media ON enrichment_status(media_type, media_id);

-- Step 7: Add trigger functions for cascade deletes
CREATE OR REPLACE FUNCTION fn_media_cleanup_enrichment_status()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM enrichment_status WHERE media_type IN ('movie', 'tv') AND media_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION fn_tv_shows_cleanup_enrichment_status()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM enrichment_status WHERE media_type = 'tv_show' AND media_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION fn_tv_seasons_cleanup_enrichment_status()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM enrichment_status WHERE media_type = 'tv_season' AND media_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

-- Step 8: Create triggers
CREATE TRIGGER tr_media_delete_enrichment_status
AFTER DELETE ON media
FOR EACH ROW
EXECUTE FUNCTION fn_media_cleanup_enrichment_status();

CREATE TRIGGER tr_tv_shows_delete_enrichment_status
AFTER DELETE ON tv_shows
FOR EACH ROW
EXECUTE FUNCTION fn_tv_shows_cleanup_enrichment_status();

CREATE TRIGGER tr_tv_seasons_delete_enrichment_status
AFTER DELETE ON tv_seasons
FOR EACH ROW
EXECUTE FUNCTION fn_tv_seasons_cleanup_enrichment_status();
