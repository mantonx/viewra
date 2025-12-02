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

This ADR documents 51 architectural decisions made through systematic Q&A.

## Decision

Implement a plugin system using Hashicorp's go-plugin library with gRPC for communication.

### Architecture Overview

```text
┌─────────────────────────────────────────────────────────────┐
│                        ViewRA Host                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │    Plugin    │  │    Plugin    │  │      Event       │  │
│  │   Manager    │  │   Registry   │  │   Dispatcher     │  │
│  └──────┬───────┘  └──────┬───────┘  └────────┬─────────┘  │
│         │                 │                    │            │
│         └─────────────────┴────────────────────┘            │
│                           │                                 │
│  ┌────────────────────────┴────────────────────────────┐   │
│  │                  Host Services (gRPC)                │   │
│  │  HostData | HostStorage | HostCache | HostEvents    │   │
│  └────────────────────────┬────────────────────────────┘   │
│                           │                                 │
│                     gRPC over stdio                         │
└───────────────────────────┬─────────────────────────────────┘
                            │
         ┌──────────────────┼──────────────────┐
         │                  │                  │
         ▼                  ▼                  ▼
    ┌─────────┐       ┌──────────┐       ┌─────────┐
    │  TMDb   │       │   NFO    │       │ Webhook │
    │ Plugin  │       │  Plugin  │       │ Plugin  │
    └─────────┘       └──────────┘       └─────────┘
```

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
| MetadataProvider | TMDb, MusicBrainz, NFO | Phase 1 |
| NotificationSink | Webhooks, Discord, Telegram | Phase 2 |
| SubtitleProvider | OpenSubtitles (future) | Deferred |
| AnalyticsProvider | Trakt, Last.fm (future) | Deferred |

---

## Plugin Categories & Services

### MetadataProvider

```protobuf
service MetadataProvider {
  rpc Search(SearchRequest) returns (SearchResponse);
  rpc GetMetadata(MetadataRequest) returns (MetadataResponse);
  rpc GetImages(ImagesRequest) returns (ImagesResponse);
  rpc GetCapabilities(Empty) returns (Capabilities);
}

// Optional extensions for TV
service TVProvider {
  rpc SearchSeries(SeriesSearchRequest) returns (SeriesSearchResponse);
  rpc GetSeasonEpisodes(SeasonRequest) returns (EpisodesResponse);
}

// Optional extensions for Music
service MusicProvider {
  rpc SearchAlbum(AlbumSearchRequest) returns (AlbumSearchResponse);
  rpc GetAlbumTracks(AlbumTracksRequest) returns (TracksResponse);
}
```

### NotificationSink

```protobuf
service NotificationSink {
  rpc SendNotification(Notification) returns (NotificationResult);
  rpc GetCapabilities(Empty) returns (NotificationCapabilities);
}
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

### Concurrency

- Max concurrent plugins: 3 (configurable)
- Max concurrent requests per plugin: 5 (configurable)
- Per-plugin rate limiting for API quotas

---

## Event System

### Available Events

**Library Events:**
- `library.scan.started`, `library.scan.completed`
- `media.added`, `media.updated`, `media.removed`
- `media.matched`

**Playback Events:**
- `playback.started`, `playback.paused`, `playback.resumed`, `playback.stopped`
- `playback.progress` (configurable interval)
- `playback.completed` (95%+ threshold)

**User Events:**
- `user.created`, `user.login`, `user.logout`

**System Events:**
- `server.started`, `server.stopping`
- `transcode.started`, `transcode.completed`, `transcode.failed`

### Event Delivery

- Push via callback RPC (`OnEvent`)
- Queue events when plugin unavailable
- Deliver missed events on reconnect
- Plugin returns acknowledgment or retry hint

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
  - ui:media-tabs           # Add media detail tabs
```

**Category Defaults:**
- MetadataProvider: `network`, `storage:database`, `data:media:read`, `data:metadata:write`
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

### SQLite Database Standards

- Host provides path via `GetDatabasePath()`
- Plugin manages schema and migrations
- Plugin registers schema version with host
- Recommended cache table pattern provided in SDK

### Quotas & Limits

| Setting | Default | Notes |
|---------|---------|-------|
| Max DB size | 100 MB | Configurable per plugin |
| Cache TTL | 7 days | Plugin-configurable |
| Vacuum frequency | Weekly | Automatic |

### Cleanup

- `PrepareForUninstall()` called before removal
- Host deletes entire `data/plugins/<plugin-id>/` directory
- Plugin DBs included in backup (cache tables optional)

---

## Metadata System

### Search Flow

