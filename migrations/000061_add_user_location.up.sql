-- Add location columns to users table
ALTER TABLE users ADD COLUMN location_latitude REAL;
ALTER TABLE users ADD COLUMN location_longitude REAL;
ALTER TABLE users ADD COLUMN location_timezone TEXT DEFAULT 'auto';
ALTER TABLE users ADD COLUMN location_enabled INTEGER DEFAULT 0;
