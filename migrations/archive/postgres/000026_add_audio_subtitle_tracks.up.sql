-- Add audio and subtitle track tables for multi-language support
-- ADR: 030-multi-language-audio-subtitles.md

-- Audio tracks extracted from media files
CREATE TABLE IF NOT EXISTS media_audio_tracks (
    id SERIAL PRIMARY KEY,
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    stream_index INTEGER NOT NULL,
    codec TEXT NOT NULL,
    codec_profile TEXT,
    channels INTEGER NOT NULL DEFAULT 2,
    channel_layout TEXT,
    sample_rate INTEGER,
    bit_rate INTEGER,
    language TEXT,
    title TEXT,
    is_default BOOLEAN DEFAULT FALSE,
    is_commentary BOOLEAN DEFAULT FALSE,
    is_descriptive BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(media_id, stream_index)
);

-- Subtitle tracks (embedded and external)
-- For embedded: stream_index is set, file_path is NULL
-- For external: stream_index is NULL, file_path is set
CREATE TABLE IF NOT EXISTS media_subtitle_tracks (
    id SERIAL PRIMARY KEY,
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    stream_index INTEGER,
    source_type TEXT NOT NULL CHECK(source_type IN ('embedded', 'external')),
    codec TEXT,
    language TEXT,
    title TEXT,
    file_path TEXT,
    is_default BOOLEAN DEFAULT FALSE,
    is_forced BOOLEAN DEFAULT FALSE,
    is_sdh BOOLEAN DEFAULT FALSE,
    is_commentary BOOLEAN DEFAULT FALSE,
    is_bitmap BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Unique index for embedded subtitles (by stream_index)
CREATE UNIQUE INDEX IF NOT EXISTS idx_subtitle_tracks_embedded
    ON media_subtitle_tracks(media_id, stream_index)
    WHERE source_type = 'embedded';

-- Unique index for external subtitles (by file_path)
CREATE UNIQUE INDEX IF NOT EXISTS idx_subtitle_tracks_external
    ON media_subtitle_tracks(media_id, file_path)
    WHERE source_type = 'external';

-- Library language preferences
ALTER TABLE libraries ADD COLUMN IF NOT EXISTS preferred_audio_lang TEXT DEFAULT 'eng';
ALTER TABLE libraries ADD COLUMN IF NOT EXISTS preferred_subtitle_lang TEXT DEFAULT 'eng';
ALTER TABLE libraries ADD COLUMN IF NOT EXISTS auto_enable_subtitles TEXT DEFAULT 'foreign_only'
    CHECK(auto_enable_subtitles IN ('always', 'foreign_only', 'never'));

-- Indexes for efficient lookups
CREATE INDEX IF NOT EXISTS idx_audio_tracks_media_id ON media_audio_tracks(media_id);
CREATE INDEX IF NOT EXISTS idx_audio_tracks_language ON media_audio_tracks(language);
CREATE INDEX IF NOT EXISTS idx_subtitle_tracks_media_id ON media_subtitle_tracks(media_id);
CREATE INDEX IF NOT EXISTS idx_subtitle_tracks_language ON media_subtitle_tracks(language);
