-- Remove restrictive CHECK constraint on tv_shows.status
-- TMDB returns various status values that don't match our limited list
-- (e.g., "Canceled" vs "Cancelled", or other status values)

-- Step 1: Drop the existing CHECK constraint
ALTER TABLE tv_shows DROP CONSTRAINT IF EXISTS tv_shows_status_check;

-- Note: PostgreSQL maintains triggers and indexes automatically,
-- they don't need to be recreated when modifying the table.
