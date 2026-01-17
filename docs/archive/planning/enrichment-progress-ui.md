# Library Progress UI - Implementation Plan

## Overview

Expose enrichment queue progress to the frontend as a separate indicator from scan progress. Users should see real-time enrichment status per library (e.g., "47 items enriching") without blocking scan completion.

**Approach:** Use Server-Sent Events (SSE) for real-time updates - migrate all library progress (scan + enrichment) to SSE for consistency.

## Current State

### What Exists

**Backend:**
- `enrichment_queue` table with status tracking (pending, processing, completed, failed, skipped)
- `enrichment_status` table for persistent completion state across sessions
- `StatusRepository.GetLibraryProgress()` - SQL query returning per-stage stats for a library
- `GET /api/enrichment/stats` - Global queue stats (not per-library)
- `GET /api/enrichment/progress` - SSE stream for real-time events (global, all libraries)
- `GET /api/libraries/:id/scan/stream` - SSE stream for scan progress (1-second updates)
- `GET /api/libraries/:id/enrichment/stream` - SSE stream for enrichment progress ✅ (added)
- `GET /api/libraries/:id/enrichment/progress` - REST endpoint for initial state ✅ (added)
- Event bus publishing `enrichment.*` events

**Frontend:**
- Scan progress uses **polling** (`SCAN_POLL_INTERVAL_MS = 5000`) - should migrate to SSE
- `LibraryCard` component displays scan progress bar
- React Query hooks for API calls
- **No SSE infrastructure** - need to create reusable hooks

### What's Missing

1. ~~Per-library SSE stream or library filtering on existing stream~~ ✅ Done
2. ~~Initial state endpoint (SSE only sends deltas)~~ ✅ Done
3. ~~Frontend SSE infrastructure - generic `useSSE` hook with auth support~~ ✅ Done
4. **Migrate scan progress to SSE** - replace polling in LibraryCard (future enhancement)
5. ~~UI component to display enrichment status~~ ✅ Done
6. ~~`statusRepo` dependency in `EnrichmentHandler`~~ ✅ Done

---

## Implementation Steps

### Phase 1: Backend - Library-Scoped SSE

#### 1.0 Add StatusRepository Dependency to EnrichmentHandler

**File:** `internal/api/handlers/enrichment.go`

The handler needs access to `StatusRepository` to query library progress:

```go
type EnrichmentHandler struct {
    manager    *pipeline.Manager
    statusRepo *enrichment.StatusRepository  // Add this
    eventBus   *events.Bus
    logger     *slog.Logger
}

func NewEnrichmentHandler(
    manager *pipeline.Manager,
    statusRepo *enrichment.StatusRepository,  // Add this parameter
    eventBus *events.Bus,
    logger *slog.Logger,
) *EnrichmentHandler {
    return &EnrichmentHandler{
        manager:    manager,
        statusRepo: statusRepo,  // Wire it up
        eventBus:   eventBus,
        logger:     logger,
    }
}
```

**File:** `internal/app/container.go` (or wherever handlers are wired)

Pass the `StatusRepository` when constructing `EnrichmentHandler`.

#### 1.1 Add Library Progress SSE Endpoint

**File:** `internal/api/handlers/enrichment.go`

Add new handler for library-scoped streaming:

```go
// StreamLibraryProgress streams enrichment progress for a specific library via SSE.
// GET /api/libraries/{id}/enrichment/stream
func (h *EnrichmentHandler) StreamLibraryProgress(c *gin.Context) {
    libraryID, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid library ID"})
        return
    }

    // Set SSE headers
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    c.Header("X-Accel-Buffering", "no")

    // Send initial state immediately
    progress, err := h.statusRepo.GetLibraryProgress(c.Request.Context(), libraryID)
    if err == nil {
        initialData := h.buildProgressResponse(libraryID, progress)
        jsonData, _ := json.Marshal(initialData)
        fmt.Fprintf(c.Writer, "event: init\ndata: %s\n\n", jsonData)
        c.Writer.(http.Flusher).Flush()
    }

    // Subscribe to enrichment events
    sub := h.eventBus.Subscribe(
        events.WithBufferSize(100),
        events.WithEventPrefix("enrichment."),
    )
    defer h.eventBus.Unsubscribe(sub)

    clientGone := c.Request.Context().Done()

    for {
        select {
        case <-clientGone:
            return
        case event, ok := <-sub.Events():
            if !ok {
                return
            }

            // Filter events by library ID
            if eventLibraryID, ok := event.Data["library_id"].(int64); ok {
                if eventLibraryID != libraryID {
                    continue // Skip events for other libraries
                }
            }

            // Send filtered event
            jsonData, _ := json.Marshal(event.Data)
            fmt.Fprintf(c.Writer, "event: update\ndata: %s\n\n", jsonData)
            c.Writer.(http.Flusher).Flush()
        }
    }
}

func (h *EnrichmentHandler) buildProgressResponse(libraryID int64, progress map[string]*enrichment.QueueStats) *EnrichmentProgressResponse {
    var totalPending, totalProcessing, totalCompleted, totalFailed int64
    for _, stats := range progress {
        totalPending += stats.PendingCount
        totalProcessing += stats.ProcessingCount
        totalCompleted += stats.CompletedCount
        totalFailed += stats.FailedCount
    }

    return &EnrichmentProgressResponse{
        LibraryID:       libraryID,
        StageProgress:   progress,
        TotalPending:    totalPending,
        TotalProcessing: totalProcessing,
        TotalCompleted:  totalCompleted,
        TotalFailed:     totalFailed,
        IsActive:        totalPending > 0 || totalProcessing > 0,
    }
}
```

