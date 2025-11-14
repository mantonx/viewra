-- Add type column to transcode_jobs table to support different job types (remux, remux_audio, transcode)
ALTER TABLE transcode_jobs ADD COLUMN type TEXT NOT NULL DEFAULT 'transcode' CHECK(type IN ('remux', 'remux_audio', 'transcode'));

-- Create index on type column for efficient filtering
CREATE INDEX idx_transcode_jobs_type ON transcode_jobs(type);
