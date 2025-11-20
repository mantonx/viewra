# ADR 009: Migrate Transcode Cleanup to Unified Scheduler

**Status**: Proposed
**Date**: 2025-11-17
**Author**: ViewRA Team
**Supersedes**: N/A
**Related**: [ADR 007: Unified Task Scheduler](007-unified-task-scheduler.md), [ADR 005: On-Demand Transcoding Strategy](005-on-demand-transcoding-strategy.md)

## Context

### Current Architecture

Transcode cleanup currently uses a standalone `CleanupScheduler` class with its own ticker-based scheduling loop:

```go
// internal/application/transcode/cleanup_scheduler.go
type CleanupScheduler struct {
    config         *CleanupSchedulerConfig
    cleanupService *CleanupService
    repo           transcode.Repository
    outputDir      string
    logger         *slog.Logger
    ctx            context.Context
    cancel         context.CancelFunc
    wg             sync.WaitGroup
}

func (s *CleanupScheduler) run() {
    ticker := time.NewTicker(s.config.Interval)  // Custom ticker loop
    defer ticker.Stop()

    for {
        select {
        case <-s.ctx.Done():
            return
        case <-ticker.C:
            s.performCleanup()
        }
    }
}
```

**Configuration** (via environment variables):
- `Interval`: 6 hours (fixed interval, not cron-based)
- `DiskThresholdPercent`: 85%
- `MaxAgeHours`: 720 (30 days)
- `MaxIdleHours`: 168 (7 days)
- `KeepFailedHours`: 24

**Cleanup Operations**:
1. **Policy-Based** (always runs):
   - Clean failed jobs > 24 hours
   - Clean completed jobs > 30 days
   - Clean idle jobs (not accessed in 7 days)
   - Clean orphaned files (no DB record)

2. **Threshold-Based LRU** (when disk >= 85%):
   - Delete least-recently-used transcodes in batches

### Problem: Architectural Duplication

ViewRA already has a **unified scheduler** (ADR 007) that:
- Uses cron expressions for flexible scheduling
- Provides centralized task management
- Tracks execution history in database
- Offers API endpoints for task control
- Supports manual triggers and enable/disable

**Current State**:
- ✅ Image cleanup uses unified scheduler
- ❌ Transcode cleanup uses custom scheduler
- ❌ Two separate scheduling systems in codebase
- ❌ Inconsistent management interfaces
- ❌ Duplicate ticker/cron logic

This violates **DRY** and creates **unnecessary complexity**.

## Decision

**Migrate transcode cleanup to use the unified scheduler**, aligning with ADR 007 and eliminating the standalone `CleanupScheduler`.

### Architecture

#### 1. Task Registration

Register cleanup tasks with the unified scheduler during application startup:

```go
// internal/app/container.go
func setupTranscodeCleanupTasks(
    scheduler *scheduler.Scheduler,
    cleanupService *transcode.CleanupService,
    config *TranscodeCleanupConfig,
) error {
    // Task 1: Policy-based cleanup (failed, old, idle, orphans)
    err := scheduler.RegisterTask(scheduler.Task{
        ID:          "transcode-cleanup-policy",
        Name:        "Transcode Policy Cleanup",
        Description: "Clean failed/old/idle/orphaned transcodes",
        Schedule:    "0 */6 * * *",  // Every 6 hours at :00
        Enabled:     config.Enabled,
        Handler: func(ctx context.Context) error {
            return performPolicyCleanup(ctx, cleanupService, config)
        },
    })
    if err != nil {
        return err
    }

    // Task 2: Disk threshold monitoring
    err = scheduler.RegisterTask(scheduler.Task{
        ID:          "transcode-cleanup-disk-check",
        Name:        "Transcode Disk Monitor",
        Description: "Monitor disk usage and perform LRU cleanup if needed",
        Schedule:    "*/30 * * * *",  // Every 30 minutes
        Enabled:     config.Enabled,
        Handler: func(ctx context.Context) error {
            return performDiskMonitoring(ctx, cleanupService, config)
        },
    })

    return err
}
```

#### 2. Cleanup Logic Refactoring

Extract cleanup logic from `CleanupScheduler` into standalone handler functions:

