# ADR 025: Resilient Library Scanner V2 - Checkpoint-Based Recovery

**Status**: Proposed
**Date**: 2025-11-22
**Author**: ViewRA Team
**Supersedes**: [ADR 014](014-library-scanner-resilience-improvements.md) (builds upon it)

## Context

The library scanner is a critical component responsible for discovering, cataloging, and indexing media files across movie, TV, and music libraries. While ADR-014 proposed improvements around logging and error handling, **real-world usage has revealed more fundamental resilience gaps**:

### Critical Pain Points

1. **No Recovery After Shutdown/Panic**
   - Application restart or panic mid-scan = complete restart from file 1
   - Large libraries (10,000+ files) waste hours of work
   - Users report frustration with "all or nothing" scanning

2. **No Incremental/Differential Scanning**
   - Full rescan required to detect new files added to filesystem
   - Can't efficiently detect which files are new/modified/deleted
   - File system monitoring (inotify/fsevents) not implemented

3. **Error Transparency Issues**
   - Errors logged but not aggregated for user visibility
   - No way to see "which files failed and why"
   - Error counts tracked but details are ephemeral

4. **Incomplete Records Create Data Corruption**
   - Failed track creation leaves orphaned media records
   - Partial album/artist creation causes relationship inconsistencies
   - Music library particularly fragile due to entity relationships

### Evidence from Codebase

