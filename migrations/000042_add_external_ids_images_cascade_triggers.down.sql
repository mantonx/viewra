-- Remove external_ids and images cascade triggers
DROP TRIGGER IF EXISTS tr_tv_shows_delete_external_ids;
DROP TRIGGER IF EXISTS tr_tv_seasons_delete_external_ids;
DROP TRIGGER IF EXISTS tr_tv_episodes_delete_external_ids;
DROP TRIGGER IF EXISTS tr_people_delete_external_ids;
DROP TRIGGER IF EXISTS tr_studios_delete_external_ids;
DROP TRIGGER IF EXISTS tr_tv_shows_delete_images;
DROP TRIGGER IF EXISTS tr_tv_seasons_delete_images;
DROP TRIGGER IF EXISTS tr_tv_episodes_delete_images;
