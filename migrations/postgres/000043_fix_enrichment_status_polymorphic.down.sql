-- Revert enrichment_status to original schema with FK constraint

-- Drop triggers
DROP TRIGGER IF EXISTS tr_media_delete_enrichment_status ON media;
DROP TRIGGER IF EXISTS tr_tv_shows_delete_enrichment_status ON tv_shows;
DROP TRIGGER IF EXISTS tr_tv_seasons_delete_enrichment_status ON tv_seasons;

-- Drop functions
DROP FUNCTION IF EXISTS fn_media_cleanup_enrichment_status();
DROP FUNCTION IF EXISTS fn_tv_shows_cleanup_enrichment_status();
DROP FUNCTION IF EXISTS fn_tv_seasons_cleanup_enrichment_status();

-- Drop index
DROP INDEX IF EXISTS idx_enrichment_status_media;

-- Delete non-media entries (tv_show, tv_season)
DELETE FROM enrichment_status WHERE media_type NOT IN ('movie', 'tv');

-- Delete orphaned entries
DELETE FROM enrichment_status WHERE NOT EXISTS (SELECT 1 FROM media m WHERE m.id = enrichment_status.media_id);

-- Drop check constraint
ALTER TABLE enrichment_status DROP CONSTRAINT IF EXISTS enrichment_status_media_type_check;

-- Change primary key back to original
ALTER TABLE enrichment_status DROP CONSTRAINT enrichment_status_pkey;
ALTER TABLE enrichment_status ADD PRIMARY KEY (media_id, stage);

-- Add back FK constraint
ALTER TABLE enrichment_status ADD CONSTRAINT enrichment_status_media_id_fkey
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE;

-- Drop media_type column
ALTER TABLE enrichment_status DROP COLUMN media_type;
