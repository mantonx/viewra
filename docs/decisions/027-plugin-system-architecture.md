# ADR 027: Plugin System Architecture

## Status

Proposed

## Date

December 2, 2025

## Context

ViewRA needs an extensible plugin system that:

1. Supports multiple metadata sources (TMDb, MusicBrainz, NFO files)
2. Enables notifications, analytics, and other integrations
3. Provides isolation and security boundaries
4. Allows community contributions without core code changes
5. Maintains consistent interfaces across all plugin types

This ADR documents 64 architectural decisions made through systematic Q&A.

## Decision

Implement a plugin system using Hashicorp's go-plugin library with gRPC for communication, backed by an Event Bus for observability and an Enrichment Queue for reliable work processing.

### Architecture Overview

```text
┌─────────────────────────────────────────────────────────────────────┐
│                            ViewRA Host                               │
│                                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐   │
│  │   Scanner    │  │  Event Bus   │  │    Pipeline Manager      │   │
│  └──────┬───────┘  └──────┬───────┘  └────────────┬─────────────┘   │
│         │                 │                       │                  │
│         │ enqueues        │ observes              │ configures       │
│         ▼                 ▼                       ▼                  │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │                    Enrichment Queue                             │ │
│  └────────────────────────────┬───────────────────────────────────┘ │
│                               │                                      │
│                               ▼                                      │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │              Per-Stage Worker Pools                             │ │
│  │   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │ │
│  │   │ NFO Workers │  │TMDB Workers │  │Fanart Workers│           │ │
│  │   │ (high conc) │  │(rate limit) │  │(rate limit)  │           │ │
│  │   └─────────────┘  └─────────────┘  └─────────────┘            │ │
│  └────────────────────────────┬───────────────────────────────────┘ │
│                               │                                      │
│  ┌────────────────────────────┴───────────────────────────────────┐ │
│  │                  Host Services (gRPC)                           │ │
│  │      HostData | HostStorage | HostCache | HostEvents            │ │
│  └────────────────────────────┬───────────────────────────────────┘ │
│                               │                                      │
│                         gRPC over stdio                              │
└───────────────────────────────┬──────────────────────────────────────┘
                                │
         ┌──────────────────────┼──────────────────┐
         │                      │                  │
         ▼                      ▼                  ▼
    ┌─────────┐           ┌──────────┐       ┌─────────┐
    │  TMDb   │           │   NFO    │       │ Webhook │
    │ Plugin  │           │  Plugin  │       │ Plugin  │
    └─────────┘           └──────────┘       └─────────┘
```

---

## Event Bus

Internal infrastructure for observability and real-time updates.

### Purpose

- Stream updates to UI (scan progress, enrichment progress)
- Aggregate logs from app and plugins
- Enable diagnostic exports
- Provide replay of recent events for new subscribers

### Architecture

```go
type Bus struct {
    subscribers map[uint64]*Subscription
    ring        *RingBuffer[Event]  // Recent history for replay
}

func (b *Bus) Subscribe(opts ...SubscribeOption) *Subscription
func (b *Bus) Publish(e Event)  // Non-blocking, fire-and-forget
```

### Event Types

**Domain Events** (typed, semantic):
- `MediaDiscovered`, `MediaUpdated`, `MediaRemoved`
- `EnrichmentStageComplete`, `EnrichmentComplete`
- `PluginLoaded`, `PluginCrashed`, `PluginHealthUpdate`
- `TranscodeStarted`, `TranscodeCompleted`, `TranscodeFailed`

**Log Events** (from slog integration):
- Converted from `slog.Record` to `LogEvent`
- Tagged with source (app component or plugin ID)
- Preserved for debugging and export

### slog Integration

```go
// Multi-handler writes to stderr AND publishes to bus
logger := slog.New(events.NewMultiHandler(
    slog.NewTextHandler(os.Stderr, nil),
    events.NewBusHandler(bus, "app"),
))
```

---

## Enrichment Queue

Persistent job queue for reliable, async work processing.

### Why a Queue?

- **Persistence**: Survives restarts (scanner completes, enrichment continues later)
- **Retry logic**: Failed enrichments retry with exponential backoff
- **Progress visibility**: Users see "1,203 / 2,847 enriched"
- **Non-blocking scanner**: Scanner enqueues work and moves on immediately

Events are observational (notify that something happened). Jobs are imperative (work that must be done).

### Schema

