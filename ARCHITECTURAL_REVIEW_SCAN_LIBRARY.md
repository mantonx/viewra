# Comprehensive Architectural Review: Library Scanner Implementation

**Reviewer**: Senior Backend Engineer
**Date**: 2025-11-19
**Files Analyzed**:
- `/home/fictional/Projects/viewra2/internal/application/library/scan_library.go`
- `/home/fictional/Projects/viewra2/internal/infrastructure/filesystem/coordinator.go`
- `/home/fictional/Projects/viewra2/internal/domain/scanner/types.go`
- Related infrastructure components

**Scope**: Backend architecture, Go best practices, concurrency, resource management beyond ADR 014 issues

---

## Executive Summary

The library scanner is a critical component with **good foundational architecture** but contains **10 significant issues** across concurrency, resource management, and Go best practices. While ADR 014 correctly identifies logging and database operation improvements, this review uncovers **deeper architectural concerns** that could lead to:

- **Goroutine leaks** under error conditions
- **Data races** in shared state management
- **Context misuse** risking premature cancellation
- **Mutex deadlock potential** in cleanup operations
- **Interface design issues** reducing testability

**Severity Breakdown**:
- **CRITICAL**: 2 issues (goroutine leaks, context lifetime management)
- **HIGH**: 4 issues (data races, channel closing safety, mutex lock ordering, state machine violations)
- **MEDIUM**: 4 issues (interface design, error wrapping, defer placement, memory efficiency)

---

## CRITICAL SEVERITY ISSUES

### Issue 1: Goroutine Leak via Context Cancellation

**Category**: Concurrency & Synchronization
**Severity**: CRITICAL
**File**: `scan_library.go:150-154`

#### Description
The `StartScan` method creates a new background context with timeout that is **never canceled on error paths**, leading to guaranteed goroutine leaks when scans fail before the timeout expires.

#### Evidence
```go
// Line 150-154
scanCtx, cancel := context.WithTimeout(context.Background(), uc.scanTimeout)
go func() {
    defer cancel()
    uc.runScan(scanCtx, job.ID, lib)
}()
```

**Problem**: The `cancel()` is only called when the goroutine completes naturally. If `runScan` panics or the coordinator hangs, the goroutine and its context remain alive until the timeout expires (potentially hours).

#### Impact
- **Resource Leak**: Each failed scan leaks a goroutine until timeout
- **Timer Leak**: `context.WithTimeout` creates a timer that won't be cleaned up
- **Memory Pressure**: Context chain and associated allocations remain in memory
- **Production Risk**: After 100 failed scans, server has 100 leaked goroutines

#### Recommendation

**Solution 1 - Proper Context Cancellation** (Recommended):
```go
func (uc *ScanLibraryUseCase) StartScan(ctx context.Context, libraryID int64) (StartScanResponse, error) {
    // ... validation code ...

    if err := uc.scanJobRepo.Create(ctx, job); err != nil {
        return StartScanResponse{}, fmt.Errorf("failed to create scan job: %w", err)
    }

    // Create context with timeout AND ensure cancellation
    scanCtx, cancel := context.WithTimeout(context.Background(), uc.scanTimeout)

    // Start background scan with proper cleanup
    go func() {
        defer cancel() // Always called, even on panic
        defer func() {
            if r := recover(); r != nil {
                // Log panic and mark job as failed
                uc.logger.Error("Scan panicked",
                    "library_id", libraryID,
                    "panic", r,
                    "stack", debug.Stack())

                job.Status = scanner.ScanStatusFailed
                job.ErrorMessage = fmt.Sprintf("scan panicked: %v", r)
                _ = uc.scanJobRepo.Complete(context.Background(), job)
            }
        }()
        uc.runScan(scanCtx, job.ID, lib)
    }()

    return ToStartScanResponse(job), nil
}
```

**Solution 2 - Timeout Management**:
Store the cancel function to allow early cancellation:
```go
type ScanLibraryUseCase struct {
    // ... existing fields ...

    activeScansMu sync.RWMutex
    activeScans   map[int64]context.CancelFunc // jobID -> cancel
}

func (uc *ScanLibraryUseCase) StartScan(ctx context.Context, libraryID int64) {
    // ... validation ...

    scanCtx, cancel := context.WithTimeout(context.Background(), uc.scanTimeout)

    uc.activeScansMu.Lock()
    uc.activeScans[job.ID] = cancel
    uc.activeScansMu.Unlock()

    go func() {
        defer func() {
            uc.activeScansMu.Lock()
            delete(uc.activeScans, job.ID)
            uc.activeScansMu.Unlock()
            cancel()
        }()
        uc.runScan(scanCtx, job.ID, lib)
    }()
}
```

---

### Issue 2: Context Lifetime Violation - Using Expired Context

**Category**: Concurrency & Synchronization
**Severity**: CRITICAL
**File**: `scan_library.go:206-208, 232`

#### Description
After `coordinator.Scan()` completes (which may be due to context cancellation), the code continues to use the **expired context** for database operations in `cleanupStaleMedia` and `Complete`. This violates Go context semantics and causes operations to fail unnecessarily.

