-- Widget preferences table for user home screen customization
-- Stores user-specific widget ordering and visibility settings

CREATE TABLE widget_preferences (
    id SERIAL PRIMARY KEY,
    user_id TEXT NOT NULL,
    widget_id TEXT NOT NULL,
    location TEXT NOT NULL,
    position INTEGER NOT NULL,
    hidden BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, widget_id)
);

-- Index for fast lookup by user and location
CREATE INDEX idx_widget_prefs_user_location ON widget_preferences(user_id, location);

-- Index for fast lookup by user
CREATE INDEX idx_widget_prefs_user ON widget_preferences(user_id);
