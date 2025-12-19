-- SQLite doesn't support DROP COLUMN directly
-- This creates a new table without startup_time_ms and migrates data
CREATE TABLE playback_sessions_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT UNIQUE NOT NULL,
    media_id INTEGER NOT NULL,
    start_time INTEGER NOT NULL,
    end_time INTEGER,
    total_play_time_ms INTEGER DEFAULT 0,
    total_buffer_time_ms INTEGER DEFAULT 0,
    stall_count INTEGER DEFAULT 0,
    quality_switch_count INTEGER DEFAULT 0,
    average_quality TEXT,
    device_type TEXT,
    connection_type TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

INSERT INTO playback_sessions_new SELECT
    id, session_id, media_id, start_time, end_time,
    total_play_time_ms, total_buffer_time_ms, stall_count,
    quality_switch_count, average_quality, device_type,
    connection_type, created_at
FROM playback_sessions;

DROP TABLE playback_sessions;
ALTER TABLE playback_sessions_new RENAME TO playback_sessions;

CREATE INDEX idx_playback_sessions_media_id ON playback_sessions(media_id);
CREATE INDEX idx_playback_sessions_start_time ON playback_sessions(start_time);
