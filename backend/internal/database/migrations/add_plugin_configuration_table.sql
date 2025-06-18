-- Migration: Add plugin_configurations table for enhanced plugin configuration management
-- This table stores plugin configuration schemas and settings for the admin panel

-- Create plugin_configurations table
CREATE TABLE IF NOT EXISTS plugin_configurations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plugin_id VARCHAR(255) NOT NULL UNIQUE,
    schema_data TEXT,
    settings_data TEXT,
    version VARCHAR(50) NOT NULL,
    modified_by VARCHAR(255),
    dependencies TEXT,
    permissions TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create index for fast plugin lookups
CREATE INDEX IF NOT EXISTS idx_plugin_configurations_plugin_id ON plugin_configurations(plugin_id);
CREATE INDEX IF NOT EXISTS idx_plugin_configurations_is_active ON plugin_configurations(is_active);

-- Insert default configurations for existing plugins if they don't exist
INSERT OR IGNORE INTO plugin_configurations (plugin_id, schema_data, settings_data, version, modified_by)
SELECT 
    p.plugin_id,
    '{"version": "1.0", "title": "Plugin Configuration", "description": "Configuration for ' || p.name || '", "properties": {}}' as schema_data,
    '{}' as settings_data,
    p.version,
    'system'
FROM plugins p 
WHERE p.status = 'enabled'; 