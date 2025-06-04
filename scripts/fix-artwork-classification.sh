#!/bin/bash

# Script to fix the critical artwork classification flaw
# This script will:
# 1. Stop the backend to release database locks
# 2. Clean up incorrectly classified artwork files
# 3. Restart the backend with the scanner fixes

set -e

echo "=== Fixing Critical Artwork Classification Flaw ==="
echo "This script will clean up artwork files that were incorrectly processed as TV episodes"
echo ""

# Function to run SQL and show results
run_sql() {
    local description="$1"
    local sql="$2"
    echo ">>> $description"
    echo "$sql" | sqlite3 viewra-data/viewra.db
    echo ""
}

# Stop the backend to release database locks
echo "1. Stopping backend to release database locks..."
docker-compose stop backend
sleep 2

echo "2. Analyzing the scope of the problem..."

# Show the current state
run_sql "Total media files before cleanup:" "SELECT COUNT(*) as total FROM media_files;"

run_sql "Artwork files incorrectly classified as episodes:" \
"SELECT COUNT(*) as count FROM media_files 
 WHERE media_type = 'episode' 
   AND (path LIKE '%poster%' OR path LIKE '%banner%' OR path LIKE '%thumb%' 
        OR path LIKE '%fanart%' OR path LIKE '%artwork%' OR path LIKE '%cover%');"

run_sql "Sample of incorrectly classified files:" \
"SELECT path, media_type FROM media_files 
 WHERE media_type = 'episode' 
   AND (path LIKE '%poster%' OR path LIKE '%banner%') 
 LIMIT 5;"

run_sql "TV shows with suspicious titles (likely from artwork):" \
"SELECT title FROM tv_shows WHERE title REGEXP '^[0-9]+$' OR title IN ('poster', 'banner') LIMIT 10;"

echo "3. Cleaning up incorrectly classified artwork files..."

# Delete artwork files that were incorrectly classified as episodes
run_sql "Deleting artwork files classified as episodes..." \
"DELETE FROM media_files 
 WHERE media_type = 'episode' 
   AND (path LIKE '%poster%' OR path LIKE '%banner%' OR path LIKE '%thumb%' 
        OR path LIKE '%fanart%' OR path LIKE '%artwork%' OR path LIKE '%cover%'
        OR path LIKE '%season%-poster%' OR path LIKE '%season%-banner%'
        OR path LIKE '%specials-poster%' OR path LIKE '%specials-banner%'
        OR path LIKE '%-poster.jpg' OR path LIKE '%-banner.jpg'
        OR path LIKE '%folder.jpg' OR path LIKE '%albumart%');"

# Clean up any track-type artwork files
run_sql "Deleting artwork files classified as tracks..." \
"DELETE FROM media_files 
 WHERE media_type = 'track' 
   AND (path LIKE '%poster%' OR path LIKE '%banner%' OR path LIKE '%thumb%' 
        OR path LIKE '%fanart%' OR path LIKE '%artwork%' OR path LIKE '%cover%'
        OR path LIKE '%-poster.jpg' OR path LIKE '%-banner.jpg'
        OR path LIKE '%folder.jpg' OR path LIKE '%albumart%');"

# Clean up any movie-type artwork files
run_sql "Deleting artwork files classified as movies..." \
"DELETE FROM media_files 
 WHERE media_type = 'movie' 
   AND (path LIKE '%poster%' OR path LIKE '%banner%' OR path LIKE '%thumb%' 
        OR path LIKE '%fanart%' OR path LIKE '%artwork%' OR path LIKE '%cover%'
        OR path LIKE '%-poster.jpg' OR path LIKE '%-banner.jpg'
        OR path LIKE '%folder.jpg' OR path LIKE '%albumart%');"

echo "4. Cleaning up orphaned enrichment data..."

# Clean up related enrichment data
run_sql "Deleting orphaned enrichment data..." \
"DELETE FROM media_enrichments WHERE media_id NOT IN (SELECT id FROM media_files);"

echo "5. Cleaning up TV shows created from artwork files..."

# Clean up TV show records created from artwork files
run_sql "Deleting TV shows with artwork-related titles..." \
"DELETE FROM tv_shows 
 WHERE title IN ('poster', 'banner', 'thumb', 'fanart', 'artwork', 'cover',
                 'season01-poster', 'season01-banner', 'season02-poster', 'season02-banner',
                 'season03-poster', 'season03-banner', 'season04-poster', 'season04-banner',
                 'season05-poster', 'season05-banner', 'specials-poster', 'specials-banner',
                 'season-specials-poster', 'season-specials-banner');"

# Clean up numeric-only TV show titles (likely from artwork files)
run_sql "Deleting TV shows with numeric-only titles..." \
"DELETE FROM tv_shows WHERE title REGEXP '^[0-9]+$';"

echo "6. Showing results after cleanup..."

run_sql "Total media files after cleanup:" "SELECT COUNT(*) as total FROM media_files;"

run_sql "Remaining artwork files (should be properly classified or moved to assets):" \
"SELECT media_type, COUNT(*) as count FROM media_files 
 WHERE path LIKE '%poster%' OR path LIKE '%banner%' OR path LIKE '%thumb%' 
    OR path LIKE '%fanart%' OR path LIKE '%artwork%' OR path LIKE '%cover%'
 GROUP BY media_type;"

run_sql "Total TV shows after cleanup:" "SELECT COUNT(*) as total FROM tv_shows;"

echo "7. Restarting backend with scanner fixes..."
docker-compose start backend

echo ""
echo "=== Cleanup Complete! ==="
echo "✅ Removed incorrectly classified artwork files from media_files table"
echo "✅ Cleaned up orphaned TV show records created from artwork files"
echo "✅ Scanner fixes are now active to prevent future misclassification"
echo "✅ Backend restarted with intelligent file filtering"
echo ""
echo "Next steps:"
echo "- Monitor future scans to ensure artwork files are properly skipped"
echo "- Consider implementing an asset scanner for artwork management"
echo "- Review media libraries to ensure only actual video files are processed" 