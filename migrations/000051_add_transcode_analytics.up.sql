-- Transcode session analytics table
-- Stores backend timing metrics for correlation with frontend playback_sessions
CREATE TABLE IF NOT EXISTS transcode_analytics (
    session_id TEXT PRIMARY KEY,
    media_id INTEGER NOT NULL,

    -- Quality and strategy info
    quality_profile TEXT NOT NULL,
    strategy TEXT NOT NULL,           -- transcode, remux, remux_audio, remux_hevc
    hw_accel TEXT,                    -- vaapi, nvenc, qsv, videotoolbox, or empty

    -- Timing breakdown (all in milliseconds)
    ffmpeg_start_ms INTEGER,          -- Time from session create to FFmpeg process start
    first_frame_ms INTEGER,           -- Time from FFmpeg start to first frame decoded
    first_segment_ms INTEGER,         -- Time from FFmpeg start to first segment written
    manifest_ready_ms INTEGER,        -- Total time until manifest is ready for playback

    -- Session outcome
    status TEXT NOT NULL DEFAULT 'started',  -- started, completed, failed
    error_reason TEXT,                -- Reason if failed (ffmpeg_error, stalled, etc.)
    total_duration_ms INTEGER,        -- Total session duration (if completed)
    segments_created INTEGER,         -- Number of segments generated

    -- Timestamps
    created_at INTEGER NOT NULL,
    completed_at INTEGER,

    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

-- Index for correlation queries with playback_sessions
CREATE INDEX IF NOT EXISTS idx_transcode_analytics_session ON transcode_analytics(session_id);
CREATE INDEX IF NOT EXISTS idx_transcode_analytics_media ON transcode_analytics(media_id);
CREATE INDEX IF NOT EXISTS idx_transcode_analytics_created ON transcode_analytics(created_at);
