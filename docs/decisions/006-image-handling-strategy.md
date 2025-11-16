# ADR 006: Image Handling Strategy

**Status**: Accepted
**Date**: 2025-11-15
**Author**: ViewRA Team
**Updated**: 2025-11-15 (Finalized design decisions)

## Context

ViewRA needs comprehensive image handling for media artwork. Survey of existing media libraries reveals substantial image assets:

### Existing Image Assets (Survey Results)

**Movies** (~2,155 movies):

- 2,155 posters (`poster.jpg`)
- 2,153 fanart/backdrops (`fanart.jpg`)
- 522 clearlogos (`clearlogo.png`)
- 234 landscapes (`landscape.jpg`)
- 412 movies with actor images (`.actors/Actor_Name.jpg`)
- 370 movies with extra thumbnails (`extrathumbs/thumb1.jpg`, etc.)

**TV Shows** (Chicago P.D. as example):

- Show-level: poster, fanart, banner, clearlogo, landscape
- Season-level: season01-poster.jpg, season02-poster.jpg, etc.
- Episode-level: 25,751+ episode thumbnails (`*-thumb.jpg`)

**Music** (~1,663 tracks from 25 artists):

- Album artwork: 5,653+ folder/cover images (`folder.jpg`, `cover.jpg`)
- Disc art: (`discart.png`, `discart.jpg`)
- Artist-level: fanart, logo, clearlogo, folder images

### Current State

- **Database**: No image columns in media tables
- **API**: No image serving endpoints
- **Frontend**: Using placeholder images
- **Scanner**: Not extracting or tracking image assets

### Requirements

1. **Storage**: Store image metadata (paths, types, dimensions) without duplicating files
2. **Serving**: Efficiently serve images with proper caching, resizing, format conversion
3. **Extraction**: Detect and catalog existing images during library scan
4. **Enrichment**: Download missing images from TMDb/MusicBrainz
5. **Types**: Support multiple image types per media item (poster, fanart, logo, etc.)
6. **Performance**: Fast serving with CDN-ready caching headers
7. **Flexibility**: Support both local files and future external URLs

## Decision

Implement a **hybrid reference-based image system** with the following architecture:

### 1. Database Schema

Create a new `media_images` table to track image assets:

```sql
CREATE TABLE media_images (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER,
    media_type TEXT NOT NULL CHECK(media_type IN ('movie', 'tv_show', 'tv_season', 'tv_episode', 'music_artist', 'music_album', 'music_track')),
    entity_id INTEGER NOT NULL,  -- ID in the specific entity table
    image_type TEXT NOT NULL CHECK(image_type IN (
        'poster', 'fanart', 'backdrop', 'banner', 'clearlogo', 'landscape',
        'thumb', 'discart', 'cover', 'folder', 'logo',
        'actor', 'extrafanart', 'characterart', 'clearart'
    )),
    source_type TEXT NOT NULL CHECK(source_type IN ('local', 'tmdb', 'musicbrainz', 'tvdb', 'fanart.tv', 'manual')),
    file_path TEXT,              -- For local files: relative or absolute path
    external_url TEXT,            -- For downloaded images: original URL
    local_cache_path TEXT,        -- For external images: cached local path
    width INTEGER,
    height INTEGER,
    file_size_bytes BIGINT,
    mime_type TEXT,
    file_hash TEXT,               -- SHA256 hash for deduplication and cache lookup
    language TEXT,                -- For multi-language posters
    priority INTEGER DEFAULT 0,   -- For multiple images of same type
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

CREATE INDEX idx_media_images_media_id ON media_images(media_id);
CREATE INDEX idx_media_images_entity ON media_images(media_type, entity_id);
CREATE INDEX idx_media_images_type ON media_images(image_type);
CREATE INDEX idx_media_images_hash ON media_images(file_hash);
CREATE INDEX idx_media_images_source ON media_images(source_type);
CREATE UNIQUE INDEX idx_media_images_unique ON media_images(media_id, image_type, priority) WHERE media_id IS NOT NULL;
CREATE INDEX idx_media_images_type_entity ON media_images(media_type, entity_id, image_type);
```

**Design Rationale**:

- **Reference-based**: Store paths, not binary data (avoid bloating database)
- **Polymorphic**: Single table handles all media types via `media_type` + `entity_id`
- **Multi-source**: Track whether image is local file or downloaded from external API
- **Flexible**: Support both `media_id` (for media files) and entity-specific IDs (for shows, seasons, artists, albums)
- **Priority system**: Allow multiple images per type, user can set preferred

### 2. Image Types by Media Type

**Movies**:

- `poster` - Main movie poster (2:3 aspect ratio)
- `fanart` - Backdrop/background (16:9)
- `clearlogo` - Transparent logo PNG
- `landscape` - Landscape poster (16:9)
- `banner` - Wide banner (varies)
- `actor` - Actor headshots (from `.actors/` folder)
- `extrafanart` - Additional backdrops (from `extrathumbs/`)

**TV Shows**:

- Show-level: `poster`, `fanart`, `banner`, `clearlogo`, `landscape`
- Season-level: `poster` (season-specific)
- Episode-level: `thumb` (episode thumbnail)

**Music**:

- Artist-level: `fanart`, `logo`, `clearlogo`, `folder` (artist image)
- Album-level: `cover`, `folder`, `discart`
- Track-level: Inherit from album

### 3. API Endpoints

```
GET /api/images/:id                    # Get image metadata by ID
GET /api/images/:id/file               # Serve actual image file (with resizing, caching)
GET /api/media/:id/images              # Get all images for media item
GET /api/movies/:id/images             # Get all images for movie
GET /api/tv/shows/:id/images           # Get all images for show
GET /api/tv/seasons/:id/images         # Get all images for season
GET /api/tv/episodes/:id/images        # Get all images for episode
GET /api/music/artists/:name/images    # Get all images for artist
GET /api/music/albums/:id/images       # Get all images for album

# Image serving with transformations
GET /api/images/:id/file?width=300&height=450&format=webp&quality=85

# Utilities
POST /api/images                       # Manually upload image
PUT /api/images/:id                    # Update image metadata
DELETE /api/images/:id                 # Delete image reference
POST /api/images/refresh/:media_id     # Re-scan for images
```

### 4. Image Serving Strategy

**Direct File Serving** (for local images):

```go
// Serve local file with caching headers
func (h *ImageHandler) ServeImage(c *gin.Context) {
    image := h.getImage(c.Param("id"))

    // Set caching headers (1 year for immutable content)
    c.Header("Cache-Control", "public, max-age=31536000, immutable")
    c.Header("ETag", image.ETag)

    // Optional: Resize on-the-fly using imaging library
    if width := c.Query("width"); width != "" {
        resized := h.resizeImage(image.FilePath, width, height)
        c.File(resized)
        return
    }

    c.File(image.FilePath)
}
```

**Image Transformations** (optional, Phase 4.5):

- On-demand resizing using `github.com/disintegration/imaging`
- Format conversion (JPEG → WebP for smaller sizes)
- Cache transformed images in `data/cache/images/`
- LRU cleanup similar to transcode cleanup

### 5. Scanner Integration

Update library scanner to detect and catalog images:

```go
// In processMovie()
func (s *ScanLibraryUseCase) processMovie(ctx context.Context, file FileInfo, libraryID int) {
    // ... existing movie processing ...

    // Extract images
    movieDir := filepath.Dir(file.Path)
    images := s.imageExtractor.ExtractMovieImages(movieDir)

    for _, img := range images {
        s.imageRepo.Create(ctx, ImageMetadata{
            MediaID:    mediaID,
            MediaType:  "movie",
            EntityID:   movieID,
            ImageType:  img.Type,      // "poster", "fanart", etc.
            SourceType: "local",
            FilePath:   img.Path,
            Width:      img.Width,
            Height:     img.Height,
            FileSize:   img.Size,
            MimeType:   img.MimeType,
        })
    }
}
```

**Image Detection Logic**:

```go
type ImageExtractor struct{}

func (e *ImageExtractor) ExtractMovieImages(movieDir string) []ImageInfo {
    var images []ImageInfo

    // Check for standard Kodi/Plex naming
    patterns := map[string]string{
        "poster":    "poster.*",
        "fanart":    "fanart.*",
        "clearlogo": "clearlogo.*",
        "landscape": "landscape.*",
        "banner":    "banner.*",
    }

    for imageType, pattern := range patterns {
        matches := filepath.Glob(filepath.Join(movieDir, pattern))
        for _, path := range matches {
            images = append(images, ImageInfo{
                Type: imageType,
                Path: path,
                // ... get dimensions, size, mime type
            })
        }
    }

    // Check for actor images
    actorDir := filepath.Join(movieDir, ".actors")
    if exists(actorDir) {
        actorImages := filepath.Glob(filepath.Join(actorDir, "*.jpg"))
        for _, path := range actorImages {
            images = append(images, ImageInfo{
                Type: "actor",
                Path: path,
                // Extract actor name from filename
            })
        }
    }

    // Check for extra thumbs
    thumbsDir := filepath.Join(movieDir, "extrathumbs")
    if exists(thumbsDir) {
        thumbs := filepath.Glob(filepath.Join(thumbsDir, "thumb*.jpg"))
        for i, path := range thumbs {
            images = append(images, ImageInfo{
                Type:     "extrafanart",
                Path:     path,
                Priority: i,
            })
        }
    }

    return images
}
```

### 6. Frontend Integration

Update media cards to display images:

```typescript
// MovieCard.tsx
const MovieCard = ({ movie }: { movie: Movie }) => {
  const { data: images } = useQuery({
    queryKey: ['movie-images', movie.id],
    queryFn: () => api.getMovieImages(movie.id),
  });

  const poster = images?.find(img => img.image_type === 'poster');
  const fanart = images?.find(img => img.image_type === 'fanart');
  const clearlogo = images?.find(img => img.image_type === 'clearlogo');

  return (
    <div className="movie-card">
      {/* Poster with fallback */}
      <img
        src={poster ? `/api/images/${poster.id}/file?width=300&height=450` : '/placeholder-poster.jpg'}
        alt={movie.title}
      />

      {/* Fanart background */}
      {fanart && (
        <div
          className="backdrop"
          style={{backgroundImage: `url(/api/images/${fanart.id}/file?width=1280&height=720)`}}
        />
      )}

      {/* Clearlogo overlay */}
      {clearlogo && (
        <img src={`/api/images/${clearlogo.id}/file`} className="logo" />
      )}
    </div>
  );
};
```

### 7. External Enrichment (Phase 4.2)

For missing images, fetch from external APIs:

```go
// TMDb enrichment
func (s *TMDbService) EnrichMovieImages(ctx context.Context, movie *Movie) error {
    // Search TMDb for movie
    tmdbMovie := s.searchMovie(movie.Title, movie.Year)

    // Download missing images
    if !hasImage(movie.ID, "poster") && tmdbMovie.PosterPath != "" {
        posterURL := fmt.Sprintf("https://image.tmdb.org/t/p/original%s", tmdbMovie.PosterPath)
        localPath := s.downloadImage(posterURL, movie.ID, "poster")

        s.imageRepo.Create(ctx, ImageMetadata{
            MediaID:        movie.MediaID,
            MediaType:      "movie",
            EntityID:       movie.ID,
            ImageType:      "poster",
            SourceType:     "tmdb",
            ExternalURL:    posterURL,
            LocalCachePath: localPath,
        })
    }

    // Similar for fanart, clearlogo, etc.
    return nil
}
```

**Download Strategy**:

- Store downloaded images in `data/images/tmdb/`, `data/images/musicbrainz/`, etc.
- Organize by media type and ID: `data/images/tmdb/movies/12345/poster.jpg`
- Track `external_url` for re-downloading if needed
- Include in cleanup system (delete old/unused downloaded images)

### 7. Data Directory Structure

ViewRA organizes all data under `data/` with clear separation between persistent and ephemeral storage:

```text
data/
├── viewra.db                          # Persistent database (SQLite)
│                                      # Contains media_images table with paths and metadata
│
├── cache/                             # All ephemeral/regenerable data
│   ├── images/                        # Image cache (resized, downloaded)
│   │   ├── {hash}_original.jpg       # Downloaded or cached original
│   │   ├── {hash}_300x450.jpg        # Resized thumbnail
│   │   ├── {hash}_1280x720.jpg       # Resized fanart
│   │   └── {hash}_1920x1080.webp     # WebP conversion
│   │
│   └── transcodes/                    # On-demand transcoded media
│       ├── dash/
│       │   └── {media-id}/
│       │       ├── manifest.mpd
│       │       └── *.m4s
│       └── hls/
│           └── {media-id}/
│               ├── master.m3u8
│               └── *.ts
│
└── logs/                              # Application logs (optional)
    └── viewra.log
```

