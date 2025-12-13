# Library Package Refactoring Opportunities

> Analysis date: 2025-12-13
> Last verified: 2025-12-13
> Package: `internal/application/library`
> Total size: **3,865 lines** across **20 implementation files** (excluding tests)

This document catalogs DRYness and organizational improvements identified in the library package. Items are prioritized by impact and grouped by category.

---

## File Inventory (Verified)

| File | Lines | Purpose |
|------|-------|---------|
| `scan_orchestrator.go` | 559 | Main scan lifecycle, StartScan, ResumeScan |
| `scan_media_tv.go` | 393 | TV episode processing |
| `scan_discovery.go` | 378 | File discovery phases |
| `scan_media_music.go` | 348 | Music track processing |
| `scan_checkpoint_batch.go` | 314 | Checkpoint worker coordination |
| `library_service.go` | 218 | Library CRUD operations |
| `scan_hasher.go` | 218 | File hashing pipeline |
| `scan_image_extraction.go` | 195 | Image extraction orchestration |
| `scan_utils.go` | 193 | Utilities (timeouts, validation) |
| `scan_worker.go` | 182 | Worker goroutine logic |
| `incremental_scanner.go` | 180 | Change detection |
| `scan_media_movie.go` | 164 | Movie processing |
| `scan_tracks.go` | 143 | Audio/subtitle persistence |
| `scan_config.go` | 122 | Configuration structs |
| `scan_status.go` | 121 | Scan status queries |
| `dto.go` | 93 | Library DTOs |
| `scan_cleanup.go` | 91 | Stale media cleanup |
| `scan_dto.go` | 87 | Scan DTOs |
| `scan_file_handler.go` | 109 | Per-file processing wrapper |
| `image_cleanup_shared.go` | 45 | Image cleanup interface |
| `interfaces.go` | 23 | Public interfaces |

---

## Priority Legend

- **P0 (Critical)**: High duplication, significant maintenance burden
- **P1 (High)**: Notable repetition, moderate maintenance impact
- **P2 (Medium)**: Minor duplication, nice-to-have cleanup
- **P3 (Low)**: Style/consistency improvements

---

## P0: Critical - Media Processing Duplication

### 1. Race Condition Handling (~150 duplicated lines)

**Files affected (verified):**

- `scan_media_movie.go:122-154` (33 lines)
- `scan_media_tv.go:165-198` (34 lines)
- `scan_media_music.go:258-340` (83 lines - extended with artist/album logic)

**Pattern:** All three files have nearly identical UNIQUE constraint recovery logic:
```go
if strings.Contains(err.Error(), "UNIQUE constraint failed") {
    if value, found := existingMediaCache.Load(result.FilePath); found {
        // Update existing entry
        media.ID = value.(int64)
        uc.mediaRepos.[Type].Update(ctx, &media)
        uc.extractImagesFor[Type](ctx, media, ...)
        return &media.ID, nil
    }
    // Fetch from DB, store in cache, update...
}
```

**Proposed solution:** Extract to a generic handler:
```go
type MediaUpsertHandler[T any] struct {
    Cache       *sync.Map
    Create      func(ctx context.Context, media *T) error
    Update      func(ctx context.Context, media *T) error
    FetchByPath func(ctx context.Context, path string) (*T, error)
    Enrich      func(ctx context.Context, media *T) error
}

func (h *MediaUpsertHandler[T]) ProcessWithCache(ctx context.Context, filePath string, media *T) (*int64, error)
```

- [ ] Design generic MediaUpsertHandler interface
- [ ] Implement for Movie, TVEpisode, MusicTrack
- [ ] Add comprehensive tests
- [ ] Migrate existing code

---

### 2. Cache-Based Deduplication (~50 duplicated lines)

**Files affected:**
- `scan_media_movie.go:103-118`
- `scan_media_tv.go:146-161`
- `scan_media_music.go:152-216`

**Pattern:** Each media type checks cache before creating:
```go
if value, found := existingMediaCache.Load(result.FilePath); found {
    media.ID = value.(int64)
    uc.mediaRepos.[Type].Update(ctx, &media)
    uc.extractImagesFor[Type](ctx, media, ...)
    return &media.ID, nil
}
```

