# ADR 007: Unified Task Scheduler System

**Status**: Proposed
**Date**: 2025-11-16
**Author**: ViewRA Team

## Context

ViewRA requires various periodic maintenance tasks:

### Current/Planned Scheduled Tasks

1. **Transcode Cleanup** (Phase 3)
   - Delete old/unused transcoded files
   - Clean by access time, age, or size
   - Prevent unbounded disk usage

2. **Image Cache Cleanup** (Phase 4.1)
   - Remove orphaned cache files (no DB reference)
   - Delete unused resized images
   - Cleanup missing local file references

3. **Library Health Checks** (Future)
   - Verify media files still exist
   - Check for file corruption
   - Update metadata for changed files

4. **Database Maintenance** (Future)
   - Vacuum SQLite database
   - Update statistics
   - Prune old watch progress

5. **Log Rotation** (Future)
   - Archive old log files
   - Compress historical logs
   - Delete aged logs

### Problems with Ad-Hoc Scheduling

- **Scattered Code**: Each cleanup in different module
- **No Central Control**: Can't enable/disable tasks globally
- **No Visibility**: Can't see what's scheduled or when
- **No Manual Triggers**: Can't run cleanup on-demand
- **Configuration Hell**: Each task manages own schedule

## Decision

Implement a **unified, configurable task scheduler** with the following design:

### 1. Scheduler Architecture

```go
type Task struct {
    ID          string                                    // Unique identifier
    Name        string                                    // Human-readable name
    Description string                                    // What this task does
    Schedule    string                                    // Cron expression
    Handler     func(ctx context.Context) error          // Task logic
    Enabled     bool                                      // Can be disabled
    LastRun     *time.Time                                // Last execution time
    NextRun     *time.Time                                // Next scheduled run
    LastError   error                                     // Last execution error
}

type Scheduler struct {
    cron   *cron.Cron                                     // Underlying cron engine
    tasks  map[string]*Task                               // Registered tasks
    logger *slog.Logger                                   // Structured logging
}
```

**Key Features**:
- Built on `github.com/robfig/cron/v3` (battle-tested cron library)
- Thread-safe task registration and management
- Timezone-aware scheduling (configurable location)
- Automatic logging of task execution
- Error tracking per task

### 2. Task Registration Pattern

```go
// During application startup
scheduler := scheduler.New(config, logger)

// Register transcode cleanup
scheduler.RegisterTask(scheduler.Task{
    ID:          "transcode-cleanup",
    Name:        "Transcode Cleanup",
    Description: "Delete old transcoded files by access time",
    Schedule:    "0 3 * * *",  // 3 AM daily
    Enabled:     true,
    Handler: func(ctx context.Context) error {
        return transcodeCleanupUC.Execute(ctx, 7*24*time.Hour) // 7 days
    },
})

// Register image cache cleanup
scheduler.RegisterTask(scheduler.Task{
    ID:          "image-cache-cleanup",
    Name:        "Image Cache Cleanup",
    Description: "Remove orphaned image cache files",
    Schedule:    "0 3 * * *",  // 3 AM daily
    Enabled:     true,
    Handler: func(ctx context.Context) error {
        return imageCleanupUC.CleanOrphanedImages(ctx)
    },
})

// Register database vacuum (weekly on Sunday at 4 AM)
scheduler.RegisterTask(scheduler.Task{
    ID:          "database-vacuum",
    Name:        "Database Maintenance",
    Description: "Vacuum and optimize SQLite database",
    Schedule:    "0 4 * * 0",  // 4 AM Sunday
    Enabled:     true,
    Handler: func(ctx context.Context) error {
        return dbMaintenanceUC.Vacuum(ctx)
    },
})

// Start scheduler (blocks until context canceled)
go scheduler.Start(ctx)
```

### 3. Configuration

**Environment Variables** (`.env`):
```bash
# Scheduler Settings
SCHEDULER_ENABLED=true
SCHEDULER_TIMEZONE=America/New_York  # or "UTC", "Local"

# Task-specific schedules (override defaults)
TASK_TRANSCODE_CLEANUP_SCHEDULE="0 3 * * *"
TASK_TRANSCODE_CLEANUP_ENABLED=true

TASK_IMAGE_CLEANUP_SCHEDULE="0 3 * * *"
TASK_IMAGE_CLEANUP_ENABLED=true

TASK_DATABASE_VACUUM_SCHEDULE="0 4 * * 0"
TASK_DATABASE_VACUUM_ENABLED=true
```

