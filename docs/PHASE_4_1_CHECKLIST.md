# Phase 4.1: Image Handling Infrastructure - Implementation Checklist

**Status**: In Progress
**Started**: 2025-11-15
**Target Completion**: 1 week

## Overview

Implement core image infrastructure to catalog and serve 36,000+ existing local images from media libraries.

## Pre-Implementation Setup

- [x] Survey existing image assets (completed)
- [x] Design database schema (completed)
- [x] Finalize architecture decisions (completed)
- [x] Document ADR 006 (completed)
- [ ] Install required Go dependencies

### Dependencies to Install

```bash
go get github.com/disintegration/imaging
```

## Task 1: Database Migration (Priority: Critical)

**File**: `migrations/000007_add_media_images.up.sql`

### Subtasks

- [ ] Create migration file for `media_images` table
- [ ] Add all required columns with constraints
- [ ] Create indexes for performance
- [ ] Add unique constraint for media_id + image_type + priority
- [ ] Test migration on SQLite
- [ ] Create PostgreSQL version (if needed)
- [ ] Test rollback migration (down.sql)

### SQL Schema Checklist

```sql
CREATE TABLE media_images (
    [ ] id INTEGER PRIMARY KEY AUTO INCREMENT
    [ ] media_id INTEGER (nullable, FK to media.id)
    [ ] media_type TEXT NOT NULL
    [ ] entity_id INTEGER NOT NULL
    [ ] image_type TEXT NOT NULL
    [ ] source_type TEXT NOT NULL
    [ ] file_path TEXT
    [ ] external_url TEXT
    [ ] local_cache_path TEXT
    [ ] width INTEGER
    [ ] height INTEGER
    [ ] file_size_bytes BIGINT
    [ ] mime_type TEXT
    [ ] file_hash TEXT
    [ ] language TEXT
    [ ] priority INTEGER DEFAULT 0
    [ ] created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    [ ] updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
    [ ] FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);
```

### Indexes

- [ ] `idx_media_images_media_id` ON `(media_id)`
- [ ] `idx_media_images_entity` ON `(media_type, entity_id)`
- [ ] `idx_media_images_type` ON `(image_type)`
- [ ] `idx_media_images_source` ON `(source_type)`
- [ ] `idx_media_images_unique` UNIQUE ON `(media_id, image_type, priority)` WHERE `media_id IS NOT NULL`

### Validation

- [ ] Run migration on test database
- [ ] Verify all indexes created
- [ ] Test CASCADE delete behavior
- [ ] Verify constraints work (CHECK, NOT NULL)

---

## Task 2: Domain Layer (Priority: Critical)

### 2.1 Image Entity

**File**: `internal/domain/images/entity.go`

- [ ] Create `Image` struct with all fields
- [ ] Add validation methods (ValidateImageType, ValidateSourceType)
- [ ] Add helper methods (IsLocal, IsExternal, GetActualPath)
- [ ] Add constants for ImageType enum
- [ ] Add constants for SourceType enum
- [ ] Add constants for MediaType enum
- [ ] Document all fields with comments

### 2.2 Repository Interface

**File**: `internal/domain/images/repository.go`

- [ ] Define `ImageRepository` interface
- [ ] `Create(ctx, image) error`
- [ ] `GetByID(ctx, id) (*Image, error)`
- [ ] `GetByMediaID(ctx, mediaID) ([]*Image, error)`
- [ ] `GetByEntity(ctx, mediaType, entityID) ([]*Image, error)`
- [ ] `GetByTypeAndEntity(ctx, mediaType, entityID, imageType) (*Image, error)`
- [ ] `Update(ctx, image) error`
- [ ] `Delete(ctx, id) error`
- [ ] `DeleteByMediaID(ctx, mediaID) error`
- [ ] `ListOrphans(ctx) ([]*Image, error)` - for cleanup

### 2.3 Value Objects

**File**: `internal/domain/images/types.go`

- [ ] `ImageType` type with constants (Poster, Fanart, etc.)
- [ ] `SourceType` type with constants (Local, TMDb, etc.)
- [ ] `MediaType` type with constants (Movie, TVShow, etc.)
- [ ] Validation functions for each type

