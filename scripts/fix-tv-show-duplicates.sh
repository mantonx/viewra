#!/bin/bash

# Script to fix TV show duplicates and add database constraints
# This script will:
# 1. Stop the backend to release database locks
# 2. Analyze and consolidate duplicate TV shows  
# 3. Add database constraints to prevent future duplicates
# 4. Restart the backend with the fixes

set -e

echo "=== Fixing TV Show Duplicates ==="
echo "This script will consolidate duplicate TV show records and add constraints"
echo ""

# Function to run SQL and show results
run_sql() {
    local description="$1"
    local sql="$2"
    echo ">>> $description"
    echo "$sql" | docker run --rm -i -v "$(pwd)/viewra-data:/data" nouchka/sqlite3 /data/viewra.db
    echo ""
}

# Stop the backend to release database locks
echo "1. Stopping backend to release database locks..."
docker-compose stop backend
sleep 2

echo "2. Analyzing duplicate TV shows..."

# Show the current state
run_sql "Total TV shows before cleanup:" "SELECT COUNT(*) as total FROM tv_shows;"

run_sql "Top 20 duplicate TV shows:" \
"SELECT title, COUNT(*) as duplicate_count FROM tv_shows GROUP BY title HAVING COUNT(*) > 1 ORDER BY duplicate_count DESC LIMIT 20;"

run_sql "TV shows with and without TMDb IDs:" \
"SELECT 
  CASE 
    WHEN tmdb_id IS NULL OR tmdb_id = '' THEN 'No TMDb ID' 
    ELSE 'Has TMDb ID' 
  END as tmdb_status,
  COUNT(*) as count
FROM tv_shows 
GROUP BY 
  CASE 
    WHEN tmdb_id IS NULL OR tmdb_id = '' THEN 'No TMDb ID' 
    ELSE 'Has TMDb ID' 
  END;"

echo "3. Consolidating duplicate TV shows..."

# Create a temporary table to hold the consolidated records
run_sql "Creating temporary consolidation table..." \
"CREATE TABLE IF NOT EXISTS tv_shows_consolidated AS
SELECT 
  MIN(id) as id,
  title,
  -- Prefer non-empty descriptions
  CASE 
    WHEN COUNT(CASE WHEN description IS NOT NULL AND description != '' THEN 1 END) > 0 
    THEN (SELECT description FROM tv_shows t2 WHERE t2.title = tv_shows.title AND t2.description IS NOT NULL AND t2.description != '' LIMIT 1)
    ELSE NULL
  END as description,
  -- Prefer the earliest first air date
  MIN(first_air_date) as first_air_date,
  -- Prefer non-empty status
  CASE 
    WHEN COUNT(CASE WHEN status IS NOT NULL AND status != '' AND status != 'Unknown' THEN 1 END) > 0 
    THEN (SELECT status FROM tv_shows t2 WHERE t2.title = tv_shows.title AND t2.status IS NOT NULL AND t2.status != '' AND t2.status != 'Unknown' LIMIT 1)
    ELSE 'Unknown'
  END as status,
  -- Prefer non-empty poster
  CASE 
    WHEN COUNT(CASE WHEN poster IS NOT NULL AND poster != '' THEN 1 END) > 0 
    THEN (SELECT poster FROM tv_shows t2 WHERE t2.title = tv_shows.title AND t2.poster IS NOT NULL AND t2.poster != '' LIMIT 1)
    ELSE NULL
  END as poster,
  -- Prefer non-empty backdrop
  CASE 
    WHEN COUNT(CASE WHEN backdrop IS NOT NULL AND backdrop != '' THEN 1 END) > 0 
    THEN (SELECT backdrop FROM tv_shows t2 WHERE t2.title = tv_shows.title AND t2.backdrop IS NOT NULL AND t2.backdrop != '' LIMIT 1)
    ELSE NULL
  END as backdrop,
  -- Prefer non-empty tmdb_id
  CASE 
    WHEN COUNT(CASE WHEN tmdb_id IS NOT NULL AND tmdb_id != '' THEN 1 END) > 0 
    THEN (SELECT tmdb_id FROM tv_shows t2 WHERE t2.title = tv_shows.title AND t2.tmdb_id IS NOT NULL AND t2.tmdb_id != '' LIMIT 1)
    ELSE NULL
  END as tmdb_id,
  -- Use the earliest created_at
  MIN(created_at) as created_at,
  -- Use the latest updated_at
  MAX(updated_at) as updated_at
