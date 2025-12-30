-- SQLite doesn't support DROP COLUMN directly, so we need to recreate tables
-- This is a destructive operation - data in these columns will be lost

-- For SQLite, we need to use the table rebuild approach
-- However, for simplicity in rollback, we'll create new tables without these columns
-- and copy data back. This is complex, so for safety we'll just document the columns
-- that would need manual removal if rolling back.

-- Note: In practice, rolling back these changes requires:
-- 1. Creating new tables without the added columns
-- 2. Copying data from old tables
-- 3. Dropping old tables
-- 4. Renaming new tables

-- For now, this is a no-op since SQLite ALTER TABLE DROP COLUMN
-- is only supported in SQLite 3.35.0+ (2021-03-12)
-- Most deployments should have this version, so we'll use it:

ALTER TABLE movies DROP COLUMN rating;
ALTER TABLE movies DROP COLUMN rating_votes;

ALTER TABLE tv_shows DROP COLUMN rating;
ALTER TABLE tv_shows DROP COLUMN rating_votes;
ALTER TABLE tv_shows DROP COLUMN tagline;

ALTER TABLE tv_episodes DROP COLUMN rating;
ALTER TABLE tv_episodes DROP COLUMN rating_votes;
ALTER TABLE tv_episodes DROP COLUMN runtime_minutes;
