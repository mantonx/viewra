-- Recreate ai_settings table for rollback
-- Note: Data cannot be restored; AI settings must be reconfigured after rollback
CREATE TABLE IF NOT EXISTS ai_settings (
    id INTEGER PRIMARY KEY,
    key TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
