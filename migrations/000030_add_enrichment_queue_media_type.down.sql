-- Remove media_type column from enrichment_queue
-- Recreate the original table structure

CREATE TABLE enrichment_queue_old (
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

-- Copy data (only rows that still exist in media table due to FK constraint)
INSERT INTO enrichment_queue_old (id, media_id, stage, priority, status, attempts, max_attempts, error_message, error_category, next_retry_at, locked_by, locked_at, created_at, updated_at)
SELECT eq.id, eq.media_id, eq.stage, eq.priority, eq.status, eq.attempts, eq.max_attempts, eq.error_message, eq.error_category, eq.next_retry_at, eq.locked_by, eq.locked_at, eq.created_at, eq.updated_at
FROM enrichment_queue eq
WHERE EXISTS (SELECT 1 FROM media WHERE media.id = eq.media_id);

DROP TABLE enrichment_queue;

ALTER TABLE enrichment_queue_old RENAME TO enrichment_queue;

-- Recreate original indexes
CREATE INDEX idx_enrichment_queue_claim
    ON enrichment_queue(stage, status, priority DESC, created_at)
    WHERE status = 'pending';

CREATE INDEX idx_enrichment_queue_locked
    ON enrichment_queue(status, locked_at)
    WHERE status = 'processing';

CREATE INDEX idx_enrichment_queue_retry
    ON enrichment_queue(status, next_retry_at)
    WHERE status = 'failed' AND next_retry_at IS NOT NULL;