#### Evidence
```go
// Line 195-208
scanErr := coordinator.Scan(ctx, lib.Path, resultChan)
close(resultChan)

// Wait for result processing to complete
<-processDone
<-foundFilesCollectorDone

// THIS IS THE BUG: ctx may be cancelled/expired here
if scanErr == nil && uc.imageRepo != nil && uc.imageCleanup != nil {
    uc.cleanupStaleMedia(ctx, lib.ID, foundFiles)  // Uses potentially cancelled ctx
}

// Line 232
if err := uc.scanJobRepo.Complete(ctx, job); err != nil {  // Uses cancelled ctx
    fmt.Printf("failed to complete scan job: %v\n", err)
}
```

#### Impact
- **Silent Failures**: Database operations fail with "context canceled" but are logged as generic errors
- **Incomplete State**: Scan job may not be marked complete in database
- **Stale Data**: Cleanup doesn't run, leaving orphaned database records
- **Misleading Errors**: Timeout looks like database failure

#### Recommendation

Use `context.Background()` for cleanup and completion operations that must succeed regardless of scan status:

```go
func (uc *ScanLibraryUseCase) runScan(ctx context.Context, jobID int64, lib *library.Library) {
    // ... existing scan logic ...

    scanErr := coordinator.Scan(ctx, lib.Path, resultChan)
    close(resultChan)

    <-processDone
    <-foundFilesCollectorDone

    // Create a fresh context for cleanup operations
    // These MUST complete even if scan context was cancelled
    cleanupCtx := context.Background()

    // Cleanup with timeout to prevent indefinite blocking
    cleanupDeadline := time.Now().Add(5 * time.Minute)
    cleanupCtx, cleanupCancel := context.WithDeadline(cleanupCtx, cleanupDeadline)
    defer cleanupCancel()

    if scanErr == nil && uc.imageRepo != nil && uc.imageCleanup != nil {
        uc.cleanupStaleMedia(cleanupCtx, lib.ID, foundFiles)
    }

    // ... status update logic ...

    // Final completion MUST succeed
    if err := uc.scanJobRepo.Complete(cleanupCtx, job); err != nil {
        uc.logger.Error("Failed to complete scan job",
            "job_id", jobID,
            "error", err)
    }
}
```

**Rationale**: Cleanup and job completion are **critical operations** that must complete even if the scan itself was cancelled. Using a cancelled context causes these operations to fail immediately.

---

## HIGH SEVERITY ISSUES

### Issue 3: Shared State Data Race in Artist Processing

**Category**: Concurrency & Synchronization
**Severity**: HIGH
**File**: `scan_library.go:39-42, 162-164, 660-664, 692-696, 805-816`

#### Description
The `processedArtists` map is used as **per-scan session state** but is shared across ALL scans via the use case struct. This creates **data races** when multiple scans run concurrently (different libraries) and can cause **incorrect artist image extraction**.

#### Evidence
```go
// Line 39-42: Struct field (shared across all scans)
type ScanLibraryUseCase struct {
    // ... other fields ...
    processedArtists   map[string]bool      // SHARED STATE
    processedArtistsMu sync.Mutex
}

// Line 162-164: Reset per scan
func (uc *ScanLibraryUseCase) runScan(ctx context.Context, jobID int64, lib *library.Library) {
    uc.processedArtistsMu.Lock()
    uc.processedArtists = make(map[string]bool)  // OVERWRITES previous scan data
    uc.processedArtistsMu.Unlock()
    // ...
}

// Line 660-664: Concurrent access from multiple goroutines
if !uc.isArtistProcessed(track.Artist) {
    if err := uc.extractArtistImages.Execute(ctx, artistDir, images.MediaTypeMusicArtist, entityID); err != nil {
        fmt.Printf("failed to extract artist images for %s: %v\n", track.Artist, err)
    }
    uc.markArtistProcessed(track.Artist)
}
```

#### Impact

**Scenario**: Two libraries scanning simultaneously:
1. **Library A** (Music) starts scanning at t=0
2. **Library B** (Music) starts scanning at t=5s
3. At t=5s, `runScan` for Library B **resets** the `processedArtists` map
4. Library A's tracking is **wiped out**, causing **duplicate artist image extraction**
5. **Race condition** between goroutines in Library A checking/updating the map

**Consequences**:
- Duplicate artist image processing (performance hit)
- Potential file system race conditions
- Map corruption due to concurrent read/write without proper locking
- Go race detector would flag this immediately

#### Recommendation

**Solution 1 - Per-Scan State** (Recommended):
```go
// Remove from struct
type ScanLibraryUseCase struct {
    // Remove these fields:
    // processedArtists   map[string]bool
    // processedArtistsMu sync.Mutex

    // ... other fields remain ...
}

// Pass as parameter through call chain
func (uc *ScanLibraryUseCase) runScan(ctx context.Context, jobID int64, lib *library.Library) {
    // Create per-scan state
    scanState := &ScanState{
        processedArtists: make(map[string]bool),
        mu:              sync.Mutex{},
    }

    // ... existing code ...

    // Pass to processor
    go func() {
        defer close(processDone)
        uc.processResults(ctx, jobID, lib.ID, lib.Type, resultChan, foundFilePaths, scanState)
    }()
}

type ScanState struct {
    processedArtists map[string]bool
    mu              sync.Mutex
}

func (uc *ScanLibraryUseCase) processResults(
    ctx context.Context,
    jobID int64,
    libraryID int64,
    libraryType library.LibraryType,
    resultChan <-chan scanner.ScanResult,
    foundFilePaths chan<- string,
    scanState *ScanState,  // Pass state explicitly
) {
    // ... existing code ...
    uc.processMusicTrack(ctx, libraryID, &result, scanState)
}

func (uc *ScanLibraryUseCase) processMusicTrack(
    ctx context.Context,
    libraryID int64,
    result *scanner.ScanResult,
    scanState *ScanState,
) {
    // ... existing code ...

    if !scanState.isArtistProcessed(track.Artist) {
        // ... extract images ...
        scanState.markArtistProcessed(track.Artist)
    }
}

func (s *ScanState) isArtistProcessed(artistName string) bool {
    s.mu.Lock()
    defer s.mu.Unlock()
    return s.processedArtists[artistName]
}

func (s *ScanState) markArtistProcessed(artistName string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.processedArtists[artistName] = true
}
```

