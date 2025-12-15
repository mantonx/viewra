-- Add rating columns to movies, tv_shows, and tv_episodes tables
-- Also adds tagline to tv_shows and runtime_minutes to tv_episodes

-- Movies: rating and vote count
ALTER TABLE movies ADD COLUMN rating REAL;
ALTER TABLE movies ADD COLUMN rating_votes INTEGER;

-- TV Shows: rating, vote count, and tagline
ALTER TABLE tv_shows ADD COLUMN rating REAL;
ALTER TABLE tv_shows ADD COLUMN rating_votes INTEGER;
ALTER TABLE tv_shows ADD COLUMN tagline TEXT;

-- TV Episodes: rating, vote count, and runtime
ALTER TABLE tv_episodes ADD COLUMN rating REAL;
ALTER TABLE tv_episodes ADD COLUMN rating_votes INTEGER;
ALTER TABLE tv_episodes ADD COLUMN runtime_minutes INTEGER;
