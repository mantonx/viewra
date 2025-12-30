-- AI Settings table for storing provider configuration
-- API keys are stored encrypted (encryption handled at application layer)
CREATE TABLE IF NOT EXISTS ai_settings (
    id INTEGER PRIMARY KEY,
    key TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- AI Usage tracking (token counts only, no cost estimates as pricing changes frequently)
CREATE TABLE IF NOT EXISTS ai_usage (
    id INTEGER PRIMARY KEY,
    user_id TEXT NOT NULL,
    date TEXT NOT NULL,  -- YYYY-MM-DD format
    prompt_tokens INTEGER DEFAULT 0,
    completion_tokens INTEGER DEFAULT 0,
    embedding_tokens INTEGER DEFAULT 0,
    request_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, date)
);

-- Mood tags for media items
CREATE TABLE IF NOT EXISTS media_mood_tags (
    id INTEGER PRIMARY KEY,
    media_id INTEGER NOT NULL,
    tag TEXT NOT NULL,
    confidence REAL DEFAULT 1.0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_media_mood_tags_media_id ON media_mood_tags(media_id);
CREATE INDEX IF NOT EXISTS idx_media_mood_tags_tag ON media_mood_tags(tag);
CREATE INDEX IF NOT EXISTS idx_ai_usage_user_date ON ai_usage(user_id, date);
