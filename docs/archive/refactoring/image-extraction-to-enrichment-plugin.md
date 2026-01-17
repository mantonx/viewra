# Image Extraction Migration to Enrichment Plugin

> Analysis date: 2025-12-14
> Status: **All Phases Complete** (Phases 0-6)
> Related ADRs: [006-image-handling-strategy](../decisions/006-image-handling-strategy.md), [027-plugin-system-architecture](../decisions/027-plugin-system-architecture.md)

This document details the refactoring plan to move all image extraction logic from the library scan package to the enrichment plugin system, while also addressing the growing complexity of the worker pool.

---

## Executive Summary

**Goal**: Move image extraction from synchronous scan-time processing to asynchronous enrichment pipeline processing.

**Current State**: Image extraction runs synchronously during library scans via 7 specialized use cases, blocking the scanner and tightly coupling image logic to scan logic.

**Target State**: A single `local_images` enricher handles all image extraction asynchronously, with the worker pool managing metadata extraction, preset generation, and database storage.

**Benefits**:
- Faster library scans (no blocking on image I/O)
- Unified enrichment pipeline for all metadata sources
- User-configurable via pipeline settings
- Consistent with plugin architecture (ADR 027)

---

## Current Architecture

### Components Involved

| Layer | Component | Location | Lines |
|-------|-----------|----------|-------|
| Application | 7 Image Use Cases | `internal/application/images/` | ~400 |
| Application | Shared Processing | `internal/application/images/extract_shared.go` | 162 |
| Application | Scan Integration | `internal/application/library/scan/media/images.go` | 171 |
| Application | Enricher (partial) | `internal/application/enrichment/builtin/local_images.go` | 155 |
| Infrastructure | File Discovery | `internal/infrastructure/images/extractor.go` | ~600 |
| Infrastructure | Embedded Extractor | `internal/infrastructure/images/embedded_extractor.go` | ~300 |
| Infrastructure | Metadata Extractor | `internal/infrastructure/images/metadata.go` | ~150 |
| Infrastructure | Transformer | `internal/infrastructure/images/transformer.go` | ~400 |
| Infrastructure | Cache Service | `internal/infrastructure/images/cache_service.go` | ~100 |
| Domain | Image Types | `internal/domain/images/` | ~200 |

### Current Data Flow

```
Library Scan
    │
    ├─► ProcessMovie()
    │       │
    │       └─► PostSave callback
    │               │
    │               ├─► ExtractImagesForMovie()
    │               │       │
    │               │       └─► MovieExtractor.Execute()
    │               │               │
    │               │               ├─► infrastructure/Extractor.ExtractMovieImages()
    │               │               ├─► ProcessAndSaveImages()
    │               │               │       ├─► MetadataExtractor.ExtractMetadata()
    │               │               │       ├─► Check deduplication (hash)
    │               │               │       ├─► Transformer.TransformAllPresets()
    │               │               │       └─► ImageRepo.Create()
    │               │               └─► Return
    │               │
    │               └─► enqueueForEnrichment()  ←── Enrichment starts AFTER images
    │
    └─► (next file)
```

### Key Issues

1. **Synchronous Blocking**: Image extraction blocks the scanner
2. **Dual Systems**: Scan extracts images AND enqueues for enrichment separately
3. **Incomplete Enricher**: `local_images.go` only discovers files, doesn't process them
4. **Coupling**: TV show NFO enrichment called from image extraction code
5. **7 Use Cases**: Separate implementations for each media type

---

## Target Architecture

### Simplified Data Flow

```
Library Scan
    │
    ├─► ProcessMovie()
    │       │
    │       └─► PostSave callback
    │               │
    │               └─► enqueueForEnrichment()  ←── Only this remains
    │
    └─► (next file, immediately)

                    ║
                    ║  (async)
                    ▼

Enrichment Pipeline
    │
    ├─► Worker Pool claims job
    │       │
    │       └─► LocalImagesEnricher.Enrich()
    │               │
    │               ├─► infrastructure/Extractor.Extract*Images()
    │               └─► Return discovered images in EnrichResponse
    │
    └─► Worker Pool processes response
            │
            └─► processImages()  (already exists in worker_pool.go)
                    │
                    ├─► MetadataExtractor.ExtractMetadata()
                    ├─► Check deduplication (hash)
                    ├─► Transformer.TransformAllPresets()
                    └─► ImageRepo.Create()
```

### Component Changes

| Component | Change | Effort |
|-----------|--------|--------|
| `LocalImagesEnricher` | Add infrastructure deps, use full extractor | Medium |
| `worker_pool.processImages()` | Add metadata extraction, presets, dedup | Medium |
| `pipeline/deps.go` | Add transformer, metadata extractor deps | Low |
| Scan `PostSave` callbacks | Remove image extraction calls | Low |
| 7 Image Use Cases | Delete (replaced by enricher) | Low |
| `extract_shared.go` | Move logic to worker pool, then delete | Medium |
| `media/images.go` | Simplify to only enqueue parent entities | Low |