**Solution 2 - Sync.Map** (If keeping shared state):
```go
type ScanLibraryUseCase struct {
    // Use sync.Map which is safe for concurrent use
    processedArtists sync.Map // map[int64]map[string]bool - jobID -> artists
}

func (uc *ScanLibraryUseCase) runScan(ctx context.Context, jobID int64, lib *library.Library) {
    // Store per-job tracking
    uc.processedArtists.Store(jobID, &sync.Map{})
    defer uc.processedArtists.Delete(jobID)

    // ... rest of scan ...
}
```

---

### Issue 4: Unsafe Channel Closing Pattern

**Category**: Concurrency & Synchronization
**Severity**: HIGH
**File**: `scan_library.go:169-187, 272`

#### Description
The `foundFilePaths` channel is closed in `processResults` (line 272) but sent to from within the same function. This creates a **close-before-done** pattern where the channel is closed while the collector goroutine may still be reading from it.

#### Evidence
```go
// Line 169-187
foundFilePaths := make(chan string)
foundFiles := make(map[string]bool)
foundFilesMu := sync.Mutex{}

// Goroutine to collect found file paths
foundFilesCollectorDone := make(chan struct{})
go func() {
    defer close(foundFilesCollectorDone)
    for filePath := range foundFilePaths {  // Reads until channel closed
        foundFilesMu.Lock()
        foundFiles[filePath] = true
        foundFilesMu.Unlock()
    }
}()

// Line 189-193
processDone := make(chan struct{})
go func() {
    defer close(processDone)
    uc.processResults(ctx, jobID, lib.ID, lib.Type, resultChan, foundFilePaths)
}()

// Line 269-272 (in processResults)
func (uc *ScanLibraryUseCase) processResults(..., foundFilePaths chan<- string) {
    // ...
    defer close(foundFilePaths)  // Closes the channel

    for result := range resultChan {
        // ...
        foundFilePaths <- result.FilePath  // Sends to channel
    }
}
```

#### Impact

**Current behavior is actually correct but fragile**:
- The defer ensures `foundFilePaths` is closed AFTER all sends complete
- However, the ownership model is unclear: who is responsible for closing?
- Future refactoring could easily introduce a bug (e.g., early return before defer)

**Risk**: If someone adds an early return path in `processResults` before all results are processed:
```go
for result := range resultChan {
    if someCondition {
        return  // BUG: foundFilePaths not closed, collector goroutine leaks
    }
    foundFilePaths <- result.FilePath
}
```

#### Recommendation

**Follow Go Best Practice - Writer Closes**:
```go
func (uc *ScanLibraryUseCase) runScan(ctx context.Context, jobID int64, lib *library.Library) {
    // ... existing code ...

    foundFilePaths := make(chan string, 100)  // Buffer to reduce blocking
    foundFiles := make(map[string]bool)
    foundFilesMu := sync.Mutex{}

    // Collector goroutine
    foundFilesCollectorDone := make(chan struct{})
    go func() {
        defer close(foundFilesCollectorDone)
        for filePath := range foundFilePaths {
            foundFilesMu.Lock()
            foundFiles[filePath] = true
            foundFilesMu.Unlock()
        }
    }()

    // Result processor
    processDone := make(chan struct{})
    go func() {
        defer close(processDone)
        defer close(foundFilePaths)  // MOVE HERE: processResults owns the channel
        uc.processResults(ctx, jobID, lib.ID, lib.Type, resultChan, foundFilePaths)
    }()

    // ... rest of function ...
}

// Remove defer from processResults
func (uc *ScanLibraryUseCase) processResults(
    ctx context.Context,
    jobID int64,
    libraryID int64,
    libraryType library.LibraryType,
    resultChan <-chan scanner.ScanResult,
    foundFilePaths chan<- string,
) {
    // REMOVE: defer close(foundFilePaths)

    updateTicker := time.NewTicker(2 * time.Second)
    defer updateTicker.Stop()

    for result := range resultChan {
        // ... existing logic ...
        foundFilePaths <- result.FilePath
    }

    // Final progress update
    // ... existing code ...
}
```

**Rationale**: The goroutine that creates and **writes** to a channel should be responsible for closing it. This makes ownership clear and prevents accidental leaks.

---

### Issue 5: Potential Mutex Lock Inversion / Deadlock

**Category**: Resource Management
**Severity**: HIGH
**File**: `coordinator.go:257-262, 344-357, 378-381, 447-452`

#### Description
The `Coordinator` uses `sync.RWMutex` (`mu`) to protect multiple pieces of state (`sizeMap`, `FileCache`, `isRunning`, `startTime`). However, lock acquisition order is **inconsistent** across methods, creating potential for deadlock.

