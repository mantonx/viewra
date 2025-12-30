-- ViewRA Media Server - Consolidated PostgreSQL Schema
-- Type mappings match sqlc.yaml overrides:
--   - int4/int2/serial -> int64 (matches SQLite INTEGER)
--   - bool -> int64 (SQLite uses INTEGER for booleans)
--   - jsonb/json -> string (SQLite uses TEXT)

-- ============================================================================
-- Core Tables
-- ============================================================================

CREATE TABLE libraries (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL CHECK(type IN ('movies', 'tv', 'music')),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    preferred_audio_lang TEXT DEFAULT 'eng',
    preferred_subtitle_lang TEXT DEFAULT 'eng',
    auto_enable_subtitles TEXT DEFAULT 'foreign_only' CHECK(auto_enable_subtitles IN ('always', 'foreign_only', 'never')),
    monitoring_enabled INTEGER NOT NULL DEFAULT 1,
    monitoring_config TEXT
);

CREATE INDEX idx_libraries_type ON libraries(type);
CREATE INDEX idx_libraries_path ON libraries(path);

CREATE TABLE media (
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
    codec_profile TEXT,
    bit_rate INTEGER,
    frame_rate DOUBLE PRECISION,
    scan_type TEXT CHECK(scan_type IN ('progressive', 'interlaced', NULL)),
    hdr_format TEXT CHECK(hdr_format IN ('SDR', 'HDR10', 'HDR10+', 'Dolby Vision', 'HLG', NULL)),
    color_space TEXT,
    color_primaries TEXT,
    thumbnail_path TEXT,
    type TEXT NOT NULL CHECK(type IN ('movie', 'tv_episode', 'music_track')),
    source_type TEXT,
    resolution_label TEXT,
    quality_score INTEGER,
    is_3d INTEGER DEFAULT 0,
    stereo_mode TEXT CHECK(stereo_mode IN ('sbs', 'tab', 'mvc', NULL)),
    has_dash INTEGER DEFAULT 0,
    dash_manifest_path TEXT,
    transcoding_status TEXT CHECK(transcoding_status IN ('pending', 'processing', 'completed', 'failed', NULL)),
    date_added TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    date_modified TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    is_extra INTEGER DEFAULT 0 NOT NULL,
    audio_codec TEXT,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE CASCADE
);

CREATE INDEX idx_media_library_id ON media(library_id);
CREATE INDEX idx_media_type ON media(type);
CREATE INDEX idx_media_title ON media(title);
CREATE INDEX idx_media_file_path ON media(file_path);
CREATE INDEX idx_media_file_hash ON media(file_hash);
CREATE INDEX idx_media_transcoding_status ON media(transcoding_status);
CREATE INDEX idx_media_resolution_label ON media(resolution_label);
CREATE INDEX idx_media_hdr_format ON media(hdr_format);
CREATE INDEX idx_media_date_added ON media(date_added);
CREATE INDEX idx_media_is_extra ON media(is_extra);

-- ============================================================================
-- Movies
-- ============================================================================

