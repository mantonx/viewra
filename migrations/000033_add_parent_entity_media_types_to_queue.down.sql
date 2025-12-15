-- Remove tv_season, music_album, music_artist media types from enrichment_queue

-- Delete queue entries for parent entity types
DELETE FROM enrichment_queue WHERE media_type IN ('tv_season', 'music_album', 'music_artist');

-- SQLite doesn't support ALTER CHECK constraint, so we need to recreate the table
CREATE TABLE enrichment_queue_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    media_type TEXT NOT NULL CHECK(media_type IN ('movie', 'tv', 'tv_show', 'music')),
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
    UNIQUE(media_id, media_type, stage)
);

-- Copy data from old table (excluding deleted types)
INSERT INTO enrichment_queue_new (id, media_id, media_type, stage, priority, status, attempts, max_attempts, error_message, error_category, next_retry_at, locked_by, locked_at, created_at, updated_at)
SELECT id, media_id, media_type, stage, priority, status, attempts, max_attempts, error_message, error_category, next_retry_at, locked_by, locked_at, created_at, updated_at
FROM enrichment_queue;

-- Drop old table
DROP TABLE enrichment_queue;

-- Rename new table
ALTER TABLE enrichment_queue_new RENAME TO enrichment_queue;

-- Recreate indexes
CREATE INDEX idx_enrichment_queue_claim
    ON enrichment_queue(stage, status, priority DESC, created_at)
    WHERE status = 'pending';

CREATE INDEX idx_enrichment_queue_locked
    ON enrichment_queue(status, locked_at)
    WHERE status = 'processing';

CREATE INDEX idx_enrichment_queue_retry
    ON enrichment_queue(status, next_retry_at)
    WHERE status = 'failed' AND next_retry_at IS NOT NULL;

CREATE INDEX idx_enrichment_queue_media_type
    ON enrichment_queue(media_type, status);