```sql
CREATE TABLE enrichment_queue (
    id INTEGER PRIMARY KEY,
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    stage TEXT NOT NULL,
    priority INTEGER DEFAULT 0,
    status TEXT DEFAULT 'pending',  -- pending, processing, completed, failed
    attempts INTEGER DEFAULT 0,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    error TEXT,
    UNIQUE(media_id, stage)
);

CREATE INDEX idx_enrichment_queue_claim
    ON enrichment_queue(stage, status, priority DESC, created_at)
    WHERE status = 'pending';

CREATE TABLE enrichment_status (
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    stage TEXT NOT NULL,
    status TEXT DEFAULT 'pending',  -- pending, processing, completed, skipped, failed
    completed_at TIMESTAMP,
    error TEXT,
    PRIMARY KEY (media_id, stage)
);
```

### Scanner Integration

```go
func (uc *ScanLibraryUseCase) processMovie(ctx context.Context, ...) error {
    // ... existing create/update logic ...

    // Single line addition—enqueue for enrichment
    uc.pipeline.EnqueueFirstStage(ctx, movie.Media.ID, MediaTypeMovie)
    return nil
}
```

---

## Enrichment Pipeline

User-configurable sequence of stages that process media metadata.

### Per-Media-Type Pipelines

```
Movies Pipeline:          Music Pipeline:
  1. NFO Parser             1. NFO Parser
  2. Local Images           2. Local Images
  3. TMDB                   3. MusicBrainz
  4. Fanart.tv
```

### Pipeline Configuration

```sql
CREATE TABLE enrichment_pipelines (
    id INTEGER PRIMARY KEY,
    media_type TEXT NOT NULL,      -- 'movie', 'tv', 'music'
    plugin_id TEXT NOT NULL REFERENCES plugins(id),
    position INTEGER NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    config JSONB,
    UNIQUE(media_type, position),
    UNIQUE(media_type, plugin_id)
);
```

### Apply Scopes

When users modify the pipeline, they explicitly choose scope:

```go
type PipelineChangeScope int

const (
    ScopeNewOnly        // Only affects new items
    ScopeMissingStages  // Re-enrich items missing new stages
    ScopeFullLibrary    // Re-enrich entire library
)
```

| Change | New Only | Missing Stages | Full Library |
|--------|----------|----------------|--------------|
| Add stage | ✓ future only | ✓ enqueue missing | ✓ re-run all |
| Remove stage | No effect | No effect | No effect |
| Reorder stages | ✓ future only | N/A | ✓ re-run all |

---

## Worker Concurrency Model

Per-stage worker pools with different characteristics based on plugin capabilities.

### Why Per-Stage?

| Plugin Type | Latency | Rate Limit | Optimal Concurrency |
|-------------|---------|------------|---------------------|
| NFO Parser | 10ms | None | High (CPU × 2) |
| Local Images | 50ms | None | High (CPU × 2) |
| TMDB | 200ms | 40/min | Low (rate-limited) |
| Fanart.tv | 300ms | 10/min | Very low |

A single worker pool can't optimize for both local and remote plugins.

### Configuration

```go
type StageWorkerConfig struct {
    PluginID    string
    Concurrency int           // Parallel workers for this stage
    BatchSize   int           // Items per batch claim
    RateLimit   rate.Limit    // Requests per second (0 = unlimited)
    Timeout     time.Duration // Per-item timeout
    RetryPolicy RetryPolicy
}
```

Worker config derived from plugin capabilities:
- Local plugins (`IsLocal: true`): High concurrency, no rate limit
- Remote plugins: Low concurrency, respect `RateLimit` from capabilities

---

## Core Plugin Interface

All plugins implement `PluginCore`:

```protobuf
service PluginCore {
  // Identity
  rpc GetInfo(Empty) returns (PluginInfo);

  // Lifecycle
  rpc Initialize(InitRequest) returns (InitResponse);
  rpc Shutdown(Empty) returns (Empty);
  rpc HealthCheck(Empty) returns (HealthStatus);

  // Settings
  rpc GetSettingsSchema(Empty) returns (SettingsSchema);
  rpc Configure(Settings) returns (ConfigureResponse);

  // Events
  rpc GetSubscriptions(Empty) returns (EventSubscriptions);
  rpc OnEvent(Event) returns (EventResponse);
}
```

### Plugin Categories

