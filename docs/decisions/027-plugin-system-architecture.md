# ADR 027: Plugin System Architecture

## Status

Proposed

## Date

December 2, 2025

## Context

ViewRA needs to support multiple metadata sources (TMDb, MusicBrainz, NFO files, etc.) in a consistent, extensible way. Rather than hardcoding each provider, we want a plugin architecture that:

1. Allows third-party metadata providers
2. Makes all sources (including NFO) consistent through the same interface
3. Provides isolation and security boundaries
4. Enables community contributions without core code changes

## Decision

Implement a plugin system using Hashicorp's go-plugin library with gRPC for communication.

### Architecture Overview

```text
┌─────────────────────────────────────────────────────────┐
│                      ViewRA Host                        │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐ │
│  │   Plugin    │  │   Plugin    │  │     Plugin      │ │
│  │   Manager   │  │   Registry  │  │   Orchestrator  │ │
│  └──────┬──────┘  └──────┬──────┘  └────────┬────────┘ │
│         │                │                   │          │
│         └────────────────┴───────────────────┘          │
│                          │                              │
│                    gRPC over stdio                      │
└──────────────────────────┬──────────────────────────────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
        ▼                  ▼                  ▼
   ┌─────────┐       ┌──────────┐       ┌─────────┐
   │  TMDb   │       │   NFO    │       │  Custom │
   │ Plugin  │       │  Plugin  │       │ Plugin  │
   └─────────┘       └──────────┘       └─────────┘
   (separate process) (separate process) (separate process)
```

### Plugin Interface

The primary plugin type is `MetadataProvider`:

```protobuf
service MetadataProvider {
  // Search for media by title/query
  rpc Search(SearchRequest) returns (SearchResponse);

  // Get full metadata for a specific item
  rpc GetMetadata(MetadataRequest) returns (MetadataResponse);

  // Get available images (posters, backdrops, etc.)
  rpc GetImages(ImagesRequest) returns (ImagesResponse);

  // Declare plugin capabilities and settings schema
  rpc GetCapabilities(Empty) returns (Capabilities);
}

message Capabilities {
  repeated MediaType supported_types = 1;  // movie, tvshow, music
  SettingsSchema settings_schema = 2;
  string refresh_hint = 3;  // "30d", "7d", etc.
}
```

### Plugin Discovery & Lifecycle

1. **Discovery**: Scan `plugins/` directory for executables with valid manifests
2. **Built-in plugins**: Embedded as executables, extracted on first run
3. **Loading**: Start plugin process on demand, keep warm pool for active plugins
4. **Health**: Heartbeat checks, automatic restart on crash (max 3 retries)
5. **Shutdown**: Graceful shutdown with timeout, then SIGKILL

### Security Model

| Resource | Access |
|----------|--------|
| Core database | None |
| Plugin storage | Isolated key-value store per plugin |
| Filesystem | Read media directories, read/write own plugin dir |
| Network | Unrestricted (allowlist in future if needed) |
| Host services | Limited protobuf API (logging, storage, image cache) |

### Metadata Merging Strategy

Field-level priority configuration allows different sources for different fields:

```yaml
metadata_priority:
  movies:
    # Technical fields from NFO (often more accurate)
    title: [nfo, tmdb]
    plot: [nfo, tmdb]

    # Visual content from TMDb (better images)
    poster: [tmdb, nfo]
    backdrop: [tmdb]

    # Ratings from multiple sources
    rating: [tmdb, nfo]
```

### Error Handling

Smart retry/fallback strategy:
- **Transient errors** (network timeout): Retry with exponential backoff
- **Persistent errors** (404, auth failure): Fall back to next provider
- **Plugin crash**: Restart plugin, retry once, then skip
- **All plugins fail**: Use cached data if available, mark item for retry

### Concurrency

- Max concurrent plugins: 3 (configurable)
- Max concurrent requests per plugin: 5 (configurable)
- Per-plugin rate limiting to respect API quotas
- Graceful degradation under load

### Plugin Distribution

- Primary: Git-based installation (`viewra plugin install github.com/user/plugin`)
- Fallback: Manual binary installation to `plugins/` directory
- Updates: `viewra plugin update` checks for new releases

### Image Handling

Plugins return image URLs with metadata. ViewRA downloads and caches centrally:

```protobuf
message ImageInfo {
  string url = 1;
  ImageType type = 2;  // poster, backdrop, logo, thumb
  int32 width = 3;
  int32 height = 4;
  string language = 5;
}
```

### Development Experience

1. **Standalone test harness**: `viewra-plugin-test ./my-plugin --scenario search`
2. **Dev mode**: Hot-reload plugins during development
3. **Template repository**: Example plugin with CI/CD setup

## Implementation Phases

### Phase 1: Foundation (3-4 days)
- Plugin manager and process lifecycle
- gRPC protocol definitions (protobuf)
- Basic MetadataProvider interface
- NFO plugin as first implementation

### Phase 2: Integration (2-3 days)
- Hook into library scanner
- Metadata merging logic
- Settings storage for plugins
- Image URL handling

### Phase 3: Built-in Providers (3-4 days)
- TMDb plugin for movies/TV
- MusicBrainz plugin for music
- Remove hardcoded provider code

### Phase 4: Polish (2-3 days)
- Plugin CLI commands (install, update, list)
- Developer test harness
- Documentation and template repo

## Consequences

### Positive

- All metadata sources use consistent interface
- Third-party providers without core changes
- Process isolation prevents plugin crashes from affecting host
- Community can contribute plugins
- Clean separation of concerns

### Negative

- Process overhead for each plugin (mitigated by warm pool)
- gRPC complexity vs direct function calls
- Initial migration effort to convert existing providers
- Plugin authors need to learn protobuf/gRPC

### Neutral

- Breaking change for any existing metadata customization
- Requires maintaining backward compatibility for plugin protocol

## Alternatives Considered

### In-process plugins (Go interfaces)

Simpler but no isolation - a buggy plugin crashes ViewRA. Also limits plugins to Go.

### HTTP-based plugins

More language-agnostic but higher overhead and complexity for local plugins.

### Lua/JavaScript embedded scripting

Simpler for basic plugins but limited capability and harder to debug.

## References

- [Hashicorp go-plugin](https://github.com/hashicorp/go-plugin)
- [gRPC Go](https://grpc.io/docs/languages/go/)
- [Protocol Buffers](https://protobuf.dev/)
- Terraform provider architecture (similar pattern)
