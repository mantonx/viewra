-- Add settings storage to plugins table
-- Settings are stored as JSON and passed to plugins on Configure()

ALTER TABLE plugins ADD COLUMN settings TEXT DEFAULT '{}';
ALTER TABLE plugins ADD COLUMN settings_schema TEXT;
