# ViewRA Database Schema

## Overview

ViewRA uses a **hybrid schema design** that combines a base `media` table for common fields with type-specific tables for movies, TV shows, and music. This approach provides:

- Type safety and clear data modeling
- Flexibility for type-specific fields
- Efficient querying with proper indexes
- Easy migration between SQLite and PostgreSQL

## Schema Design Philosophy

### Hybrid Approach

**Base Table** (`media`):
- Common fields shared across all media types
- File path, size, duration, codec info
- References to library
- Transcoding status

**Type-Specific Tables**:
- `movies` - Movie-specific metadata
- `tv_shows` - TV show series information
- `tv_episodes` - Episode-specific data
- `music_tracks` - Music track metadata

**Benefits**:
- ✅ No nullable columns for type-specific data
- ✅ Clear separation of concerns
- ✅ Type-safe queries via sqlc
- ✅ Easy to add new media types

## Core Tables

### Libraries Table

Stores library definitions (Movies, TV Shows, Music collections).

```sql
CREATE TABLE libraries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL CHECK(type IN ('movies', 'tv', 'music')),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_libraries_type ON libraries(type);
CREATE INDEX idx_libraries_path ON libraries(path);
```

**Fields**:
- `id` - Primary key
- `name` - Display name (e.g., "My Movies")
- `path` - Absolute file system path to scan
- `type` - Library type: `movies`, `tv`, or `music`
- `created_at` - Creation timestamp
- `updated_at` - Last update timestamp

