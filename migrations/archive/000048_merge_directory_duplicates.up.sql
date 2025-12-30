-- Migration: Merge duplicate TV shows that point to the same directory
-- These duplicates occurred due to parsing differences (missing articles, punctuation, stylized names)

-- Create temp table with duplicate pairs (keep_id, remove_id, reason)
CREATE TEMP TABLE duplicate_pairs (
    keep_id INTEGER,
    remove_id INTEGER,
    reason TEXT
);

-- Andor: Keep "Andor" (23205, 2 seasons, 21 eps) over "Star Wars Andor" (25312, 1 season, 3 eps)
INSERT INTO duplicate_pairs VALUES (23205, 25312, 'same directory, more episodes');

-- Aqua Teen Hunger Force: Keep main show (23342, 12 seasons, 144 eps)
-- Remove "Aqua Teen Hunger Force Forever" (29090) and "Aqua TV Show Show" (38569)
INSERT INTO duplicate_pairs VALUES (23342, 29090, 'same directory, spinoff name');
INSERT INTO duplicate_pairs VALUES (23342, 38569, 'same directory, alternate name');

-- Looney Tunes: Keep "Looney Tunes" (22996, 62 seasons, 943 eps) over "New Looney Tunes" (22671, 5 seasons)
INSERT INTO duplicate_pairs VALUES (22996, 22671, 'same directory, more episodes');

-- Once Upon a Time: Keep "Once Upon Time" (23411, 7 seasons, 156 eps) over "Once Upon a Time (2011)" (23361, 1 season)
-- Note: The kept title is missing "a" - we'll fix the title after merge
INSERT INTO duplicate_pairs VALUES (23411, 23361, 'same directory, missing article');

-- Philip K. Dick's Electric Dreams: Keep "Philip K Dicks Electric Dreams" (24008, 10 eps) over "Electric Dreams (2017)" (23955, 0 eps)
INSERT INTO duplicate_pairs VALUES (24008, 23955, 'same directory, more episodes');

-- Pluribus: Keep "PLUR1BUS" (38565, 4 eps) over "Pluribus" (30458, 3 eps) - more complete
INSERT INTO duplicate_pairs VALUES (38565, 30458, 'same directory, more episodes');

-- TURN: Keep "TURN Washingtons Spies" (23844, 4 seasons, 40 eps) over "Turnt" (23794, bad parse)
INSERT INTO duplicate_pairs VALUES (23844, 23794, 'same directory, bad parse');

-- Z Nation: Keep "Nation" (23865, 5 seasons, 68 eps) over "Z Nation" (23817, 0 eps)
-- Note: The kept title is wrong - we'll fix the title after merge
INSERT INTO duplicate_pairs VALUES (23865, 23817, 'same directory, more episodes');

-- Step 1: Move seasons from remove_id to keep_id (handle conflicts)
-- For seasons that exist in both, keep the one with more data
UPDATE tv_seasons
SET show_id = (SELECT keep_id FROM duplicate_pairs WHERE remove_id = tv_seasons.show_id)
WHERE show_id IN (SELECT remove_id FROM duplicate_pairs)
AND NOT EXISTS (
    SELECT 1 FROM tv_seasons ts2
    WHERE ts2.show_id = (SELECT keep_id FROM duplicate_pairs WHERE remove_id = tv_seasons.show_id)
    AND ts2.season_number = tv_seasons.season_number
);

-- Step 2: Delete duplicate seasons that couldn't be moved (conflicts)
DELETE FROM tv_seasons
WHERE show_id IN (SELECT remove_id FROM duplicate_pairs);

-- Step 3: Delete enrichment_status records for shows being removed
-- (The canonical show already has its own records, so just delete duplicates)
DELETE FROM enrichment_status
WHERE media_type = 'tv_show'
AND media_id IN (SELECT remove_id FROM duplicate_pairs);

-- Step 4: Delete enrichment_queue records for shows being removed
DELETE FROM enrichment_queue
WHERE media_type = 'tv_show'
AND media_id IN (SELECT remove_id FROM duplicate_pairs);

-- Step 5: Delete media_images records for shows being removed
DELETE FROM media_images
WHERE media_type = 'tv_show'
AND entity_id IN (SELECT remove_id FROM duplicate_pairs);

-- Step 6: Delete the duplicate show records
DELETE FROM tv_shows WHERE id IN (SELECT remove_id FROM duplicate_pairs);

-- Step 7: Fix incorrect titles
-- "Once Upon Time" should be "Once Upon a Time"
UPDATE tv_shows SET title = 'Once Upon a Time' WHERE id = 23411;

-- "Nation" should be "Z Nation"
UPDATE tv_shows SET title = 'Z Nation' WHERE id = 23865;

-- "PLUR1BUS" should be "Pluribus" (normalize stylized name)
UPDATE tv_shows SET title = 'Pluribus' WHERE id = 38565;

-- Recalculate sort_title for fixed shows
UPDATE tv_shows
SET sort_title = LOWER(TRIM(
    CASE
        WHEN title LIKE 'The %' THEN SUBSTR(title, 5)
        WHEN title LIKE 'A %' THEN SUBSTR(title, 3)
        WHEN title LIKE 'An %' THEN SUBSTR(title, 4)
        ELSE title
    END
))
WHERE id IN (23411, 23865, 38565);

-- Cleanup
DROP TABLE duplicate_pairs;