**Proposed solution:** Part of the MediaUpsertHandler above, or a simpler callback pattern:
```go
func (uc *ScanLibraryUseCase) processMediaWithCache(
    ctx context.Context,
    filePath string,
    cache *sync.Map,
    onCacheHit func(id int64) error,
    onCacheMiss func() (*int64, error),
) (*int64, error)
```

- [ ] Consolidate with race condition handling refactor

---

## P1: High Priority

### 3. Panic Recovery Variants (~52 lines, 3 implementations)

**File:** `scan_orchestrator.go`

**Current implementations (verified):**

- `recoverFromPanic()` (lines 507-527) - marks job as FAILED, used for main scan goroutines
- `recoverFromPanicWithError()` (lines 532-546) - sends error to channel, non-blocking
- `recoverWorkerPanic()` (lines 551-559) - logs only, does NOT fail job (other workers continue)

All share identical stack trace formatting logic (`debug.Stack()`).

**Proposed solution:**
```go
type PanicRecoveryMode int
const (
    RecoveryMarkFailed PanicRecoveryMode = iota
    RecoverySendError
    RecoveryLogOnly
)

func (uc *ScanLibraryUseCase) recoverFromPanic(
    mode PanicRecoveryMode,
    jobID, libraryID int64,
    description string,
    errChan chan<- error, // nil for non-channel modes
) func()
```

- [ ] Implement unified panic recovery
- [ ] Add tests for each recovery mode
- [ ] Migrate existing usages

---

### 4. Atomic Deduplication Pattern (~20 duplicated lines)

**File:** `scan_image_extraction.go`

**Current implementations:**
```go
func (uc *ScanLibraryUseCase) tryMarkArtistProcessed(artistName string) bool
func (uc *ScanLibraryUseCase) tryMarkShowMetadataProcessed(showTitle string) bool
```

Both use identical `LoadOrStore` on separate `sync.Map` instances.

**Proposed solution:**
```go
type AtomicDeduplicator struct {
    seen sync.Map
}

func (d *AtomicDeduplicator) TryMark(key string) bool {
    _, loaded := d.seen.LoadOrStore(key, struct{}{})
    return !loaded
}

func (d *AtomicDeduplicator) Reset() {
    d.seen = sync.Map{}
}
```

Usage:
```go
type ScanLibraryUseCase struct {
    processedArtists AtomicDeduplicator
    processedShows   AtomicDeduplicator
}
```

- [ ] Create AtomicDeduplicator type
- [ ] Replace existing sync.Map usages
- [ ] Add tests

---

### 5. Image Extraction Warning Tracking (~25 duplicated lines, 5 occurrences)

**File:** `scan_image_extraction.go`

**Pattern repeated 5 times:**
```go
if setErr := uc.scanRepos.ScanState.SetWarning(ctx, libraryID, filePath, err.Error(), "image_extraction"); setErr != nil {
    uc.logger.Warn("failed to set image extraction warning in scan_state",
        "library_id", libraryID,
        "file_path", filePath,
        "original_error", err.Error(),
        "set_warning_error", setErr.Error(),
    )
}
```

**Proposed solution:**
```go
func (uc *ScanLibraryUseCase) recordImageWarning(ctx context.Context, libraryID int64, filePath string, err error) {
    if setErr := uc.scanRepos.ScanState.SetWarning(ctx, libraryID, filePath, err.Error(), "image_extraction"); setErr != nil {
        uc.logger.Warn("failed to set image extraction warning",
            "library_id", libraryID,
            "file_path", filePath,
            "original_error", err.Error(),
            "set_warning_error", setErr.Error(),
        )
    }
}
```

- [ ] Extract helper method
- [ ] Replace 5 occurrences

---

### 6. Image Extraction Interface for Testability (~195 lines, 0-28% coverage)

**File:** `scan_image_extraction.go`

**Current issue:** Image extraction uses concrete types (`movieImageExtractor`, `episodeImageExtractor`, etc.) created inline, making it impossible to mock for unit tests. Current coverage is 0-28.6%.

**Current implementations (7 extractor types):**
```go
extractor := movieImageExtractor{
    media:    movie,
    filepath: filePath,
    logger:   uc.logger,
    // ...concrete dependencies
}
result := extractor.Execute(ctx)
```

