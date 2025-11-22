# Scan Robustness Improvements

## Problem Statement

Scan jobs can get stuck in "running" state with 0 files when:
1. Server crashes after job creation but before file discovery
2. Port conflicts prevent server from starting properly
3. File discovery phase fails silently

**Example**: TV and Movie libraries created scan jobs but server crashed due to port 8080 conflict, leaving jobs in "running" state with no files discovered.

## Root Causes

### 1. Optimistic Job Creation
- Scan jobs are created in "running" state immediately
- File discovery happens asynchronously after job creation
- If server crashes between these steps, job is orphaned

### 2. No Early Health Checks
- No validation that server is fully operational before accepting scan requests
- Port conflicts discovered too late (after jobs created)

### 3. Insufficient Timeout Detection
- Jobs can stay "running" indefinitely if workers die
- Cleanup task marks jobs "completed" if no pending files, but doesn't detect "never started" case

### 4. Silent File Discovery Failures
- If file discovery fails, no error is recorded
- Job stays "running" with 0/0 progress

## Proposed Solutions

### Solution 1: Two-Phase Scan Job Creation

**Current Flow:**
```
1. Create job (status=running)
2. Discover files → Create checkpoints
3. Process checkpoints
```

**Improved Flow:**
```
1. Create job (status=discovering)
2. Discover files → Create checkpoints
   - If discovery fails: Mark job as failed
   - If 0 files found: Mark job as completed with warning
3. Transition to status=running
4. Process checkpoints
```

**Implementation:**
```go
// Add new status to scan_jobs
ALTER TABLE scan_jobs MODIFY COLUMN status TEXT CHECK(
    status IN ('discovering', 'pending', 'running', 'paused', 'completed', 'failed')
);

// In scan_library.go
func (uc *ScanLibraryUseCase) Execute(ctx context.Context, libraryID int64) error {
    // Create job in 'discovering' state
    job := &scanner.ScanJob{
        LibraryID: libraryID,
        Status:    scanner.StatusDiscovering,
    }

    // Discover files BEFORE marking as running
    filesFound, err := uc.discoverFiles(ctx, job)
    if err != nil {
        job.Status = scanner.StatusFailed
        job.ErrorMessage = err.Error()
        return err
    }

    if filesFound == 0 {
        job.Status = scanner.StatusCompleted
        job.ErrorMessage = "No media files found in library"
        return nil
    }

    // Now transition to running
    job.Status = scanner.StatusRunning
    job.FilesFound = filesFound
}
```

### Solution 2: Scan Job Heartbeat

Add heartbeat mechanism to detect dead workers:

```go
// In scan_jobs table
ALTER TABLE scan_jobs ADD COLUMN last_heartbeat DATETIME;

// Update heartbeat every 30 seconds during active scanning
func (uc *ScanLibraryUseCase) processWithHeartbeat(ctx context.Context, job *scanner.ScanJob) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    go func() {
        for range ticker.C {
            uc.scanJobRepo.UpdateHeartbeat(ctx, job.ID)
        }
    }()

    // ... process files ...
}

// Cleanup task detects stale jobs
func (t *CleanupTask) detectStaleJobs() {
    staleThreshold := time.Now().Add(-5 * time.Minute)

    staleJobs := scanJobRepo.FindRunningWithHeartbeatBefore(ctx, staleThreshold)
    for _, job := range staleJobs {
        // Mark as failed with timeout error
        job.Status = scanner.StatusFailed
        job.ErrorMessage = "Scan worker died (no heartbeat for 5+ minutes)"
    }
}
```

### Solution 3: File Discovery Validation

Add explicit validation that file discovery succeeded:

```go
func (uc *ScanLibraryUseCase) validateDiscovery(job *scanner.ScanJob) error {
    checkpoints, err := uc.checkpointRepo.CountByJobID(ctx, job.ID)
    if err != nil {
        return fmt.Errorf("failed to count checkpoints: %w", err)
    }

    if checkpoints == 0 {
        return fmt.Errorf("file discovery produced 0 files - library may be empty or inaccessible")
    }

    return nil
}
```