| Category | Description | Implementation |
|----------|-------------|----------------|
| Enricher | TMDb, MusicBrainz, NFO, Local Images | Phase 1-3 |
| NotificationSink | Webhooks, Discord, Telegram | Phase 5 |
| SubtitleProvider | OpenSubtitles (future) | Deferred |
| AnalyticsProvider | Trakt, Last.fm (future) | Deferred |

---

## Enricher Interface

Single `Enrich()` call for metadata enrichment (replaces separate Search/GetMetadata/GetImages).

```protobuf
service Enricher {
  rpc Enrich(EnrichRequest) returns (EnrichResponse);
  rpc GetCapabilities(Empty) returns (EnricherCapabilities);
}

message EnrichRequest {
  string media_id = 1;
  MediaType media_type = 2;
  string title = 3;
  int32 year = 4;
  string file_path = 5;
  map<string, string> existing_ids = 6;  // IDs from previous stages
}

message EnrichResponse {
  bool matched = 1;
  Metadata metadata = 2;
  map<string, string> ids = 3;  // IDs discovered by this enricher
}

message EnricherCapabilities {
  repeated MediaType media_types = 1;  // movies, tv, music
  repeated string provides = 2;         // "metadata", "artwork", "subtitles"
  bool is_local = 3;                    // Local = high concurrency
  int32 rate_limit = 4;                 // Requests per minute (0 = unlimited)
  repeated string requires = 5;         // Required fields/IDs
}
```

### ID Propagation

IDs discovered by one stage are passed to subsequent stages:

1. NFO enricher finds `imdb: tt0133093` → returns in `ids`
2. Host merges into `media_external_ids` table
3. TMDB enricher receives `existing_ids: {"imdb": "tt0133093"}`
4. TMDB skips search, looks up directly via IMDB ID
5. TMDB returns `ids: {"tmdb": "603"}`

---

## SDK Design

Go SDK with compile-time enforcement via required embedding.

### Base Struct

```go
type Base struct {
    logger    *slog.Logger
    requestID string
    metrics   *MetricsCollector
    config    *BaseConfig
}

// Unexported method forces embedding
func (Base) mustEmbedBase() {}

// Base provides common functionality for free
func (b *Base) Log() *slog.Logger {
    return b.logger.With("request_id", b.requestID)
}

func (b *Base) RecordLatency(operation string, d time.Duration) {
    b.metrics.RecordLatency(operation, d)
}
```

### Plugin Interface

```go
type EnricherPlugin interface {
    mustEmbedBase()  // Forces embedding Base—won't compile without it
    Capabilities() EnricherCapabilities
    Enrich(ctx context.Context, req *EnrichRequest) (*EnrichResponse, error)
}
```

### Example Plugin

```go
type TMDBPlugin struct {
    plugin.Base  // Required, won't compile without it
    client *tmdb.Client
}

func (p *TMDBPlugin) Enrich(ctx context.Context, req *EnrichRequest) (*EnrichResponse, error) {
    p.Log().Info("enriching", "title", req.Title)  // Logging comes free
    // ...
}
```

---

## Debugging & Observability

### Correlation IDs

Request IDs propagate across app → gRPC → plugin:

```go
// Generated in the app
requestID := uuid.New().String()
ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-request-id", requestID))

// Plugin SDK extracts and includes in all logs
func (b *Base) Log() *slog.Logger {
    return b.logger.With("request_id", b.requestID)
}
```

Result in logs:
```
[12:35:00] INFO  req=abc123 app/worker    Processing media 42
[12:35:00] INFO  req=abc123 plugin/tmdb   Searching for "The Matrix"
[12:35:00] INFO  req=abc123 app/worker    Media 42 enrichment complete
```

### gRPC Debug Mode

`VIEWRA_PLUGIN_DEBUG=1` enables full request/response logging:
```
[12:35:00] DEBUG grpc  method=Enrich plugin=tmdb
                       request={"media_id": 42, "title": "The Matrix"}
[12:35:00] DEBUG grpc  response={"matched": true, "ids": {"tmdb": "603"}}
```

### Plugin Health Monitoring

```go
type PluginHealth struct {
    PluginID      string
    Status        HealthStatus  // healthy, degraded, unhealthy, crashed
    LastHeartbeat time.Time
    ErrorRate     float64       // Errors per minute
    AvgLatency    time.Duration
}
```

UI display:
```
Plugins:
  ✓ NFO Parser        healthy     avg 12ms    0 errors
  ✓ TMDB              degraded    avg 890ms   3 errors/min (rate limited)
```