---

## Worker Pool Decomposition

The current `worker_pool.go` is **726 lines** with multiple responsibilities that have grown organically. Before adding image processing logic, we must decompose it into focused, testable components.

### Current State Analysis

| Responsibility | Lines | Methods |
|----------------|-------|---------|
| Pool lifecycle | ~70 | `NewWorkerPool`, `Run`, `SetEnqueueNext` |
| Worker loop | ~60 | `worker` |
| Job processing | ~60 | `processJob` |
| Request building | ~100 | `buildEnrichRequest`, `buildMediaEntityRequest`, `buildTVShowRequest` |
| Response handling | ~30 | `applyEnrichResponse` |
| Metadata updates | ~200 | `applyMetadataUpdates`, `applyMovieMetadata`, `applyTVEpisodeMetadata`, `applyTVShowMetadata`, `applyMusicMetadata` |
| Image processing | ~65 | `processImages`, `mapToImageMediaType` |
| Success/failure | ~75 | `handleSuccess`, `handleFailure` |
| Error handling | ~40 | `categorizeError`, `containsAny` |
| Helpers | ~10 | `intPtr`, `stringPtr` |

### Proposed File Structure

```text
internal/application/enrichment/pipeline/
├── manager.go              # Pipeline orchestration (existing)
├── deps.go                 # Dependencies (existing)
├── config.go               # Configuration types (existing)
├── worker_pool.go          # Pool lifecycle + worker loop only (~150 lines)
├── job_processor.go        # NEW: Job processing orchestration (~100 lines)
├── request_builder.go      # NEW: EnrichRequest construction (~120 lines)
├── response_applier.go     # NEW: Response application orchestration (~50 lines)
├── metadata_applier.go     # NEW: Type-specific metadata updates (~220 lines)
├── image_processor.go      # NEW: Image processing pipeline (~150 lines)
├── errors.go               # NEW: Error categorization (~50 lines)
└── helpers.go              # NEW: Shared utilities (~20 lines)
```

### Decomposition Details

#### 1. `worker_pool.go` (Slim down to ~150 lines)

Keep only:

- `WorkerPool` struct definition
- `NewWorkerPool`
- `SetEnqueueNext`
- `Run`
- `worker` (main loop)

Move out: `processJob` and everything it calls.

#### 2. `job_processor.go` (NEW ~100 lines)

```go
// JobProcessor handles the execution of a single enrichment job.
type JobProcessor struct {
    deps           *Deps
    enricher       appenrich.Enricher
    requestBuilder *RequestBuilder
    responseApplier *ResponseApplier
    config         StageWorkerConfig
    logger         *slog.Logger
}

func (p *JobProcessor) Process(ctx context.Context, job *enrichment.QueueJob) error
func (p *JobProcessor) handleSuccess(ctx context.Context, job *enrichment.QueueJob, ...) error
func (p *JobProcessor) handleFailure(ctx context.Context, job *enrichment.QueueJob, ...) error
```

#### 3. `request_builder.go` (NEW ~120 lines)

```go
// RequestBuilder constructs EnrichRequest from queue jobs.
type RequestBuilder struct {
    deps       *Deps
    typedRepos *TypedMediaRepos
    logger     *slog.Logger
}

func (b *RequestBuilder) Build(ctx context.Context, job *enrichment.QueueJob) (*pluginv1.EnrichRequest, enrichment.MediaType, error)
func (b *RequestBuilder) buildMediaEntityRequest(ctx context.Context, job *enrichment.QueueJob, existingIDs map[string]string) (...)
func (b *RequestBuilder) buildTVShowRequest(ctx context.Context, job *enrichment.QueueJob, existingIDs map[string]string) (...)
```

#### 4. `response_applier.go` (NEW ~50 lines)

```go
// ResponseApplier coordinates applying enrichment results to the database.
type ResponseApplier struct {
    deps            *Deps
    metadataApplier *MetadataApplier
    imageProcessor  *ImageProcessor
    logger          *slog.Logger
}

func (a *ResponseApplier) Apply(ctx context.Context, job *enrichment.QueueJob, mediaType enrichment.MediaType, resp *pluginv1.EnrichResponse) error
```

#### 5. `metadata_applier.go` (NEW ~220 lines)

```go
// MetadataApplier handles type-specific metadata updates.
type MetadataApplier struct {
    typedRepos *TypedMediaRepos
    logger     *slog.Logger
}

func (a *MetadataApplier) Apply(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, metadata *pluginv1.EnrichedMetadata) error
func (a *MetadataApplier) applyMovieMetadata(ctx context.Context, mediaID int64, metadata *pluginv1.EnrichedMetadata) error
func (a *MetadataApplier) applyTVEpisodeMetadata(...)
func (a *MetadataApplier) applyTVShowMetadata(...)
func (a *MetadataApplier) applyMusicMetadata(...)
```

