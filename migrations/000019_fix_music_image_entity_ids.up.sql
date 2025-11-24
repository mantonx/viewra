-- Fix music image entity_id references to use proper artist_id and album_id from entity tables
-- The previous implementation incorrectly used media_id (track ID) instead of artist/album entity IDs

-- Delete all existing artist and album images since they have incorrect entity_id references
-- A rescan will regenerate them with correct references using the fixed code
DELETE FROM media_images
WHERE media_type IN ('music_artist', 'music_album');