**Proposed solution - Define interface and factory:**
```go
// ImageExtractor is the common interface for all media type extractors
type ImageExtractor interface {
    Execute(ctx context.Context) *imageExtractionResult
}

// ImageExtractorFactory creates extractors for different media types
type ImageExtractorFactory interface {
    NewMovieExtractor(movie *media.Movie, filepath string) ImageExtractor
    NewEpisodeExtractor(episode *media.TVEpisode, filepath string) ImageExtractor
    NewTrackExtractor(track *media.Track, filepath string) ImageExtractor
    NewAlbumExtractor(album *media.Album, filepath string) ImageExtractor
    NewArtistExtractor(artistName string) ImageExtractor
    NewShowExtractor(show *media.TVShow) ImageExtractor
    NewSeasonExtractor(season *media.TVSeason, showID int64) ImageExtractor
}

// Inject into ScanLibraryUseCase
type ScanLibraryUseCase struct {
    // existing fields...
    imageExtractorFactory ImageExtractorFactory // nil = use default
}
```

**Files to modify:**

- `scan_image_extraction.go` - Define interface, wrap existing extractors
- `scan_usecase.go` - Add factory field to struct
- `scan_config.go` - Add factory to constructor params (optional)
- `testutil/mocks/mock_image_extractor.go` - New mock implementation

**Benefits:**

- Enables mocking image extraction in tests
- Improves coverage of `processMovie`, `processTVEpisode`, `processMusicTrack`
- Decouples scan logic from FFmpeg/image dependencies

**Tasks:**

- [ ] Define `ImageExtractor` interface
- [ ] Define `ImageExtractorFactory` interface
- [ ] Create `defaultImageExtractorFactory` implementing the interface
- [ ] Add factory field to `ScanLibraryUseCase`
- [ ] Update extraction call sites to use factory
- [ ] Create mock factory for tests
- [ ] Add tests for extraction orchestration logic

---

### 7. Worker Pool Pattern (~80 duplicated lines)

**Files affected:**
- `scan_hasher.go:52-114` - Hash worker pool
- `scan_checkpoint_batch.go:96-111` - Checkpoint worker pool

**Shared pattern:**
1. Create buffered channel
2. Launch N workers with WaitGroup
3. Feed work through channel
4. Close channel when done
5. Wait for workers with panic recovery

**Proposed solution:**
```go
type WorkerPool[T any] struct {
    NumWorkers int
    Process    func(ctx context.Context, item T) error
    OnError    func(err error)
    OnPanic    func(workerID int, recovered any)
}

func (p *WorkerPool[T]) Run(ctx context.Context, items <-chan T) error
```

- [ ] Design WorkerPool generic type
- [ ] Implement with proper lifecycle management
- [ ] Migrate hasher and checkpoint workers

---

### 7. Repository CRUD Patterns (~100 duplicated lines)

**Files affected:**
- `scan_media_movie.go:157-163`
- `scan_media_tv.go:201-208`
- `scan_media_music.go:343-347`

**Pattern:** Post-create/update enrichment:
```go
uc.extractImagesFor[Type](ctx, media, result.FilePath, ...)
uc.persistMediaTracks(ctx, media.ID, result)
checkpoint.MediaID = &media.ID
return &media.ID, nil
```

**Proposed solution:** Part of MediaUpsertHandler with enrichment callback.

- [ ] Consolidate with item #1

---

## P2: Medium Priority

### 8. NFO Metadata Parsing (~90 similar lines)

**Files affected:**
- `scan_media_movie.go:62-94` - Movie NFO parsing
- `scan_media_tv.go:111-142` - Episode NFO parsing
- `scan_media_tv.go:213-276` - Show NFO enrichment

**Pattern:** Find NFO → Parse → Conditionally populate fields

**Proposed solution:** Consider a shared NFO enrichment helper, but complexity may not justify it since field mappings differ per type.

- [ ] Evaluate if abstraction is worth the complexity
- [ ] If yes, create NFOEnricher interface

---

### 9. Context Timeout Patterns (inconsistent)