#### Evidence
```go
// Pattern 1: RLock for read (processFile:259-262)
if c.config.EnableIncrementalScan {
    c.mu.RLock()
    cached, exists := c.config.FileCache[fileInfo.Path]
    c.mu.RUnlock()
    // ... use cached ...
}

// Pattern 2: Lock after read (updateFileCache:378-381)
c.mu.Lock()
c.config.FileCache[fileInfo.Path] = entry
c.mu.Unlock()

// Pattern 3: Lock for write to sizeMap (recordFileSize:448-452)
c.mu.Lock()
c.sizeMap[size]++
c.mu.Unlock()

// Pattern 4: RLock then check sizeMap (shouldHashFile:435-437)
c.mu.RLock()
count := c.sizeMap[size]
c.mu.RUnlock()
```

**Problem**: In `processFile`, the following sequence can occur:
1. Line 256: `c.recordFileSize(fileInfo.Size)` - acquires **Lock**
2. Line 259-262: `c.mu.RLock()` - attempts **RLock** while same goroutine holds Lock

While this specific case **doesn't deadlock** (single goroutine), it's a **code smell** indicating lock granularity issues.

**Real Deadlock Scenario** (with concurrent coordinator instances):
- Thread A: Holds `Lock` in `recordFileSize`, waiting on RLock in `processFile`
- Thread B: Holds `RLock` in `shouldHashFile`, waiting for Lock in `recordFileSize`
- **DEADLOCK**

#### Impact
- Potential for deadlock under concurrent access (though coordinator is per-scan)
- Poor lock granularity - entire config and multiple maps protected by one mutex
- Performance bottleneck - unnecessary lock contention

#### Recommendation

**Solution - Separate Locks for Independent State**:
```go
type Coordinator struct {
    config       CoordinatorConfig
    // ... other fields ...

    // Separate mutexes for independent state
    progressMu   sync.RWMutex  // Protects startTime, isRunning
    sizeMapMu    sync.RWMutex  // Protects sizeMap
    cacheMu      sync.RWMutex  // Protects FileCache
}

func (c *Coordinator) recordFileSize(size int64) {
    c.sizeMapMu.Lock()
    c.sizeMap[size]++
    c.sizeMapMu.Unlock()
}

func (c *Coordinator) shouldHashFile(size int64) bool {
    // ... strategy checks ...

    c.sizeMapMu.RLock()
    count := c.sizeMap[size]
    c.sizeMapMu.RUnlock()

    return count > 0
}

func (c *Coordinator) updateFileCache(fileInfo FileInfo, result *scanner.ScanResult) {
    // ... build entry ...

    c.cacheMu.Lock()
    c.config.FileCache[fileInfo.Path] = entry
    c.cacheMu.Unlock()
}

func (c *Coordinator) IsRunning() bool {
    c.progressMu.RLock()
    defer c.progressMu.RUnlock()
    return c.isRunning
}
```

**Benefits**:
- Eliminates lock inversion potential
- Reduces lock contention (independent state can be accessed concurrently)
- Clearer lock ownership model

---

### Issue 6: State Machine Violation - Missing Status Validation

**Category**: Code Organization
**Severity**: HIGH
**File**: `scan_library.go:119-128, 214-229`

#### Description
The scan job status transitions are **not validated**, allowing invalid state transitions like `completed → running` or `failed → completed`. The code checks for running scans before starting but doesn't validate other state transitions.

#### Evidence
```go
// Line 119-128: Only checks for running scans
running, err := uc.scanJobRepo.ListRunning(ctx)
if err != nil {
    return StartScanResponse{}, fmt.Errorf("failed to check running scans: %w", err)
}
for _, job := range running {
    if job.LibraryID == libraryID {
        return StartScanResponse{}, scanner.ErrAlreadyRunning
    }
}

// Line 224-229: Direct status assignment without validation
if scanErr != nil {
    job.Status = scanner.ScanStatusFailed
    job.ErrorMessage = scanErr.Error()
} else {
    job.Status = scanner.ScanStatusCompleted
}
```

**Missing Validations**:
- Can't transition from `completed` to `running` (re-running scan)
- Can't transition from `failed` to `running` (retry scan)
- No check if job is already `completed` or `failed`

#### Impact
- **Data Integrity**: Job status can become inconsistent
- **Concurrent Scans**: Two scans could start for same library if timing is unlucky
- **Audit Trail**: No validation of what transitions are legal
- **Testing Difficulty**: State machine behavior not explicit

#### Recommendation

