# ADR 014: Library Scanner Resilience Improvements

**Status**: Proposed
**Date**: 2025-11-18
**Author**: ViewRA Team
**Context**: Phase 5+ optimization and reliability improvements

## Context

The library scanner (`internal/application/library/scan_library.go`) is a critical component that processes media libraries (movies, TV shows, music). Analysis shows it's reasonably resilient for normal operations but has several opportunities for improvement in edge cases and operational visibility.

### Current Architecture

**Strengths**:
- ✅ Graceful error handling (individual file errors don't stop scan)
- ✅ Timeout protection via configurable scan timeout
- ✅ Background processing using goroutines
- ✅ Duplicate scan prevention (checks for running scans)
- ✅ Real-time progress tracking with periodic updates
- ✅ Deduplication checks for existing media
- ✅ Stale media cleanup after scan completes
- ✅ Stuck scan recovery on startup (see `scanjob/recovery.go`)

**Processing Flow**:
```
StartScan() → runScan() → processResults() → [processMovie|processTVEpisode|processMusicTrack]
     ↓
   Creates scan job
     ↓
   Background goroutine with timeout
     ↓
   Coordinator.Scan() → results channel (buffered 100)
     ↓
   Single-threaded result processor
     ↓
   Individual DB operations per media item
     ↓
   Progress updates every 2 seconds
     ↓
   Stale media cleanup
```

### Identified Issues

#### 1. **Logging Quality** (Priority: HIGH)
- Uses `fmt.Printf()` instead of structured logging (`slog`)
- No error context (file path, error type, operation)
- Critical errors logged but not aggregated for visibility
- No distinction between recoverable and fatal errors

**Current**:
```go
fmt.Printf("failed to create movie %s: %v\n", result.FilePath, err)
```

**Codebase Standard** (from existing `slog` usage):
```go
logger.Error("Failed to create movie",
    "path", result.FilePath,
    "library_id", libraryID,
    "error", err)
```

#### 2. **No Retry Logic** (Priority: MEDIUM)
- Database transient errors fail immediately
- No backoff for temporary filesystem issues
- One-shot operations without resilience

**Common Transient Errors**:
- `database is locked` (SQLite under concurrent access)
- Temporary filesystem I/O errors
- Network filesystem hiccups
- Race conditions during concurrent operations

#### 3. **Limited Error Tracking** (Priority: MEDIUM)
- Error count tracked but no categorization
- No visibility into error patterns (all DB errors? all parsing errors?)
- Can't distinguish between "scan completed with minor issues" vs "major failures"

#### 4. **No Partial Failure Recovery** (Priority: LOW)
- If server crashes mid-scan, no checkpointing
- Stuck scan recovery marks as "failed" - requires full rescan
- Large libraries waste work on crashes

#### 5. **Single-Threaded Result Processing** (Priority: LOW)
- Results channel buffered to 100 but processed sequentially
- Potential bottleneck for large libraries (10,000+ files)
- Database writes could be batched/parallelized

#### 6. **No Circuit Breaker** (Priority: LOW)
- High error rates don't trigger early termination
- Could waste resources scanning corrupted filesystem
- No feedback loop to stop futile operations

#### 7. **Inefficient Database Operations** (Priority: MEDIUM)
- Each media item = separate transaction (2-4 queries per file)
- No batch inserts/updates
- Lock contention on SQLite with concurrent writes

**Current Pattern**:
```go
for result := range resultChan {
    uc.processMovie(ctx, libraryID, &result)  // Individual DB transaction
}
```

**Impact**: For 1,000 files = 2,000+ individual DB operations

## Decision

Implement **pragmatic, phased resilience improvements** balancing robustness with complexity. Focus on high-impact, low-effort changes first.

### Guiding Principles

1. **Home Media Server Scale**: Optimize for 1,000-10,000 files, not millions
2. **User Experience First**: Better error visibility > perfect recovery
3. **Incremental Improvement**: Ship small, testable changes
4. **Preserve Simplicity**: Avoid over-engineering (no complex state machines)
5. **Structured Logging**: Use `slog` consistently with proper context

## Proposed Improvements

### Phase 0: Critical Bug Fixes (MUST DO FIRST)

**Priority**: CRITICAL - These bugs can cause data loss, crashes, or resource leaks
**Effort**: 2-3 days
**Risk**: LOW (fixes are straightforward)

#### 0.1 Fix Stale Media Cleanup Data Corruption
**Effort**: 2 hours | **Severity**: CRITICAL | **Risk**: LOW

**Problem**: Lines 731-770 - If scan partially fails (e.g., permission denied on subdirectory), cleanup deletes **valid media from database** because they weren't found in incomplete scan.

**Scenario**:
```
Library: 1000 movies
Subdirectory /movies/recent/ has permission denied
Scan processes: 900 movies (skips 100 due to permissions)
Cleanup runs: DELETES 100 VALID MOVIES FROM DATABASE
User loses access to media that still exists on disk
```

**Fix**:
```go
// Line 206-208: Only cleanup on completely successful scan
if scanErr == nil && progress.ErrorCount == 0 && progress.FilesProcessed == progress.FilesFound {
    if uc.imageRepo != nil && uc.imageCleanup != nil {
        uc.cleanupStaleMedia(ctx, lib.ID, foundFiles)
    }
}

// Inside cleanupStaleMedia: Add safety threshold
func (uc *ScanLibraryUseCase) cleanupStaleMedia(ctx context.Context, libraryID int64, foundFiles map[string]bool) {
    allMedia, err := uc.mediaRepo.ListByLibrary(ctx, libraryID)
    if err != nil {
        uc.logger.Error("Failed to list media for cleanup", "error", err)
        return
    }

    // Count stale files
    staleCount := 0
    for _, m := range allMedia {
        if !foundFiles[m.FilePath] {
            staleCount++
        }
    }

    // Safety: Don't delete if >10% of library is "stale"
    // This likely indicates scan failure, not actual deletions
    if len(allMedia) > 0 {
        stalePercent := float64(staleCount) / float64(len(allMedia)) * 100
        if stalePercent > 10.0 {
            uc.logger.Error("Refusing to cleanup - too many files marked stale",
                "stale_count", staleCount,
                "total", len(allMedia),
                "percentage", stalePercent,
                "library_id", libraryID)
            return
        }
    }

    // Safe to proceed with cleanup
    for _, m := range allMedia {
        if !foundFiles[m.FilePath] {
            // Collect hashes before deletion
            mediaHashes := CollectImageHashesForMedia(ctx, uc.imageRepo, m.ID)
            hashesToClean = append(hashesToClean, mediaHashes...)

            if err := uc.mediaRepo.Delete(ctx, m.ID); err != nil {
                uc.logger.Warn("Failed to delete stale media",
                    "media_id", m.ID,
                    "path", m.FilePath,
                    "error", err)
            } else {
                uc.logger.Info("Removed stale media",
                    "media_id", m.ID,
                    "path", m.FilePath)
            }
        }
    }
}
```

**Impact**: Prevents permanent data loss from partial scan failures.

---

#### 0.2 Fix Context Cancellation Goroutine Leak
**Effort**: 2 hours | **Severity**: CRITICAL | **Risk**: LOW

**Problem**: Line 150 - Uses `context.Background()` instead of parent context + `cancel()` never called on error paths.

**Current**:
```go
// Line 148-156
scanCtx, cancel := context.WithTimeout(context.Background(), uc.scanTimeout)
go func() {
    defer cancel()
    uc.runScan(scanCtx, job.ID, lib)
}()

return ToStartScanResponse(job), nil  // Returns immediately, cancel() not guaranteed
```

**Issues**:
1. Scan continues even if HTTP request is cancelled
2. Server shutdown hangs waiting for scan timeout
3. Every failed scan leaks a goroutine + timer

**Fix**:
```go
// Use parent context with independent timeout
func (uc *ScanLibraryUseCase) StartScan(ctx context.Context, libraryID int64) (StartScanResponse, error) {
    // ... existing validation code ...

    // Create scan job
    if err := uc.scanJobRepo.Create(ctx, job); err != nil {
        return StartScanResponse{}, fmt.Errorf("failed to create scan job: %w", err)
    }

    // Create scan context with timeout
    scanCtx, cancel := context.WithTimeout(context.Background(), uc.scanTimeout)

    // Start background scan with panic recovery
    go func() {
        defer cancel()  // Always called
        defer func() {
            if r := recover(); r != nil {
                uc.logger.Error("Scan panicked",
                    "job_id", job.ID,
                    "panic", r,
                    "stack", string(debug.Stack()))

                // Mark job as failed
                failedJob := &scanner.ScanJob{
                    ID:           job.ID,
                    Status:       scanner.ScanStatusFailed,
                    ErrorMessage: fmt.Sprintf("scan panicked: %v", r),
                }
                if err := uc.scanJobRepo.Complete(context.Background(), failedJob); err != nil {
                    uc.logger.Error("Failed to mark panicked job as failed", "error", err)
                }
            }
        }()

        uc.runScan(scanCtx, job.ID, lib)
    }()

    // Monitor parent context cancellation
    go func() {
        <-ctx.Done()
        cancel()  // Parent cancelled, stop scan
    }()

    return ToStartScanResponse(job), nil
}
```

**Impact**: Prevents goroutine leaks, enables graceful shutdown, recovers from panics.

---

#### 0.3 Fix processedArtists Race Condition
**Effort**: 1 hour | **Severity**: CRITICAL | **Risk**: LOW

**Problem**: Lines 692-696 - Check-then-act pattern on shared map is not atomic.

**Current**:
```go
// Line 692-696
if !uc.isArtistProcessed(track.Artist) {  // Thread A checks
    // Thread B also checks before A marks
    if err := uc.extractArtistImages.Execute(...); err != nil {
        // Both threads extract same artist images
    }
    uc.markArtistProcessed(track.Artist)
}
```

**Fix**:
```go
// Change struct field
type ScanLibraryUseCase struct {
    // ... other fields ...
    processedArtists sync.Map  // string -> bool (was: map[string]bool + mutex)
}

// Remove mutex
// Remove isArtistProcessed() and markArtistProcessed() methods

// Replace with atomic check-and-set
if _, loaded := uc.processedArtists.LoadOrStore(track.Artist, true); !loaded {
    // Only one goroutine gets here per artist
    if err := uc.extractArtistImages.Execute(...); err != nil {
        uc.logger.Error("Failed to extract artist images",
            "artist", track.Artist,
            "error", err)
    }
}

// Initialize in runScan
func (uc *ScanLibraryUseCase) runScan(ctx context.Context, jobID int64, lib *library.Library) {
    // Reset for this scan (sync.Map doesn't need initialization)
    uc.processedArtists = sync.Map{}

    // ... rest of function
}
```

**Impact**: Prevents duplicate artist image extraction, eliminates race condition.

---

#### 0.4 Fix Channel Deadlock on foundFilePaths
**Effort**: 1 hour | **Severity**: HIGH | **Risk**: LOW

**Problem**: Lines 174-187, 291 - Unbuffered channel can deadlock if collector goroutine exits.

**Current**:
```go
// Line 174: Unbuffered channel
foundFilePaths := make(chan string)

// Line 180-187: Collector
go func() {
    defer close(foundFilesCollectorDone)
    for filePath := range foundFilePaths {
        foundFilesMu.Lock()
        foundFiles[filePath] = true
        foundFilesMu.Unlock()
    }
}()

// Line 291: Blocking send
foundFilePaths <- result.FilePath  // Deadlocks if collector exits
```

**Fix**:
```go
// Option 1: Buffered channel
foundFilePaths := make(chan string, 1000)

// Option 2: Context-aware send (preferred)
select {
case foundFilePaths <- result.FilePath:
    // Sent successfully
case <-ctx.Done():
    // Context cancelled, exit gracefully
    return
}
```

**Impact**: Prevents goroutine deadlocks and leaks.

---

#### 0.5 Fix foundFiles Map Thread-Safety
**Effort**: 1 hour | **Severity**: HIGH | **Risk**: LOW

**Problem**: Lines 175-186, 748 - Map written by goroutine, read without lock in cleanup.

**Current**:
```go
// Line 175-186: Collector writes with lock
foundFiles := make(map[string]bool)
foundFilesMu := sync.Mutex{}

go func() {
    for filePath := range foundFilePaths {
        foundFilesMu.Lock()
        foundFiles[filePath] = true
        foundFilesMu.Unlock()
    }
}()

// Line 748: Cleanup reads WITHOUT lock
if !foundFiles[m.FilePath] {  // RACE: concurrent read/write
    // Delete media
}
```

**Fix**:
```go
// Wait for collector to finish before accessing map
<-foundFilesCollectorDone  // Ensure all writes complete
close(foundFilePaths)       // Signal no more paths

// Now safe to read without lock
if scanErr == nil && progress.ErrorCount == 0 {
    uc.cleanupStaleMedia(ctx, lib.ID, foundFiles)
}
```

**Impact**: Eliminates concurrent map access race, prevents panics.

---

#### 0.6 Add Nil Check for TV Episode Parsing
**Effort**: 30 minutes | **Severity**: HIGH | **Risk**: MINIMAL

**Problem**: Lines 424-447 - Can panic on nil `tvInfo` after parse failure.

**Current**:
```go
// Line 424
tvInfo, err := parser.ParseTVEpisode(result.FilePath)
if err != nil {
    fmt.Printf("failed to parse TV episode filename %s: %v\n", result.FilePath, err)
    return  // Returns but tvInfo might be nil
}

// Line 440: Potential nil dereference
season = tvInfo.Season  // PANIC if tvInfo is nil
```

**Fix**:
```go
tvInfo, err := parser.ParseTVEpisode(result.FilePath)
if err != nil || tvInfo == nil {
    uc.logger.Error("Failed to parse TV episode filename",
        "path", result.FilePath,
        "error", err)
    return  // Exit early, don't continue with nil tvInfo
}

// Now safe to use tvInfo
season := tvInfo.Season
```

**Impact**: Prevents panics on malformed TV episode filenames.

---

### Phase 1: Quick Wins (High Impact, Low Effort)

#### 1.1 Replace fmt.Printf with Structured Logging
**Effort**: 2 hours | **Impact**: HIGH | **Risk**: MINIMAL

**Changes**:
- Add `logger *slog.Logger` to `ScanLibraryUseCase`
- Replace all `fmt.Printf` calls with `logger.Error/Warn/Info`
- Add context fields: file path, library ID, media type, operation

**Example Transformation**:
```go
// Before
fmt.Printf("failed to create movie %s: %v\n", result.FilePath, err)

// After
uc.logger.Error("Failed to create movie",
    "path", result.FilePath,
    "library_id", libraryID,
    "operation", "create",
    "media_type", "movie",
    "error", err)
```

**Benefits**:
- Structured logs parseable by log aggregators
- Easy to filter/search by library, media type, error type
- Consistent with codebase logging patterns
- No behavior changes, pure improvement

**Files**:
- `internal/application/library/scan_library.go` (234, 307, 316, 389, 392, 398, 408, 415, 426, 518, 521, 527, 540, 548, 641, 644, 651, 673, 682, 694, 739, 759, 767, 778, 785, 792, 799)

#### 1.2 Enhanced Error Categorization
**Effort**: 3 hours | **Impact**: MEDIUM | **Risk**: LOW

**Changes**:
- Track error types separately (DB errors, parsing errors, I/O errors, metadata errors)
- Include in scan job completion summary
- Add to progress API response

**Schema Addition**:
```go
type ScanJob struct {
    // ... existing fields ...
    ErrorCount       int       // Total errors (existing)
    DBErrors         int       // Database operation failures (new)
    ParseErrors      int       // Filename/metadata parsing failures (new)
    IOErrors         int       // Filesystem I/O errors (new)
    MetadataErrors   int       // NFO/ID3 parsing failures (new)
}
```

**Implementation**:
```go
func (uc *ScanLibraryUseCase) categorizeError(err error, category string) {
    uc.mu.Lock()
    defer uc.mu.Unlock()

    uc.errorStats[category]++
    uc.logger.Warn("Categorized error",
        "category", category,
        "error", err)
}

// Usage
if err := uc.movieRepo.CreateMovie(ctx, movie); err != nil {
    uc.categorizeError(err, "database")
    uc.logger.Error("Failed to create movie", ...)
    return
}
```

**Benefits**:
- Identify systemic issues (e.g., all DB errors = database problem)
- Better troubleshooting ("fixing permissions" vs "fixing NFO files")
- Informs prioritization for future improvements

**Migration**:
```sql
-- Add to existing scan_jobs table
ALTER TABLE scan_jobs ADD COLUMN db_errors INTEGER DEFAULT 0;
ALTER TABLE scan_jobs ADD COLUMN parse_errors INTEGER DEFAULT 0;
ALTER TABLE scan_jobs ADD COLUMN io_errors INTEGER DEFAULT 0;
ALTER TABLE scan_jobs ADD COLUMN metadata_errors INTEGER DEFAULT 0;
```

#### 1.3 Error Sampling for Large Scans
**Effort**: 2 hours | **Impact**: MEDIUM | **Risk**: LOW

**Problem**: Logging every error in a 10,000 file scan floods logs

**Solution**: Log first N errors per category, then summarize

```go
type ErrorSampler struct {
    maxSamplesPerCategory int
    samples               map[string][]error
    counts                map[string]int
    mu                    sync.Mutex
}

func (es *ErrorSampler) Record(category string, err error) {
    es.mu.Lock()
    defer es.mu.Unlock()

    es.counts[category]++

    if len(es.samples[category]) < es.maxSamplesPerCategory {
        es.samples[category] = append(es.samples[category], err)
    }
}

func (es *ErrorSampler) LogSummary(logger *slog.Logger) {
    for category, count := range es.counts {
        logger.Info("Error summary",
            "category", category,
            "total_count", count,
            "sample_errors", es.samples[category])
    }
}
```

**Usage**:
```go
// In runScan()
errorSampler := NewErrorSampler(10) // Max 10 samples per category

// In processMovie/processTVEpisode/processMusicTrack
if err := uc.movieRepo.CreateMovie(ctx, movie); err != nil {
    errorSampler.Record("database_create", err)
    return
}

// After scan completes
errorSampler.LogSummary(uc.logger)
```

**Benefits**:
- Logs stay manageable even for large scans
- Still capture representative error samples
- Summary provides high-level health check

### Phase 1.5: Progress Reporting & User Experience

#### 1.5.1 Frontend Progress Display (CRITICAL)
**Effort**: 4-6 hours | **Impact**: CRITICAL | **Risk**: LOW

**Current Problem**:
- User clicks "Scan" and sees toast message, then nothing
- Backend has real-time SSE streaming, but frontend doesn't use it
- Users must manually refresh to see scan completion
- Zero visibility into scan progress

**Evidence**:
- `useGetApiLibrariesIdScanStream` hook exists but is never imported/used
- `LibraryCard.tsx` has scan button but no progress UI
- SSE endpoint (`/api/libraries/{id}/scan/stream`) works but is unused

**Implementation**:

1. **Create ScanProgressModal Component**:
```typescript
// web/src/components/library/ScanProgressModal.tsx
interface ScanProgressModalProps {
  libraryId: number
  onClose: () => void
}

export const ScanProgressModal: React.FC<ScanProgressModalProps> = ({
  libraryId,
  onClose
}) => {
  const { data, isLoading } = useGetApiLibrariesIdScanStream(libraryId)

  // Extract progress metrics
  const progress = data?.progress ?? 0
  const filesFound = data?.files_found ?? 0
  const filesProcessed = data?.files_processed ?? 0
  const errorCount = data?.error_count ?? 0
  const status = data?.status ?? 'running'

  // Calculate derived metrics
  const elapsedSeconds = data?.started_at
    ? Math.floor((Date.now() - new Date(data.started_at).getTime()) / 1000)
    : 0

  const processingRate = elapsedSeconds > 0
    ? filesProcessed / elapsedSeconds
    : 0

  const remainingFiles = filesFound - filesProcessed
  const estimatedRemainingSeconds = processingRate > 0
    ? Math.ceil(remainingFiles / processingRate)
    : 0

  return (
    <Dialog open={true} onClose={onClose}>
      <DialogTitle>Scanning Library</DialogTitle>
      <DialogContent>
        {/* Progress bar */}
        <LinearProgress
          variant="determinate"
          value={progress}
        />

        {/* File counts - Absolute numbers instead of percentage-only */}
        <Typography variant="body2">
          {filesProcessed} of {filesFound} files processed
          {filesFound > filesProcessed && ' (discovering more files...)'}
        </Typography>

        {/* Time estimates */}
        <Typography variant="body2" color="textSecondary">
          Elapsed: {formatDuration(elapsedSeconds)}
          {estimatedRemainingSeconds > 0 &&
            ` • ~${formatDuration(estimatedRemainingSeconds)} remaining`
          }
        </Typography>

        {/* Error count with link */}
        {errorCount > 0 && (
          <Alert severity="warning">
            {errorCount} errors encountered
            <Link onClick={() => setShowErrors(true)}>View details</Link>
          </Alert>
        )}

        {/* Status indicator */}
        <Chip
          label={status}
          color={status === 'completed' ? 'success' : 'primary'}
        />
      </DialogContent>
    </Dialog>
  )
}
```

2. **Wire into LibraryCard**:
```typescript
// web/src/components/library/LibraryCard.tsx
export const LibraryCard: React.FC<Props> = ({ library }) => {
  const [scanInProgress, setScanInProgress] = useState(false)
  const startScan = usePostApiLibrariesIdScan()

  const handleScanClick = async () => {
    await startScan.mutateAsync({ id: library.id })
    setScanInProgress(true) // Show modal instead of just toast
  }

  return (
    <>
      <Card>
        {/* ... existing card content ... */}
        <Button onClick={handleScanClick}>Scan Library</Button>
      </Card>

      {scanInProgress && (
        <ScanProgressModal
          libraryId={library.id}
          onClose={() => setScanInProgress(false)}
        />
      )}
    </>
  )
}
```

**Benefits**:
- Users see real-time progress during scan
- Absolute file counts prevent confusion from percentage-only display
- Time estimates help users plan (can walk away vs wait)
- Error visibility prompts investigation

#### 1.5.2 Fix Progress Calculation Accuracy
**Effort**: 3-4 hours | **Impact**: HIGH | **Risk**: MEDIUM

**Current Problem**:
- Progress percentage can appear to go backwards
- Files are discovered *while* processing happens
- Formula: `progress = (filesProcessed / filesFound) * 100`
- But `filesFound` grows as coordinator walks directory tree

**Example Scenario**:
```
t=5s:  filesFound=200, processed=100  → 50% complete
t=6s:  filesFound=400, processed=120  → 30% complete (backwards!)
t=7s:  filesFound=600, processed=150  → 25% complete
t=20s: filesFound=1000,processed=1000 → 100% complete
```

**Evidence**:
- `coordinator.go:184` increments `filesFound` during walk (progressive discovery)
- `scan_library.go:270-311` processes results while discovery continues

**Solution A: Two-Phase Approach** (Recommended):
```go
// Phase 1: Quick discovery (count files without processing)
func (c *Coordinator) CountFiles(ctx context.Context, rootPath string) (int, error) {
    count := 0
    err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return nil // Skip errors during count
        }
        if !info.IsDir() && c.isMediaFile(path) {
            count++
        }
        return nil
    })
    return count, err
}

// Phase 2: Process with accurate count
func (uc *ScanLibraryUseCase) runScan(ctx context.Context, jobID int64, lib *library.Library) {
    // Quick pre-count (1-2 seconds for most libraries)
    totalFiles, err := coordinator.CountFiles(ctx, lib.Path)
    if err != nil {
        uc.logger.Warn("Failed to count files, using progressive discovery", "error", err)
    }

    // Store total in job for accurate percentage
    job.FilesFound = totalFiles
    uc.scanJobRepo.UpdateProgress(ctx, jobID, &scanner.Progress{FilesFound: totalFiles})

    // Now process with accurate denominator
    coordinator.Scan(ctx, lib.Path, resultChan)
}
```

**Solution B: Show Discovery and Processing Separately** (Alternative):
```go
type Progress struct {
    DiscoveryProgress   float64 // 0-100%
    ProcessingProgress  float64 // 0-100%
    FilesFound          int
    FilesProcessed      int
}

// Frontend shows two progress bars:
// "Discovering files: 85%"
// "Processing: 247 of 1000 files (24.7%)"
```

**Solution C: De-emphasize Percentage** (Simplest):
- Don't show percentage at all
- Display: "Processing 500 of ~1000 files" (tilde indicates estimate)
- Show elapsed time and processing rate instead
- More honest about progressive discovery

**Recommendation**: Start with Solution C (simplest), implement Solution A if user feedback requests it.

**Benefits**:
- Users see trustworthy progress information
- No confusion from backwards-moving percentages
- Processing rate and time estimates are more useful anyway

#### 1.5.3 Persist and Display Error Details
**Effort**: 4-6 hours | **Impact**: MEDIUM | **Risk**: LOW

**Current Problem**:
- Errors are counted but not persisted
- Users see "50 errors" with no way to investigate
- Individual file errors only logged via `fmt.Printf()`
- No error history or categorization

**Implementation**:

1. **Create scan_errors table**:
```sql
-- Migration: add scan error tracking
CREATE TABLE IF NOT EXISTS scan_errors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    scan_job_id INTEGER NOT NULL,
    file_path TEXT NOT NULL,
    error_message TEXT NOT NULL,
    error_category TEXT NOT NULL, -- 'database', 'parsing', 'ffmpeg', 'filesystem', 'metadata'
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (scan_job_id) REFERENCES scan_jobs(id) ON DELETE CASCADE
);

CREATE INDEX idx_scan_errors_job_id ON scan_errors(scan_job_id);
CREATE INDEX idx_scan_errors_category ON scan_errors(scan_job_id, error_category);
```

2. **Update scanner to persist errors**:
```go
// In processMovie/processTVEpisode/processMusicTrack
if err := uc.movieRepo.CreateMovie(ctx, movie); err != nil {
    // Categorize error
    category := categorizeError(err)

    // Persist error detail
    scanError := &scanner.ScanError{
        ScanJobID:    jobID,
        FilePath:     result.FilePath,
        ErrorMessage: err.Error(),
        ErrorCategory: category,
    }

    if persistErr := uc.scanJobRepo.CreateError(ctx, scanError); persistErr != nil {
        uc.logger.Warn("Failed to persist scan error",
            "error", persistErr,
            "original_error", err)
    }

    // Increment category counter
    uc.categorizeError(err, category)

    uc.logger.Error("Failed to create movie",
        "path", result.FilePath,
        "error", err,
        "category", category)
    return
}
```

3. **Add API endpoint for error details**:
```go
// GET /api/libraries/{id}/scan/{jobId}/errors
func (h *ScanJobHandler) GetScanErrors(c *gin.Context) {
    jobID := parseJobID(c)

    errors, err := h.scanJobRepo.ListErrors(ctx, jobID)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    // Group by category
    errorsByCategory := make(map[string][]ScanErrorResponse)
    for _, e := range errors {
        errorsByCategory[e.ErrorCategory] = append(
            errorsByCategory[e.ErrorCategory],
            ScanErrorResponse{
                FilePath:     e.FilePath,
                ErrorMessage: e.ErrorMessage,
                CreatedAt:    e.CreatedAt,
            },
        )
    }

    c.JSON(200, gin.H{
        "total_errors": len(errors),
        "by_category": errorsByCategory,
    })
}
```

4. **Display in frontend**:
```typescript
// Expandable error list in ScanProgressModal
{errorCount > 0 && (
  <Accordion>
    <AccordionSummary>
      <ErrorIcon color="warning" />
      {errorCount} errors (click to view)
    </AccordionSummary>
    <AccordionDetails>
      {/* Fetch and display errors by category */}
      <ErrorList jobId={scanJobId} />
    </AccordionDetails>
  </Accordion>
)}
```

**Benefits**:
- Users can see exactly which files failed
- Error categories help diagnose systemic issues
- Historical error tracking for pattern recognition
- Actionable information ("fix NFO files" vs "database issue")

### Phase 2: Medium Complexity Improvements

#### 2.1 Retry Logic for Transient Failures
**Effort**: 1 day | **Impact**: MEDIUM | **Risk**: MEDIUM

**Scope**: Add exponential backoff retry for database operations

**Implementation**:
```go
type RetryConfig struct {
    MaxAttempts int
    InitialWait time.Duration
    MaxWait     time.Duration
    Multiplier  float64
}

func DefaultRetryConfig() RetryConfig {
    return RetryConfig{
        MaxAttempts: 3,
        InitialWait: 100 * time.Millisecond,
        MaxWait:     2 * time.Second,
        Multiplier:  2.0,
    }
}

func (uc *ScanLibraryUseCase) withRetry(ctx context.Context, operation string, fn func() error) error {
    cfg := DefaultRetryConfig()
    wait := cfg.InitialWait

    for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
        err := fn()
        if err == nil {
            if attempt > 1 {
                uc.logger.Info("Operation succeeded after retry",
                    "operation", operation,
                    "attempt", attempt)
            }
            return nil
        }

        // Only retry transient errors
        if !isTransientError(err) {
            return err
        }

        if attempt < cfg.MaxAttempts {
            uc.logger.Warn("Transient error, retrying",
                "operation", operation,
                "attempt", attempt,
                "wait", wait,
                "error", err)

            select {
            case <-ctx.Done():
                return ctx.Err()
            case <-time.After(wait):
            }

            wait = time.Duration(float64(wait) * cfg.Multiplier)
            if wait > cfg.MaxWait {
                wait = cfg.MaxWait
            }
        }
    }

    return fmt.Errorf("operation failed after %d attempts: %w", cfg.MaxAttempts, err)
}

func isTransientError(err error) bool {
    if err == nil {
        return false
    }

    errStr := err.Error()

    // SQLite-specific transient errors
    transientPatterns := []string{
        "database is locked",
        "database table is locked",
        "SQLITE_BUSY",
        "SQLITE_LOCKED",
        "temporary failure",
        "resource temporarily unavailable",
    }

    for _, pattern := range transientPatterns {
        if strings.Contains(errStr, pattern) {
            return true
        }
    }

    return false
}
```

**Usage**:
```go
// In processMovie
err := uc.withRetry(ctx, "create_movie", func() error {
    return uc.movieRepo.CreateMovie(ctx, movie)
})
if err != nil {
    uc.logger.Error("Failed to create movie after retries", ...)
    return
}
```

**Benefits**:
- Handles SQLite lock contention gracefully
- Reduces transient error noise
- No code duplication across operations

**Risks**:
- Increases scan duration for high-contention scenarios
- Could mask underlying issues (mitigated by logging)

**Mitigation**:
- Conservative retry limits (3 attempts, max 2s wait)
- Log all retry attempts for visibility
- Only retry clearly transient errors

#### 2.2 Batch Database Operations
**Effort**: 2 days | **Impact**: HIGH | **Risk**: MEDIUM

**Problem**: Each file = 1 transaction = 2,000+ DB operations for 1,000 files

**Solution**: Batch media creation/updates in groups

**Implementation**:
```go
type MediaBatch struct {
    movies   []*media.Movie
    episodes []*media.TVEpisode
    tracks   []*media.MusicTrack
    maxSize  int
    mu       sync.Mutex
}

func (b *MediaBatch) AddMovie(m *media.Movie) bool {
    b.mu.Lock()
    defer b.mu.Unlock()

    b.movies = append(b.movies, m)
    return len(b.movies) >= b.maxSize
}

func (uc *ScanLibraryUseCase) flushBatch(ctx context.Context, batch *MediaBatch) error {
    batch.mu.Lock()
    defer batch.mu.Unlock()

    if len(batch.movies) == 0 && len(batch.episodes) == 0 && len(batch.tracks) == 0 {
        return nil
    }

    // Use transaction for batch
    return uc.txManager.WithTransaction(ctx, func(tx *sql.Tx) error {
        // Batch insert movies
        for _, movie := range batch.movies {
            if err := uc.movieRepo.CreateMovieWithTx(ctx, tx, movie); err != nil {
                return err
            }
        }

        // Batch insert episodes
        for _, episode := range batch.episodes {
            if err := uc.tvRepo.CreateTVEpisodeWithTx(ctx, tx, episode); err != nil {
                return err
            }
        }

        // Batch insert tracks
        for _, track := range batch.tracks {
            if err := uc.musicRepo.CreateMusicTrackWithTx(ctx, tx, track); err != nil {
                return err
            }
        }

        // Clear batch
        batch.movies = batch.movies[:0]
        batch.episodes = batch.episodes[:0]
        batch.tracks = batch.tracks[:0]

        return nil
    })
}
```

**Usage**:
```go
// In processResults
batch := &MediaBatch{maxSize: 50} // Batch 50 items

for result := range resultChan {
    // ... existing logic ...

    // Add to batch instead of immediate insert
    switch libraryType {
    case library.LibraryTypeMovies:
        if batch.AddMovie(movie) {
            if err := uc.flushBatch(ctx, batch); err != nil {
                uc.logger.Error("Failed to flush batch", "error", err)
            }
        }
    // ... similar for TV and music
    }
}

// Final flush
uc.flushBatch(ctx, batch)
```

**Benefits**:
- Reduces DB operations by ~50x (2,000 → 40 for 1,000 files)
- Fewer transaction begin/commit cycles
- Better SQLite performance (reduced lock contention)

**Risks**:
- More complex error handling (partial batch failures)
- Increased memory usage (buffering)
- Need to handle batch rollbacks

**Mitigation**:
- Moderate batch size (50 items = ~100KB memory)
- Retry individual items on batch failure
- Flush on timeout to prevent indefinite buffering

#### 2.3 Improved Error Context and Logging
**Effort**: 1 day | **Impact**: MEDIUM | **Risk**: LOW

**Enhancement**: Add operation tracing and span logging

**Implementation**:
```go
type ScanSpan struct {
    operation string
    startTime time.Time
    logger    *slog.Logger
    attrs     []any
}

func (s *ScanSpan) End(err error) {
    duration := time.Since(s.startTime)

    attrs := append(s.attrs,
        "duration_ms", duration.Milliseconds(),
        "operation", s.operation)

    if err != nil {
        attrs = append(attrs, "error", err)
        s.logger.Error("Operation failed", attrs...)
    } else {
        s.logger.Debug("Operation completed", attrs...)
    }
}

func (uc *ScanLibraryUseCase) startSpan(operation string, attrs ...any) *ScanSpan {
    return &ScanSpan{
        operation: operation,
        startTime: time.Now(),
        logger:    uc.logger,
        attrs:     attrs,
    }
}
```

**Usage**:
```go
func (uc *ScanLibraryUseCase) processMovie(ctx context.Context, libraryID int64, result *scanner.ScanResult) {
    span := uc.startSpan("process_movie",
        "path", result.FilePath,
        "library_id", libraryID)
    defer func() {
        span.End(nil) // Or pass error
    }()

    // ... existing logic ...
}
```

**Benefits**:
- Performance profiling (slow file operations)
- Distributed tracing ready (can integrate OpenTelemetry later)
- Better troubleshooting with timing data

### Phase 3: Complex Architectural Changes (Future Consideration)

#### 3.1 Checkpointing for Partial Scan Recovery
**Effort**: 3-5 days | **Impact**: LOW | **Risk**: HIGH

**Deferral Rationale**:
- Complexity doesn't justify benefits for home media server
- Stuck scan recovery already handles crashes
- Rescans are rare after initial setup
- Database-backed checkpointing adds significant complexity

**Future Trigger**: User feedback about frequent scan interruptions

#### 3.2 Circuit Breaker for High Error Rates
**Effort**: 2 days | **Impact**: LOW | **Risk**: MEDIUM

**Deferral Rationale**:
- Rare scenario (corrupted filesystem)
- Timeout protection already exists
- Better error categorization (Phase 1) provides visibility
- Can add later if needed

**Implementation Sketch** (for future):
```go
type CircuitBreaker struct {
    threshold      float64 // Error rate threshold (e.g., 0.5 = 50%)
    windowSize     int     // Sample window
    consecutiveFail int
    mu             sync.Mutex
}

func (cb *CircuitBreaker) ShouldStop(errorCount, totalCount int) bool {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    if totalCount < cb.windowSize {
        return false // Not enough samples
    }

    errorRate := float64(errorCount) / float64(totalCount)
    return errorRate > cb.threshold
}
```

#### 3.3 Parallel Result Processing
**Effort**: 3 days | **Impact**: LOW | **Risk**: HIGH

**Deferral Rationale**:
- Adds significant complexity (worker pools, synchronization)
- Filesystem I/O is often the bottleneck, not CPU
- Database writes need coordination (SQLite = single writer)
- Batching (Phase 2.2) provides similar benefits with less complexity

**Future Trigger**: Profiling shows CPU-bound processing bottleneck

## Consequences

### Positive

**Phase 1**:
- ✅ **Better Observability**: Structured logs, error categorization, sampling
- ✅ **Easy Troubleshooting**: Identify root causes faster
- ✅ **No Behavior Changes**: Pure logging improvements
- ✅ **Quick Implementation**: 1-2 days total

**Phase 2**:
- ✅ **Improved Reliability**: Retry logic handles transient failures
- ✅ **Better Performance**: Batch operations reduce DB overhead
- ✅ **Enhanced Visibility**: Operation timing and tracing
- ✅ **Backward Compatible**: No schema breaking changes

### Negative

**Phase 1**:
- ⚠️ Slightly more verbose logs (mitigated by sampling)
- ⚠️ Minor schema migration for error categorization

**Phase 2**:
- ⚠️ Increased complexity (retry logic, batching)
- ⚠️ More memory usage (batch buffering ~100KB)
- ⚠️ Need for additional testing (retry paths, batch failures)

### Risks

**Retry Logic**:
- **Risk**: Masks underlying database issues
- **Mitigation**: Log all retries, limit attempts, only retry transient errors

**Batch Operations**:
- **Risk**: Partial batch failures complicate error handling
- **Mitigation**: Moderate batch size, fallback to individual operations on error

**Complexity Creep**:
- **Risk**: Over-engineering for home media server use case
- **Mitigation**: Phased approach, defer low-impact/high-complexity changes

## Alternatives Considered

### A. Keep Current Implementation (No Changes)
**Rejected**: Low-hanging fruit improvements provide significant value
- Structured logging is codebase standard
- Error categorization aids troubleshooting
- Minimal effort for meaningful gains

### B. Implement All Improvements Immediately
**Rejected**: Some changes have low ROI for home media server
- Checkpointing: Complex, rare benefit
- Circuit breaker: Handles edge cases only
- Parallel processing: Adds complexity without clear bottleneck

### C. External Job Queue (e.g., Asynq, Temporal)
**Rejected**: Massive overkill for scanning use case
- Adds Redis/database dependency
- Increases operational complexity
- Current architecture handles scanning well
- No requirement for distributed processing

### D. Event Sourcing for Scan State
**Rejected**: Over-engineering for simple progress tracking
- Complexity far exceeds benefits
- Current database-backed progress works well
- No requirement for audit trail or replay

## Implementation Plan

### Phase 1: Quick Wins (Week 1)
**Target**: 2-3 days

**Tasks**:
1. Add `logger *slog.Logger` to `ScanLibraryUseCase` constructor ✅
2. Replace all `fmt.Printf` with structured logging (27 occurrences) ✅
3. Add error categorization fields to `ScanJob` schema ✅
4. Implement `ErrorSampler` for large scans ✅
5. Update API responses to include error breakdown ✅

**Testing**:
- Unit tests for error sampler
- Integration test: verify logs for failed scans
- Manual test: scan library with intentional errors

**Success Criteria**:
- Zero `fmt.Printf` calls in scanner code
- Error categories tracked and reported
- Log volume reasonable for large scans (10,000+ files)

### Phase 2: Medium Complexity (Week 2-3)
**Target**: 5-7 days

**Tasks**:
1. Implement retry logic with exponential backoff ✅
2. Add `isTransientError()` detection ✅
3. Update repositories to support `WithTx` variants ✅
4. Implement batch operations for media creation ✅
5. Add span logging for performance tracking ✅
6. Write comprehensive tests for retry and batching ✅

**Testing**:
- Unit tests: retry logic, batch operations
- Integration tests: simulate database lock contention
- Load test: scan 1,000+ file library
- Performance benchmark: compare batch vs individual operations

**Success Criteria**:
- Retry logic handles SQLite lock errors gracefully
- Batch operations reduce DB transaction count by 50%+
- No regressions in scan accuracy or completeness
- Performance improvement measurable (10%+ faster for large scans)

### Phase 3: Deferred (Future)
**Trigger**: User feedback or profiling data

- Checkpointing (if scan interruptions become frequent)
- Circuit breaker (if corrupted filesystems are common)
- Parallel processing (if CPU becomes bottleneck)

## Monitoring and Metrics

### Key Metrics to Track

**Before Implementation**:
- Scan completion rate
- Average scan duration
- Error counts (total only)
- Database transaction count

**After Phase 1**:
- Error breakdown by category (DB, parse, I/O, metadata)
- Top error samples per category
- Scan success vs partial success vs failure rate

**After Phase 2**:
- Retry success rate
- Average retries per scan
- Batch flush frequency
- Database transaction reduction percentage
- Scan duration improvement

### Logging Examples

**Phase 1 - Structured Logging**:
```json
{
  "level": "error",
  "msg": "Failed to create movie",
  "path": "/media/movies/Inception (2010).mkv",
  "library_id": 1,
  "operation": "create",
  "media_type": "movie",
  "error": "SQLITE_BUSY: database is locked"
}
```

**Phase 1 - Error Summary**:
```json
{
  "level": "info",
  "msg": "Scan completed with errors",
  "library_id": 1,
  "total_files": 1250,
  "processed": 1240,
  "errors": {
    "database": 7,
    "parse": 2,
    "io": 1,
    "metadata": 0
  },
  "sample_errors": {
    "database": ["SQLITE_BUSY: database is locked", "connection timeout"],
    "parse": ["invalid season/episode format", "missing show name"]
  }
}
```

**Phase 2 - Retry Success**:
```json
{
  "level": "info",
  "msg": "Operation succeeded after retry",
  "operation": "create_movie",
  "attempt": 2,
  "path": "/media/movies/Inception (2010).mkv"
}
```

**Phase 2 - Batch Operation**:
```json
{
  "level": "debug",
  "msg": "Flushed media batch",
  "batch_size": 50,
  "media_type": "movies",
  "duration_ms": 145
}
```

## References

### Codebase Patterns
- **Structured Logging**: `internal/api/server.go:6`, `internal/app/container.go:6`
- **Transaction Manager**: `internal/application/common/transaction.go`
- **Stuck Scan Recovery**: `internal/infrastructure/persistence/scanjob/recovery.go`
- **Scheduler Error Handling**: `internal/infrastructure/scheduler/scheduler.go`

### Related ADRs
- [ADR 007: Unified Task Scheduler System](007-unified-task-scheduler.md)
- [ADR 010: Container Refactoring Strategy](010-container-refactoring-strategy.md)
- [ADR 011: Architectural Improvements Phase 1](011-architectural-improvements-phase-1.md)

### External Resources
- Go `slog` best practices: https://go.dev/blog/slog
- Retry strategies: https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/
- SQLite locking: https://sqlite.org/lockingv3.html
- Batch processing patterns: https://encore.dev/docs/how-to/batch-jobs

## Implementation Status

**Not Started** 📋

**Next Steps**:
1. Team review and approval
2. Create GitHub issues for Phase 1 tasks
3. Implement and test Phase 1 (target: 2-3 days)
4. Evaluate Phase 2 based on Phase 1 learnings
5. Consider Phase 3 based on production feedback

---

**Approved by**: _Pending Review_
**Implementation Start**: _TBD_
**Target Completion (Phase 1)**: _TBD + 3 days_