**Original Media Files** (not managed by ViewRA):

```text
/media/Movies/The Matrix (1999)/
├── The Matrix (1999).mkv              # Media file
├── poster.jpg                         # Local poster (referenced in DB)
├── fanart.jpg                         # Local fanart (referenced in DB)
├── clearlogo.png                      # Local logo (referenced in DB)
└── actors/
    └── keanu-reeves.jpg               # Actor image (referenced in DB)

/media/TV Shows/Breaking Bad/Season 01/
├── S01E01.mkv
└── S01E01-thumb.jpg                   # Episode thumbnail (referenced in DB)

/media/Music/Artist/Album/
├── cover.jpg                          # Album cover (referenced in DB)
└── 01 - Track.flac
```

**Design Principles**:

- **Persistent data** (`viewra.db`): Must be backed up, contains all metadata
- **Cache** (`data/cache/`): Can be deleted safely, will regenerate on-demand
- **Original files**: Never moved/modified, only referenced by absolute/relative paths
- **Hash-based cache**: Enables deduplication, source-agnostic storage
- **Unified cleanup**: Single cleanup job for all cache types (images + transcodes)

### 8. Image Lifecycle Management

**Critical**: Images require comprehensive lifecycle management across database AND filesystem.

#### Smart Deletion: Two-Layer Cleanup

When content is deleted, we must clean up in TWO places:

1. **Database Records** (`media_images` table)
   - Automatic via `ON DELETE CASCADE` foreign key
   - Removes image metadata instantly

2. **Cache Files** (`data/cache/images/`)
   - Manual via application logic
   - Only delete if hash not used elsewhere (deduplication-aware)
   - Pattern: `{hash}_*.jpg` (all size variants)

**Why Two Phases?**
- Multiple images can share same hash (deduplication)
- Can't delete cache file if another image references it
- Must check hash usage AFTER database deletion

#### Automatic Cascading Deletion

**Database Foreign Keys** (already implemented):
```sql
FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
```

When media is deleted, all associated `media_images` rows are automatically removed.

**Application-Level Cache Cleanup** (REQUIRED for filesystem):

```go
// When library is deleted
func (uc *DeleteLibraryUseCase) Execute(ctx context.Context, libraryID int64) error {
    // 1. Get all media in library
    mediaItems := uc.mediaRepo.ListByLibrary(ctx, libraryID)

    // 2. Delete images for each media item
    for _, media := range mediaItems {
        if err := uc.imageRepo.DeleteByMediaID(ctx, media.ID); err != nil {
            logger.Warn("Failed to delete images", "media_id", media.ID, "error", err)
        }
    }

    // 3. Clean orphaned cache files
    uc.cleanupOrphanedCacheFiles(ctx)

    // 4. Delete library
    return uc.libraryRepo.Delete(ctx, libraryID)
}

// When individual movie is deleted
func (uc *DeleteMovieUseCase) Execute(ctx context.Context, mediaID int64) error {
    // CRITICAL: Two-phase deletion for images

    // PHASE 1: Collect image hashes BEFORE database deletion
    images := uc.imageRepo.GetByMediaID(ctx, mediaID)
    hashes := extractHashes(images) // e.g., ["abc123...", "def456..."]

    // PHASE 2: Delete database records
    // This cascades to media_images table via ON DELETE CASCADE
    uc.movieRepo.DeleteMovie(ctx, mediaID)

    // PHASE 3: Clean cache files (only if hash not used by other records)
    for _, hash := range hashes {
        // Check if this hash is still referenced by other images
        remainingImages := uc.imageRepo.GetImagesByHash(ctx, hash)
        if len(remainingImages) == 0 {
            // No other images use this hash - safe to delete cache files
            uc.deleteCacheFilesForHash(ctx, hash)
            // Deletes: {hash}_original.jpg, {hash}_300x450.jpg, etc.
        }
    }

    return nil
}

// Helper: Delete all cache variants for a hash
func (uc *DeleteMovieUseCase) deleteCacheFilesForHash(ctx context.Context, hash string) error {
    cacheDir := "data/cache/images/"
    pattern := filepath.Join(cacheDir, hash+"_*")
    files, _ := filepath.Glob(pattern)

    for _, file := range files {
        if err := os.Remove(file); err != nil {
            logger.Warn("Failed to delete cache file",
                "file", file,
                "hash", hash,
                "error", err)
        }
    }

    return nil
}

// When TV show is deleted
func (uc *DeleteTVShowUseCase) Execute(ctx context.Context, showID int64) error {
    // 1. Get all seasons and episodes
    seasons := uc.tvRepo.GetSeasonsByShow(ctx, showID)

    // 2. Delete images for show, seasons, and episodes
    uc.imageRepo.DeleteByEntity(ctx, images.MediaTypeTVShow, showID)
    for _, season := range seasons {
        uc.imageRepo.DeleteByEntity(ctx, images.MediaTypeTVSeason, season.ID)

        episodes := uc.tvRepo.GetEpisodesBySeason(ctx, season.ID)
        for _, ep := range episodes {
            uc.imageRepo.DeleteByMediaID(ctx, ep.MediaID)
        }
    }

    // 3. Clean orphaned cache files
    uc.cleanupOrphanedCacheFiles(ctx)

    // 4. Delete show (cascades via FK)
    return uc.tvRepo.DeleteShow(ctx, showID)
}
```

