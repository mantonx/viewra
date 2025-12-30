-- Add system_settings table for admin-controlled configuration
CREATE TABLE system_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,              -- JSON-encoded value
    value_type TEXT NOT NULL,         -- string, int, bool, json
    category TEXT NOT NULL,           -- server, transcoding, scanning, security
    description TEXT,
    updated_at TEXT NOT NULL,
    updated_by TEXT REFERENCES users(public_id) ON DELETE SET NULL
);

-- Add user_settings table for per-user preferences
CREATE TABLE user_settings (
    user_id TEXT NOT NULL REFERENCES users(public_id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value TEXT NOT NULL,              -- JSON-encoded value
    updated_at TEXT NOT NULL,
    PRIMARY KEY (user_id, key)
);

-- Indexes for efficient lookups
CREATE INDEX idx_system_settings_category ON system_settings(category);
CREATE INDEX idx_user_settings_user_id ON user_settings(user_id);

-- Seed default system settings
INSERT INTO system_settings (key, value, value_type, category, description, updated_at) VALUES
    ('transcoding.default_quality', '"720p"', 'string', 'transcoding', 'Default video transcoding quality', datetime('now')),
    ('transcoding.hw_accel', '"none"', 'string', 'transcoding', 'Hardware acceleration mode (none, nvenc, qsv, vaapi)', datetime('now')),
    ('scanning.watch_interval', '300', 'int', 'scanning', 'Directory watch interval in seconds', datetime('now')),
    ('scanning.ignore_patterns', '[".*", "*.nfo", "*.txt", "*.srt"]', 'json', 'scanning', 'File patterns to ignore during scanning', datetime('now')),
    ('server.base_url', '""', 'string', 'server', 'External base URL for the server', datetime('now'));