#### 1.2 Add Initial State Endpoint (for SSE reconnection)

**File:** `internal/api/handlers/enrichment.go`

```go
// GetLibraryProgress returns current enrichment progress snapshot.
// GET /api/libraries/{id}/enrichment/progress
func (h *EnrichmentHandler) GetLibraryProgress(c *gin.Context) {
    libraryID, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid library ID"})
        return
    }

    progress, err := h.statusRepo.GetLibraryProgress(c.Request.Context(), libraryID)
    if err != nil {
        h.logger.Error("Failed to get library enrichment progress", "error", err)
        c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get progress"})
        return
    }

    c.JSON(http.StatusOK, h.buildProgressResponse(libraryID, progress))
}
```

#### 1.3 Ensure Events Include Library ID

**File:** `internal/application/enrichment/pipeline/job_processor.go`

When publishing events, include `library_id`:

```go
h.eventBus.Publish(events.Event{
    Type: "enrichment.completed",
    Data: map[string]any{
        "media_id":    job.MediaID,
        "media_type":  job.MediaType,
        "stage":       job.Stage,
        "library_id":  libraryID,  // Add this
    },
})
```

#### 1.4 Register Routes

**File:** `internal/api/routes/library.go`

```go
libraries.GET("/:id/enrichment/progress", h.Enrichment.GetLibraryProgress)
libraries.GET("/:id/enrichment/stream", h.Enrichment.StreamLibraryProgress)
```

---

### Phase 2: Frontend - SSE Hook & Component

#### 2.1 Create SSE Hook

**File:** `web/src/hooks/useEnrichmentSSE.ts`

```typescript
import { useState, useEffect, useCallback } from 'react'

interface EnrichmentProgress {
  library_id: number
  stage_progress: Record<string, StageStats>
  total_pending: number
  total_processing: number
  total_completed: number
  total_failed: number
  is_active: boolean
}

interface StageStats {
  stage: string
  pending_count: number
  processing_count: number
  completed_count: number
  failed_count: number
  skipped_count: number
  total_count: number
}

export function useEnrichmentSSE(libraryId: number) {
  const [progress, setProgress] = useState<EnrichmentProgress | null>(null)
  const [connected, setConnected] = useState(false)

  useEffect(() => {
    if (libraryId <= 0) return

    const eventSource = new EventSource(
      `/api/libraries/${libraryId}/enrichment/stream`
    )

    eventSource.addEventListener('init', (e) => {
      setProgress(JSON.parse(e.data))
      setConnected(true)
    })

    eventSource.addEventListener('update', (e) => {
      const update = JSON.parse(e.data)
      setProgress((prev) => {
        if (!prev) return prev
        // Merge update into current state
        // For now, refetch full state on any update
        return { ...prev, ...update }
      })
    })

    eventSource.onerror = () => {
      setConnected(false)
      // EventSource auto-reconnects
    }

    return () => {
      eventSource.close()
    }
  }, [libraryId])

  return { progress, connected }
}
```

#### 2.2 Create EnrichmentIndicator Component

**File:** `web/src/components/library/EnrichmentIndicator/EnrichmentIndicator.tsx`