#### Cache File Cleanup Strategy

**Orphan Detection**:
```go
// Find cache files not referenced in database
func (uc *CleanupCacheUseCase) FindOrphanedImages(ctx context.Context) ([]string, error) {
    // 1. Get all hashes from database
    dbHashes := uc.imageRepo.GetAllFileHashes(ctx)
    hashSet := make(map[string]bool)
    for _, hash := range dbHashes {
        hashSet[hash] = true
    }

    // 2. Scan cache directory
    cacheDir := "data/cache/images/"
    cacheFiles, _ := filepath.Glob(filepath.Join(cacheDir, "*"))

    var orphans []string
    for _, file := range cacheFiles {
        // Extract hash from filename: {hash}_300x450.jpg
        hash := extractHashFromFilename(file)
        if !hashSet[hash] {
            orphans = append(orphans, file)
        }
    }

    return orphans, nil
}

// Cleanup job runs periodically
func (uc *CleanupCacheUseCase) CleanOrphanedImages(ctx context.Context) error {
    orphans, _ := uc.FindOrphanedImages(ctx)

    for _, file := range orphans {
        if err := os.Remove(file); err != nil {
            logger.Warn("Failed to delete orphaned cache file", "path", file, "error", err)
            continue
        }
        logger.Info("Deleted orphaned cache file", "path", file)
    }

    return nil
}
```

**Cleanup Scheduler** (see [ADR 007: Unified Task Scheduler](007-unified-task-scheduler.md)):
```go
// Register with unified scheduler
scheduler.RegisterTask(scheduler.Task{
    ID:          "image-cache-cleanup",
    Name:        "Image Cache Cleanup",
    Description: "Remove orphaned image cache files",
    Schedule:    "0 3 * * *",  // Daily at 3 AM
    Enabled:     true,
    Handler: func(ctx context.Context) error {
        return cleanupUC.CleanOrphanedImages(ctx)
    },
})
```

#### Image Update/Refresh Strategy

**Rescan Triggers**:

1. **Manual rescan**: User triggers library rescan
   - Re-extract local images (update if changed)
   - Optionally re-fetch external images

2. **File modification detected**:
   - If `poster.jpg` modified date changes, re-extract metadata
   - Update dimensions, hash, size in database

3. **New content added**:
   - New season of TV show → scan for new season posters
   - New episode → extract episode thumbnail

