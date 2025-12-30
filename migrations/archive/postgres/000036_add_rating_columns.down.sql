-- Remove rating columns from movies, tv_shows, and tv_episodes tables

ALTER TABLE movies DROP COLUMN IF EXISTS rating;
ALTER TABLE movies DROP COLUMN IF EXISTS rating_votes;

ALTER TABLE tv_shows DROP COLUMN IF EXISTS rating;
ALTER TABLE tv_shows DROP COLUMN IF EXISTS rating_votes;
ALTER TABLE tv_shows DROP COLUMN IF EXISTS tagline;

ALTER TABLE tv_episodes DROP COLUMN IF EXISTS rating;
ALTER TABLE tv_episodes DROP COLUMN IF EXISTS rating_votes;
ALTER TABLE tv_episodes DROP COLUMN IF EXISTS runtime_minutes;