**Files affected:**
- `scan_orchestrator.go:272` - `context.WithTimeout(context.Background(), uc.config.Timeout)`
- `scan_worker.go:57` - `context.WithTimeout(ctx, uc.config.WorkerTimeout)`
- `scan_file_handler.go:39` - `context.WithTimeout(ctx, timeout)` (dynamic)
- `scan_utils.go:72-94` - `statWithTimeout()` with channel-based timeout

**Issues:**
- Three different timeout strategies
- `scan_file_handler.go:21` has hardcoded `30*time.Second`

**Proposed solution:**
```go
func (uc *ScanLibraryUseCase) newFileContext(ctx context.Context, fileSize int64) (context.Context, context.CancelFunc) {
    timeout := uc.calculateProcessingTimeout(fileSize)
    return context.WithTimeout(ctx, timeout)
}
```

- [ ] Audit all timeout usages
- [ ] Create consistent timeout helpers
- [ ] Remove hardcoded values

---

### 10. Progress Update Logic (~40 duplicated lines)

**Files affected:**
- `scan_checkpoint_batch.go:189-209`
- `scan_discovery.go:141-159`
- `scan_discovery.go:221-233`
- `scan_orchestrator.go:130-135`

**Pattern:** Progress struct creation with similar fields:
```go
progress := &scanner.Progress{
    FilesFound:     filesFound,
    FilesProcessed: processed,
    LastUpdate:     time.Now(),
    Phase:          phase,
    EstimatedTotal: estimated,
    DiscoveryDone:  done,
}
```

**Proposed solution:**
```go
func (uc *ScanLibraryUseCase) updateProgress(ctx context.Context, jobID int64, opts ProgressUpdate) error {
    // Build progress, handle errors consistently
}

type ProgressUpdate struct {
    Phase          string
    FilesFound     int
    FilesProcessed int
    DiscoveryDone  bool
}
```

- [ ] Create progress update helper
- [ ] Migrate existing usages

---

### 11. Error Wrapping Inconsistencies

**Issues identified:**
- 50+ errors wrapped with `fmt.Errorf("failed to X: %w", err)`
- 8+ errors silently ignored with `_ = ` (`scan_orchestrator.go:237,267,501,521`)
- Inconsistent use of `scanner.IsScanJobDeleted()` check

**Proposed solution:**
```go
func (uc *ScanLibraryUseCase) wrapError(action string, err error) error {
    return fmt.Errorf("failed to %s: %w", action, err)
}

func (uc *ScanLibraryUseCase) isScanDeleted(err error) bool {
    return scanner.IsScanJobDeleted(err)
}

func (uc *ScanLibraryUseCase) isConstraintError(err error) bool {
    return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
```

- [ ] Audit error handling patterns
- [ ] Create consistent error helpers
- [ ] Document intentionally ignored errors

---

### 12. Logging Patterns (~118 occurrences)

**Files with most repetition:**
- `scan_discovery.go` - 23 log statements
- `scan_checkpoint_batch.go` - 18 log statements
- `scan_orchestrator.go` - 23 log statements

**Repeated patterns:**
- `uc.logger.Warn("failed to X", "field", value, "error", err)` - 28+ occurrences
- `uc.logger.Error("failed to X", "job_id", jobID, "error", err)` - 20+ occurrences

**Proposed solution:** Consider if helpers add value or just indirection. Logging is already fairly consistent.

- [ ] Review if abstraction is worthwhile
- [ ] At minimum, ensure consistent field ordering

---

## P3: Low Priority

### 13. Magic Numbers & Hardcoded Values

**Values needing constants:**
| Value | Location | Purpose |
|-------|----------|---------|
| `30*time.Second` | `scan_file_handler.go:21` | Base file timeout |
| `4` | `scan_checkpoint_batch.go:115` | Default worker count |
| `8` | `scan_hasher.go:39` | Hash workers |
| `10` | `scan_hasher.go:40` | Hash batch size |
| `10.0` | `scan_cleanup.go:34` | Stale percent threshold |
| `10.0` | `scan_utils.go:141` | File drop warning threshold |
| `10` | `scan_utils.go:119` | Permission error threshold |

**Proposed solution:** Move to `scan_config.go` with named constants.

- [ ] Audit all magic numbers
- [ ] Move to configuration or named constants
- [ ] Add documentation for threshold rationale

