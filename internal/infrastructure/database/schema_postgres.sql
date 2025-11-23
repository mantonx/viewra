-- Consolidated PostgreSQL schema for sqlc code generation
-- This is built from all migrations but optimized for sqlc

CREATE TABLE IF NOT EXISTS libraries (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL CHECK(type IN ('movies', 'tv', 'music')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS media (
    id SERIAL PRIMARY KEY,
    library_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    file_path TEXT NOT NULL UNIQUE,
    file_size BIGINT,
    file_hash TEXT,
    container_format TEXT,
    duration DOUBLE PRECISION,
    width INTEGER,
    height INTEGER,
    aspect_ratio TEXT,
    codec TEXT,
    audio_codec TEXT,
    codec_profile TEXT,
    bit_rate BIGINT,
    frame_rate DOUBLE PRECISION,
    scan_type TEXT CHECK(scan_type IN ('progressive', 'interlaced')),
    hdr_format TEXT CHECK(hdr_format IN ('SDR', 'HDR10', 'HDR10+', 'Dolby Vision', 'HLG')),
    color_space TEXT,
    color_primaries TEXT,
    thumbnail_path TEXT,
    type TEXT NOT NULL CHECK(type IN ('movie', 'tv_episode', 'music_track')),
    source_type TEXT,
    resolution_label TEXT,
    quality_score INTEGER,
    is_3d BOOLEAN DEFAULT FALSE,
    stereo_mode TEXT CHECK(stereo_mode IN ('sbs', 'tab', 'mvc')),
    has_dash BOOLEAN DEFAULT FALSE,
    dash_manifest_path TEXT,
    transcoding_status TEXT CHECK(transcoding_status IN ('pending', 'processing', 'completed', 'failed')),
    is_extra BOOLEAN DEFAULT false NOT NULL,
    date_added TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    date_modified TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS watch_progress (
    id SERIAL PRIMARY KEY,
    media_id INTEGER NOT NULL,
    user_id INTEGER,
    position DOUBLE PRECISION NOT NULL DEFAULT 0,
    duration DOUBLE PRECISION,
    watched BOOLEAN DEFAULT FALSE,
    last_watched TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    UNIQUE(media_id, user_id)
);

CREATE TABLE IF NOT EXISTS scan_jobs (
    id SERIAL PRIMARY KEY,
    library_id INTEGER NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('pending', 'running', 'paused', 'completed', 'failed')),
    progress DOUBLE PRECISION DEFAULT 0,
    files_found BIGINT DEFAULT 0,
    files_processed BIGINT DEFAULT 0,
    bytes_processed BIGINT DEFAULT 0,
    error_count BIGINT DEFAULT 0,
    error_message TEXT,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    phase TEXT DEFAULT 'processing' CHECK(phase IN ('discovering', 'processing', 'completed')),
    estimated_total BIGINT DEFAULT 0,
    discovery_done BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS transcode_jobs (
    id SERIAL PRIMARY KEY,
    media_id INTEGER NOT NULL,
    quality TEXT NOT NULL CHECK(quality IN ('360p', '720p', '1080p', '4k')),
    status TEXT NOT NULL CHECK(status IN ('queued', 'processing', 'completed', 'failed')),
    progress INTEGER DEFAULT 0,
    error TEXT,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    UNIQUE(media_id, quality)
);

CREATE TABLE IF NOT EXISTS media_images (
    id SERIAL PRIMARY KEY,
    media_id INTEGER,
    media_type TEXT NOT NULL CHECK(media_type IN (
        'movie', 'tv_show', 'tv_season', 'tv_episode',
        'music_artist', 'music_album', 'music_track'
    )),
    entity_id INTEGER NOT NULL,
    image_type TEXT NOT NULL CHECK(image_type IN (
        'poster', 'fanart', 'backdrop', 'banner', 'clearlogo', 'landscape',
        'thumb', 'discart', 'cover', 'folder', 'logo',
        'actor', 'extrafanart', 'characterart', 'clearart'
    )),
    source_type TEXT NOT NULL CHECK(source_type IN (
        'local', 'tmdb', 'musicbrainz', 'tvdb', 'fanart.tv', 'manual'
    )),
    file_path TEXT,
    external_url TEXT,
    local_cache_path TEXT,
    width INTEGER,
    height INTEGER,
    file_size_bytes BIGINT,
    mime_type TEXT,
    file_hash TEXT,
    language TEXT,
    priority INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);