```go
// internal/application/transcode/cleanup_tasks.go (NEW FILE)

// performPolicyCleanup executes all policy-based cleanup rules
func performPolicyCleanup(
    ctx context.Context,
    svc *CleanupService,
    config *TranscodeCleanupConfig,
) error {
    logger := slog.Default().With("task", "policy-cleanup")

    // 1. Clean failed jobs
    if config.KeepFailedHours > 0 {
        olderThan := time.Duration(config.KeepFailedHours) * time.Hour
        result, err := svc.CleanFailed(ctx, olderThan, false)
        if err != nil {
            logger.Error("failed to clean failed jobs", "error", err)
        } else if result.DeletedCount > 0 {
            logger.Info("cleaned failed jobs",
                "count", result.DeletedCount,
                "size_bytes", result.DeletedSizeBytes)
        }
    }

    // 2. Clean old completed jobs
    if config.MaxAgeHours > 0 {
        olderThan := time.Duration(config.MaxAgeHours) * time.Hour
        result, err := svc.CleanOld(ctx, olderThan, false)
        if err != nil {
            logger.Error("failed to clean old transcodes", "error", err)
        } else if result.DeletedCount > 0 {
            logger.Info("cleaned old transcodes",
                "count", result.DeletedCount,
                "size_bytes", result.DeletedSizeBytes,
                "max_age_hours", config.MaxAgeHours)
        }
    }

    // 3. Clean idle transcodes
    if config.MaxIdleHours > 0 {
        idleSince := time.Now().Add(-time.Duration(config.MaxIdleHours) * time.Hour)
        result, err := cleanIdleTranscodes(ctx, svc, idleSince)
        if err != nil {
            logger.Error("failed to clean idle transcodes", "error", err)
        } else if result.DeletedCount > 0 {
            logger.Info("cleaned idle transcodes",
                "count", result.DeletedCount,
                "size_bytes", result.DeletedSizeBytes,
                "max_idle_hours", config.MaxIdleHours)
        }
    }

    // 4. Clean orphaned files
    result, err := svc.CleanOrphans(ctx, false)
    if err != nil {
        logger.Error("failed to clean orphans", "error", err)
    } else if result.DeletedCount > 0 {
        logger.Info("cleaned orphaned files",
            "count", result.DeletedCount,
            "size_bytes", result.DeletedSizeBytes)
    }

    return nil  // Don't fail the task for individual cleanup errors
}

// performDiskMonitoring checks disk usage and performs LRU cleanup if needed
func performDiskMonitoring(
    ctx context.Context,
    svc *CleanupService,
    config *TranscodeCleanupConfig,
) error {
    logger := slog.Default().With("task", "disk-monitor")

    // Get disk usage
    diskUsage, err := getDiskUsage(svc.outputDir)
    if err != nil {
        return fmt.Errorf("failed to check disk usage: %w", err)
    }

    usagePercent := int(diskUsage.UsedPercent)
    freeSpaceGB := diskUsage.FreeBytes / (1024 * 1024 * 1024)

    logger.Info("disk usage check",
        "used_percent", usagePercent,
        "free_gb", freeSpaceGB,
        "threshold_percent", config.DiskThresholdPercent)

    // Check if cleanup is needed
    needsCleanup := false
    reason := ""

    if usagePercent >= config.DiskThresholdPercent {
        needsCleanup = true
        reason = fmt.Sprintf("disk usage %d%% exceeds threshold %d%%",
            usagePercent, config.DiskThresholdPercent)
    } else if freeSpaceGB < config.MinFreeSpaceGB {
        needsCleanup = true
        reason = fmt.Sprintf("free space %dGB below minimum %dGB",
            freeSpaceGB, config.MinFreeSpaceGB)
    }

    // Perform LRU cleanup if needed
    if needsCleanup {
        logger.Warn("disk threshold exceeded, performing LRU cleanup",
            "reason", reason)
        return performLRUCleanup(ctx, svc, config)
    }

    return nil
}

// performLRUCleanup deletes least-recently-used transcodes
func performLRUCleanup(
    ctx context.Context,
    svc *CleanupService,
    config *TranscodeCleanupConfig,
) error {
    logger := slog.Default().With("task", "lru-cleanup")

    // Get LRU transcodes
    jobs, err := svc.repo.ListByLRU(ctx, config.CleanupBatchSize)
    if err != nil {
        return fmt.Errorf("failed to list LRU transcodes: %w", err)
    }

    if len(jobs) == 0 {
        logger.Info("no LRU transcodes to clean")
        return nil
    }

    // Delete each job
    var deletedCount int
    var deletedBytes int64

    for _, job := range jobs {
        if job == nil {
            continue
        }

        result, err := svc.CleanByMediaID(ctx, job.MediaID, false)
        if err != nil {
            logger.Error("failed to clean transcode",
                "media_id", job.MediaID,
                "quality", job.Quality,
                "error", err)
            continue
        }

        deletedCount += result.DeletedCount
        deletedBytes += result.DeletedSizeBytes
    }

    logger.Info("LRU cleanup completed",
        "deleted_count", deletedCount,
        "deleted_bytes", deletedBytes)

    return nil
}
```

#### 3. Configuration Updates

Keep environment-based configuration, but use cron schedules:

```go
type TranscodeCleanupConfig struct {
    Enabled              bool
    PolicySchedule       string  // "0 */6 * * *" (every 6 hours)
    DiskMonitorSchedule  string  // "*/30 * * * *" (every 30 min)
    DiskThresholdPercent int     // 85
    DiskWarningPercent   int     // 80 (for logging)
    MinFreeSpaceGB       int64   // 10
    MaxAgeHours          int     // 720 (30 days)
    MaxIdleHours         int     // 168 (7 days)
    MaxStorageGB         int64   // 0 (unlimited)
    CleanupBatchSize     int     // 10
    KeepFailedHours      int     // 24
}
```

#### 4. Remove Deprecated Code

Delete the following after migration:

```
internal/application/transcode/cleanup_scheduler.go  ❌ DELETE
```

Keep the following (unchanged):

```
internal/application/transcode/cleanup.go            ✅ KEEP (CleanupService)
internal/application/transcode/cleanup_tasks.go      ✅ NEW (task handlers)
```

### Migration Steps

1. **Create cleanup task handlers** (`cleanup_tasks.go`)
2. **Update container.go** to register tasks with unified scheduler
3. **Remove `CleanupScheduler` from container**
4. **Update configuration to use cron schedules**
5. **Delete `cleanup_scheduler.go`**
6. **Update tests** to use new task pattern
7. **Update documentation** (README, API docs)

## Consequences

### Positive

✅ **Eliminates Duplication**: Single scheduling system for all tasks
✅ **Consistent Management**: All tasks managed through same API
✅ **Better Observability**: Execution history tracked in database
✅ **Flexible Scheduling**: Cron expressions more powerful than fixed intervals
✅ **API Control**: Enable/disable/trigger tasks via REST API
✅ **Follows ADR 007**: Aligns with architectural decision
✅ **Simpler Codebase**: Less code to maintain (remove ~400 lines)

### Negative

⚠️ **Migration Effort**: ~2-3 hours of refactoring work
⚠️ **Breaking Change**: Environment variable changes (minor)
⚠️ **Testing Required**: Ensure cleanup logic still works correctly

### Neutral

🔹 **Configuration Changes**: Interval → cron schedule (more flexible)
🔹 **Task Granularity**: Split into 2 tasks (policy + disk) instead of 1

## Implementation Checklist

### Phase 1: Create New Task Handlers
- [ ] Create `internal/application/transcode/cleanup_tasks.go`
- [ ] Implement `performPolicyCleanup()` function
- [ ] Implement `performDiskMonitoring()` function
- [ ] Implement `performLRUCleanup()` helper
- [ ] Extract `cleanIdleTranscodes()` logic
- [ ] Extract `getDiskUsage()` helper

### Phase 2: Update Container Wiring
- [ ] Add `setupTranscodeCleanupTasks()` in `container.go`
- [ ] Register "transcode-cleanup-policy" task
- [ ] Register "transcode-cleanup-disk-check" task
- [ ] Remove `CleanupScheduler` initialization
- [ ] Remove `CleanupScheduler.Start()` call
- [ ] Update configuration parsing for cron schedules

### Phase 3: Cleanup
- [ ] Delete `internal/application/transcode/cleanup_scheduler.go`
- [ ] Remove `CleanupScheduler` field from container
- [ ] Update `CleanupSchedulerConfig` → `TranscodeCleanupConfig`
- [ ] Remove unused environment variables (if any)

### Phase 4: Testing & Documentation
- [ ] Test policy cleanup task execution
- [ ] Test disk monitoring task execution
- [ ] Test manual task triggers via API
- [ ] Verify cleanup works correctly after migration
- [ ] Update environment variable documentation
- [ ] Update API documentation for scheduler endpoints

### Phase 5: Deployment
- [ ] Add migration notes to CHANGELOG
- [ ] Update README with new cron schedule config
- [ ] Deploy and monitor cleanup execution

## Alternatives Considered

### 1. Keep Both Schedulers
**Pros**: No migration work, no breaking changes
**Cons**: Technical debt, duplicate code, inconsistent management
**Verdict**: ❌ Violates DRY and ADR 007

### 2. Migrate to External Cron (system cron)
**Pros**: OS-level scheduling, simple
**Cons**: Requires shell scripts, no centralized control, harder deployment
**Verdict**: ❌ Less flexible, harder to manage

### 3. Event-Driven Cleanup (no schedule)
**Pros**: Clean on-demand only
**Cons**: Requires manual triggers, could lead to disk exhaustion
**Verdict**: ❌ Not suitable for automated maintenance

## References

- [ADR 007: Unified Task Scheduler](007-unified-task-scheduler.md)
- [ADR 005: On-Demand Transcoding Strategy](005-on-demand-transcoding-strategy.md)
- Current implementation: `internal/application/transcode/cleanup_scheduler.go`
- Unified scheduler: `internal/infrastructure/scheduler/scheduler.go`
- Image cleanup example: `internal/app/container.go:273`

## Notes

**Environment Variable Changes**:

**Before** (fixed interval):
```bash
CLEANUP_INTERVAL=6h
```

**After** (cron schedules):
```bash
CLEANUP_POLICY_SCHEDULE="0 */6 * * *"     # Every 6 hours at :00
CLEANUP_DISK_MONITOR_SCHEDULE="*/30 * * * *"  # Every 30 minutes
```

**Benefits of Cron Over Interval**:
- Run at specific times (e.g., 3 AM daily when load is low)
- More flexible patterns (e.g., weekdays only, monthly)
- Industry-standard syntax
- Aligns with unified scheduler

**Disk Monitoring Strategy**:
- Separate task from policy cleanup
- Runs more frequently (30 min vs 6 hours)
- Only performs LRU cleanup when threshold exceeded
- Lighter-weight check, heavy cleanup when needed