**Implement Explicit State Machine**:
```go
// In domain/scanner/types.go
type ScanStatus string

const (
    ScanStatusPending   ScanStatus = "pending"
    ScanStatusRunning   ScanStatus = "running"
    ScanStatusCompleted ScanStatus = "completed"
    ScanStatusFailed    ScanStatus = "failed"
)

// ValidateTransition checks if a status transition is allowed
func (s ScanStatus) ValidateTransition(to ScanStatus) error {
    validTransitions := map[ScanStatus][]ScanStatus{
        ScanStatusPending: {ScanStatusRunning},
        ScanStatusRunning: {ScanStatusCompleted, ScanStatusFailed},
        ScanStatusCompleted: {},  // Terminal state
        ScanStatusFailed: {},     // Terminal state
    }

    allowed, exists := validTransitions[s]
    if !exists {
        return fmt.Errorf("unknown status: %s", s)
    }

    for _, valid := range allowed {
        if valid == to {
            return nil
        }
    }

    return fmt.Errorf("invalid status transition: %s -> %s", s, to)
}

// Usage in StartScan
func (uc *ScanLibraryUseCase) StartScan(ctx context.Context, libraryID int64) (StartScanResponse, error) {
    // ... existing validation ...

    // Check for ANY existing scan (not just running)
    existing, err := uc.scanJobRepo.GetLatestByLibrary(ctx, libraryID)
    if err != nil && !errors.Is(err, scanner.ErrNotFound) {
        return StartScanResponse{}, fmt.Errorf("failed to check existing scans: %w", err)
    }

    // Validate we can start a new scan
    if existing != nil {
        if existing.Status == scanner.ScanStatusRunning {
            return StartScanResponse{}, scanner.ErrAlreadyRunning
        }
        // Completed/Failed scans are OK - this is a new scan
    }

    job := &scanner.ScanJob{
        LibraryID: libraryID,
        Status:    scanner.ScanStatusPending,  // Start as pending
        // ... other fields ...
    }

    if err := uc.scanJobRepo.Create(ctx, job); err != nil {
        return StartScanResponse{}, fmt.Errorf("failed to create scan job: %w", err)
    }

    // Transition to running
    if err := job.Status.ValidateTransition(scanner.ScanStatusRunning); err != nil {
        return StartScanResponse{}, fmt.Errorf("invalid state transition: %w", err)
    }
    job.Status = scanner.ScanStatusRunning

    // Update status in DB
    if err := uc.scanJobRepo.UpdateStatus(ctx, job.ID, scanner.ScanStatusRunning); err != nil {
        return StartScanResponse{}, fmt.Errorf("failed to update status: %w", err)
    }

    // ... start background scan ...
}
```

---

## MEDIUM SEVERITY ISSUES

### Issue 7: Poor Interface Design - Overly Specific Executors

**Category**: Code Organization
**Severity**: MEDIUM
**File**: `scan_library.go:44-72`

#### Description
The use case has **6 separate image extraction executors** with near-identical signatures, violating DRY and making testing cumbersome. Each executor interface is defined specifically for one entity type.

#### Evidence
```go
// Lines 44-72: Six separate executor interfaces
type ExtractMovieImagesExecutor interface {
    Execute(ctx context.Context, movieFilePath string, mediaType images.MediaType, entityID int, mediaID *int) error
}

type ExtractTVEpisodeImagesExecutor interface {
    Execute(ctx context.Context, episodeFilePath string, mediaType images.MediaType, entityID int, mediaID *int) error
}

type ExtractTVShowImagesExecutor interface {
    Execute(ctx context.Context, showDir string, mediaType images.MediaType, entityID int) error
}

type ExtractTVSeasonImagesExecutor interface {
    Execute(ctx context.Context, showDir string, seasonNumber int, mediaType images.MediaType, entityID int) error
}

type ExtractMusicAlbumImagesExecutor interface {
    Execute(ctx context.Context, albumDir string, mediaType images.MediaType, entityID int) error
}

type ExtractMusicArtistImagesExecutor interface {
    Execute(ctx context.Context, artistDir string, mediaType images.MediaType, entityID int) error
}
```

#### Impact
- **Testing Burden**: Must mock 6 interfaces instead of 1
- **Maintenance**: Changes to signature require updating 6 interfaces
- **Dependency Injection**: Constructor takes 6 parameters instead of 1
- **Type Safety Illusion**: mediaType parameter is redundant (executor type implies it)

#### Recommendation

**Solution 1 - Unified Interface with Strategy Pattern**:
```go
// Define a single, flexible interface
type ImageExtractor interface {
    ExtractImages(ctx context.Context, req ImageExtractionRequest) error
}

type ImageExtractionRequest struct {
    SourcePath   string             // File or directory path
    MediaType    images.MediaType   // Type of media
    EntityID     int                // Database entity ID
    MediaID      *int               // Optional media ID (for movies/episodes)
    SeasonNumber *int               // Optional season number (for TV seasons)
}

// Use case simplified
type ScanLibraryUseCase struct {
    // ... other fields ...
    imageExtractor ImageExtractor  // Single dependency
    imageRepo      images.Repository
    imageCleanup   ImageCleanupExecutor
}

// Usage
func (uc *ScanLibraryUseCase) processMovie(ctx context.Context, libraryID int64, result *scanner.ScanResult) {
    // ... existing movie creation logic ...

    if uc.imageExtractor != nil {
        mediaID := int(movie.Media.ID)
        req := ImageExtractionRequest{
            SourcePath: result.FilePath,
            MediaType:  images.MediaTypeMovie,
            EntityID:   mediaID,
            MediaID:    &mediaID,
        }
        if err := uc.imageExtractor.ExtractImages(ctx, req); err != nil {
            uc.logger.Error("Failed to extract images",
                "path", result.FilePath,
                "media_type", images.MediaTypeMovie,
                "error", err)
        }
    }
}
```

**Solution 2 - Generic Interface (Go 1.18+)**:
```go
type ImageExtractor[T any] interface {
    Extract(ctx context.Context, source T) error
}

// Concrete request types
type MovieImageRequest struct {
    FilePath string
    MediaID  int
}

type TVShowImageRequest struct {
    ShowDir string
    ShowID  int
}

// Use case
type ScanLibraryUseCase struct {
    movieImageExtractor ImageExtractor[MovieImageRequest]
    tvImageExtractor    ImageExtractor[TVShowImageRequest]
    // Still fewer than 6!
}
```

