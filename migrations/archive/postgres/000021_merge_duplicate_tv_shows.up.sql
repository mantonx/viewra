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
        -- replace remaining hyphens with spaces, collapse multiple spaces
        LOWER(
            TRIM(
                REGEXP_REPLACE(
                    REGEXP_REPLACE(
                        REGEXP_REPLACE(title, ' - ', ' ', 'g'),
                        '-', ' ', 'g'
                    ),
                    '\s+', ' ', 'g'
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
        TRIM(REGEXP_REPLACE(normalized_title, '\s+\d{4}$', '')) as normalized_title
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
DELETE FROM media
WHERE id NOT IN (SELECT media_id FROM tv_episodes)
AND id NOT IN (SELECT media_id FROM movies)
AND id NOT IN (SELECT media_id FROM music_tracks)
AND type = 'tv_episode';

-- Step 3: Update remaining episodes from duplicate shows to point to canonical show
-- First, we need to update the season_id to match the canonical show's season
UPDATE tv_episodes te
SET
    show_id = dm.keep_id,
    season_id = s.id
FROM duplicate_show_mapping dm
JOIN tv_seasons s ON s.show_id = dm.keep_id AND s.season_number = te.season_number
WHERE te.show_id = dm.duplicate_id;

-- Step 4: For episodes whose season doesn't exist in canonical show yet,
-- move the season to canonical show first
UPDATE tv_seasons ts
SET show_id = dm.keep_id
FROM duplicate_show_mapping dm
WHERE ts.show_id = dm.duplicate_id
AND NOT EXISTS (
    SELECT 1 FROM tv_seasons s2
    WHERE s2.show_id = dm.keep_id
    AND s2.season_number = ts.season_number
);

-- Step 5: Now update any remaining episodes to point to canonical show
UPDATE tv_episodes te
SET show_id = dm.keep_id
FROM duplicate_show_mapping dm
WHERE te.show_id = dm.duplicate_id;

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
