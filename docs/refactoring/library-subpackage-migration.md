# Library Package Sub-Package Migration Plan

> Created: 2025-12-13
> Status: **Phase 6 Complete** (Phases 1-6 implemented, Phase 7 pending)

## Goal

Reduce the `internal/application/library/` package from **~3,500 lines** to **~500 lines** by moving scan functionality into `scan/` sub-packages.

## Design Decision

**Keep `ScanLibraryUseCase` in parent package**, but convert implementation methods to functions in sub-packages. The parent keeps thin wrapper methods that delegate to sub-package functions with dependency bundles.

This preserves backwards compatibility (no import path changes for consumers) while achieving the organizational goal.

### Why This Approach?

1. **Backwards compatible** - External consumers continue importing `library` package
2. **No circular imports** - Sub-packages don't import parent; parent passes dependencies down
3. **Testable** - Existing tests continue to work; sub-package functions can have unit tests
4. **Incremental** - Each phase can be completed and tested independently

---

## Current State

### Parent Package (~3,500 lines in 19 files)

| File | Lines | Purpose |
|------|-------|---------|
| `scan_orchestrator.go` | 558 | Main struct, StartScan, ResumeScan |
| `scan_discovery.go` | 362 | File discovery phases |
| `scan_checkpoint_batch.go` | 322 | Checkpoint batch processing |
| `scan_media_tv.go` | 357 | TV episode/show processing |
| `scan_media_music.go` | 264 | Music track/album/artist processing |
| `scan_hasher.go` | 216 | File hashing pipeline |
| `scan_image_extraction.go` | 186 | Image extraction orchestration |
| `scan_worker.go` | 182 | Worker goroutine logic |
| `scan_config.go` | 160 | Config structs and constants |
| `scan_tracks.go` | 143 | Audio/subtitle track persistence |
| `scan_media_movie.go` | 128 | Movie processing |
| `scan_progress.go` | 124 | Progress update builder |
| `scan_status.go` | 121 | Scan status queries |
| `scan_file_handler.go` | 109 | Per-file processing wrapper |
| `scan_cleanup.go` | 91 | Stale media cleanup |
| `scan_dto.go` | 88 | Scan DTOs |
| `scan_media_common.go` | 76 | Media upsert helper |
| `scan_validation.go` | 69 | Discovery validation (delegates) |
| `scan_utils.go` | 56 | Utility wrappers |

### Existing Sub-Packages (~500 lines)

| Package | Files | Purpose |
|---------|-------|---------|
| `scan/discovery/` | incremental.go, validation.go | Change detection, validation helpers |
| `scan/processing/` | pool.go | Generic WorkerPool |
| `scan/media/` | upsert.go | UpsertCallbacks, IsConstraintError |
| `scan/scanutil/` | dedup.go, utils.go | AtomicDeduplicator, file utilities |
| `scan/cleanup/` | (empty) | Not yet used |

---

## Target Structure

```text
internal/application/library/          # ~500 lines (down from ~3,500)
├── library_service.go                 # LibraryService CRUD (unchanged)
├── dto.go                             # Library DTOs (unchanged)
├── interfaces.go                      # Public interfaces (unchanged)
├── image_cleanup_shared.go            # Shared helper (unchanged)
├── scan_orchestrator.go               # ScanLibraryUseCase struct + thin wrappers (~150 lines)
├── scan_config.go                     # Re-exports from scan/config.go (~30 lines)
├── scan_dto.go                        # Re-exports from scan/dto.go (~20 lines)
│
└── scan/
    ├── config.go                      # ScanConfig, MediaRepositories, ScanRepositories
    ├── dto.go                         # StartScanResponse, ScanProgressResponse, etc.
    │
    ├── discovery/                     # ~550 lines total
    │   ├── incremental.go             # (existing) IncrementalScanner
    │   ├── validation.go              # (existing) CheckWalkStatsErrors, etc.
    │   ├── context.go                 # NEW: DiscoveryContext type
    │   └── phases.go                  # NEW: PhaseCountFiles, PhaseWalkDirectory, etc.
    │
    ├── processing/                    # ~830 lines total
    │   ├── pool.go                    # (existing) WorkerPool generic type
    │   ├── deps.go                    # NEW: ProcessingDeps struct
    │   ├── batch.go                   # NEW: from scan_checkpoint_batch.go
    │   ├── worker.go                  # NEW: from scan_worker.go
    │   ├── handler.go                 # NEW: from scan_file_handler.go
    │   └── hasher.go                  # NEW: from scan_hasher.go
    │
    ├── media/                         # ~1,150 lines total
    │   ├── upsert.go                  # (existing) UpsertCallbacks, IsConstraintError
    │   ├── deps.go                    # NEW: MediaDeps struct
    │   ├── interfaces.go              # NEW: Image extractor interfaces
    │   ├── common.go                  # NEW: ProcessMediaWithCache
    │   ├── movie.go                   # NEW: ProcessMovie
    │   ├── tv.go                      # NEW: ProcessTVEpisode, EnrichTVShowMetadata
    │   ├── music.go                   # NEW: ProcessMusicTrack
    │   ├── images.go                  # NEW: Image extraction functions
    │   └── tracks.go                  # NEW: PersistMediaTracks
    │
    ├── status/                        # ~250 lines total
    │   ├── status.go                  # NEW: GetScanStatus logic
    │   └── progress.go                # NEW: ProgressUpdate builder
    │
    ├── cleanup/                       # ~140 lines total
    │   └── cleanup.go                 # NEW: CleanupStaleMedia
    │
    └── scanutil/                      # (existing, unchanged)
        ├── dedup.go                   # AtomicDeduplicator
        └── utils.go                   # IsMediaFile, StatWithTimeout, etc.
```

