-- Remove polymorphic cascade triggers and functions
DROP TRIGGER IF EXISTS tr_media_delete_credits ON media;
DROP TRIGGER IF EXISTS tr_media_delete_media_studios ON media;
DROP TRIGGER IF EXISTS tr_tv_shows_delete_credits ON tv_shows;
DROP TRIGGER IF EXISTS tr_tv_shows_delete_media_studios ON tv_shows;
DROP TRIGGER IF EXISTS tr_tv_episodes_delete_credits ON tv_episodes;
DROP TRIGGER IF EXISTS tr_credits_cleanup_orphan_people ON credits;
DROP TRIGGER IF EXISTS tr_media_studios_cleanup_orphan_studios ON media_studios;

DROP FUNCTION IF EXISTS fn_media_cleanup_credits();
DROP FUNCTION IF EXISTS fn_media_cleanup_media_studios();
DROP FUNCTION IF EXISTS fn_tv_shows_cleanup_credits();
DROP FUNCTION IF EXISTS fn_tv_shows_cleanup_media_studios();
DROP FUNCTION IF EXISTS fn_tv_episodes_cleanup_credits();
DROP FUNCTION IF EXISTS fn_credits_cleanup_orphan_people();
DROP FUNCTION IF EXISTS fn_media_studios_cleanup_orphan_studios();
