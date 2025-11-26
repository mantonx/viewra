-- Down migration: Revert adaptive transcode fields
-- WARNING: This will lose adaptive-specific data (codec, device_type, etc.)

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
-- Step 3: Restore transcode_jobs table with original quality constraint
-- ============================================================================

-- Create table with original schema (including type column from migration 000005)
CREATE TABLE transcode_jobs_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    quality TEXT NOT NULL CHECK(quality IN ('360p', '720p', '1080p', '4k')),
    type TEXT NOT NULL DEFAULT 'transcode' CHECK(type IN ('remux', 'remux_audio', 'transcode')),
    status TEXT NOT NULL CHECK(status IN ('queued', 'processing', 'completed', 'failed')),
    progress INTEGER DEFAULT 0,
    error TEXT,
    started_at DATETIME,
    completed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    file_path TEXT,
    file_size_bytes INTEGER DEFAULT 0,
    last_accessed_at DATETIME,
    access_count INTEGER DEFAULT 0,
    start_position REAL DEFAULT 0,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    UNIQUE(media_id, quality)
);

-- Copy data back, mapping adaptive qualities to legacy equivalents
-- Jobs with qualities that don't fit the legacy schema will be dropped
INSERT OR IGNORE INTO transcode_jobs_old (
    id, media_id, quality, type, status, progress, error, started_at, completed_at,
    created_at, file_path, file_size_bytes, last_accessed_at, access_count, start_position
)
SELECT
    id, media_id,
    CASE
        WHEN quality LIKE '360p%' THEN '360p'
        WHEN quality LIKE '720p%' THEN '720p'
        WHEN quality LIKE '1080p%' THEN '1080p'
        WHEN quality LIKE '4k%' OR quality LIKE '1440p%' THEN '4k'
        WHEN quality LIKE '480p%' THEN '360p'
        WHEN quality LIKE '240p%' THEN '360p'
        ELSE quality
    END,
    COALESCE(type, 'transcode'),
    status, progress, error, started_at, completed_at,
    created_at, file_path, file_size_bytes, last_accessed_at, access_count, start_position
FROM transcode_jobs
WHERE quality IN ('360p', '720p', '1080p', '4k')
   OR quality LIKE '360p-%'
   OR quality LIKE '720p-%'
   OR quality LIKE '1080p-%'
   OR quality LIKE '4k-%'
   OR quality LIKE '480p-%'
   OR quality LIKE '240p-%'
   OR quality LIKE '1440p-%';

-- Drop new table and indexes
DROP INDEX IF EXISTS idx_transcode_jobs_media_id;
DROP INDEX IF EXISTS idx_transcode_jobs_status;
DROP INDEX IF EXISTS idx_transcode_jobs_created_at;
DROP INDEX IF EXISTS idx_transcode_jobs_last_accessed;
DROP INDEX IF EXISTS idx_transcode_jobs_file_size;
DROP INDEX IF EXISTS idx_transcode_jobs_type;
DROP INDEX IF EXISTS idx_transcode_jobs_codec;
DROP INDEX IF EXISTS idx_transcode_jobs_device_type;
DROP TABLE transcode_jobs;

-- Rename old table
ALTER TABLE transcode_jobs_old RENAME TO transcode_jobs;

-- Recreate original indexes
CREATE INDEX idx_transcode_jobs_media_id ON transcode_jobs(media_id);
CREATE INDEX idx_transcode_jobs_status ON transcode_jobs(status);
CREATE INDEX idx_transcode_jobs_created_at ON transcode_jobs(created_at);
CREATE INDEX idx_transcode_jobs_last_accessed ON transcode_jobs(last_accessed_at);
CREATE INDEX idx_transcode_jobs_file_size ON transcode_jobs(file_size_bytes);
CREATE INDEX idx_transcode_jobs_type ON transcode_jobs(type);
