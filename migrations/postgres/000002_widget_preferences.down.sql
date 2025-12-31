-- Rollback widget preferences table

DROP INDEX IF EXISTS idx_widget_prefs_user;
DROP INDEX IF EXISTS idx_widget_prefs_user_location;
DROP TABLE IF EXISTS widget_preferences;