#### 6. `image_processor.go` (NEW ~150 lines)

This is where the enhanced image processing lives:

```go
// ImageProcessor handles image discovery, metadata extraction, and storage.
type ImageProcessor struct {
    imageRepo         ImageRepository
    metadataExtractor MetadataExtractor  // NEW dependency
    transformer       ImageTransformer   // NEW dependency
    cacheService      CacheService       // NEW dependency
    logger            *slog.Logger
}

func (p *ImageProcessor) Process(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, images []*pluginv1.EnrichedImage) error
func (p *ImageProcessor) processLocalImage(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, img *pluginv1.EnrichedImage) error
func (p *ImageProcessor) processRemoteImage(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, img *pluginv1.EnrichedImage) error
func (p *ImageProcessor) extractMetadata(path string) (*ImageMetadata, error)
func (p *ImageProcessor) generatePresets(path string, hash string, imgType images.ImageType) (map[string]string, error)
func (p *ImageProcessor) checkDuplicate(ctx context.Context, hash string, mediaID int64, imgType images.ImageType) (bool, error)
```

#### 7. `errors.go` (NEW ~50 lines)

```go
// Error categorization for retry logic
func CategorizeError(err error) enrichment.ErrorCategory
func containsAny(s string, substrings ...string) bool
```

### Benefits of Decomposition

| Benefit | Description |
|---------|-------------|
| **Testability** | Each component can be unit tested in isolation |
| **Single Responsibility** | Each file has one clear purpose |
| **Extensibility** | New media types only need changes in `metadata_applier.go` |
| **Readability** | Smaller files are easier to understand and review |
| **Image Processing** | Dedicated `ImageProcessor` can grow without bloating worker pool |

### Migration Strategy

1. **Phase 0** (prerequisite): Extract files WITHOUT changing behavior
   - Move code to new files
   - Update imports
   - Run tests to verify no regressions

2. **Phase 1+**: Add new functionality to the appropriate component
   - Image processing enhancements go in `image_processor.go`
   - New media types go in `metadata_applier.go`
   - Request building changes go in `request_builder.go`

---

## Implementation Phases

### Phase 0: Worker Pool Decomposition (Prerequisite)

**Goal**: Split `worker_pool.go` into focused components before adding new functionality.

**Tasks**:

1. Create `errors.go` with `categorizeError`, `containsAny`
2. Create `helpers.go` with `intPtr`, `stringPtr`, `mapToImageMediaType`
3. Create `metadata_applier.go` with all `apply*Metadata` methods
4. Create `image_processor.go` with `processImages` (current implementation)
5. Create `request_builder.go` with all `build*Request` methods
6. Create `response_applier.go` coordinating metadata and image application
7. Create `job_processor.go` with `processJob`, `handleSuccess`, `handleFailure`
8. Slim `worker_pool.go` to lifecycle + worker loop only
9. Update tests, verify no regressions

**Estimated effort**: 1 day

---

### Phase 1: Enhance Image Processor

**Goal**: Make `ImageProcessor` capable of full image processing (not just storage).

After Phase 0, `image_processor.go` will contain the basic `processImages` logic. This phase adds:

- Metadata extraction (dimensions, hash, MIME type)
- Deduplication via file hash
- WebP preset generation (thumb, medium, large, xlarge)

**Current** (after Phase 0, in `image_processor.go`):

```go
func (p *ImageProcessor) Process(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, images []*pluginv1.EnrichedImage) error {
    for _, img := range images {
        // Basic storage only - no metadata, no presets, no dedup
        if err := p.imageRepo.Create(ctx, &images.Image{...}); err != nil {
            return err
        }
    }
    return nil
}
```

**Target** (full pipeline):

