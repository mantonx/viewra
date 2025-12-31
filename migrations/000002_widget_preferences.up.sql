-- Widget preferences table for user home screen customization
-- Stores user-specific widget ordering and visibility settings

CREATE TABLE widget_preferences (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    widget_id TEXT NOT NULL,
    location TEXT NOT NULL,
    position INTEGER NOT NULL,
    hidden INTEGER DEFAULT 0,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, widget_id)
);

-- Index for fast lookup by user and location
CREATE INDEX idx_widget_prefs_user_location ON widget_preferences(user_id, location);

-- Index for fast lookup by user
CREATE INDEX idx_widget_prefs_user ON widget_preferences(user_id);
