-- Remove filesystem monitoring support from libraries

ALTER TABLE libraries DROP COLUMN IF EXISTS monitoring_config;
ALTER TABLE libraries DROP COLUMN IF EXISTS monitoring_enabled;
