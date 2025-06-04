-- Cleanup script for incorrectly classified artwork files
-- These files were processed as media files (episodes) before our scanner fixes

-- First, let's see what we're dealing with
SELECT 'BEFORE CLEANUP - Total artwork files in media_files:' as status, COUNT(*) as count
FROM media_files 
WHERE path LIKE '%poster%' OR path LIKE '%banner%' OR path LIKE '%thumb%' 
   OR path LIKE '%fanart%' OR path LIKE '%artwork%' OR path LIKE '%cover%';

SELECT 'BEFORE CLEANUP - Artwork files by media_type:' as status, media_type, COUNT(*) as count
FROM media_files 
WHERE path LIKE '%poster%' OR path LIKE '%banner%' OR path LIKE '%thumb%' 
   OR path LIKE '%fanart%' OR path LIKE '%artwork%' OR path LIKE '%cover%'
GROUP BY media_type;

-- Clean up incorrectly classified artwork files (those marked as 'episode' or other non-image types)
-- These should never have been processed as media files

-- Delete artwork files that were incorrectly classified as episodes
DELETE FROM media_files 
WHERE media_type = 'episode' 
  AND (path LIKE '%poster%' OR path LIKE '%banner%' OR path LIKE '%thumb%' 
       OR path LIKE '%fanart%' OR path LIKE '%artwork%' OR path LIKE '%cover%'
       OR path LIKE '%season%-poster%' OR path LIKE '%season%-banner%'
       OR path LIKE '%specials-poster%' OR path LIKE '%specials-banner%'
       OR path LIKE '%-poster.jpg' OR path LIKE '%-banner.jpg'
       OR path LIKE '%folder.jpg' OR path LIKE '%albumart%');

-- Also clean up any track-type artwork files (shouldn't happen but just in case)
DELETE FROM media_files 
WHERE media_type = 'track' 
  AND (path LIKE '%poster%' OR path LIKE '%banner%' OR path LIKE '%thumb%' 
       OR path LIKE '%fanart%' OR path LIKE '%artwork%' OR path LIKE '%cover%'
       OR path LIKE '%-poster.jpg' OR path LIKE '%-banner.jpg'
       OR path LIKE '%folder.jpg' OR path LIKE '%albumart%');

-- Also clean up any movie-type artwork files
DELETE FROM media_files 
WHERE media_type = 'movie' 
  AND (path LIKE '%poster%' OR path LIKE '%banner%' OR path LIKE '%thumb%' 
       OR path LIKE '%fanart%' OR path LIKE '%artwork%' OR path LIKE '%cover%'
       OR path LIKE '%-poster.jpg' OR path LIKE '%-banner.jpg'
       OR path LIKE '%folder.jpg' OR path LIKE '%albumart%');

-- Keep image-type artwork files for now (they're correctly classified)
-- but they shouldn't be in media_files either - they should be handled by asset system

-- Clean up related enrichment data for deleted media files
DELETE FROM media_enrichments 
WHERE media_id NOT IN (SELECT id FROM media_files);

-- Clean up any orphaned TV show records that might have been created from artwork files
DELETE FROM tv_shows 
WHERE title IN ('poster', 'banner', 'thumb', 'fanart', 'artwork', 'cover',
                'season01-poster', 'season01-banner', 'season02-poster', 'season02-banner',
                'season03-poster', 'season03-banner', 'season04-poster', 'season04-banner',
                'season05-poster', 'season05-banner', 'specials-poster', 'specials-banner',
                'season-specials-poster', 'season-specials-banner');

-- Clean up TV shows with numeric-only titles (likely from artwork files)
DELETE FROM tv_shows 
WHERE title REGEXP '^[0-9]+$';

-- Show results after cleanup
SELECT 'AFTER CLEANUP - Total artwork files remaining in media_files:' as status, COUNT(*) as count
FROM media_files 
WHERE path LIKE '%poster%' OR path LIKE '%banner%' OR path LIKE '%thumb%' 
   OR path LIKE '%fanart%' OR path LIKE '%artwork%' OR path LIKE '%cover%';

SELECT 'AFTER CLEANUP - Remaining artwork files by media_type:' as status, media_type, COUNT(*) as count
FROM media_files 
WHERE path LIKE '%poster%' OR path LIKE '%banner%' OR path LIKE '%thumb%' 
   OR path LIKE '%fanart%' OR path LIKE '%artwork%' OR path LIKE '%cover%'
GROUP BY media_type;

SELECT 'CLEANUP COMPLETE - Total media files now:' as status, COUNT(*) as count FROM media_files;
SELECT 'CLEANUP COMPLETE - Total TV shows now:' as status, COUNT(*) as count FROM tv_shows; 