---

## Task 3: Infrastructure - Image Extraction (Priority: High)

### 3.1 Image Metadata Extractor

**File**: `internal/infrastructure/images/metadata.go`

- [ ] Create `MetadataExtractor` struct
- [ ] `ExtractMetadata(filePath string) (*ImageMetadata, error)`
- [ ] Read image dimensions using `image.DecodeConfig`
- [ ] Get file size from `os.Stat`
- [ ] Detect MIME type from file extension
- [ ] Calculate file hash (SHA256)
- [ ] Handle errors gracefully (corrupted images, permissions)
- [ ] Support formats: JPG, PNG, WebP, GIF

### 3.2 Image Extractor Service

**File**: `internal/infrastructure/images/extractor.go`

- [ ] Create `ImageExtractor` struct
- [ ] Inject `MetadataExtractor` dependency
- [ ] `ExtractMovieImages(movieDir string) ([]ImageInfo, error)`
  - [ ] Detect poster.jpg/png
  - [ ] Detect fanart.jpg/png
  - [ ] Detect clearlogo.png
  - [ ] Detect landscape.jpg
  - [ ] Detect banner.jpg
  - [ ] Scan `.actors/` directory
  - [ ] Scan `extrathumbs/` directory
- [ ] `ExtractTVShowImages(showDir string) ([]ImageInfo, error)`
  - [ ] Detect show-level: poster, fanart, banner, clearlogo, landscape
  - [ ] Detect season posters (seasonXX-poster.jpg)
- [ ] `ExtractTVEpisodeImages(episodeFile string) ([]ImageInfo, error)`
  - [ ] Detect episode thumbnail (*-thumb.jpg)
- [ ] `ExtractMusicArtistImages(artistDir string) ([]ImageInfo, error)`
  - [ ] Detect fanart.jpg
  - [ ] Detect logo.png/clearlogo.png
  - [ ] Detect folder.jpg
- [ ] `ExtractMusicAlbumImages(albumDir string) ([]ImageInfo, error)`
  - [ ] Detect folder.jpg/cover.jpg
  - [ ] Detect discart.png/jpg
- [ ] Add logging for found/missing images
- [ ] Handle case-insensitive filenames (poster.JPG, POSTER.jpg)

### 3.3 Image Info Struct

**File**: `internal/infrastructure/images/types.go`

- [ ] Create `ImageInfo` struct
- [ ] Fields: Type, Path, Width, Height, Size, MimeType, Hash
- [ ] `ToImage(mediaID, mediaType, entityID) *domain.Image` converter

---

## Task 4: Infrastructure - Repository Implementation (Priority: Critical)

### 4.1 SQLite Implementation

**File**: `internal/infrastructure/persistence/image/sqlite_repository.go`

- [ ] Create `ImageRepository` struct
- [ ] Inject `*sqlx.DB` dependency
- [ ] Implement all repository interface methods
- [ ] Use sqlc-generated queries (after creating queries)
- [ ] Add proper error handling
- [ ] Add logging for database operations

### 4.2 SQLC Queries

**File**: `internal/infrastructure/database/queries/sqlite/images.sql`

- [ ] `-- name: CreateImage :one` - INSERT with RETURNING
- [ ] `-- name: GetImageByID :one` - SELECT by ID
- [ ] `-- name: ListImagesByMediaID :many` - SELECT by media_id
- [ ] `-- name: ListImagesByEntity :many` - SELECT by media_type + entity_id
- [ ] `-- name: GetImageByTypeAndEntity :one` - SELECT single image by type
- [ ] `-- name: UpdateImage :exec` - UPDATE statement
- [ ] `-- name: DeleteImage :exec` - DELETE by ID
- [ ] `-- name: DeleteImagesByMediaID :exec` - DELETE by media_id
- [ ] `-- name: ListOrphanImages :many` - Find images with missing files

### 4.3 Generate SQL Code

- [ ] Run `sqlc generate`
- [ ] Verify generated code in `internal/infrastructure/database/sqlc_sqlite/images.sql.go`
- [ ] Fix any sqlc errors

