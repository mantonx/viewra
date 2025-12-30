-- Remove enrichment_queue cascade triggers
DROP TRIGGER IF EXISTS tr_media_delete_enrichment_queue;
DROP TRIGGER IF EXISTS tr_tv_shows_delete_enrichment_queue;
DROP TRIGGER IF EXISTS tr_tv_seasons_delete_enrichment_queue;