### Error Categorization

```go
type ErrorCategory string

const (
    ErrorCategoryNetwork   = "network"     // Connection failed, timeout
    ErrorCategoryRateLimit = "rate_limit"  // API rate limited
    ErrorCategoryNotFound  = "not_found"   // No match in external DB
    ErrorCategoryParsing   = "parsing"     // Failed to parse file/response
    ErrorCategoryPlugin    = "plugin"      // Plugin crashed or returned error
)
```

### Diagnostic Export

Exportable bundles for debugging/support:

```
viewra-diagnostics-2025-12-02/
├── system.json           # OS, version, hardware acceleration
├── plugins.json          # Installed plugins, versions, status
├── pipelines.json        # Pipeline configuration per media type
├── queue-status.json     # Current queue depths, processing stats
├── errors.json           # Recent errors with context
├── logs/
│   ├── app.log
│   └── plugin-*.log
└── stats.json            # Processing rates, success/failure counts
```

---

## Progress Visibility

### Library-Level Progress

```
Library: Movies (2,847 items)

Enrichment Pipeline:
  ✓ NFO Metadata     2,847 / 2,847  complete
  ✓ Local Images     2,834 / 2,847  13 not found
  ⏳ TMDB             1,203 / 2,847  in progress (est. 12 min)
  ○ Fanart.tv            0 / 2,847  waiting
```

### Item-Level Status

```
The Matrix (1999)

Enrichment Status:
  ✓ NFO Parser      completed 2 min ago    Found metadata + IMDB ID
  ✓ Local Images    completed 2 min ago    Found poster.jpg, fanart.jpg
  ✓ TMDB            completed 1 min ago    Matched via IMDB ID
  ⏳ Fanart.tv       in progress            Fetching artwork...

Metadata Sources:
  • Title: NFO file
  • Synopsis: TMDB
  • Poster: Local file (poster.jpg)
```

---

## Lifecycle & Process Management

### Adaptive Warm Pool

- Start plugins on first use
- Keep "hot" plugins warm (used within last 30 minutes)
- Gracefully shutdown cold plugins after extended idle
- Pre-warm during library scan startup
- Configurable `keep_warm: true` for critical plugins

### Health & Recovery

- Heartbeat checks every 30 seconds
- Automatic restart on crash (max 3 retries)
- Smart retry/fallback:
  - Transient errors: Retry with exponential backoff
  - Persistent errors: Fall back to next provider
  - Plugin crash: Restart, retry once, then skip
  - All fail: Use cached data, mark for retry

---

## Data Access & Security

### Host Services

```protobuf
service HostData {
  rpc GetMedia(MediaQuery) returns (MediaList);
  rpc GetLibrary(LibraryId) returns (Library);
  rpc GetWatchProgress(MediaId) returns (Progress);
  rpc SearchMedia(SearchQuery) returns (MediaList);
  rpc GetMediaByExternalId(ExternalIdQuery) returns (Media);
}

service HostStorage {
  // Plugin's sandboxed key-value store
  rpc KVGet(KVKey) returns (KVValue);
  rpc KVSet(KVEntry) returns (Empty);
  rpc KVDelete(KVKey) returns (Empty);

  // Plugin's SQLite database
  rpc GetDatabasePath(Empty) returns (DatabasePath);
  rpc RegisterSchema(SchemaVersion) returns (Empty);
  rpc GetDatabaseStats(Empty) returns (DatabaseStats);
}
```

### Permission Model

Plugins declare required permissions in manifest:

```yaml
permissions:
  - network                 # Make HTTP requests
  - storage:kv              # Use key-value storage
  - storage:database        # Use SQLite database
  - data:media:read         # Read media information
  - data:metadata:write     # Write metadata results
  - data:progress:read      # Read watch progress
  - events:playback.*       # Receive playback events
  - ui:settings             # Provide settings page
```

**Category Defaults:**
- Enricher: `network`, `storage:database`, `data:media:read`, `data:metadata:write`
- NotificationSink: `network`, `storage:kv`, `data:progress:read`, `events:playback.*`

**Never Exposed:**
- Direct database connection
- User passwords, tokens, sessions
- Other plugins' storage
- System configuration paths

---

## Plugin Storage

### Directory Structure