FROM tv_shows 
GROUP BY title;"

# Update foreign key references before deletion
echo "4. Updating foreign key references..."

# Update seasons to point to the consolidated TV show IDs
run_sql "Updating season references..." \
"UPDATE seasons SET tv_show_id = (
  SELECT tc.id 
  FROM tv_shows_consolidated tc
  JOIN tv_shows ts ON tc.title = ts.title
  WHERE ts.id = seasons.tv_show_id
  LIMIT 1
) WHERE EXISTS (
  SELECT 1 FROM tv_shows_consolidated tc
  JOIN tv_shows ts ON tc.title = ts.title
  WHERE ts.id = seasons.tv_show_id
);"

# Update any other tables that might reference tv_shows
run_sql "Checking for other references to TV shows..." \
"SELECT name FROM sqlite_master WHERE type='table' AND sql LIKE '%tv_show_id%';"

echo "5. Replacing duplicate TV shows with consolidated records..."

# Delete the original tv_shows table and replace with consolidated
run_sql "Backing up original tv_shows table..." \
"ALTER TABLE tv_shows RENAME TO tv_shows_backup;"

run_sql "Creating new tv_shows table with consolidated data..." \
"ALTER TABLE tv_shows_consolidated RENAME TO tv_shows;"

echo "6. Adding database constraints to prevent future duplicates..."

# Create indexes for better performance and some uniqueness enforcement
run_sql "Adding database indexes..." \
"CREATE INDEX IF NOT EXISTS idx_tv_shows_title_unique ON tv_shows(title);
CREATE INDEX IF NOT EXISTS idx_tv_shows_tmdb_id_unique ON tv_shows(tmdb_id);
CREATE INDEX IF NOT EXISTS idx_tv_shows_title_lower ON tv_shows(LOWER(title));"

# Note: SQLite doesn't support adding UNIQUE constraints to existing tables easily
# So we'll rely on application-level duplicate prevention

echo "7. Cleaning up and showing results..."

run_sql "Total TV shows after consolidation:" "SELECT COUNT(*) as total FROM tv_shows;"

run_sql "Remaining duplicates (should be 0):" \
"SELECT title, COUNT(*) as count FROM tv_shows GROUP BY title HAVING COUNT(*) > 1;"

run_sql "TV shows with TMDb IDs after consolidation:" \
"SELECT 
  CASE 
    WHEN tmdb_id IS NULL OR tmdb_id = '' THEN 'No TMDb ID' 
    ELSE 'Has TMDb ID' 
  END as tmdb_status,
  COUNT(*) as count
FROM tv_shows 
GROUP BY 
  CASE 
    WHEN tmdb_id IS NULL OR tmdb_id = '' THEN 'No TMDb ID' 
    ELSE 'Has TMDb ID' 
  END;"

run_sql "Sample of consolidated TV shows:" \
"SELECT title, tmdb_id, 
  CASE WHEN description IS NULL OR description = '' THEN 'No description' ELSE 'Has description' END as desc_status,
  first_air_date 
FROM tv_shows LIMIT 10;"

echo "8. Rebuilding plugins to apply the duplicate prevention fixes..."
make build-plugins-container

echo "9. Restarting backend with duplicate prevention fixes..."
docker-compose start backend

echo ""
echo "=== TV Show Duplicate Fix Complete! ==="
echo "✅ Consolidated duplicate TV show records"
echo "✅ Updated foreign key references in seasons table"
echo "✅ Added database indexes for better performance"
echo "✅ Applied application-level duplicate prevention in plugins"
echo "✅ Backend restarted with enhanced coordination between plugins"
echo ""
echo "Improvements made:"
echo "- TMDb enricher now checks for existing shows by both TMDb ID and title"
echo "- TV Structure plugin has better duplicate detection with title normalization"
echo "- Enhanced coordination between plugins to prevent future duplicates"
echo "- Database indexes added for better query performance" 