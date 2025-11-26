-- Down migration: Revert adaptive transcode fields (PostgreSQL)
-- WARNING: This will lose adaptive-specific data

-- ============================================================================
-- Step 1: Drop analytics table
-- ============================================================================

DROP INDEX IF EXISTS idx_playback_quality_events_media;
DROP INDEX IF EXISTS idx_playback_quality_events_user;
DROP INDEX IF EXISTS idx_playback_quality_events_created;
DROP INDEX IF EXISTS idx_playback_quality_events_reason;
DROP TABLE IF EXISTS playback_quality_events;

-- ============================================================================
-- Step 2: Drop user preferences table
-- ============================================================================

DROP INDEX IF EXISTS idx_user_video_preferences_user;
DROP TABLE IF EXISTS user_video_preferences;

-- ============================================================================
-- Step 3: Revert transcode_jobs table changes
-- ============================================================================

-- Drop new indexes
DROP INDEX IF EXISTS idx_transcode_jobs_codec;
DROP INDEX IF EXISTS idx_transcode_jobs_device_type;

-- Drop new columns
ALTER TABLE transcode_jobs
    DROP COLUMN IF EXISTS codec,
    DROP COLUMN IF EXISTS client_device_type,
    DROP COLUMN IF EXISTS client_network_type,
    DROP COLUMN IF EXISTS recommended_quality,
    DROP COLUMN IF EXISTS streaming_strategy;

-- Re-add the original quality constraint
-- Note: This will fail if there are rows with non-standard quality values
ALTER TABLE transcode_jobs
    ADD CONSTRAINT transcode_jobs_quality_check
    CHECK (quality IN ('360p', '720p', '1080p', '4k'));
