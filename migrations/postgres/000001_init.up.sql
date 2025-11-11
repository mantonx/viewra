-- Initial schema for ViewRA media server (PostgreSQL version)
-- Phase 0: Core tables (libraries, media, movies, TV shows, music, watch progress, transcode jobs)

-- ============================================================================
-- Core Tables
-- ============================================================================

-- Libraries table: Stores library definitions
CREATE TABLE libraries (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL CHECK(type IN ('movies', 'tv', 'music')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_libraries_type ON libraries(type);
CREATE INDEX idx_libraries_path ON libraries(path);

-- Media table: Base table for all media types
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
    date_added TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    date_modified TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
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

-- Movies table: Movie-specific metadata
CREATE TABLE movies (
    media_id INTEGER PRIMARY KEY,
    year INTEGER,
    release_date DATE,
    genre TEXT,
    director TEXT,
    cast TEXT,
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
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

CREATE INDEX idx_movies_year ON movies(year);
CREATE INDEX idx_movies_genre ON movies(genre);
CREATE INDEX idx_movies_imdb_id ON movies(imdb_id);
CREATE INDEX idx_movies_tmdb_id ON movies(tmdb_id);
CREATE INDEX idx_movies_content_rating ON movies(content_rating);
CREATE INDEX idx_movies_sort_title ON movies(sort_title);

-- TV Shows table: TV show series information
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
    status TEXT CHECK(status IN ('Returning Series', 'Planned', 'In Production', 'Ended', 'Cancelled', 'Pilot')),
    content_rating TEXT,
    maturity_rating INTEGER,
    network TEXT,
    original_language TEXT,
    country_of_origin TEXT,
    imdb_id TEXT,
    tmdb_id INTEGER,
    tvdb_id INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE CASCADE
);

CREATE INDEX idx_tv_shows_library_id ON tv_shows(library_id);
CREATE INDEX idx_tv_shows_title ON tv_shows(title);
CREATE INDEX idx_tv_shows_sort_title ON tv_shows(sort_title);
CREATE INDEX idx_tv_shows_tvdb_id ON tv_shows(tvdb_id);
CREATE INDEX idx_tv_shows_tmdb_id ON tv_shows(tmdb_id);
CREATE INDEX idx_tv_shows_imdb_id ON tv_shows(imdb_id);
CREATE INDEX idx_tv_shows_status ON tv_shows(status);

-- TV Seasons table: Season-level information
CREATE TABLE tv_seasons (
    id SERIAL PRIMARY KEY,
    show_id INTEGER NOT NULL,
    season_number INTEGER NOT NULL,
    name TEXT,
    overview TEXT,
    air_date DATE,
    poster_path TEXT,
    episode_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (show_id) REFERENCES tv_shows(id) ON DELETE CASCADE,
    UNIQUE(show_id, season_number)
);

CREATE INDEX idx_tv_seasons_show_id ON tv_seasons(show_id);

-- TV Episodes table: Individual episode data
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

-- Music Tracks table: Music track metadata
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
    compilation BOOLEAN DEFAULT FALSE,
    musicbrainz_track_id TEXT,
    musicbrainz_album_id TEXT,
    musicbrainz_artist_id TEXT,
    original_title TEXT,
    sort_title TEXT,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

CREATE INDEX idx_music_tracks_artist ON music_tracks(artist);
CREATE INDEX idx_music_tracks_album ON music_tracks(album);
CREATE INDEX idx_music_tracks_album_artist ON music_tracks(album_artist);
CREATE INDEX idx_music_tracks_genre ON music_tracks(genre);
CREATE INDEX idx_music_tracks_year ON music_tracks(year);
CREATE INDEX idx_music_tracks_isrc ON music_tracks(isrc);
CREATE INDEX idx_music_tracks_musicbrainz_track_id ON music_tracks(musicbrainz_track_id);

-- Watch Progress table: Tracks user watch progress
CREATE TABLE watch_progress (
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

CREATE INDEX idx_watch_progress_media_id ON watch_progress(media_id);
CREATE INDEX idx_watch_progress_user_id ON watch_progress(user_id);
CREATE INDEX idx_watch_progress_watched ON watch_progress(watched);
CREATE INDEX idx_watch_progress_last_watched ON watch_progress(last_watched);

-- Transcode Jobs table: Tracks transcoding job queue
CREATE TABLE transcode_jobs (
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

CREATE INDEX idx_transcode_jobs_media_id ON transcode_jobs(media_id);
CREATE INDEX idx_transcode_jobs_status ON transcode_jobs(status);
CREATE INDEX idx_transcode_jobs_created_at ON transcode_jobs(created_at);