---

### 14. Test Helper Consolidation

**Files affected:** Multiple `*_test.go` files

**Repeated patterns:**
- Mock repository setup
- Logger initialization (`slog.New(slog.NewTextHandler(io.Discard, nil))`)
- Test library/checkpoint/job creation

**Proposed solution:** Create `scan_test_helpers_test.go`:
```go
func createTestLibrary() *library.Library
func createTestScanJob(libraryID int64) *scanner.ScanJob
func createTestCheckpoint(jobID int64) *scanner.ScanCheckpoint
func setupMockRepositories(t *testing.T) (*MockRepos, func())
func discardLogger() *slog.Logger
```

- [ ] Identify common test patterns
- [ ] Extract to shared test helpers
- [ ] Update existing tests

---

### 15. Discovery Validation Function Size

**File:** `scan_utils.go:141-193`

`validateDiscovery()` does multi-level checks that could be smaller functions:
- Discovery error checking
- File drop comparison with previous scan
- Network error detection

**Proposed solution:**
```go
func (uc *ScanLibraryUseCase) checkDiscoveryErrors(stats *filesystem.DiscoveryStats) error
func (uc *ScanLibraryUseCase) detectFileDrop(current, previous int) bool
func (uc *ScanLibraryUseCase) detectNetworkErrors(stats *filesystem.DiscoveryStats) error
```

- [ ] Split validateDiscovery into focused functions

---

## Proposed File Structure

The package has grown to 20 files and ~3,865 lines. While the current flat structure works, it could benefit from sub-package organization to improve discoverability and enforce boundaries.

### Current Structure (Flat)

```text
internal/application/library/
├── dto.go                      # Library DTOs
├── image_cleanup_shared.go     # Image cleanup interface
├── incremental_scanner.go      # Change detection
├── interfaces.go               # Public interfaces
├── library_service.go          # CRUD operations
├── scan_checkpoint_batch.go    # Checkpoint workers
├── scan_cleanup.go             # Stale media cleanup
├── scan_config.go              # Configuration
├── scan_discovery.go           # File discovery
├── scan_dto.go                 # Scan DTOs
├── scan_file_handler.go        # Per-file processing
├── scan_hasher.go              # Hashing pipeline
├── scan_image_extraction.go    # Image extraction
├── scan_media_movie.go         # Movie processor
├── scan_media_music.go         # Music processor
├── scan_media_tv.go            # TV processor
├── scan_orchestrator.go        # Main orchestrator
├── scan_status.go              # Status queries
├── scan_tracks.go              # Track persistence
├── scan_utils.go               # Utilities
└── scan_worker.go              # Worker logic
```

### Proposed Structure (Sub-packages)

```text
internal/application/library/
├── library.go                  # Re-exports, package docs
├── service.go                  # LibraryService (CRUD)
├── dto.go                      # Library DTOs
├── interfaces.go               # Public interfaces
│
├── scan/                       # Scan orchestration
│   ├── orchestrator.go         # StartScan, ResumeScan, lifecycle
│   ├── config.go               # ScanConfig, MediaRepositories, ScanRepositories
│   ├── status.go               # GetScanStatus, ETA calculations
│   ├── dto.go                  # Scan DTOs
│   └── errors.go               # Scan-specific errors (new)
│
├── scan/discovery/             # File discovery phase
│   ├── discovery.go            # phaseCountFiles, phaseWalkDirectory
│   ├── incremental.go          # IncrementalScanner, DetermineChanges
│   └── validation.go           # validateDiscovery (extracted from utils)
│
├── scan/processing/            # Checkpoint processing pipeline
│   ├── batch.go                # Checkpoint batch coordination
│   ├── worker.go               # Worker goroutine logic
│   ├── handler.go              # Per-file processing wrapper
│   ├── hasher.go               # Hashing pipeline
│   └── pool.go                 # Generic WorkerPool (new - item #6)
│
├── scan/media/                 # Media type processors
│   ├── common.go               # MediaUpsertHandler, shared logic (new - items #1,2,7)
│   ├── movie.go                # processMovie
│   ├── tv.go                   # processTVEpisode, enrichTVShowMetadata
│   ├── music.go                # processMusicTrack
│   ├── tracks.go               # persistMediaTracks, discoverExternalSubtitles
│   └── images.go               # extractImagesFor*, recordImageWarning
│
├── scan/cleanup/               # Cleanup operations
│   ├── cleanup.go              # handleDeletedFiles, stale media cleanup
│   └── images.go               # ImageCleanupExecutor, CollectImageHashes
│
└── scan/internal/              # Internal utilities (not exported)
    ├── utils.go                # isMediaFile, calculateTimeout, statWithTimeout
    ├── dedup.go                # AtomicDeduplicator (new - item #4)
    └── recovery.go             # Panic recovery helpers (new - item #3)
```

