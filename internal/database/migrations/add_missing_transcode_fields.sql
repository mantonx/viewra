-- Add missing fields to transcode_sessions table

-- Add provider field with default value
ALTER TABLE transcode_sessions ADD COLUMN provider VARCHAR(64) DEFAULT 'unknown';
UPDATE transcode_sessions SET provider = 'unknown' WHERE provider IS NULL;

-- Add last_accessed field
ALTER TABLE transcode_sessions ADD COLUMN last_accessed DATETIME;
UPDATE transcode_sessions SET last_accessed = datetime('now') WHERE last_accessed IS NULL;

-- Add start_time field if missing
ALTER TABLE transcode_sessions ADD COLUMN start_time DATETIME;  
UPDATE transcode_sessions SET start_time = created_at WHERE start_time IS NULL;

-- Add other nullable fields
ALTER TABLE transcode_sessions ADD COLUMN request JSONB;
ALTER TABLE transcode_sessions ADD COLUMN result JSONB;
ALTER TABLE transcode_sessions ADD COLUMN hardware JSONB;
ALTER TABLE transcode_sessions ADD COLUMN directory_path VARCHAR(512); 