-- Migration: Add adaptive transcode fields (PostgreSQL)
-- Purpose: Support ADR 0003 Adaptive Video Transcoding System with granular quality profiles

-- ============================================================================
-- Step 1: Update transcode_jobs table to support adaptive quality profiles
-- ============================================================================

-- Remove the CHECK constraint on quality column to allow granular profiles
-- PostgreSQL allows modifying constraints without recreating the table

-- First, drop the existing constraint
ALTER TABLE transcode_jobs DROP CONSTRAINT IF EXISTS transcode_jobs_quality_check;

-- Add new columns for adaptive transcoding
ALTER TABLE transcode_jobs
    ADD COLUMN IF NOT EXISTS codec VARCHAR(10) DEFAULT 'h264',
    ADD COLUMN IF NOT EXISTS client_device_type VARCHAR(20),
    ADD COLUMN IF NOT EXISTS client_network_type VARCHAR(20),
    ADD COLUMN IF NOT EXISTS recommended_quality VARCHAR(20),
    ADD COLUMN IF NOT EXISTS streaming_strategy VARCHAR(20);

-- Create indexes on new columns
CREATE INDEX IF NOT EXISTS idx_transcode_jobs_codec ON transcode_jobs(codec);
CREATE INDEX IF NOT EXISTS idx_transcode_jobs_device_type ON transcode_jobs(client_device_type);

-- ============================================================================
-- Step 2: Create user_video_preferences table
-- ============================================================================

CREATE TABLE IF NOT EXISTS user_video_preferences (
    id BIGSERIAL PRIMARY KEY,
    -- Unique identifier for the user/client
    user_identifier VARCHAR(255) NOT NULL UNIQUE,
    -- Quality preference: specific quality like "1080p-8000k" or resolution like "1080p"
    quality_preference VARCHAR(20),
    -- User prefers to save data (bias toward lower quality)
    prefer_data_saving BOOLEAN DEFAULT FALSE,
    -- User prefers quality over bandwidth (bias toward higher quality)
    prefer_quality BOOLEAN DEFAULT FALSE,
    -- Allow HD streaming on cellular/metered connections
    allow_cellular_hd BOOLEAN DEFAULT FALSE,
    -- Allow 4K streaming on cellular/metered connections
    allow_cellular_4k BOOLEAN DEFAULT FALSE,
    -- Preferred codec if device supports it (h264, h265, vp9, av1)
    preferred_codec VARCHAR(10),
    -- Auto quality mode enabled
    auto_quality_enabled BOOLEAN DEFAULT TRUE,
    -- Maximum quality to auto-select (null means no limit)
    max_auto_quality VARCHAR(20),
    -- Minimum quality to auto-select (null means no limit)
    min_auto_quality VARCHAR(20),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_user_video_preferences_user ON user_video_preferences(user_identifier);

-- ============================================================================
-- Step 3: Create playback_quality_events table for analytics
-- ============================================================================

CREATE TABLE IF NOT EXISTS playback_quality_events (
    id BIGSERIAL PRIMARY KEY,
    -- Reference to the transcode job or playback session
    transcode_job_id BIGINT REFERENCES transcode_jobs(id) ON DELETE SET NULL,
    media_id BIGINT NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    -- User/client identifier
    user_identifier VARCHAR(255),
    -- Quality before switch (null for initial quality)
    from_quality VARCHAR(20),
    -- Quality after switch
    to_quality VARCHAR(20),
    -- Reason for switch: 'auto_bandwidth', 'auto_buffer', 'user_manual', 'initial'
    switch_reason VARCHAR(20) NOT NULL,
    -- Playback position when switch occurred (seconds)
    position_seconds REAL,
    -- Network speed at time of switch (Mbps)
    network_speed_mbps REAL,
    -- Buffer health at time of switch (seconds of buffered content)
    buffer_seconds REAL,
    -- Whether the switch caused a visible stall
    caused_stall BOOLEAN DEFAULT FALSE,
    -- Device type at time of event
    device_type VARCHAR(20),
    -- Connection type at time of event
    connection_type VARCHAR(20),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_playback_quality_events_media ON playback_quality_events(media_id);
CREATE INDEX IF NOT EXISTS idx_playback_quality_events_user ON playback_quality_events(user_identifier);
CREATE INDEX IF NOT EXISTS idx_playback_quality_events_created ON playback_quality_events(created_at);
CREATE INDEX IF NOT EXISTS idx_playback_quality_events_reason ON playback_quality_events(switch_reason);