### 4.4 PostgreSQL (Optional for Phase 4.1)

- [ ] Create PostgreSQL queries if dual-DB support needed
- [ ] Otherwise, defer to Phase 4.2

---

## Task 5: Application Layer - Use Cases (Priority: High)

### 5.1 Get Images Use Case

**File**: `internal/application/images/get_images.go`

- [ ] Create `GetImagesUseCase` struct
- [ ] Inject `ImageRepository`
- [ ] `GetByMediaID(ctx, mediaID) ([]*Image, error)`
- [ ] `GetByEntity(ctx, mediaType, entityID) ([]*Image, error)`
- [ ] Add caching (optional, can defer)
- [ ] Add logging

### 5.2 Extract Images Use Case

**File**: `internal/application/images/extract_images.go`

- [ ] Create `ExtractImagesUseCase` struct
- [ ] Inject `ImageExtractor`, `ImageRepository`
- [ ] `ExtractForMovie(ctx, mediaID, movieID, moviePath) error`
- [ ] `ExtractForTVEpisode(ctx, mediaID, episodeID, episodePath) error`
- [ ] `ExtractForMusicAlbum(ctx, albumName, albumPath) error`
- [ ] Batch insert images for performance
- [ ] Handle duplicate detection (same file path)
- [ ] Add logging and error handling

### 5.3 Background Job for Image Extraction

**File**: `internal/application/images/extract_job.go`

- [ ] Create `ImageExtractionJob` struct
- [ ] Run as goroutine after media scan completes
- [ ] Queue media items to process
- [ ] Process in batches (e.g., 10 items at a time)
- [ ] Add progress tracking (optional)
- [ ] Add cancellation support (context.Context)

---

## Task 6: API Layer - Handlers (Priority: High)

### 6.1 Image Serving Handler

**File**: `internal/api/handlers/images.go`

- [ ] Create `ImageHandler` struct
- [ ] Inject `ImageRepository`, `Config`
- [ ] `ServeImage(c *gin.Context)` - GET /api/images/:id/file
  - [ ] Get image by ID
  - [ ] Validate file exists
  - [ ] Set caching headers (Cache-Control, ETag)
  - [ ] Support `?width` and `?height` query params (defer resizing to Phase 4.3)
  - [ ] Serve file using `c.File()`
  - [ ] Handle 404 gracefully
- [ ] `GetImageMetadata(c *gin.Context)` - GET /api/images/:id
  - [ ] Return JSON with image metadata
- [ ] `GetMediaImages(c *gin.Context)` - GET /api/media/:id/images
  - [ ] Get all images for media item
  - [ ] Return JSON array
- [ ] `GetMovieImages(c *gin.Context)` - GET /api/movies/:id/images
  - [ ] Get all images for movie (by movie ID)
- [ ] `GetTVEpisodeImages(c *gin.Context)` - GET /api/tv/episodes/:id/images
  - [ ] Get all images for TV episode

### 6.2 Swagger Documentation

- [ ] Add Swagger comments to all handler methods
- [ ] Document query parameters (width, height)
- [ ] Document response schemas
- [ ] Run `swag init`

---

## Task 7: API Layer - Routes (Priority: High)

**File**: `internal/api/routes/images.go`

- [ ] Create `RegisterImageRoutes(router *gin.RouterGroup, handler *ImageHandler)`
- [ ] `GET /api/images/:id` - Get metadata
- [ ] `GET /api/images/:id/file` - Serve image
- [ ] `GET /api/media/:id/images` - Get all for media
- [ ] `GET /api/movies/:id/images` - Get all for movie
- [ ] `GET /api/tv/episodes/:id/images` - Get all for episode
- [ ] Add to main server router

**File**: `internal/api/server.go`

- [ ] Wire up `ImageHandler` in container
- [ ] Register image routes

---

## Task 8: Scanner Integration (Priority: High)

### 8.1 Integrate into Movie Scanner

**File**: `internal/application/library/scan_library.go`