```
data/plugins/
├── tmdb/
│   ├── cache.db          # Plugin's SQLite database
│   ├── cache.db-wal
│   └── settings.json     # Plugin settings (host-managed)
├── musicbrainz/
│   ├── cache.db
│   └── settings.json
└── webhook-notifier/
    └── settings.json     # No DB needed
```

### Quotas & Limits

| Setting | Default | Notes |
|---------|---------|-------|
| Max DB size | 100 MB | Configurable per plugin |
| Cache TTL | 7 days | Plugin-configurable |
| Vacuum frequency | Weekly | Automatic |

---

## Metadata Storage

### Merged Record + Source Tracking

```sql
-- Main metadata in media table (merged result)
-- Source tracking for transparency
CREATE TABLE media_metadata_sources (
  media_id INTEGER NOT NULL,
  field_name TEXT NOT NULL,
  plugin_name TEXT NOT NULL,
  raw_value TEXT,
  updated_at TIMESTAMP,
  PRIMARY KEY (media_id, field_name, plugin_name)
);

-- External IDs (extensible)
CREATE TABLE media_external_ids (
  media_id INTEGER NOT NULL,
  provider TEXT NOT NULL,
  external_id TEXT NOT NULL,
  PRIMARY KEY (media_id, provider)
);
```

### Field-Level Priority

```yaml
metadata_priority:
  movies:
    title: [nfo, tmdb]
    plot: [nfo, tmdb]
    poster: [tmdb, nfo]
    backdrop: [tmdb]
    rating: [tmdb, nfo]
```

### Match Confidence

- Auto-match above threshold (85%)
- Below threshold: show top candidates with scores
- User picks correct one or searches manually
- Confirmed matches "locked" to prevent rescan overwrite

---

## UI Extensions

### Extension Points

| Extension Point | Description |
|-----------------|-------------|
| Settings page | Custom configuration UI |
| Media detail tabs | Additional tabs on movie/show/album pages |
| Context menu items | Actions on media items |
| Info panels | Additional info sections |
| Dashboard widgets | Cards on home/dashboard |
| Navigation items | New top-level menu entries |
| Player overlays | Controls/info during playback |

### UI Delivery

**Hybrid approach:**
- Simple UI: JSON schema rendered by frontend
- Complex UI: Bundled React components

```protobuf
message UIExtension {
  string extension_point = 1;
  oneof content {
    UISchema schema = 2;      // JSON schema for simple UI
    string component_path = 3; // Path to React component
  }
}
```

---

## NotificationSink

```protobuf
service NotificationSink {
  rpc SendNotification(Notification) returns (NotificationResult);
  rpc GetCapabilities(Empty) returns (NotificationCapabilities);
}
```

---

## Distribution & Updates

### Plugin Manifest

```yaml
# plugin.yaml
name: tmdb
version: 1.2.0
min_host_version: 0.5.0
author: ViewRA Team
license: MIT
permissions:
  - network
  - storage:database
  - data:media:read
  - data:metadata:write
```

### Installation

```bash
viewra plugin install tmdb
viewra plugin install github.com/user/my-plugin
```

### Updates

- Check on startup (or daily)
- Notify in UI: "3 plugin updates available"
- User manually approves updates

---

## Built-in Plugins

Following same plugin interface (can be disabled):

| Plugin | Category | Notes |
|--------|----------|-------|
| NFO | Enricher | Local file parsing, runs first |
| Local Images | Enricher | Scans for poster.jpg, fanart.jpg |
| TMDb | Enricher | Movies/TV |
| MusicBrainz | Enricher | Music |

---

## Implementation Phases

### Phase 1: Core Infrastructure (5-6 days)

- Event Bus with ring buffer, slog integration
- Enrichment Queue tables and operations
- Pipeline Manager with user configuration
- Per-stage worker pools

### Phase 2: Plugin Foundation (4-5 days)

- Plugin manager and process lifecycle
- PluginCore gRPC definitions
- Enricher interface with capabilities
- SDK Base struct with compile-time enforcement
- Host services (HostData, HostStorage)
- Permission system

### Phase 3: First Enrichers (4-5 days)

- NFO plugin (local file parsing)
- Local Images plugin
- Scanner integration (enqueue on discovery)
- Progress tracking (library + item level)
- ID propagation between stages

### Phase 4: Remote Enrichers (4-5 days)

- TMDb plugin for movies/TV
- MusicBrainz plugin for music
- Rate limiting and retry logic
- Plugin SQLite databases with quotas
- Remove hardcoded provider code