**Recommendation**: Use Solution 1. It provides flexibility without generics complexity.

---

### Issue 8: Improper Error Wrapping Loses Context

**Category**: Error Handling
**Severity**: MEDIUM
**File**: Multiple locations in `scan_library.go`

#### Description
Error wrapping is **inconsistent** throughout the scanner. Some errors use `fmt.Errorf` with `%w`, others don't wrap at all, and many printed errors don't propagate up the call stack.

#### Evidence
```go
// Good wrapping (line 116)
return StartScanResponse{}, fmt.Errorf("failed to get library: %w", err)

// Bad: No wrapping, error lost (line 389-391)
if err := uc.mediaRepo.Update(ctx, &movie.Media); err != nil {
    fmt.Printf("failed to update media %s: %v\n", result.FilePath, err)
}
// Execution continues, error not propagated

// Bad: No wrapping (line 408)
fmt.Printf("failed to create movie %s: %v\n", result.FilePath, err)
return  // Error context lost
```

#### Impact
- **Debugging Difficulty**: Stack trace lost, can't determine error origin
- **Error Handling**: Caller can't inspect error type with `errors.Is` or `errors.As`
- **Observability**: Logs lack context about where error occurred
- **Silent Failures**: Errors printed but not counted or propagated

#### Recommendation

**Consistent Error Wrapping Strategy**:
```go
// Define domain errors
package scanner

var (
    ErrMediaCreation    = errors.New("media creation failed")
    ErrMetadataExtract  = errors.New("metadata extraction failed")
    ErrImageExtraction  = errors.New("image extraction failed")
)

// Wrap all errors consistently
func (uc *ScanLibraryUseCase) processMovie(
    ctx context.Context,
    libraryID int64,
    result *scanner.ScanResult,
) error {  // Change signature to return error
    // ... build movie struct ...

    // Check if movie already exists
    existing, err := uc.mediaRepo.GetByFilePath(ctx, libraryID, result.FilePath)
    if err == nil && existing != nil {
        // Update existing entry
        movie.Media.ID = existing.ID
        movie.Media.Type = "movie"

        if err := uc.mediaRepo.Update(ctx, &movie.Media); err != nil {
            return fmt.Errorf("%w: %s: %w",
                scanner.ErrMediaCreation,
                "update media",
                err)
        }

        if err := uc.movieRepo.UpdateMovie(ctx, movie); err != nil {
            return fmt.Errorf("%w: %s: %w",
                scanner.ErrMediaCreation,
                "update movie metadata",
                err)
        }

        // Extract images
        if uc.extractMovieImages != nil {
            mediaID := int(movie.Media.ID)
            if err := uc.extractMovieImages.Execute(ctx, result.FilePath, images.MediaTypeMovie, mediaID, &mediaID); err != nil {
                // Image extraction failure is non-fatal
                uc.logger.Warn("Image extraction failed",
                    "path", result.FilePath,
                    "error", err)
            }
        }
        return nil
    }

    // Create new entry
    movie.Media.Type = "movie"
    if err := uc.movieRepo.CreateMovie(ctx, movie); err != nil {
        return fmt.Errorf("%w: %s: %w",
            scanner.ErrMediaCreation,
            "create movie",
            err)
    }

    // Extract images (non-fatal)
    if uc.extractMovieImages != nil {
        mediaID := int(movie.Media.ID)
        if err := uc.extractMovieImages.Execute(ctx, result.FilePath, images.MediaTypeMovie, mediaID, &mediaID); err != nil {
            uc.logger.Warn("Image extraction failed",
                "path", result.FilePath,
                "media_id", mediaID,
                "error", err)
        }
    }

    return nil
}

// Caller handles error
func (uc *ScanLibraryUseCase) processResults(...) {
    for result := range resultChan {
        // ... progress tracking ...

        var err error
        switch libraryType {
        case library.LibraryTypeMovies:
            err = uc.processMovie(ctx, libraryID, &result)
        case library.LibraryTypeTV:
            err = uc.processTVEpisode(ctx, libraryID, &result)
        case library.LibraryTypeMusic:
            err = uc.processMusicTrack(ctx, libraryID, &result)
        }

        if err != nil {
            // Categorize and log
            category := categorizeError(err)
            uc.logger.Error("Failed to process media",
                "path", result.FilePath,
                "library_type", libraryType,
                "category", category,
                "error", err)

            // Track error
            progress.ErrorCount++

            // Persist error (Phase 1.5 from ADR 014)
            scanError := &scanner.ScanError{
                ScanJobID:     jobID,
                FilePath:      result.FilePath,
                ErrorMessage:  err.Error(),
                ErrorCategory: category,
            }
            _ = uc.scanJobRepo.CreateError(ctx, scanError)
        }
    }
}
```

---

### Issue 9: Defer Statement Placement Issues

**Category**: Go Best Practices
**Severity**: MEDIUM
**File**: `scan_library.go:152-154`, `coordinator.go:106-110`

#### Description
The `defer cancel()` in `StartScan` is inside a goroutine, which is correct, but the **timeout context** is created in the parent function and passed to an unbounded goroutine. If the parent returns quickly (which it does), the timeout effectively becomes the **only** lifecycle management.

Additionally, in `coordinator.go`, the defer unlock pattern has a **TOCTOU race**.