**Database-backed Configuration** (Phase 4.3+):
```sql
CREATE TABLE scheduled_tasks (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    schedule TEXT NOT NULL,  -- Cron expression
    enabled BOOLEAN DEFAULT true,
    last_run DATETIME,
    last_error TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

This allows runtime configuration via admin UI.

### 4. Admin API Endpoints

```
GET    /api/admin/scheduler/tasks           # List all scheduled tasks
GET    /api/admin/scheduler/tasks/:id       # Get task details
POST   /api/admin/scheduler/tasks/:id/run   # Run task immediately
PUT    /api/admin/scheduler/tasks/:id       # Update task (schedule, enabled)
POST   /api/admin/scheduler/tasks/:id/enable
POST   /api/admin/scheduler/tasks/:id/disable
```

**Response Example**:
```json
{
  "tasks": [
    {
      "id": "transcode-cleanup",
      "name": "Transcode Cleanup",
      "description": "Delete old transcoded files by access time",
      "schedule": "0 3 * * *",
      "enabled": true,
      "last_run": "2025-11-16T03:00:00Z",
      "next_run": "2025-11-17T03:00:00Z",
      "last_error": null
    },
    {
      "id": "image-cache-cleanup",
      "name": "Image Cache Cleanup",
      "description": "Remove orphaned image cache files",
      "schedule": "0 3 * * *",
      "enabled": true,
      "last_run": "2025-11-16T03:00:00Z",
      "next_run": "2025-11-17T03:00:00Z",
      "last_error": null
    }
  ]
}
```

### 5. Cron Expression Format

Using standard cron syntax:
```
┌───────────── minute (0 - 59)
│ ┌───────────── hour (0 - 23)
│ │ ┌───────────── day of month (1 - 31)
│ │ │ ┌───────────── month (1 - 12)
│ │ │ │ ┌───────────── day of week (0 - 6) (Sunday to Saturday)
│ │ │ │ │
│ │ │ │ │
* * * * *
```

**Common Examples**:
- `0 3 * * *` - Daily at 3:00 AM
- `0 */6 * * *` - Every 6 hours
- `0 4 * * 0` - Sundays at 4:00 AM
- `*/15 * * * *` - Every 15 minutes
- `0 0 1 * *` - Monthly on 1st at midnight

### 6. Task Execution Guarantees

**What the scheduler provides**:
- Tasks run at scheduled times (best effort)
- Tasks are logged (start, completion, errors)
- Failed tasks don't crash scheduler
- Tasks can be disabled without code changes

**What the scheduler does NOT provide**:
- **No persistence**: If server crashes mid-task, it won't resume
- **No retries**: Failed tasks wait until next schedule
- **No distributed scheduling**: Single-server only
- **No job queue**: Not for long-running async jobs

For long-running jobs (like full library rescans), use separate job queue system.

### 7. Integration with Existing Cleanup

**Transcode Cleanup** (already exists):
```go
// internal/application/transcode/cleanup_scheduler.go
// Currently: Standalone with own ticker
// Future: Register with unified scheduler
```

**Image Cleanup** (to implement):
```go
// internal/application/images/cleanup.go
type CleanupUseCase struct {
    imageRepo images.Repository
    logger    *slog.Logger
}