CREATE TABLE movies (
    media_id INTEGER PRIMARY KEY,
    year INTEGER,
    release_date DATE,
    genre TEXT,
    director TEXT,
    "cast" TEXT,
    content_rating TEXT,
    maturity_rating INTEGER,
    content_advisories TEXT,
    plot TEXT,
    tagline TEXT,
    original_title TEXT,
    sort_title TEXT,
    imdb_id TEXT,
    tmdb_id INTEGER,
    runtime_minutes INTEGER,
    budget BIGINT,
    revenue BIGINT,
    original_language TEXT,
    country_of_origin TEXT,
    awards_summary TEXT,
    rating DOUBLE PRECISION,
    rating_votes INTEGER,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

CREATE INDEX idx_movies_year ON movies(year);
CREATE INDEX idx_movies_genre ON movies(genre);
CREATE INDEX idx_movies_imdb_id ON movies(imdb_id);
CREATE INDEX idx_movies_tmdb_id ON movies(tmdb_id);
CREATE INDEX idx_movies_content_rating ON movies(content_rating);
CREATE INDEX idx_movies_sort_title ON movies(sort_title);

-- ============================================================================
-- TV Shows
-- ============================================================================

CREATE TABLE tv_shows (
    id SERIAL PRIMARY KEY,
    library_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    original_title TEXT,
    sort_title TEXT,
    year INTEGER,
    first_air_date DATE,
    last_air_date DATE,
    genre TEXT,
    plot TEXT,
    status TEXT,
    content_rating TEXT,
    maturity_rating INTEGER,
    network TEXT,
    original_language TEXT,
    country_of_origin TEXT,
    imdb_id TEXT,
    tmdb_id INTEGER,
    tvdb_id INTEGER,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    directory TEXT,
    rating DOUBLE PRECISION,
    rating_votes INTEGER,
    tagline TEXT,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE CASCADE
);

CREATE INDEX idx_tv_shows_library_id ON tv_shows(library_id);
CREATE INDEX idx_tv_shows_title ON tv_shows(title);
CREATE INDEX idx_tv_shows_sort_title ON tv_shows(sort_title);
CREATE INDEX idx_tv_shows_tvdb_id ON tv_shows(tvdb_id);
CREATE INDEX idx_tv_shows_tmdb_id ON tv_shows(tmdb_id);
CREATE INDEX idx_tv_shows_imdb_id ON tv_shows(imdb_id);
CREATE INDEX idx_tv_shows_status ON tv_shows(status);
CREATE INDEX idx_tv_shows_directory ON tv_shows(directory);
CREATE UNIQUE INDEX idx_tv_shows_library_title_unique ON tv_shows(library_id, LOWER(title));

CREATE TABLE tv_seasons (
    id SERIAL PRIMARY KEY,
    show_id INTEGER NOT NULL,
    season_number INTEGER NOT NULL,
    name TEXT,
    overview TEXT,
    air_date DATE,
    poster_path TEXT,
    episode_count INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (show_id) REFERENCES tv_shows(id) ON DELETE CASCADE,
    UNIQUE(show_id, season_number)
);

CREATE INDEX idx_tv_seasons_show_id ON tv_seasons(show_id);

CREATE TABLE tv_episodes (
    media_id INTEGER PRIMARY KEY,
    show_id INTEGER NOT NULL,
    season_id INTEGER NOT NULL,
    season_number INTEGER NOT NULL,
    episode_number INTEGER NOT NULL,
    absolute_number INTEGER,
    dvd_season INTEGER,
    dvd_episode INTEGER,
    episode_title TEXT,
    original_title TEXT,
    air_date DATE,
    plot TEXT,
    content_rating TEXT,
    maturity_rating INTEGER,
    imdb_id TEXT,
    tmdb_id INTEGER,
    tvdb_id INTEGER,
    rating DOUBLE PRECISION,
    rating_votes INTEGER,
    runtime_minutes INTEGER,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    FOREIGN KEY (show_id) REFERENCES tv_shows(id) ON DELETE CASCADE,
    FOREIGN KEY (season_id) REFERENCES tv_seasons(id) ON DELETE CASCADE,
    UNIQUE(show_id, season_number, episode_number)
);

CREATE INDEX idx_tv_episodes_show_id ON tv_episodes(show_id);
CREATE INDEX idx_tv_episodes_season_id ON tv_episodes(season_id);
CREATE INDEX idx_tv_episodes_season_number ON tv_episodes(season_number);
CREATE INDEX idx_tv_episodes_show_season ON tv_episodes(show_id, season_number);
CREATE INDEX idx_tv_episodes_absolute_number ON tv_episodes(absolute_number);
CREATE INDEX idx_tv_episodes_imdb_id ON tv_episodes(imdb_id);

-- ============================================================================
-- Music
-- ============================================================================

CREATE TABLE music_artists (
    id SERIAL PRIMARY KEY,
    library_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    sort_name TEXT,
    musicbrainz_artist_id TEXT UNIQUE,
    bio TEXT,
    country TEXT,
    formed_year INTEGER,
    genre TEXT,
    image_path TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    directory TEXT,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE CASCADE,
    UNIQUE(library_id, name)
);

CREATE INDEX idx_music_artists_library_id ON music_artists(library_id);
CREATE INDEX idx_music_artists_name ON music_artists(name);
CREATE INDEX idx_music_artists_sort_name ON music_artists(sort_name);
CREATE INDEX idx_music_artists_musicbrainz_artist_id ON music_artists(musicbrainz_artist_id);
CREATE INDEX idx_music_artists_directory ON music_artists(directory);

CREATE TABLE music_albums (
    id SERIAL PRIMARY KEY,
    library_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    album_artist TEXT,
    artist TEXT,
    year INTEGER,
    release_date DATE,
    genre TEXT,
    total_tracks INTEGER,
    total_discs INTEGER,
    record_label TEXT,
    release_type TEXT CHECK(release_type IN ('album', 'single', 'ep', 'compilation', 'live', 'remix', 'soundtrack')),
    compilation INTEGER DEFAULT 0,
    musicbrainz_album_id TEXT UNIQUE,
    cover_art_path TEXT,
    sort_title TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    artist_id INTEGER REFERENCES music_artists(id) ON DELETE SET NULL,
    directory TEXT,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE CASCADE,
    UNIQUE(library_id, title, album_artist)
);

CREATE INDEX idx_music_albums_library_id ON music_albums(library_id);
CREATE INDEX idx_music_albums_album_artist ON music_albums(album_artist);
CREATE INDEX idx_music_albums_artist ON music_albums(artist);
CREATE INDEX idx_music_albums_year ON music_albums(year);
CREATE INDEX idx_music_albums_musicbrainz_album_id ON music_albums(musicbrainz_album_id);
CREATE INDEX idx_music_albums_artist_id ON music_albums(artist_id);
CREATE INDEX idx_music_albums_directory ON music_albums(directory);

CREATE TABLE music_tracks (
    media_id INTEGER PRIMARY KEY,
    artist TEXT,
    album TEXT,
    album_artist TEXT,
    track_number INTEGER,
    disc_number INTEGER,
    total_tracks INTEGER,
    total_discs INTEGER,
    genre TEXT,
    year INTEGER,
    release_date DATE,
    composer TEXT,
    lyricist TEXT,
    record_label TEXT,
    isrc TEXT,
    release_type TEXT CHECK(release_type IN ('album', 'single', 'ep', 'compilation', 'live', 'remix', 'soundtrack')),
    compilation INTEGER DEFAULT 0,
    musicbrainz_track_id TEXT,
    musicbrainz_album_id TEXT,
    musicbrainz_artist_id TEXT,
    original_title TEXT,
    sort_title TEXT,
    album_id INTEGER REFERENCES music_albums(id) ON DELETE SET NULL,
    artist_id INTEGER REFERENCES music_artists(id) ON DELETE SET NULL,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

CREATE INDEX idx_music_tracks_artist ON music_tracks(artist);
CREATE INDEX idx_music_tracks_album ON music_tracks(album);
CREATE INDEX idx_music_tracks_album_artist ON music_tracks(album_artist);
CREATE INDEX idx_music_tracks_genre ON music_tracks(genre);
CREATE INDEX idx_music_tracks_year ON music_tracks(year);
CREATE INDEX idx_music_tracks_isrc ON music_tracks(isrc);
CREATE INDEX idx_music_tracks_musicbrainz_track_id ON music_tracks(musicbrainz_track_id);
CREATE INDEX idx_music_tracks_album_id ON music_tracks(album_id);
CREATE INDEX idx_music_tracks_artist_id ON music_tracks(artist_id);

-- ============================================================================
-- People & Credits
-- ============================================================================

CREATE TABLE people (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    sort_name TEXT,
    photo_path TEXT,
    photo_url TEXT,
    imdb_id TEXT,
    tmdb_id INTEGER,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_people_name ON people(name);
CREATE UNIQUE INDEX idx_people_tmdb_id ON people(tmdb_id) WHERE tmdb_id IS NOT NULL;
CREATE UNIQUE INDEX idx_people_imdb_id ON people(imdb_id) WHERE imdb_id IS NOT NULL;

CREATE TABLE credits (
    id SERIAL PRIMARY KEY,
    person_id INTEGER NOT NULL,
    media_type TEXT NOT NULL CHECK(media_type IN ('movie', 'tv_show', 'tv_episode', 'music_track')),
    entity_id INTEGER NOT NULL,
    credit_type TEXT NOT NULL CHECK(credit_type IN ('cast', 'director', 'writer', 'creator', 'guest', 'composer', 'producer')),
    character_name TEXT,
    department TEXT,
    job TEXT,
    billing_order INTEGER,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (person_id) REFERENCES people(id) ON DELETE CASCADE
);

CREATE INDEX idx_credits_person_id ON credits(person_id);
CREATE INDEX idx_credits_entity ON credits(media_type, entity_id);
CREATE INDEX idx_credits_type ON credits(credit_type);
CREATE INDEX idx_credits_billing ON credits(media_type, entity_id, billing_order);

CREATE TABLE studios (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    logo_path TEXT,
    tmdb_id INTEGER,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_studios_tmdb_id ON studios(tmdb_id) WHERE tmdb_id IS NOT NULL;

CREATE TABLE media_studios (
    media_type TEXT NOT NULL CHECK(media_type IN ('movie', 'tv_show')),
    entity_id INTEGER NOT NULL,
    studio_id INTEGER NOT NULL,
    PRIMARY KEY (media_type, entity_id, studio_id),
    FOREIGN KEY (studio_id) REFERENCES studios(id) ON DELETE CASCADE
);

CREATE INDEX idx_media_studios_studio ON media_studios(studio_id);

-- ============================================================================
-- Images
-- ============================================================================

CREATE TABLE media_images (
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
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

CREATE INDEX idx_media_images_media_id ON media_images(media_id);
CREATE INDEX idx_media_images_entity ON media_images(media_type, entity_id);
CREATE INDEX idx_media_images_type ON media_images(image_type);
CREATE INDEX idx_media_images_source ON media_images(source_type);
CREATE INDEX idx_media_images_hash ON media_images(file_hash) WHERE file_hash IS NOT NULL;
CREATE UNIQUE INDEX idx_media_images_unique ON media_images(media_id, image_type, priority) WHERE media_id IS NOT NULL;
CREATE UNIQUE INDEX idx_media_images_entity_unique ON media_images(media_type, entity_id, image_type, priority) WHERE media_id IS NULL;

-- ============================================================================
-- Audio & Subtitle Tracks
-- ============================================================================

CREATE TABLE media_audio_tracks (
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
    is_default INTEGER DEFAULT 0,
    is_commentary INTEGER DEFAULT 0,
    is_descriptive INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(media_id, stream_index)
);

CREATE INDEX idx_audio_tracks_media_id ON media_audio_tracks(media_id);
CREATE INDEX idx_audio_tracks_language ON media_audio_tracks(language);

CREATE TABLE media_subtitle_tracks (
    id SERIAL PRIMARY KEY,
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    stream_index INTEGER,
    source_type TEXT NOT NULL CHECK(source_type IN ('embedded', 'external')),
    codec TEXT,
    language TEXT,
    title TEXT,
    file_path TEXT,
    is_default INTEGER DEFAULT 0,
    is_forced INTEGER DEFAULT 0,
    is_sdh INTEGER DEFAULT 0,
    is_commentary INTEGER DEFAULT 0,
    is_bitmap INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_subtitle_tracks_embedded ON media_subtitle_tracks(media_id, stream_index) WHERE source_type = 'embedded';
CREATE UNIQUE INDEX idx_subtitle_tracks_external ON media_subtitle_tracks(media_id, file_path) WHERE source_type = 'external';
CREATE INDEX idx_subtitle_tracks_media_id ON media_subtitle_tracks(media_id);
CREATE INDEX idx_subtitle_tracks_language ON media_subtitle_tracks(language);

-- ============================================================================
-- Keywords
-- ============================================================================

CREATE TABLE media_keywords (
    id SERIAL PRIMARY KEY,
    media_type TEXT NOT NULL CHECK(media_type IN ('movie', 'tv_show')),
    entity_id INTEGER NOT NULL,
    keyword_id INTEGER NOT NULL,
    keyword TEXT NOT NULL,
    is_location INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(media_type, entity_id, keyword_id)
);

CREATE INDEX idx_media_keywords_entity ON media_keywords(media_type, entity_id);
CREATE INDEX idx_media_keywords_location ON media_keywords(media_type, entity_id) WHERE is_location = 1;
CREATE INDEX idx_media_keywords_keyword ON media_keywords(keyword);

-- ============================================================================
-- Users & Sessions
-- ============================================================================

CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    public_id TEXT NOT NULL UNIQUE,
    username TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    is_admin INTEGER NOT NULL DEFAULT 0,
    is_disabled INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    location_latitude DOUBLE PRECISION,
    location_longitude DOUBLE PRECISION,
    location_timezone TEXT,
    location_enabled INTEGER,
    location_name TEXT
);

CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_public_id ON users(public_id);

CREATE TABLE sessions (
    id SERIAL PRIMARY KEY,
    public_id TEXT NOT NULL UNIQUE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash TEXT NOT NULL UNIQUE,
    user_agent TEXT,
    ip_address TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

-- ============================================================================
-- Settings
-- ============================================================================

CREATE TABLE system_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    value_type TEXT NOT NULL,
    category TEXT NOT NULL,
    description TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by TEXT REFERENCES users(public_id) ON DELETE SET NULL
);

CREATE INDEX idx_system_settings_category ON system_settings(category);

CREATE TABLE user_settings (
    user_id TEXT NOT NULL REFERENCES users(public_id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, key)
);

CREATE INDEX idx_user_settings_user_id ON user_settings(user_id);

-- ============================================================================
-- Watch Progress
-- ============================================================================

CREATE TABLE watch_progress (
    id SERIAL PRIMARY KEY,
    media_id INTEGER NOT NULL,
    user_id INTEGER,
    position DOUBLE PRECISION NOT NULL DEFAULT 0,
    duration DOUBLE PRECISION,
    watched INTEGER DEFAULT 0,
    last_watched TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    selected_quality TEXT,
    selected_audio_track INTEGER,
    selected_subtitle_track INTEGER,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    UNIQUE(media_id, user_id)
);

CREATE INDEX idx_watch_progress_media_id ON watch_progress(media_id);
CREATE INDEX idx_watch_progress_user_id ON watch_progress(user_id);
CREATE INDEX idx_watch_progress_watched ON watch_progress(watched);
CREATE INDEX idx_watch_progress_last_watched ON watch_progress(last_watched);

CREATE TABLE playback_preferences (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    media_id INTEGER NOT NULL,
    device_profile TEXT NOT NULL,
    selected_quality TEXT,
    selected_audio_track INTEGER,
    selected_subtitle_track INTEGER,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    UNIQUE(user_id, media_id, device_profile)
);

CREATE INDEX idx_playback_preferences_user_media ON playback_preferences(user_id, media_id);
CREATE INDEX idx_playback_preferences_device ON playback_preferences(device_profile);

-- ============================================================================
-- Scanning
-- ============================================================================

CREATE TABLE scan_jobs (
    id SERIAL PRIMARY KEY,
    library_id INTEGER NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('pending', 'running', 'paused', 'completed', 'failed')),
    progress DOUBLE PRECISION DEFAULT 0.0,
    files_found INTEGER DEFAULT 0,
    files_processed INTEGER DEFAULT 0,
    bytes_processed BIGINT DEFAULT 0,
    error_count INTEGER DEFAULT 0,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    last_checkpoint_at TIMESTAMPTZ,
    resume_count INTEGER DEFAULT 0,
    phase TEXT DEFAULT 'processing' CHECK(phase IN ('discovering', 'processing', 'completed')),
    estimated_total INTEGER DEFAULT 0,
    discovery_done INTEGER DEFAULT 1,
    warning_count INTEGER DEFAULT 0,
    discovery_errors INTEGER DEFAULT 0,
    discovery_warnings INTEGER DEFAULT 0,
    dirs_scanned INTEGER DEFAULT 0,
    dirs_skipped INTEGER DEFAULT 0,
    files_skipped INTEGER DEFAULT 0,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE CASCADE
);

CREATE INDEX idx_scan_jobs_library_id ON scan_jobs(library_id);
CREATE INDEX idx_scan_jobs_status ON scan_jobs(status);
CREATE INDEX idx_scan_jobs_created_at ON scan_jobs(created_at);
CREATE INDEX idx_scan_jobs_started_at ON scan_jobs(started_at);

CREATE TABLE scan_checkpoints (
    id SERIAL PRIMARY KEY,
    scan_job_id INTEGER NOT NULL,
    file_path TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('pending', 'processing', 'completed', 'failed', 'warning')),
    file_size BIGINT,
    file_hash TEXT,
    error_message TEXT,
    error_category TEXT CHECK(error_category IN ('parsing', 'ffmpeg', 'database', 'filesystem', 'metadata')),
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    retry_count INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (scan_job_id) REFERENCES scan_jobs(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX idx_scan_checkpoints_job_path ON scan_checkpoints(scan_job_id, file_path);
CREATE INDEX idx_scan_checkpoints_status ON scan_checkpoints(scan_job_id, status);
CREATE INDEX idx_scan_checkpoints_failed ON scan_checkpoints(scan_job_id, status) WHERE status = 'failed';
CREATE INDEX idx_scan_checkpoints_warning ON scan_checkpoints(scan_job_id, status) WHERE status = 'warning';

CREATE TABLE scan_state (
    id SERIAL PRIMARY KEY,
    library_id INTEGER NOT NULL,
    file_path TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    file_mtime TIMESTAMPTZ NOT NULL,
    file_hash TEXT,
    media_id INTEGER,
    last_scanned_at TIMESTAMPTZ NOT NULL,
    scan_job_id INTEGER NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    has_warning INTEGER DEFAULT 0,
    warning_message TEXT,
    warning_category TEXT,
    has_error INTEGER DEFAULT 0,
    error_message TEXT,
    error_category TEXT,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE CASCADE,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE SET NULL,
    FOREIGN KEY (scan_job_id) REFERENCES scan_jobs(id)
);

CREATE UNIQUE INDEX idx_scan_state_library_path ON scan_state(library_id, file_path);
CREATE INDEX idx_scan_state_library_mtime ON scan_state(library_id, file_mtime);
CREATE INDEX idx_scan_state_media_id ON scan_state(media_id);
CREATE INDEX idx_scan_state_library_id ON scan_state(library_id);
CREATE INDEX idx_scan_state_warnings ON scan_state(library_id, has_warning) WHERE has_warning = 1;
CREATE INDEX idx_scan_state_errors ON scan_state(library_id, has_error) WHERE has_error = 1;

-- ============================================================================
-- Transcoding
-- ============================================================================

CREATE TABLE transcode_jobs (
    id SERIAL PRIMARY KEY,
    media_id INTEGER NOT NULL,
    quality TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'transcode' CHECK(type IN ('remux', 'remux_audio', 'transcode')),
    status TEXT NOT NULL CHECK(status IN ('queued', 'processing', 'completed', 'failed')),
    progress INTEGER DEFAULT 0,
    error TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    file_path TEXT,
    file_size_bytes BIGINT DEFAULT 0,
    last_accessed_at TIMESTAMPTZ,
    access_count INTEGER DEFAULT 0,
    start_position DOUBLE PRECISION DEFAULT 0,
    codec TEXT DEFAULT 'h264',
    client_device_type TEXT,
    client_network_type TEXT,
    recommended_quality TEXT,
    streaming_strategy TEXT,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    UNIQUE(media_id, quality)
);

CREATE INDEX idx_transcode_jobs_media_id ON transcode_jobs(media_id);
CREATE INDEX idx_transcode_jobs_status ON transcode_jobs(status);
CREATE INDEX idx_transcode_jobs_created_at ON transcode_jobs(created_at);
CREATE INDEX idx_transcode_jobs_last_accessed ON transcode_jobs(last_accessed_at);
CREATE INDEX idx_transcode_jobs_file_size ON transcode_jobs(file_size_bytes);
CREATE INDEX idx_transcode_jobs_type ON transcode_jobs(type);
CREATE INDEX idx_transcode_jobs_codec ON transcode_jobs(codec);
CREATE INDEX idx_transcode_jobs_device_type ON transcode_jobs(client_device_type);

CREATE TABLE transcode_analytics (
    session_id TEXT PRIMARY KEY,
    media_id INTEGER NOT NULL,
    quality_profile TEXT NOT NULL,
    strategy TEXT NOT NULL,
    hw_accel TEXT,
    ffmpeg_start_ms INTEGER,
    first_frame_ms INTEGER,
    first_segment_ms INTEGER,
    manifest_ready_ms INTEGER,
    status TEXT NOT NULL DEFAULT 'started',
    error_reason TEXT,
    total_duration_ms INTEGER,
    segments_created INTEGER,
    created_at BIGINT NOT NULL,
    completed_at BIGINT,
    strategy_reason TEXT,
    strategy_display TEXT,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

CREATE INDEX idx_transcode_analytics_session ON transcode_analytics(session_id);
CREATE INDEX idx_transcode_analytics_media ON transcode_analytics(media_id);
CREATE INDEX idx_transcode_analytics_created ON transcode_analytics(created_at);

CREATE TABLE user_video_preferences (
    id SERIAL PRIMARY KEY,
    user_identifier TEXT NOT NULL UNIQUE,
    quality_preference TEXT,
    prefer_data_saving INTEGER DEFAULT 0,
    prefer_quality INTEGER DEFAULT 0,
    allow_cellular_hd INTEGER DEFAULT 0,
    allow_cellular_4k INTEGER DEFAULT 0,
    preferred_codec TEXT,
    auto_quality_enabled INTEGER DEFAULT 1,
    max_auto_quality TEXT,
    min_auto_quality TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_user_video_preferences_user ON user_video_preferences(user_identifier);

-- ============================================================================
-- Playback Analytics
-- ============================================================================

CREATE TABLE playback_sessions (
    id SERIAL PRIMARY KEY,
    session_id TEXT UNIQUE NOT NULL,
    media_id INTEGER NOT NULL,
    start_time BIGINT NOT NULL,
    end_time BIGINT,
    total_play_time_ms INTEGER DEFAULT 0,
    total_buffer_time_ms INTEGER DEFAULT 0,
    stall_count INTEGER DEFAULT 0,
    quality_switch_count INTEGER DEFAULT 0,
    average_quality TEXT,
    device_type TEXT,
    connection_type TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    startup_time_ms INTEGER,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

CREATE INDEX idx_playback_sessions_media_id ON playback_sessions(media_id);
CREATE INDEX idx_playback_sessions_start_time ON playback_sessions(start_time);

CREATE TABLE quality_switch_events (
    id SERIAL PRIMARY KEY,
    session_id TEXT NOT NULL,
    media_id INTEGER NOT NULL,
    from_quality TEXT,
    to_quality TEXT NOT NULL,
    switch_reason TEXT NOT NULL,
    position_seconds DOUBLE PRECISION NOT NULL,
    network_speed_mbps DOUBLE PRECISION,
    buffer_seconds DOUBLE PRECISION,
    caused_stall INTEGER DEFAULT 0,
    device_type TEXT,
    connection_type TEXT,
    timestamp BIGINT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES playback_sessions(session_id) ON DELETE CASCADE,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

CREATE INDEX idx_quality_switch_events_session_id ON quality_switch_events(session_id);
CREATE INDEX idx_quality_switch_events_media_id ON quality_switch_events(media_id);
CREATE INDEX idx_quality_switch_events_timestamp ON quality_switch_events(timestamp);

CREATE TABLE playback_quality_events (
    id SERIAL PRIMARY KEY,
    transcode_job_id INTEGER,
    media_id INTEGER NOT NULL,
    user_identifier TEXT,
    from_quality TEXT,
    to_quality TEXT,
    switch_reason TEXT NOT NULL,
    position_seconds DOUBLE PRECISION,
    network_speed_mbps DOUBLE PRECISION,
    buffer_seconds DOUBLE PRECISION,
    caused_stall INTEGER DEFAULT 0,
    device_type TEXT,
    connection_type TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    FOREIGN KEY (transcode_job_id) REFERENCES transcode_jobs(id) ON DELETE SET NULL
);

CREATE INDEX idx_playback_quality_events_media ON playback_quality_events(media_id);
CREATE INDEX idx_playback_quality_events_user ON playback_quality_events(user_identifier);
CREATE INDEX idx_playback_quality_events_created ON playback_quality_events(created_at);
CREATE INDEX idx_playback_quality_events_reason ON playback_quality_events(switch_reason);

-- ============================================================================
-- Enrichment
-- ============================================================================

CREATE TABLE enrichment_queue (
    id SERIAL PRIMARY KEY,
    media_id INTEGER NOT NULL,
    media_type TEXT NOT NULL DEFAULT 'movie' CHECK(media_type IN ('movie', 'tv', 'tv_show', 'tv_season', 'music_track', 'music_album', 'music_artist')),
    stage TEXT NOT NULL,
    priority INTEGER DEFAULT 0,
    status TEXT DEFAULT 'pending' CHECK(status IN ('pending', 'processing', 'completed', 'failed', 'skipped')),
    attempts INTEGER DEFAULT 0,
    max_attempts INTEGER DEFAULT 3,
    error_message TEXT,
    error_category TEXT CHECK(error_category IS NULL OR error_category IN ('network', 'rate_limit', 'not_found', 'parsing', 'plugin', 'database')),
    next_retry_at TIMESTAMPTZ,
    locked_by TEXT,
    locked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    library_id INTEGER
);

CREATE INDEX idx_enrichment_queue_claim ON enrichment_queue(stage, status, priority DESC, created_at) WHERE status = 'pending';
CREATE INDEX idx_enrichment_queue_locked ON enrichment_queue(status, locked_at) WHERE status = 'processing';
CREATE INDEX idx_enrichment_queue_retry ON enrichment_queue(status, next_retry_at) WHERE status = 'failed' AND next_retry_at IS NOT NULL;
CREATE INDEX idx_enrichment_queue_media ON enrichment_queue(media_type, media_id);

CREATE TABLE enrichment_status (
    media_type TEXT NOT NULL CHECK(media_type IN ('movie', 'tv', 'tv_show', 'tv_season', 'music_track', 'music_album', 'music_artist')),
    media_id INTEGER NOT NULL,
    stage TEXT NOT NULL,
    status TEXT DEFAULT 'pending' CHECK(status IN ('pending', 'processing', 'completed', 'skipped', 'failed')),
    plugin_id TEXT,
    completed_at TIMESTAMPTZ,
    error_message TEXT,
    metadata_json TEXT,
    PRIMARY KEY (media_type, media_id, stage)
);

CREATE INDEX idx_enrichment_status_stage ON enrichment_status(stage, status);
CREATE INDEX idx_enrichment_status_media ON enrichment_status(media_type, media_id);

CREATE TABLE enrichment_pipelines (
    id SERIAL PRIMARY KEY,
    media_type TEXT NOT NULL CHECK(media_type IN ('movie', 'tv', 'tv_show', 'tv_season', 'music', 'music_album', 'music_artist')),
    plugin_id TEXT NOT NULL,
    stage_name TEXT NOT NULL,
    position INTEGER NOT NULL,
    enabled INTEGER DEFAULT 1,
    config_json TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(media_type, position),
    UNIQUE(media_type, plugin_id)
);

CREATE INDEX idx_enrichment_pipelines_order ON enrichment_pipelines(media_type, enabled, position);

CREATE TABLE media_external_ids (
    id SERIAL PRIMARY KEY,
    media_id INTEGER,
    media_type TEXT NOT NULL CHECK(media_type IN (
        'movie', 'tv_show', 'tv_season', 'tv_episode',
        'music_artist', 'music_album', 'music_track',
        'person', 'studio'
    )),
    entity_id INTEGER NOT NULL,
    provider TEXT NOT NULL,
    external_id TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

CREATE INDEX idx_media_external_ids_media_id ON media_external_ids(media_id) WHERE media_id IS NOT NULL;
CREATE INDEX idx_media_external_ids_entity ON media_external_ids(media_type, entity_id);
CREATE INDEX idx_media_external_ids_lookup ON media_external_ids(provider, external_id);
CREATE UNIQUE INDEX idx_media_external_ids_unique ON media_external_ids(media_type, entity_id, provider);

CREATE TABLE media_metadata_sources (
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    field_name TEXT NOT NULL,
    plugin_id TEXT NOT NULL,
    raw_value TEXT,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (media_id, field_name, plugin_id)
);

CREATE INDEX idx_media_metadata_sources_media ON media_metadata_sources(media_id);

-- ============================================================================
-- Plugins
-- ============================================================================

CREATE TABLE plugins (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    description TEXT,
    author TEXT,
    license TEXT,
    homepage TEXT,
    categories TEXT NOT NULL,
    is_builtin INTEGER DEFAULT 0,
    enabled INTEGER DEFAULT 1,
    path TEXT,
    health_status TEXT DEFAULT 'unknown' CHECK(health_status IN ('healthy', 'degraded', 'unhealthy', 'unknown')),
    last_heartbeat TIMESTAMPTZ,
    restart_count INTEGER DEFAULT 0,
    installed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    settings TEXT,
    settings_schema TEXT
);

CREATE INDEX idx_plugins_enabled ON plugins(enabled, is_builtin);
CREATE INDEX idx_plugins_categories ON plugins(categories);

CREATE TABLE plugin_api_keys (
    id TEXT PRIMARY KEY,
    plugin_id TEXT NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
    key_hash TEXT NOT NULL UNIQUE,
    permissions TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_plugin_api_keys_plugin ON plugin_api_keys(plugin_id);
CREATE INDEX idx_plugin_api_keys_expires ON plugin_api_keys(expires_at) WHERE expires_at IS NOT NULL;

CREATE TABLE plugin_kv (
    plugin_id TEXT NOT NULL,
    key TEXT NOT NULL,
    value BYTEA NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (plugin_id, key)
);

CREATE INDEX idx_plugin_kv_expires ON plugin_kv(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX idx_plugin_kv_plugin ON plugin_kv(plugin_id);

CREATE TABLE plugin_user_metadata (
    plugin_id TEXT NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (plugin_id, user_id, key)
);

CREATE INDEX idx_plugin_user_metadata_user ON plugin_user_metadata(user_id);
CREATE INDEX idx_plugin_user_metadata_plugin ON plugin_user_metadata(plugin_id);

-- ============================================================================
-- Scheduler
-- ============================================================================

CREATE TABLE scheduled_tasks (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    schedule TEXT,
    enabled INTEGER NOT NULL DEFAULT 1,
    source TEXT NOT NULL CHECK (source IN ('internal', 'plugin')),
    source_id TEXT,
    depends_on TEXT,
    timeout_seconds INTEGER DEFAULT 300,
    retry_count INTEGER DEFAULT 0,
    retry_delay_seconds INTEGER DEFAULT 60,
    concurrency_key TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE scheduler_executions (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES scheduled_tasks(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled', 'skipped', 'interrupted')),
    scheduled_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    duration_ms INTEGER,
    success INTEGER,
    error TEXT,
    logs TEXT,
    attempt INTEGER DEFAULT 1,
    parent_execution_id TEXT REFERENCES scheduler_executions(id),
    triggered_by TEXT NOT NULL CHECK (triggered_by IN ('schedule', 'manual', 'retry', 'dependency')),
    dependency_exec_id TEXT,
    resumable INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_scheduler_executions_task_id ON scheduler_executions(task_id);
CREATE INDEX idx_scheduler_executions_status ON scheduler_executions(status);
CREATE INDEX idx_scheduler_executions_started ON scheduler_executions(started_at DESC);
CREATE INDEX idx_scheduler_executions_running ON scheduler_executions(status) WHERE status IN ('pending', 'running');

CREATE TABLE scheduler_locks (
    lock_key TEXT PRIMARY KEY,
    execution_id TEXT NOT NULL,
    acquired_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_scheduler_locks_expires ON scheduler_locks(expires_at);

-- ============================================================================
-- Mood Tags (Polymorphic)
-- ============================================================================

CREATE TABLE mood_tags (
    id SERIAL PRIMARY KEY,
    media_type TEXT NOT NULL CHECK(media_type IN ('movie', 'tv_show', 'music_track', 'music_album')),
    entity_id INTEGER NOT NULL,
    mood TEXT NOT NULL,
    confidence DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    source TEXT NOT NULL DEFAULT 'ai' CHECK(source IN ('ai', 'user', 'imported')),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(media_type, entity_id, mood)
);

CREATE INDEX idx_mood_tags_entity ON mood_tags(media_type, entity_id);
CREATE INDEX idx_mood_tags_mood ON mood_tags(mood);
CREATE INDEX idx_mood_tags_source ON mood_tags(source);

-- ============================================================================
-- Embeddings (pgvector)
-- ============================================================================

-- Enable pgvector extension if available
-- CREATE EXTENSION IF NOT EXISTS vector;

-- CREATE TABLE embeddings (
--     id SERIAL PRIMARY KEY,
--     media_type TEXT NOT NULL CHECK(media_type IN ('movie', 'tv_show', 'music_track', 'music_album')),
--     entity_id INTEGER NOT NULL,
--     embedding_type TEXT NOT NULL CHECK(embedding_type IN ('content', 'semantic', 'mood')),
--     provider TEXT NOT NULL,
--     model TEXT NOT NULL,
--     embedding vector(1536),
--     metadata JSONB,
--     created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
--     updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
--     UNIQUE(media_type, entity_id, embedding_type, provider, model)
-- );
-- 
-- CREATE INDEX idx_embeddings_entity ON embeddings(media_type, entity_id);
-- CREATE INDEX idx_embeddings_type ON embeddings(embedding_type);
-- CREATE INDEX idx_embeddings_hnsw ON embeddings USING hnsw (embedding vector_cosine_ops)
--     WITH (m = 16, ef_construction = 64);

-- ============================================================================
-- Cascade Triggers for Polymorphic Tables
-- ============================================================================

-- Delete mood_tags when entity is deleted
CREATE OR REPLACE FUNCTION delete_mood_tags_on_movie_delete()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM mood_tags WHERE media_type = 'movie' AND entity_id = OLD.media_id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tr_movies_delete_mood_tags
AFTER DELETE ON movies
FOR EACH ROW EXECUTE FUNCTION delete_mood_tags_on_movie_delete();

CREATE OR REPLACE FUNCTION delete_mood_tags_on_tv_show_delete()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM mood_tags WHERE media_type = 'tv_show' AND entity_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tr_tv_shows_delete_mood_tags
AFTER DELETE ON tv_shows
FOR EACH ROW EXECUTE FUNCTION delete_mood_tags_on_tv_show_delete();

-- Delete credits when entity is deleted
CREATE OR REPLACE FUNCTION delete_credits_on_movie_delete()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM credits WHERE media_type = 'movie' AND entity_id = OLD.media_id;
    DELETE FROM media_studios WHERE media_type = 'movie' AND entity_id = OLD.media_id;
    DELETE FROM media_keywords WHERE media_type = 'movie' AND entity_id = OLD.media_id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tr_movies_delete_credits
AFTER DELETE ON movies
FOR EACH ROW EXECUTE FUNCTION delete_credits_on_movie_delete();

CREATE OR REPLACE FUNCTION delete_credits_on_tv_show_delete()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM credits WHERE media_type = 'tv_show' AND entity_id = OLD.id;
    DELETE FROM media_studios WHERE media_type = 'tv_show' AND entity_id = OLD.id;
    DELETE FROM media_keywords WHERE media_type = 'tv_show' AND entity_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tr_tv_shows_delete_credits
AFTER DELETE ON tv_shows
FOR EACH ROW EXECUTE FUNCTION delete_credits_on_tv_show_delete();

CREATE OR REPLACE FUNCTION delete_credits_on_tv_episode_delete()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM credits WHERE media_type = 'tv_episode' AND entity_id = OLD.media_id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tr_tv_episodes_delete_credits
AFTER DELETE ON tv_episodes
FOR EACH ROW EXECUTE FUNCTION delete_credits_on_tv_episode_delete();

-- Delete images when entity is deleted
CREATE OR REPLACE FUNCTION delete_images_on_movie_delete()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM media_images WHERE media_type = 'movie' AND entity_id = OLD.media_id;
    DELETE FROM media_external_ids WHERE media_type = 'movie' AND entity_id = OLD.media_id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tr_movies_delete_images
AFTER DELETE ON movies
FOR EACH ROW EXECUTE FUNCTION delete_images_on_movie_delete();

CREATE OR REPLACE FUNCTION delete_images_on_tv_show_delete()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM media_images WHERE media_type = 'tv_show' AND entity_id = OLD.id;
    DELETE FROM media_external_ids WHERE media_type = 'tv_show' AND entity_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tr_tv_shows_delete_images
AFTER DELETE ON tv_shows
FOR EACH ROW EXECUTE FUNCTION delete_images_on_tv_show_delete();

CREATE OR REPLACE FUNCTION delete_images_on_tv_season_delete()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM media_images WHERE media_type = 'tv_season' AND entity_id = OLD.id;
    DELETE FROM media_external_ids WHERE media_type = 'tv_season' AND entity_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tr_tv_seasons_delete_images
AFTER DELETE ON tv_seasons
FOR EACH ROW EXECUTE FUNCTION delete_images_on_tv_season_delete();

CREATE OR REPLACE FUNCTION delete_images_on_tv_episode_delete()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM media_images WHERE media_type = 'tv_episode' AND entity_id = OLD.media_id;
    DELETE FROM media_external_ids WHERE media_type = 'tv_episode' AND entity_id = OLD.media_id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tr_tv_episodes_delete_images
AFTER DELETE ON tv_episodes
FOR EACH ROW EXECUTE FUNCTION delete_images_on_tv_episode_delete();

CREATE OR REPLACE FUNCTION delete_images_on_music_artist_delete()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM media_images WHERE media_type = 'music_artist' AND entity_id = OLD.id;
    DELETE FROM media_external_ids WHERE media_type = 'music_artist' AND entity_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tr_music_artists_delete_images
AFTER DELETE ON music_artists
FOR EACH ROW EXECUTE FUNCTION delete_images_on_music_artist_delete();

CREATE OR REPLACE FUNCTION delete_images_on_music_album_delete()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM media_images WHERE media_type = 'music_album' AND entity_id = OLD.id;
    DELETE FROM media_external_ids WHERE media_type = 'music_album' AND entity_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tr_music_albums_delete_images
AFTER DELETE ON music_albums
FOR EACH ROW EXECUTE FUNCTION delete_images_on_music_album_delete();

CREATE OR REPLACE FUNCTION delete_images_on_music_track_delete()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM media_images WHERE media_type = 'music_track' AND entity_id = OLD.media_id;
    DELETE FROM media_external_ids WHERE media_type = 'music_track' AND entity_id = OLD.media_id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tr_music_tracks_delete_images
AFTER DELETE ON music_tracks
FOR EACH ROW EXECUTE FUNCTION delete_images_on_music_track_delete();

-- Delete external_ids when person/studio is deleted
CREATE OR REPLACE FUNCTION delete_external_ids_on_person_delete()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM media_external_ids WHERE media_type = 'person' AND entity_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tr_people_delete_external_ids
AFTER DELETE ON people
FOR EACH ROW EXECUTE FUNCTION delete_external_ids_on_person_delete();

CREATE OR REPLACE FUNCTION delete_external_ids_on_studio_delete()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM media_external_ids WHERE media_type = 'studio' AND entity_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tr_studios_delete_external_ids
AFTER DELETE ON studios
FOR EACH ROW EXECUTE FUNCTION delete_external_ids_on_studio_delete();

-- ============================================================================
-- Seed Data
-- ============================================================================

-- Default system settings
INSERT INTO system_settings (key, value, value_type, category, description, updated_at) VALUES
    ('transcoding.default_quality', '"720p"', 'string', 'transcoding', 'Default video transcoding quality', CURRENT_TIMESTAMP),
    ('transcoding.hw_accel', '"none"', 'string', 'transcoding', 'Hardware acceleration mode (none, nvenc, qsv, vaapi)', CURRENT_TIMESTAMP),
    ('scanning.watch_interval', '300', 'int', 'scanning', 'Directory watch interval in seconds', CURRENT_TIMESTAMP),
    ('scanning.ignore_patterns', '[".*", "*.nfo", "*.txt", "*.srt"]', 'json', 'scanning', 'File patterns to ignore during scanning', CURRENT_TIMESTAMP),
    ('server.base_url', '""', 'string', 'server', 'External base URL for the server', CURRENT_TIMESTAMP);

-- Default enrichment pipelines (built-in plugins)
INSERT INTO enrichment_pipelines (media_type, plugin_id, stage_name, position, enabled) VALUES
    ('movie', 'builtin:nfo', 'nfo', 1, 1),
    ('movie', 'builtin:local-images', 'local-images', 2, 1),
    ('tv', 'builtin:nfo', 'nfo', 1, 1),
    ('tv', 'builtin:local-images', 'local-images', 2, 1),
    ('tv_show', 'builtin:nfo', 'nfo', 1, 1),
    ('tv_show', 'builtin:local-images', 'local-images', 2, 1),
    ('music', 'builtin:nfo', 'nfo', 1, 1),
    ('music', 'builtin:local-images', 'local-images', 2, 1);

-- Built-in plugins
INSERT INTO plugins (id, name, version, description, categories, is_builtin, enabled, health_status, installed_at, updated_at) VALUES
    ('builtin:nfo', 'NFO Parser', '1.0.0', 'Parse NFO metadata files for movies, TV shows, and music', '["enricher"]', 1, 1, 'healthy', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('builtin:local-images', 'Local Images', '1.0.0', 'Extract artwork from local files (poster.jpg, fanart.jpg, embedded images)', '["enricher"]', 1, 1, 'healthy', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
