-- Remove settings columns from plugins table
-- SQLite doesn't support DROP COLUMN directly, but modern SQLite (3.35.0+) does
-- For compatibility, we'll use the approach that works on both SQLite and PostgreSQL

-- For SQLite 3.35.0+ and PostgreSQL:
ALTER TABLE plugins DROP COLUMN settings;
ALTER TABLE plugins DROP COLUMN settings_schema;
