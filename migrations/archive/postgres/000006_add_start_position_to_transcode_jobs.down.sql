-- Remove start_position column from transcode_jobs table
ALTER TABLE transcode_jobs DROP COLUMN IF EXISTS start_position;