**Constraints**:
- `path` must be unique (can't have duplicate library paths)
- `type` must be one of the allowed values

---

### Media Table (Base)

Common fields for all media types.

```sql
CREATE TABLE media (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    library_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    file_path TEXT NOT NULL UNIQUE,
    file_size INTEGER,
    file_hash TEXT,
    container_format TEXT,
    duration REAL,
    width INTEGER,
    height INTEGER,
    aspect_ratio TEXT,
    codec TEXT,
    codec_profile TEXT,
    bit_rate INTEGER,
    frame_rate REAL,
    scan_type TEXT CHECK(scan_type IN ('progressive', 'interlaced', NULL)),
    hdr_format TEXT CHECK(hdr_format IN ('SDR', 'HDR10', 'HDR10+', 'Dolby Vision', 'HLG', NULL)),
    color_space TEXT,
    color_primaries TEXT,
    thumbnail_path TEXT,
    type TEXT NOT NULL CHECK(type IN ('movie', 'tv_episode', 'music_track')),
    source_type TEXT,
    resolution_label TEXT,
    quality_score INTEGER,
    is_3d BOOLEAN DEFAULT 0,
    stereo_mode TEXT CHECK(stereo_mode IN ('sbs', 'tab', 'mvc', NULL)),
    has_dash BOOLEAN DEFAULT FALSE,
    dash_manifest_path TEXT,
    transcoding_status TEXT CHECK(transcoding_status IN ('pending', 'processing', 'completed', 'failed', NULL)),
    date_added DATETIME DEFAULT CURRENT_TIMESTAMP,
    date_modified DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
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
```

**Fields**:
- `id` - Primary key
- `library_id` - Reference to parent library
- `title` - Display title (from filename or metadata)
- `file_path` - Absolute path to media file
- `file_size` - File size in bytes
- `file_hash` - SHA256/MD5 hash for duplicate detection
- `container_format` - MKV, MP4, AVI, FLAC, MP3, etc.
- `duration` - Duration in seconds
- `width` / `height` - Video dimensions in pixels
- `aspect_ratio` - Display aspect ratio (16:9, 2.35:1, 4:3, etc.)
- `codec` - Video/audio codec (H.264, HEVC, AAC, etc.)
- `codec_profile` - Codec profile (High, Main, Baseline)
- `bit_rate` - Bitrate in bits per second
- `frame_rate` - FPS for video
- `scan_type` - progressive or interlaced
- `hdr_format` - SDR, HDR10, HDR10+, Dolby Vision, HLG
- `color_space` - Color space (BT.709, BT.2020, etc.)
- `color_primaries` - Color primaries specification
- `thumbnail_path` - Path to generated thumbnail
- `type` - Media type discriminator (movie, tv_episode, music_track)
- `source_type` - BluRay, WEB-DL, HDTV, DVD-Rip, etc.
- `resolution_label` - 480p, 720p, 1080p, 4K, 8K
- `quality_score` - Numeric quality score for duplicate management (0-100)
- `is_3d` - Whether this is 3D content
- `stereo_mode` - 3D format: sbs (side-by-side), tab (top-and-bottom), mvc
- `has_dash` - Whether DASH version exists
- `dash_manifest_path` - Path to .mpd manifest
- `transcoding_status` - Current transcode state
- `date_added` - When file was first scanned/added
- `date_modified` - File system modification timestamp
- `created_at` / `updated_at` - Database record timestamps

**Indexes**:
- `library_id` - Fast library filtering
- `type` - Fast type filtering
- `title` - Search optimization
- `file_path` - Unique constraint + lookups
- `file_hash` - Duplicate detection
- `transcoding_status` - Queue queries
- `resolution_label` - Filter by quality
- `hdr_format` - Filter HDR content
- `date_added` - Recently added sorting

---

### Movies Table

Movie-specific metadata.

```sql
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
```

**Fields**:
- `media_id` - Primary key, references `media.id`
- `year` - Release year
- `release_date` - Full release date
- `genre` - Genre(s), comma-separated (deprecated in favor of normalized genres table)
- `director` - Director name(s) (deprecated in favor of movie_credits)
- `cast` - Cast members (deprecated in favor of movie_credits)
- `content_rating` - MPAA/regional rating (G, PG, PG-13, R, NC-17, etc.)
- `maturity_rating` - Numeric maturity level (0-18+)
- `content_advisories` - JSON array of advisories (violence, language, sexual content, etc.)
- `plot` - Plot summary
- `tagline` - Movie tagline/slogan
- `original_title` - Title in original language
- `sort_title` - Title for alphabetical sorting (e.g., "Matrix, The")
- `imdb_id` - IMDb identifier (e.g., tt0133093)
- `tmdb_id` - The Movie Database ID
- `runtime_minutes` - Official runtime in minutes
- `budget` - Production budget in USD
- `revenue` - Box office revenue in USD
- `original_language` - ISO 639-1 language code
- `country_of_origin` - ISO 3166-1 country code
- `awards_summary` - Quick summary (e.g., "Won 4 Oscars")

**Indexes**:
- `year` - Filter by release year
- `genre` - Filter by genre
- `imdb_id` - External API lookups
- `tmdb_id` - External API lookups
- `content_rating` - Parental controls filtering
- `sort_title` - Alphabetical browsing

---

### TV Shows Table

TV show series information.

```sql
CREATE TABLE tv_shows (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
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
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE CASCADE
);

CREATE INDEX idx_tv_shows_library_id ON tv_shows(library_id);
CREATE INDEX idx_tv_shows_title ON tv_shows(title);
CREATE INDEX idx_tv_shows_sort_title ON tv_shows(sort_title);
CREATE INDEX idx_tv_shows_tvdb_id ON tv_shows(tvdb_id);
CREATE INDEX idx_tv_shows_tmdb_id ON tv_shows(tmdb_id);
CREATE INDEX idx_tv_shows_imdb_id ON tv_shows(imdb_id);
CREATE INDEX idx_tv_shows_status ON tv_shows(status);
```

**Fields**:
- `id` - Primary key
- `library_id` - Reference to TV library
- `title` - Show name
- `original_title` - Title in original language
- `sort_title` - Title for alphabetical sorting
- `year` - First air year
- `first_air_date` - Premiere date
- `last_air_date` - Most recent episode date
- `genre` - Genre(s), comma-separated
- `plot` - Show description
- `status` - Returning Series, Ended, Cancelled, etc.
- `content_rating` - TV rating (TV-Y, TV-PG, TV-14, TV-MA, etc.)
- `maturity_rating` - Numeric maturity level (0-18+)
- `network` - Broadcasting network/service (NBC, Netflix, HBO)
- `original_language` - ISO 639-1 language code
- `country_of_origin` - ISO 3166-1 country code
- `imdb_id` - IMDb identifier
- `tmdb_id` - The Movie Database ID
- `tvdb_id` - TheTVDB identifier
- `created_at` / `updated_at` - Timestamps

**Indexes**:
- `library_id` - Get shows in library
- `title` - Search by title
- `sort_title` - Alphabetical browsing
- `tvdb_id`, `tmdb_id`, `imdb_id` - External API lookups
- `status` - Filter by show status

---

### TV Seasons Table

Season-level information for TV shows.

```sql
CREATE TABLE tv_seasons (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    show_id INTEGER NOT NULL,
    season_number INTEGER NOT NULL,
    name TEXT,
    overview TEXT,
    air_date DATE,
    poster_path TEXT,
    episode_count INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (show_id) REFERENCES tv_shows(id) ON DELETE CASCADE,
    UNIQUE(show_id, season_number)
);

CREATE INDEX idx_tv_seasons_show_id ON tv_seasons(show_id);
```

**Fields**:
- `id` - Primary key
- `show_id` - Reference to parent show
- `season_number` - Season number (0 for specials)
- `name` - Season name (e.g., "Season 1" or custom)
- `overview` - Season description
- `air_date` - Season premiere date
- `poster_path` - Season-specific poster
- `episode_count` - Number of episodes
- `created_at` / `updated_at` - Timestamps

**Constraints**:
- Unique `(show_id, season_number)` - One season record per show/season

---

### TV Episodes Table

Individual episode data.

```sql
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
```

**Fields**:
- `media_id` - Primary key, references `media.id`
- `show_id` - Reference to parent show
- `season_id` - Reference to parent season
- `season_number` - Season number (aired order, 0 for specials)
- `episode_number` - Episode number within season (aired order)
- `absolute_number` - Absolute episode number (useful for anime)
- `dvd_season` - Season number on DVD release
- `dvd_episode` - Episode number on DVD release
- `episode_title` - Episode name
- `original_title` - Title in original language
- `air_date` - Original air date
- `plot` - Episode description
- `content_rating` - Episode-specific rating (if different from show)
- `maturity_rating` - Numeric maturity level
- `imdb_id` - IMDb identifier
- `tmdb_id` - The Movie Database ID
- `tvdb_id` - TheTVDB identifier

**Constraints**:
- Unique constraint on `(show_id, season_number, episode_number)` - no duplicate episodes

**Indexes**:
- `show_id` - Get all episodes for a show
- `season_id` - Get all episodes for a season
- `season_number` - Filter by season
- `(show_id, season_number)` - Composite for season queries
- `absolute_number` - Anime/absolute numbering lookups
- `imdb_id` - External API lookups

---

### Music Tracks Table

Music track metadata.

```sql
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
    compilation BOOLEAN DEFAULT 0,
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
```

**Fields**:
- `media_id` - Primary key, references `media.id`
- `artist` - Track artist (from ID3 tags)
- `album` - Album name
- `album_artist` - Album artist (may differ from track artist)
- `track_number` - Track number within disc
- `disc_number` - Disc number for multi-disc albums
- `total_tracks` - Total tracks on disc
- `total_discs` - Total discs in album
- `genre` - Music genre
- `year` - Release year
- `release_date` - Full release date
- `composer` - Composer name
- `lyricist` - Lyricist/songwriter
- `record_label` - Publishing label
- `isrc` - International Standard Recording Code
- `release_type` - album, single, ep, compilation, live, remix, soundtrack
- `compilation` - Is this a compilation album track
- `musicbrainz_track_id` - MusicBrainz recording ID
- `musicbrainz_album_id` - MusicBrainz release ID
- `musicbrainz_artist_id` - MusicBrainz artist ID
- `original_title` - Title in original language
- `sort_title` - Title for alphabetical sorting

**Indexes**:
- `artist` - Browse by artist
- `album` - Browse by album
- `album_artist` - Group by album artist
- `genre` - Filter by genre
- `year` - Filter by year
- `isrc` - External lookups
- `musicbrainz_track_id` - External API lookups

---

### Watch Progress Table

Tracks user watch progress and playback position.

```sql
CREATE TABLE watch_progress (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    user_id INTEGER, -- NULL for single-user, future multi-user support
    position REAL NOT NULL DEFAULT 0,
    duration REAL,
    watched BOOLEAN DEFAULT FALSE,
    last_watched DATETIME DEFAULT CURRENT_TIMESTAMP,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    UNIQUE(media_id, user_id)
);

CREATE INDEX idx_watch_progress_media_id ON watch_progress(media_id);
CREATE INDEX idx_watch_progress_user_id ON watch_progress(user_id);
CREATE INDEX idx_watch_progress_watched ON watch_progress(watched);
CREATE INDEX idx_watch_progress_last_watched ON watch_progress(last_watched);
```

**Fields**:
- `id` - Primary key
- `media_id` - Reference to media
- `user_id` - User identifier (NULL for single-user)
- `position` - Current playback position in seconds
- `duration` - Total duration (cached for convenience)
- `watched` - Mark as watched (true if >90% complete)
- `last_watched` - Last playback timestamp
- `created_at` / `updated_at` - Timestamps

**Constraints**:
- Unique `(media_id, user_id)` - One progress record per user per media

**Indexes**:
- `media_id` - Get progress for media
- `user_id` - Get all progress for user
- `watched` - Filter watched/unwatched
- `last_watched` - Sort by recently watched

---

### Transcode Jobs Table

Tracks transcoding job queue and status.

```sql
CREATE TABLE transcode_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    quality TEXT NOT NULL CHECK(quality IN ('360p', '720p', '1080p', '4k')),
    status TEXT NOT NULL CHECK(status IN ('queued', 'processing', 'completed', 'failed')),
    progress INTEGER DEFAULT 0,
    error TEXT,
    started_at DATETIME,
    completed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    UNIQUE(media_id, quality)
);

CREATE INDEX idx_transcode_jobs_media_id ON transcode_jobs(media_id);
CREATE INDEX idx_transcode_jobs_status ON transcode_jobs(status);
CREATE INDEX idx_transcode_jobs_created_at ON transcode_jobs(created_at);
```

**Fields**:
- `id` - Primary key
- `media_id` - Reference to media being transcoded
- `quality` - Target quality profile
- `status` - Job status
- `progress` - Percentage complete (0-100)
- `error` - Error message if failed
- `started_at` - When processing began
- `completed_at` - When job finished
- `created_at` - When job was queued

**Constraints**:
- Unique `(media_id, quality)` - One job per quality per media

**Indexes**:
- `media_id` - Get all jobs for media
- `status` - Find queued/processing jobs
- `created_at` - FIFO queue ordering

---

### Metadata Cache Table

Stores full external API responses for metadata enrichment (Phase 2).

```sql
CREATE TABLE metadata_cache (
    media_id INTEGER PRIMARY KEY,
    source TEXT NOT NULL,
    data TEXT NOT NULL,
    confidence REAL,
    fetched_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

CREATE INDEX idx_metadata_cache_source ON metadata_cache(source);
CREATE INDEX idx_metadata_cache_fetched_at ON metadata_cache(fetched_at);
```

**Fields**:
- `media_id` - Primary key, references `media.id`
- `source` - Metadata source: `tmdb`, `tvdb`, `musicbrainz`
- `data` - Full JSON API response (stored as TEXT for SQLite, JSONB for PostgreSQL)
- `confidence` - Auto-match confidence score (0.0-1.0)
- `fetched_at` - When metadata was fetched
- `expires_at` - Optional expiry for cache invalidation

**Indexes**:
- `source` - Filter by metadata provider
- `fetched_at` - Find stale metadata for refresh

**Note**: This table stores complete API responses while main tables (movies, tv_episodes, music_tracks) store normalized key fields for efficient querying.

---

### Images Table

Multiple images per media (posters, backdrops, banners, logos) with different sizes.

```sql
CREATE TABLE images (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    image_type TEXT NOT NULL CHECK(image_type IN ('poster', 'backdrop', 'banner', 'logo', 'thumb', 'clearart')),
    file_path TEXT NOT NULL,
    width INTEGER,
    height INTEGER,
    size_variant TEXT CHECK(size_variant IN ('original', 'large', 'medium', 'small', 'thumb')),
    language TEXT,
    is_primary BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

CREATE INDEX idx_images_media_id ON images(media_id);
CREATE INDEX idx_images_type ON images(image_type);
CREATE INDEX idx_images_primary ON images(media_id, image_type, is_primary);
```

**Fields**:
- `id` - Primary key
- `media_id` - Reference to media item
- `image_type` - poster, backdrop, banner, logo, thumb, clearart
- `file_path` - Path to image file (relative or absolute)
- `width` / `height` - Image dimensions
- `size_variant` - original, large, medium, small, thumb
- `language` - ISO 639-1 language code for localized images
- `is_primary` - Mark primary image for each type
- `created_at` - Timestamp

**Indexes**:
- `media_id` - Get all images for media
- `image_type` - Filter by type
- `(media_id, image_type, is_primary)` - Get primary image of type

---

### Audio Tracks Table

Audio streams for each media file.

```sql
CREATE TABLE audio_tracks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    stream_index INTEGER NOT NULL,
    codec TEXT,
    language TEXT,
    title TEXT,
    channels INTEGER,
    channel_layout TEXT,
    sample_rate INTEGER,
    bitrate INTEGER,
    is_default BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    UNIQUE(media_id, stream_index)
);

CREATE INDEX idx_audio_tracks_media_id ON audio_tracks(media_id);
CREATE INDEX idx_audio_tracks_language ON audio_tracks(language);
```

**Fields**:
- `id` - Primary key
- `media_id` - Reference to media item
- `stream_index` - FFmpeg stream index
- `codec` - Audio codec (e.g., aac, ac3, dts)
- `language` - ISO 639-2 language code
- `title` - Track title/description
- `channels` - Number of channels (2, 6, 8)
- `channel_layout` - stereo, 5.1, 7.1, etc.
- `sample_rate` - Sample rate in Hz
- `bitrate` - Bitrate in bits/s
- `is_default` - Default track flag
- `created_at` - Timestamp

**Constraints**:
- Unique `(media_id, stream_index)` - No duplicate streams

---

### Subtitle Tracks Table

Subtitle/caption streams for each media file.

```sql
CREATE TABLE subtitle_tracks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    stream_index INTEGER NOT NULL,
    codec TEXT,
    language TEXT,
    title TEXT,
    is_forced BOOLEAN DEFAULT 0,
    is_default BOOLEAN DEFAULT 0,
    is_sdh BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    UNIQUE(media_id, stream_index)
);

CREATE INDEX idx_subtitle_tracks_media_id ON subtitle_tracks(media_id);
CREATE INDEX idx_subtitle_tracks_language ON subtitle_tracks(language);
```

**Fields**:
- `id` - Primary key
- `media_id` - Reference to media item
- `stream_index` - FFmpeg stream index
- `codec` - Subtitle codec (e.g., srt, ass, pgs)
- `language` - ISO 639-2 language code
- `title` - Track title/description
- `is_forced` - Forced subtitle flag
- `is_default` - Default track flag
- `is_sdh` - SDH/CC flag (hearing impaired)
- `created_at` - Timestamp

**Constraints**:
- Unique `(media_id, stream_index)` - No duplicate streams

---

### People Table

Cast, crew, directors, actors, musicians, and other people associated with media.

```sql
CREATE TABLE people (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    original_name TEXT,
    biography TEXT,
    birth_date DATE,
    death_date DATE,
    birth_place TEXT,
    profile_image_path TEXT,
    imdb_id TEXT,
    tmdb_id INTEGER,
    gender INTEGER CHECK(gender IN (0, 1, 2, 3)), -- 0: not set, 1: female, 2: male, 3: non-binary
    known_for_department TEXT,
    popularity REAL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_people_name ON people(name);
CREATE INDEX idx_people_imdb_id ON people(imdb_id);
CREATE INDEX idx_people_tmdb_id ON people(tmdb_id);
CREATE INDEX idx_people_known_for ON people(known_for_department);
```

**Fields**:
- `id` - Primary key
- `name` - Person's name
- `original_name` - Name in original language
- `biography` - Biography/description
- `birth_date` - Date of birth
- `death_date` - Date of death (if applicable)
- `birth_place` - Place of birth
- `profile_image_path` - Path to profile photo
- `imdb_id` - IMDb identifier (e.g., nm0000206)
- `tmdb_id` - The Movie Database person ID
- `gender` - 0: not set, 1: female, 2: male, 3: non-binary
- `known_for_department` - Acting, Directing, Writing, etc.
- `popularity` - Popularity score from metadata providers
- `created_at` / `updated_at` - Timestamps

**Indexes**:
- `name` - Unique constraint and search
- `imdb_id` - External API lookups
- `tmdb_id` - External API lookups
- `known_for_department` - Filter by role

---

### Movie Credits Table

Links people to movies with their roles.

```sql
CREATE TABLE movie_credits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    person_id INTEGER NOT NULL,
    role TEXT NOT NULL CHECK(role IN ('actor', 'director', 'writer', 'producer', 'cinematographer', 'editor', 'composer', 'other')),
    character_name TEXT,
    credit_order INTEGER,
    department TEXT,
    job_title TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    FOREIGN KEY (person_id) REFERENCES people(id) ON DELETE CASCADE
);

CREATE INDEX idx_movie_credits_media_id ON movie_credits(media_id);
CREATE INDEX idx_movie_credits_person_id ON movie_credits(person_id);
CREATE INDEX idx_movie_credits_role ON movie_credits(role);
CREATE INDEX idx_movie_credits_order ON movie_credits(media_id, credit_order);
```

**Fields**:
- `id` - Primary key
- `media_id` - Reference to movie (media with type='movie')
- `person_id` - Reference to person
- `role` - Primary role category
- `character_name` - Character name for actors
- `credit_order` - Display order (lower numbers appear first)
- `department` - Department (e.g., "Sound", "Visual Effects")
- `job_title` - Specific job title
- `created_at` - Timestamp

**Indexes**:
- `media_id` - Get all credits for a movie
- `person_id` - Get all movies for a person
- `role` - Filter by role type
- `(media_id, credit_order)` - Ordered cast list

---

### TV Episode Credits Table

Links people to TV episodes with their roles.

```sql
CREATE TABLE episode_credits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    person_id INTEGER NOT NULL,
    role TEXT NOT NULL CHECK(role IN ('actor', 'director', 'writer', 'producer', 'guest_star', 'other')),
    character_name TEXT,
    credit_order INTEGER,
    is_recurring BOOLEAN DEFAULT 0,
    is_guest BOOLEAN DEFAULT 0,
    department TEXT,
    job_title TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    FOREIGN KEY (person_id) REFERENCES people(id) ON DELETE CASCADE
);

CREATE INDEX idx_episode_credits_media_id ON episode_credits(media_id);
CREATE INDEX idx_episode_credits_person_id ON episode_credits(person_id);
CREATE INDEX idx_episode_credits_role ON episode_credits(role);
CREATE INDEX idx_episode_credits_order ON episode_credits(media_id, credit_order);
```

**Fields**:
- `id` - Primary key
- `media_id` - Reference to episode (media with type='tv_episode')
- `person_id` - Reference to person
- `role` - Primary role category
- `character_name` - Character name for actors
- `credit_order` - Display order
- `is_recurring` - Recurring cast member flag
- `is_guest` - Guest star flag
- `department` - Department
- `job_title` - Specific job title
- `created_at` - Timestamp

**Indexes**:
- `media_id` - Get all credits for an episode
- `person_id` - Get all episodes for a person
- `role` - Filter by role type
- `(media_id, credit_order)` - Ordered cast list

---

### Music Credits Table

Links people to music tracks with their roles.

```sql
CREATE TABLE music_credits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    person_id INTEGER NOT NULL,
    role TEXT NOT NULL CHECK(role IN ('artist', 'composer', 'lyricist', 'producer', 'performer', 'conductor', 'engineer', 'mixer', 'other')),
    instrument TEXT,
    credit_order INTEGER,
    department TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    FOREIGN KEY (person_id) REFERENCES people(id) ON DELETE CASCADE
);

CREATE INDEX idx_music_credits_media_id ON music_credits(media_id);
CREATE INDEX idx_music_credits_person_id ON music_credits(person_id);
CREATE INDEX idx_music_credits_role ON music_credits(role);
```

**Fields**:
- `id` - Primary key
- `media_id` - Reference to music track (media with type='music_track')
- `person_id` - Reference to person
- `role` - Primary role category
- `instrument` - Instrument played (for performers)
- `credit_order` - Display order
- `department` - Department (e.g., "Production", "Engineering")
- `created_at` - Timestamp

**Indexes**:
- `media_id` - Get all credits for a track
- `person_id` - Get all tracks for a person
- `role` - Filter by role type

---

### Collections Table

Group related media together (e.g., MCU, Star Wars, James Bond, etc.).

```sql
CREATE TABLE collections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT,
    poster_path TEXT,
    backdrop_path TEXT,
    collection_type TEXT CHECK(collection_type IN ('franchise', 'series', 'anthology', 'universe', 'custom')) DEFAULT 'custom',
    tmdb_id INTEGER,
    sort_order_mode TEXT CHECK(sort_order_mode IN ('manual', 'release_date', 'title', 'chronological')) DEFAULT 'manual',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_collections_name ON collections(name);
CREATE INDEX idx_collections_tmdb_id ON collections(tmdb_id);
CREATE INDEX idx_collections_type ON collections(collection_type);
```

**Fields**:
- `id` - Primary key
- `name` - Collection name (e.g., "The Matrix Collection", "Marvel Cinematic Universe")
- `description` - Collection description
- `poster_path` - Collection poster image
- `backdrop_path` - Collection backdrop image
- `collection_type` - Type of collection
- `tmdb_id` - The Movie Database collection ID
- `sort_order_mode` - How to sort items in collection
- `created_at` / `updated_at` - Timestamps

**Indexes**:
- `name` - Unique constraint and search
- `tmdb_id` - External API lookups
- `collection_type` - Filter by type

---

### Collection Media Junction Table

Many-to-many relationship between collections and media.

```sql
CREATE TABLE collection_media (
    collection_id INTEGER NOT NULL,
    media_id INTEGER NOT NULL,
    sort_order INTEGER DEFAULT 0,
    notes TEXT,
    added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (collection_id, media_id),
    FOREIGN KEY (collection_id) REFERENCES collections(id) ON DELETE CASCADE,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

CREATE INDEX idx_collection_media_collection_id ON collection_media(collection_id);
CREATE INDEX idx_collection_media_media_id ON collection_media(media_id);
CREATE INDEX idx_collection_media_order ON collection_media(collection_id, sort_order);
```

**Fields**:
- `collection_id` - Reference to collection
- `media_id` - Reference to media
- `sort_order` - Manual ordering within collection
- `notes` - Optional notes about this item in the collection
- `added_at` - When item was added to collection

**Indexes**:
- `collection_id` - Get all media in a collection
- `media_id` - Get all collections for media
- `(collection_id, sort_order)` - Ordered collection items

---

### Genres Table

Normalized genre list (alternative to comma-separated genre field).

```sql
CREATE TABLE genres (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    genre_type TEXT CHECK(genre_type IN ('movie', 'tv', 'music', 'all')) DEFAULT 'all',
    tmdb_id INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_genres_name ON genres(name);
CREATE INDEX idx_genres_type ON genres(genre_type);
CREATE INDEX idx_genres_tmdb_id ON genres(tmdb_id);
```

**Fields**:
- `id` - Primary key
- `name` - Genre name (e.g., "Action", "Drama", "Rock", "Jazz")
- `genre_type` - Applicable media types
- `tmdb_id` - The Movie Database genre ID
- `created_at` - Timestamp

**Indexes**:
- `name` - Unique constraint and search
- `genre_type` - Filter by media type
- `tmdb_id` - External API lookups

**Note**: This is an alternative to the comma-separated `genre` field in movies/tv_shows/music_tracks tables. Can be used alongside or as a replacement for better querying.

---

### Media Genres Junction Table

Many-to-many relationship between media and genres (if using normalized genres).

```sql
CREATE TABLE media_genres (
    media_id INTEGER NOT NULL,
    genre_id INTEGER NOT NULL,
    PRIMARY KEY (media_id, genre_id),
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    FOREIGN KEY (genre_id) REFERENCES genres(id) ON DELETE CASCADE
);

CREATE INDEX idx_media_genres_media_id ON media_genres(media_id);
CREATE INDEX idx_media_genres_genre_id ON media_genres(genre_id);
```

**Fields**:
- `media_id` - Reference to media
- `genre_id` - Reference to genre

**Indexes**:
- `media_id` - Get all genres for media
- `genre_id` - Get all media for genre

**Note**: Use this if migrating away from comma-separated genre fields. Provides better query performance and consistency.

---

### Studios Table

Production companies and networks.

```sql
CREATE TABLE studios (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    logo_path TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_studios_name ON studios(name);
```

**Fields**:
- `id` - Primary key
- `name` - Studio/network name
- `logo_path` - Path to logo image
- `created_at` - Timestamp

---

### Media Studios Junction Table

Many-to-many relationship between media and studios.

```sql
CREATE TABLE media_studios (
    media_id INTEGER NOT NULL,
    studio_id INTEGER NOT NULL,
    studio_type TEXT CHECK(studio_type IN ('production', 'network', 'distributor')),
    PRIMARY KEY (media_id, studio_id),
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    FOREIGN KEY (studio_id) REFERENCES studios(id) ON DELETE CASCADE
);

CREATE INDEX idx_media_studios_studio_id ON media_studios(studio_id);
```

**Fields**:
- `media_id` - Reference to media
- `studio_id` - Reference to studio
- `studio_type` - production, network, or distributor

---

### Tags Table

User-generated tags for organizing media.

```sql
CREATE TABLE tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_tags_name ON tags(name);
```

**Fields**:
- `id` - Primary key
- `name` - Tag name (unique)
- `created_at` - Timestamp

---

### Media Tags Junction Table

Many-to-many relationship between media and tags.

```sql
CREATE TABLE media_tags (
    media_id INTEGER NOT NULL,
    tag_id INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (media_id, tag_id),
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

CREATE INDEX idx_media_tags_tag_id ON media_tags(tag_id);
```

**Fields**:
- `media_id` - Reference to media
- `tag_id` - Reference to tag
- `created_at` - When tag was applied

---

### Watch History Table

Complete viewing history for all users.

```sql
CREATE TABLE watch_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    user_id INTEGER DEFAULT 1,
    watched_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    duration_seconds INTEGER,
    completed BOOLEAN DEFAULT 0,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

CREATE INDEX idx_watch_history_media_id ON watch_history(media_id);
CREATE INDEX idx_watch_history_user_id ON watch_history(user_id);
CREATE INDEX idx_watch_history_watched_at ON watch_history(watched_at DESC);
```

**Fields**:
- `id` - Primary key
- `media_id` - Reference to media watched
- `user_id` - User ID (default 1 for single-user)
- `watched_at` - Timestamp of view
- `duration_seconds` - How long watched
- `completed` - Whether watched to end
- No unique constraint - allows multiple views

**Indexes**:
- `media_id` - Get watch history for media
- `user_id` - Get user's watch history
- `watched_at` - Sort by recent views

---

### Ratings Table

User ratings and external ratings from various sources.

```sql
CREATE TABLE ratings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    rating_type TEXT NOT NULL CHECK(rating_type IN ('user', 'imdb', 'tmdb', 'rt_critics', 'rt_audience', 'metacritic')),
    rating_value REAL NOT NULL,
    rating_scale REAL DEFAULT 10.0,
    vote_count INTEGER,
    user_id INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    UNIQUE(media_id, rating_type, user_id)
);

CREATE INDEX idx_ratings_media_id ON ratings(media_id);
CREATE INDEX idx_ratings_type ON ratings(rating_type);
CREATE INDEX idx_ratings_user ON ratings(user_id);
```

**Fields**:
- `id` - Primary key
- `media_id` - Reference to media
- `rating_type` - user, imdb, tmdb, rt_critics, rt_audience, metacritic
- `rating_value` - Numeric rating
- `rating_scale` - Max value (10.0, 5.0, 100.0)
- `vote_count` - Number of votes (external ratings)
- `user_id` - User ID for user ratings
- `created_at` / `updated_at` - Timestamps

**Constraints**:
- Unique `(media_id, rating_type, user_id)` - One rating per user per type per media

**Indexes**:
- `media_id` - Get all ratings for media
- `rating_type` - Filter by rating source
- `user_id` - Get user's ratings

---

### Alternative Titles Table

Store alternative titles, original titles, and localized titles for media.

```sql
CREATE TABLE alternative_titles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    title_type TEXT NOT NULL CHECK(title_type IN ('original', 'localized', 'aka', 'sort')),
    language TEXT,
    country TEXT,
    is_primary BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

CREATE INDEX idx_alternative_titles_media_id ON alternative_titles(media_id);
CREATE INDEX idx_alternative_titles_title ON alternative_titles(title);
CREATE INDEX idx_alternative_titles_type ON alternative_titles(title_type);
```

**Fields**:
- `id` - Primary key
- `media_id` - Reference to media
- `title` - Alternative title text
- `title_type` - original, localized, aka (also known as), sort
- `language` - ISO 639-1 language code
- `country` - ISO 3166-1 country code
- `is_primary` - Primary title for this type/language
- `created_at` - Timestamp

**Indexes**:
- `media_id` - Get all titles for media
- `title` - Search by alternative title
- `title_type` - Filter by type

---

### Release Dates Table

Track different release dates by type and region.

```sql
CREATE TABLE release_dates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    release_type TEXT NOT NULL CHECK(release_type IN ('theatrical', 'digital', 'physical', 'tv', 'premiere', 'limited', 'rerelease')),
    release_date DATE NOT NULL,
    country TEXT,
    region TEXT,
    note TEXT,
    certification TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

CREATE INDEX idx_release_dates_media_id ON release_dates(media_id);
CREATE INDEX idx_release_dates_type ON release_dates(release_type);
CREATE INDEX idx_release_dates_country ON release_dates(country);
CREATE INDEX idx_release_dates_date ON release_dates(release_date);
```

**Fields**:
- `id` - Primary key
- `media_id` - Reference to media
- `release_type` - theatrical, digital, physical, tv, premiere, limited, rerelease
- `release_date` - Date of this release
- `country` - ISO 3166-1 country code
- `region` - Region within country (optional)
- `note` - Additional info (e.g., "Director's Cut", "Festival Premiere")
- `certification` - Content rating for this region
- `created_at` - Timestamp

**Indexes**:
- `media_id` - Get all releases for media
- `release_type` - Filter by release type
- `country` - Filter by country
- `release_date` - Sort by date

---

### Media Versions Table

Track different versions/editions of the same content.

```sql
CREATE TABLE media_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    base_media_id INTEGER NOT NULL,
    version_media_id INTEGER NOT NULL,
    version_type TEXT NOT NULL CHECK(version_type IN ('theatrical', 'directors_cut', 'extended', 'unrated', 'international', 'special_edition', 'remastered', 'criterion')),
    version_name TEXT,
    is_default BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (base_media_id) REFERENCES media(id) ON DELETE CASCADE,
    FOREIGN KEY (version_media_id) REFERENCES media(id) ON DELETE CASCADE,
    UNIQUE(base_media_id, version_media_id)
);

CREATE INDEX idx_media_versions_base ON media_versions(base_media_id);
CREATE INDEX idx_media_versions_version ON media_versions(version_media_id);
```

**Fields**:
- `id` - Primary key
- `base_media_id` - Reference to base/original media
- `version_media_id` - Reference to version media file
- `version_type` - Type of version
- `version_name` - Custom version name
- `is_default` - Default version to play
- `created_at` - Timestamp

**Note**: This allows linking multiple physical files that represent different cuts of the same movie.

---

### Awards Table

Track awards and nominations.

```sql
CREATE TABLE awards (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER,
    person_id INTEGER,
    award_name TEXT NOT NULL,
    award_category TEXT,
    award_year INTEGER,
    won BOOLEAN DEFAULT 0,
    organization TEXT,
    notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    FOREIGN KEY (person_id) REFERENCES people(id) ON DELETE CASCADE,
    CHECK ((media_id IS NOT NULL) OR (person_id IS NOT NULL))
);

CREATE INDEX idx_awards_media_id ON awards(media_id);
CREATE INDEX idx_awards_person_id ON awards(person_id);
CREATE INDEX idx_awards_organization ON awards(organization);
CREATE INDEX idx_awards_year ON awards(award_year);
CREATE INDEX idx_awards_won ON awards(won);
```

**Fields**:
- `id` - Primary key
- `media_id` - Reference to media (if award is for media)
- `person_id` - Reference to person (if award is for person)
- `award_name` - Award name (e.g., "Best Picture", "Best Actor")
- `award_category` - Category within award
- `award_year` - Year of award ceremony
- `won` - TRUE if won, FALSE if nominated
- `organization` - Academy Awards, Golden Globes, Emmy, etc.
- `notes` - Additional information
- `created_at` - Timestamp

**Constraints**:
- Either `media_id` or `person_id` must be set

**Indexes**:
- `media_id` - Get awards for media
- `person_id` - Get awards for person
- `organization` - Filter by award organization
- `award_year` - Filter by year
- `won` - Filter winners vs nominees

---

### Chapters Table

Chapter markers for skip intro/outro functionality.

```sql
CREATE TABLE chapters (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    chapter_number INTEGER NOT NULL,
    start_time REAL NOT NULL,
    end_time REAL NOT NULL,
    title TEXT,
    chapter_type TEXT CHECK(chapter_type IN ('chapter', 'intro', 'credits', 'recap', 'preview', 'commercial')),
    thumbnail_path TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    UNIQUE(media_id, chapter_number)
);

CREATE INDEX idx_chapters_media_id ON chapters(media_id);
CREATE INDEX idx_chapters_type ON chapters(chapter_type);
CREATE INDEX idx_chapters_start_time ON chapters(start_time);
```

**Fields**:
- `id` - Primary key
- `media_id` - Reference to media
- `chapter_number` - Chapter sequence number
- `start_time` - Start timestamp in seconds
- `end_time` - End timestamp in seconds
- `title` - Chapter title
- `chapter_type` - chapter, intro, credits, recap, preview, commercial
- `thumbnail_path` - Chapter preview image
- `created_at` - Timestamp

**Constraints**:
- Unique `(media_id, chapter_number)` - Sequential chapter numbering

**Indexes**:
- `media_id` - Get all chapters for media
- `chapter_type` - Find intros/credits for skip
- `start_time` - Time-based lookups

---

### Playback Markers Table

Intro/outro/credits markers for auto-skip and "next episode" features.

```sql
CREATE TABLE playback_markers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    marker_type TEXT NOT NULL CHECK(marker_type IN ('intro_start', 'intro_end', 'credits_start', 'recap_start', 'recap_end', 'preview_start')),
    timestamp REAL NOT NULL,
    confidence REAL DEFAULT 1.0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    UNIQUE(media_id, marker_type)
);

CREATE INDEX idx_playback_markers_media_id ON playback_markers(media_id);
CREATE INDEX idx_playback_markers_type ON playback_markers(marker_type);
```

**Fields**:
- `id` - Primary key
- `media_id` - Reference to media
- `marker_type` - intro_start, intro_end, credits_start, recap_start, recap_end, preview_start
- `timestamp` - Time in seconds
- `confidence` - Confidence score if auto-detected (0.0-1.0)
- `created_at` / `updated_at` - Timestamps

**Constraints**:
- Unique `(media_id, marker_type)` - One marker of each type per media

**Use Cases**:
- Skip intro button (intro_start to intro_end)
- Auto-play next episode (credits_start)
- Skip recap for binge watching

---

### External Subtitles Table

Track external subtitle files (not embedded in media container).

```sql
CREATE TABLE external_subtitles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    file_path TEXT NOT NULL,
    language TEXT NOT NULL,
    title TEXT,
    format TEXT CHECK(format IN ('srt', 'ass', 'ssa', 'vtt', 'sub', 'idx')),
    is_forced BOOLEAN DEFAULT 0,
    is_sdh BOOLEAN DEFAULT 0,
    is_default BOOLEAN DEFAULT 0,
    source TEXT CHECK(source IN ('manual', 'opensubtitles', 'subscene', 'embedded_extract', 'other')),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

CREATE INDEX idx_external_subtitles_media_id ON external_subtitles(media_id);
CREATE INDEX idx_external_subtitles_language ON external_subtitles(language);
CREATE INDEX idx_external_subtitles_file_path ON external_subtitles(file_path);
```

**Fields**:
- `id` - Primary key
- `media_id` - Reference to media
- `file_path` - Path to subtitle file
- `language` - ISO 639-2 language code
- `title` - Subtitle track description
- `format` - srt, ass, ssa, vtt, sub, idx
- `is_forced` - Forced subtitle flag
- `is_sdh` - SDH/CC flag (hearing impaired)
- `is_default` - Default subtitle track
- `source` - manual, opensubtitles, subscene, embedded_extract, other
- `created_at` - Timestamp

**Indexes**:
- `media_id` - Get subtitles for media
- `language` - Filter by language
- `file_path` - File lookups

---

### Trailers Table

Links to trailer files and URLs.

```sql
CREATE TABLE trailers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    title TEXT,
    file_path TEXT,
    url TEXT,
    trailer_type TEXT CHECK(trailer_type IN ('teaser', 'theatrical', 'tv_spot', 'clip', 'behind_scenes', 'featurette')),
    duration REAL,
    resolution TEXT,
    language TEXT,
    published_date DATE,
    source TEXT,
    is_official BOOLEAN DEFAULT 1,
    sort_order INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    CHECK ((file_path IS NOT NULL) OR (url IS NOT NULL))
);

CREATE INDEX idx_trailers_media_id ON trailers(media_id);
CREATE INDEX idx_trailers_type ON trailers(trailer_type);
```

**Fields**:
- `id` - Primary key
- `media_id` - Reference to media
- `title` - Trailer title
- `file_path` - Path to local trailer file
- `url` - URL to online trailer (YouTube, Vimeo, etc.)
- `trailer_type` - teaser, theatrical, tv_spot, clip, behind_scenes, featurette
- `duration` - Length in seconds
- `resolution` - Video resolution
- `language` - Audio language
- `published_date` - When trailer was released
- `source` - Where trailer came from
- `is_official` - Official vs fan-made
- `sort_order` - Display order
- `created_at` - Timestamp

**Constraints**:
- Either `file_path` or `url` must be provided

---

### Extras Table

Special features, behind-the-scenes content, deleted scenes, etc.

```sql
CREATE TABLE extras (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    file_path TEXT NOT NULL,
    extra_type TEXT CHECK(extra_type IN ('deleted_scene', 'behind_scenes', 'interview', 'featurette', 'blooper', 'commentary', 'making_of', 'other')),
    duration REAL,
    description TEXT,
    sort_order INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

CREATE INDEX idx_extras_media_id ON extras(media_id);
CREATE INDEX idx_extras_type ON extras(extra_type);
```

**Fields**:
- `id` - Primary key
- `media_id` - Reference to main media
- `title` - Extra title
- `file_path` - Path to extra file
- `extra_type` - Type of special feature
- `duration` - Length in seconds
- `description` - Description text
- `sort_order` - Display order
- `created_at` - Timestamp

---

### Similar Media Table

Store recommendations for similar/related content.

```sql
CREATE TABLE similar_media (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL,
    similar_media_id INTEGER NOT NULL,
    similarity_score REAL DEFAULT 0.5,
    source TEXT CHECK(source IN ('manual', 'tmdb', 'algorithm', 'user')),
    reason TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    FOREIGN KEY (similar_media_id) REFERENCES media(id) ON DELETE CASCADE,
    UNIQUE(media_id, similar_media_id)
);

CREATE INDEX idx_similar_media_media_id ON similar_media(media_id);
CREATE INDEX idx_similar_media_similar_id ON similar_media(similar_media_id);
CREATE INDEX idx_similar_media_score ON similar_media(similarity_score DESC);
```

**Fields**:
- `id` - Primary key
- `media_id` - Reference to base media
- `similar_media_id` - Reference to similar media
- `similarity_score` - Score 0.0-1.0
- `source` - manual, tmdb, algorithm, user
- `reason` - Why they're similar (e.g., "Same director", "Similar genre")
- `created_at` - Timestamp

**Constraints**:
- Unique `(media_id, similar_media_id)` - No duplicate pairs

**Indexes**:
- `media_id` - Get recommendations for media
- `similar_media_id` - Reverse lookups
- `similarity_score` - Sort by relevance

---

### Plugins Table

Plugin registry for installed extensions.

```sql
CREATE TABLE plugins (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    version TEXT NOT NULL,
    plugin_type TEXT NOT NULL CHECK(plugin_type IN ('metadata_provider', 'auth_provider', 'notifier', 'transcoder', 'scanner', 'storage_backend', 'analytics', 'other')),
    runtime TEXT NOT NULL CHECK(runtime IN ('grpc', 'http', 'wasm')),
    endpoint TEXT,
    enabled BOOLEAN DEFAULT 1,
    config JSONB,
    permissions JSONB,
    author TEXT,
    description TEXT,
    homepage_url TEXT,
    installed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_health_check DATETIME,
    health_status TEXT CHECK(health_status IN ('healthy', 'unhealthy', 'unknown', NULL))
);

CREATE INDEX idx_plugins_enabled ON plugins(enabled);
CREATE INDEX idx_plugins_type ON plugins(plugin_type);
CREATE INDEX idx_plugins_name ON plugins(name);
```

**Fields**:
- `id` - Primary key
- `name` - Unique plugin identifier
- `version` - Semantic version (e.g., 1.0.0)
- `plugin_type` - Type of plugin (metadata_provider, auth_provider, notifier, etc.)
- `runtime` - Execution runtime (grpc, http, wasm)
- `endpoint` - Connection endpoint (gRPC address, HTTP URL, or WASM file path)
- `enabled` - Whether plugin is active
- `config` - JSON configuration specific to plugin
- `permissions` - JSON array of granted permissions
- `author` - Plugin author name
- `description` - Plugin description
- `homepage_url` - Plugin homepage or repository
- `installed_at` - Installation timestamp
- `updated_at` - Last update timestamp
- `last_health_check` - Last health check timestamp
- `health_status` - Current health status

**Indexes**:
- `enabled` - Filter active plugins
- `plugin_type` - Filter by type
- `name` - Unique constraint and lookups

---

### Plugin Data Table

Key-value store for plugin-specific data.

```sql
CREATE TABLE plugin_data (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plugin_id INTEGER NOT NULL,
    key TEXT NOT NULL,
    value JSONB,
    expires_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (plugin_id) REFERENCES plugins(id) ON DELETE CASCADE,
    UNIQUE(plugin_id, key)
);

CREATE INDEX idx_plugin_data_plugin_id ON plugin_data(plugin_id);
CREATE INDEX idx_plugin_data_key ON plugin_data(key);
CREATE INDEX idx_plugin_data_expires_at ON plugin_data(expires_at);
```

**Fields**:
- `id` - Primary key
- `plugin_id` - Reference to plugin
- `key` - Data key (unique per plugin)
- `value` - JSON value
- `expires_at` - Optional expiration timestamp for cache invalidation
- `created_at` / `updated_at` - Timestamps

**Constraints**:
- Unique `(plugin_id, key)` - One value per key per plugin

**Indexes**:
- `plugin_id` - Get all data for plugin
- `key` - Search by key
- `expires_at` - Cleanup expired data

**Use Cases**:
- Cache external API responses
- Store plugin state
- Persist configuration overrides

---

### Plugin Events Table

Audit log for plugin activities and lifecycle events.

```sql
CREATE TABLE plugin_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plugin_id INTEGER NOT NULL,
    event_type TEXT NOT NULL CHECK(event_type IN (
        'installed', 'uninstalled', 'enabled', 'disabled', 'updated',
        'config_changed', 'health_check_failed', 'error', 
        'metadata_fetched', 'notification_sent', 'custom'
    )),
    event_data JSONB,
    severity TEXT CHECK(severity IN ('info', 'warning', 'error', 'critical')) DEFAULT 'info',
    message TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (plugin_id) REFERENCES plugins(id) ON DELETE CASCADE
);

CREATE INDEX idx_plugin_events_plugin_id ON plugin_events(plugin_id);
CREATE INDEX idx_plugin_events_type ON plugin_events(event_type);
CREATE INDEX idx_plugin_events_created_at ON plugin_events(created_at DESC);
CREATE INDEX idx_plugin_events_severity ON plugin_events(severity);
```

**Fields**:
- `id` - Primary key
- `plugin_id` - Reference to plugin
- `event_type` - Type of event
- `event_data` - JSON event details
- `severity` - Event severity level
- `message` - Human-readable message
- `created_at` - Event timestamp

**Indexes**:
- `plugin_id` - Get events for plugin
- `event_type` - Filter by event type
- `created_at` - Sort by recency
- `severity` - Filter by severity

**Use Cases**:
- Audit trail for plugin actions
- Debugging plugin issues
- Security monitoring
- Usage analytics

---

### Plugin Hooks Table

Event hook subscriptions for plugins.

```sql
CREATE TABLE plugin_hooks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plugin_id INTEGER NOT NULL,
    hook_event TEXT NOT NULL CHECK(hook_event IN (
        'media.added', 'media.updated', 'media.deleted',
        'library.scan.started', 'library.scan.completed',
        'playback.started', 'playback.stopped', 'playback.progress',
        'user.login', 'user.logout', 'user.created',
        'transcode.started', 'transcode.completed', 'transcode.failed',
        'server.started', 'server.stopped'
    )),
    priority INTEGER DEFAULT 0,
    enabled BOOLEAN DEFAULT 1,
    filter_conditions JSONB,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (plugin_id) REFERENCES plugins(id) ON DELETE CASCADE
);

CREATE INDEX idx_plugin_hooks_plugin_id ON plugin_hooks(plugin_id);
CREATE INDEX idx_plugin_hooks_event ON plugin_hooks(hook_event);
CREATE INDEX idx_plugin_hooks_enabled ON plugin_hooks(enabled);
CREATE INDEX idx_plugin_hooks_priority ON plugin_hooks(priority DESC);
```

**Fields**:
- `id` - Primary key
- `plugin_id` - Reference to plugin
- `hook_event` - Event to hook into (e.g., 'media.added')
- `priority` - Execution priority (higher runs first)
- `enabled` - Whether hook is active
- `filter_conditions` - JSON conditions to filter events (e.g., only movies)
- `created_at` - Registration timestamp

**Indexes**:
- `plugin_id` - Get hooks for plugin
- `hook_event` - Find hooks for specific event
- `enabled` - Filter active hooks
- `priority` - Sort by execution order

**Use Cases**:
- Register plugins to receive notifications on media events
- Send notifications when new media is added
- Trigger metadata enrichment on library scan
- Custom post-processing workflows

---

## Relationships

### Entity Relationship Diagram

```
libraries (1) ──────< (N) media ───────< (N) images
                        │               < (N) audio_tracks
                        │               < (N) subtitle_tracks
                        │               < (N) external_subtitles
                        │               < (N) alternative_titles
                        │               < (N) release_dates
                        │               < (N) chapters
                        │               < (N) playback_markers
                        │               < (N) trailers
                        │               < (N) extras
                        ├──< (1) movies < (N) watch_history
                        ├──< (1) tv_episodes ──> (1) tv_seasons ──> (1) tv_shows
                        ├──< (1) music_tracks   < (N) ratings
                        │                       < (N) awards
                        └──< (0..1) metadata_cache

media (N) ──────> (N) media_tags ────────< (1) tags
media (N) ──────> (N) media_studios ─────< (1) studios
media (N) ──────> (N) media_genres ──────< (1) genres
media (N) ──────> (N) collection_media ──< (1) collections
media (N) ──────> (N) movie_credits ─────< (1) people ──────< (N) awards
media (N) ──────> (N) episode_credits ───< (1) people
media (N) ──────> (N) music_credits ─────< (1) people
media (N) ──────> (N) similar_media ─────> (N) media
media (N) ──────> (N) media_versions ────> (N) media

media (1) ──────< (N) watch_progress
media (1) ──────< (N) transcode_jobs

libraries (1) ──────< (N) tv_shows ──────< (N) tv_seasons

plugins (1) ──────< (N) plugin_data
plugins (1) ──────< (N) plugin_events
plugins (1) ──────< (N) plugin_hooks
```

### Key Relationships

1. **Library → Media** (One-to-Many)
   - One library contains many media files
   - Cascade delete: Deleting library removes all media

2. **Media → Type-Specific** (One-to-One)
   - Each media record has exactly one type-specific record (movies/tv_episodes/music_tracks)
   - Cascade delete: Deleting media removes type data

3. **TV Show Hierarchy** (One-to-Many chains)
   - Library → TV Shows → Seasons → Episodes
   - Cascade delete: Deleting show removes all seasons and episodes

4. **Media → Images** (One-to-Many)
   - One media can have multiple images (posters, backdrops, etc.)
   - Cascade delete: Deleting media removes images

5. **Media → Audio/Subtitle Tracks** (One-to-Many)
   - One media can have multiple audio and subtitle streams
   - Cascade delete: Deleting media removes tracks

6. **Media → External Subtitles** (One-to-Many)
   - External subtitle files linked to media
   - Cascade delete: Deleting media removes subtitle links

7. **Media → Alternative Titles** (One-to-Many)
   - Multiple titles for localization and search
   - Cascade delete: Deleting media removes titles

8. **Media → Release Dates** (One-to-Many)
   - Multiple release dates by region and type
   - Cascade delete: Deleting media removes release data

9. **Media → Chapters** (One-to-Many)
   - Chapter markers with timestamps
   - Cascade delete: Deleting media removes chapters

10. **Media → Playback Markers** (One-to-Many)
    - Intro/outro/credits markers for skip functionality
    - Cascade delete: Deleting media removes markers

11. **Media → Trailers** (One-to-Many)
    - Multiple trailers per media item
    - Cascade delete: Deleting media removes trailer links

12. **Media → Extras** (One-to-Many)
    - Special features and bonus content
    - Cascade delete: Deleting media removes extras

13. **Media → People** (Many-to-Many)
    - Media items linked to people via movie_credits/episode_credits/music_credits
    - Stores role (actor, director, writer, etc.) and character names
    - Cascade delete: Deleting media removes credits, but people remain

14. **Media → Collections** (Many-to-Many)
    - Media can belong to multiple collections (e.g., MCU, Star Wars)
    - Cascade delete: Deleting media removes membership, but collection remains

15. **Media → Genres** (Implicit via genre field)
    - Movies, TV shows, and music tracks have comma-separated genre strings
    - Query via `WHERE genre LIKE '%Action%'` or full-text search

16. **Media → Studios** (Many-to-Many)
    - Media linked to production companies/networks via media_studios
    - Cascade delete: Deleting media removes associations, but studios remain

17. **Media → Tags** (Many-to-Many)
    - User-generated tags via media_tags junction
    - Cascade delete: Deleting media removes tags, but tag entities remain

18. **Media → Watch Progress** (One-to-Many)
    - One media can have multiple progress records (multi-user)
    - Cascade delete: Deleting media removes progress

19. **Media → Watch History** (One-to-Many)
    - Complete viewing history with timestamps
    - Cascade delete: Deleting media removes history

20. **Media → Ratings** (One-to-Many)
    - User ratings and external ratings (IMDb, TMDb, Rotten Tomatoes)
    - Cascade delete: Deleting media removes ratings

21. **Media → Awards** (One-to-Many)
    - Awards and nominations for media
    - Cascade delete: Deleting media removes awards

22. **People → Awards** (One-to-Many)
    - Awards and nominations for people (actors, directors)
    - Cascade delete: Deleting person removes awards

23. **Media → Similar Media** (Many-to-Many self-reference)
    - Recommendations and related content
    - Cascade delete: Deleting media removes similarity links

24. **Media → Media Versions** (One-to-Many self-reference)
    - Different cuts/editions of same content
    - Links theatrical, director's cut, extended editions

25. **Media → Transcode Jobs** (One-to-Many)
    - One media can have multiple transcode jobs (different qualities)
    - Cascade delete: Deleting media removes jobs

26. **Media → Metadata Cache** (One-to-Zero-or-One)
    - Optional external metadata for media
    - Cascade delete: Deleting media removes cached metadata

27. **Plugins → Plugin Data** (One-to-Many)
    - Each plugin can store multiple key-value pairs
    - Used for plugin state, cache, and configuration overrides
    - Cascade delete: Deleting plugin removes all data

28. **Plugins → Plugin Events** (One-to-Many)
    - Audit log of plugin activities and lifecycle events
    - Used for debugging, monitoring, and security
    - Cascade delete: Deleting plugin removes event history

29. **Plugins → Plugin Hooks** (One-to-Many)
    - Each plugin can subscribe to multiple system events
    - Enables event-driven architecture and extensibility
    - Cascade delete: Deleting plugin removes hook subscriptions

---

## Query Patterns

### Get All Movies in a Library

```sql
SELECT m.*, mov.*
FROM media m
JOIN movies mov ON m.id = mov.media_id
WHERE m.library_id = ?
  AND m.type = 'movie'
ORDER BY m.title;
```

### Get TV Show with All Episodes

```sql
-- Get show
SELECT * FROM tv_shows WHERE id = ?;

-- Get all seasons
SELECT * FROM tv_seasons WHERE show_id = ? ORDER BY season_number;

-- Get all episodes for a season
SELECT m.*, e.*
FROM tv_episodes e
JOIN media m ON e.media_id = m.id
WHERE e.show_id = ? AND e.season_id = ?
ORDER BY e.episode_number;

-- Get all episodes for a show
SELECT m.*, e.*, s.name as season_name
FROM tv_episodes e
JOIN media m ON e.media_id = m.id
JOIN tv_seasons s ON e.season_id = s.id
WHERE e.show_id = ?
ORDER BY e.season_number, e.episode_number;
```

### Get Media with All Images

```sql
-- Get primary poster
SELECT * FROM images
WHERE media_id = ? AND image_type = 'poster' AND is_primary = 1;

-- Get all backdrops
SELECT * FROM images
WHERE media_id = ? AND image_type = 'backdrop'
ORDER BY is_primary DESC;

-- Get all image types for media
SELECT image_type, COUNT(*) as count
FROM images
WHERE media_id = ?
GROUP BY image_type;
```

### Get Audio/Subtitle Track Info

```sql
-- Get all audio tracks for media
SELECT * FROM audio_tracks
WHERE media_id = ?
ORDER BY is_default DESC, stream_index;

-- Get default audio track
SELECT * FROM audio_tracks
WHERE media_id = ? AND is_default = 1;

-- Get all subtitle tracks
SELECT * FROM subtitle_tracks
WHERE media_id = ?
ORDER BY is_default DESC, is_forced DESC, language;

-- Get forced subtitles in user's language
SELECT * FROM subtitle_tracks
WHERE media_id = ? AND is_forced = 1 AND language = ?;
```

### Get Media with Cast and Crew

```sql
-- Get all cast for a movie
SELECT p.*, mc.role, mc.character_name, mc.credit_order
FROM movie_credits mc
JOIN people p ON mc.person_id = p.id
WHERE mc.media_id = ? AND mc.role = 'actor'
ORDER BY mc.credit_order;

-- Get director
SELECT p.*
FROM movie_credits mc
JOIN people p ON mc.person_id = p.id
WHERE mc.media_id = ? AND mc.role = 'director';

-- Get all movies by a specific actor
SELECT m.*, mov.*, mc.character_name
FROM movie_credits mc
JOIN media m ON mc.media_id = m.id
JOIN movies mov ON m.id = mov.media_id
WHERE mc.person_id = ? AND mc.role = 'actor'
ORDER BY mov.release_date DESC;
```

### Get Media in Collections

```sql
-- Get all media in a collection
SELECT m.*, mov.*, cm.sort_order
FROM collection_media cm
JOIN media m ON cm.media_id = m.id
JOIN movies mov ON m.id = mov.media_id
WHERE cm.collection_id = ?
ORDER BY cm.sort_order;

-- Get all collections for media
SELECT c.*
FROM collection_media cm
JOIN collections c ON cm.collection_id = c.id
WHERE cm.media_id = ?;
```

### Get Media by Studio/Network

```sql
-- Get all media from a studio
SELECT m.*, mov.*
FROM media_studios ms
JOIN media m ON ms.media_id = m.id
JOIN movies mov ON m.id = mov.media_id
WHERE ms.studio_id = ? AND ms.studio_type = 'production'
ORDER BY mov.release_date DESC;

-- Get all TV shows from a network
SELECT ts.*
FROM media_studios ms
JOIN tv_shows ts ON ms.media_id = ts.id
WHERE ms.studio_id = ? AND ms.studio_type = 'network';
```

### Get Media by Tags

```sql
-- Get all media with a specific tag
SELECT m.*, mt.*
FROM media_tags mtags
JOIN media m ON mtags.media_id = m.id
LEFT JOIN movies mt ON m.id = mt.media_id
WHERE mtags.tag_id = ?;

-- Get all tags for media
SELECT t.name
FROM media_tags mt
JOIN tags t ON mt.tag_id = t.id
WHERE mt.media_id = ?
ORDER BY t.name;
```

### Get Media by Genres

```sql
-- Get all media in a specific genre (using normalized genres table)
SELECT m.*, mov.*
FROM media_genres mg
JOIN media m ON mg.media_id = m.id
LEFT JOIN movies mov ON m.id = mov.media_id
WHERE mg.genre_id = ?
ORDER BY m.title;

-- Get all genres for media
SELECT g.name
FROM media_genres mg
JOIN genres g ON mg.genre_id = g.id
WHERE mg.media_id = ?
ORDER BY g.name;

-- Get genre statistics
SELECT g.name, COUNT(mg.media_id) as media_count
FROM genres g
LEFT JOIN media_genres mg ON g.id = mg.genre_id
GROUP BY g.id
ORDER BY media_count DESC;

-- Get movies by genre (using comma-separated field - legacy)
SELECT m.*, mov.*
FROM media m
JOIN movies mov ON m.id = mov.media_id
WHERE mov.genre LIKE '%Action%'
ORDER BY m.title;
```

### Get People and Their Work

```sql
-- Get all work by a person (across all media types)
SELECT 
    m.title,
    m.type,
    mc.role,
    mc.character_name,
    COALESCE(mov.year, te.air_date) as year
FROM people p
LEFT JOIN movie_credits mc ON p.id = mc.person_id
LEFT JOIN episode_credits ec ON p.id = ec.person_id
LEFT JOIN music_credits muc ON p.id = muc.person_id
JOIN media m ON m.id = COALESCE(mc.media_id, ec.media_id, muc.media_id)
LEFT JOIN movies mov ON m.id = mov.media_id
LEFT JOIN tv_episodes te ON m.id = te.media_id
WHERE p.id = ?
ORDER BY year DESC;

-- Get person's biography and stats
SELECT 
    p.*,
    COUNT(DISTINCT mc.media_id) as movie_count,
    COUNT(DISTINCT ec.media_id) as episode_count,
    COUNT(DISTINCT muc.media_id) as track_count
FROM people p
LEFT JOIN movie_credits mc ON p.id = mc.person_id
LEFT JOIN episode_credits ec ON p.id = ec.person_id
LEFT JOIN music_credits muc ON p.id = muc.person_id
WHERE p.id = ?
GROUP BY p.id;

-- Find actors who worked together
SELECT 
    p1.name as actor1,
    p2.name as actor2,
    m.title,
    mc1.character_name as character1,
    mc2.character_name as character2
FROM movie_credits mc1
JOIN movie_credits mc2 ON mc1.media_id = mc2.media_id
JOIN people p1 ON mc1.person_id = p1.id
JOIN people p2 ON mc2.person_id = p2.id
JOIN media m ON mc1.media_id = m.id
WHERE mc1.person_id < mc2.person_id
  AND mc1.role = 'actor'
  AND mc2.role = 'actor'
  AND mc1.person_id = ?
ORDER BY m.title;
```

### Get Watch History

```sql
-- Get user's complete watch history
SELECT m.title, wh.watched_at, wh.duration_seconds, wh.completed
FROM watch_history wh
JOIN media m ON wh.media_id = m.id
WHERE wh.user_id = ?
ORDER BY wh.watched_at DESC
LIMIT 50;

-- Get most watched media
SELECT m.title, COUNT(*) as watch_count, MAX(wh.watched_at) as last_watched
FROM watch_history wh
JOIN media m ON wh.media_id = m.id
WHERE wh.user_id = ?
GROUP BY m.id
ORDER BY watch_count DESC
LIMIT 20;
```

### Get Ratings

```sql
-- Get all ratings for media
SELECT rating_type, rating_value, rating_scale, vote_count
FROM ratings
WHERE media_id = ?;

-- Get user's rating
SELECT rating_value FROM ratings
WHERE media_id = ? AND rating_type = 'user' AND user_id = ?;

-- Get average external rating
SELECT AVG(rating_value * 10.0 / rating_scale) as normalized_rating
FROM ratings
WHERE media_id = ? AND rating_type IN ('imdb', 'tmdb', 'rt_critics');

-- Get top rated movies
SELECT m.title, AVG(r.rating_value * 10.0 / r.rating_scale) as avg_rating
FROM ratings r
JOIN media m ON r.media_id = m.id
WHERE r.rating_type IN ('imdb', 'tmdb') AND m.type = 'movie'
GROUP BY m.id
HAVING COUNT(r.id) >= 2
ORDER BY avg_rating DESC
LIMIT 20;
```

### Get Recently Watched Media

```sql
SELECT m.*, wp.*
FROM watch_progress wp
JOIN media m ON wp.media_id = m.id
WHERE wp.user_id = ? OR wp.user_id IS NULL
ORDER BY wp.last_watched DESC
LIMIT 20;
```

### Get Media Needing Transcoding

```sql
SELECT m.*
FROM media m
LEFT JOIN transcode_jobs tj ON m.id = tj.media_id AND tj.quality = '720p'
WHERE m.has_dash = FALSE
  AND (tj.id IS NULL OR tj.status = 'failed');
```

### Search Across All Media

```sql
SELECT m.*, 'movie' as media_type
FROM media m
JOIN movies mov ON m.id = mov.media_id
WHERE m.title LIKE ?

UNION ALL

SELECT m.*, 'tv_episode' as media_type
FROM media m
JOIN tv_episodes e ON m.id = e.media_id
WHERE m.title LIKE ? OR e.episode_title LIKE ?

UNION ALL

SELECT m.*, 'music_track' as media_type
FROM media m
JOIN music_tracks mt ON m.id = mt.media_id
WHERE m.title LIKE ? OR mt.artist LIKE ? OR mt.album LIKE ?;
```

### Get Active Plugins

```sql
-- Get all enabled plugins
SELECT * FROM plugins
WHERE enabled = 1
ORDER BY plugin_type, name;

-- Get plugins by type
SELECT * FROM plugins
WHERE plugin_type = 'metadata_provider' AND enabled = 1;

-- Get plugin health status
SELECT name, version, plugin_type, health_status, last_health_check
FROM plugins
WHERE enabled = 1
ORDER BY CASE health_status 
    WHEN 'unhealthy' THEN 1 
    WHEN 'unknown' THEN 2 
    WHEN 'healthy' THEN 3 
END;
```

### Get Plugin Data and Events

```sql
-- Get plugin configuration
SELECT key, value FROM plugin_data
WHERE plugin_id = ? AND expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP
ORDER BY key;

-- Get recent plugin events
SELECT event_type, severity, message, created_at
FROM plugin_events
WHERE plugin_id = ?
ORDER BY created_at DESC
LIMIT 50;

-- Get plugin errors
SELECT event_type, message, event_data, created_at
FROM plugin_events
WHERE plugin_id = ? AND severity IN ('error', 'critical')
ORDER BY created_at DESC;
```

### Get Plugin Hooks

```sql
-- Get hooks for a specific event
SELECT p.name, ph.priority, ph.filter_conditions
FROM plugin_hooks ph
JOIN plugins p ON ph.plugin_id = p.id
WHERE ph.hook_event = 'media.added' AND ph.enabled = 1 AND p.enabled = 1
ORDER BY ph.priority DESC;

-- Get all hooks for a plugin
SELECT hook_event, priority, enabled, filter_conditions
FROM plugin_hooks
WHERE plugin_id = ?
ORDER BY hook_event, priority DESC;
```

---

## Migration Strategy

### Automatic Migration Execution

**Strategy**: Auto-run on startup with version check

**Behavior**:
- Check database version on application startup
- Only run migrations if database version < code version
- Skip if already current
- Automatic backup before applying migrations

**Configuration**:
- Default: `AUTO_MIGRATE=true`
- Production override: `AUTO_MIGRATE=false` for manual control

**Safety**:
```go
// Check current version
currentVersion, err := migrate.Version()

// Only run if behind
if currentVersion < targetVersion {
    // Backup database first
    backupDatabase()
    
    // Apply migrations
    migrate.Up()
}
```

---

### Initial Migration (000001_init.up.sql)

Creates core tables:
- `libraries` table
- `media` table (base hybrid table with technical metadata)
- `movies` table (with content ratings, external IDs, localization)
- `watch_progress` table
- `transcode_jobs` table

### TV Shows Migration (000002_add_tv_shows.up.sql)

Adds TV show hierarchy:
- `tv_shows` table (with status, network, content ratings, external IDs)
- `tv_seasons` table
- `tv_episodes` table (with season_id FK, multiple numbering systems, external IDs)

### Music Migration (000003_add_music.up.sql)

Adds music support:
- `music_tracks` table (with extended metadata: ISRC, MusicBrainz IDs, release types)

### Metadata Migration (000004_add_metadata.up.sql)

Adds metadata caching:
- `metadata_cache` table

### People & Credits Migration (000005_add_people.up.sql)

Adds cast and crew:
- `people` table
- `movie_credits` table
- `episode_credits` table
- `music_credits` table

### Collections & Genres Migration (000006_add_collections.up.sql)

Adds grouping features:
- `collections` table
- `collection_media` junction table
- `genres` table (if normalizing genre field)

### Media Assets Migration (000007_add_media_assets.up.sql)

Adds rich media metadata:
- `images` table (posters, backdrops, banners, logos)
- `audio_tracks` table
- `subtitle_tracks` table
- `external_subtitles` table

### Studios Migration (000008_add_studios.up.sql)

Adds production company tracking:
- `studios` table
- `media_studios` junction table

### User Features Migration (000009_add_user_features.up.sql)

Adds user-generated content:
- `tags` table
- `media_tags` junction table
- `watch_history` table
- `ratings` table

### Localization & Releases Migration (000010_add_localization.up.sql)

Adds multi-language and release information:
- `alternative_titles` table
- `release_dates` table

### Media Versions Migration (000011_add_versions.up.sql)

Adds support for different cuts and editions:
- `media_versions` table (theatrical, director's cut, extended, etc.)

### Awards Migration (000012_add_awards.up.sql)

Adds awards tracking:
- `awards` table (for both media and people)

### Playback Features Migration (000013_add_playback_features.up.sql)

Adds advanced playback functionality:
- `chapters` table
- `playback_markers` table (intro/outro/credits skip)

### Related Content Migration (000014_add_related_content.up.sql)

Adds trailers, extras, and recommendations:
- `trailers` table
- `extras` table (deleted scenes, behind-the-scenes, etc.)
- `similar_media` table (recommendations)

### Plugin System Migration (000015_add_plugin_system.up.sql)

Adds extensibility infrastructure:
- `plugins` table (plugin registry)
- `plugin_data` table (key-value store for plugins)
- `plugin_events` table (audit log)
- `plugin_hooks` table (event subscriptions)

### Down Migrations

Each `.down.sql` file reverses the corresponding `.up.sql`:
- Drop tables in reverse order (respect foreign keys)
- Remove indexes
- Example: `000015_add_plugin_system.down.sql` drops plugin_hooks, plugin_events, plugin_data, and plugins tables
- Example: `000014_add_related_content.down.sql` drops similar_media, extras, and trailers tables
- Example: `000001_init.down.sql` drops transcode_jobs, watch_progress, movies, media, and libraries tables

---

## Database Switching

### SQLite (Development/Single-User)

```go
db, err := sql.Open("sqlite3", "data/viewra2.db")
```

**Advantages**:
- ✅ No setup required
- ✅ File-based, portable
- ✅ Perfect for home servers
- ✅ Good performance for <100k records

**Limitations**:
- ❌ Single writer at a time
- ❌ Not ideal for >1M records

### PostgreSQL (Production/Multi-User)

```go
db, err := sql.Open("postgres", "postgres://user:pass@localhost/viewra2")
```

**Advantages**:
- ✅ Multi-user concurrency
- ✅ Advanced features (full-text search, JSON)
- ✅ Scales to millions of records
- ✅ Better tooling ecosystem

**Schema Adjustments**:
```sql
-- SQLite: INTEGER PRIMARY KEY AUTOINCREMENT
-- PostgreSQL: SERIAL PRIMARY KEY or BIGSERIAL

-- SQLite: DATETIME
-- PostgreSQL: TIMESTAMP

-- SQLite: BOOLEAN
-- PostgreSQL: BOOLEAN (native support)
```

---

## Database Backup Strategy

### Automatic Backups

**Daily Backups**:
- Schedule: 3:00 AM (configurable via `BACKUP_TIME`)
- Location: `data/backups/viewra_YYYY-MM-DD.db`
- Retention: Keep last 7 days
- Automatic cleanup of old backups

**Pre-Migration Backups**:
- Automatic backup before running any migration
- Location: `data/backups/viewra_pre_migration_<version>.db`
- Kept permanently (manual cleanup)

**Manual Backups**:
- Admin UI button: "Backup Database Now"
- API endpoint: `POST /api/admin/backup`
- Same location as automatic backups

**Implementation**:
```go
func BackupDatabase(dbPath, backupPath string) error {
    // SQLite backup using VACUUM INTO
    _, err := db.Exec(fmt.Sprintf("VACUUM INTO '%s'", backupPath))
    return err
}

// Scheduled backup
func StartBackupScheduler() {
    ticker := time.NewTicker(24 * time.Hour)
    go func() {
        for range ticker.C {
            if time.Now().Hour() == 3 {
                BackupDatabase()
                CleanupOldBackups(7) // Keep 7 days
            }
        }
    }()
}
```

---

## Transcode File Management

### LRU Cache Strategy

**Disk Limit**: Configurable maximum for transcoded DASH segments
- Default: 50GB
- Environment variable: `TRANSCODE_CACHE_SIZE_GB`

**Cleanup Behavior**:
- Track last access time for each transcode job
- When approaching limit (90% full), delete oldest accessed files
- Keep job records in database for history
- Re-transcode if requested again

**Implementation**:
```sql
-- Track access time
UPDATE transcode_jobs 
SET last_accessed_at = CURRENT_TIMESTAMP 
WHERE id = ?;

-- Find files to clean when limit reached
SELECT id, dash_manifest_path, file_size
FROM transcode_jobs
WHERE status = 'completed'
ORDER BY last_accessed_at ASC
LIMIT ?;
```

**Background Cleanup Task**:
```go
func CleanupTranscodedFiles() {
    // Check disk usage
    usage := getTranscodeDiskUsage()
    limit := getTranscodeLimitGB() * 1024 * 1024 * 1024
    
    if usage > limit * 0.9 { // 90% threshold
        // Delete oldest files
        toDelete := usage - (limit * 0.8) // Target 80%
        deleteOldestTranscodes(toDelete)
    }
}
```

---

## Performance Optimization

### Index Strategy

**Always Index**:
- Foreign keys
- Frequently filtered columns (type, status)
- Sort columns (title, created_at)
- Unique constraints (file_path)

**Consider Indexing**:
- Search fields (title, artist, album)
- Composite indexes for common query patterns

**Avoid Over-Indexing**:
- Columns rarely used in WHERE/ORDER BY
- Low-cardinality columns (boolean with few distinct values)

### Query Optimization

1. **Use EXPLAIN QUERY PLAN** to analyze queries
2. **Limit result sets** - Always paginate
3. **Select only needed columns** - Avoid `SELECT *` in production
4. **Use prepared statements** - sqlc generates these automatically
5. **Batch operations** - Insert multiple records in one transaction

### Connection Pooling

```go
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(5 * time.Minute)
```

---

## Data Integrity

### Constraints

- **Foreign Keys**: Enforce referential integrity
- **Unique Constraints**: Prevent duplicates
- **Check Constraints**: Validate enum values
- **NOT NULL**: Ensure required fields

### Cascade Deletes

- Library deleted → All media deleted
- Media deleted → Type-specific data deleted
- Media deleted → Watch progress deleted
- Show deleted → All episodes deleted

### Transaction Management

Critical operations wrapped in transactions:
- Scanning library (multiple inserts)
- Deleting library (cascade operations)
- Moving episodes between shows

---

## Future Considerations

### Planned Additions

1. **Users Table** (Multi-user support)

```sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    email TEXT,
    role TEXT CHECK(role IN ('admin', 'user')) DEFAULT 'user',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_username ON users(username);
```

**Features:**
- User authentication and authorization
- Per-user watch progress and history
- Per-user ratings and tags
- Admin controls for library management

**Changes Required:**
- Update `watch_progress` to use FK to users table
- Update `watch_history` to use FK to users table
- Update `ratings` to use FK to users table
- Add user authentication middleware

2. **Playlists**

```sql
CREATE TABLE playlists (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    is_public BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE playlist_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    playlist_id INTEGER NOT NULL,
    media_id INTEGER NOT NULL,
    position INTEGER NOT NULL,
    added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (playlist_id) REFERENCES playlists(id) ON DELETE CASCADE,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    UNIQUE(playlist_id, position)
);

CREATE INDEX idx_playlist_items_playlist_id ON playlist_items(playlist_id);
CREATE INDEX idx_playlist_items_media_id ON playlist_items(media_id);
```

**Features:**
- User-created playlists
- Ordered media items
- Public/private sharing
- Smart playlists based on filters

3. **Advanced Metadata Features**

- **Chapter Markers:** Track chapter information for movies/episodes
- **Lyrics:** Store synchronized lyrics for music tracks
- **Alternate Versions:** Track different cuts (theatrical, director's, extended)
- **Trivia/Facts:** Store interesting facts about media
- **Parental Ratings:** Content ratings (G, PG, PG-13, R, TV-MA, etc.)

4. **Social Features** (Phase 3+)

- Watch parties with synchronized playback
- User reviews and comments
- Activity feeds
- Recommendations based on viewing history
- Shared libraries across users

### Scaling Strategies

- **Partitioning:** Partition large tables by `library_id` or date ranges
- **Archiving:** Archive old `watch_history` records after 2 years
- **Read Replicas:** Separate read replicas for metadata queries
- **Materialized Views:** Cache complex aggregations (top rated, most watched)
- **Sharding:** Shard by user_id when scaling to many users

### Performance Optimizations

- Add composite indexes for common filter combinations
- Use covering indexes to avoid table lookups
- Implement query result caching at application level
- Consider denormalizing frequently accessed data
- Use database connection pooling with appropriate limits

---