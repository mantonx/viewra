-- Add device-specific playback preferences table
-- Preferences are stored per user + media + device profile to handle different client capabilities
-- (e.g., Firefox supports HEVC/DV natively, Chromium requires transcode)

CREATE TABLE playback_preferences (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    media_id INTEGER NOT NULL,
    device_profile TEXT NOT NULL,  -- Hash of client capabilities (codecs, HDR support, etc.)
    selected_quality TEXT,         -- Quality ID (e.g., "original", "1080p-10m", "720p-4m")
    selected_audio_track INTEGER,  -- Audio stream index
    selected_subtitle_track INTEGER, -- Subtitle track ID (-1 = off)
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    UNIQUE(user_id, media_id, device_profile)
);

CREATE INDEX idx_playback_preferences_user_media ON playback_preferences(user_id, media_id);
CREATE INDEX idx_playback_preferences_device ON playback_preferences(device_profile);
