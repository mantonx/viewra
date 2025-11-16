# Phase 4.1: Image Handling - Gap Analysis & Missing Implementation

**Created**: 2025-11-16
**Status**: Critical Review
**Author**: Reality Check

## Executive Summary

Phase 4.1 is marked as "100% Complete" in PROJECT_PLAN.md, but **critical gaps exist between what ADR 006 specifies and what we've actually implemented**. This document identifies what's missing and what needs urgent attention.

---

## Critical Gap: Image Serving Strategy

### What ADR 006 Specifies (Lines 754-778)

**Hash-Based Cache Storage** for ALL images (local and external):

```
data/cache/images/
├── {hash}_original.jpg       # Original (local or downloaded)
├── {hash}_300x450.jpg        # Resized thumbnail
├── {hash}_1280x720.jpg       # Resized fanart
└── {hash}_1920x1080.webp     # WebP conversion
```

**Key Requirements**:
1. All images stored in cache with hash-based filenames
2. On-demand resizing via query params (`?width=300&format=webp`)
3. Cache transformed images to disk
4. Source-agnostic cache (database tracks origin via `source_type`)

### What We Actually Implemented

**Current Implementation** (extract_shared.go:38-49):
```go
img := &images.Image{
    MediaID:       mediaID,
    MediaType:     mediaType,
    EntityID:      entityID,
    ImageType:     imgInfo.Type,
    SourceType:    images.SourceTypeLocal,
    FilePath:      imgInfo.Path,        // Points to ORIGINAL file
    Width:         metadata.Width,
    Height:        metadata.Height,
    FileSizeBytes: metadata.FileSizeBytes,
    MimeType:      metadata.MimeType,
    FileHash:      metadata.FileHash,
    // LocalCachePath: NOT SET - This is the problem!
}
```

**What's Wrong**:
- We catalog images by reference only (just store `FilePath`)
- We do NOT copy/cache images to `data/cache/images/`
- We do NOT populate `LocalCachePath` field
- We do NOT support resizing or format conversion
- We serve original files directly from user's media directories

### Impact Assessment

**Severity**: HIGH - Major architectural deviation from ADR

**Consequences**:
1. **No deduplication**: Multiple identical images not deduplicated by hash
2. **No transformations**: Cannot resize or convert images on-demand
3. **No WebP support**: Missing major optimization opportunity
4. **Cache cleanup broken**: Cleanup logic expects hash-based filenames that don't exist
5. **Future features blocked**: External API downloads can't integrate properly

---

## What's Actually Implemented vs. ADR 006

### ✅ Implemented Correctly

1. **Database Schema** (Migration 000007)
   - ✅ Polymorphic `media_images` table
   - ✅ All required columns including `local_cache_path`
   - ✅ CASCADE deletion
   - ✅ Indexes for performance

2. **Image Discovery** (Infrastructure)
   - ✅ Kodi/Plex naming convention detection
   - ✅ Metadata extraction (dimensions, hash, MIME)
   - ✅ Support for all media types

3. **Domain Layer**
   - ✅ Complete entity with validation
   - ✅ Repository interface with all methods
   - ✅ Type-safe enums

4. **Application Layer - Cataloging**
   - ✅ Extract and catalog images
   - ✅ Store metadata in database
   - ✅ Scanner integration

5. **Cleanup System - Database**
   - ✅ CASCADE deletion for database records
   - ✅ Orphan detection queries
   - ✅ Hash-based cleanup use cases

### ❌ NOT Implemented (Critical Gaps)

1. **Image Caching to Disk** ⚠️ CRITICAL
   - ❌ No copying of local images to `data/cache/images/`
   - ❌ No hash-based filename generation
   - ❌ `LocalCachePath` field never populated
   - ❌ Cache directory structure not created

