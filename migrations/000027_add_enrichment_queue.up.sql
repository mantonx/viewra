-- Enrichment Queue: persistent async job queue for media enrichment
-- Jobs are created when media is discovered and processed through enrichment stages
CREATE TABLE enrichment_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    stage TEXT NOT NULL,
    priority INTEGER DEFAULT 0,
    status TEXT DEFAULT 'pending' CHECK(status IN ('pending', 'processing', 'completed', 'failed', 'skipped')),
    attempts INTEGER DEFAULT 0,
    max_attempts INTEGER DEFAULT 3,
    error_message TEXT,
    error_category TEXT CHECK(error_category IS NULL OR error_category IN ('network', 'rate_limit', 'not_found', 'parsing', 'plugin', 'database')),
    next_retry_at TEXT,
    locked_by TEXT,
    locked_at TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    UNIQUE(media_id, stage)
);

-- Efficient claim index: pending jobs by stage, ordered by priority (desc) and age (asc)
-- Workers claim jobs: SELECT ... WHERE stage = ? AND status = 'pending' ORDER BY priority DESC, created_at LIMIT ?
CREATE INDEX idx_enrichment_queue_claim
    ON enrichment_queue(stage, status, priority DESC, created_at)
    WHERE status = 'pending';

-- Index for finding stuck jobs (locked but not updated recently)
CREATE INDEX idx_enrichment_queue_locked
    ON enrichment_queue(status, locked_at)
    WHERE status = 'processing';

-- Index for retry scheduling
CREATE INDEX idx_enrichment_queue_retry
    ON enrichment_queue(status, next_retry_at)
    WHERE status = 'failed' AND next_retry_at IS NOT NULL;

-- Enrichment Status: tracks completion status per media item per stage
-- Persists across queue job completion for UI visibility ("Show enrichment status")
CREATE TABLE enrichment_status (
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    stage TEXT NOT NULL,
    status TEXT DEFAULT 'pending' CHECK(status IN ('pending', 'processing', 'completed', 'skipped', 'failed')),
    plugin_id TEXT,
    completed_at TEXT,
    error_message TEXT,
    metadata_json TEXT,
    PRIMARY KEY (media_id, stage)
);

-- Index for stage-level progress queries ("2,341 / 5,000 movies enriched by TMDB")
CREATE INDEX idx_enrichment_status_stage ON enrichment_status(stage, status);

-- Pipeline Configuration: user-configurable enrichment stages per media type
-- Users can reorder, enable/disable, and configure stages through the UI
CREATE TABLE enrichment_pipelines (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_type TEXT NOT NULL CHECK(media_type IN ('movie', 'tv', 'tv_show', 'music')),
    plugin_id TEXT NOT NULL,
    stage_name TEXT NOT NULL,
    position INTEGER NOT NULL,
    enabled INTEGER DEFAULT 1,
    config_json TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    UNIQUE(media_type, position),
    UNIQUE(media_type, plugin_id)
);

-- Index for loading pipeline configuration by media type
CREATE INDEX idx_enrichment_pipelines_order ON enrichment_pipelines(media_type, enabled, position);

-- External IDs table: stores provider-specific IDs discovered during enrichment
-- Extensible to any provider (tmdb, imdb, tvdb, musicbrainz, etc.)
CREATE TABLE media_external_ids (
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    external_id TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (media_id, provider)
);

-- Index for reverse lookups ("find media by TMDB ID 603")
CREATE INDEX idx_media_external_ids_lookup ON media_external_ids(provider, external_id);

-- Metadata Sources: tracks which plugin provided each metadata field
-- Enables transparency ("Title from NFO", "Synopsis from TMDB")
CREATE TABLE media_metadata_sources (
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    field_name TEXT NOT NULL,
    plugin_id TEXT NOT NULL,
    raw_value TEXT,
    updated_at TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (media_id, field_name, plugin_id)
);

-- Index for querying all sources for a media item
CREATE INDEX idx_media_metadata_sources_media ON media_metadata_sources(media_id);

-- Seed default pipeline configuration for built-in enrichers
-- NFO parsing runs first (local metadata), then local image extraction
INSERT INTO enrichment_pipelines (media_type, plugin_id, stage_name, position, enabled) VALUES
    ('movie', 'builtin:nfo', 'nfo', 1, 1),
    ('movie', 'builtin:local-images', 'local-images', 2, 1),
    ('tv', 'builtin:nfo', 'nfo', 1, 1),
    ('tv', 'builtin:local-images', 'local-images', 2, 1),
    ('tv_show', 'builtin:nfo', 'nfo', 1, 1),
    ('tv_show', 'builtin:local-images', 'local-images', 2, 1),
    ('music', 'builtin:nfo', 'nfo', 1, 1),
    ('music', 'builtin:local-images', 'local-images', 2, 1);