### Phase 5: Events & Notifications (3-4 days)

- Event delivery to plugins
- NotificationSink category
- Webhook plugin as example
- Playback events integration

### Phase 6: Observability (3-4 days)

- Correlation IDs across boundaries
- gRPC debug mode
- Plugin health monitoring
- Error categorization
- Diagnostic export

### Phase 7: UI & Polish (3-4 days)

- Pipeline configuration UI
- Progress visibility UI
- UI extension points
- Plugin CLI commands
- Documentation

**Total estimated: 26-33 days**

---

## Consequences

### Positive

- All integrations use consistent interface
- Non-blocking scanner with async enrichment
- User control over pipeline configuration
- Full observability with correlation IDs
- Progress visibility at library and item level
- Process isolation prevents crashes from affecting host

### Negative

- More complex than synchronous approach
- Process overhead for each plugin (mitigated by warm pool)
- gRPC complexity vs direct function calls
- Significant initial implementation effort

### Neutral

- Breaking change for any existing metadata customization
- Requires maintaining backward compatibility for plugin protocol

---

## Alternatives Considered

### Synchronous enrichment in scanner

Simpler but scanner would block on slow API calls. Large libraries would take hours to scan.

### Single worker pool for all plugins

Simpler but can't optimize for both local (high concurrency) and remote (rate-limited) plugins.

### Separate Search/GetMetadata/GetImages RPCs

More flexible but adds round trips and complexity. Plugins can search internally if needed.

### In-process plugins (Go interfaces)

Simpler but no isolation—a buggy plugin crashes ViewRA.

---

## Decision Summary

| # | Topic | Decision |
|---|-------|----------|
| 1-6 | Core use case | Metadata providers first, then notifications |
| 7-11 | Architecture | go-plugin + gRPC, process isolation |
| 12-14 | Operations | Protocol versioning, smart retry, structured logging |
| 15 | Merging | Field-level priority configuration |
| 16-20 | Dev experience | Test harness, Git distribution, parallel limits |
| 21 | Manifest | Hybrid (minimal file + RPC capabilities) |
| 22 | Lifecycle | Adaptive warm pool |
| 23 | Search flow | ViewRA parses, plugins search, ViewRA selects |
| 24 | NFO handling | Runs first, provides hints |
| 25 | Metadata storage | Merged record + source tracking |
| 26 | External IDs | Separate extensible table |
| 27 | Match confidence | Show candidates, lock confirmed |
| 28-29 | Hierarchies | Series/album first, then episodes/tracks |
| 30 | Protobuf schema | Base + optional TV/Music services |
| 31 | Categories | Define now, implement incrementally |
| 32-34 | Events | Comprehensive set, push via callback |
| 35 | Dependencies | No inter-plugin communication |
| 36-38 | UI | Full extension points, hybrid delivery |
| 39 | Permissions | Explicit declarations, category defaults |
| 40 | Language | Go SDK, protobuf as source of truth |
| 41-42 | Distribution | Registry API, notify on updates |
| 43 | Built-ins | NFO, TMDb, MusicBrainz (same interface) |
| 44 | Isolation | Process only (v1) |
| 45 | Priority | Metadata + Notifications first |
| 47-48 | Data access | Host-mediated, permission-gated |
| 50-51 | Plugin storage | SQLite per plugin with quotas |
| 52 | Event Bus | Internal with ring buffer, slog integration |
| 53 | Enrichment Queue | Persistent async job queue |
| 54 | Pipeline | User-configurable per media type |
| 55 | Apply scopes | User chooses (new only, missing, full) |
| 56 | Worker pools | Per-stage, configured from capabilities |
| 57 | SDK | Base struct with compile-time enforcement |
| 58 | Debugging | Correlation IDs, debug mode, health monitoring |
| 59 | Progress | Library-level and item-level visibility |
| 60 | Enricher interface | Single Enrich() call |
| 61 | ID propagation | Pass accumulated IDs, merge returned |
| 62 | Capabilities | Rich fields (MediaTypes, Provides, IsLocal, etc.) |
| 63 | Diagnostics | Exportable diagnostic bundles |
| 64 | Phases | Reorganized for new components |

---

## References

- [Hashicorp go-plugin](https://github.com/hashicorp/go-plugin)
- [gRPC Go](https://grpc.io/docs/languages/go/)
- [Protocol Buffers](https://protobuf.dev/)
- Terraform provider architecture (similar pattern)
