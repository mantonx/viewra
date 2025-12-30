-- Drop ai_settings table as AI settings are now stored in system_settings
-- The system_settings table provides unified settings storage with encryption support
DROP TABLE IF EXISTS ai_settings;
