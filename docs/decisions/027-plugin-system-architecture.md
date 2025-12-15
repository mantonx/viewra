# ADR 027: Plugin System Architecture

## Status

**In Progress** - Phase 1 Complete

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
│  │   HostData | HostStorage | HostUserMetadata | HostEvents        │ │
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
    media_type TEXT NOT NULL,      -- 'movie', 'tv', 'tv_show', 'music'
    plugin_id TEXT NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
    stage_name TEXT NOT NULL,      -- Display name for the stage
    position INTEGER NOT NULL,
    enabled INTEGER DEFAULT 1,
    config_json TEXT,              -- Plugin-specific configuration
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    UNIQUE(media_type, position),
    UNIQUE(media_type, plugin_id)
);

-- Index for loading pipeline configuration by media type
CREATE INDEX idx_enrichment_pipelines_order ON enrichment_pipelines(media_type, enabled, position);
```

The `plugin_id` column has a foreign key constraint to `plugins(id)` with `ON DELETE CASCADE`, ensuring pipeline entries are cleaned up when a plugin is uninstalled.

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

All plugins implement `PluginCore`. Plugin identity comes from the `plugin.yml` manifest file, which the host reads before starting the plugin binary.

```protobuf
service PluginCore {
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

message InitRequest {
  string host_version = 1;  // For compatibility checking
  string data_dir = 2;      // Plugin's data directory
  bytes config = 3;         // Contents of config.yml (runtime configuration)
}
```

**Plugin Identity Flow:**

1. Host discovers plugin at `plugins/<name>/plugin.yml`
2. Host reads manifest to get ID, name, version, categories, permissions
3. Host starts plugin binary with `go-plugin`
4. Host calls `Initialize(data_dir, config)` with runtime configuration
5. Plugin is now ready to receive requests

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
// EnricherPlugin is the interface that enricher plugins must implement.
// Plugin identity comes from plugin.yml manifest file, not code.
type EnricherPlugin interface {
    mustEmbedBase()  // Forces embedding Base—won't compile without it

    // GetCapabilities returns what this enricher provides and requires.
    GetCapabilities() EnricherCapabilities

    // Initialize is called when the plugin is loaded.
    // dataDir is the plugin's isolated data directory.
    // config is the contents of config.yml (runtime configuration).
    Initialize(ctx context.Context, dataDir string, config []byte) error

    // Shutdown is called before the plugin is unloaded.
    Shutdown(ctx context.Context) error

    // Enrich processes a single media item.
    Enrich(ctx context.Context, req *EnrichRequest) (*EnrichResponse, error)
}
```

### Example Plugin

```go
type TMDBPlugin struct {
    sdk.Base  // Required, won't compile without it
    client *tmdb.Client
}

// Plugin identity comes from plugin.yml manifest, not code constants.

func (p *TMDBPlugin) GetCapabilities() sdk.EnricherCapabilities {
    return sdk.EnricherCapabilities{
        MediaTypes: []string{"movie", "tv"},
        Provides:   []string{"metadata", "artwork", "external_ids"},
        IsLocal:    false,
        RateLimit:  40,  // TMDb: ~40 req/10s
    }
}

func (p *TMDBPlugin) Initialize(ctx context.Context, dataDir string, config []byte) error {
    // Parse config.yml contents
    var cfg Config
    if err := yaml.Unmarshal(config, &cfg); err != nil {
        return err
    }
    p.client = tmdb.NewClient(cfg.APIKey)
    return nil
}

func (p *TMDBPlugin) Shutdown(ctx context.Context) error {
    return nil
}

func (p *TMDBPlugin) Enrich(ctx context.Context, req *sdk.EnrichRequest) (*sdk.EnrichResponse, error) {
    p.Log().Info("enriching", "title", req.Title)  // Logging comes free from Base
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

### Plugin Registry Database Schema

The `plugins` table provides centralized plugin management with health persistence:

```sql
CREATE TABLE plugins (
    id TEXT PRIMARY KEY,           -- From manifest (e.g., "tmdb", "builtin:nfo")
    name TEXT NOT NULL,            -- Display name from manifest
    version TEXT NOT NULL,         -- Installed version
    description TEXT,
    author TEXT,
    license TEXT,
    homepage TEXT,
    categories TEXT NOT NULL,      -- JSON array (e.g., '["enricher"]')
    is_builtin INTEGER DEFAULT 0,  -- 1 for built-in plugins
    enabled INTEGER DEFAULT 1,     -- Global enable/disable toggle
    path TEXT,                     -- Binary path (NULL for built-ins)
    -- Health tracking (persisted across restarts)
    health_status TEXT DEFAULT 'unknown',  -- healthy, degraded, unhealthy, unknown
    last_heartbeat TEXT,
    restart_count INTEGER DEFAULT 0,
    -- Timestamps
    installed_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- Index for listing enabled plugins
CREATE INDEX idx_plugins_enabled ON plugins(enabled, is_builtin);
```

**Built-in vs External Plugins:**

| Type | `is_builtin` | `path` | Example IDs |
|------|--------------|--------|-------------|
| Built-in | 1 | NULL | `builtin:nfo`, `builtin:local-images` |
| External | 0 | `/path/to/binary` | `tmdb`, `musicbrainz` |

Built-in plugins are seeded in the migration and cannot be uninstalled (only disabled).

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

// Per-user metadata storage for plugins (see ADR 028)
service HostUserMetadata {
  // Get plugin-specific data for a user
  rpc Get(UserMetadataKey) returns (UserMetadataValue);
  // Set plugin-specific data for a user
  rpc Set(UserMetadataEntry) returns (Empty);
  // Delete plugin-specific data for a user
  rpc Delete(UserMetadataKey) returns (Empty);
  // List all keys for a user
  rpc ListKeys(UserId) returns (UserMetadataKeyList);
}

message UserMetadataKey {
  string user_id = 1;
  string key = 2;
}

message UserMetadataValue {
  bytes value = 1;
  bool exists = 2;
}

message UserMetadataEntry {
  string user_id = 1;
  string key = 2;
  bytes value = 3;
}

message UserMetadataKeyList {
  repeated string keys = 1;
}
```

### Permission Model

Plugins declare required permissions in manifest:

```yaml
permissions:
  - network                 # Make HTTP requests
  - storage:kv              # Use key-value storage
  - storage:database        # Use SQLite database
  - storage:user_metadata   # Store per-user data
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

### Plugin API Keys

For server-to-server communication or scheduled tasks (not on behalf of a user), plugins can request API keys:

```go
type PluginAPIKey struct {
    PluginID    string
    Key         string      // Stored hashed (SHA-256)
    Permissions []string    // Scoped to plugin's declared permissions
    ExpiresAt   *time.Time  // Optional expiration
    CreatedAt   time.Time
}
```

**Use Cases:**
- Background sync jobs (e.g., webhook retry queue)
- Scheduled metadata refresh
- Inter-plugin communication (if approved)

**Database Schema:**

```sql
CREATE TABLE plugin_api_keys (
    id TEXT PRIMARY KEY,
    plugin_id TEXT NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
    key_hash TEXT NOT NULL UNIQUE,
    permissions TEXT NOT NULL,  -- JSON array
    expires_at TEXT,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_plugin_api_keys_plugin ON plugin_api_keys(plugin_id);
```

**Security:**
- Keys are scoped to the plugin's declared permissions (cannot escalate)
- Keys can be revoked by admin or plugin uninstall
- Requests authenticated via `X-Plugin-API-Key` header
- Rate limited separately from user requests

### User Metadata Storage

Plugins store per-user data via `HostUserMetadata` RPC (see [ADR 028](028-user-authentication.md)):

**Database Schema:**

```sql
CREATE TABLE plugin_user_metadata (
    plugin_id TEXT NOT NULL REFERENCES plugins(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value BLOB NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (plugin_id, user_id, key)
);

CREATE INDEX idx_plugin_user_metadata_user ON plugin_user_metadata(user_id);
CREATE INDEX idx_plugin_user_metadata_plugin ON plugin_user_metadata(plugin_id);
```

**Notes:**
- Keys are namespaced by plugin ID automatically
- Data is cleaned up when users are deleted (`ON DELETE CASCADE` on `user_id`)
- Data is cleaned up when plugins are uninstalled (`ON DELETE CASCADE` on `plugin_id`)
- Requires `storage:user_metadata` permission

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

The `plugin.yml` manifest is the **single source of truth** for plugin identity. The host reads this file before starting the plugin binary.

```yaml
# plugin.yml - Static metadata (checked into source control)
id: tmdb                      # Unique identifier (used in DB, logs, etc.)
name: TMDb                    # Display name
version: 1.2.0
description: Fetch movie and TV metadata from The Movie Database
author: ViewRA Team
license: MIT
homepage: https://github.com/mantonx/viewra
min_host_version: 0.5.0
categories:
  - enricher
permissions:
  - network
  - storage:database
  - data:media:read
  - data:metadata:write
```

```yaml
# config.yml - Runtime configuration (user-editable, not in source control)
api_key: "${TMDB_API_KEY}"    # Can use environment variables
language: en-US
include_adult: false
```

**Why Two Files?**

| File | Purpose | Versioned? | User-editable? |
|------|---------|------------|----------------|
| `plugin.yml` | Static identity, permissions | Yes | No |
| `config.yml` | Runtime settings (API keys, preferences) | No | Yes |

The host passes `config.yml` contents to `Initialize()` as bytes.

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

## Built-in vs Third-Party Plugins

### Execution Model

Plugins use different execution models based on trust level:

| Plugin Type | Execution | gRPC | Library Access | Use Case |
|-------------|-----------|------|----------------|----------|
| **Built-in** | In-process | No | Direct imports | First-party, trusted code |
| **Third-party** | go-plugin | Yes | Via Host Services | Community plugins, untrusted |

### Why This Distinction?

**Problem with shared libraries across process boundaries:**

A go-plugin binary is compiled separately and cannot import ViewRA's internal packages:

```text
┌─────────────────────────┐      ┌─────────────────────────┐
│      ViewRA Host        │      │  Third-Party Plugin     │
│                         │      │  (separate binary)      │
│  internal/              │  ✗   │                         │
│    infrastructure/      │ ───► │  Can't import internal/ │
│      metadata/nfo/      │      │                         │
└─────────────────────────┘      └─────────────────────────┘
```

Options considered:

1. **Duplicate code** - Copy parsing logic into each plugin (maintenance nightmare)
2. **Shared module** - Extract to `github.com/mantonx/viewra-sdk` (overkill for built-ins)
3. **Host Services** - Expose parsing via gRPC (adds latency for local operations)
4. **In-process built-ins** ✓ - Built-ins run in-process, third-party uses gRPC

**Solution**: Built-in plugins implement the same `Enricher` interface but run in-process, directly using existing library code. Third-party plugins use go-plugin + gRPC with Host Services for any parsing needs.

### Built-in Plugins

Built-in plugins are for **local file operations only**—parsing files on disk that require direct access to ViewRA's internal libraries:

- Implement the same `Enricher` interface as external plugins
- Run **in-process** (no gRPC serialization overhead)
- Can be disabled/reordered via pipeline configuration
- Directly import ViewRA's internal libraries

| Plugin | Category | Notes |
|--------|----------|-------|
| NFO | Enricher | Uses `internal/infrastructure/metadata/nfo/` directly |
| Local Images | Enricher | Uses `internal/infrastructure/images/extractor/` directly |

### External Plugins (go-plugin)

All plugins that make **network requests** or are **third-party contributed** run as external processes:

- Run as **separate processes** via go-plugin
- Communicate via gRPC (process isolation, crash protection)
- Access data through Host Services only
- Cannot import ViewRA internals
- Crash/hang in plugin doesn't affect ViewRA host

| Plugin | Category | Notes |
|--------|----------|-------|
| TMDb | Enricher | Movies/TV metadata from external API |
| MusicBrainz | Enricher | Music metadata from external API |
| Fanart.tv | Enricher | Additional artwork from external API |
| Webhooks | NotificationSink | Send notifications to external services |

**Why external for API-based enrichers?**

1. **Isolation**: Network failures, timeouts, or API issues don't affect host stability
2. **Rate limiting**: Each plugin manages its own rate limits independently
3. **Restartable**: Plugin can be restarted without restarting ViewRA
4. **Upgradeable**: Plugin binaries can be updated independently
5. **Replaceable**: Community can contribute alternative implementations

```protobuf
// Host Services available to external plugins
service HostData {
  rpc GetMedia(MediaQuery) returns (Media);
  rpc GetFilePath(MediaId) returns (FilePath);
}

service HostFileParser {
  // If an external plugin needs NFO parsing, host provides it
  rpc ParseNFO(ParseNFORequest) returns (NFOMetadata);
}
```

### Interface Consistency

Both built-in and third-party plugins implement the same interface:

```go
// Shared interface - implementation differs by execution model
type Enricher interface {
    Stage() string
    Capabilities() EnricherCapabilities
    Enrich(ctx context.Context, req *EnrichRequest) (*EnrichResponse, error)
}

// Built-in: direct implementation
type NFOEnricher struct {
    parser *nfo.Parser  // Direct import of internal library
}

// Third-party: gRPC client wrapper
type ExternalEnricher struct {
    client EnricherClient  // gRPC client to plugin process
}
```

The Pipeline Manager doesn't care which execution model is used - it calls `Enrich()` on both.

---

## Migration Strategy: Existing Code to Plugins

### Current State

NFO parsing and image extraction currently run **synchronously within the scanner**:

```
internal/infrastructure/metadata/nfo/     # NFO parsing (movie, tvshow)
internal/application/library/scan/media/  # Image extraction helpers
```

The scanner calls these directly in `ProcessMovie()`, `ProcessTVEpisode()`, etc.:

```go
// Current: NFO parsed inline during scan
nfoPath, _ := nfo.FindMovieNFO(result.FilePath)
nfoMetadata, _ := nfo.ParseMovieNFO(nfoPath)
movie.IMDbID = nfoMetadata.IMDbID  // Available immediately

// Current: Images extracted inline
ExtractImagesForMovie(ctx, deps, movie, filePath)
```

### Target State

All metadata enrichment moves to the async plugin pipeline:

```
Scanner → saves minimal media → enqueues → NFO Plugin → Images Plugin → TMDb Plugin
              ↓
         (filename-parsed title/year only)
```

### Migration Approach

**Phase 3 transforms the scanner to "discovery only":**

1. **Scanner changes:**
   - Remove NFO parsing calls from `ProcessMovie()`, `ProcessTVEpisode()`, `ProcessMusicTrack()`
   - Remove image extraction calls
   - Save media with filename-parsed metadata only (title, year from filename)
   - Enqueue media for enrichment pipeline

2. **NFO Enricher (built-in):**
   - Implements `Enricher` interface
   - Runs **in-process** (not go-plugin)
   - Directly imports `internal/infrastructure/metadata/nfo/`
   - Returns metadata + external IDs (IMDB, TMDB)
   - Host merges results into media record

3. **Local Images Enricher (built-in):**
   - Implements `Enricher` interface
   - Runs **in-process** (not go-plugin)
   - Directly imports existing image extraction libraries
   - Handles poster.jpg, fanart.jpg, folder.jpg patterns
   - Extracts embedded images from media files via ffmpeg

4. **Code disposition:**

| Current Location | Disposition |
|------------------|-------------|
| `internal/infrastructure/metadata/nfo/` | Keep as library, imported by NFO enricher |
| `internal/application/library/scan/media/images.go` | Refactor into Local Images enricher |
| `internal/infrastructure/images/extractor/` | Keep as library, imported by enricher |

**New enricher locations:**

```text
internal/application/enrichment/
├── pipeline/          # Pipeline manager, workers (Phase 1 ✓)
└── enrichers/         # Built-in enrichers (Phase 3)
    ├── nfo/           # NFO enricher (in-process)
    ├── localimages/   # Local images enricher (in-process)
    ├── tmdb/          # TMDb enricher (in-process, could be external later)
    └── musicbrainz/   # MusicBrainz enricher (in-process)
```

### Why Full Extraction (Not Hybrid)

We chose to move **all** enrichment to plugins rather than keeping NFO inline because:

1. **Consistency**: All metadata sources use the same plugin interface
2. **User control**: Users can disable/reorder any stage including NFO
3. **Observability**: All enrichment visible in pipeline progress UI
4. **Testability**: Scanner becomes simpler (discovery only)
5. **Future flexibility**: Easy to add alternative local metadata sources

**Trade-off accepted**: Two database writes per media item (initial save, then update after NFO). This is acceptable because:
- SQLite/Postgres writes are fast (~1ms)
- NFO plugin runs first with high concurrency
- Decoupling benefits outweigh minor overhead

### Scanner Before/After

**Before (current):**
```go
func ProcessMovie(ctx context.Context, deps *Deps, ...) (*int64, error) {
    movie := parseFilename(result)

    // Inline NFO enrichment
    if nfoPath, _ := nfo.FindMovieNFO(result.FilePath); nfoPath != "" {
        nfoData, _ := nfo.ParseMovieNFO(nfoPath)
        movie.IMDbID = nfoData.IMDbID
        movie.Plot = nfoData.Plot
        // ... 20+ fields
    }

    // Save with full metadata
    savedMovie, _ := deps.MediaRepos.Movies.Create(ctx, movie)

    // Inline image extraction
    ExtractImagesForMovie(ctx, deps, savedMovie, filePath)

    return &savedMovie.Media.ID, nil
}
```

**After (Phase 3+):**
```go
func ProcessMovie(ctx context.Context, deps *Deps, ...) (*int64, error) {
    movie := parseFilename(result)  // Title, year from filename only

    // Save minimal record
    savedMovie, _ := deps.MediaRepos.Movies.Create(ctx, movie)

    // Enqueue for async enrichment (fire-and-forget)
    deps.EnrichmentEnqueuer.EnqueueFirstStage(ctx, savedMovie.Media.ID, enrichment.MediaTypeMovie)

    return &savedMovie.Media.ID, nil
}
```

---

## Implementation Phases

### Phase 1: Core Infrastructure (5-6 days) ✓ COMPLETE

- ✓ Event Bus with ring buffer, slog integration (`internal/domain/events/`, `internal/infrastructure/events/`)
- ✓ Enrichment Queue tables and operations (migration 000027)
- ✓ Pipeline Manager with user configuration (`internal/application/enrichment/pipeline/`)
- ✓ Per-stage worker pools (`internal/application/enrichment/pipeline/worker_pool.go`)

### Phase 2: Built-in Enrichers + Scanner Integration (5-6 days)

#### Phase 2 Overview

This phase transforms the scanner from "scan + enrich synchronously" to "scan → enqueue → async enrich". The enrichment infrastructure (queue, workers, event bus, Enricher interface) already exists from Phase 1—we're adding the actual enricher implementations and wiring them to the scanner.

#### Prerequisites from Phase 1 (✓ Complete)

| Component | Status | Location |
|-----------|--------|----------|
| Event Bus (domain types) | ✓ | `internal/domain/events/` |
| Event Bus (infrastructure) | ✓ | `internal/infrastructure/events/bus/` |
| slog Handler | ✓ | `internal/infrastructure/events/slog/handler.go` |
| Enrichment Queue | ✓ | migration 000027 |
| Domain Types | ✓ | `internal/domain/enrichment/types.go` |
| Enricher Interface | ✓ | `internal/application/enrichment/enricher.go` |
| Pipeline Manager | ✓ | `internal/application/enrichment/pipeline/manager.go` |
| Worker Pools | ✓ | `internal/application/enrichment/pipeline/worker_pool.go` |
| external_ids queries | ✓ | `internal/infrastructure/database/queries/*/external_ids.sql` |

#### Current State Analysis

**Scanner already calls `enqueueForEnrichment()`** but `EnrichmentEnqueuer` is NOT wired:

```go
// internal/application/library/scan/media/deps.go:50
EnrichmentEnqueuer EnrichmentEnqueuer  // OPTIONAL, nil if not configured

// internal/application/library/scan_orchestrator.go:254-271
func (uc *ScanLibraryUseCase) mediaDeps() *scanmedia.Deps {
    return &scanmedia.Deps{
        MediaRepos:       uc.mediaRepos,
        // ... all extractors set ...
        // EnrichmentEnqueuer: ???  ← NOT SET
    }
}
```

**NFO parsing is inline** in movie.go (lines 72-103) and tv.go (lines 121-151):

```go
// internal/application/library/scan/media/movie.go:72-103
nfoPath, err := nfo.FindMovieNFO(result.FilePath)
if err == nil && nfoPath != "" {
    nfoMetadata, err := nfo.ParseMovieNFO(nfoPath)
    if err == nil && nfoMetadata != nil {
        movie.Media.Title = nfoMetadata.Title
        movie.IMDbID = nfoMetadata.IMDbID
        // ... 20+ fields populated synchronously
    }
}
```

**Image extraction is inline** via PostSave callbacks:

```go
// internal/application/library/scan/media/movie.go:131
PostSave: func(ctx context.Context) {
    ExtractImagesForMovie(ctx, deps, movie, result.FilePath)
    PersistMediaTracks(ctx, deps, movie.Media.ID, result)
    enqueueForEnrichment(ctx, deps, movie.Media.ID, enrichment.MediaTypeMovie)
}
```

**TV Show NFO is coupled to image extraction** (architectural issue):

```go
// internal/application/library/scan/media/images.go:122-126
// Inside extractTVShowAndSeasonImages():
if deps.ProcessedShows.TryMark(showTitle) {
    EnrichTVShowMetadataFromNFO(ctx, deps, show.ID, episodeFilePath)  // ← Called from image extraction!
}
```

#### Phase 2 Implementation Tasks

**2.1 Enricher Interface** (`internal/application/enrichment/enricher.go`) ✓ COMPLETE

```go
package enrichment

import (
    "context"
    "github.com/mantonx/viewra/internal/domain/enrichment"
)

// Enricher is the unified interface for all enrichment stages.
// Both built-in (in-process) and third-party (gRPC) plugins implement this.
type Enricher interface {
    // Stage returns the unique identifier for this enrichment stage (e.g., "nfo", "local_images", "tmdb")
    Stage() string

    // Capabilities describes what this enricher provides and requires.
    Capabilities() EnricherCapabilities

    // Enrich processes a single media item.
    // Returns EnrichResponse with metadata, discovered IDs, and images.
    Enrich(ctx context.Context, req *EnrichRequest) (*EnrichResponse, error)
}

// EnricherCapabilities describes what an enricher provides and its operational characteristics.
type EnricherCapabilities struct {
    // MediaTypes this enricher supports (e.g., ["movie", "tv", "music"])
    MediaTypes []enrichment.MediaType

    // Provides describes what data this enricher can produce
    Provides []string // "metadata", "artwork", "external_ids"

    // IsLocal indicates whether this enricher operates locally (high concurrency)
    // or remotely (rate limited)
    IsLocal bool

    // RateLimit is requests per minute for remote enrichers (0 = unlimited)
    RateLimit int

    // Requires lists external IDs this enricher needs (e.g., ["imdb", "tmdb"])
    // If empty, enricher can work from title/year alone
    Requires []string
}

// EnrichRequest contains all information needed to enrich a media item.
type EnrichRequest struct {
    MediaID   int64
    MediaType enrichment.MediaType
    FilePath  string

    // Parsed metadata (from filename or previous enrichers)
    Title string
    Year  int

    // For TV episodes
    ShowTitle     string
    SeasonNumber  int
    EpisodeNumber int

    // For music
    Artist string
    Album  string

    // External IDs from previous stages (e.g., {"imdb": "tt0133093", "tmdb": "603"})
    ExistingIDs map[string]string
}

// EnrichResponse contains the results of enrichment.
type EnrichResponse struct {
    // Matched indicates whether the enricher found a match
    Matched bool

    // Metadata contains field updates (only non-nil fields are applied)
    Metadata *EnrichedMetadata

    // DiscoveredIDs contains external IDs found by this enricher
    // e.g., {"imdb": "tt0133093", "tmdb": "603"}
    DiscoveredIDs map[string]string

    // Images contains paths or URLs to discovered images
    Images []EnrichedImage

    // Skipped indicates the enricher intentionally skipped this item
    // (e.g., NFO file not found - not an error, just nothing to do)
    Skipped bool
    SkipReason string
}

// EnrichedMetadata contains metadata fields that can be updated.
// Only non-nil/non-zero fields are applied to the media record.
type EnrichedMetadata struct {
    Title         *string
    OriginalTitle *string
    SortTitle     *string
    Year          *int
    Plot          *string
    Tagline       *string
    Genre         []string
    Director      *string
    Cast          []string
    ContentRating *string
    RuntimeMinutes *int
    // ... additional fields as needed
}

// EnrichedImage represents an image discovered by an enricher.
type EnrichedImage struct {
    Type     string // "poster", "fanart", "banner", "thumb"
    Path     string // Local file path or remote URL
    IsRemote bool   // Whether Path is a URL to download
    Width    int
    Height   int
}
```

**2.2 Wire EnrichmentEnqueuer into Scanner**

File: `internal/application/library/scan_orchestrator.go`

```go
// Add to ScanLibraryUseCase struct:
enrichmentEnqueuer scanmedia.EnrichmentEnqueuer

// Update NewScanLibraryUseCase signature and initialization:
func NewScanLibraryUseCase(
    // ... existing params ...
    enrichmentEnqueuer scanmedia.EnrichmentEnqueuer, // NEW
) *ScanLibraryUseCase {
    return &ScanLibraryUseCase{
        // ... existing fields ...
        enrichmentEnqueuer: enrichmentEnqueuer,
    }
}

// Update mediaDeps():
func (uc *ScanLibraryUseCase) mediaDeps() *scanmedia.Deps {
    return &scanmedia.Deps{
        // ... existing fields ...
        EnrichmentEnqueuer: uc.enrichmentEnqueuer, // WIRE IT UP
    }
}
```

Files to update for wiring:
- `internal/app/usecases/usecases.go` - Pass enrichment enqueuer to ScanLibraryUseCase
- `internal/app/services/services.go` - Ensure PipelineManager is created
- `internal/app/container.go` - Wire dependencies

**2.3 Create NFO Enricher** (`internal/application/enrichment/enrichers/nfo/enricher.go`)

```go
package nfo

import (
    "context"
    "github.com/mantonx/viewra/internal/application/enrichment"
    "github.com/mantonx/viewra/internal/infrastructure/metadata/nfo"  // Direct import!
)

type Enricher struct {
    logger *slog.Logger
}

func New(logger *slog.Logger) *Enricher {
    return &Enricher{logger: logger}
}

func (e *Enricher) Stage() string { return "nfo" }

func (e *Enricher) Capabilities() enrichment.EnricherCapabilities {
    return enrichment.EnricherCapabilities{
        MediaTypes: []enrichment.MediaType{
            enrichment.MediaTypeMovie,
            enrichment.MediaTypeTV,
            enrichment.MediaTypeMusic,
        },
        Provides: []string{"metadata", "external_ids"},
        IsLocal:  true,  // High concurrency, no rate limit
        Requires: nil,   // Can work from filename alone
    }
}

func (e *Enricher) Enrich(ctx context.Context, req *enrichment.EnrichRequest) (*enrichment.EnrichResponse, error) {
    switch req.MediaType {
    case enrichment.MediaTypeMovie:
        return e.enrichMovie(ctx, req)
    case enrichment.MediaTypeTV:
        return e.enrichTVEpisode(ctx, req)
    case enrichment.MediaTypeMusic:
        return e.enrichMusicTrack(ctx, req)
    }
    return &enrichment.EnrichResponse{Skipped: true, SkipReason: "unsupported media type"}, nil
}

func (e *Enricher) enrichMovie(ctx context.Context, req *enrichment.EnrichRequest) (*enrichment.EnrichResponse, error) {
    // Use existing NFO library directly
    nfoPath, err := nfo.FindMovieNFO(req.FilePath)
    if err != nil || nfoPath == "" {
        return &enrichment.EnrichResponse{Skipped: true, SkipReason: "no NFO file found"}, nil
    }

    nfoData, err := nfo.ParseMovieNFO(nfoPath)
    if err != nil {
        return nil, fmt.Errorf("failed to parse NFO: %w", err)
    }

    resp := &enrichment.EnrichResponse{
        Matched:       true,
        Metadata:      convertMovieNFOToMetadata(nfoData),
        DiscoveredIDs: make(map[string]string),
    }

    if nfoData.IMDbID != "" {
        resp.DiscoveredIDs["imdb"] = nfoData.IMDbID
    }
    if nfoData.TMDbID > 0 {
        resp.DiscoveredIDs["tmdb"] = fmt.Sprintf("%d", nfoData.TMDbID)
    }

    return resp, nil
}
```

**2.4 Create Local Images Enricher** (`internal/application/enrichment/enrichers/localimages/enricher.go`)

```go
package localimages

import (
    "context"
    "github.com/mantonx/viewra/internal/application/enrichment"
    "github.com/mantonx/viewra/internal/infrastructure/images/extractor"  // Direct import!
)

type Enricher struct {
    movieExtractor   extractor.MovieExtractor
    episodeExtractor extractor.EpisodeExtractor
    // ... other extractors
    logger *slog.Logger
}

func (e *Enricher) Stage() string { return "local_images" }

func (e *Enricher) Capabilities() enrichment.EnricherCapabilities {
    return enrichment.EnricherCapabilities{
        MediaTypes: []enrichment.MediaType{
            enrichment.MediaTypeMovie,
            enrichment.MediaTypeTV,
            enrichment.MediaTypeMusic,
        },
        Provides: []string{"artwork"},
        IsLocal:  true,  // High concurrency
        Requires: nil,   // Works from file path alone
    }
}

func (e *Enricher) Enrich(ctx context.Context, req *enrichment.EnrichRequest) (*enrichment.EnrichResponse, error) {
    // Use existing image extractor infrastructure
    // This extracts poster.jpg, fanart.jpg, embedded album art, etc.
    // ...
}
```

**2.5 Update Pipeline Manager** (`internal/application/enrichment/pipeline/manager.go`)

Replace `StageProcessor` references with `Enricher`:

```go
// Change from:
type Manager struct {
    processors map[string]StageProcessor
}

// Change to:
type Manager struct {
    enrichers map[string]enrichment.Enricher
}

func (m *Manager) RegisterEnricher(e enrichment.Enricher) {
    m.enrichers[e.Stage()] = e
    // Configure worker pool based on capabilities
    if e.Capabilities().IsLocal {
        m.configureLocalWorkers(e.Stage())
    } else {
        m.configureRemoteWorkers(e.Stage(), e.Capabilities().RateLimit)
    }
}
```

**2.6 Refactor Scanner: Remove Inline NFO Parsing**

File: `internal/application/library/scan/media/movie.go`

```go
// BEFORE (lines 72-103):
nfoPath, err := nfo.FindMovieNFO(result.FilePath)
if err == nil && nfoPath != "" {
    nfoMetadata, err := nfo.ParseMovieNFO(nfoPath)
    // ... 30 lines of field assignment
}

// AFTER:
// Remove entire NFO block. Movie is saved with filename-parsed metadata only.
// NFO enricher will update it asynchronously.
```

File: `internal/application/library/scan/media/tv.go`

```go
// BEFORE (lines 121-151):
nfoPath, err := nfo.FindEpisodeNFO(result.FilePath)
// ... NFO parsing

// AFTER:
// Remove entire NFO block.
```

**2.7 Refactor Scanner: Remove Inline Image Extraction**

File: `internal/application/library/scan/media/movie.go`

```go
// BEFORE:
PostSave: func(ctx context.Context) {
    ExtractImagesForMovie(ctx, deps, movie, result.FilePath)  // REMOVE
    PersistMediaTracks(ctx, deps, movie.Media.ID, result)
    enqueueForEnrichment(ctx, deps, movie.Media.ID, enrichment.MediaTypeMovie)
}

// AFTER:
PostSave: func(ctx context.Context) {
    PersistMediaTracks(ctx, deps, movie.Media.ID, result)  // Keep track persistence
    enqueueForEnrichment(ctx, deps, movie.Media.ID, enrichment.MediaTypeMovie)
}
```

**2.8 Break TV Show NFO/Image Coupling**

File: `internal/application/library/scan/media/images.go`

The function `extractTVShowAndSeasonImages()` currently calls `EnrichTVShowMetadataFromNFO()`. This coupling needs to be broken:

```go
// BEFORE (lines 122-126):
if deps.ProcessedShows.TryMark(showTitle) {
    EnrichTVShowMetadataFromNFO(ctx, deps, show.ID, episodeFilePath)
}

// AFTER:
// Remove this call entirely. TV show metadata enrichment happens via:
// 1. Episode is scanned → enqueued for enrichment
// 2. NFO enricher processes episode → updates episode metadata
// 3. NFO enricher also checks for tvshow.nfo → updates show metadata
// This happens in the NFO enricher, not during image extraction
```

#### Files to Modify (Complete List)

| File | Change |
|------|--------|
| `internal/application/enrichment/enricher.go` | NEW: Define `Enricher` interface |
| `internal/application/enrichment/enrichers/nfo/enricher.go` | NEW: NFO enricher |
| `internal/application/enrichment/enrichers/localimages/enricher.go` | NEW: Local images enricher |
| `internal/application/enrichment/pipeline/deps.go` | Replace `StageProcessor` with `Enricher` |
| `internal/application/enrichment/pipeline/manager.go` | Update to use `Enricher` interface |
| `internal/application/enrichment/pipeline/worker.go` | Update to call `Enricher.Enrich()` |
| `internal/application/library/scan_orchestrator.go` | Add `enrichmentEnqueuer` field, wire in `mediaDeps()` |
| `internal/application/library/scan/media/movie.go` | Remove inline NFO parsing (lines 72-103), remove image extraction from PostSave |
| `internal/application/library/scan/media/tv.go` | Remove inline NFO parsing (lines 121-151), remove image extraction from PostSave |
| `internal/application/library/scan/media/music.go` | Remove image extraction from PostSave |
| `internal/application/library/scan/media/images.go` | Remove `EnrichTVShowMetadataFromNFO` call from image extraction |
| `internal/app/usecases/usecases.go` | Pass enrichment enqueuer to ScanLibraryUseCase |
| `internal/app/services/services.go` | Create and register enrichers with PipelineManager |

#### Code to Remove (Technical Debt Cleanup)

After Phase 2, these become dead code and should be removed:

| File | Code to Remove |
|------|----------------|
| `internal/application/library/scan/media/images.go` | `ExtractImagesForMovie()`, `ExtractImagesForEpisode()`, `ExtractImagesForTrack()` (move logic to enricher) |
| `internal/application/library/scan/media/tv.go` | `EnrichTVShowMetadataFromNFO()` (move to NFO enricher) |
| `internal/application/library/scan_orchestrator.go` | `enrichTVShowMetadataFromNFO()` wrapper method |

#### Testing Strategy

1. **Unit tests for enrichers**: Test NFO and Local Images enrichers in isolation
2. **Integration test**: Scan library → verify enrichment queue populated → verify enrichers called
3. **Regression test**: Ensure media items end up with same metadata as before (just async)
4. **Performance test**: Verify scan completes faster (no blocking on NFO/images)

### Phase 3: External Plugin Foundation (4-5 days)

#### Phase 3 Overview

This phase builds the go-plugin + gRPC infrastructure for **external plugins**. All plugins that make network requests (TMDb, MusicBrainz, Fanart.tv) run as separate processes for isolation and stability.

**Key insight**: The go-plugin infrastructure must be built BEFORE any API-based enrichers, because those enrichers ARE external plugins.

#### Prerequisites

- Phase 2 complete (Enricher interface, built-in enrichers working)
- Protocol Buffers toolchain installed

#### Why External Plugins for API Enrichers?

| Concern | In-Process Problem | External Plugin Solution |
|---------|-------------------|-------------------------|
| Network failures | Can hang/block worker pools | Plugin timeout doesn't affect host |
| API rate limits | Hard to manage across shared pool | Each plugin manages independently |
| Crashes | Brings down ViewRA | Plugin restarts, host continues |
| Updates | Requires ViewRA restart | Hot-swap plugin binary |
| Community | Must fork ViewRA | Distribute standalone binary |

#### Implementation Tasks

**3.1 Proto Definitions** (`api/proto/plugin/`)

```protobuf
// plugin_core.proto
syntax = "proto3";
package viewra.plugin.v1;

// Plugin identity comes from plugin.yml manifest file.
// The host reads the manifest before starting the plugin binary.
service PluginCore {
  rpc Initialize(InitRequest) returns (InitResponse);
  rpc Shutdown(Empty) returns (Empty);
  rpc HealthCheck(Empty) returns (HealthStatus);
  rpc GetSettingsSchema(Empty) returns (SettingsSchema);
  rpc Configure(Settings) returns (ConfigureResponse);
  rpc GetSubscriptions(Empty) returns (EventSubscriptions);
  rpc OnEvent(Event) returns (EventResponse);
}

message InitRequest {
  string host_version = 1;
  string data_dir = 2;
  bytes config = 3;  // Contents of config.yml
}
```

```protobuf
// enricher.proto
syntax = "proto3";
package viewra.plugin.v1;

service Enricher {
  rpc GetCapabilities(Empty) returns (EnricherCapabilities);
  rpc Enrich(EnrichRequest) returns (EnrichResponse);
}

message EnricherCapabilities {
  repeated string media_types = 1;
  repeated string provides = 2;
  bool is_local = 3;
  int32 rate_limit = 4;
  repeated string requires = 5;
}

message EnrichRequest {
  int64 media_id = 1;
  string media_type = 2;
  string file_path = 3;
  string title = 4;
  int32 year = 5;
  map<string, string> existing_ids = 6;
  // TV/Music specific fields...
}

message EnrichResponse {
  bool matched = 1;
  EnrichedMetadata metadata = 2;
  map<string, string> discovered_ids = 3;
  repeated EnrichedImage images = 4;
  bool skipped = 5;
  string skip_reason = 6;
}
```

**3.2 Plugin Manager** (`internal/infrastructure/plugins/manager.go`)

```go
package plugins

import (
    "github.com/hashicorp/go-plugin"
)

type Manager struct {
    plugins    map[string]*PluginInstance
    pluginDir  string
    logger     *slog.Logger
    warmPool   *WarmPool
}

type PluginInstance struct {
    ID       string
    Client   *plugin.Client
    Protocol plugin.ClientProtocol
    Enricher enrichment.Enricher  // gRPC wrapper
    Health   HealthStatus
}

func (m *Manager) LoadPlugin(path string) (*PluginInstance, error) {
    client := plugin.NewClient(&plugin.ClientConfig{
        HandshakeConfig: handshakeConfig,
        Plugins: map[string]plugin.Plugin{
            "enricher": &EnricherGRPCPlugin{},
        },
        Cmd:              exec.Command(path),
        AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
        Logger:           m.logger,
    })

    rpcClient, err := client.Client()
    if err != nil {
        return nil, err
    }

    raw, err := rpcClient.Dispense("enricher")
    if err != nil {
        return nil, err
    }

    return &PluginInstance{
        Client:   client,
        Enricher: raw.(enrichment.Enricher),
    }, nil
}
```

**3.3 gRPC Enricher Wrapper** (`internal/infrastructure/plugins/grpc_enricher.go`)

```go
package plugins

// GRPCEnricher wraps a gRPC client to implement the Enricher interface.
// Plugin identity (stage name) comes from the manifest, not RPC.
type GRPCEnricher struct {
    pluginID string  // From manifest
    client   pb.EnricherClient
}

func (e *GRPCEnricher) Stage() string {
    return e.pluginID  // From manifest, no RPC needed
}

func (e *GRPCEnricher) Capabilities() enrichment.EnricherCapabilities {
    caps, _ := e.client.GetCapabilities(context.Background(), &pb.Empty{})
    return convertProtoCapabilities(caps)
}

func (e *GRPCEnricher) Enrich(ctx context.Context, req *enrichment.EnrichRequest) (*enrichment.EnrichResponse, error) {
    protoReq := convertToProtoRequest(req)
    protoResp, err := e.client.Enrich(ctx, protoReq)
    if err != nil {
        return nil, err
    }
    return convertFromProtoResponse(protoResp), nil
}
```

**3.4 Host Services** (`internal/infrastructure/plugins/host_services.go`)

```go
package plugins

// HostDataServer implements the HostData gRPC service
type HostDataServer struct {
    pb.UnimplementedHostDataServer
    mediaRepo media.Repository
}

func (s *HostDataServer) GetMedia(ctx context.Context, req *pb.MediaQuery) (*pb.Media, error) {
    // Validate permissions from context
    // Fetch and return media
}

// HostStorageServer implements plugin key-value storage
type HostStorageServer struct {
    pb.UnimplementedHostStorageServer
    store PluginStorage
}

func (s *HostStorageServer) KVGet(ctx context.Context, req *pb.KVKey) (*pb.KVValue, error) {
    // Scoped to plugin ID from context
}
```

**3.5 SDK Base Package** (`pkg/plugin/sdk/`)

```go
package sdk

// Base provides common functionality for all plugins.
// Plugin authors must embed this in their plugin struct.
type Base struct {
    logger    *slog.Logger
    requestID string
    metrics   *MetricsCollector
    dataDir   string
}

// mustEmbedBase forces plugins to embed Base
func (Base) mustEmbedBase() {}

func (b *Base) Log() *slog.Logger {
    return b.logger.With("request_id", b.requestID)
}

func (b *Base) Init(dataDir string) {
    b.dataDir = dataDir
}

// EnricherPlugin is the interface plugins must implement.
// Plugin identity comes from plugin.yml manifest file, not code.
type EnricherPlugin interface {
    mustEmbedBase()
    GetCapabilities() EnricherCapabilities
    Initialize(ctx context.Context, dataDir string, config []byte) error
    Shutdown(ctx context.Context) error
    Enrich(ctx context.Context, req *EnrichRequest) (*EnrichResponse, error)
}
```

#### Files to Create

| File | Purpose |
|------|---------|
| `api/proto/plugin/plugin_core.proto` | Core plugin protocol |
| `api/proto/plugin/enricher.proto` | Enricher protocol |
| `api/proto/plugin/host_services.proto` | Host services protocol |
| `internal/infrastructure/plugins/manager.go` | Plugin lifecycle management |
| `internal/infrastructure/plugins/grpc_enricher.go` | gRPC → Enricher adapter |
| `internal/infrastructure/plugins/host_services.go` | Host service implementations |
| `internal/infrastructure/plugins/warm_pool.go` | Plugin warm pool management |
| `pkg/plugin/sdk/base.go` | SDK base struct |
| `pkg/plugin/sdk/enricher.go` | SDK enricher helpers |

#### Plugin Directory Structure

```text
data/plugins/
├── installed/
│   ├── fanart-tv/
│   │   ├── plugin.yaml
│   │   └── fanart-tv           # Binary
│   └── opensubtitles/
│       ├── plugin.yaml
│       └── opensubtitles       # Binary
└── storage/
    ├── fanart-tv/
    │   └── cache.db
    └── opensubtitles/
        └── cache.db
```

---

### Phase 4: API-Based Enricher Plugins (4-5 days)

#### Phase 4 Overview

This phase implements the first external plugins: TMDb and MusicBrainz. These run as **separate go-plugin processes**, demonstrating the full plugin lifecycle.

#### Prerequisites

- Phase 3 complete (go-plugin infrastructure, SDK)
- `media_external_ids` table for ID propagation
- `media_metadata_sources` table for source tracking

#### Implementation Tasks

**4.1 TMDb Plugin** (`plugins/tmdb/`)

This is a standalone Go binary that implements the Enricher gRPC service. Plugin identity comes from `plugin.yml`, not code constants.

**plugin.yml:**

```yaml
id: tmdb
name: TMDb
version: 1.0.0
description: Fetch movie and TV metadata from The Movie Database
author: ViewRA Team
categories:
  - enricher
permissions:
  - network
  - storage:database
```

**main.go:**

```go
// plugins/tmdb/main.go
package main

import (
    "github.com/hashicorp/go-plugin"
    "github.com/mantonx/viewra/pkg/plugin/sdk"
)

// TMDbPlugin implements the Enricher interface.
// Plugin identity comes from plugin.yml manifest.
type TMDbPlugin struct {
    sdk.Base
    client *tmdb.Client
}

func (p *TMDbPlugin) GetCapabilities() sdk.EnricherCapabilities {
    return sdk.EnricherCapabilities{
        MediaTypes: []string{"movie", "tv"},
        Provides:   []string{"metadata", "artwork", "external_ids"},
        IsLocal:    false,
        RateLimit:  40,  // TMDb: ~40 req/10s
    }
}

func (p *TMDbPlugin) Initialize(ctx context.Context, dataDir string, config []byte) error {
    var cfg Config
    if err := yaml.Unmarshal(config, &cfg); err != nil {
        return err
    }
    p.client = tmdb.NewClient(cfg.APIKey)
    return nil
}

func (p *TMDbPlugin) Shutdown(ctx context.Context) error {
    return nil
}

func (p *TMDbPlugin) Enrich(ctx context.Context, req *sdk.EnrichRequest) (*sdk.EnrichResponse, error) {
    // 1. Try lookup by IMDB ID (faster, more accurate)
    if imdbID, ok := req.ExistingIDs["imdb"]; ok && imdbID != "" {
        return p.lookupByIMDb(ctx, imdbID, req.MediaType)
    }

    // 2. Fall back to search by title/year
    return p.searchByTitleYear(ctx, req)
}

func main() {
    plugin.Serve(&plugin.ServeConfig{
        HandshakeConfig: sdk.Handshake,
        Plugins: map[string]plugin.Plugin{
            "enricher": &sdk.EnricherGRPCPlugin{Impl: &TMDbPlugin{}},
        },
        GRPCServer: plugin.DefaultGRPCServer,
    })
}
```

**4.2 MusicBrainz Plugin** (`plugins/musicbrainz/`)

```go
// plugins/musicbrainz/main.go
package main

type MusicBrainzPlugin struct {
    sdk.Base
    client *musicbrainz.Client
}

func (p *MusicBrainzPlugin) Capabilities() *sdk.EnricherCapabilities {
    return &sdk.EnricherCapabilities{
        MediaTypes: []string{"music"},
        Provides:   []string{"metadata", "external_ids"},
        IsLocal:    false,
        RateLimit:  1,  // MusicBrainz: strict 1 req/sec
    }
}

func (p *MusicBrainzPlugin) Enrich(ctx context.Context, req *sdk.EnrichRequest) (*sdk.EnrichResponse, error) {
    // Lookup by MusicBrainz ID, or search by artist/album/track
    // ...
}
```

**4.3 Plugin-Local Caching**

Each plugin manages its own cache via the SQLite database provided by Host Services:

```go
// Plugin requests its database path from host
dbPath, _ := p.HostStorage().GetDatabasePath(ctx, &pb.Empty{})

// Plugin manages its own schema
db, _ := sql.Open("sqlite3", dbPath.Path)
db.Exec(`CREATE TABLE IF NOT EXISTS cache (
    key TEXT PRIMARY KEY,
    response BLOB,
    expires_at INTEGER
)`)
```

**4.4 Metadata Merging** (`internal/application/enrichment/pipeline/merger.go`)

After each enricher completes, the host merges results:

```go
func MergeEnrichResponse(ctx context.Context, repos *Repositories, mediaID int64, resp *EnrichResponse, stage string) error {
    // 1. Update media record with non-nil metadata fields
    if resp.Metadata != nil {
        if err := updateMediaFields(ctx, repos, mediaID, resp.Metadata); err != nil {
            return err
        }
    }

    // 2. Store external IDs for downstream stages
    for provider, id := range resp.DiscoveredIDs {
        if err := repos.ExternalIDs.Upsert(ctx, mediaID, provider, id); err != nil {
            return err
        }
    }

    // 3. Track metadata sources for transparency
    for field := range resp.Metadata.NonNilFields() {
        if err := repos.MetadataSources.Upsert(ctx, mediaID, field, stage); err != nil {
            return err
        }
    }

    return nil
}
```

#### Files to Create

| File | Purpose |
|------|---------|
| `plugins/tmdb/main.go` | TMDb plugin entry point |
| `plugins/tmdb/client.go` | TMDb API client |
| `plugins/tmdb/cache.go` | Local response caching |
| `plugins/musicbrainz/main.go` | MusicBrainz plugin entry point |
| `plugins/musicbrainz/client.go` | MusicBrainz API client |
| `internal/application/enrichment/pipeline/merger.go` | Metadata merging logic |

#### Build & Distribution

```bash
# Build plugins as separate binaries
go build -o bin/plugins/tmdb ./plugins/tmdb
go build -o bin/plugins/musicbrainz ./plugins/musicbrainz

# Plugins are distributed alongside ViewRA
bin/
├── viewra
├── subtitle-extractor
└── plugins/
    ├── tmdb
    └── musicbrainz
```

#### Configuration

```yaml
# config.yaml
plugins:
  tmdb:
    enabled: true
    api_key: "${TMDB_API_KEY}"
  musicbrainz:
    enabled: true
    # No API key needed for MusicBrainz
```

---

### Phase 5: Events & Notifications (3-4 days)

#### Phase 5 Overview

This phase enables plugins to receive events and implements the NotificationSink category for webhooks, Discord, etc.

#### Prerequisites

- Phase 3-4 complete (plugin infrastructure, example plugins working)
- Event Bus from Phase 1

#### Implementation Tasks

**5.1 Event Delivery to Plugins** (`internal/infrastructure/plugins/event_dispatcher.go`)

```go
package plugins

type EventDispatcher struct {
    bus      *events.Bus
    manager  *Manager
    logger   *slog.Logger
}

func (d *EventDispatcher) Start(ctx context.Context) {
    sub := d.bus.Subscribe(events.WithFilter(func(e events.Event) bool {
        return e.Category == events.CategoryDomain
    }))

    go func() {
        for event := range sub.Events() {
            d.dispatchToPlugins(ctx, event)
        }
    }()
}

func (d *EventDispatcher) dispatchToPlugins(ctx context.Context, event events.Event) {
    for _, plugin := range d.manager.GetPluginsSubscribedTo(event.Type) {
        go func(p *PluginInstance) {
            if err := p.OnEvent(ctx, event); err != nil {
                d.logger.Warn("plugin event delivery failed",
                    "plugin", p.ID,
                    "event", event.Type,
                    "error", err)
            }
        }(plugin)
    }
}
```

**5.2 NotificationSink Interface** (`api/proto/plugin/notification.proto`)

```protobuf
service NotificationSink {
  rpc GetCapabilities(Empty) returns (NotificationCapabilities);
  rpc SendNotification(Notification) returns (NotificationResult);
}

message NotificationCapabilities {
  repeated string supported_events = 1;  // "playback.started", "media.added", etc.
  bool supports_rich_content = 2;
  int32 rate_limit = 3;
}

message Notification {
  string event_type = 1;
  google.protobuf.Timestamp timestamp = 2;
  oneof payload {
    MediaEvent media_event = 3;
    PlaybackEvent playback_event = 4;
    LibraryEvent library_event = 5;
  }
}
```

**5.3 Webhook Plugin (Example)** (`examples/plugins/webhook/`)

```go
package main

import (
    "github.com/mantonx/viewra/pkg/plugin/sdk"
)

type WebhookPlugin struct {
    sdk.Base
    webhookURL string
    client     *http.Client
}

func (p *WebhookPlugin) SendNotification(ctx context.Context, n *sdk.Notification) (*sdk.NotificationResult, error) {
    payload, _ := json.Marshal(n)
    resp, err := p.client.Post(p.webhookURL, "application/json", bytes.NewReader(payload))
    if err != nil {
        return &sdk.NotificationResult{Success: false, Error: err.Error()}, nil
    }
    defer resp.Body.Close()

    return &sdk.NotificationResult{
        Success:    resp.StatusCode >= 200 && resp.StatusCode < 300,
        StatusCode: int32(resp.StatusCode),
    }, nil
}

func main() {
    plugin.Serve(&plugin.ServeConfig{
        HandshakeConfig: sdk.Handshake,
        Plugins: map[string]plugin.Plugin{
            "notification_sink": &sdk.NotificationSinkPlugin{Impl: &WebhookPlugin{}},
        },
        GRPCServer: plugin.DefaultGRPCServer,
    })
}
```

**5.4 Playback Events Integration**

Connect streaming/transcode events to the event bus:

```go
// internal/infrastructure/streaming/events.go
func (s *Service) emitPlaybackStarted(ctx context.Context, mediaID int64, userID string) {
    s.eventBus.Publish(events.Event{
        Type:     events.PlaybackStarted,
        Category: events.CategoryDomain,
        Payload: events.PlaybackPayload{
            MediaID:   mediaID,
            UserID:    userID,
            Timestamp: time.Now(),
        },
    })
}
```

#### Files to Create

| File | Purpose |
|------|---------|
| `api/proto/plugin/notification.proto` | Notification protocol |
| `internal/infrastructure/plugins/event_dispatcher.go` | Event delivery to plugins |
| `internal/infrastructure/streaming/events.go` | Playback event emission |
| `examples/plugins/webhook/main.go` | Example webhook plugin |
| `pkg/plugin/sdk/notification.go` | SDK notification helpers |

---

### Phase 6: Observability (3-4 days)

#### Phase 6 Overview

This phase adds comprehensive debugging, monitoring, and diagnostic capabilities across the plugin system.

#### Implementation Tasks

**6.1 Correlation IDs** (`internal/infrastructure/plugins/correlation.go`)

```go
package plugins

// InjectCorrelationID adds request ID to outgoing gRPC calls
func InjectCorrelationID(ctx context.Context) context.Context {
    requestID := middleware.GetRequestID(ctx)
    if requestID == "" {
        requestID = uuid.New().String()
    }
    return metadata.AppendToOutgoingContext(ctx, "x-request-id", requestID)
}

// ExtractCorrelationID gets request ID from incoming gRPC calls (SDK side)
func ExtractCorrelationID(ctx context.Context) string {
    md, ok := metadata.FromIncomingContext(ctx)
    if !ok {
        return ""
    }
    values := md.Get("x-request-id")
    if len(values) > 0 {
        return values[0]
    }
    return ""
}
```

**6.2 gRPC Debug Mode** (`internal/infrastructure/plugins/debug.go`)

```go
package plugins

type DebugInterceptor struct {
    enabled bool
    logger  *slog.Logger
}

func (d *DebugInterceptor) UnaryClientInterceptor() grpc.UnaryClientInterceptor {
    return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
        if d.enabled {
            d.logger.Debug("gRPC request",
                "method", method,
                "request", fmt.Sprintf("%+v", req))
        }

        err := invoker(ctx, method, req, reply, cc, opts...)

        if d.enabled {
            d.logger.Debug("gRPC response",
                "method", method,
                "response", fmt.Sprintf("%+v", reply),
                "error", err)
        }

        return err
    }
}
```

Enable with: `VIEWRA_PLUGIN_DEBUG=1`

**6.3 Plugin Health Monitoring** (`internal/infrastructure/plugins/health.go`)

```go
package plugins

type HealthMonitor struct {
    manager   *Manager
    interval  time.Duration
    eventBus  *events.Bus
    logger    *slog.Logger
}

type PluginHealth struct {
    PluginID      string
    Status        HealthStatus  // healthy, degraded, unhealthy, crashed
    LastHeartbeat time.Time
    ErrorRate     float64       // Errors per minute (rolling window)
    AvgLatency    time.Duration
    Restarts      int
}

func (m *HealthMonitor) Start(ctx context.Context) {
    ticker := time.NewTicker(m.interval)
    for {
        select {
        case <-ticker.C:
            m.checkAllPlugins(ctx)
        case <-ctx.Done():
            return
        }
    }
}

func (m *HealthMonitor) checkAllPlugins(ctx context.Context) {
    for _, plugin := range m.manager.ListPlugins() {
        health := m.checkPlugin(ctx, plugin)
        if health.Status != plugin.Health.Status {
            m.eventBus.Publish(events.Event{
                Type:    events.PluginHealthUpdate,
                Payload: health,
            })
        }
        plugin.Health = health
    }
}
```

**6.4 Error Categorization** (`internal/application/enrichment/errors.go`)

```go
package enrichment

type ErrorCategory string

const (
    ErrorCategoryNetwork   ErrorCategory = "network"
    ErrorCategoryRateLimit ErrorCategory = "rate_limit"
    ErrorCategoryNotFound  ErrorCategory = "not_found"
    ErrorCategoryParsing   ErrorCategory = "parsing"
    ErrorCategoryPlugin    ErrorCategory = "plugin"
    ErrorCategoryInternal  ErrorCategory = "internal"
)

type CategorizedError struct {
    Category ErrorCategory
    Message  string
    Cause    error
    Retryable bool
}

func CategorizeError(err error) *CategorizedError {
    // Analyze error and categorize
    if isNetworkError(err) {
        return &CategorizedError{
            Category:  ErrorCategoryNetwork,
            Retryable: true,
            Cause:     err,
        }
    }
    // ... more categorization
}
```

**6.5 Diagnostic Export** (`internal/infrastructure/diagnostics/export.go`)

```go
package diagnostics

type Exporter struct {
    pluginManager *plugins.Manager
    pipelineManager *pipeline.Manager
    eventBus      *events.Bus
    systemInfo    *system.Profile
}

func (e *Exporter) Export(ctx context.Context, outputPath string) error {
    bundle := &DiagnosticBundle{
        Timestamp: time.Now(),
        System:    e.collectSystemInfo(),
        Plugins:   e.collectPluginInfo(ctx),
        Pipelines: e.collectPipelineConfig(ctx),
        Queue:     e.collectQueueStatus(ctx),
        Errors:    e.collectRecentErrors(ctx),
        Logs:      e.collectRecentLogs(),
    }

    return e.writeBundle(bundle, outputPath)
}

// Output: viewra-diagnostics-2025-12-14.zip
```

#### Files to Create

| File | Purpose |
|------|---------|
| `internal/infrastructure/plugins/correlation.go` | Request ID propagation |
| `internal/infrastructure/plugins/debug.go` | gRPC debug interceptor |
| `internal/infrastructure/plugins/health.go` | Health monitoring |
| `internal/application/enrichment/errors.go` | Error categorization |
| `internal/infrastructure/diagnostics/export.go` | Diagnostic bundle export |

---

### Phase 7: UI & Polish (3-4 days)

#### Phase 7 Overview

This phase adds user-facing UI for pipeline configuration, progress visibility, and plugin management.

#### Implementation Tasks

**7.1 Pipeline Configuration API** (`internal/api/handlers/pipeline.go`)

```go
package handlers

type PipelineHandler struct {
    pipelineRepo enrichment.PipelineRepository
    manager      *pipeline.Manager
}

// GET /api/pipelines/:mediaType
func (h *PipelineHandler) GetPipeline(c *gin.Context) {
    mediaType := c.Param("mediaType")
    stages, err := h.pipelineRepo.GetPipelineStages(c, mediaType)
    // Return ordered list of stages with enabled status
}

// PUT /api/pipelines/:mediaType
func (h *PipelineHandler) UpdatePipeline(c *gin.Context) {
    var req UpdatePipelineRequest
    // Validate, update order, enable/disable stages
}

// POST /api/pipelines/:mediaType/apply
func (h *PipelineHandler) ApplyChanges(c *gin.Context) {
    var req ApplyChangesRequest
    // Scope: "new_only", "missing_stages", "full_library"
    // Enqueue affected media items for re-enrichment
}
```

**7.2 Progress Visibility API** (`internal/api/handlers/enrichment_progress.go`)

```go
package handlers

// GET /api/libraries/:id/enrichment/progress
func (h *EnrichmentHandler) GetLibraryProgress(c *gin.Context) {
    libraryID := c.Param("id")

    progress := h.statusRepo.GetLibraryEnrichmentProgress(c, libraryID)
    // Returns:
    // {
    //   "total_items": 2847,
    //   "stages": [
    //     {"stage": "nfo", "completed": 2847, "pending": 0, "failed": 0},
    //     {"stage": "local_images", "completed": 2834, "pending": 0, "failed": 13},
    //     {"stage": "tmdb", "completed": 1203, "pending": 1644, "failed": 0},
    //     {"stage": "fanart", "completed": 0, "pending": 2847, "failed": 0}
    //   ]
    // }
}

// GET /api/media/:id/enrichment/status
func (h *EnrichmentHandler) GetMediaEnrichmentStatus(c *gin.Context) {
    mediaID := c.Param("id")

    status := h.statusRepo.GetMediaEnrichmentStatus(c, mediaID)
    // Returns per-stage status with timestamps, errors, sources
}
```

**7.3 Frontend Components** (`web/src/components/`)

```typescript
// PipelineConfig.tsx
interface PipelineStage {
  id: string;
  name: string;
  enabled: boolean;
  position: number;
  isLocal: boolean;
  status: 'healthy' | 'degraded' | 'error';
}

export function PipelineConfig({ mediaType }: { mediaType: string }) {
  const { data: stages } = useQuery(['pipeline', mediaType], () =>
    api.getPipeline(mediaType)
  );

  // Drag-and-drop reordering
  // Enable/disable toggles
  // Apply changes with scope selector
}

// EnrichmentProgress.tsx
export function EnrichmentProgress({ libraryId }: { libraryId: number }) {
  const { data: progress } = useQuery(
    ['enrichment-progress', libraryId],
    () => api.getEnrichmentProgress(libraryId),
    { refetchInterval: 5000 }
  );

  // Progress bars per stage
  // ETA calculation
  // Error counts with drill-down
}
```

**7.4 Plugin CLI Commands** (`cmd/viewra/plugin.go`)

```go
// viewra plugin list
// viewra plugin install <name-or-url>
// viewra plugin uninstall <name>
// viewra plugin update [name]
// viewra plugin info <name>

func pluginListCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "list",
        Short: "List installed plugins",
        Run: func(cmd *cobra.Command, args []string) {
            plugins := manager.ListPlugins()
            table := tablewriter.NewWriter(os.Stdout)
            table.SetHeader([]string{"Name", "Version", "Category", "Status"})
            for _, p := range plugins {
                table.Append([]string{p.Name, p.Version, p.Category, p.Health.Status})
            }
            table.Render()
        },
    }
}
```

**7.5 Settings UI Integration**

Add plugin settings to the existing settings page:

```typescript
// web/src/pages/Settings/Plugins.tsx
export function PluginSettings() {
  const { data: plugins } = useQuery('plugins', api.getPlugins);

  return (
    <SettingsSection title="Plugins">
      {plugins?.map(plugin => (
        <PluginCard
          key={plugin.id}
          plugin={plugin}
          onConfigure={() => openPluginSettings(plugin.id)}
          onDisable={() => disablePlugin(plugin.id)}
        />
      ))}
    </SettingsSection>
  );
}
```

#### Files to Create

| File | Purpose |
|------|---------|
| `internal/api/handlers/pipeline.go` | Pipeline configuration API |
| `internal/api/routes/pipeline.go` | Pipeline routes |
| `web/src/components/PipelineConfig.tsx` | Pipeline configuration UI |
| `web/src/components/EnrichmentProgress.tsx` | Progress visibility UI |
| `web/src/pages/Settings/Plugins.tsx` | Plugin settings page |
| `cmd/viewra/plugin.go` | Plugin CLI commands |

#### API Endpoints Summary

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/pipelines/:mediaType` | GET | Get pipeline configuration |
| `/api/pipelines/:mediaType` | PUT | Update pipeline order/stages |
| `/api/pipelines/:mediaType/apply` | POST | Apply changes with scope |
| `/api/libraries/:id/enrichment/progress` | GET | Library enrichment progress |
| `/api/media/:id/enrichment/status` | GET | Media item enrichment status |
| `/api/plugins` | GET | List installed plugins |
| `/api/plugins/:id` | GET | Plugin details |
| `/api/plugins/:id/settings` | GET/PUT | Plugin settings |

---

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
| 21 | Manifest | plugin.yml for identity + config.yml for runtime settings (no GetInfo RPC) |
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
| 43 | Built-ins | NFO, LocalImages only (TMDb/MusicBrainz are external plugins) |
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

## Current Implementation Status

As of December 2025, the plugin system is partially implemented. This section tracks what's complete, in progress, and planned.

### Complete

| Component | Location | Notes |
|-----------|----------|-------|
| Plugin Manager | `internal/infrastructure/plugins/manager.go` | Lifecycle, loading, health monitoring |
| Manifest Parser | `internal/infrastructure/plugins/manifest.go` | Reads `plugin.yml`, validates fields |
| gRPC Wiring | `internal/infrastructure/plugins/grpc_*.go` | PluginCore, Enricher, Host services |
| Warm Pool | `internal/infrastructure/plugins/warm_pool.go` | Keep-warm, pre-warming, idle unloading |
| Host Storage (KV) | `internal/infrastructure/plugins/host_storage.go` | Per-plugin key-value with TTL |
| Enrichment Queue | `migrations/000027`, `internal/infrastructure/database/queries/*/enrichment_queue.sql` | Job queue with retry logic |
| Pipeline Manager | `internal/application/enrichment/pipeline/manager.go` | Stage orchestration, worker pools |
| Worker Pools | `internal/application/enrichment/pipeline/worker_pool.go` | Concurrent processing, rate limiting |
| Response Appliers | `internal/application/enrichment/pipeline/*_applier.go` | Metadata, credits, studios, images |
| Plugin KV Queries | `internal/infrastructure/database/queries/*/plugin_kv.sql` | CRUD, expiration cleanup |
| TMDb Plugin | `plugins/tmdb/` | Reference implementation, movie/TV enrichment |
| SDK | `pkg/plugin/sdk/` | Base struct, EnricherPlugin interface |
| Built-in NFO Enricher | `internal/application/enrichment/builtin/nfo.go` | Parses movie, TV, TV show NFO files |
| Built-in Local Images | `internal/application/enrichment/builtin/local_images.go` | Extracts local artwork for all media types |
| Plugins Table | `migrations/000037`, queries in `plugins.sql` | Plugin registry with health tracking |
| Plugin API Keys Table | `migrations/000037`, queries in `plugin_api_keys.sql` | Server-to-server auth |
| Plugin User Metadata | `migrations/000037`, queries in `plugin_user_metadata.sql` | Per-user data storage |
| Enrichment Pipelines FK | `migrations/000038` | FK constraint to plugins table |
| Scanner → Pipeline Integration | `internal/application/library/scan/media/*.go` | Scanner enqueues to pipeline, no inline NFO/images |

### In Progress

| Component | Status | Notes |
|-----------|--------|-------|
| Plugin API endpoints | Planned | REST API to manage plugins from UI |

### Known Gaps

| Gap | Impact | Priority |
|-----|--------|----------|
| **MediaQuerier not implemented** | `host_data.go` defines interface but plugins can't query media beyond EnrichRequest | Medium |
| **External IDs for people/studios** | `media_external_ids` only references `media.id`, can't store TMDB person_id etc. | Medium |
| **Plugin API endpoints** | No REST API to list/enable/disable/configure plugins | Medium |
| **Event system incomplete** | `GetSubscriptions`/`OnEvent` are stubs, no event types defined | Low |
| **GetLibrary stub** | `host_data.go` returns "not implemented" for library queries | Low |

### Implementation Order

Based on dependencies and impact:

1. **MediaQuerier implementation** - Plugins need to query media
2. **External IDs for people/studios** - Enable full enrichment chain
3. **Plugin API endpoints** - UI management
4. **Event system** - Notification plugins

---

## References

- [Hashicorp go-plugin](https://github.com/hashicorp/go-plugin)
- [gRPC Go](https://grpc.io/docs/languages/go/)
- [Protocol Buffers](https://protobuf.dev/)
- Terraform provider architecture (similar pattern)