#### Evidence
```go
// scan_library.go:150-154
scanCtx, cancel := context.WithTimeout(context.Background(), uc.scanTimeout)
go func() {
    defer cancel()  // Correct placement
    uc.runScan(scanCtx, job.ID, lib)
}()

return ToStartScanResponse(job), nil  // Returns immediately, goroutine continues

// coordinator.go:95-110
func (c *Coordinator) Scan(...) error {
    c.mu.Lock()
    if c.isRunning {
        c.mu.Unlock()
        return scanner.ErrAlreadyRunning
    }
    c.isRunning = true
    c.startTime = time.Now()
    c.resetCounters()
    c.mu.Unlock()  // PROBLEM: Gap between unlock and defer

    defer func() {
        c.mu.Lock()
        c.isRunning = false
        c.mu.Unlock()
    }()

    // ... scan logic ...
}
```

**Problem in Coordinator**: Between lines 103 (unlock) and 106 (defer), if the function panics, `isRunning` is never reset to `false`, permanently blocking future scans.

#### Impact
- **Coordinator**: Panic before defer = permanent "scan running" state
- **StartScan**: Timeout is the only safety mechanism (intended, but not documented)

#### Recommendation

**Fix Coordinator Defer Order**:
```go
func (c *Coordinator) Scan(ctx context.Context, libraryPath string, resultChan chan<- scanner.ScanResult) error {
    // Check and set running state atomically
    c.mu.Lock()
    if c.isRunning {
        c.mu.Unlock()
        return scanner.ErrAlreadyRunning
    }
    c.isRunning = true
    c.startTime = time.Now()
    c.resetCounters()
    c.mu.Unlock()

    // Ensure cleanup happens IMMEDIATELY after setting running state
    defer func() {
        c.mu.Lock()
        c.isRunning = false
        c.mu.Unlock()
    }()

    // Now safe to proceed with scan logic
    fileChan := make(chan scanner.FileInfo, c.config.ResultBufferSize)
    discoveryErrChan := make(chan error, 1)

    go c.discoverFiles(ctx, libraryPath, fileChan, discoveryErrChan)

    // ... rest of function ...
}
```

**Document StartScan Timeout Behavior**:
```go
// StartScan initiates a new scan for a library.
//
// The scan runs asynchronously in a background goroutine with a configurable timeout.
// The returned response contains the scan job ID for progress tracking.
//
// Context Lifecycle:
//   - The provided ctx is used only for the initial database operations (validation, job creation)
//   - The background scan uses context.Background() with the configured scanTimeout
//   - This ensures scans complete even if the HTTP request context is cancelled
//
// Error Handling:
//   - Returns error if library doesn't exist or validation fails
//   - Returns scanner.ErrAlreadyRunning if a scan is already in progress for this library
//   - Background scan errors are recorded in the scan job status (not returned here)
func (uc *ScanLibraryUseCase) StartScan(ctx context.Context, libraryID int64) (StartScanResponse, error) {
    // ... implementation ...
}
```

---

### Issue 10: Inefficient Memory Usage in foundFiles Map

**Category**: Performance & Scalability
**Severity**: MEDIUM
**File**: `scan_library.go:174-187, 735-770`

#### Description
The `foundFiles` map stores **every file path** as a string key with a boolean value, consuming significant memory for large libraries. For a library with 50,000 files averaging 100 bytes per path, this is **~5MB** just for tracking. The map is used only for **membership testing** in `cleanupStaleMedia`.

#### Evidence
```go
// Line 174-187: Build map of all found files
foundFilePaths := make(chan string)
foundFiles := make(map[string]bool)  // Stores every file path
foundFilesMu := sync.Mutex{}

go func() {
    defer close(foundFilesCollectorDone)
    for filePath := range foundFilePaths {
        foundFilesMu.Lock()
        foundFiles[filePath] = true  // Every file stored
        foundFilesMu.Unlock()
    }
}()

// Line 747-749: Used only for membership test
for _, m := range allMedia {
    if !foundFiles[m.FilePath] {
        // Delete stale media
    }
}
```

#### Impact
- **Memory Overhead**: 5-10MB for large libraries
- **GC Pressure**: Large map allocations
- **Unnecessary**: Boolean value is redundant (existence in map is sufficient)
- **Optimization Opportunity**: Could use more memory-efficient structure

#### Recommendation

**Solution 1 - Use Empty Struct (Zero Memory)**:
```go
// Line 174-176
foundFiles := make(map[string]struct{})  // struct{} has zero size
foundFilesMu := sync.Mutex{}

// Collector
go func() {
    defer close(foundFilesCollectorDone)
    for filePath := range foundFilePaths {
        foundFilesMu.Lock()
        foundFiles[filePath] = struct{}{}  // Zero memory value
        foundFilesMu.Unlock()
    }
}()

// Usage
if _, found := foundFiles[m.FilePath]; !found {
    // Delete stale media
}
```

**Solution 2 - Use sync.Map (Concurrent)**:
```go
// Line 174
foundFiles := &sync.Map{}

// Collector (can be concurrent now)
go func() {
    defer close(foundFilesCollectorDone)
    for filePath := range foundFilePaths {
        foundFiles.Store(filePath, struct{}{})  // No lock needed
    }
}()

// Usage
if _, found := foundFiles.Load(m.FilePath); !found {
    // Delete stale media
}
```