### Benefits of Proposed Structure

| Benefit | Description |
|---------|-------------|
| **Discoverability** | Clear where to find code for each concern |
| **Encapsulation** | `scan/internal/` hides implementation details |
| **Testability** | Sub-packages can be tested independently |
| **Dependency clarity** | Import paths show relationships |
| **Onboarding** | New developers understand structure faster |

### Migration Strategy

1. **Phase 1: Extract shared utilities** (low risk)
   - Create `scan/internal/` with `dedup.go`, `recovery.go`, `utils.go`
   - Update imports in existing files

2. **Phase 2: Extract media processors** (medium risk)
   - Create `scan/media/` with `common.go` implementing MediaUpsertHandler
   - Move `scan_media_*.go` files, refactor to use common handler

3. **Phase 3: Extract discovery** (medium risk)
   - Create `scan/discovery/` with incremental scanner and validation

4. **Phase 4: Extract processing pipeline** (higher risk)
   - Create `scan/processing/` with worker pool abstraction
   - Most complex due to shared state in ScanLibraryUseCase

5. **Phase 5: Reorganize orchestration** (final)
   - Move remaining scan files to `scan/` root
   - Update all imports

### Considerations

**Shared State Challenge:**

The `ScanLibraryUseCase` struct holds shared state used across files:

- `processedArtists sync.Map`
- `processedShows sync.Map`
- `coordinator *filesystem.Coordinator`
- All repository references

**Options:**

1. **Keep ScanLibraryUseCase in parent package** - Sub-packages receive it as dependency
2. **Split into smaller use cases** - DiscoveryUseCase, ProcessingUseCase, etc.
3. **Use context-based state passing** - More explicit but verbose

**Recommendation:** Option 1 is safest. Keep `ScanLibraryUseCase` in `library/scan/` and have sub-packages define interfaces they need, with the orchestrator wiring them together.

### Alternative: Minimal Reorganization

If full sub-packages feel too disruptive, a lighter approach:

```text
internal/application/library/
├── library_service.go          # CRUD (unchanged)
├── dto.go                      # DTOs (unchanged)
├── interfaces.go               # Interfaces (unchanged)
│
├── scan.go                     # Re-exports scan types
├── scan_orchestrator.go        # Orchestration (unchanged)
├── scan_config.go              # Config (unchanged)
│
├── scan_discovery.go           # Discovery (unchanged)
├── scan_incremental.go         # Renamed from incremental_scanner.go
│
├── scan_processing.go          # Merged: batch + worker + handler
├── scan_hasher.go              # Hashing (unchanged)
│
├── scan_media.go               # NEW: MediaUpsertHandler + shared logic
├── scan_media_movie.go         # Simplified, uses scan_media.go
├── scan_media_tv.go            # Simplified, uses scan_media.go
├── scan_media_music.go         # Simplified, uses scan_media.go
│
├── scan_support.go             # Merged: tracks + images + cleanup
├── scan_status.go              # Status (unchanged)
└── scan_utils.go               # Utils + dedup + recovery helpers
```

This reduces 20 files to 15 while extracting duplicated patterns, without introducing sub-packages.

---

## Implementation Order

Recommended order based on impact and dependencies:

1. **MediaUpsertHandler** (items 1, 2, 8) - Highest impact, ~270 lines
2. **Image extraction interface** (item 6) - Enables testability, ~195 lines
3. **AtomicDeduplicator** (item 4) - Simple, enables cleaner code
4. **Panic recovery consolidation** (item 3) - Moderate impact
5. **Image warning helper** (item 5) - Quick win
6. **Worker pool abstraction** (item 7) - Useful but complex
7. **Progress update helper** (item 11) - Moderate impact
8. **Error helpers** (item 12) - Improves consistency
9. **Magic numbers** (item 14) - Easy cleanup
10. **Test helpers** (item 15) - Improves test maintainability
11. **Remaining items** - As time permits