**Update Logic**:
```go
func (uc *UpdateImageUseCase) RefreshLocalImage(ctx context.Context, imagePath string) error {
    // 1. Check if image exists in DB
    existingImage := uc.imageRepo.GetByFilePath(ctx, imagePath)

    // 2. Extract fresh metadata
    metadata := uc.metadataExtractor.ExtractMetadata(imagePath)

    // 3. Compare hashes
    if existingImage != nil && existingImage.FileHash == metadata.FileHash {
        // No change, skip update
        return nil
    }

    // 4. File changed or new - update/create
    if existingImage != nil {
        // Update existing record
        existingImage.FileHash = metadata.FileHash
        existingImage.Width = metadata.Width
        existingImage.Height = metadata.Height
        existingImage.FileSizeBytes = metadata.FileSizeBytes
        existingImage.UpdatedAt = time.Now()
        return uc.imageRepo.Update(ctx, existingImage)
    }

    // Create new record
    return uc.imageRepo.Create(ctx, &images.Image{
        // ... populate from metadata
    })
}

// Rescan strategy for TV shows (new seasons)
func (uc *RescanTVShowUseCase) Execute(ctx context.Context, showID int64) error {
    // 1. Re-extract show-level images
    show := uc.tvRepo.GetShow(ctx, showID)
    showImages := uc.imageExtractor.ExtractTVShowImages(show.Path)

    // 2. Update existing or create new
    for _, imgInfo := range showImages.Images {
        uc.updateImageUC.RefreshLocalImage(ctx, imgInfo.Path)
    }

    // 3. Scan for new seasons
    seasons := uc.scanForSeasons(show.Path)
    for _, season := range seasons {
        // Extract season images
        seasonImages := uc.imageExtractor.ExtractTVSeasonImages(show.Path, season.Number)
        for _, imgInfo := range seasonImages.Images {
            uc.updateImageUC.RefreshLocalImage(ctx, imgInfo.Path)
        }
    }

    return nil
}
```

#### Cleanup Integration Points

1. **Library deletion** → Cascade delete images + cache cleanup
2. **Media deletion** → Cascade delete images + cache cleanup
3. **Library rescan** → Update changed images, remove missing images
4. **Scheduled cleanup** → Remove orphaned cache files
5. **Manual cleanup API** → `/api/admin/cleanup/images?dry_run=true`

#### Missing Image Detection

**Scanner Integration**:
```go
func (uc *ScanLibraryUseCase) detectMissingImages(ctx context.Context) {
    // Get all images with source_type = 'local'
    localImages := uc.imageRepo.ListBySource(ctx, images.SourceTypeLocal)

    for _, img := range localImages {
        // Check if file still exists
        if !fileExists(img.FilePath) {
            logger.Info("Image file missing, marking for cleanup",
                "path", img.FilePath,
                "media_id", img.MediaID,
                "image_type", img.ImageType)

            // Delete orphaned database record
            uc.imageRepo.Delete(ctx, img.ID)
        }
    }
}
```

## Consequences

### Positive

- **Efficient**: No database bloat, files served directly from filesystem
- **Flexible**: Support both local assets and external downloads
- **Scalable**: Can add CDN serving layer later
- **Comprehensive**: Handles all image types across all media types
- **User-friendly**: Frontend gets rich visual experience
- **Performance**: Proper caching headers, optional on-demand resizing

### Negative

- **Complexity**: Additional table and repository layer
- **Storage**: Downloaded images consume disk space (mitigated by cleanup)
- **Processing**: Image dimension detection adds scan time (minimal)
- **Dependencies**: May need image processing library for resizing/conversion

### Risks & Mitigations

1. **Disk Space**: Downloaded images could consume significant space
   - **Mitigation**: Extend cleanup system to manage image cache with LRU
   - **Mitigation**: Make external download opt-in, prioritize local assets

2. **Scan Performance**: Image detection adds overhead
   - **Mitigation**: Make image scanning optional/async
   - **Mitigation**: Cache image metadata, only re-scan on file changes

3. **Broken Paths**: Local files might move/delete
   - **Mitigation**: Validate paths before serving, return 404 gracefully
   - **Mitigation**: Periodic cleanup to remove orphaned image records

## Finalized Design Decisions

After evaluation of alternatives, the following decisions have been finalized:

### 1. Implementation Approach

**Decision**: Phase 4.1 first (local image infrastructure)

- Leverage existing 36,000+ local images immediately
- External APIs (TMDb, MusicBrainz) enhance in Phase 4.2
- Provides immediate visual value to users

### 2. Database Architecture

**Decision**: Polymorphic single table approach

- Single `media_images` table handles all media types
- Uses `media_type` + `entity_id` for flexibility
- Simpler than separate tables, easier to extend

### 3. Image Transformation Strategy

**Decision**: On-demand resizing with disk caching

- Resize images when requested via query params (`?width=300`)
- Cache transformed images to `data/cache/images/`
- Balance between flexibility and performance

### 4. Cache Storage Location

**Decision**: `data/cache/images/` with unified hash-based storage