**Current Implementation** ([scan_library.go:186-275](../../internal/application/library/scan_library.go#L186-L275)):
```go
func (uc *ScanLibraryUseCase) runScan(ctx context.Context, jobID int64, lib *library.Library) {
    // ... setup ...

    // No checkpoint loading
    // No state restoration
    // No resume capability

    scanErr := coordinator.Scan(ctx, lib.Path, resultChan)
    // If crashes here, all progress lost
}
```

**Database Schema** ([000001_init.up.sql:235-248](../../migrations/postgres/000001_init.up.sql#L235-L248)):
```sql
CREATE TABLE scan_jobs (
    id SERIAL PRIMARY KEY,
    library_id INTEGER NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('pending', 'running', 'paused', 'completed', 'failed')),
    progress DOUBLE PRECISION DEFAULT 0.0,
    files_found BIGINT DEFAULT 0,
    files_processed BIGINT DEFAULT 0,
    bytes_processed BIGINT DEFAULT 0,
    error_count BIGINT DEFAULT 0,
    -- No checkpoint data
    -- No processed file tracking
    -- No error details
);
```

**Incremental Scanning** ([coordinator.go:29-30](../../internal/infrastructure/filesystem/coordinator.go#L29-L30)):
```go
// EnableIncrementalScan enables smart file skipping based on ModTime
EnableIncrementalScan bool
// FileCache stores previously scanned file metadata
FileCache map[string]*scanner.FileCacheEntry
```
- Flag exists but **disabled by default**
- FileCache structure exists but not persisted
- No integration with scan_jobs table

### Real-World Failure Scenarios

**Scenario 1: Large Library Interrupted**
```
User starts scan of 15,000 movie collection
After 3 hours, processes 12,000 files (80% complete)
Server crashes due to power outage
On restart: Scan marked "failed", all work lost
User must rescan all 15,000 files from scratch
```

**Scenario 2: Music Track Relationship Corruption**
```
Scanner processes album with 15 tracks
Track 1-10 succeed, create media + music_track records
Track 11 fails FFmpeg extraction (corrupted file)
Track 12-15 continue processing
Result: Album has incomplete track list, no error details
Database shows album but missing tracks with no indication why
```

**Scenario 3: Incremental Update**
```
User adds 50 new albums to music library (600 tracks)
Existing library has 10,000 tracks
No way to scan only new files
Must rescan entire 10,600 track library
Takes 45 minutes vs potential 2-3 minutes for incremental
```

## Decision

Implement **checkpoint-based scan recovery** with **database-persisted state**, **incremental scanning support**, and **detailed error tracking**. This is a multi-phase approach that builds on ADR-014's logging improvements.

### Guiding Principles

1. **Database as Source of Truth**: All scan state persisted to survive restarts
2. **Graceful Degradation**: Partial progress is valuable, not wasted
3. **Error Transparency**: Users must see what failed and why
4. **Incremental by Default**: Only process changed files
5. **Transactional Safety**: No partial/corrupted records

## Proposed Solution

### Phase 1: Checkpoint System (CRITICAL)

**Goal**: Enable scan resume after interruption

#### 1.1 Database Schema Extensions

```sql
-- Migration: 000010_add_scan_checkpoints.up.sql

-- Checkpoint state for resumable scans
CREATE TABLE scan_checkpoints (
    id SERIAL PRIMARY KEY,
    scan_job_id INTEGER NOT NULL,
    file_path TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('pending', 'processing', 'completed', 'failed')),
    file_size BIGINT,
    file_hash TEXT,
    error_message TEXT,
    error_category TEXT CHECK(error_category IN ('parsing', 'ffmpeg', 'database', 'filesystem', 'metadata')),
    processed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (scan_job_id) REFERENCES scan_jobs(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX idx_scan_checkpoints_job_path ON scan_checkpoints(scan_job_id, file_path);
CREATE INDEX idx_scan_checkpoints_status ON scan_checkpoints(scan_job_id, status);
CREATE INDEX idx_scan_checkpoints_failed ON scan_checkpoints(scan_job_id, status) WHERE status = 'failed';

-- Extend scan_jobs table
ALTER TABLE scan_jobs ADD COLUMN last_checkpoint_at TIMESTAMP;
ALTER TABLE scan_jobs ADD COLUMN resume_count INTEGER DEFAULT 0;
```

#### 1.2 Checkpoint Repository

```go
// internal/domain/scanner/checkpoint.go
package scanner

type ScanCheckpoint struct {
    ID            int64
    ScanJobID     int64
    FilePath      string
    Status        CheckpointStatus
    FileSize      int64
    FileHash      string
    ErrorMessage  string
    ErrorCategory string
    ProcessedAt   *time.Time
    CreatedAt     time.Time
}

type CheckpointStatus string

const (
    CheckpointPending    CheckpointStatus = "pending"
    CheckpointProcessing CheckpointStatus = "processing"
    CheckpointCompleted  CheckpointStatus = "completed"
    CheckpointFailed     CheckpointStatus = "failed"
)

type CheckpointRepository interface {
    // Create batch of pending checkpoints for discovered files
    CreateBatch(ctx context.Context, checkpoints []*ScanCheckpoint) error

    // Get next batch of pending files to process
    GetPendingBatch(ctx context.Context, jobID int64, limit int) ([]*ScanCheckpoint, error)

    // Update checkpoint status after processing
    UpdateStatus(ctx context.Context, id int64, status CheckpointStatus, errorMsg, errorCategory string) error

    // Get scan statistics
    GetStats(ctx context.Context, jobID int64) (*CheckpointStats, error)

    // List failed checkpoints for user review
    ListFailed(ctx context.Context, jobID int64, limit int) ([]*ScanCheckpoint, error)

    // Check if file already processed in previous scan
    GetByPath(ctx context.Context, jobID int64, filePath string) (*ScanCheckpoint, error)
}

type CheckpointStats struct {
    TotalFiles      int64
    PendingFiles    int64
    ProcessedFiles  int64
    FailedFiles     int64
    ErrorsByCategory map[string]int64
}
```

#### 1.3 Modified Scan Flow

```go
// internal/application/library/scan_library.go

func (uc *ScanLibraryUseCase) runScan(ctx context.Context, jobID int64, lib *library.Library) {
    // STEP 1: Check for existing incomplete scan
    existingCheckpoints, err := uc.checkpointRepo.GetStats(ctx, jobID)
    if err == nil && existingCheckpoints.PendingFiles > 0 {
        uc.logger.Info("Resuming interrupted scan",
            "job_id", jobID,
            "pending_files", existingCheckpoints.PendingFiles,
            "completed_files", existingCheckpoints.ProcessedFiles)

        // Resume from checkpoint
        uc.resumeScan(ctx, jobID, lib, existingCheckpoints)
        return
    }

    // STEP 2: Fresh scan - discover files first
    uc.logger.Info("Starting fresh scan", "job_id", jobID, "library_path", lib.Path)

    // Phase 1: File Discovery (no processing yet)
    discoveredFiles := uc.discoverAllFiles(ctx, lib.Path)

    // Phase 2: Create checkpoints for all discovered files
    checkpoints := make([]*scanner.ScanCheckpoint, len(discoveredFiles))
    for i, file := range discoveredFiles {
        checkpoints[i] = &scanner.ScanCheckpoint{
            ScanJobID: jobID,
            FilePath:  file.Path,
            Status:    scanner.CheckpointPending,
            FileSize:  file.Size,
            CreatedAt: time.Now(),
        }
    }

    if err := uc.checkpointRepo.CreateBatch(ctx, checkpoints); err != nil {
        uc.logger.Error("Failed to create checkpoints", "error", err)
        return
    }

    // Phase 3: Process files (with checkpoint updates)
    uc.processScanWithCheckpoints(ctx, jobID, lib)
}

func (uc *ScanLibraryUseCase) processScanWithCheckpoints(ctx context.Context, jobID int64, lib *library.Library) {
    batchSize := 50

    for {
        // Get next batch of pending files
        batch, err := uc.checkpointRepo.GetPendingBatch(ctx, jobID, batchSize)
        if err != nil {
            uc.logger.Error("Failed to get pending batch", "error", err)
            break
        }

        if len(batch) == 0 {
            // No more pending files
            break
        }

        // Process each file in batch
        for _, checkpoint := range batch {
            // Mark as processing
            uc.checkpointRepo.UpdateStatus(ctx, checkpoint.ID, scanner.CheckpointProcessing, "", "")

            // Process file
            err := uc.processFile(ctx, lib.ID, lib.Type, checkpoint.FilePath)

            if err != nil {
                // Mark as failed with error details
                category := categorizeError(err)
                uc.checkpointRepo.UpdateStatus(ctx, checkpoint.ID, scanner.CheckpointFailed, err.Error(), category)
                uc.logger.Error("File processing failed",
                    "path", checkpoint.FilePath,
                    "error", err,
                    "category", category)
            } else {
                // Mark as completed
                uc.checkpointRepo.UpdateStatus(ctx, checkpoint.ID, scanner.CheckpointCompleted, "", "")
            }

            // Update job progress
            stats, _ := uc.checkpointRepo.GetStats(ctx, jobID)
            progress := &scanner.Progress{
                FilesFound:     stats.TotalFiles,
                FilesProcessed: stats.ProcessedFiles,
                ErrorCount:     stats.FailedFiles,
            }
            uc.scanJobRepo.UpdateProgress(ctx, jobID, progress)
        }
    }

    // Final status
    stats, _ := uc.checkpointRepo.GetStats(ctx, jobID)
    if stats.FailedFiles > 0 {
        uc.logger.Warn("Scan completed with errors",
            "job_id", jobID,
            "total", stats.TotalFiles,
            "failed", stats.FailedFiles,
            "errors_by_category", stats.ErrorsByCategory)
    } else {
        uc.logger.Info("Scan completed successfully",
            "job_id", jobID,
            "files_processed", stats.ProcessedFiles)
    }
}
```

### Phase 2: Incremental Scanning (HIGH PRIORITY)

**Goal**: Only process new/modified/deleted files

#### 2.1 Scan State Table

```sql
-- Migration: 000011_add_scan_state.up.sql

-- Persistent file state for incremental scanning
CREATE TABLE scan_state (
    id SERIAL PRIMARY KEY,
    library_id INTEGER NOT NULL,
    file_path TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    file_mtime TIMESTAMP NOT NULL,
    file_hash TEXT,
    media_id INTEGER,
    last_scanned_at TIMESTAMP NOT NULL,
    scan_job_id INTEGER NOT NULL,
    FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE CASCADE,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE SET NULL,
    FOREIGN KEY (scan_job_id) REFERENCES scan_jobs(id)
);

CREATE UNIQUE INDEX idx_scan_state_library_path ON scan_state(library_id, file_path);
CREATE INDEX idx_scan_state_library_mtime ON scan_state(library_id, file_mtime);
CREATE INDEX idx_scan_state_media_id ON scan_state(media_id);
```

#### 2.2 Incremental Scan Logic

```go
// internal/application/library/incremental_scan.go

type IncrementalScanner struct {
    scanStateRepo scanner.ScanStateRepository
    logger        *slog.Logger
}

func (is *IncrementalScanner) DetermineChanges(ctx context.Context, libraryID int64, currentFiles []scanner.FileInfo) (*ScanDiff, error) {
    // Get previous scan state
    previousState, err := is.scanStateRepo.GetLibraryState(ctx, libraryID)
    if err != nil {
        return nil, err
    }

    // Build maps for efficient lookup
    prevFileMap := make(map[string]*scanner.ScanState)
    for _, state := range previousState {
        prevFileMap[state.FilePath] = state
    }

    currentFileMap := make(map[string]scanner.FileInfo)
    for _, file := range currentFiles {
        currentFileMap[file.Path] = file
    }

    diff := &ScanDiff{
        NewFiles:      []scanner.FileInfo{},
        ModifiedFiles: []scanner.FileInfo{},
        DeletedFiles:  []string{},
        UnchangedFiles: []string{},
    }

    // Find new and modified files
    for path, currentFile := range currentFileMap {
        prevState, existed := prevFileMap[path]

        if !existed {
            // New file
            diff.NewFiles = append(diff.NewFiles, currentFile)
        } else if is.isFileModified(prevState, currentFile) {
            // Modified file
            diff.ModifiedFiles = append(diff.ModifiedFiles, currentFile)
        } else {
            // Unchanged file
            diff.UnchangedFiles = append(diff.UnchangedFiles, path)
        }
    }

    // Find deleted files
    for path := range prevFileMap {
        if _, exists := currentFileMap[path]; !exists {
            diff.DeletedFiles = append(diff.DeletedFiles, path)
        }
    }

    return diff, nil
}

func (is *IncrementalScanner) isFileModified(prev *scanner.ScanState, current scanner.FileInfo) bool {
    // Check modification time
    if !current.ModTime.Equal(prev.FileMTime) {
        return true
    }

    // Check size
    if current.Size != prev.FileSize {
        return true
    }

    return false
}

type ScanDiff struct {
    NewFiles       []scanner.FileInfo
    ModifiedFiles  []scanner.FileInfo
    DeletedFiles   []string
    UnchangedFiles []string
}

func (d *ScanDiff) NeedsProcessing() bool {
    return len(d.NewFiles) > 0 || len(d.ModifiedFiles) > 0 || len(d.DeletedFiles) > 0
}

func (d *ScanDiff) Summary() string {
    return fmt.Sprintf("new=%d, modified=%d, deleted=%d, unchanged=%d",
        len(d.NewFiles), len(d.ModifiedFiles), len(d.DeletedFiles), len(d.UnchangedFiles))
}
```

#### 2.3 Modified Scan Flow for Incremental

```go
func (uc *ScanLibraryUseCase) runScan(ctx context.Context, jobID int64, lib *library.Library) {
    // Check if incremental scan is enabled
    if uc.config.EnableIncrementalScan {
        diff, err := uc.incrementalScanner.DetermineChanges(ctx, lib.ID, discoveredFiles)
        if err != nil {
            uc.logger.Warn("Incremental scan failed, falling back to full scan", "error", err)
        } else {
            if !diff.NeedsProcessing() {
                uc.logger.Info("No changes detected, skipping scan",
                    "library_id", lib.ID,
                    "total_files", len(diff.UnchangedFiles))
                return
            }

            uc.logger.Info("Incremental scan detected changes",
                "library_id", lib.ID,
                "summary", diff.Summary())

            // Process only changed files
            uc.processIncrementalChanges(ctx, jobID, lib, diff)
            return
        }
    }

    // Fall through to full scan
    uc.processFullScan(ctx, jobID, lib)
}

func (uc *ScanLibraryUseCase) processIncrementalChanges(ctx context.Context, jobID int64, lib *library.Library, diff *ScanDiff) {
    // Process new files
    for _, file := range diff.NewFiles {
        uc.processFile(ctx, lib.ID, lib.Type, file.Path)
    }

    // Process modified files (update existing media records)
    for _, file := range diff.ModifiedFiles {
        uc.updateExistingFile(ctx, lib.ID, lib.Type, file.Path)
    }

    // Handle deleted files (remove from database)
    for _, path := range diff.DeletedFiles {
        uc.removeDeletedFile(ctx, lib.ID, path)
    }

    // Update scan state
    uc.updateScanState(ctx, lib.ID, jobID, diff)
}
```

### Phase 3: Error Transparency & Reporting

**Goal**: Users can see exactly what failed and why

#### 3.1 Error Details API

```go
// internal/api/handlers/scanjob.go

// GET /api/libraries/{id}/scan/{jobId}/errors
func (h *ScanJobHandler) GetScanErrors(c *gin.Context) {
    jobID := parseJobID(c)

    // Get failed checkpoints
    failed, err := h.checkpointRepo.ListFailed(ctx, jobID, 1000)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    // Group by error category
    errorsByCategory := make(map[string][]ScanErrorDetail)
    for _, checkpoint := range failed {
        errorsByCategory[checkpoint.ErrorCategory] = append(
            errorsByCategory[checkpoint.ErrorCategory],
            ScanErrorDetail{
                FilePath:     checkpoint.FilePath,
                ErrorMessage: checkpoint.ErrorMessage,
                FileSize:     checkpoint.FileSize,
                ProcessedAt:  checkpoint.ProcessedAt,
            },
        )
    }

    c.JSON(200, gin.H{
        "total_errors": len(failed),
        "by_category": errorsByCategory,
    })
}

// GET /api/libraries/{id}/scan/{jobId}/retry-failed
func (h *ScanJobHandler) RetryFailedFiles(c *gin.Context) {
    jobID := parseJobID(c)

    // Reset failed checkpoints to pending
    count, err := h.checkpointRepo.ResetFailed(ctx, jobID)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    c.JSON(200, gin.H{
        "message": "Failed files queued for retry",
        "count": count,
    })
}
```

#### 3.2 Frontend Error Display

```typescript
// web/src/components/library/ScanErrorsDialog.tsx

interface ScanErrorsDialogProps {
  jobId: number
  onClose: () => void
  onRetry: () => void
}

export const ScanErrorsDialog: React.FC<ScanErrorsDialogProps> = ({
  jobId,
  onClose,
  onRetry,
}) => {
  const { data: errors, isLoading } = useGetApiLibrariesIdScanJobIdErrors(jobId)

  if (isLoading) return <CircularProgress />

  return (
    <Dialog open={true} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>
        Scan Errors ({errors?.total_errors || 0})
      </DialogTitle>
      <DialogContent>
        {Object.entries(errors?.by_category || {}).map(([category, items]) => (
          <Accordion key={category}>
            <AccordionSummary>
              <ErrorIcon color="error" />
              <Typography>{category}: {items.length} errors</Typography>
            </AccordionSummary>
            <AccordionDetails>
              <List>
                {items.map((item, idx) => (
                  <ListItem key={idx}>
                    <ListItemText
                      primary={item.file_path}
                      secondary={item.error_message}
                    />
                  </ListItem>
                ))}
              </List>
            </AccordionDetails>
          </Accordion>
        ))}
      </DialogContent>
      <DialogActions>
        <Button onClick={onRetry} color="primary">
          Retry Failed Files
        </Button>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  )
}
```

### Phase 4: Transactional Safety for Music

**Goal**: Prevent partial/corrupted music entity relationships

#### 4.1 Track Creation Transaction

```go
// internal/application/library/music_processor.go

func (uc *ScanLibraryUseCase) processMusicTrack(ctx context.Context, libraryID int64, result *scanner.ScanResult) error {
    // Use transaction for atomicity
    return uc.txManager.WithTransaction(ctx, func(tx *sql.Tx) error {
        // Step 1: Create or get artist entity
        artistID, err := uc.ensureArtist(ctx, tx, libraryID, result.Artist, result)
        if err != nil {
            return fmt.Errorf("failed to ensure artist: %w", err)
        }

        // Step 2: Create or get album entity
        albumID, err := uc.ensureAlbum(ctx, tx, libraryID, artistID, result.Album, result)
        if err != nil {
            return fmt.Errorf("failed to ensure album: %w", err)
        }

        // Step 3: Create media record
        mediaID, err := uc.createMediaRecord(ctx, tx, libraryID, result)
        if err != nil {
            return fmt.Errorf("failed to create media: %w", err)
        }

        // Step 4: Create track record with relationships
        track := &media.MusicTrack{
            Media:    media.Media{ID: mediaID},
            ArtistID: artistID,
            AlbumID:  albumID,
            // ... other fields
        }

        if err := uc.musicRepo.CreateMusicTrackWithTx(ctx, tx, track); err != nil {
            return fmt.Errorf("failed to create track: %w", err)
        }

        return nil
    })
}

func (uc *ScanLibraryUseCase) ensureArtist(ctx context.Context, tx *sql.Tx, libraryID int64, artistName string, result *scanner.ScanResult) (int64, error) {
    // Try to find existing
    existing, err := uc.musicRepo.FindArtistByNameWithTx(ctx, tx, libraryID, artistName)
    if err == nil && existing != nil {
        return existing.ID, nil
    }

    // Create new artist
    artist := &media.Artist{
        LibraryID: libraryID,
        Name:      artistName,
        // ... metadata from result
    }

    if err := uc.musicRepo.CreateArtistWithTx(ctx, tx, artist); err != nil {
        return 0, err
    }

    return artist.ID, nil
}
```

## Implementation Plan

### Phase 1: Checkpoint System (Week 1-2)
**Priority**: CRITICAL | **Effort**: 10-12 days | **Risk**: MEDIUM

**Tasks**:
1. Create migration for scan_checkpoints table ✅
2. Implement CheckpointRepository interface ✅
3. Modify runScan() to create checkpoints on discovery ✅
4. Implement batch-based processing with checkpoint updates ✅
5. Add resume logic for interrupted scans ✅
6. Update scan recovery to use checkpoints ✅
7. Add integration tests for checkpoint flow ✅
8. Test with intentional crashes/restarts ✅

**Success Criteria**:
- Scan can resume after app restart at exact file where stopped
- No duplicate processing of already-completed files
- Failed files tracked with error details
- Progress accurate across restarts

### Phase 2: Incremental Scanning (Week 3-4)
**Priority**: HIGH | **Effort**: 8-10 days | **Risk**: MEDIUM

**Tasks**:
1. Create migration for scan_state table ✅
2. Implement ScanStateRepository ✅
3. Create IncrementalScanner service ✅
4. Modify scan flow to use incremental logic ✅
5. Add API flag to force full scan ✅
6. Update scan state after successful processing ✅
7. Test with add/modify/delete scenarios ✅
8. Benchmark performance (incremental vs full) ✅

**Success Criteria**:
- Adding 50 files to 10,000 file library only processes 50 files
- Modified files correctly detected by mtime + size
- Deleted files removed from database
- Unchanged files skipped entirely
- 10x+ speedup for incremental vs full scan

### Phase 3: Error Transparency (Week 5)
**Priority**: MEDIUM | **Effort**: 4-5 days | **Risk**: LOW

**Tasks**:
1. Add error listing API endpoint ✅
2. Add retry failed files endpoint ✅
3. Create ScanErrorsDialog frontend component ✅
4. Wire error display into ScanProgressModal ✅
5. Add error category filtering ✅
6. Test error display with intentional failures ✅

**Success Criteria**:
- Users can see list of failed files
- Errors grouped by category (parsing, ffmpeg, database, etc.)
- One-click retry for failed files
- Error history preserved across scans

### Phase 4: Music Transactional Safety (Week 6)
**Priority**: MEDIUM | **Effort**: 5-6 days | **Risk**: MEDIUM

**Tasks**:
1. Add `WithTx` variants to MusicRepository methods ✅
2. Wrap music track processing in transaction ✅
3. Implement `ensureArtist` and `ensureAlbum` helpers ✅
4. Add rollback on any step failure ✅
5. Test with intentional failures at each step ✅
6. Verify no orphaned records created ✅

**Success Criteria**:
- Track creation is all-or-nothing (no partial records)
- Artist/Album/Track relationships always consistent
- Failed track doesn't leave orphaned media record
- Rollback on any error in chain

## Monitoring & Metrics

### Key Metrics to Track

**Checkpoint System**:
- Resume success rate (% of interrupted scans successfully resumed)
- Average files processed before interruption
- Time saved by resuming vs full rescan
- Checkpoint overhead (storage, query time)

**Incremental Scanning**:
- Incremental scan ratio (changed files / total files)
- Time savings (incremental vs full scan)
- False positive rate (unchanged files incorrectly marked as changed)
- False negative rate (changed files missed)

**Error Tracking**:
- Error rate by category
- Most common error messages
- Retry success rate
- Files permanently failed after N retries

### Logging Examples

**Checkpoint Resume**:
```json
{
  "level": "info",
  "msg": "Resuming interrupted scan",
  "job_id": 123,
  "library_id": 1,
  "pending_files": 3452,
  "completed_files": 8123,
  "resume_count": 1,
  "last_checkpoint_at": "2025-11-22T14:32:15Z"
}
```

**Incremental Scan**:
```json
{
  "level": "info",
  "msg": "Incremental scan detected changes",
  "library_id": 1,
  "new_files": 52,
  "modified_files": 8,
  "deleted_files": 3,
  "unchanged_files": 10240,
  "processing_only": 63
}
```

**Failed File Tracking**:
```json
{
  "level": "error",
  "msg": "File processing failed",
  "checkpoint_id": 4567,
  "file_path": "/media/music/Artist/Album/03 - Track.flac",
  "error": "FFmpeg extraction failed: codec not supported",
  "error_category": "ffmpeg",
  "retry_count": 0
}
```

## Consequences

### Positive

**Checkpoint System**:
- ✅ **Resilient to Interruptions**: Scans survive app restarts, panics, crashes
- ✅ **Progress Preservation**: Hours of work not lost on failure
- ✅ **User Confidence**: Can safely restart app during long scans
- ✅ **Better Observability**: Clear audit trail of what succeeded/failed

**Incremental Scanning**:
- ✅ **Massive Performance Gain**: 50 file addition = 50 file scan (not 10,000)
- ✅ **Lower Resource Usage**: Less CPU, disk I/O, database writes
- ✅ **Faster Library Updates**: New media appears in minutes, not hours
- ✅ **Reduced Server Load**: Less strain during regular library maintenance

**Error Transparency**:
- ✅ **User Empowerment**: See exactly what failed and why
- ✅ **Actionable Feedback**: Fix underlying issues (permissions, codecs, etc.)
- ✅ **Retry Capability**: One-click retry for transient failures
- ✅ **Historical Record**: Error patterns visible over time

**Music Transactional Safety**:
- ✅ **Data Integrity**: No partial/orphaned records
- ✅ **Consistent Relationships**: Artist/Album/Track always valid
- ✅ **Rollback on Error**: Failed track doesn't corrupt database
- ✅ **Simpler Cleanup**: No need to manually fix inconsistencies

### Negative

**Checkpoint System**:
- ⚠️ **Increased Complexity**: More database tables, more code paths
- ⚠️ **Storage Overhead**: Checkpoint data for every file in every scan
- ⚠️ **Query Complexity**: Batch retrieval of pending checkpoints
- ⚠️ **Migration Risk**: Existing in-progress scans invalid after upgrade

**Incremental Scanning**:
- ⚠️ **False Positive Risk**: Unchanged files incorrectly marked as changed
- ⚠️ **False Negative Risk**: Changed files missed (e.g., metadata-only changes)
- ⚠️ **Complexity**: Additional logic for diff calculation
- ⚠️ **State Drift**: scan_state table can become stale if files moved outside app

**Error Transparency**:
- ⚠️ **More Database Load**: Every error written to database
- ⚠️ **UI Complexity**: Error display adds frontend complexity
- ⚠️ **User Confusion**: May overwhelm users with error details

**Music Transactional Safety**:
- ⚠️ **Performance Impact**: Transactions add overhead
- ⚠️ **Lock Contention**: SQLite single-writer limitation
- ⚠️ **Complexity**: More complex repository method signatures (`WithTx` variants)

### Risks & Mitigation

**Risk: Checkpoint Table Growth**
- **Mitigation**: Automatic cleanup of old checkpoints after scan completion
- **Mitigation**: Index on scan_job_id for efficient deletion
- **Mitigation**: Configurable retention (e.g., keep last 5 scans)

**Risk: Incremental Scan False Negatives**
- **Mitigation**: Provide "Force Full Scan" button in UI
- **Mitigation**: Log when incremental scan is used vs full scan
- **Mitigation**: Weekly automatic full scan via scheduler (configurable)

**Risk: Transaction Deadlocks (SQLite)**
- **Mitigation**: Retry logic with exponential backoff
- **Mitigation**: Batch size tuning to reduce transaction duration
- **Mitigation**: Consider batching artist/album creation separately

**Risk: Migration Breaking In-Progress Scans**
- **Mitigation**: Mark all running scans as "failed" during migration
- **Mitigation**: Clear communication in migration notes
- **Mitigation**: Provide migration script to preserve progress if possible

## Alternatives Considered

### A. Keep Current "All or Nothing" Approach
**Rejected**: Real-world usage shows this is insufficient
- Large libraries waste hours on interruption
- Users frustrated by lack of resume capability
- Incremental scanning essential for usability

### B. File System Watching (inotify/fsevents)
**Deferred**: More complex, not needed for Phase 1
- Cross-platform complexity (Linux, macOS, Windows)
- Doesn't help with existing files (still need incremental scan)
- Can be added later as Phase 5 enhancement

### C. External Job Queue (e.g., Asynq, Temporal)
**Rejected**: Overkill for single-server media server
- Adds Redis/database dependency
- Operational complexity too high
- Checkpoint system provides 90% of benefits with 10% of complexity

### D. Event Sourcing for Scan State
**Rejected**: Over-engineering for this use case
- Complexity far exceeds benefits
- No requirement for audit trail replay
- Checkpoint table provides sufficient state

## Related ADRs

- [ADR 014: Library Scanner Resilience Improvements](014-library-scanner-resilience-improvements.md) - Logging and error handling
- [ADR 007: Unified Task Scheduler System](007-unified-task-scheduler.md) - Background task management
- [ADR 011: Architectural Improvements Phase 1](011-architectural-improvements-phase-1.md) - Clean architecture patterns

## References

### Codebase Components

- **Scanner Core**: [internal/application/library/scan_library.go](../../internal/application/library/scan_library.go)
- **Coordinator**: [internal/infrastructure/filesystem/coordinator.go](../../internal/infrastructure/filesystem/coordinator.go)
- **Scan Jobs**: [internal/infrastructure/persistence/scanjob/repository.go](../../internal/infrastructure/persistence/scanjob/repository.go)
- **Music Repository**: [internal/infrastructure/persistence/music/repository.go](../../internal/infrastructure/persistence/music/repository.go)
- **Transaction Manager**: [internal/application/common/transaction.go](../../internal/application/common/transaction.go)

### External Resources

- **Database Checkpointing Patterns**: https://martin.kleppmann.com/2015/05/11/please-stop-calling-databases-cp-or-ap.html
- **Incremental Processing**: https://encore.dev/docs/how-to/incremental-processing
- **SQLite Transaction Best Practices**: https://sqlite.org/lang_transaction.html
- **Go Context Patterns**: https://go.dev/blog/context

## Implementation Status

**Status**: Proposed 📋
**Target Start**: November 25, 2025
**Target Completion**: January 15, 2026 (8 weeks)

**Next Steps**:
1. Team review and approval
2. Create GitHub issues for each phase
3. Phase 1 implementation (checkpoints) - highest priority
4. Integration testing with real-world libraries
5. Phase 2 implementation (incremental scanning)
6. User acceptance testing
7. Documentation and migration guide

---

**Approved by**: _Pending Review_
**Implementation Start**: _TBD_
**Estimated Completion**: _TBD + 8 weeks_