---

## Migration Phases

### Phase 1: Move Config Types to scan/config.go

**Risk: Low**

**Files:**

- CREATE: `scan/config.go`
- MODIFY: `scan_config.go` (becomes re-exports)

**What moves:**

```go
// scan/config.go
package scan

type Config struct {
    Timeout              time.Duration
    WorkerTimeout        time.Duration
    BaseFileTimeout      time.Duration
    RemoteStorageTimeout time.Duration
    MaxExtraTimeout      time.Duration
    ParallelWalkers      int
    ProgressInterval     time.Duration
    CheckpointBatchSize  int
    // ... other fields
}

type MediaRepositories struct {
    Library library.Repository
    Media   media.Repository
    Movie   media.MovieRepository
    TV      media.TVRepository
    Music   media.MusicRepository
}

type ScanRepositories struct {
    ScanJob    scanner.ScanJobRepository
    Checkpoint scanner.CheckpointRepository
    ScanState  scanner.ScanStateRepository
}

const (
    DefaultHashWorkers              = 8
    DefaultHashBatchSize            = 10
    DefaultProcessingWorkers        = 4
    StaleMediaThresholdPercent      = 10.0
    FileDropWarningThresholdPercent = 10.0
    PermissionErrorWarningThreshold = 10
    PreviousJobsToCompare           = 5
)
```

**Parent re-exports:**

```go
// scan_config.go
package library

import "github.com/mantonx/viewra/internal/application/library/scan"

// Re-export config types for backwards compatibility
type ScanConfig = scan.Config
type MediaRepositories = scan.MediaRepositories
type ScanRepositories = scan.ScanRepositories

// Re-export constants
const (
    DefaultHashWorkers              = scan.DefaultHashWorkers
    DefaultHashBatchSize            = scan.DefaultHashBatchSize
    DefaultProcessingWorkers        = scan.DefaultProcessingWorkers
    StaleMediaThresholdPercent      = scan.StaleMediaThresholdPercent
    FileDropWarningThresholdPercent = scan.FileDropWarningThresholdPercent
    PermissionErrorWarningThreshold = scan.PermissionErrorWarningThreshold
    PreviousJobsToCompare           = scan.PreviousJobsToCompare
)
```

**Validation:** `go test ./internal/application/library/...`

---

### Phase 2: Move Scan DTOs to scan/dto.go

**Risk: Low**

**Files:**

- CREATE: `scan/dto.go`
- MODIFY: `scan_dto.go` (becomes re-exports)

**What moves:**

- `StartScanResponse`
- `ScanProgressResponse`
- `ScanHistoryResponse`
- `ScanJobSummary`

**Validation:** `go test ./internal/application/library/...`

---

### Phase 3: Migrate Media Processors to scan/media/

**Risk: Medium**

This is the largest migration phase. The pattern is to convert methods on `*ScanLibraryUseCase` to functions that receive a `MediaDeps` struct.

**Files to CREATE:**

1. `scan/media/deps.go` - Dependency bundle
2. `scan/media/interfaces.go` - Image extractor interfaces (moved from parent)
3. `scan/media/common.go` - `ProcessMediaWithCache` function
4. `scan/media/movie.go` - `ProcessMovie` function
5. `scan/media/tv.go` - `ProcessTVEpisode`, `EnrichTVShowMetadata` functions
6. `scan/media/music.go` - `ProcessMusicTrack` function
7. `scan/media/images.go` - Image extraction delegation functions
8. `scan/media/tracks.go` - `PersistMediaTracks` function

