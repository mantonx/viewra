# Real-Time Updates Guide

This guide explains how ViewRA implements real-time updates using Server-Sent Events (SSE) and the internal Event Bus. This enables live progress tracking for library scanning, media enrichment, and other long-running operations.

## Architecture Overview

```text
┌─────────────────────────────────────────────────────────────┐
│                      ViewRA Backend                         │
│                                                             │
│  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐       │
│  │   Scanner   │   │  Enrichment │   │  Transcode  │       │
│  │             │   │   Pipeline  │   │   Worker    │       │
│  └──────┬──────┘   └──────┬──────┘   └──────┬──────┘       │
│         │                 │                 │               │
│         └────────────────┬┴─────────────────┘               │
│                          ▼                                  │
│  ┌───────────────────────────────────────────────────────┐ │
│  │                     Event Bus                          │ │
│  │  - Pub/Sub communication                               │ │
│  │  - Ring buffer for replay                              │ │
│  │  - Non-blocking delivery                               │ │
│  └──────────────────────┬────────────────────────────────┘ │
│                         │                                   │
│  ┌──────────────────────┴────────────────────────────────┐ │
│  │                  SSE Endpoints                         │ │
│  │  GET /api/libraries/{id}/enrichment/stream             │ │
│  │  GET /api/libraries/{id}/scan/stream                   │ │
│  └──────────────────────┬────────────────────────────────┘ │
└─────────────────────────┼───────────────────────────────────┘
                          │ SSE Stream
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                     Frontend (React)                         │
│                                                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                      useSSE Hook                       │  │
│  │  - Fetch-based SSE (supports auth headers)             │  │
│  │  - Auto-reconnect with backoff                         │  │
│  │  - Event type filtering                                │  │
│  └──────────────────────┬────────────────────────────────┘  │
│                         │                                    │
│  ┌──────────────────────┴────────────────────────────────┐  │
│  │                    UI Components                       │  │
│  │  - Progress bars                                       │  │
│  │  - Live status indicators                              │  │
│  │  - Activity feeds                                      │  │
│  └───────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

## Event Bus

The Event Bus is the central pub/sub mechanism for all real-time communication.

### Creating the Bus

```go
import "github.com/mantonx/viewra/internal/infrastructure/events"

// Create with 10,000 event ring buffer
bus := events.NewBus(10000, logger)
```

### Publishing Events

Events are published using a fluent builder API:

```go
// Simple event
bus.Publish(events.NewEvent(events.EventScanStarted, "scanner").
    WithLibraryID(libraryID).
    Build())

// Progress event
bus.Publish(events.NewEvent(events.EventScanProgress, "scanner").
    WithLibraryID(libraryID).
    WithProgress(current, total).
    WithData("file", currentFile).
    Build())

// Error event
bus.Publish(events.NewEvent(events.EventEnrichmentFailed, "worker").
    WithMediaID(mediaID).
    WithLibraryID(libraryID).
    WithError(err).
    Build())
```

### Event Types

ViewRA defines these event types:

| Category | Event Type | Description |
|----------|------------|-------------|
| **Media** | `media.discovered` | New media file found |
| | `media.updated` | Media metadata changed |
| | `media.removed` | Media file deleted |
| **Enrichment** | `enrichment.queued` | Job added to queue |
| | `enrichment.started` | Worker picked up job |
| | `enrichment.stage_complete` | Stage finished |
| | `enrichment.complete` | All stages done |
| | `enrichment.failed` | Stage failed |
| | `enrichment.skipped` | Stage skipped |
| **Scan** | `scan.started` | Library scan began |
| | `scan.progress` | Files processed |
| | `scan.completed` | Scan finished |
| | `scan.failed` | Scan error |
| **Transcode** | `transcode.started` | Transcoding began |
| | `transcode.progress` | Encoding progress |
| | `transcode.completed` | File ready |
| | `transcode.failed` | Encoding error |
| **Plugin** | `plugin.loaded` | Plugin initialized |
| | `plugin.unloaded` | Plugin stopped |
| | `plugin.crashed` | Plugin failed |
| | `plugin.health_update` | Health status changed |

### Subscribing to Events

Components subscribe with optional filtering:

```go
// Subscribe to all events
sub := bus.Subscribe()

// Subscribe to specific event types
sub := bus.Subscribe(
    events.WithEventTypes(events.EventEnrichmentStarted, events.EventEnrichmentComplete),
)

// Subscribe to events by prefix
sub := bus.Subscribe(
    events.WithEventPrefix("enrichment."),
)