### Solution 4: Enhanced Cleanup Task

Improve the automatic cleanup to handle edge cases:

```go
func (t *CleanupTask) cleanupStuckScans(ctx context.Context) error {
    // Find scans that are "running" but have:
    // 1. No checkpoints at all (never started)
    // 2. No heartbeat for 5+ minutes (worker died)
    // 3. Created more than 1 hour ago (stuck)

    stuckScans := scanJobRepo.FindStuckScans(ctx, ScanStuckCriteria{
        MinAge:                 1 * time.Hour,
        MaxHeartbeatAge:        5 * time.Minute,
        AllowZeroCheckpoints:   false,
    })

    for _, scan := range stuckScans {
        checkpoints := checkpointRepo.CountByJobID(ctx, scan.ID)

        if checkpoints == 0 {
            // Never actually started - mark as failed
            scan.Status = scanner.StatusFailed
            scan.ErrorMessage = "File discovery never completed - worker may have crashed"
        } else if scan.LastHeartbeat.Before(time.Now().Add(-5 * time.Minute)) {
            // Worker died mid-scan
            scan.Status = scanner.StatusFailed
            scan.ErrorMessage = "Scan worker stopped responding (no heartbeat)"
        }

        scanJobRepo.Update(ctx, scan)
    }
}
```

### Solution 5: Pre-Flight Health Check

Add health validation before creating scan jobs:

```go
func (h *LibraryHandler) Scan(c *gin.Context) {
    // Health check BEFORE creating job
    if err := h.validateServerHealth(); err != nil {
        c.JSON(http.StatusServiceUnavailable, gin.H{
            "error": "Server not ready for scan",
            "details": err.Error(),
        })
        return
    }

    // Now safe to create scan job
    // ...
}

func (h *LibraryHandler) validateServerHealth() error {
    // Check workers are running
    if !h.scanUseCase.AreWorkersHealthy() {
        return fmt.Errorf("scan workers not initialized")
    }

    // Check database connectivity
    if err := h.scanJobRepo.Ping(); err != nil {
        return fmt.Errorf("database unavailable: %w", err)
    }

    return nil
}
```

## Implementation Priority

1. **High Priority** (Prevents future stuck scans):
   - ✅ Solution 4: Enhanced cleanup task (quick win, minimal changes)
   - ✅ Solution 3: File discovery validation

2. **Medium Priority** (Better detection):
   - Solution 2: Heartbeat mechanism
   - Solution 5: Pre-flight health check

3. **Low Priority** (Nice to have):
   - Solution 1: Two-phase job creation (requires schema migration)

## Quick Fix for Current Issue

For the immediate problem (TV/Movie scans stuck):

```sql
-- Manual cleanup of stuck scans
UPDATE scan_jobs
SET status = 'failed',
    error_message = 'File discovery never completed - marked failed during cleanup',
    completed_at = CURRENT_TIMESTAMP
WHERE status = 'running'
  AND (SELECT COUNT(*) FROM scan_checkpoints WHERE scan_job_id = scan_jobs.id) = 0
  AND created_at < datetime('now', '-1 hour');
```

## Testing Plan

1. **Simulate port conflict** - Ensure new scans fail gracefully
2. **Kill worker mid-scan** - Ensure heartbeat detects and marks failed
3. **Empty library scan** - Ensure "discovering" status handles 0 files
4. **Server crash test** - Create job, crash server, restart - ensure cleanup works

## Related Files

- `internal/application/library/scan_library.go` - Main scan logic
- `internal/infrastructure/scheduler/tasks/scan_job_cleanup.go` - Cleanup task
- `internal/api/handlers/library_handler.go` - Scan endpoint
- `migrations/` - Schema changes for new columns

## Success Metrics

- Zero stuck scans in "running" state after 1 hour
- All failed scans have descriptive error messages
- File discovery failures are caught and reported
- Cleanup task successfully detects and fixes edge cases
