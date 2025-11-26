-- Playback analytics tables for Phase 2 adaptive streaming
-- Stores quality switch events and session metrics for optimization

-- Playback sessions track individual viewing sessions
CREATE TABLE IF NOT EXISTS playback_sessions (
    id SERIAL PRIMARY KEY,
    session_id TEXT UNIQUE NOT NULL,
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    start_time BIGINT NOT NULL,
    end_time BIGINT,
    total_play_time_ms INTEGER DEFAULT 0,
    total_buffer_time_ms INTEGER DEFAULT 0,
    stall_count INTEGER DEFAULT 0,
    quality_switch_count INTEGER DEFAULT 0,
    average_quality TEXT,
    device_type TEXT,
    connection_type TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Quality switch events record each quality change during playback
CREATE TABLE IF NOT EXISTS quality_switch_events (
    id SERIAL PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES playback_sessions(session_id) ON DELETE CASCADE,
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    from_quality TEXT,
    to_quality TEXT NOT NULL,
    switch_reason TEXT NOT NULL,
    position_seconds REAL NOT NULL,
    network_speed_mbps REAL,
    buffer_seconds REAL,
    caused_stall BOOLEAN DEFAULT FALSE,
    device_type TEXT,
    connection_type TEXT,
    timestamp BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Index for efficient queries
CREATE INDEX IF NOT EXISTS idx_playback_sessions_media_id ON playback_sessions(media_id);
CREATE INDEX IF NOT EXISTS idx_playback_sessions_start_time ON playback_sessions(start_time);
CREATE INDEX IF NOT EXISTS idx_quality_switch_events_session_id ON quality_switch_events(session_id);
CREATE INDEX IF NOT EXISTS idx_quality_switch_events_media_id ON quality_switch_events(media_id);
CREATE INDEX IF NOT EXISTS idx_quality_switch_events_timestamp ON quality_switch_events(timestamp);
