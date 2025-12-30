-- Drop index first
DROP INDEX IF EXISTS idx_transcode_jobs_type;

-- SQLite doesn't support DROP COLUMN, so we need to recreate the table
CREATE TABLE transcode_jobs_backup (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    quality TEXT NOT NULL CHECK(quality IN ('360p', '720p', '1080p', '4k')),
    status TEXT NOT NULL CHECK(status IN ('queued', 'processing', 'completed', 'failed')),
    progress INTEGER DEFAULT 0,
    error TEXT,
    started_at DATETIME,
    completed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

INSERT INTO transcode_jobs_backup (id, media_id, quality, status, progress, error, started_at, completed_at, created_at)
SELECT id, media_id, quality, status, progress, error, started_at, completed_at, created_at FROM transcode_jobs;

DROP TABLE transcode_jobs;

ALTER TABLE transcode_jobs_backup RENAME TO transcode_jobs;

-- Recreate the unique constraint
CREATE UNIQUE INDEX idx_transcode_jobs_media_quality ON transcode_jobs(media_id, quality);
