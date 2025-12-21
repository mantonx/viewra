-- Add location columns to users table
ALTER TABLE users ADD COLUMN location_latitude DOUBLE PRECISION;
ALTER TABLE users ADD COLUMN location_longitude DOUBLE PRECISION;
ALTER TABLE users ADD COLUMN location_timezone TEXT DEFAULT 'auto';
ALTER TABLE users ADD COLUMN location_enabled BOOLEAN DEFAULT FALSE;
