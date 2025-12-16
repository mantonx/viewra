-- Revert: Remove person and studio from media_external_ids

-- Delete migrated person/studio external IDs
DELETE FROM media_external_ids WHERE media_type IN ('person', 'studio');

-- Drop the current constraint
ALTER TABLE media_external_ids DROP CONSTRAINT IF EXISTS media_external_ids_media_type_check;

-- Restore original constraint without person/studio
ALTER TABLE media_external_ids ADD CONSTRAINT media_external_ids_media_type_check
    CHECK (media_type IN (
        'movie', 'tv_show', 'tv_season', 'tv_episode',
        'music_artist', 'music_album', 'music_track'
    ));