```go
func (p *ImageProcessor) Process(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, protoImages []*pluginv1.EnrichedImage) error {
    for _, protoImg := range protoImages {
        if protoImg.IsRemote {
            if err := p.processRemoteImage(ctx, mediaID, mediaType, protoImg); err != nil {
                p.logger.Warn("failed to process remote image", ...)
            }
            continue
        }
        if err := p.processLocalImage(ctx, mediaID, mediaType, protoImg); err != nil {
            p.logger.Warn("failed to process local image", ...)
        }
    }
    return nil
}

func (p *ImageProcessor) processLocalImage(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, img *pluginv1.EnrichedImage) error {
    // 1. Extract metadata (dimensions, hash, MIME type)
    metadata, err := p.metadataExtractor.ExtractMetadata(img.Path)
    if err != nil {
        return fmt.Errorf("extract metadata: %w", err)
    }

    // 2. Check for deduplication via hash
    if metadata.FileHash != nil {
        isDuplicate, err := p.checkDuplicate(ctx, *metadata.FileHash, mediaID, images.ImageType(img.Type))
        if err == nil && isDuplicate {
            return nil // Already have this exact image
        }
    }

    // 3. Generate WebP presets (thumb, medium, large, xlarge)
    var localCachePath *string
    if p.transformer != nil && metadata.FileHash != nil {
        presetPaths, err := p.generatePresets(img.Path, *metadata.FileHash, images.ImageType(img.Type))
        if err == nil {
            if medium, ok := presetPaths["medium"]; ok {
                localCachePath = &medium
            }
        }
    }

    // 4. Create image record with full metadata
    return p.imageRepo.Create(ctx, &images.Image{
        MediaID:        intPtr(int(mediaID)),
        MediaType:      mapToImageMediaType(mediaType),
        ImageType:      images.ImageType(img.Type),
        SourceType:     images.SourceTypeLocal,
        FilePath:       img.Path,
        LocalCachePath: localCachePath,
        Width:          metadata.Width,
        Height:         metadata.Height,
        FileSizeBytes:  metadata.FileSizeBytes,
        MimeType:       metadata.MimeType,
        FileHash:       metadata.FileHash,
    })
}
```

**New dependencies for `ImageProcessor`**:

```go
// In deps.go or image_processor.go
type MetadataExtractor interface {
    ExtractMetadata(path string) (*ImageMetadata, error)
}

type ImageTransformer interface {
    TransformAllPresets(srcPath, hash string, imgType images.ImageType) (map[string]string, error)
}
```

**Files to modify**:

- `internal/application/enrichment/pipeline/image_processor.go` - Add full pipeline
- `internal/application/enrichment/pipeline/deps.go` - Add `MetadataExtractor`, `Transformer` interfaces
- `internal/app/services/services.go` - Wire infrastructure implementations

**Tests**:

- Unit tests for `ImageProcessor` with mock extractor/transformer
- Integration test verifying presets are generated

---

## Remote Image Handling (TVDB, TMDB, Fanart.tv)

External enrichment plugins will return remote image URLs rather than local file paths. This requires different handling than local images.

### Remote vs Local Images

| Aspect | Local Images | Remote Images |
|--------|--------------|---------------|
| Source | Filesystem path | HTTP/HTTPS URL |
| Availability | Immediate | Requires download |
| Storage | Reference original file | Must cache locally |
| Presets | Generate from local file | Generate after download |
| Failure mode | File not found | Network errors, rate limits |
| Deduplication | File hash | URL + entity match |

### Design Decisions

#### 1. When to Download Remote Images

**Option A: Eager download in worker pool** (recommended)
- Download during `processRemoteImage()` in the enrichment pipeline
- Generate presets immediately after download
- Single pass through the pipeline

**Option B: Lazy download on first request**
- Store URL in database, download when UI requests image
- Faster enrichment, slower first image load
- More complex serving logic

**Recommendation**: Option A - eager download keeps the serving path simple and ensures images are available offline.

#### 2. Remote Image Storage Structure

```text
data/cache/images/
├── remote/                      # Downloaded remote images
│   ├── {first2}/{next2}/       # Sharded by URL hash
│   │   └── {url_hash}.{ext}    # Original downloaded file
│   └── ...
└── presets/                     # Generated WebP presets (existing)
    └── {first2}/{next2}/
        └── {file_hash}_{type}_{preset}.webp
```

#### 3. Database Schema for Remote Images

The existing `media_images` table already supports this:

```sql
-- Existing columns that support remote images:
file_path       -- NULL for remote-only images
remote_url      -- Original URL from enricher
local_cache_path -- Path to downloaded file (after download)
source_type     -- 'tmdb', 'tvdb', 'fanart_tv', etc.
```

#### 4. ImageProcessor.processRemoteImage() Implementation