- Consistent with `data/cache/transcodes/` structure
- **Hash-based filenames** for all images (local and external)
- Source-agnostic cache - database tracks origin via `source_type`
- Benefits:
  - **Deduplication**: Same image from different sources = single file
  - **Simpler structure**: No source-specific subdirectories
  - **Efficient cleanup**: Delete files not referenced in database
  - **Uniform URLs**: `/api/images/{hash}?size=300x450`

- Directory structure:
  ```
  data/cache/images/
  ├── a1b2c3d4e5f6...def_original.jpg       # Original (local or downloaded)
  ├── a1b2c3d4e5f6...def_300x450.jpg        # Resized thumbnail
  ├── a1b2c3d4e5f6...def_1280x720.jpg       # Resized fanart
  ├── f6e5d4c3b2a1...abc_original.jpg
  └── f6e5d4c3b2a1...abc_500x500.jpg
  ```

- Filename format: `{sha256_hash}_{size}.{ext}`
  - Hash from original file content
  - Size: `original`, `300x450`, `1920x1080`, etc.
  - Extension: `jpg`, `png`, `webp`

### 5. Scan Performance Strategy

**Decision**: Async background job for image extraction

- Image extraction runs after media scan completes
- Keeps library scans fast and responsive
- Consistent with existing async patterns (transcoding)

### 6. Image Metadata Extraction

**Decision**: Complete metadata extraction

- Extract: path, type, dimensions, file size, MIME type, hash
- Enables future features: deduplication, integrity checks, cleanup
- Slight overhead acceptable for comprehensive data

### 7. Frontend Display Strategy

**Decision**: Lazy loading with intersection observer

- Only load images when scrolled into view
- Browser-native `loading="lazy"` attribute
- Optimal for large collections (25,000+ episode thumbnails)

### 8. Image Fallback Priority

**Decision**: Local first (Plex/Jellyfin style)

- Priority: Local files → TMDb download → Placeholder
- Respects user's existing artwork curation
- TMDb enhances (fills gaps) rather than replaces

### 9. External API Integration Timing

**Decision**: Background job after scan

- Queue TMDb/MusicBrainz enrichment to run post-scan
- Async enrichment doesn't block library scans
- Batch API calls to respect rate limits

### 10. Cleanup System Integration

**Decision**: Extend existing transcode cleanup

- Single unified cleanup system for transcodes + images
- Shared configuration and scheduling
- Reuse LRU eviction patterns

### 11. Migration Strategy

**Decision**: Dedicated migration tool

- One-time command: `viewra migrate-images`
- Populates image metadata for existing libraries
- No full rescan required

### 12. Testing Approach

**Decision**: Unit tests for core functionality

- Test image extraction logic
- Test repository CRUD operations
- Test serving with caching headers
- Integration tests deferred to Phase 4.3

## Implementation Plan

**UPDATED 2025-11-16**: Phased implementation approach with deferred caching

See [PHASE_4_1_GAP_ANALYSIS.md](../PHASE_4_1_GAP_ANALYSIS.md) for detailed analysis of implementation vs specification.

### Phase 4.1: Core Image Infrastructure ✅ COMPLETE (Week 1)

**Scope**: Image cataloging and reference-based serving

1. **Database Migration** ✅
   - Create `media_images` table with cache support (schema ready)
   - Create repository interface and implementation
   - Update `media`, `movies`, `tv_episodes`, etc. to join with images

2. **Image Extraction** ✅
   - Implement `ImageExtractor` service with metadata extraction
   - Integrate into movie/TV/music scanner
   - Detect standard Kodi/Plex image files
   - Extract dimensions, hash, MIME type

3. **API Endpoints** ✅
   - `GET /api/images/:id/file` - Direct file serving from original paths
   - `GET /api/media/:id/images` - Get images for media
   - Proper caching headers (Cache-Control, ETag)

4. **Frontend Integration** ✅
   - Update MovieCard to display posters
   - Update TV episode cards to show thumbnails
   - Update music albums to show cover art
   - Add loading states and placeholder images

**Implementation Detail**: Images are cataloged in database with metadata but served directly from original file paths. The `LocalCachePath` field is reserved for Phase 4.3.

### Phase 4.2: External Enrichment & Scheduler (Week 2)

1. **Unified Task Scheduler** (ADR 007)
   - Cron-based task scheduler with admin API
   - Register image cleanup task (3 AM daily)
   - Task management UI

