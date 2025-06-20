-- Create unified transcode_sessions table for all transcoding providers
CREATE TABLE IF NOT EXISTS transcode_sessions (
    id VARCHAR(128) PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    -- Core fields
    provider VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL,
    request JSONB,
    progress JSONB,
    result JSONB,
    hardware JSONB,
    
    -- Time tracking
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE,
    last_accessed TIMESTAMP WITH TIME ZONE NOT NULL,
    
    -- Path information
    directory_path VARCHAR(512),
    
    -- Indexes for performance
    CONSTRAINT idx_provider_status UNIQUE (provider, status, id)
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_transcode_sessions_provider ON transcode_sessions(provider);
CREATE INDEX IF NOT EXISTS idx_transcode_sessions_status ON transcode_sessions(status);
CREATE INDEX IF NOT EXISTS idx_transcode_sessions_start_time ON transcode_sessions(start_time);
CREATE INDEX IF NOT EXISTS idx_transcode_sessions_end_time ON transcode_sessions(end_time);
CREATE INDEX IF NOT EXISTS idx_transcode_sessions_last_accessed ON transcode_sessions(last_accessed);
CREATE INDEX IF NOT EXISTS idx_transcode_sessions_deleted_at ON transcode_sessions(deleted_at);

-- Composite indexes for common queries
CREATE INDEX IF NOT EXISTS idx_transcode_sessions_status_last_accessed ON transcode_sessions(status, last_accessed);
CREATE INDEX IF NOT EXISTS idx_transcode_sessions_provider_status ON transcode_sessions(provider, status);

-- Drop old plugin-specific tables if they exist
DROP TABLE IF EXISTS ffmpeg_sessions CASCADE;
DROP TABLE IF EXISTS plugin_transcode_sessions CASCADE;
DROP TABLE IF EXISTS direct_sessions CASCADE; 