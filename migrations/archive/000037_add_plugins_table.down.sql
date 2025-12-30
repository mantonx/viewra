-- Remove plugin tables in reverse order of dependencies
DROP INDEX IF EXISTS idx_plugin_user_metadata_plugin;
DROP INDEX IF EXISTS idx_plugin_user_metadata_user;
DROP TABLE IF EXISTS plugin_user_metadata;

DROP INDEX IF EXISTS idx_plugin_api_keys_expires;
DROP INDEX IF EXISTS idx_plugin_api_keys_plugin;
DROP TABLE IF EXISTS plugin_api_keys;

DROP INDEX IF EXISTS idx_plugins_categories;
DROP INDEX IF EXISTS idx_plugins_enabled;
DROP TABLE IF EXISTS plugins;