// Subscribe with custom filter
sub := bus.Subscribe(
    events.WithFilter(func(e events.Event) bool {
        libraryID, ok := e.Data["library_id"].(int64)
        return ok && libraryID == targetLibraryID
    }),
)

// Read events from channel
for event := range sub.Events() {
    processEvent(event)
}

// Always unsubscribe when done
defer bus.Unsubscribe(sub)
```

### Event Replay

New subscribers can replay recent events:

```go
// Replay last 100 events to catch up
sub := bus.Subscribe(
    events.WithReplayLast(100),
    events.WithEventPrefix("scan."),
)
```

## SSE Backend Implementation

### Handler Structure

SSE handlers follow this pattern:

```go
func (h *Handler) StreamProgress(c *gin.Context) {
    libraryID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

    // 1. Set SSE headers
    sse.SetHeaders(c)

    // 2. Send initial state
    progress, _ := h.repo.GetProgress(c.Request.Context(), libraryID)
    jsonData, _ := json.Marshal(progress)
    fmt.Fprintf(c.Writer, "event: init\ndata: %s\n\n", jsonData)
    sse.Flush(c)

    // 3. Subscribe to events
    sub := h.eventBus.Subscribe(
        events.WithBufferSize(100),
        events.WithEventPrefix("enrichment."),
    )
    defer h.eventBus.Unsubscribe(sub)

    // 4. Stream loop
    clientGone := c.Request.Context().Done()
    for {
        select {
        case <-clientGone:
            return
        case event, ok := <-sub.Events():
            if !ok {
                return
            }

            // Filter by library
            eventLibraryID, ok := sse.GetInt64FromEvent(event, "library_id")
            if !ok || eventLibraryID != libraryID {
                continue
            }

            // Fetch fresh data and send
            progress, _ := h.repo.GetProgress(c.Request.Context(), libraryID)
            jsonData, _ := json.Marshal(progress)
            fmt.Fprintf(c.Writer, "event: update\ndata: %s\n\n", jsonData)
            sse.Flush(c)
        }
    }
}
```

### SSE Protocol

Events follow the standard SSE format:

```
event: init
data: {"total": 100, "completed": 0}

event: update
data: {"total": 100, "completed": 42}

event: complete
data: {"total": 100, "completed": 100, "status": "done"}
```

### Headers

```go
func SetHeaders(c *gin.Context) {
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    c.Header("X-Accel-Buffering", "no")  // Disable nginx buffering
}
```

## Frontend Integration

### The useSSE Hook

ViewRA provides a generic SSE hook that handles authentication, reconnection, and parsing:

```typescript
import { useSSE } from '@/lib/hooks/useSSE'

interface EnrichmentProgress {
  library_id: number
  total_pending: number
  total_completed: number
  overall_progress: {
    percentage: number
  }
}

const { connectionState, lastEvent, error, reconnect } = useSSE<EnrichmentProgress>(
  `/api/libraries/${libraryId}/enrichment/stream`,
  {
    enabled: isVisible,
    onEvent: (event) => {
      console.log('Progress update:', event.overall_progress.percentage)
    },
    eventTypes: ['init', 'update'],
    reconnectDelay: 3000,
    maxReconnectAttempts: 5,
  }
)
```

### Hook Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | `boolean` | `true` | Enable/disable connection |
| `onEvent` | `(event: T) => void` | - | Event callback |
| `onStateChange` | `(state) => void` | - | State change callback |
| `onError` | `(error) => void` | - | Error callback |
| `reconnectDelay` | `number` | `3000` | Reconnect delay (ms) |
| `maxReconnectAttempts` | `number` | `5` | Max reconnects (0 = infinite) |
| `eventTypes` | `string[]` | all | Filter event types |

### Connection States

```typescript
type SSEConnectionState = 'connecting' | 'connected' | 'disconnected' | 'error'
```

### Why Fetch Instead of EventSource?

The hook uses `fetch` with streaming instead of the native `EventSource` API because:

1. **Authentication**: `EventSource` doesn't support custom headers
2. **Control**: Better control over connection lifecycle
3. **Error handling**: More detailed error information

```typescript
// This is what useSSE does internally
const response = await fetch(url, {
  headers: {
    Authorization: `Bearer ${token}`,
    Accept: 'text/event-stream',
    'Cache-Control': 'no-cache',
  },
  signal: abortController.signal,
})

const reader = response.body.getReader()
const decoder = new TextDecoder()

while (true) {
  const { done, value } = await reader.read()
  if (done) break

  const chunk = decoder.decode(value, { stream: true })
  // Parse SSE events from chunk...
}
```

## Example: Library Enrichment Progress

### Backend

```go
// internal/api/handlers/enrichment.go

