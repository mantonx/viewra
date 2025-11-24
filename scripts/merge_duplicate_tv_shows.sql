-- Script to merge duplicate TV shows
-- This handles the duplicates created by parser bugs that have now been fixed

BEGIN TRANSACTION;

-- ============================================================================
-- 1. Ampersand-related duplicates (keep the one WITH ampersand)
-- ============================================================================

-- The Adventures of Pete & Pete: keep 960 (with &), merge 253 (without &)
UPDATE tv_episodes SET show_id = 960 WHERE show_id = 253;
-- Delete duplicate seasons (both shows have overlapping seasons)
DELETE FROM tv_seasons WHERE show_id = 253;
-- Delete duplicate images (both shows point to same image files)
DELETE FROM media_images WHERE media_type = 'tv_show' AND entity_id = 253;
DELETE FROM tv_shows WHERE id = 253;

-- Interview with the Vampire: keep 397 (15 episodes), delete 398 (0 episodes)
DELETE FROM tv_shows WHERE id = 398;

-- ============================================================================
-- 2. Case sensitivity duplicates (keep the properly cased one)
-- ============================================================================

-- Feud vs FEUD: keep 234 "Feud" (11 episodes), merge 375 "FEUD" (5 episodes)
UPDATE tv_episodes SET show_id = 234 WHERE show_id = 375;
DELETE FROM tv_seasons WHERE show_id = 375;
DELETE FROM media_images WHERE media_type = 'tv_show' AND entity_id = 375;
DELETE FROM tv_shows WHERE id = 375;

-- Quiet on Set: keep 866 (proper case), merge 1126
UPDATE tv_episodes SET show_id = 866 WHERE show_id = 1126;
DELETE FROM tv_seasons WHERE show_id = 1126;
DELETE FROM media_images WHERE media_type = 'tv_show' AND entity_id = 1126;
DELETE FROM tv_shows WHERE id = 1126;

-- Real Time with Bill Maher: keep 394 (7 episodes), merge 673 (6 episodes)
UPDATE tv_episodes SET show_id = 394 WHERE show_id = 673;
DELETE FROM tv_seasons WHERE show_id = 673;
DELETE FROM media_images WHERE media_type = 'tv_show' AND entity_id = 673;
DELETE FROM tv_shows WHERE id = 673;

-- Top of the Lake: keep 531 (12 episodes), merge 607 (1 episode)
UPDATE tv_episodes SET show_id = 531 WHERE show_id = 607;
DELETE FROM tv_seasons WHERE show_id = 607;
DELETE FROM media_images WHERE media_type = 'tv_show' AND entity_id = 607;
DELETE FROM tv_shows WHERE id = 607;

-- Andor: keep 448 (21 episodes), delete 449 (0 episodes)
DELETE FROM tv_shows WHERE id = 449;

-- ============================================================================
-- 3. Looney Tunes duplicates with episode info in title (empty duplicates)
-- ============================================================================

-- All these have 0 episodes in the duplicate, so just delete them
DELETE FROM tv_shows WHERE id IN (
    1314, 1407, 694, 1422, 828, 442, 795, 493, 850
);

-- ============================================================================
-- Update show titles to remove episode info for Looney Tunes
-- (The rescan will fix this properly, but let's clean up the worst ones)
-- ============================================================================

UPDATE tv_shows
SET title = 'Looney Tunes'
WHERE title LIKE 'Looney Tunes (1929) S%';

COMMIT;

-- Verification queries
SELECT 'Duplicate check - should return 0 rows:' as check_type;
SELECT title, COUNT(*) as count
FROM tv_shows
GROUP BY library_id, LOWER(title)
HAVING COUNT(*) > 1;

SELECT 'Shows with episode patterns in title - should return 0 rows:' as check_type;
SELECT id, title FROM tv_shows WHERE title LIKE '%S[0-9]%E[0-9]%';

SELECT 'Shows updated to Looney Tunes:' as check_type;
SELECT COUNT(*) FROM tv_shows WHERE title = 'Looney Tunes';
