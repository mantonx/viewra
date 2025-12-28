-- Note: This requires SQLite 3.35.0+ (2021-03-12)
ALTER TABLE libraries DROP COLUMN monitoring_config;
ALTER TABLE libraries DROP COLUMN monitoring_enabled;
