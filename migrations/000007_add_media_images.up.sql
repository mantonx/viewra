-- Migration: Add media_images table for image asset tracking
-- Created: 2025-11-15
-- Description: Polymorphic table to track local and external images for all media types

CREATE TABLE IF NOT EXISTS media_images (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    -- Polymorphic association: either media_id OR (media_type + entity_id)
    media_id INTEGER,                       -- FK to media.id (for movies, episodes, tracks with media files)
    media_type TEXT NOT NULL CHECK(media_type IN (
        'movie', 'tv_show', 'tv_season', 'tv_episode',
        'music_artist', 'music_album', 'music_track'
    )),
    entity_id INTEGER NOT NULL,             -- ID in specific table (tv_shows.id, albums, etc.)

    -- Image classification
    image_type TEXT NOT NULL CHECK(image_type IN (
        'poster', 'fanart', 'backdrop', 'banner', 'clearlogo', 'landscape',
        'thumb', 'discart', 'cover', 'folder', 'logo',
        'actor', 'extrafanart', 'characterart', 'clearart'
    )),

    -- Source tracking
    source_type TEXT NOT NULL CHECK(source_type IN (
        'local', 'tmdb', 'musicbrainz', 'tvdb', 'fanart.tv', 'manual'
    )),

    -- File locations
    file_path TEXT,                         -- For local files: absolute or relative path
    external_url TEXT,                      -- For downloaded images: original URL
    local_cache_path TEXT,                  -- For external images: cached local path

    -- Image metadata
    width INTEGER,                          -- Image width in pixels
    height INTEGER,                         -- Image height in pixels
    file_size_bytes BIGINT,                 -- File size for cleanup decisions
    mime_type TEXT,                         -- image/jpeg, image/png, image/webp, etc.
    file_hash TEXT,                         -- SHA256 hash for deduplication/integrity

    -- Multi-language and priority
    language TEXT,                          -- For multi-language posters (en, fr, ja, etc.)
    priority INTEGER DEFAULT 0,             -- For multiple images of same type (0=highest)

    -- Timestamps
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    -- Foreign key constraints
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

-- Index for querying images by media item
CREATE INDEX IF NOT EXISTS idx_media_images_media_id
    ON media_images(media_id);

-- Index for querying images by entity (shows, seasons, albums, artists)
CREATE INDEX IF NOT EXISTS idx_media_images_entity
    ON media_images(media_type, entity_id);

-- Index for filtering by image type
CREATE INDEX IF NOT EXISTS idx_media_images_type
    ON media_images(image_type);

-- Index for filtering by source
CREATE INDEX IF NOT EXISTS idx_media_images_source
    ON media_images(source_type);

-- Index for cleanup operations (find by hash)
CREATE INDEX IF NOT EXISTS idx_media_images_hash
    ON media_images(file_hash)
    WHERE file_hash IS NOT NULL;

-- Unique constraint: prevent duplicate images per media item
-- Allows multiple images of same type via priority system
CREATE UNIQUE INDEX IF NOT EXISTS idx_media_images_unique
    ON media_images(media_id, image_type, priority)
    WHERE media_id IS NOT NULL;

-- Unique constraint for entity-based images (shows, seasons, albums)
CREATE UNIQUE INDEX IF NOT EXISTS idx_media_images_entity_unique
    ON media_images(media_type, entity_id, image_type, priority)
    WHERE media_id IS NULL;
