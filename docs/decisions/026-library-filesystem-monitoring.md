# ADR-026: Library Filesystem Monitoring

## Status

Accepted

## Context

Currently, Viewra discovers new media through scheduled library scans or manual user-initiated scans. This approach has several limitations:

1. **Delayed discovery**: New files aren't detected until the next scheduled scan
2. **Resource intensive**: Full directory walks are expensive, especially for large libraries
3. **Poor user experience**: Users expect media to appear immediately after adding files
4. **Clumsy scheduling**: Cron-based scheduling is inflexible and not user-friendly

Users expect a modern media server to detect new content in real-time, similar to how Plex and Jellyfin handle library updates.

## Decision

Implement a real-time filesystem monitoring system that:

1. **Watches library directories** using OS-native file notification APIs (fsnotify)
2. **Falls back to polling** for network drives where fsnotify doesn't work (NFS, SMB/CIFS)
3. **Integrates with enrichment** by triggering targeted re-enrichment based on what changed
4. **Replaces scheduled scans** as the primary discovery mechanism
5. **Works per-library** with configurable settings

### Key Design Decisions

#### 1. Watch Method Selection

| Storage Type | Watch Method | Reason |
|--------------|--------------|--------|
| Local SSD/HDD | fsnotify | Instant detection, low overhead |
| Network (NFS/SMB) | Polling | fsnotify doesn't work on network mounts |

Detection uses existing `system.Profile.Storage.IsRemote` to determine storage type.

#### 2. Smart Re-enrichment

Instead of re-running the full enrichment pipeline on file changes, we target specific stages based on what changed:

| File Change | Enrichment Action |
|-------------|-------------------|
| New media file (.mkv, .mp4, etc.) | Full pipeline from first stage |
| NFO file added/modified | Re-run `nfo` stage only |
| Image file added/modified (poster.jpg, etc.) | Re-run `local-images` stage only |
| Media file modified | Re-run from first stage |
| File deleted | Mark for cleanup |

This minimizes unnecessary API calls to TMDB/MusicBrainz.

#### 3. Debouncing

To handle bulk file operations (e.g., copying 1000 files):

- **Window**: 5 seconds (configurable)
- **Deduplication**: Multiple events for same file are merged
- **Batching**: Events are processed in batches for efficiency

#### 4. Scan Coordination

Monitoring pauses during active library scans to avoid conflicts:

```
Scan starts → Monitoring pauses → Scan completes → Monitoring resumes
```

This prevents duplicate processing and race conditions.

#### 5. Default Configuration

| Setting | Default | Reason |
|---------|---------|--------|
| Enabled | `true` | On by default for all libraries |
| Priority | `1000` (Interactive) | Immediate enrichment for new files |
| Polling interval | 60 minutes | Balance between responsiveness and I/O for network drives |
| Debounce window | 5 seconds | Prevents overwhelming system during bulk copies |

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    FileMonitorService                        │
│  - Lifecycle management (start/stop with app)               │
│  - Per-library watcher management                           │
│  - Coordinates with active scans                            │
└───────────────────────────┬─────────────────────────────────┘
                            │
            ┌───────────────┼───────────────┐
            │               │               │
            ▼               ▼               ▼
     ┌──────────┐    ┌──────────┐    ┌──────────┐
     │ Library  │    │ Library  │    │ Library  │
     │ Watcher  │    │ Watcher  │    │ Watcher  │
     │ (Local)  │    │ (Network)│    │ (Local)  │
     └────┬─────┘    └────┬─────┘    └────┬─────┘
          │               │               │
          │  fsnotify     │  Polling      │  fsnotify
          │               │  (60 min)     │
          └───────────────┼───────────────┘
                          │
                          ▼
                   ┌──────────────┐
                   │   Debouncer  │
                   └──────┬───────┘
                          │
                          ▼
                   ┌──────────────┐
                   │   Handler    │
                   └──────┬───────┘
                          │
          ┌───────────────┼───────────────┐
          ▼               ▼               ▼
   ┌────────────┐  ┌────────────┐  ┌────────────┐
   │ New Media  │  │  NFO File  │  │ Image File │
   │ → Process  │  │ → nfo stage│  │ → images   │
   │ + Enqueue  │  │            │  │   stage    │
   └────────────┘  └────────────┘  └────────────┘