**Dependency struct:**

```go
// scan/media/deps.go
package media

type Deps struct {
    // Repositories
    MediaRepos *scan.MediaRepositories
    ScanRepos  *scan.ScanRepositories
    ImageRepo  domainImages.Repository

    // Image extractors
    MovieExtractor   MovieImageExtractor
    EpisodeExtractor TVEpisodeImageExtractor
    ShowExtractor    TVShowImageExtractor
    SeasonExtractor  TVSeasonImageExtractor
    AlbumExtractor   MusicAlbumImageExtractor
    ArtistExtractor  MusicArtistImageExtractor
    TrackExtractor   MusicTrackImageExtractor

    // Deduplication (shared across scan session)
    ProcessedArtists *scanutil.AtomicDeduplicator
    ProcessedShows   *scanutil.AtomicDeduplicator

    // Infrastructure
    Coordinator *filesystem.Coordinator
    Logger      *slog.Logger
}
```

**Example migration (movie.go):**

```go
// scan/media/movie.go
package media

func ProcessMovie(
    ctx context.Context,
    deps *Deps,
    libraryID int64,
    result *scanner.ScanResult,
    checkpoint *scanner.ScanCheckpoint,
    existingMediaCache *sync.Map,
) (*int64, error) {
    // Implementation moved from scan_media_movie.go
    // Replace uc.mediaRepos with deps.MediaRepos
    // Replace uc.logger with deps.Logger
    // etc.
}
```

**Parent wrapper (in scan_orchestrator.go):**

```go
func (uc *ScanLibraryUseCase) processMovie(
    ctx context.Context,
    libraryID int64,
    result *scanner.ScanResult,
    checkpoint *scanner.ScanCheckpoint,
    existingMediaCache *sync.Map,
) (*int64, error) {
    return media.ProcessMovie(ctx, uc.mediaDeps(), libraryID, result, checkpoint, existingMediaCache)
}

func (uc *ScanLibraryUseCase) mediaDeps() *media.Deps {
    return &media.Deps{
        MediaRepos:       uc.mediaRepos,
        ScanRepos:        uc.scanRepos,
        ImageRepo:        uc.imageRepo,
        MovieExtractor:   uc.movieImageExtractor,
        EpisodeExtractor: uc.episodeImageExtractor,
        // ... etc
        ProcessedArtists: &uc.processedArtists,
        ProcessedShows:   &uc.processedShows,
        Coordinator:      uc.coordinator,
        Logger:           uc.logger,
    }
}
```

**Files to DELETE after migration:**

- `scan_media_movie.go`
- `scan_media_tv.go`
- `scan_media_music.go`
- `scan_media_common.go`
- `scan_image_extraction.go`
- `scan_tracks.go`

**Validation:** `go test ./internal/application/library/...`

---

### Phase 4: Migrate Processing Pipeline to scan/processing/

**Risk: Medium**

**Challenge:** Processing code calls back into media processors. Solution: pass a `MediaProcessor` interface.

**Files to CREATE:**

1. `scan/processing/deps.go` - Dependency bundle + MediaProcessor interface
2. `scan/processing/batch.go` - `ProcessFilesWithCheckpoints`
3. `scan/processing/worker.go` - `ProcessCheckpointWorker`
4. `scan/processing/handler.go` - `ProcessFileWithCheckpoint`
5. `scan/processing/hasher.go` - `HashAndStreamCheckpoints`

**MediaProcessor interface:**

```go
// scan/processing/deps.go
package processing

// MediaProcessor allows processing package to call back to media processing
type MediaProcessor interface {
    ProcessMovie(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error)
    ProcessTVEpisode(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error)
    ProcessMusicTrack(ctx context.Context, libraryID int64, result *scanner.ScanResult, checkpoint *scanner.ScanCheckpoint, cache *sync.Map) (*int64, error)
}

type Deps struct {
    ScanRepos      *scan.ScanRepositories
    MediaProcessor MediaProcessor
    Config         *scan.Config
    SystemProfile  *system.Profile
    Coordinator    *filesystem.Coordinator
    Logger         *slog.Logger
}
```

**Parent implements MediaProcessor:**

```go
// ScanLibraryUseCase implements processing.MediaProcessor
var _ processing.MediaProcessor = (*ScanLibraryUseCase)(nil)

func (uc *ScanLibraryUseCase) ProcessMovie(...) (*int64, error) {
    return uc.processMovie(...)
}
// etc.
```

