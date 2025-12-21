-- Revert plugin settings columns

ALTER TABLE plugins DROP COLUMN IF EXISTS settings_schema;
ALTER TABLE plugins DROP COLUMN IF EXISTS settings;