```

### File Classification

The classifier determines what type of file changed:

**NFO Files:**
- `*.nfo`
- `*-nfo.xml`
- `movie.nfo`, `tvshow.nfo`

**Local Image Files:**
- `poster.jpg/png`, `folder.jpg/png`
- `fanart.jpg/png`, `backdrop.jpg/png`
- `cover.jpg/png`, `album.jpg/png`
- `clearlogo.png`, `banner.jpg/png`
- Season-specific: `season##-poster.jpg`
- Episode thumbnails: `{filename}-thumb.jpg`

**Media Files:**
- Video: `.mkv`, `.mp4`, `.avi`, `.m4v`, `.mov`, `.wmv`, `.flv`, `.webm`
- Audio: `.mp3`, `.flac`, `.m4a`, `.aac`, `.ogg`, `.opus`, `.wav`, `.wma`

### Database Schema

```sql
ALTER TABLE libraries ADD COLUMN monitoring_enabled BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE libraries ADD COLUMN monitoring_config TEXT; -- JSON

-- Example monitoring_config:
-- {"priority": 1000, "polling_interval_minutes": 60, "debounce_seconds": 5}
```

### API Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `GET /api/libraries/{id}/monitoring/status` | GET | Current monitoring status |
| `GET /api/libraries/{id}/monitoring/stream` | SSE | Real-time monitoring events |
| `PATCH /api/libraries/{id}` | PATCH | Update library settings including monitoring |

### SSE Events

```go
EventMonitoringStarted      = "monitoring.started"
EventMonitoringPaused       = "monitoring.paused"
EventMonitoringStopped      = "monitoring.stopped"
EventMonitoringFileDetected = "monitoring.file_detected"
```

### Frontend Changes

1. **Library Card**: Add monitoring indicator (always visible when enabled)
2. **Library Settings Modal**: Toggle monitoring, configure priority/polling
3. **Expanded View**: Show monitoring status, events today, last event time

### What This Replaces

- **Scheduled library scans**: The `scan_all_libraries` scheduler task is removed
- Monitoring is now the primary discovery mechanism
- Manual "Scan" button remains for on-demand full rescans

## Consequences

### Positive

1. **Immediate discovery**: New files appear within seconds (local) or within polling interval (network)
2. **Reduced resource usage**: No more full directory walks on schedule
3. **Better UX**: Users see content as soon as they add it
4. **Targeted enrichment**: Only re-enriches what's needed based on file type
5. **Per-library control**: Users can configure monitoring per library

### Negative

1. **Increased complexity**: More moving parts than simple scheduled scans
2. **inotify limits**: Linux systems may need increased `fs.inotify.max_user_watches`
3. **Network drive latency**: Polling-based detection has inherent delay (60 min default)
4. **Memory usage**: Watchers consume memory for each monitored directory

### Mitigations

1. **inotify limits**: Document how to increase limits; detect and warn if insufficient
2. **Network latency**: Make polling interval configurable; document limitations
3. **Memory**: Use efficient polling that doesn't hold directory trees in memory

## Implementation Plan

### Phase 1: Backend Core
1. Database migration
2. Domain changes (Library entity)
3. Classifier module
4. Debouncer module

### Phase 2: Backend Watcher
5. LibraryWatcher (fsnotify mode)
6. LibraryWatcher (polling mode)
7. EventHandler
8. FileMonitorService

### Phase 3: Backend Integration
9. Container wiring
10. Scan orchestrator integration
11. Library service integration
12. Remove scheduled scan task
13. API endpoints

### Phase 4: Frontend
14. API client regeneration
15. useMonitoringStatus hook
16. MonitoringIndicator component
17. LibrarySettings modal
18. LibraryCard modifications

### Phase 5: Testing
19. Unit tests for classifier, debouncer
20. Integration tests for watcher responsiveness
21. E2E tests for full flow

## References

- [fsnotify library](https://github.com/fsnotify/fsnotify)
- [ADR-025: Resilient Library Scanner V2](./025-resilient-library-scanner-v2.md)
- [Linux inotify limits](https://github.com/fsnotify/fsnotify#watching-a-file-doesnt-work-well)