**Files to DELETE after migration:**

- `scan_checkpoint_batch.go`
- `scan_worker.go`
- `scan_file_handler.go`
- `scan_hasher.go`

**Validation:** `go test ./internal/application/library/...`

---

### Phase 5: Migrate Discovery to scan/discovery/

**Risk: Medium**

**Files to CREATE:**

1. `scan/discovery/context.go` - `DiscoveryContext` type (scan session state)
2. `scan/discovery/phases.go` - Phase functions

**DiscoveryContext:**

```go
// scan/discovery/context.go
package discovery

// Context holds state for a discovery session
type Context struct {
    Job              *scanner.ScanJob
    Library          *library.Library
    Coordinator      *filesystem.Coordinator
    IncrScanner      *IncrementalScanner
    ExistingMedia    map[string]int64  // filepath -> mediaID
    FoundFiles       map[string]bool
    WalkStats        *filesystem.WalkStats
    FilesToProcess   []*scanner.ScanCheckpoint
}
```

**Phase functions:**

```go
// scan/discovery/phases.go
package discovery

func PhaseCountFiles(ctx context.Context, dctx *Context, deps *Deps) (int64, error)
func PhaseWalkDirectory(ctx context.Context, dctx *Context, deps *Deps) error
func PhaseDetermineChanges(ctx context.Context, dctx *Context, deps *Deps) error
func PhaseCreateCheckpoints(ctx context.Context, dctx *Context, deps *Deps) error
```

**Files to MODIFY:**

- `scan_discovery.go` - becomes thin wrapper, keeps `runFreshScan` orchestration

**Validation:** `go test ./internal/application/library/...`

---

### Phase 6: Migrate Status & Cleanup

**Risk: Low**

**Files to CREATE:**

1. `scan/status/status.go` - `GetScanStatus` logic
2. `scan/status/progress.go` - `ProgressUpdate` builder
3. `scan/cleanup/cleanup.go` - `CleanupStaleMedia`

**Files to DELETE after migration:**

- `scan_status.go`
- `scan_progress.go`
- `scan_cleanup.go`

**Validation:** `go test ./internal/application/library/...`

---

### Phase 7: Final Cleanup

**Risk: Low**

1. Delete `scan_validation.go` (already just delegates to `discovery.*`)
2. Simplify `scan_utils.go` (may be able to delete entirely)
3. Move image extractor interfaces from `interfaces.go` to `scan/media/interfaces.go`
4. Update package documentation
5. Run full test suite: `make test`

---

## Backwards Compatibility

External consumers will continue to work unchanged because:

1. They import `library` package (not sub-packages)
2. They use interfaces (`ScanLibraryExecutor`) not concrete types
3. DTOs and config types are re-exported with type aliases

**No changes needed to:**

- `internal/app/usecases/usecases.go`
- `internal/api/handlers/library.go`
- `internal/api/handlers/scanjob.go`

---

## Test Strategy

**Test files total: ~19,500 lines**

**Approach: Tests stay in parent package**

Tests don't need to move because:

1. They test the public API (method calls on `*ScanLibraryUseCase`)
2. The implementation moving to sub-packages is an internal detail
3. Moving 19k lines of tests is high risk for low reward

**What changes in tests:**

- Tests continue to work as-is (wrapper methods still exist)
- Sub-package unit tests can be added for pure functions if desired

**Validation after each phase:**

```bash
go test ./internal/application/library/...
```

**Final validation:**

```bash
make test
```

---

## Estimated Effort

| Phase | Risk | Files Created | Files Deleted | Lines Moved |
|-------|------|---------------|---------------|-------------|
| 1. Config types | Low | 1 | 0 | ~160 |
| 2. DTOs | Low | 1 | 0 | ~88 |
| 3. Media processors | Medium | 8 | 6 | ~1,150 |
| 4. Processing pipeline | Medium | 5 | 4 | ~830 |
| 5. Discovery | Medium | 2 | 0 | ~300 |
| 6. Status & cleanup | Low | 3 | 3 | ~340 |
| 7. Cleanup | Low | 0 | 2 | ~120 |

**Net result:** Parent package reduced from ~3,500 to ~500 lines

---

## Rollback Strategy

Each phase is independent. If a phase causes issues:

1. Revert the phase's commits
2. Tests should pass again
3. Previous phases remain intact

The type alias approach means we can partially complete the migration and still have a working system.