- [ ] In `processMovie()`, call `ExtractImagesUseCase.ExtractForMovie()`
- [ ] Run async in goroutine (defer to background job)
- [ ] Add error logging (don't fail scan on image errors)

### 8.2 Integrate into TV Scanner

- [ ] In `processTVEpisode()`, call `ExtractImagesUseCase.ExtractForTVEpisode()`
- [ ] Run async in goroutine
- [ ] Extract show/season images when creating show/season

### 8.3 Integrate into Music Scanner

- [ ] In music processing, call `ExtractImagesUseCase.ExtractForMusicAlbum()`
- [ ] Run async in goroutine

### 8.4 Background Job Trigger

- [ ] After scan completes, trigger `ImageExtractionJob`
- [ ] Queue all newly scanned media items
- [ ] Process queue in background

---

## Task 9: Migration Tool (Priority: Medium)

**File**: `cmd/migrate-images/main.go`

- [ ] Create CLI tool for one-time migration
- [ ] Connect to database
- [ ] Query all existing media items (movies, TV episodes, music)
- [ ] For each item, extract images
- [ ] Insert into `media_images` table
- [ ] Show progress bar (optional)
- [ ] Add dry-run flag
- [ ] Add verbose logging flag

### Usage

```bash
./bin/migrate-images --dry-run
./bin/migrate-images --verbose
```

---

## Task 10: Frontend - API Client (Priority: High)

### 10.1 TypeScript Types

**File**: `web/src/lib/types/images.ts`

- [ ] Create `Image` interface matching backend
- [ ] Create `ImageType` enum
- [ ] Create `SourceType` enum

### 10.2 API Client Functions

**File**: `web/src/lib/api/images.ts`

- [ ] `getMediaImages(mediaId: number): Promise<Image[]>`
- [ ] `getMovieImages(movieId: number): Promise<Image[]>`
- [ ] `getTVEpisodeImages(episodeId: number): Promise<Image[]>`
- [ ] Helper: `getImageUrl(imageId: number, width?: number, height?: number): string`
- [ ] Helper: `getPosterUrl(images: Image[]): string | undefined`
- [ ] Helper: `getFanartUrl(images: Image[]): string | undefined`

### 10.3 Generate from OpenAPI (if using Orval)

- [ ] Run `npm run generate:api` after adding Swagger docs
- [ ] Verify generated types and functions

---

## Task 11: Frontend - Components (Priority: High)

### 11.1 Update MovieCard

**File**: `web/src/components/media/MovieCard/MovieCard.tsx`

- [ ] Fetch images using `useQuery`
- [ ] Display poster image
- [ ] Add fanart background (optional)
- [ ] Add clearlogo overlay (optional)
- [ ] Add `loading="lazy"` attribute
- [ ] Show placeholder if no poster
- [ ] Handle image load errors

### 11.2 Update TVEpisodeCard

**File**: `web/src/components/tv/EpisodeCard/EpisodeCard.tsx`

- [ ] Fetch episode thumbnail
- [ ] Display with lazy loading
- [ ] Show placeholder if no thumb

### 11.3 Update MusicAlbumCard

**File**: `web/src/components/music/AlbumCard/AlbumCard.tsx`

- [ ] Fetch album cover
- [ ] Display with lazy loading
- [ ] Show placeholder if no cover

### 11.4 Create Placeholder Images

**File**: `web/public/placeholders/`

- [ ] Add `poster-placeholder.jpg` (2:3 ratio)
- [ ] Add `fanart-placeholder.jpg` (16:9 ratio)
- [ ] Add `thumb-placeholder.jpg`
- [ ] Add `album-placeholder.jpg`

---

## Task 12: Testing (Priority: Medium)

### 12.1 Unit Tests - Domain

**File**: `internal/domain/images/entity_test.go`

- [ ] Test Image entity validation
- [ ] Test ImageType validation
- [ ] Test SourceType validation
- [ ] Test helper methods (IsLocal, GetActualPath)

### 12.2 Unit Tests - Extractor

**File**: `internal/infrastructure/images/extractor_test.go`

- [ ] Test ExtractMovieImages with mock filesystem
- [ ] Test ExtractTVEpisodeImages
- [ ] Test ExtractMusicAlbumImages
- [ ] Test case-insensitive file matching
- [ ] Test missing images (should not error)

### 12.3 Unit Tests - Repository

**File**: `internal/infrastructure/persistence/image/repository_test.go`

- [ ] Test Create
- [ ] Test GetByID
- [ ] Test GetByMediaID
- [ ] Test GetByEntity
- [ ] Test Update
- [ ] Test Delete
- [ ] Test ListOrphans
- [ ] Use in-memory SQLite for tests

### 12.4 Unit Tests - Handlers

**File**: `internal/api/handlers/images_test.go`

- [ ] Test ServeImage with valid ID
- [ ] Test ServeImage with invalid ID (404)
- [ ] Test ServeImage with missing file (404)
- [ ] Test caching headers
- [ ] Test GetMediaImages
- [ ] Mock repository responses

### 12.5 Manual Testing

- [ ] Scan a movie with poster/fanart
- [ ] Verify images inserted in database
- [ ] Access /api/images/:id/file
- [ ] Verify image served correctly
- [ ] Check caching headers in browser
- [ ] Verify lazy loading works
- [ ] Test with missing images (placeholder display)

---

## Task 13: Documentation (Priority: Low)

- [ ] Update DATABASE_SCHEMA.md with `media_images` table
- [ ] Update API_SPECIFICATION.md with image endpoints
- [ ] Add usage examples to README (optional)
- [ ] Document environment variables (if any)

---

## Task 14: Build & Deploy Validation (Priority: Medium)

- [ ] Run `go build ./cmd/viewra`
- [ ] Verify binary builds successfully
- [ ] Run `npm run build` for frontend
- [ ] Verify frontend builds successfully
- [ ] Test migration on clean database
- [ ] Run full library scan with image extraction
- [ ] Verify images display in frontend
- [ ] Check for memory leaks (large image collections)
- [ ] Profile performance (optional)

---

## Dependencies & Blockers

**External Dependencies**:

- `github.com/disintegration/imaging` - For future image resizing (Phase 4.3)

**Internal Dependencies**:

- Existing scanner (`scan_library.go`) - Must integrate without breaking
- Existing domain entities (Media, Movie, TVEpisode) - Need IDs to associate
- Database migration system - Must support new table

**Potential Blockers**:

- Performance with 36,000+ images - May need batching
- Filesystem permissions - Images might not be readable
- Case-sensitive filesystems - Need case-insensitive matching

---

## Success Criteria

After completing Phase 4.1, the following should work:

- [x] `media_images` table exists in database
- [ ] Library scan catalogs all existing local images
- [ ] API endpoint `/api/images/:id/file` serves images with caching
- [ ] Frontend displays posters for movies
- [ ] Frontend displays thumbnails for TV episodes
- [ ] Frontend displays covers for music albums
- [ ] Lazy loading works (images load on scroll)
- [ ] Placeholder images display when no artwork found
- [ ] Migration tool can populate images for existing library
- [ ] Unit tests pass (>80% coverage for new code)
- [ ] Build succeeds (backend + frontend)

---

## Estimated Time Breakdown

| Task                               | Estimate |
| ---------------------------------- | -------- |
| 1. Database Migration              | 1 hour   |
| 2. Domain Layer                    | 2 hours  |
| 3. Image Extraction Infrastructure | 4 hours  |
| 4. Repository Implementation       | 3 hours  |
| 5. Application Use Cases           | 3 hours  |
| 6. API Handlers                    | 3 hours  |
| 7. Routes & Server Wiring          | 1 hour   |
| 8. Scanner Integration             | 2 hours  |
| 9. Migration Tool                  | 2 hours  |
| 10. Frontend API Client            | 2 hours  |
| 11. Frontend Components            | 4 hours  |
| 12. Testing                        | 4 hours  |
| 13. Documentation                  | 1 hour   |
| 14. Build & Validation             | 2 hours  |
| **Total**                          | **34 hours** (~1 week) |

---

## Next Steps After Phase 4.1

Once Phase 4.1 is complete, proceed to:

- **Phase 4.2**: External API Integration (TMDb, MusicBrainz)
- **Phase 4.3**: Advanced features (image transformations, cleanup)

---

**Last Updated**: 2025-11-15