2. **TMDb Integration**
   - Search and match movies/TV shows
   - Download posters, backdrops, logos
   - Store metadata in database with source tracking

3. **MusicBrainz Integration**
   - Search and match artists/albums
   - Download cover art metadata

4. **Manual Management**
   - API endpoints to upload custom images
   - Priority system for multiple images per type
   - UI to manage/delete images

### Phase 4.3: Image Caching & Transformations (Week 3)

**Scope**: Implement hash-based cache as specified in original ADR

1. **Cache Service** 📋
   - Implement `CacheService` to copy images to `data/cache/images/`
   - Hash-based filenames: `{hash}_original.{ext}`
   - Populate `LocalCachePath` field in database
   - Background job to populate cache for existing images

2. **Image Deduplication** 📋
   - Share cache files across identical images (by hash)
   - Delete cache files only when last reference removed
   - Storage optimization for duplicate images

3. **Image Transformations** 📋
   - On-demand resizing (`?width=300&height=450`)
   - Format conversion (`?format=webp`)
   - Quality control (`?quality=85`)
   - Cache transformed images: `{hash}_300x450.jpg`, `{hash}_1920x1080.webp`

4. **Cache Management** 📋
   - LRU eviction for cache files
   - Disk space monitoring
   - Configurable cache size limits
   - Cleanup integration with existing orphan detection

5. **Serving Updates** 📋
   - Update `ServeImage` handler to prefer cache over original
   - Fallback to original file if cache missing
   - Generate cache on-demand if needed

6. **Performance** 📋
   - Progressive image loading (blur-up)
   - CDN integration support (CloudFlare, AWS CloudFront)
   - Lazy loading (already implemented in Phase 4.1)

## Alternative Approaches Considered

### Alternative 1: Store Image URLs Only (No Local Tracking)

**Approach**: Only track external URLs from TMDb/MusicBrainz, ignore local files.

**Rejected Because**:

- Ignores 36,000+ existing local images in media libraries
- Requires internet for all image serving
- External APIs have rate limits and may be unavailable
- Users with existing Kodi/Plex setups expect local images to work

### Alternative 2: Duplicate Images to ViewRA Directory

**Approach**: Copy/move all images to `data/images/` during scan.

**Rejected Because**:

- Wastes disk space (duplicate 5GB+ of images)
- Breaks compatibility with Kodi/Plex/Jellyfin
- Users expect images to stay with media files
- Complicates backup/restore

### Alternative 3: Embed Images in Database as BLOBs

**Approach**: Store image binary data in database.

**Rejected Because**:

- Massive database bloat (5GB+ images)
- Slower queries and backups
- Poor serving performance vs filesystem
- Can't leverage web server's static file serving

## Success Criteria

### Phase 4.1 ✅ COMPLETE

- ✅ Movie posters display in frontend (using local `poster.jpg` files)
- ✅ TV episode thumbnails display (using `*-thumb.jpg` files)
- ✅ Music album covers display (using `folder.jpg` files)
- ✅ Images served with proper caching headers (1 year TTL)
- ✅ Scanner catalogs all existing images in < 10 seconds per movie
- ✅ API can fetch images by media ID
- ✅ Frontend gracefully handles missing images (placeholder)
- ✅ Database schema supports caching (ready for Phase 4.3)
- ✅ Cleanup system handles missing cache gracefully

### Phase 4.2 📋

- 📋 TMDb can download missing movie posters
- 📋 MusicBrainz can download missing album art
- 📋 Unified scheduler runs image cleanup daily
- 📋 Manual image upload works
- 📋 Priority system for multiple images

### Phase 4.3 📋

- 📋 Images cached to `data/cache/images/` with hash-based filenames
- 📋 Deduplication works (same image shared across media)
- 📋 On-demand resizing works (`?width=300`)
- 📋 WebP conversion works (`?format=webp`)
- 📋 LRU eviction prevents disk exhaustion
- 📋 Cache serves faster than original files

## References

- Kodi Image Naming: https://kodi.wiki/view/Artwork
- Plex Image Naming: https://support.plex.tv/articles/200220677-local-media-assets-movies/
- TMDb Image API: https://developers.themoviedb.org/3/getting-started/images
- MusicBrainz Cover Art Archive: https://musicbrainz.org/doc/Cover_Art_Archive