```go
func (p *ImageProcessor) processRemoteImage(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, img *pluginv1.EnrichedImage) error {
    // 1. Check if we already have this URL for this entity
    existing, err := p.imageRepo.GetByRemoteURL(ctx, img.Path, mediaID)
    if err == nil && existing != nil {
        return nil // Already downloaded
    }

    // 2. Download the image
    localPath, err := p.downloader.Download(ctx, img.Path)
    if err != nil {
        return fmt.Errorf("download image: %w", err)
    }

    // 3. Extract metadata from downloaded file
    metadata, err := p.metadataExtractor.ExtractMetadata(localPath)
    if err != nil {
        // Cleanup failed download
        os.Remove(localPath)
        return fmt.Errorf("extract metadata: %w", err)
    }

    // 4. Generate presets
    var presetPath *string
    if p.transformer != nil && metadata.FileHash != nil {
        presetPaths, err := p.generatePresets(localPath, *metadata.FileHash, images.ImageType(img.Type))
        if err == nil {
            if medium, ok := presetPaths["medium"]; ok {
                presetPath = &medium
            }
        }
    }

    // 5. Store image record with both remote URL and local cache path
    return p.imageRepo.Create(ctx, &images.Image{
        MediaID:        intPtr(int(mediaID)),
        MediaType:      mapToImageMediaType(mediaType),
        ImageType:      images.ImageType(img.Type),
        SourceType:     mapSourceType(img.Source), // tmdb, tvdb, fanart_tv
        RemoteURL:      &img.Path,
        LocalCachePath: presetPath,
        FilePath:       &localPath, // Downloaded original
        Width:          metadata.Width,
        Height:         metadata.Height,
        FileSizeBytes:  metadata.FileSizeBytes,
        MimeType:       metadata.MimeType,
        FileHash:       metadata.FileHash,
        Language:       stringPtrIfNotEmpty(img.Language),
        Priority:       int(img.Priority), // Remote images typically lower priority than local
    })
}
```

#### 5. New Dependencies for Remote Images

```go
// ImageDownloader handles downloading remote images
type ImageDownloader interface {
    // Download fetches a remote image and stores it locally
    // Returns the local file path
    Download(ctx context.Context, url string) (string, error)
}

// In infrastructure/images/downloader.go
type Downloader struct {
    cacheDir   string
    httpClient *http.Client
    logger     *slog.Logger
}

func (d *Downloader) Download(ctx context.Context, url string) (string, error) {
    // 1. Hash URL for cache path
    urlHash := hashURL(url)
    ext := filepath.Ext(url)
    if ext == "" {
        ext = ".jpg" // Default
    }

    // 2. Create sharded path
    localPath := filepath.Join(d.cacheDir, "remote", urlHash[:2], urlHash[2:4], urlHash+ext)

    // 3. Check if already downloaded
    if _, err := os.Stat(localPath); err == nil {
        return localPath, nil
    }

    // 4. Download with timeout
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return "", err
    }

    resp, err := d.httpClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
    }

    // 5. Write to disk
    os.MkdirAll(filepath.Dir(localPath), 0755)
    f, err := os.Create(localPath)
    if err != nil {
        return "", err
    }
    defer f.Close()

    _, err = io.Copy(f, resp.Body)
    return localPath, err
}
```

#### 6. EnrichResponse Image Format from Remote Plugins

External plugins should return images with full metadata:

```go
// From TVDB enricher:
resp.Images = append(resp.Images, &pluginv1.EnrichedImage{
    Type:     "poster",
    Path:     "https://thetvdb.com/banners/posters/12345.jpg",
    IsRemote: true,
    Source:   "tvdb",
    Width:    680,
    Height:   1000,
    Language: "en",
    Priority: 10, // Lower priority than local images
})
```

#### 7. Rate Limiting and Retry for Downloads

Remote image downloads should respect rate limits:

```go
type Downloader struct {
    // ... existing fields
    rateLimiter *rate.Limiter // Per-host rate limiting
}

func (d *Downloader) Download(ctx context.Context, url string) (string, error) {
    // Wait for rate limiter
    if err := d.rateLimiter.Wait(ctx); err != nil {
        return "", err
    }
    // ... download logic
}
```

### Impact on Implementation Phases

| Phase | Additional Work for Remote Images |
|-------|-----------------------------------|
| 0 | None (decomposition only) |
| 1 | Add `ImageDownloader` interface, implement `processRemoteImage()` |
| 2 | Ensure enrichers set `IsRemote: true` and `Source` field |
| 3 | None |
| 4 | None |
| 5 | None |
| 6 | None |

### Updated Phase 1 Tasks

Add to Phase 1 checklist:

- [ ] Create `ImageDownloader` interface
- [ ] Implement `infrastructure/images/downloader.go`
- [ ] Implement `processRemoteImage()` in `ImageProcessor`
- [ ] Add `GetByRemoteURL()` to image repository
- [ ] Wire downloader in `services.go`
- [ ] Add unit tests for download + processing flow

---

### Phase 2: Enhance LocalImagesEnricher

**Goal**: Make the enricher use full infrastructure extractor logic.

**Current** (`builtin/local_images.go`):
```go
type LocalImagesEnricher struct{}  // No dependencies!

func (e *LocalImagesEnricher) Enrich(ctx context.Context, req *pluginv1.EnrichRequest) (*pluginv1.EnrichResponse, error) {
    dir := filepath.Dir(req.FilePath)
    // Manual pattern matching for common filenames
    // Missing: TV show/season patterns, embedded images, comprehensive discovery
}
```

