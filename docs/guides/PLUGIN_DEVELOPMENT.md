# Plugin Development Guide

This guide covers how to develop enrichment plugins for ViewRA. Plugins extend ViewRA's metadata capabilities by connecting to external services (TMDb, MusicBrainz, Fanart.tv) or processing local files (NFO, embedded images).

## Architecture Overview

ViewRA uses HashiCorp's [go-plugin](https://github.com/hashicorp/go-plugin) library with gRPC for plugin communication:

```text
┌─────────────────────────────────────────────────┐
│                ViewRA Host Process              │
│                                                 │
│  ┌───────────────────────────────────────────┐  │
│  │          Pipeline Manager                 │  │
│  │  - Registers enrichers                    │  │
│  │  - Manages worker pools per stage         │  │
│  │  - Coordinates job processing             │  │
│  └─────────────────┬─────────────────────────┘  │
│                    │                            │
│  ┌─────────────────┴─────────────────────────┐  │
│  │          Host Services (gRPC)             │  │
│  │  - HostData: Query media/libraries        │  │
│  │  - HostStorage: Plugin-isolated storage   │  │
│  └─────────────────┬─────────────────────────┘  │
│                    │                            │
└────────────────────┼────────────────────────────┘
                     │ gRPC over stdio
         ┌───────────┼───────────┐
         ▼           ▼           ▼
    ┌─────────┐ ┌─────────┐ ┌─────────┐
    │  TMDb   │ │  NFO    │ │ Fanart  │
    │ Plugin  │ │ Plugin  │ │ Plugin  │
    └─────────┘ └─────────┘ └─────────┘
```

### Plugin Types

| Category | Description | Examples |
|----------|-------------|----------|
| **Enricher** | Metadata and artwork | TMDb, MusicBrainz, NFO, Local Images |
| **NotificationSink** | Send notifications | Webhooks, Discord, Telegram |
| **SubtitleProvider** | Download subtitles | OpenSubtitles (future) |
| **AnalyticsProvider** | Sync watch history | Trakt, Last.fm (future) |

## Plugin Structure

Every plugin has this directory structure:

```
plugins/
└── my-plugin/
    ├── plugin.yml      # Manifest (required)
    ├── config.yml      # Runtime config (optional)
    └── my-plugin       # Binary (for external plugins)
```

### Manifest File (`plugin.yml`)

The manifest declares plugin identity and capabilities:

```yaml
# Plugin identity
id: my-tmdb                  # Unique identifier
name: TMDb Metadata          # Display name
version: 1.0.0
description: Fetches metadata from The Movie Database
author: Your Name
license: MIT
homepage: https://github.com/you/my-tmdb

# Compatibility
min_host_version: 0.1.0

# Plugin type(s)
categories:
  - enricher

# Enricher capabilities (required for enricher category)
capabilities:
  media_types:
    - movie
    - tv
  provides:
    - metadata
    - artwork
    - external_ids
  is_local: false           # false = remote API calls
  rate_limit: 40            # requests per minute

# Required permissions
permissions:
  - network                 # Make HTTP requests
  - storage:kv              # Use key-value storage
```

### Permissions

Plugins must declare required permissions:

| Permission | Description |
|------------|-------------|
| `network` | Make outbound HTTP requests |
| `storage:kv` | Use key-value storage |
| `storage:database` | Use SQLite database |
| `storage:user_metadata` | Store per-user data |
| `host:data` | Query media/library information |

## The Enricher Interface

Enricher plugins implement two gRPC services: `PluginCore` (lifecycle) and `Enricher` (metadata).

### Core Lifecycle

```protobuf
service PluginCore {
  rpc Initialize(InitRequest) returns (InitResponse);
  rpc Shutdown(Empty) returns (Empty);
  rpc HealthCheck(Empty) returns (HealthStatus);
  rpc GetSettingsSchema(Empty) returns (SettingsSchema);
  rpc Configure(Settings) returns (ConfigureResponse);
}
```

### Enricher Service

```protobuf
service Enricher {
  rpc GetCapabilities(Empty) returns (EnricherCapabilities);
  rpc Enrich(EnrichRequest) returns (EnrichResponse);
}
```

### EnrichRequest

The host sends this to your plugin:

```protobuf
message EnrichRequest {
  int64 media_id = 1;              // ViewRA database ID
  string media_type = 2;           // "movie", "tv", "music"
  string file_path = 3;            // Full path to media file
  string title = 4;                // Title from filename/previous enrichers
  int32 year = 5;                  // Year (0 if unknown)
  map<string, string> existing_ids = 6;  // IDs from previous stages
  TVMetadata tv = 7;               // TV-specific fields
  MusicMetadata music = 8;         // Music-specific fields
}
```

### EnrichResponse

Your plugin returns:

```protobuf
message EnrichResponse {
  bool matched = 1;                // Found a match?
  EnrichedMetadata metadata = 2;   // Enriched metadata
  map<string, string> discovered_ids = 3;  // IDs you found
  repeated EnrichedImage images = 4;       // Artwork URLs
  bool skipped = 5;                // Intentionally skipped?
  string skip_reason = 6;          // Why skipped
  float confidence = 7;            // 0.0-1.0 match confidence
}
```