```typescript
import { Sparkles } from 'lucide-react'
import { useEnrichmentSSE } from '@/hooks/useEnrichmentSSE'

interface EnrichmentIndicatorProps {
  libraryId: number
}

export function EnrichmentIndicator({ libraryId }: EnrichmentIndicatorProps) {
  const { progress } = useEnrichmentSSE(libraryId)

  if (!progress?.is_active) return null

  const total =
    progress.total_pending +
    progress.total_processing +
    progress.total_completed +
    progress.total_failed

  return (
    <div className="flex items-center gap-2 text-sm text-muted-foreground">
      <Sparkles className="h-4 w-4 animate-pulse text-amber-500" />
      <span>
        Enriching: {progress.total_completed}/{total}
      </span>
      {progress.total_failed > 0 && (
        <span className="text-destructive">
          ({progress.total_failed} failed)
        </span>
      )}
    </div>
  )
}

export default EnrichmentIndicator
```

#### 2.3 Integrate into LibraryCard

**File:** `web/src/components/library/LibraryCard/LibraryCard.tsx`

```typescript
import { EnrichmentIndicator } from '../EnrichmentIndicator/EnrichmentIndicator'

// Inside the component, after scan progress section:
{/* Enrichment Progress */}
<EnrichmentIndicator libraryId={library.id} />
```

---

### Phase 3: Polish

#### 3.1 Graceful Degradation

If SSE connection fails, fall back to polling with React Query:

```typescript
export function useEnrichmentProgress(libraryId: number) {
  const sse = useEnrichmentSSE(libraryId)

  // Fallback polling if SSE disconnected for too long
  const { data: polledData } = useQuery({
    queryKey: ['enrichment-progress', libraryId],
    queryFn: () => fetch(`/api/libraries/${libraryId}/enrichment/progress`).then(r => r.json()),
    enabled: !sse.connected && libraryId > 0,
    refetchInterval: 10000,
  })

  return sse.connected ? sse.progress : polledData
}
```

#### 3.2 Stage Breakdown Tooltip (Optional)

Hover to show per-stage progress:

```
Enriching: 2,847/5,000
  ├─ NFO Files: 5,000/5,000 ✓
  ├─ TMDb: 2,847/5,000 (processing)
  └─ Local Images: 0/5,000 (pending)
```

#### 3.3 Error Details Link (Optional)

Link to view failed items with error categories.

---

## File Changes Summary

| File | Change |
|------|--------|
| `internal/api/handlers/enrichment.go` | Add `statusRepo` field, `GetLibraryProgress`, `StreamLibraryProgress` |
| `internal/app/container.go` | Wire `StatusRepository` into `EnrichmentHandler` |
| `internal/api/routes/library.go` | Register new endpoints |
| `internal/application/enrichment/pipeline/job_processor.go` | Add `library_id` to events |
| `web/src/hooks/useEnrichmentSSE.ts` | New file - SSE hook |
| `web/src/components/library/EnrichmentIndicator/` | New component |
| `web/src/components/library/LibraryCard/LibraryCard.tsx` | Integrate indicator |

---

## Existing Infrastructure

### Event Bus (already exists)

**File:** `internal/infrastructure/events/bus.go`

- Pub/sub system with buffering
- Supports event prefix filtering (`enrichment.*`)
- Replay capability for late joiners

### SSE Handler Pattern (already exists)

**File:** `internal/api/handlers/enrichment.go:55-121`

```go
func (h *EnrichmentHandler) StreamProgress(c *gin.Context) {
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")

    sub := h.eventBus.Subscribe(
        events.WithBufferSize(100),
        events.WithEventPrefix("enrichment."),
    )
    defer h.eventBus.Unsubscribe(sub)

    for {
        select {
        case <-c.Request.Context().Done():
            return
        case event := <-sub.Events():
            fmt.Fprintf(c.Writer, "event: enrichment\ndata: %s\n\n", jsonData)
            c.Writer.(http.Flusher).Flush()
        }
    }
}
```

### SQL Query (already exists)

**File:** `internal/infrastructure/database/queries/sqlite/enrichment_status.sql`

`GetLibraryEnrichmentProgress` returns per-stage stats for a library.

### Repository Method (already exists)

**File:** `internal/infrastructure/persistence/enrichment/status_repository.go`

```go
func (r *StatusRepository) GetLibraryProgress(ctx context.Context, libraryID int64) (map[string]*enrichment.QueueStats, error)
```

---

## Why SSE Over Polling

1. **Real-time updates** - Users see enrichment progress immediately
2. **Lower server load** - No repeated API calls every 5 seconds
3. **Existing infrastructure** - Event bus and SSE patterns already implemented
4. **Consistency** - Enrichment already publishes events; just need to expose them per-library
5. **Better UX** - Progress updates smoothly instead of jumping every poll interval