func (h *EnrichmentHandler) StreamLibraryProgress(c *gin.Context) {
    libraryID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

    sse.SetHeaders(c)

    // Send initial snapshot
    progress, _ := h.statusRepo.GetLibraryProgress(c.Request.Context(), libraryID)
    currentItem, _ := h.queueRepo.GetCurrentItem(c.Request.Context(), libraryID)
    overallProgress, _ := h.statusRepo.GetOverallProgress(c.Request.Context(), libraryID)

    initialData := h.buildProgressResponse(libraryID, progress, currentItem, overallProgress)
    jsonData, _ := json.Marshal(initialData)
    fmt.Fprintf(c.Writer, "event: init\ndata: %s\n\n", jsonData)
    sse.Flush(c)

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

            // Filter to this library only
            eventLibraryID, ok := sse.GetInt64FromEvent(event, "library_id")
            if !ok || eventLibraryID != libraryID {
                continue
            }

            // Refresh and send progress
            progress, _ := h.statusRepo.GetLibraryProgress(c.Request.Context(), libraryID)
            currentItem, _ := h.queueRepo.GetCurrentItem(c.Request.Context(), libraryID)
            overallProgress, _ := h.statusRepo.GetOverallProgress(c.Request.Context(), libraryID)

            updateData := h.buildProgressResponse(libraryID, progress, currentItem, overallProgress)
            jsonData, _ := json.Marshal(updateData)
            fmt.Fprintf(c.Writer, "event: update\ndata: %s\n\n", jsonData)
            sse.Flush(c)
        }
    }
}
```

### Frontend

```tsx
// components/LibraryProgress.tsx

const LibraryProgress: React.FC<{ libraryId: number }> = ({ libraryId }) => {
  const [progress, setProgress] = useState<EnrichmentProgress | null>(null)

  const { connectionState, error } = useSSE<EnrichmentProgress>(
    `/api/libraries/${libraryId}/enrichment/stream`,
    {
      enabled: true,
      onEvent: setProgress,
    }
  )

  if (error) {
    return <div className="text-red-500">Connection error: {error.message}</div>
  }

  if (!progress) {
    return <div>Loading...</div>
  }

  return (
    <div>
      <div className="flex items-center gap-2">
        <span className={connectionState === 'connected' ? 'text-green-500' : 'text-yellow-500'}>
          {connectionState === 'connected' ? '●' : '○'}
        </span>
        <span>{connectionState}</span>
      </div>

      <div className="mt-4">
        <div className="h-2 bg-gray-200 rounded">
          <div
            className="h-2 bg-blue-500 rounded"
            style={{ width: `${progress.overall_progress.percentage}%` }}
          />
        </div>
        <div className="mt-1 text-sm">
          {progress.total_completed} / {progress.total_pending + progress.total_completed} items
        </div>
      </div>
    </div>
  )
}
```

## Best Practices

### Backend

1. **Always send initial state**: Don't make clients wait for the first event
2. **Filter by relevant ID**: Only send events the client cares about
3. **Send complete snapshots**: Clients may miss events; send full state on each update
4. **Use appropriate buffer sizes**: Balance between memory and missed events
5. **Clean up subscriptions**: Always unsubscribe when the handler exits

### Frontend

1. **Use the `enabled` flag**: Connect only when the component is visible
2. **Handle all states**: Show appropriate UI for connecting, connected, disconnected, error
3. **Implement reconnection**: The hook handles this, but show reconnection status
4. **Avoid memory leaks**: The hook cleans up on unmount

### Performance

1. **Rate limit events**: Don't publish faster than the UI can update
2. **Batch updates**: Group multiple changes into single events when possible
3. **Use event filtering**: Subscribe only to needed event types
4. **Monitor buffer overflow**: Log when events are dropped

## Debugging

### Enable Event Logging

```go
// Create bus with logger for debugging
bus := events.NewBus(10000, logger.With("component", "eventbus"))
```

### Check Bus Statistics

```go
stats := bus.Stats()
logger.Info("Event bus stats",
    "subscribers", stats.SubscriberCount,
    "buffer_count", stats.BufferCount,
    "total_published", stats.TotalPublished)
```

### Browser DevTools

1. Open Network tab
2. Filter by "EventStream" type
3. Watch the "EventStream" tab for incoming events

## See Also

- [Enrichment Pipeline Guide](./ENRICHMENT_PIPELINE.md)
- [Plugin Development Guide](./PLUGIN_DEVELOPMENT.md)
- [ADR 027: Plugin System Architecture](../decisions/027-plugin-system-architecture.md) - Event Bus section