2. **Image Serving with Transformations** ⚠️ CRITICAL
   - ❌ No on-demand resizing (`?width=300`)
   - ❌ No format conversion (`?format=webp`)
   - ❌ No quality control (`?quality=85`)
   - ❌ Serving endpoint exists but serves original files only

3. **Cache Management** ⚠️ HIGH
   - ❌ No LRU eviction for cached images
   - ❌ Cleanup logic expects files that don't exist
   - ❌ No disk space monitoring
   - ❌ No cache statistics

4. **Deduplication** ⚠️ MEDIUM
   - ❌ Multiple images with same hash not deduplicated
   - ❌ Storage waste (same poster across movies not shared)

### 📋 Deferred to Phase 4.3 (As Planned)

These are correctly deferred per ADR 006:

- 📋 Advanced resizing algorithms
- 📋 Progressive image loading (blur-up)
- 📋 CDN integration
- 📋 Image optimization pipeline

---

## Specific Missing Components

### 1. Image Cache Service (HIGH PRIORITY)

**Location**: `internal/infrastructure/images/cache.go` (DOES NOT EXIST)

**Required Functionality**:
```go
type CacheService struct {
    cacheDir string
}

// CopyToCache copies an image to cache with hash-based filename
func (s *CacheService) CopyToCache(sourcePath, hash string) (cachePath string, err error)

// GetCachedPath returns the cache path for a hash
func (s *CacheService) GetCachedPath(hash string, size string) string

// ResizeAndCache resizes an image and caches it
func (s *CacheService) ResizeAndCache(sourcePath string, width, height int, format string) (string, error)

// ConvertToWebP converts and caches as WebP
func (s *CacheService) ConvertToWebP(sourcePath string, quality int) (string, error)
```

### 2. Image Transformation Handler (HIGH PRIORITY)

**Location**: `internal/api/handlers/images.go` (EXISTS BUT INCOMPLETE)

**Current State**: Serves original files directly
**Required**: Support query parameters:
- `?width=300&height=450` - Resize to dimensions
- `?format=webp` - Convert to WebP
- `?quality=85` - Quality control
- `?fit=cover` - Fit mode (cover, contain, fill)

**Missing Implementation**:
```go
func (h *ImageHandler) ServeImage(c *gin.Context) {
    // Parse query params
    width := c.Query("width")
    height := c.Query("height")
    format := c.Query("format")

    // Get original image
    image := h.repo.GetByID(imageID)

    // Check if transformed version exists in cache
    cacheKey := fmt.Sprintf("%s_%sx%s.%s", image.FileHash, width, height, format)
    cachedPath := h.cache.GetCachedPath(image.FileHash, cacheKey)

    if !fileExists(cachedPath) {
        // Transform and cache
        cachedPath = h.transformer.TransformAndCache(image, width, height, format)
    }

    // Serve cached file
    c.File(cachedPath)
}
```

### 3. Initial Cache Population (MEDIUM PRIORITY)

**Location**: `cmd/cache-images/main.go` (DOES NOT EXIST)

**Purpose**: One-time copy of existing images to cache

**Required**:
- Read all images from database
- Copy to `data/cache/images/{hash}_original.{ext}`
- Update `local_cache_path` in database
- Generate common sizes (300x450, 1280x720)

### 4. Cleanup Service Enhancement (MEDIUM PRIORITY)

**Location**: `internal/application/images/cleanup.go` (EXISTS BUT INCOMPLETE)

**Current State**: Assumes hash-based cache files exist
**Reality**: No cache files exist yet

**Required Fix**:
```go
func (uc *CleanupUseCase) CleanCacheForHashes(ctx context.Context, hashes []string) error {
    for _, hash := range hashes {
        // Check if hash still referenced
        images, err := uc.repo.GetImagesByHash(ctx, hash)
        if err != nil || len(images) > 0 {
            continue // Still in use or error
        }

        // Delete all cached variants: {hash}_*
        pattern := filepath.Join(uc.cacheDir, hash+"_*")
        files, _ := filepath.Glob(pattern)
        for _, file := range files {
            os.Remove(file) // Delete cache file
        }
    }
    return nil
}
```

