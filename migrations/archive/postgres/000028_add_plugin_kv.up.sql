-- Plugin key-value storage
-- Each plugin gets an isolated namespace for caching and state
CREATE TABLE plugin_kv (
    plugin_id TEXT NOT NULL,
    key TEXT NOT NULL,
    value BYTEA NOT NULL,
    expires_at TIMESTAMPTZ,  -- NULL = never expires
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (plugin_id, key)
);

-- Index for expired entry cleanup
CREATE INDEX idx_plugin_kv_expires ON plugin_kv(expires_at) WHERE expires_at IS NOT NULL;

-- Index for listing keys by plugin
CREATE INDEX idx_plugin_kv_plugin ON plugin_kv(plugin_id);
