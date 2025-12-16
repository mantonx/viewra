-- Remove enrichment_queue cascade triggers and functions
DROP TRIGGER IF EXISTS tr_media_delete_enrichment_queue ON media;
DROP TRIGGER IF EXISTS tr_tv_shows_delete_enrichment_queue ON tv_shows;
DROP TRIGGER IF EXISTS tr_tv_seasons_delete_enrichment_queue ON tv_seasons;

DROP FUNCTION IF EXISTS fn_media_cleanup_enrichment_queue();
DROP FUNCTION IF EXISTS fn_tv_shows_cleanup_enrichment_queue();
DROP FUNCTION IF EXISTS fn_tv_seasons_cleanup_enrichment_queue();
