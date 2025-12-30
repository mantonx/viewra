-- Plugin Registry: centralized management of installed plugins
-- Tracks enabled state, health, and metadata from plugin.yml manifests
CREATE TABLE plugins (
    id TEXT PRIMARY KEY,              -- From manifest (e.g., "tmdb", "builtin:nfo")
    name TEXT NOT NULL,               -- Display name from manifest
    version TEXT NOT NULL,            -- Installed version
    description TEXT,
    author TEXT,
    license TEXT,
    homepage TEXT,
    categories TEXT NOT NULL,         -- JSON array (e.g., '["enricher"]')
    is_builtin INTEGER DEFAULT 0,     -- 1 for built-in plugins (can't uninstall)
    enabled INTEGER DEFAULT 1,        -- Global enable/disable toggle
    path TEXT,                        -- Binary path (NULL for built-ins)
    -- Health tracking (persisted across restarts)
    health_status TEXT DEFAULT 'unknown' CHECK(health_status IN ('healthy', 'degraded', 'unhealthy', 'unknown')),
    last_heartbeat TEXT,
    restart_count INTEGER DEFAULT 0,
    -- Timestamps
    installed_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Index for listing enabled plugins by type
CREATE INDEX idx_plugins_enabled ON plugins(enabled, is_builtin);

-- Index for finding plugins by category (requires JSON functions)
CREATE INDEX idx_plugins_categories ON plugins(categories);


-- Plugin API Keys: for server-to-server communication and scheduled tasks
-- Keys are scoped to the plugin's declared permissions
CREATE TABLE plugin_api_keys (
    id TEXT PRIMARY KEY,
    plugin_id TEXT NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
    key_hash TEXT NOT NULL UNIQUE,    -- SHA-256 hash of the key
    permissions TEXT NOT NULL,        -- JSON array of allowed permissions
    expires_at TEXT,                  -- NULL = never expires
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_plugin_api_keys_plugin ON plugin_api_keys(plugin_id);
CREATE INDEX idx_plugin_api_keys_expires ON plugin_api_keys(expires_at) WHERE expires_at IS NOT NULL;


-- Plugin User Metadata: per-user data storage for plugins
-- Cleaned up when users or plugins are deleted
CREATE TABLE plugin_user_metadata (
    plugin_id TEXT NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value BLOB NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (plugin_id, user_id, key)
);

CREATE INDEX idx_plugin_user_metadata_user ON plugin_user_metadata(user_id);
CREATE INDEX idx_plugin_user_metadata_plugin ON plugin_user_metadata(plugin_id);


-- Seed built-in plugins
-- These implement the Enricher interface in-process (no external binary)
INSERT INTO plugins (id, name, version, description, categories, is_builtin, enabled, path, health_status, installed_at, updated_at) VALUES
    ('builtin:nfo', 'NFO Parser', '1.0.0', 'Parse NFO metadata files for movies, TV shows, and music', '["enricher"]', 1, 1, NULL, 'healthy', datetime('now'), datetime('now')),
    ('builtin:local-images', 'Local Images', '1.0.0', 'Extract artwork from local files (poster.jpg, fanart.jpg, embedded images)', '["enricher"]', 1, 1, NULL, 'healthy', datetime('now'), datetime('now'));