**Solution 3 - Bloom Filter (For Very Large Libraries)**:
For 100,000+ files, consider a bloom filter:
```go
import "github.com/bits-and-blooms/bloom/v3"

// Line 174 - Estimate filter size
estimatedFiles := 100000
filter := bloom.NewWithEstimates(uint(estimatedFiles), 0.01)  // 1% false positive rate

// Collector
go func() {
    defer close(foundFilesCollectorDone)
    for filePath := range foundFilePaths {
        filter.AddString(filePath)
    }
}()

// Usage - Note: Bloom filter has false positives
if !filter.TestString(m.FilePath) {
    // Definitely not found - safe to delete
    // (False positives mean we might NOT delete some stale files)
}
```

**Recommendation**: Use Solution 1 (`map[string]struct{}`) for simplicity and immediate benefit. Consider Solution 3 only if library sizes exceed 100,000 files.

---

## Additional Observations (Non-Issues)

### Good Practices Observed

1. **Buffered Channels** (line 170, 114): Proper buffering on `resultChan` and `fileChan` reduces blocking
2. **Timeout Protection** (line 150): Configurable scan timeout prevents indefinite operations
3. **Graceful Error Handling** (lines 285-288): Individual file errors don't stop entire scan
4. **Progress Tracking** (lines 270-318): Real-time progress updates every 2 seconds
5. **Cleanup Logic** (lines 206-208): Stale media cleanup only runs on successful scans

### Minor Suggestions

1. **Magic Numbers**: Buffer size of 100 (line 170) should be a constant
2. **String Literals**: Media types "movie", "tv_episode" should use constants
3. **Ticker Resource Leak**: Line 270 ticker is properly stopped with defer
4. **Error Messages**: Line 234 `fmt.Printf` should use structured logging (covered by ADR 014)

---

## Prioritized Remediation Plan

### Immediate (Critical)
**Timeline**: 1-2 days

1. **Fix Context Lifetime Issue (#2)**
   - Impact: Prevents silent failures in cleanup
   - Effort: 2 hours
   - Risk: Low (isolated change)

2. **Fix Goroutine Leak (#1)**
   - Impact: Prevents resource leaks
   - Effort: 3 hours (includes panic recovery)
   - Risk: Low (defensive programming)

### Short Term (High)
**Timeline**: 1 week

3. **Fix Artist State Race Condition (#3)**
   - Impact: Prevents data races and duplicate processing
   - Effort: 4 hours (refactor to per-scan state)
   - Risk: Medium (requires testing across scan types)

4. **Improve Channel Ownership (#4)**
   - Impact: Clarifies ownership, prevents future bugs
   - Effort: 2 hours
   - Risk: Low (code reorganization)

5. **Add State Machine Validation (#6)**
   - Impact: Prevents invalid state transitions
   - Effort: 4 hours
   - Risk: Low (new validation layer)

### Medium Term (Medium)
**Timeline**: 2-3 weeks

6. **Refactor Interface Design (#7)**
   - Impact: Reduces testing burden, improves maintainability
   - Effort: 1 day (includes updating tests)
   - Risk: Medium (breaking change for dependents)

7. **Improve Error Wrapping (#8)**
   - Impact: Better debugging and observability
   - Effort: 1 day (systematic refactor)
   - Risk: Low (backward compatible)

8. **Fix Defer Placement (#9)**
   - Impact: Prevents permanent lock states
   - Effort: 2 hours
   - Risk: Low (defensive fix)

### Low Priority (Medium)
**Timeline**: Future iteration

9. **Separate Mutex Locks (#5)**
   - Impact: Prevents potential deadlock, improves performance
   - Effort: 4 hours
   - Risk: Medium (requires careful lock analysis)

10. **Optimize Memory Usage (#10)**
    - Impact: Reduces memory footprint by ~50%
    - Effort: 1 hour
    - Risk: Very Low (simple substitution)

---

## Testing Recommendations

### Critical Issues
- **Goroutine Leak**: Use `goleak` package to detect leaks in tests
- **Context Cancellation**: Test cleanup behavior when scan context expires
- **Race Conditions**: Run with `-race` flag: `go test -race ./internal/application/library/...`

### Integration Tests Needed
```go
func TestScanLibrary_ConcurrentScans_NoRaceCondition(t *testing.T) {
    // Start 2 music library scans simultaneously
    // Verify no data races in artist processing
}

func TestScanLibrary_ContextCancellation_CleansUp(t *testing.T) {
    // Cancel context mid-scan
    // Verify job status updated correctly
    // Verify goroutines cleaned up (use goleak)
}

func TestScanLibrary_StateTransitions_Validated(t *testing.T) {
    // Attempt invalid transitions (completed -> running)
    // Verify error returned
}
```

---

## Conclusion

The library scanner has a **solid foundation** with good separation of concerns and graceful error handling. However, it suffers from **concurrency issues** that could manifest as production bugs under load:

**Must Fix**:
- Context lifetime violations causing silent failures
- Goroutine leaks on error paths
- Data races in artist processing

**Should Fix**:
- State machine validation
- Interface design simplification
- Error wrapping consistency

**Nice to Have**:
- Memory optimizations
- Lock granularity improvements

The good news: **All issues are fixable** without major architectural changes. The prioritized plan above provides a roadmap for systematic improvement.

---

**Reviewed By**: Senior Backend Engineer
**Review Date**: 2025-11-19
**Files**: `/home/fictional/Projects/viewra2/ARCHITECTURAL_REVIEW_SCAN_LIBRARY.md`
