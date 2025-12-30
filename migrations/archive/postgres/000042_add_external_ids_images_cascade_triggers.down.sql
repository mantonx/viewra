-- Remove external_ids and images cascade triggers and functions
DROP TRIGGER IF EXISTS tr_tv_shows_delete_external_ids ON tv_shows;
DROP TRIGGER IF EXISTS tr_tv_seasons_delete_external_ids ON tv_seasons;
DROP TRIGGER IF EXISTS tr_tv_episodes_delete_external_ids ON tv_episodes;
DROP TRIGGER IF EXISTS tr_people_delete_external_ids ON people;
DROP TRIGGER IF EXISTS tr_studios_delete_external_ids ON studios;
DROP TRIGGER IF EXISTS tr_tv_shows_delete_images ON tv_shows;
DROP TRIGGER IF EXISTS tr_tv_seasons_delete_images ON tv_seasons;
DROP TRIGGER IF EXISTS tr_tv_episodes_delete_images ON tv_episodes;

DROP FUNCTION IF EXISTS fn_tv_shows_cleanup_external_ids();
DROP FUNCTION IF EXISTS fn_tv_seasons_cleanup_external_ids();
DROP FUNCTION IF EXISTS fn_tv_episodes_cleanup_external_ids();
DROP FUNCTION IF EXISTS fn_people_cleanup_external_ids();
DROP FUNCTION IF EXISTS fn_studios_cleanup_external_ids();
DROP FUNCTION IF EXISTS fn_tv_shows_cleanup_images();
DROP FUNCTION IF EXISTS fn_tv_seasons_cleanup_images();
DROP FUNCTION IF EXISTS fn_tv_episodes_cleanup_images();
