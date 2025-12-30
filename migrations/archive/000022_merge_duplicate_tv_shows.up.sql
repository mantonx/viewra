-- Migration to merge duplicate TV shows
-- This fixes issues where the same show was created multiple times due to
-- title variations like "Star Trek - Voyager" vs "Star Trek Voyager"
-- or "Real Time with Bill Maher - 2013" vs "Real Time with Bill Maher"

-- Create a temporary table to map duplicate show IDs to their canonical (keep) show ID
-- The canonical show is the one with the lowest ID (first created)
CREATE TEMPORARY TABLE duplicate_show_mapping AS
WITH normalized_shows AS (
    SELECT
        id,
        library_id,
        title,
        -- Normalize the title: lowercase, remove hyphens/dashes surrounded by spaces,
        -- remove trailing years after dash, remove year in parentheses
        LOWER(
            TRIM(
                REPLACE(
                    REPLACE(
                        REPLACE(
                            REPLACE(
                                REPLACE(title, ' - ', ' '),
                                '-', ' '
                            ),
                            '  ', ' '
                        ),
                        '  ', ' '
                    ),
                    '  ', ' '
                )
            )
        ) as normalized_title
    FROM tv_shows
),
-- Remove trailing 4-digit years (like " 2013" at the end)
further_normalized AS (
    SELECT
        id,
        library_id,
        title,
        -- Remove trailing year pattern (space + 4 digits at end)
        CASE
            WHEN normalized_title GLOB '* [0-9][0-9][0-9][0-9]'
            THEN SUBSTR(normalized_title, 1, LENGTH(normalized_title) - 5)
            ELSE normalized_title
        END as normalized_title
    FROM normalized_shows
),
-- Find the canonical show (lowest ID) for each normalized title
canonical_shows AS (
    SELECT
        library_id,
        normalized_title,
        MIN(id) as canonical_id
    FROM further_normalized
    GROUP BY library_id, normalized_title
)
-- Map each show to its canonical version
SELECT
    fn.id as duplicate_id,
    cs.canonical_id as keep_id,
    fn.library_id,
    fn.title as duplicate_title,
    (SELECT title FROM tv_shows WHERE id = cs.canonical_id) as canonical_title
FROM further_normalized fn
JOIN canonical_shows cs ON fn.library_id = cs.library_id
    AND fn.normalized_title = cs.normalized_title
WHERE fn.id != cs.canonical_id;

-- Step 1: Delete episodes from duplicate shows that would conflict with canonical show
-- (i.e., same season_number + episode_number already exists in canonical)
DELETE FROM tv_episodes
WHERE media_id IN (
    SELECT e1.media_id
    FROM tv_episodes e1
    JOIN duplicate_show_mapping dm ON e1.show_id = dm.duplicate_id
    WHERE EXISTS (
        SELECT 1 FROM tv_episodes e2
        WHERE e2.show_id = dm.keep_id
        AND e2.season_number = e1.season_number
        AND e2.episode_number = e1.episode_number
    )
);

-- Step 2: Delete the media records for those deleted episodes
-- Note: The above DELETE already removed the episode records, but we need to clean up
-- orphaned media records. Since there's a FK from tv_episodes to media, deleting
-- episodes first is correct. But the media records may have ON DELETE CASCADE on
-- other tables, so we should delete them too.
DELETE FROM media
WHERE id NOT IN (SELECT media_id FROM tv_episodes)
AND id NOT IN (SELECT media_id FROM movies)
AND id NOT IN (SELECT media_id FROM music_tracks)
AND type = 'tv_episode';

-- Step 3: Update remaining episodes from duplicate shows to point to canonical show
-- First, we need to update the season_id to match the canonical show's season
UPDATE tv_episodes
SET
    show_id = (
        SELECT keep_id
        FROM duplicate_show_mapping
        WHERE duplicate_id = tv_episodes.show_id
    ),
    season_id = (
        SELECT s.id
        FROM tv_seasons s
        JOIN duplicate_show_mapping dm ON tv_episodes.show_id = dm.duplicate_id
        WHERE s.show_id = dm.keep_id
        AND s.season_number = tv_episodes.season_number
    )
WHERE show_id IN (SELECT duplicate_id FROM duplicate_show_mapping)
AND EXISTS (
    SELECT 1 FROM tv_seasons s
    JOIN duplicate_show_mapping dm ON tv_episodes.show_id = dm.duplicate_id
    WHERE s.show_id = dm.keep_id
    AND s.season_number = tv_episodes.season_number
);

-- Step 4: For episodes whose season doesn't exist in canonical show yet,
-- move the season to canonical show first
UPDATE tv_seasons
SET show_id = (
    SELECT keep_id
    FROM duplicate_show_mapping
    WHERE duplicate_id = tv_seasons.show_id
)
WHERE show_id IN (SELECT duplicate_id FROM duplicate_show_mapping)
AND NOT EXISTS (
    SELECT 1 FROM tv_seasons s2
    JOIN duplicate_show_mapping dm ON dm.duplicate_id = tv_seasons.show_id
    WHERE s2.show_id = dm.keep_id
    AND s2.season_number = tv_seasons.season_number
);

-- Step 5: Now update any remaining episodes to point to canonical show
UPDATE tv_episodes
SET show_id = (
    SELECT keep_id
    FROM duplicate_show_mapping
    WHERE duplicate_id = tv_episodes.show_id
)
WHERE show_id IN (SELECT duplicate_id FROM duplicate_show_mapping);

-- Step 6: Delete duplicate seasons where canonical already has the same season number
DELETE FROM tv_seasons
WHERE show_id IN (SELECT duplicate_id FROM duplicate_show_mapping);

-- Step 7: Delete the duplicate shows
DELETE FROM tv_shows
WHERE id IN (SELECT duplicate_id FROM duplicate_show_mapping);

-- Step 8: Recalculate episode counts for all seasons
UPDATE tv_seasons
SET episode_count = (
    SELECT COUNT(*)
    FROM tv_episodes
    WHERE tv_episodes.season_id = tv_seasons.id
);

-- Clean up the temporary table
DROP TABLE duplicate_show_mapping;
