-- Restore CHECK constraint on tv_shows.status
-- Note: This will fail if there are rows with status values not in the CHECK list

ALTER TABLE tv_shows ADD CONSTRAINT tv_shows_status_check
    CHECK (status IN ('Returning Series', 'Planned', 'In Production', 'Ended', 'Cancelled', 'Pilot'));