---

## Exported Types (Public API)

| Type | File | Purpose |
|------|------|---------|
| `LibraryServiceInterface` | interfaces.go | CRUD operations interface |
| `ScanLibraryExecutor` | interfaces.go | Scan operations interface |
| `LibraryService` | library_service.go | CRUD implementation |
| `ScanLibraryUseCase` | scan_orchestrator.go | Scan orchestration |
| `ScanConfig` | scan_config.go | Configuration container |
| `MediaRepositories` | scan_config.go | Repository collection |
| `ScanRepositories` | scan_config.go | Repository collection |
| `IncrementalScanner` | incremental_scanner.go | Change detection |
| `ImageCleanupExecutor` | image_cleanup_shared.go | Cleanup interface |
| Various DTOs | dto.go, scan_dto.go | 12 request/response types |

Any restructuring must preserve these exports or provide migration path.

---

## Notes

- The package has sophisticated performance optimizations that should be preserved:
  - Coordinator reuse (single instance for all files)
  - XXH3-128 hashing (50-100x faster than SHA256)
  - Checkpoint streaming with batched DB writes
  - Parallel hash workers (8 concurrent)
  - Incremental scanning (mtime+size comparison)
- Current file organization by responsibility is reasonable
- Focus on pattern extraction before considering restructuring
- The `ScanLibraryUseCase` struct's shared state (`sync.Map` caches, coordinator) is the main barrier to sub-package extraction

---

## Performance Testing

### Current Infrastructure (Verified)

#### Existing Benchmarks

**Library Package** (`internal/application/library/`):

| Benchmark | File:Line | Purpose |
|-----------|-----------|---------|
| `BenchmarkIsMediaFile` | `scan_utils_test.go:442` | Media extension lookup |
| `BenchmarkIsExtra` | `scan_utils_test.go:453` | Extra file pattern matching |
| `BenchmarkTryMarkArtistProcessed` | `scan_image_extraction_test.go:248` | sync.Map contention (uses `b.RunParallel`) |

**Infrastructure Benchmarks**:

| Benchmark | File | Purpose |
|-----------|------|---------|
| `BenchmarkNewService` | `ffmpeg/service_test.go:917` | FFmpeg service init |
| `BenchmarkServiceOperations` | `ffmpeg/service_test.go:937` | Paths, ProbeClient, ThumbnailClient |
| `BenchmarkCreateLogWriter` | `transcoding/logging/ffmpeg_log_store_test.go:681` | Log writer creation |
| `BenchmarkWrite` | `transcoding/logging/ffmpeg_log_store_test.go:690` | Log write throughput |
| `BenchmarkNumber` | `transcoding/segment/segment_test.go:188` | Segment numbering |
| `BenchmarkGetInfo` | `transcoding/storage/storage_test.go:378` | Storage info queries |

#### Makefile Target

```makefile
test: ## Run tests
    go test -v -race -coverprofile=coverage.out ./...
```

#### Health Endpoint Metrics

`/health` exposes basic runtime stats:

- `runtime.NumGoroutine()` - Active goroutine count
- `runtime.MemStats.Alloc` - Memory usage in MB
- `runtime.NumCPU()` - CPU count

### What's Missing

| Gap | Impact | Effort |
|-----|--------|--------|
| No pprof endpoints | Can't profile production | Low |
| No hash throughput benchmark | Can't measure XXH3 performance | Low |
| No checkpoint batch benchmark | Can't measure worker contention | Medium |
| No full scan benchmark | Can't measure end-to-end throughput | Medium |
| No benchmark tracking in CI | Can't detect regressions | Medium |
| No load testing setup | Can't test concurrent scans | High |

### Recommended Additions

#### 1. Add pprof Endpoints (Quick Win)

```go
// internal/api/routes.go
import _ "net/http/pprof"

// In route setup, when dev mode enabled:
if os.Getenv("VIEWRA_DEV_MODE") == "1" {
    r.PathPrefix("/debug/pprof/").Handler(http.DefaultServeMux)
}
```