**Target**:
```go
type LocalImagesEnricher struct {
    extractor         *infraImages.Extractor
    embeddedExtractor *infraImages.EmbeddedExtractor
    logger            *slog.Logger
}

func NewLocalImagesEnricher(
    extractor *infraImages.Extractor,
    embeddedExtractor *infraImages.EmbeddedExtractor,
    logger *slog.Logger,
) *LocalImagesEnricher {
    return &LocalImagesEnricher{
        extractor:         extractor,
        embeddedExtractor: embeddedExtractor,
        logger:            logger,
    }
}

func (e *LocalImagesEnricher) Capabilities() appenrich.EnricherCapabilities {
    return appenrich.NewCapabilitiesBuilder().
        WithMediaTypes(
            enrichment.MediaTypeMovie,
            enrichment.MediaTypeTV,
            enrichment.MediaTypeTVShow,
            enrichment.MediaTypeMusic,
        ).
        WithProvides("artwork").
        AsLocal().
        Build()
}

func (e *LocalImagesEnricher) Enrich(ctx context.Context, req *pluginv1.EnrichRequest) (*pluginv1.EnrichResponse, error) {
    resp := appenrich.Match()
    resp.Confidence = 1.0

    switch req.MediaType {
    case string(enrichment.MediaTypeMovie):
        e.extractMovieImages(req.FilePath, resp)
    case string(enrichment.MediaTypeTV):
        e.extractTVEpisodeImages(req.FilePath, resp)
    case string(enrichment.MediaTypeTVShow):
        e.extractTVShowImages(req.FilePath, resp)
    case string(enrichment.MediaTypeMusic):
        e.extractMusicImages(req.FilePath, resp)
    }

    if len(resp.Images) == 0 {
        return appenrich.Skip("no local images found"), nil
    }
    return resp, nil
}

func (e *LocalImagesEnricher) extractMovieImages(filePath string, resp *pluginv1.EnrichResponse) {
    extracted := e.extractor.ExtractMovieImages(filePath)
    for _, img := range extracted.Images {
        appenrich.AddImage(resp, &pluginv1.EnrichedImage{
            Type:     string(img.Type),
            Path:     img.Path,
            IsRemote: false,
        })
    }
}

// Similar methods for TV, Music...
```

**Files to modify**:
- `internal/application/enrichment/builtin/local_images.go` - Add dependencies, use infrastructure
- `internal/app/services/services.go` - Wire extractor dependencies to enricher

**Tests**:
- Update `local_images_test.go` with mocked infrastructure

---

### Phase 3: Add Missing Media Types

**Goal**: Support TV shows, seasons, albums, artists as enrichment targets.

**Current MediaTypes** (`domain/enrichment/types.go`):
```go
const (
    MediaTypeMovie  MediaType = "movie"
    MediaTypeTV     MediaType = "tv"      // Episodes
    MediaTypeTVShow MediaType = "tv_show" // Shows
    MediaTypeMusic  MediaType = "music"   // Tracks
)
```

**New MediaTypes to add**:
```go
const (
    // ... existing ...
    MediaTypeTVSeason    MediaType = "tv_season"
    MediaTypeMusicAlbum  MediaType = "music_album"
    MediaTypeMusicArtist MediaType = "music_artist"
)
```

**Implications**:
- Pipeline configuration table needs entries for new types
- Worker pool routing needs to handle new types
- Scanner needs to enqueue parent entities

**Files to modify**:
- `internal/domain/enrichment/types.go` - Add new MediaType constants
- `internal/application/enrichment/pipeline/worker_pool.go` - Handle new types in routing
- Migration - Seed default pipeline stages for new types

---

### Phase 4: Update Scanner to Remove Image Extraction

**Goal**: Scanner only enqueues for enrichment, doesn't extract images.

**Current** (`media/movie.go:98-103`):
```go
PostSave: func(ctx context.Context) {
    ExtractImagesForMovie(ctx, deps, movie, result.FilePath)  // REMOVE
    PersistMediaTracks(ctx, deps, movie.Media.ID, result)
    enqueueForEnrichment(ctx, deps, movie.Media.ID, enrichment.MediaTypeMovie)
},
```

**Target**:
```go
PostSave: func(ctx context.Context) {
    PersistMediaTracks(ctx, deps, movie.Media.ID, result)
    enqueueForEnrichment(ctx, deps, movie.Media.ID, enrichment.MediaTypeMovie)
},
```

**Files to modify**:
- `internal/application/library/scan/media/movie.go` - Remove `ExtractImagesForMovie`
- `internal/application/library/scan/media/tv.go` - Remove `ExtractImagesForEpisode`
- `internal/application/library/scan/media/music.go` - Remove `ExtractImagesForTrack`
- `internal/application/library/scan/media/images.go` - Simplify or delete
- `internal/application/library/scan/media/deps.go` - Remove 7 extractor interfaces

**Handle Parent Entities**:

TV shows and albums need special handling. Currently `ExtractImagesForEpisode` also triggers show/season extraction. After refactor:

Option A: **Separate queue entries** (recommended)
- Episode scan enqueues episode AND show (once per show per scan)
- Worker pool processes each independently

Option B: **Enricher discovers parents**
- Episode enricher also extracts show/season images
- More complex enricher, single queue entry

**Recommended approach (Option A)**:
```go
// In tv.go PostSave:
enqueueForEnrichment(ctx, deps, episode.Media.ID, enrichment.MediaTypeTV)

// Show enqueuing happens via existing pattern in images.go:
if deps.ProcessedShows.TryMark(showTitle) {
    enqueueForEnrichment(ctx, deps, show.ID, enrichment.MediaTypeTVShow)
}
```

---

### Phase 5: Delete Obsolete Code

**Goal**: Remove code that's now handled by the enrichment pipeline.

**Files to delete**:
```
internal/application/images/
├── extract_images.go           # 7 use case implementations
├── extract_shared.go           # ProcessAndSaveImages (moved to worker pool)
└── (keep any remaining shared utilities)
```

**Code to remove from deps**:
```go
// internal/application/library/scan/media/deps.go
// Remove these interfaces:
type MovieImageExtractor interface { ... }
type TVEpisodeImageExtractor interface { ... }
type TVShowImageExtractor interface { ... }
type TVSeasonImageExtractor interface { ... }
type MusicAlbumImageExtractor interface { ... }
type MusicArtistImageExtractor interface { ... }
type MusicTrackImageExtractor interface { ... }

// Remove from Deps struct:
MovieExtractor   MovieImageExtractor
EpisodeExtractor TVEpisodeImageExtractor
// ... etc
```

**Wiring to remove**:
- `internal/app/usecases/usecases.go` - Remove 7 use case instantiations
- `internal/application/library/scan_orchestrator.go` - Remove extractor fields

---

### Phase 6: Handle Embedded Audio Images ✅ COMPLETE

**Goal**: Extract embedded album art from audio files via enrichment.

**Implementation**: Integrated into `LocalImagesEnricher` as part of Phase 2. The `extractMusicImages()` method:

1. Tries album directory images first via `ExtractMusicAlbumImages()`
2. Falls back to embedded artwork via `ExtractMusicTrackImages()` if no cover is found

See [local_images.go](../../internal/application/enrichment/builtin/local_images.go) lines 172-216 for implementation.

---

## Migration Checklist

### Pre-Migration

- [ ] All enrichment tests pass
- [ ] All library scan tests pass
- [ ] Document current image extraction behavior

### Phase 0: Worker Pool Decomposition ✅ COMPLETE

- [x] Create `errors.go` with `CategorizeError`, `containsAny`
- [x] Create `helpers.go` with `intPtr`, `stringPtr`, `mapToImageMediaType`
- [x] Create `metadata_applier.go` with `MetadataApplier` struct and all `apply*Metadata` methods
- [x] Create `image_processor.go` with `ImageProcessor` struct and `Process` method
- [x] Create `request_builder.go` with `RequestBuilder` struct and all `build*Request` methods
- [x] Create `response_applier.go` with `ResponseApplier` struct coordinating metadata and images
- [x] Create `job_processor.go` with `JobProcessor` struct, `Process`, `handleSuccess`, `handleFailure`
- [x] Refactor `worker_pool.go` to use `JobProcessor` (lifecycle + worker loop only)
- [x] Update all imports across the pipeline package
- [x] Run existing tests - all must pass with no behavior changes
- [x] Verify test coverage maintained

### Phase 1: Image Processor Enhancement ✅ COMPLETE

**Local Images:**

- [x] Add `MetadataExtractor` interface to `deps.go`
- [x] Add `ImageTransformer` interface to `deps.go`
- [x] Implement `processLocalImage()` with metadata extraction
- [x] Implement `checkDuplicate()` for hash-based deduplication
- [x] Implement `generatePresets()` for WebP conversion
- [x] Wire infrastructure implementations in `services.go`
- [x] Created `metadataExtractorAdapter` to bridge infrastructure types

**Remote Images:**

- [x] Create `ImageDownloader` interface in `deps.go`
- [x] Implement `infrastructure/images/downloader.go` with rate limiting
- [x] Implement `processRemoteImage()` in `ImageProcessor`
- [x] Add `GetByExternalURL()` method to image repository interface
- [x] Add SQL query for `GetImageByExternalURL` (SQLite + PostgreSQL)
- [x] Wire downloader in `services.go`
- [x] Implemented source detection from URL patterns (TMDb, TVDb, Fanart.tv)

### Phase 2: Enricher Enhancement ✅ COMPLETE