**Current Issue**: This logic exists but operates on files that don't exist!

---

## Architecture Decision Conflict

### ADR 006 Says (Lines 152-179):

> **Image Serving Strategy: Direct File Serving** (for local images)
>
> Serve local file with caching headers
>
> **Optional: Resize on-the-fly using imaging library**
> - On-demand resizing using `github.com/disintegration/imaging`
> - Format conversion (JPEG → WebP for smaller sizes)
> - **Cache transformed images in `data/cache/images/`**
> - LRU cleanup similar to transcode cleanup

**Key Point**: "Optional" for Phase 4.1, but the cache structure is NOT optional

### ADR 006 Also Says (Lines 754-778):

> **Decision**: `data/cache/images/` with unified hash-based storage
>
> - **Hash-based filenames** for all images (local and external)
> - Source-agnostic cache - database tracks origin via `source_type`

**This is NOT optional** - It's a finalized design decision!

### Resolution Needed

We need to clarify:
1. **Phase 4.1**: Copy local images to cache OR serve directly from original paths?
2. **Phase 4.3**: Add transformations to existing cache OR build cache during Phase 4.3?

**Recommended Approach** (aligns with ADR intent):
- **Phase 4.1**: Catalog by reference (current implementation) ✅
- **Phase 4.2**: Add scheduled task to populate cache in background 📋
- **Phase 4.3**: Add on-demand transformations with caching 📋

---

## Impact on Related Systems

### 1. Cleanup System

**Status**: PARTIALLY BROKEN

**Why**: `CleanCacheForHashes()` expects cache files that don't exist

**Current Behavior**:
```go
// This runs but does nothing because no cache files exist
uc.CleanCacheForHashes(ctx, hashes)
```

**Fix Options**:
1. Make cleanup no-op if cache doesn't exist (graceful degradation)
2. Implement cache population before cleanup runs
3. Add check: "if cache dir empty, skip cleanup"

### 2. Delete Operations

**Status**: WORKS (but incomplete)

**Why**: DATABASE cleanup works via CASCADE, file cleanup is gracefully skipped

**Current Behavior**:
- Database: ✅ Records deleted correctly
- Cache files: ⚠️ No-op (no files to delete)
- Original files: ✅ Never touched (correct!)

### 3. Image Deduplication

**Status**: NOT WORKING

**Why**: Multiple images with same hash are stored separately

**Example**:
```
/media/Movies/Matrix (1999)/poster.jpg       # Hash: abc123...
/media/Movies/Matrix Reloaded (2003)/poster.jpg  # Same hash: abc123...
```

**Current**: Both cataloged separately, both served from original paths
**Intended**: Both point to same cached file: `data/cache/images/abc123_original.jpg`

---

## What Needs to Happen NOW

### Option 1: Minimal Fix (Keep Current Approach)

**Goal**: Make current implementation work correctly

**Changes Required**:
1. Update cleanup logic to check if cache exists before running
2. Document that we serve original files directly (not cached)
3. Update ADR 006 to reflect Phase 4.1 reality
4. Move cache population to Phase 4.2

**Pros**: No breaking changes, current code works
**Cons**: Deviates from ADR, no deduplication, no transformations

### Option 2: Implement Cache Properly (Align with ADR)

**Goal**: Match ADR 006 specification

**Changes Required**:
1. Create `CacheService` to copy images to cache
2. Update `extract_shared.go` to populate `LocalCachePath`
3. Update `ServeImage` handler to serve from cache
4. Create migration tool to populate cache for existing images
5. Fix cleanup logic to actually work

**Pros**: Matches ADR, enables deduplication, ready for transformations
**Cons**: 4-6 hours of work, risk of bugs, storage duplication

### Option 3: Hybrid Approach (Recommended)

