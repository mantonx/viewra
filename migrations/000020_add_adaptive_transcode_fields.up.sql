-- Migration: Add adaptive transcode fields
-- Purpose: Support ADR 0003 Adaptive Video Transcoding System with granular quality profiles

-- ============================================================================
-- Step 1: Update transcode_jobs table to support adaptive quality profiles
-- ============================================================================

-- SQLite doesn't support ALTER COLUMN, so we need to:
-- 1. Create new table with updated schema
-- 2. Copy data
-- 3. Drop old table
-- 4. Rename new table

-- Create new transcode_jobs table with expanded quality support
CREATE TABLE transcode_jobs_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    -- Expanded quality field to support adaptive profiles like "1080p-8000k", "4k-40000k"
    quality TEXT NOT NULL,
    -- Job type from migration 000005 (remux, remux_audio, transcode)
    type TEXT NOT NULL DEFAULT 'transcode' CHECK(type IN ('remux', 'remux_audio', 'transcode')),
    status TEXT NOT NULL CHECK(status IN ('queued', 'processing', 'completed', 'failed')),
    progress INTEGER DEFAULT 0,
    error TEXT,
    started_at DATETIME,
    completed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    -- Existing tracking fields from migration 000006
    file_path TEXT,
    file_size_bytes INTEGER DEFAULT 0,
    last_accessed_at DATETIME,
    access_count INTEGER DEFAULT 0,
    -- Existing start position from migration 000009
    start_position REAL DEFAULT 0,
    -- New adaptive transcode fields
    codec TEXT DEFAULT 'h264',
    client_device_type TEXT,
    client_network_type TEXT,
    recommended_quality TEXT,
    -- Streaming strategy used (DirectPlay, Remux, RemuxWithAudio, Transcode)
    streaming_strategy TEXT,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    UNIQUE(media_id, quality)
);

-- Copy existing data (map old quality values to new format)
INSERT INTO transcode_jobs_new (
    id, media_id, quality, type, status, progress, error, started_at, completed_at,
    created_at, file_path, file_size_bytes, last_accessed_at, access_count, start_position
)
SELECT
    id, media_id, quality, COALESCE(type, 'transcode'), status, progress, error, started_at, completed_at,
    created_at, file_path, file_size_bytes, last_accessed_at, access_count,
    COALESCE(start_position, 0)
FROM transcode_jobs;

-- Drop old table and its indexes
DROP INDEX IF EXISTS idx_transcode_jobs_media_id;
DROP INDEX IF EXISTS idx_transcode_jobs_status;
DROP INDEX IF EXISTS idx_transcode_jobs_created_at;
DROP INDEX IF EXISTS idx_transcode_jobs_last_accessed;
DROP INDEX IF EXISTS idx_transcode_jobs_file_size;
DROP INDEX IF EXISTS idx_transcode_jobs_type;
DROP TABLE transcode_jobs;

-- Rename new table
ALTER TABLE transcode_jobs_new RENAME TO transcode_jobs;

-- Recreate indexes
CREATE INDEX idx_transcode_jobs_media_id ON transcode_jobs(media_id);
CREATE INDEX idx_transcode_jobs_status ON transcode_jobs(status);
CREATE INDEX idx_transcode_jobs_created_at ON transcode_jobs(created_at);
CREATE INDEX idx_transcode_jobs_last_accessed ON transcode_jobs(last_accessed_at);
CREATE INDEX idx_transcode_jobs_file_size ON transcode_jobs(file_size_bytes);
CREATE INDEX idx_transcode_jobs_type ON transcode_jobs(type);
CREATE INDEX idx_transcode_jobs_codec ON transcode_jobs(codec);
CREATE INDEX idx_transcode_jobs_device_type ON transcode_jobs(client_device_type);

-- ============================================================================
-- Step 2: Create user_video_preferences table
-- ============================================================================

-- Stores user preferences for video quality recommendations
-- Note: user_id is a simple identifier since there's no users table yet
-- When a users table is added, a migration can add the foreign key constraint
CREATE TABLE user_video_preferences (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    -- Unique identifier for the user/client (could be session ID, device ID, or future user ID)
    user_identifier TEXT NOT NULL UNIQUE,
    -- Quality preference: specific quality like "1080p-8000k" or resolution like "1080p"
    quality_preference TEXT,
    -- User prefers to save data (bias toward lower quality)
    prefer_data_saving BOOLEAN DEFAULT 0,
    -- User prefers quality over bandwidth (bias toward higher quality)
    prefer_quality BOOLEAN DEFAULT 0,
    -- Allow HD streaming on cellular/metered connections
    allow_cellular_hd BOOLEAN DEFAULT 0,
    -- Allow 4K streaming on cellular/metered connections
    allow_cellular_4k BOOLEAN DEFAULT 0,
    -- Preferred codec if device supports it (h264, h265, vp9, av1)
    preferred_codec TEXT,
    -- Auto quality mode enabled
    auto_quality_enabled BOOLEAN DEFAULT 1,
    -- Maximum quality to auto-select (null means no limit)
    max_auto_quality TEXT,
    -- Minimum quality to auto-select (null means no limit)
    min_auto_quality TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_user_video_preferences_user ON user_video_preferences(user_identifier);

-- ============================================================================
-- Step 3: Create playback_quality_events table for analytics
-- ============================================================================

-- Tracks quality switch events during playback for analytics and optimization
CREATE TABLE playback_quality_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    -- Reference to the transcode job or playback session
    transcode_job_id INTEGER,
    media_id INTEGER NOT NULL,
    -- User/client identifier
    user_identifier TEXT,
    -- Quality before switch (null for initial quality)
    from_quality TEXT,
    -- Quality after switch
    to_quality TEXT,
    -- Reason for switch: 'auto_bandwidth', 'auto_buffer', 'user_manual', 'initial'
    switch_reason TEXT NOT NULL,
    -- Playback position when switch occurred (seconds)
    position_seconds REAL,
    -- Network speed at time of switch (Mbps)
    network_speed_mbps REAL,
    -- Buffer health at time of switch (seconds of buffered content)
    buffer_seconds REAL,
    -- Whether the switch caused a visible stall
    caused_stall BOOLEAN DEFAULT 0,
    -- Device type at time of event
    device_type TEXT,
    -- Connection type at time of event
    connection_type TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    FOREIGN KEY (transcode_job_id) REFERENCES transcode_jobs(id) ON DELETE SET NULL
);

CREATE INDEX idx_playback_quality_events_media ON playback_quality_events(media_id);
CREATE INDEX idx_playback_quality_events_user ON playback_quality_events(user_identifier);
CREATE INDEX idx_playback_quality_events_created ON playback_quality_events(created_at);
CREATE INDEX idx_playback_quality_events_reason ON playback_quality_events(switch_reason);
