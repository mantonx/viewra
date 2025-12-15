-- Revert polymorphic external IDs back to original schema

-- Create original table structure
CREATE TABLE IF NOT EXISTS media_external_ids_old (
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    external_id TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (media_id, provider)
);

-- Migrate only entries that have a valid media_id (drop parent entity entries)
INSERT INTO media_external_ids_old (media_id, provider, external_id, created_at, updated_at)
SELECT media_id, provider, external_id, created_at, updated_at
FROM media_external_ids
WHERE media_id IS NOT NULL;

-- Drop new table and rename
DROP TABLE media_external_ids;
ALTER TABLE media_external_ids_old RENAME TO media_external_ids;

-- Recreate original index
CREATE INDEX idx_media_external_ids_lookup ON media_external_ids(provider, external_id);