1. Scanner finds new file
2. ViewRA parses filename → `ParsedMedia{title, year, season, episode}`
3. NFO plugin runs first (if available), extracts IDs as hints
4. ViewRA calls `plugin.Search(ParsedMedia, hints)`
5. Plugins return ranked candidates with confidence scores
6. ViewRA selects match (highest confidence above threshold)
7. ViewRA calls `plugin.GetMetadata(matchId)`
8. Results merged according to field-level priority

### NFO Plugin Handling

- Runs first, before other plugins
- Extracts external IDs (TMDb ID, IMDb ID, etc.)
- IDs passed as hints to other plugins (skip search)
- Embedded in ViewRA binary (no subprocess overhead)

### TV Show Matching

1. Match series first → get series ID
2. Get episodes within series context
3. Series metadata inherited to episodes
4. Separate artwork per level (series, season, episode)

### Music Matching

1. Group files by folder (assumed album)
2. Match album first → get album ID
3. Match tracks within album context
4. Audio fingerprinting deferred to future phase

### Metadata Storage

**Merged record + source tracking:**

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

## Distribution & Updates

### Plugin Manifest

Minimal external manifest for pre-load checks:

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

Full capabilities via `GetCapabilities()` RPC after load.

### Registry

- Initial: Curated list in documentation
- Future: Simple registry API at `plugins.viewra.io`
- Eventually: Full marketplace with ratings/reviews

### Installation

```bash
# From registry
viewra plugin install tmdb

# From Git
viewra plugin install github.com/user/my-plugin

# Manual
cp my-plugin /path/to/viewra/plugins/
```

### Updates

- Check on startup (or daily)
- Notify in UI: "3 plugin updates available"
- User manually approves updates
- Future: auto-update for security patches

---

## Built-in Plugins

Following same plugin interface (can be disabled):

| Plugin | Category | Notes |
|--------|----------|-------|
| NFO | MetadataProvider | Local file parsing, runs first |
| TMDb | MetadataProvider | Movies/TV, most popular |
| MusicBrainz | MetadataProvider | Music, most popular |

---

## Development Experience

### SDK

- Go SDK provided (primary)
- Protobuf contracts as source of truth
- Other languages theoretically possible, unsupported

### Testing

1. **Standalone harness**: `viewra-plugin-test ./my-plugin --scenario search`
2. **Dev mode**: Hot-reload plugins during development
3. **Template repository**: Example plugin with CI/CD

### Plugin Isolation

- Process isolation via go-plugin
- No additional sandboxing (v1)
- "Run at your own risk" for third-party plugins

---

## Implementation Phases

### Phase 1: Foundation (4-5 days)

- Plugin manager and process lifecycle
- PluginCore gRPC definitions
- MetadataProvider interface
- Host services (HostData, HostStorage)
- NFO plugin as first implementation
- Permission system

### Phase 2: Metadata Integration (3-4 days)

- Hook into library scanner
- Search flow with confidence scores
- Metadata merging logic (field-level priority)
- External IDs table
- Match locking

### Phase 3: Built-in Providers (4-5 days)

- TMDb plugin for movies/TV
- MusicBrainz plugin for music
- TV series/episode matching
- Music album/track matching
- Remove hardcoded provider code

### Phase 4: Events & Notifications (3-4 days)

- Event dispatcher
- NotificationSink category
- Webhook plugin as example
- Event queue for reliability

### Phase 5: UI Extensions (3-4 days)

- Extension point framework
- JSON schema UI rendering
- React component loading
- Settings page generation

### Phase 6: Polish (2-3 days)

- Plugin CLI commands
- Registry integration
- Update notifications
- Documentation and template repo

**Total estimated: 19-25 days**

---

## Consequences

### Positive

- All integrations use consistent interface
- Third-party extensions without core changes
- Process isolation prevents crashes from affecting host
- Community can contribute plugins
- Clean separation of concerns
- Extensible to new categories

### Negative

- Process overhead for each plugin (mitigated by warm pool)
- gRPC complexity vs direct function calls
- Significant initial implementation effort
- Plugin authors need to learn protobuf/gRPC
- UI extensions add frontend complexity

### Neutral

- Breaking change for any existing metadata customization
- Requires maintaining backward compatibility for plugin protocol

---

## Alternatives Considered

### In-process plugins (Go interfaces)

Simpler but no isolation - a buggy plugin crashes ViewRA.

### HTTP-based plugins

More language-agnostic but higher overhead and complexity.

### Lua/JavaScript embedded scripting

Simpler for basic plugins but limited capability.

### Minimal metadata-only system

Would work but limits future extensibility.

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

---

## References

- [Hashicorp go-plugin](https://github.com/hashicorp/go-plugin)
- [gRPC Go](https://grpc.io/docs/languages/go/)
- [Protocol Buffers](https://protobuf.dev/)
- Terraform provider architecture (similar pattern)
