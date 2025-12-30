-- Migration to merge TV show duplicates caused by:
-- 1. Missing/extra articles (A, The): "A Gentleman in Moscow" vs "Gentleman in Moscow"
-- 2. Missing words: "City on a Hill" vs "City on Hill"
-- 3. Truncated names: "Gen V" vs "Gen"
-- 4. Punctuation variants: "Giri/Haji" vs "Giri+Haji"

-- Define specific duplicate pairs to merge
-- Format: (duplicate_id_to_remove, canonical_id_to_keep, reason)
CREATE TEMPORARY TABLE duplicate_pairs (
    duplicate_id INTEGER,
    keep_id INTEGER,
    reason TEXT
);

-- Insert known duplicates (keep the one with more episodes, or if tied, lower ID)
-- These were identified by manual review

-- "A Discovery of Witches" (0 eps) -> "Discovery of Witches" (25 eps)
INSERT INTO duplicate_pairs VALUES (24593, 24642, 'article variant');

-- "A Gentleman in Moscow" (0 eps) -> "Gentleman in Moscow" (8 eps)
INSERT INTO duplicate_pairs VALUES (23347, 23397, 'article variant');

-- "A Small Light" (1 ep) -> "Small Light" (7 eps)
INSERT INTO duplicate_pairs VALUES (22995, 30146, 'article variant');

-- "City on a Hill" (0 eps) -> "City on Hill" (26 eps)
INSERT INTO duplicate_pairs VALUES (23951, 24002, 'word variant');

-- "Gen V" (0 eps) -> "Gen" (16 eps) - NOTE: "Gen" is truncated, but has all episodes
-- We keep "Gen" and will rely on enrichment to update title to "Gen V"
INSERT INTO duplicate_pairs VALUES (25639, 25687, 'truncated name');

-- "Giri/Haji" (0 eps) -> "Giri+Haji" (8 eps)
INSERT INTO duplicate_pairs VALUES (24412, 24461, 'punctuation variant');

-- Step 1: Move episodes from duplicate shows to canonical shows
-- First, delete any conflicting episodes (same season/episode number)
DELETE FROM tv_episodes
WHERE media_id IN (
    SELECT e1.media_id
    FROM tv_episodes e1
    JOIN duplicate_pairs dp ON e1.show_id = dp.duplicate_id
    WHERE EXISTS (
        SELECT 1 FROM tv_episodes e2
        WHERE e2.show_id = dp.keep_id
        AND e2.season_number = e1.season_number
        AND e2.episode_number = e1.episode_number
    )
);

-- Step 2: Move seasons from duplicate shows that don't exist in canonical
UPDATE tv_seasons
SET show_id = (
    SELECT keep_id FROM duplicate_pairs WHERE duplicate_id = tv_seasons.show_id
)
WHERE show_id IN (SELECT duplicate_id FROM duplicate_pairs)
AND NOT EXISTS (
    SELECT 1 FROM tv_seasons s2
    JOIN duplicate_pairs dp ON dp.duplicate_id = tv_seasons.show_id
    WHERE s2.show_id = dp.keep_id
    AND s2.season_number = tv_seasons.season_number
);

-- Step 3: Update remaining episodes to point to canonical show
UPDATE tv_episodes
SET
    show_id = (SELECT keep_id FROM duplicate_pairs WHERE duplicate_id = tv_episodes.show_id),
    season_id = COALESCE(
        (SELECT s.id FROM tv_seasons s
         JOIN duplicate_pairs dp ON tv_episodes.show_id = dp.duplicate_id
         WHERE s.show_id = dp.keep_id AND s.season_number = tv_episodes.season_number),
        tv_episodes.season_id
    )
WHERE show_id IN (SELECT duplicate_id FROM duplicate_pairs);

-- Step 4: Delete remaining duplicate seasons
DELETE FROM tv_seasons WHERE show_id IN (SELECT duplicate_id FROM duplicate_pairs);

-- Step 5: Delete duplicate shows
DELETE FROM tv_shows WHERE id IN (SELECT duplicate_id FROM duplicate_pairs);

-- Step 6: Recalculate episode counts
UPDATE tv_seasons
SET episode_count = (
    SELECT COUNT(*) FROM tv_episodes WHERE tv_episodes.season_id = tv_seasons.id
);

-- Step 7: Clean up orphaned media records
DELETE FROM media
WHERE id NOT IN (SELECT media_id FROM tv_episodes)
AND id NOT IN (SELECT media_id FROM movies)
AND id NOT IN (SELECT media_id FROM music_tracks)
AND type = 'tv_episode';

-- Clean up enrichment queue entries for deleted shows
DELETE FROM enrichment_queue
WHERE media_type = 'tv_show'
AND media_id IN (SELECT duplicate_id FROM duplicate_pairs);

-- Clean up enrichment status entries for deleted shows
DELETE FROM enrichment_status
WHERE media_type = 'tv_show'
AND media_id IN (SELECT duplicate_id FROM duplicate_pairs);

DROP TABLE duplicate_pairs;
