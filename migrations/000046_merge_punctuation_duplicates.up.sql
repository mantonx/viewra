-- Migration to merge duplicate TV shows that differ only by punctuation
-- Examples: "Are You Afraid of the Dark?" vs "Are You Afraid of the Dark!"
-- Also handles shows with/without year suffixes like "Battlestar Galactica" vs "Battlestar Galactica (2003)"

-- Create a temporary table to map duplicate show IDs to their canonical (keep) show ID
-- The canonical show is determined by:
-- 1. First, prefer the show with more episodes
-- 2. If tied, prefer the show with the lowest ID (first created)
CREATE TEMPORARY TABLE duplicate_show_mapping AS
WITH normalized_shows AS (
    SELECT
        id,
        library_id,
        title,
        -- Normalize: lowercase, remove punctuation, normalize spaces
        LOWER(
            TRIM(
                REPLACE(
                    REPLACE(
                        REPLACE(
                            REPLACE(
                                REPLACE(
                                    REPLACE(
                                        REPLACE(
                                            REPLACE(
                                                REPLACE(title, '?', ''),
                                                '!', ''
                                            ),
                                            ':', ''
                                        ),
                                        '.', ''
                                    ),
                                    '''', ''
                                ),
                                '"', ''
                            ),
                            ' - ', ' '
                        ),
                        '-', ' '
                    ),
                    '  ', ' '
                )
            )
        ) as normalized_title
    FROM tv_shows
),
-- Also remove year in parentheses at the end like " (2003)"
further_normalized AS (
    SELECT
        id,
        library_id,
        title,
        CASE
            WHEN normalized_title GLOB '* ([0-9][0-9][0-9][0-9])'
            THEN TRIM(SUBSTR(normalized_title, 1, LENGTH(normalized_title) - 7))
            ELSE normalized_title
        END as normalized_title
    FROM normalized_shows
),
-- Count episodes per show
episode_counts AS (
    SELECT show_id, COUNT(*) as ep_count
    FROM tv_episodes
    GROUP BY show_id
),
-- Rank shows within each normalized group
ranked_shows AS (
    SELECT
        fn.id,
        fn.library_id,
        fn.title,
        fn.normalized_title,
        COALESCE(ec.ep_count, 0) as ep_count,
        ROW_NUMBER() OVER (
            PARTITION BY fn.library_id, fn.normalized_title
            ORDER BY COALESCE(ec.ep_count, 0) DESC, fn.id ASC
        ) as rn
    FROM further_normalized fn
    LEFT JOIN episode_counts ec ON ec.show_id = fn.id
),
-- Find the canonical show (rank 1) for each normalized title
canonical_shows AS (
    SELECT library_id, normalized_title, id as canonical_id
    FROM ranked_shows
    WHERE rn = 1
)
-- Map each show to its canonical version
SELECT
    rs.id as duplicate_id,
    cs.canonical_id as keep_id,
    rs.library_id,
    rs.title as duplicate_title,
    (SELECT title FROM tv_shows WHERE id = cs.canonical_id) as canonical_title
FROM ranked_shows rs
JOIN canonical_shows cs ON rs.library_id = cs.library_id
    AND rs.normalized_title = cs.normalized_title
WHERE rs.id != cs.canonical_id;

-- Log what we're about to merge (for debugging)
-- SELECT * FROM duplicate_show_mapping;

-- Step 1: Delete episodes from duplicate shows that would conflict with canonical show
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

-- Step 2: Clean up orphaned media records from deleted episodes
DELETE FROM media
WHERE id NOT IN (SELECT media_id FROM tv_episodes)
AND id NOT IN (SELECT media_id FROM movies)
AND id NOT IN (SELECT media_id FROM music_tracks)
AND type = 'tv_episode';

-- Step 3: Move seasons from duplicate shows to canonical that don't exist in canonical
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

-- Step 4: Update episodes to point to canonical show and their correct season
UPDATE tv_episodes
SET
    show_id = (
        SELECT keep_id
        FROM duplicate_show_mapping
        WHERE duplicate_id = tv_episodes.show_id
    ),
    season_id = COALESCE(
        (
            SELECT s.id
            FROM tv_seasons s
            JOIN duplicate_show_mapping dm ON tv_episodes.show_id = dm.duplicate_id
            WHERE s.show_id = dm.keep_id
            AND s.season_number = tv_episodes.season_number
        ),
        tv_episodes.season_id
    )
WHERE show_id IN (SELECT duplicate_id FROM duplicate_show_mapping);

-- Step 5: Delete remaining duplicate seasons
DELETE FROM tv_seasons
WHERE show_id IN (SELECT duplicate_id FROM duplicate_show_mapping);

-- Step 6: Delete the duplicate shows
DELETE FROM tv_shows
WHERE id IN (SELECT duplicate_id FROM duplicate_show_mapping);

-- Step 7: Recalculate episode counts for all affected seasons
UPDATE tv_seasons
SET episode_count = (
    SELECT COUNT(*)
    FROM tv_episodes
    WHERE tv_episodes.season_id = tv_seasons.id
);

-- Clean up
DROP TABLE duplicate_show_mapping;
