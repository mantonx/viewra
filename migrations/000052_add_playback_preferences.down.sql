-- SQLite doesn't support DROP COLUMN directly, so we need to recreate the table
-- This preserves existing data while removing the preference columns

CREATE TABLE watch_progress_backup (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    user_id INTEGER,
    position REAL NOT NULL DEFAULT 0,
    duration REAL,
    watched BOOLEAN DEFAULT FALSE,
    last_watched DATETIME DEFAULT CURRENT_TIMESTAMP,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    UNIQUE(media_id, user_id)
);

INSERT INTO watch_progress_backup (id, media_id, user_id, position, duration, watched, last_watched, created_at, updated_at)
SELECT id, media_id, user_id, position, duration, watched, last_watched, created_at, updated_at
FROM watch_progress;

DROP TABLE watch_progress;

ALTER TABLE watch_progress_backup RENAME TO watch_progress;

CREATE INDEX idx_watch_progress_media_id ON watch_progress(media_id);
CREATE INDEX idx_watch_progress_user_id ON watch_progress(user_id);
CREATE INDEX idx_watch_progress_watched ON watch_progress(watched);
CREATE INDEX idx_watch_progress_last_watched ON watch_progress(last_watched);