- [x] Add infrastructure dependencies to `LocalImagesEnricher`
- [x] Implement `extractMovieImages()` using infrastructure `Extractor`
- [x] Implement `extractTVEpisodeImages()` and `extractTVShowImages()`
- [x] Implement `extractMusicImages()` with embedded fallback via `ExtractMusicTrackImages`
- [x] Update enricher constructor in `services.go` to wire `infraimages.NewExtractor()`
- [x] Direct infrastructure usage (no adapter needed - simpler design)

### Phase 3: Media Types ✅ COMPLETE

- [x] Add `MediaTypeTVSeason` constant
- [x] Add `MediaTypeMusicAlbum` constant
- [x] Add `MediaTypeMusicArtist` constant
- [x] Update metadata_applier for new types (with TODOs for future album/artist metadata)
- [x] Update `mapToImageMediaType` helper for new types
- [x] Update `LocalImagesEnricher.Capabilities()` for all 7 media types
- [x] Add `extractTVSeasonImages()`, `extractMusicAlbumImages()`, `extractMusicArtistImages()` methods
- [x] Create migration 000032 for pipeline stages (tv_season, music_album, music_artist)
- [x] Create migration 000033 for queue media_type CHECK constraint updates

### Phase 4: Scanner Simplification ✅ COMPLETE

- [x] Remove `ExtractImagesForMovie` call from movie.go
- [x] Remove `ExtractImagesForEpisode` call from tv.go
- [x] Remove `ExtractImagesForTrack` call from music.go
- [x] Simplify `media/images.go` to only handle parent entity enqueueing
- [x] Remove 7 extractor interfaces from deps.go and interfaces.go
- [x] Remove extractor fields from Deps struct
- [x] Update scan orchestrator to not wire extractors
- [x] Update usecases.go to not instantiate extractors
- [x] Update all test files (test_helpers_test.go, scan_orchestrator_test.go, scan_orchestrator_coverage_test.go)
- [x] Add `EnqueueTVParentEntities` and `EnqueueMusicParentEntities` functions

### Phase 5: Cleanup ✅ COMPLETE

- [x] Delete `internal/application/images/extract_images.go`
- [x] Delete `internal/application/images/extract_shared.go`
- [x] Remove 6 extraction use case fields from `ImageUseCases` struct
- [x] Remove extraction use case instantiation from `buildImageUseCases()`
- [x] Verified build passes
- [x] Verified tests pass

### Post-Migration

- [x] All enrichment tests pass
- [x] All library scan tests pass
- [ ] Integration test: scan library, verify images extracted via enrichment
- [ ] Performance test: scan should be faster
- [ ] Manual QA: images display correctly in UI

---

## Risk Mitigation

### Risk: Scan performance regression
**Mitigation**:
- Enrichment is async, should improve scan speed
- Monitor scan times before/after
- Worker pool concurrency is tunable

### Risk: Missing images after migration
**Mitigation**:
- Keep infrastructure extractors unchanged
- Comprehensive test coverage
- Parallel run period (both old and new) if needed

### Risk: Parent entity images not extracted
**Mitigation**:
- Ensure show/album enqueueing preserved
- Test multi-episode and multi-track scenarios
- Verify ProcessedShows/ProcessedAlbums deduplication works

### Risk: Embedded image extraction fails
**Mitigation**:
- Keep EmbeddedExtractor infrastructure unchanged
- Test with various audio formats (MP3, FLAC, etc.)
- Fallback gracefully if extraction fails

---

## Effort Estimate

| Phase | Description | Effort |
|-------|-------------|--------|
| 0 | Worker Pool Decomposition | 1 day |
| 1 | Image Processor (local + remote) | 2-3 days |
| 2 | Enricher Enhancement | 1-2 days |
| 3 | Media Types | 0.5 day |
| 4 | Scanner Simplification | 1 day |
| 5 | Cleanup | 0.5 day |
| 6 | Embedded Images | 0.5 day |
| Testing | Unit + Integration | 1-2 days |
| **Total** | | **7-11 days** |

---

## Success Criteria

1. **Functional**: All image types extracted correctly for all media types
2. **Performance**: Library scans complete faster (no sync image I/O)
3. **Observability**: Image extraction visible in enrichment pipeline progress
4. **Configurability**: Users can disable/reorder local_images stage
5. **Simplicity**: Scanner code reduced by ~400 lines
6. **Consistency**: Image extraction follows same pattern as NFO, TMDB enrichers

---

## References

- [ADR 006: Image Handling Strategy](../decisions/006-image-handling-strategy.md)
- [ADR 027: Plugin System Architecture](../decisions/027-plugin-system-architecture.md)
- [Current LocalImagesEnricher](../../internal/application/enrichment/builtin/local_images.go)
- [Current ProcessAndSaveImages](../../internal/application/images/extract_shared.go)
- [Worker Pool processImages](../../internal/application/enrichment/pipeline/worker_pool.go)