**Goal**: Keep current implementation, but prepare for cache

**Phase 4.1 (NOW)**:
- ✅ Keep serving from original paths (current behavior)
- ✅ Continue cataloging metadata in database
- 🔧 Fix cleanup to gracefully handle missing cache
- 📝 Document that cache is Phase 4.3

**Phase 4.2 (Next)**:
- Implement `CacheService`
- Add scheduled task to populate cache in background
- No breaking changes (cache is additive)

**Phase 4.3 (Later)**:
- Add transformation support
- Update `ServeImage` to use cache when available
- Fallback to original if cache missing

**Pros**: No immediate rework, gradual migration, aligns with agile
**Cons**: Delayed deduplication benefits

---

## Recommendation

**Adopt Option 3: Hybrid Approach**

**Immediate Actions** (30 minutes):
1. Update `CleanCacheForHashes()` to check cache directory exists
2. Add comment in `extract_shared.go` explaining cache is Phase 4.3
3. Update PROJECT_PLAN.md to reflect accurate Phase 4.1 scope
4. Create this gap analysis document ✅

**Phase 4.2 Actions** (4-6 hours):
1. Implement `CacheService` with hash-based storage
2. Create background job to populate cache gradually
3. Update cleanup to use cache when available
4. Keep serving from original as fallback

**Phase 4.3 Actions** (6-8 hours):
1. Add transformation support (resize, WebP)
2. Update `ServeImage` to prefer cache
3. Implement LRU eviction
4. Performance testing with large collections

---

## Updated Success Criteria

### Phase 4.1 (Current Reality)

- ✅ Database schema supports caching (structure ready)
- ✅ Images cataloged with metadata and hashes
- ✅ API serves images (from original paths)
- ✅ Frontend displays images (via API)
- ✅ Database cleanup works (CASCADE)
- ⚠️ File cleanup gracefully skips (no cache yet)
- ❌ Image deduplication (deferred to Phase 4.3)
- ❌ Image transformations (deferred to Phase 4.3)

### Phase 4.2 (Planned)

- 📋 Cache service implemented
- 📋 Background cache population
- 📋 Hash-based deduplication
- 📋 Cleanup works with cache

### Phase 4.3 (Planned)

- 📋 On-demand transformations
- 📋 WebP conversion
- 📋 Resize support
- 📋 LRU eviction

---

## Files That Need Updates

### Immediate (Option 3 - Hybrid)

1. **internal/application/images/cleanup.go**
   - Add check: if cache dir doesn't exist or is empty, return early
   - Log info message about cache not populated yet

2. **internal/application/images/extract_shared.go**
   - Add comment explaining `LocalCachePath` is populated in Phase 4.3

3. **docs/PROJECT_PLAN.md**
   - Update Phase 4.1 status to reflect accurate scope
   - Move cache population to Phase 4.2
   - Move transformations to Phase 4.3

4. **docs/PHASE_4_1_PROGRESS.md**
   - Update to reflect true completion (cataloging works, cache deferred)

5. **docs/decisions/006-image-handling-strategy.md**
   - Add clarification about phased implementation

### Future (Phase 4.3)

1. **internal/infrastructure/images/cache.go** (NEW)
   - Implement CacheService

2. **internal/infrastructure/images/transformer.go** (NEW)
   - Implement image transformations

3. **cmd/populate-cache/main.go** (NEW)
   - Tool to populate cache from existing images

4. **internal/api/handlers/images.go**
   - Add transformation support

---

## Lessons Learned

1. **ADRs need phasing clarity**: Distinguish between "architecture decisions" and "implementation phases"
2. **Verify completion criteria**: "100% complete" should be validated against ADR specs
3. **Document deviations**: If implementation differs from ADR, update docs immediately
4. **Gap analysis matters**: Regular reality checks prevent scope creep disguised as "completion"

---

**Next Steps**: Decide on Option 1, 2, or 3 and update project tracking accordingly.
