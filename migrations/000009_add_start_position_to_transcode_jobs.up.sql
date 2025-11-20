-- Add start_position column to transcode_jobs table for seek-based transcoding
ALTER TABLE transcode_jobs ADD COLUMN start_position INTEGER NOT NULL DEFAULT 0;
