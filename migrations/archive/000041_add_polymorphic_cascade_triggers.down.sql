-- Remove polymorphic cascade triggers
DROP TRIGGER IF EXISTS tr_media_delete_credits;
DROP TRIGGER IF EXISTS tr_media_delete_media_studios;
DROP TRIGGER IF EXISTS tr_tv_shows_delete_credits;
DROP TRIGGER IF EXISTS tr_tv_shows_delete_media_studios;
DROP TRIGGER IF EXISTS tr_tv_episodes_delete_credits;
DROP TRIGGER IF EXISTS tr_credits_cleanup_orphan_people;
DROP TRIGGER IF EXISTS tr_media_studios_cleanup_orphan_studios;