## Example: Built-in NFO Enricher

Here's how the built-in NFO enricher works. Built-in enrichers run in-process and implement the same interface:

```go
package builtin

import (
    "context"

    pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
    appenrich "github.com/mantonx/viewra/internal/application/enrichment"
    "github.com/mantonx/viewra/internal/domain/enrichment"
)

type NFOEnricher struct{}

func NewNFOEnricher() *NFOEnricher {
    return &NFOEnricher{}
}

// Stage returns the unique identifier for this enricher
func (e *NFOEnricher) Stage() string {
    return "nfo"
}

// Capabilities declares what this enricher provides
func (e *NFOEnricher) Capabilities() appenrich.EnricherCapabilities {
    return appenrich.NewCapabilitiesBuilder().
        WithMediaTypes(enrichment.MediaTypeMovie, enrichment.MediaTypeTV).
        WithProvides("metadata", "external_ids").
        AsLocal().  // High concurrency, no rate limit
        Build()
}

// Enrich processes a single media item
func (e *NFOEnricher) Enrich(ctx context.Context, req *pluginv1.EnrichRequest) (*pluginv1.EnrichResponse, error) {
    // Skip if no NFO file exists
    nfoPath := findNFOFile(req.FilePath)
    if nfoPath == "" {
        return appenrich.Skip("no NFO file found"), nil
    }

    // Parse NFO content
    metadata, err := parseNFO(nfoPath)
    if err != nil {
        return nil, fmt.Errorf("parse NFO: %w", err)
    }

    // Build successful response
    resp := appenrich.Match()
    resp.Confidence = 1.0  // NFO is authoritative

    resp.Metadata = &pluginv1.EnrichedMetadata{
        Title: appenrich.Ptr(metadata.Title),
        Year:  appenrich.Ptr(int32(metadata.Year)),
        Plot:  appenrich.Ptr(metadata.Plot),
    }

    // Pass discovered IDs to subsequent enrichers
    if metadata.IMDbID != "" {
        appenrich.AddDiscoveredID(resp, "imdb", metadata.IMDbID)
    }

    return resp, nil
}
```

## ID Propagation

IDs discovered by one enricher are passed to subsequent stages:

```text
1. NFO enricher finds IMDB ID: tt0133093
   └── Returns: discovered_ids: {"imdb": "tt0133093"}

2. Host saves to media_external_ids table

3. TMDb enricher receives:
   └── existing_ids: {"imdb": "tt0133093"}
   └── Looks up directly instead of searching

4. TMDb returns:
   └── discovered_ids: {"tmdb": "603"}
```

This enables precise lookups and avoids ambiguous title searches.

## Host Services

External plugins access ViewRA data through gRPC host services.

### HostData Service

Query media and library information:

```protobuf
service HostData {
  rpc GetMedia(MediaQuery) returns (Media);
  rpc GetMediaByExternalId(ExternalIdQuery) returns (Media);
  rpc SearchMedia(SearchQuery) returns (MediaList);
  rpc GetLibrary(LibraryId) returns (Library);
  rpc GetFilePath(MediaId) returns (FilePath);
}
```

Example usage in a plugin:

```go
func (p *MyPlugin) Enrich(ctx context.Context, req *EnrichRequest) (*EnrichResponse, error) {
    // Look up by external ID if available
    if imdbID, ok := req.ExistingIds["imdb"]; ok {
        media, err := p.hostData.GetMediaByExternalId(ctx, &ExternalIdQuery{
            Provider:   "imdb",
            ExternalId: imdbID,
        })
        if err == nil && media != nil {
            // Use media.Title, media.Year, etc.
        }
    }
    // ...
}
```

### HostStorage Service

Plugins get isolated storage:

```protobuf
service HostStorage {
  // Key-value storage
  rpc KVGet(KVKey) returns (KVValue);
  rpc KVSet(KVEntry) returns (Empty);
  rpc KVDelete(KVKey) returns (Empty);

  // SQLite database (for complex data)
  rpc GetDatabasePath(Empty) returns (DatabasePath);
}
```

## Capabilities Declaration

Capabilities control how the pipeline manager schedules your enricher:

```yaml
capabilities:
  media_types:
    - movie
    - tv
  provides:
    - metadata      # Title, plot, ratings, etc.
    - artwork       # Posters, fanart, etc.
    - external_ids  # IMDB, TMDB, TVDB IDs
  is_local: false   # false = rate-limited scheduling
  rate_limit: 40    # Requests per minute
  requires:         # Optional: required IDs
    - imdb
```

### Local vs Remote Enrichers

| Setting | Local (`is_local: true`) | Remote (`is_local: false`) |
|---------|--------------------------|---------------------------|
| Concurrency | High (CPU × 2) | Low (rate-limited) |
| Rate Limit | None | Respected |
| Examples | NFO, Local Images | TMDb, Fanart.tv |

## Error Handling

Return errors vs skip responses appropriately:

```go
func (e *Enricher) Enrich(ctx context.Context, req *EnrichRequest) (*EnrichResponse, error) {
    // SKIP: Nothing to do (not an error)
    if !fileExists(req.FilePath + ".nfo") {
        return appenrich.Skip("no NFO file found"), nil
    }

    // ERROR: Actual failure (will be retried)
    data, err := os.ReadFile(nfoPath)
    if err != nil {
        return nil, fmt.Errorf("read NFO file: %w", err)
    }

    // NO MATCH: Searched but found nothing
    result := searchAPI(req.Title, req.Year)
    if result == nil {
        return appenrich.NoMatch(), nil
    }

    // MATCH: Success
    return appenrich.Match(), nil
}
```

### Error Categories

The pipeline manager categorizes errors for appropriate handling:

| Category | Behavior | Examples |
|----------|----------|----------|
| `network` | Retry with backoff | Connection timeout |
| `rate_limit` | Delay and retry | 429 Too Many Requests |
| `not_found` | Skip stage | No match in external DB |
| `parsing` | Log and skip | Invalid response format |
| `plugin` | Restart plugin | Plugin crashed |

## Health Monitoring

Plugins report health status via `HealthCheck`:

```protobuf
message HealthStatus {
  enum Status {
    UNKNOWN = 0;
    HEALTHY = 1;
    DEGRADED = 2;
    UNHEALTHY = 3;
  }

  Status status = 1;
  string message = 2;
  int64 requests_total = 3;
  int64 errors_total = 4;
  double avg_latency_ms = 5;
}
```

The UI displays plugin health:

```
Plugins:
  ✓ NFO Parser        healthy     avg 12ms    0 errors
  ✓ TMDb              degraded    avg 890ms   3 errors/min (rate limited)
```

## Development Workflow

### 1. Define Your Manifest

```yaml
# plugins/my-enricher/plugin.yml
id: my-enricher
name: My Enricher
version: 0.1.0
categories:
  - enricher
capabilities:
  media_types: [movie]
  provides: [metadata]
  is_local: false
  rate_limit: 30
permissions:
  - network
```

### 2. Implement the Interface

Use the SDK to simplify implementation:

```go
package main

import (
    "context"

    pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
    "github.com/mantonx/viewra/sdk"
)

type MyPlugin struct {
    sdk.Base  // Required: provides logging, metrics, etc.
    client *myapi.Client
}

func (p *MyPlugin) GetCapabilities() sdk.EnricherCapabilities {
    return sdk.EnricherCapabilities{
        MediaTypes: []string{"movie"},
        Provides:   []string{"metadata"},
        IsLocal:    false,
        RateLimit:  30,
    }
}

func (p *MyPlugin) Initialize(ctx context.Context, dataDir string, config []byte) error {
    var cfg Config
    if err := yaml.Unmarshal(config, &cfg); err != nil {
        return err
    }
    p.client = myapi.NewClient(cfg.APIKey)
    return nil
}

func (p *MyPlugin) Shutdown(ctx context.Context) error {
    return nil
}

func (p *MyPlugin) Enrich(ctx context.Context, req *pluginv1.EnrichRequest) (*pluginv1.EnrichResponse, error) {
    p.Log().Info("enriching", "title", req.Title)
    // Implementation here
}

func main() {
    sdk.Serve(&MyPlugin{})
}
```

### 3. Build and Test

```bash
# Build the plugin
go build -o plugins/my-enricher/my-enricher ./cmd/my-enricher

# Test with debug logging
VIEWRA_PLUGIN_DEBUG=1 ./bin/viewra
```

### 4. Enable in Pipeline

Once installed, your plugin appears in Settings → Enrichment Pipeline where users can:
- Enable/disable it
- Reorder it relative to other enrichers
- Configure plugin-specific settings

## Debugging

### Enable Debug Logging

Set `VIEWRA_PLUGIN_DEBUG=1` for verbose gRPC logging:

```
[12:35:00] DEBUG grpc  method=Enrich plugin=my-enricher
                       request={"media_id": 42, "title": "The Matrix"}
[12:35:00] DEBUG grpc  response={"matched": true}
```

### Correlation IDs

Request IDs propagate from app through plugins:

```
[12:35:00] INFO  req=abc123 app/worker    Processing media 42
[12:35:00] INFO  req=abc123 plugin/tmdb   Searching for "The Matrix"
[12:35:00] INFO  req=abc123 app/worker    Enrichment complete
```

Use `p.Log()` to automatically include the request ID.

## Best Practices

1. **Skip gracefully**: Return `Skip()` when there's nothing to do, not an error
2. **Propagate IDs**: Always return discovered external IDs for downstream enrichers
3. **Set confidence**: Use 0.0-1.0 to indicate match quality
4. **Respect rate limits**: Declare accurate `rate_limit` in capabilities
5. **Handle timeouts**: The host enforces per-request timeouts
6. **Report health**: Implement meaningful health checks

## See Also

- [ADR 027: Plugin System Architecture](../decisions/027-plugin-system-architecture.md)
- [Enrichment Pipeline Guide](./ENRICHMENT_PIPELINE.md)
- Proto definitions: `api/proto/plugin/`