**Usage:**

```bash
# CPU profile during scan
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30

# Memory profile
go tool pprof http://localhost:8080/debug/pprof/heap
```

- [ ] Add pprof import and routes
- [ ] Gate behind dev mode flag

#### 2. Add Scanner Benchmarks

```go
// internal/application/library/scan_benchmark_test.go

func BenchmarkHashFile(b *testing.B) {
    data := make([]byte, 10*1024*1024) // 10MB
    rand.Read(data)

    b.SetBytes(int64(len(data)))
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        xxh3.Hash128(data)
    }
}

func BenchmarkCheckpointBatch(b *testing.B) {
    // Setup mock repos and workers
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            // Process checkpoint
        }
    })
}

func BenchmarkCacheLookup(b *testing.B) {
    cache := &sync.Map{}
    // Pre-populate with 10k entries
    for i := 0; i < 10000; i++ {
        cache.Store(fmt.Sprintf("/path/to/file%d.mkv", i), int64(i))
    }

    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            cache.Load(fmt.Sprintf("/path/to/file%d.mkv", i%10000))
            i++
        }
    })
}
```

- [ ] Create `scan_benchmark_test.go`
- [ ] Add `BenchmarkHashFile` with `b.SetBytes()` for throughput
- [ ] Add `BenchmarkCheckpointBatch` with `b.RunParallel()`
- [ ] Add `BenchmarkCacheLookup` for sync.Map contention

#### 3. Add Makefile Targets

```makefile
bench: ## Run all benchmarks
    go test -bench=. -benchmem -benchtime=3s ./... | tee benchmarks.txt

bench-scan: ## Run scanner benchmarks only
    go test -bench=Benchmark -benchmem ./internal/application/library/...

bench-compare: ## Compare benchmarks (requires benchstat)
    @if [ -f benchmarks.old.txt ]; then \
        benchstat benchmarks.old.txt benchmarks.txt; \
    else \
        echo "No baseline found. Run 'make bench' first, then 'mv benchmarks.txt benchmarks.old.txt'"; \
    fi
```

- [ ] Add `bench` target
- [ ] Add `bench-scan` target
- [ ] Add `bench-compare` target

#### 4. Benchmark Tracking in CI

```yaml
# .github/workflows/benchmarks.yml
name: Benchmarks

on:
  push:
    branches: [main]

jobs:
  benchmark:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run benchmarks
        run: go test -bench=. -benchmem -count=5 ./... | tee bench.txt

      - name: Compare with baseline
        uses: benchmark-action/github-action-benchmark@v1
        with:
          tool: 'go'
          output-file-path: bench.txt
          fail-on-alert: true
          alert-threshold: '150%'
```

- [ ] Create benchmark workflow
- [ ] Configure alert threshold

### Performance Metrics to Track

| Metric | Target | Benchmark |
|--------|--------|-----------|
| Hash throughput | >500 MB/s | `BenchmarkHashFile` |
| Files/second | >100 files/sec | `BenchmarkFullScan` |
| Memory per file | <1 KB | `-benchmem` output |
| Cache lookup | <100 ns/op | `BenchmarkCacheLookup` |
| Checkpoint write | <1 ms/op | `BenchmarkCheckpointBatch` |

### Test Data Generation

Create script for generating test libraries:

```bash
#!/bin/bash
# scripts/generate-test-library.sh

SIZE=${1:-1000}
OUTPUT=${2:-/tmp/viewra-test-library}

mkdir -p "$OUTPUT"/{movies,tv,music}

for i in $(seq 1 $SIZE); do
    dir="$OUTPUT/movies/Movie $i (2024)"
    mkdir -p "$dir"
    # Create small valid MKV header (avoids full media files)
    head -c 1024 /dev/urandom > "$dir/Movie $i (2024).mkv"
    echo "<?xml version=\"1.0\"?><movie><title>Movie $i</title></movie>" > "$dir/Movie $i (2024).nfo"
done
```

- [ ] Create `scripts/generate-test-library.sh`
- [ ] Add to `.gitignore`: `/tmp/viewra-test-library/`