func (uc *CleanupUseCase) CleanOrphanedImages(ctx context.Context) error {
    // 1. Get all hashes from database
    dbHashes := uc.imageRepo.GetAllFileHashes(ctx)
    hashSet := toSet(dbHashes)

    // 2. Scan cache directory
    cacheDir := "data/cache/images/"
    cacheFiles, _ := filepath.Glob(filepath.Join(cacheDir, "*"))

    var deleted, errors int
    for _, file := range cacheFiles {
        hash := extractHashFromFilename(file) // e.g., "abc123_300x450.jpg" -> "abc123"

        if !hashSet[hash] {
            // Orphaned cache file - not referenced in DB
            if err := os.Remove(file); err != nil {
                uc.logger.Warn("Failed to delete orphaned cache file",
                    "path", file, "error", err)
                errors++
                continue
            }
            deleted++
        }
    }

    uc.logger.Info("Image cache cleanup completed",
        "deleted", deleted,
        "errors", errors)

    return nil
}
```

## Consequences

### Positive

- **Centralized Management**: All scheduled tasks in one place
- **User Control**: Admin can enable/disable tasks without code deploy
- **Visibility**: See what's scheduled, when it ran, if it failed
- **Manual Triggers**: Run cleanup on-demand (before maintenance, testing)
- **Consistent Logging**: All tasks log in same format
- **Easy Addition**: New tasks = register, no new infrastructure
- **Timezone Aware**: Respects server timezone or configured location

### Negative

- **New Dependency**: `github.com/robfig/cron/v3`
- **Not Distributed**: Single server only (fine for MVP)
- **Memory Overhead**: Minimal (one goroutine per task)
- **No Retry Logic**: Failed tasks wait until next schedule

### Risks

- **Resource Contention**: Multiple heavy tasks at 3 AM
  - **Mitigation**: Stagger schedules (3:00, 3:15, 3:30)

- **Long-Running Tasks**: Cleanup takes longer than interval
  - **Mitigation**: Use separate job queue for heavy tasks

- **Configuration Drift**: Tasks in code vs database
  - **Mitigation**: Code is source of truth, DB overrides

## Alternatives Considered

### A. Keep Ad-Hoc Scheduling
**Rejected**: Doesn't scale, no visibility, scattered configuration

### B. External Cron (systemd timers, crontab)
**Rejected**: Requires external config, no runtime control, poor logging

### C. Job Queue (e.g., `asynq`, `gocraft/work`)
**Rejected**: Overkill for simple scheduled tasks, adds Redis dependency

### D. APScheduler-style (Python)
**Rejected**: Go has excellent cron libraries, no need to reinvent

## Implementation Plan

**UPDATED 2025-11-16**: Revised phasing based on plugin architecture decision

### Phase 4.2: Core Scheduler Implementation (Week 1)
**Status**: In Progress

**Scope**: Build unified task scheduler without external API dependencies

1. **Scheduler Infrastructure** (2 days)
   - Implement `Scheduler` using `robfig/cron/v3`
   - Task registration system
   - Thread-safe task management
   - Graceful shutdown handling

2. **Database Schema** (0.5 days)
   - Migration for `scheduled_tasks` table (execution history)
   - Migration for `task_executions` table (logs)
   - SQLC queries for task CRUD

3. **Admin API Endpoints** (1 day)
   - `GET /api/admin/scheduler/tasks` - List registered tasks
   - `POST /api/admin/scheduler/tasks/:id/trigger` - Manual trigger
   - `GET /api/admin/scheduler/tasks/:id/history` - Execution history

4. **Frontend UI** (1 day)
   - Route: `/settings/scheduler`
   - Task list table (name, schedule, last run, status)
   - "Run Now" buttons
   - Simple execution history view

5. **Integration** (0.5 days)
   - Register existing cleanup tasks (transcode, image)
   - Wire into application container
   - Update startup sequence

**Estimated Timeline**: 3-5 days

**Success Criteria**:
- Scheduler starts with application
- Tasks execute on schedule
- Manual triggers work via API and UI
- Execution history persists and displays
- Graceful shutdown (tasks complete before exit)

### Phase 4.3: Image Cache & Transformations
**Deferred**: Image caching implementation (see ADR 006)

When implemented, image cache cleanup task will register with existing scheduler:
```go
scheduler.RegisterTask(Task{
    ID:          "image-cache-cleanup",
    Name:        "Image Cache Cleanup",
    Description: "Remove orphaned image cache files",
    Schedule:    "0 3 * * *",
    Handler:     imageCleanupUC.Execute,
})
```

### Phase 7: Plugin System
**Deferred**: External API integrations (TMDb, MusicBrainz) as plugins

Plugins will register their own scheduled tasks:
```go
// Example: TMDb metadata refresh plugin
scheduler.RegisterTask(Task{
    ID:          "tmdb-metadata-refresh",
    Name:        "TMDb Metadata Refresh",
    Description: "Update movie metadata from TMDb",
    Schedule:    "0 2 * * 0", // Weekly
    Handler:     tmdbPlugin.RefreshMetadata,
})
```

### Future Enhancements
- Database-backed task configuration (edit schedules via UI)
- Task dependencies (run B after A completes)
- Email/webhook notifications on failure
- Retry logic for failed tasks
- Additional maintenance tasks: library health, DB vacuum, log rotation

## References

- Cron Library: https://github.com/robfig/cron
- Cron Expression Format: https://crontab.guru/
- Related ADR: [006-image-handling-strategy.md](006-image-handling-strategy.md)
