-- Add transcode_jobs table for tracking transcoding queue and status
-- Phase 2.2: DASH Transcoding

CREATE TABLE transcode_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    quality TEXT NOT NULL CHECK(quality IN ('360p', '720p', '1080p', '4k')),
    status TEXT NOT NULL CHECK(status IN ('queued', 'processing', 'completed', 'failed')),
    progress INTEGER DEFAULT 0,
    error_message TEXT,
    output_path TEXT,
    started_at DATETIME,
    completed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    UNIQUE(media_id, quality)
);

CREATE INDEX idx_transcode_jobs_media_id ON transcode_jobs(media_id);
CREATE INDEX idx_transcode_jobs_status ON transcode_jobs(status);
CREATE INDEX idx_transcode_jobs_created_at ON transcode_jobs(created_at);
