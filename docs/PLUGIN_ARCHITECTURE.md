# Plugin Architecture

## Overview

ViewRA's plugin system provides a safe, extensible way to add functionality without modifying the core application. Plugins run as separate processes communicating via gRPC or HTTP, ensuring isolation and stability.

## Design Principles

1. **Isolation**: Plugins run in separate processes, preventing crashes from affecting the main application
2. **Language Agnostic**: Support plugins written in any language via gRPC/HTTP
3. **Security First**: Permission-based access control and sandboxing
4. **Performance**: Async execution, caching, and circuit breakers
5. **Developer Friendly**: Clear APIs, SDK, and comprehensive documentation

---

## Plugin Types

### 1. Metadata Providers

Fetch metadata from external sources (TMDb, TheTVDB, MusicBrainz, etc.)

**Capabilities:**
- Search for media by title/year
- Fetch detailed metadata (plot, cast, crew, ratings)
- Download images (posters, backdrops, banners)
- Match local files to online databases

**Required Methods:**
```go
Search(query string, mediaType string, year int) ([]SearchResult, error)
GetDetails(id string) (*MediaMetadata, error)
GetImages(id string) ([]Image, error)
```

**Use Cases:**
- TMDb plugin for movies/TV shows
- TheTVDB plugin for TV shows
- MusicBrainz plugin for music
- AniDB plugin for anime
- Custom local NFO file parser

---

### 2. Authentication Providers

Handle user authentication through various methods.

**Capabilities:**
- Authenticate users with credentials
- Validate session tokens
- Manage user sessions
- Handle password resets

**Required Methods:**
```go
Authenticate(credentials map[string]string) (*User, error)
ValidateToken(token string) (*User, error)
RefreshToken(refreshToken string) (*TokenPair, error)
```

**Use Cases:**
- OAuth2 providers (Google, GitHub, Discord)
- LDAP/Active Directory integration
- SAML/SSO for enterprise
- Custom authentication backends

---

### 3. Notifiers

Send notifications about system events.

**Capabilities:**
- Send notifications to external services
- Format messages for different platforms
- Handle notification failures gracefully

**Required Methods:**
```go
SendNotification(event Event, message string) error
TestConnection() error
```

**Use Cases:**
- Discord webhooks (new media added)
- Slack notifications (library scans completed)
- Email alerts (transcoding failures)
- Pushover, Telegram, ntfy.sh
- Custom webhook integrations

---

### 4. Transcoders

Alternative or specialized transcoding engines.

**Capabilities:**
- Transcode media to different formats
- Generate thumbnails/previews
- Extract metadata from files
- Hardware-accelerated encoding

**Required Methods:**
```go
Transcode(input string, output string, profile TranscodeProfile) error
GetProgress(jobID string) (float64, error)
Cancel(jobID string) error
```

**Use Cases:**
- Hardware acceleration plugins (NVENC, QuickSync, VAAPI)
- Cloud transcoding services
- Custom encoding profiles
- HDR tone mapping

---

### 5. Scanners

Custom media scanning and file organization.

**Capabilities:**
- Parse filenames to extract metadata
- Organize media files
- Pre/post-processing during scans
- Custom metadata extraction

**Required Methods:**
```go
ParseFilename(filename string) (*ParsedMedia, error)
ShouldProcess(path string) bool
OnMediaAdded(media Media) error
```

**Use Cases:**
- Anime filename parsers (absolute numbering)
- Music folder structure parsers
- Custom thumbnail generators
- Subtitle downloaders

---

### 6. Storage Backends

Alternative storage locations for media.

**Capabilities:**
- Read media files from custom sources
- Stream media from cloud storage
- Cache frequently accessed files
- Handle authentication for storage

**Required Methods:**
```go
ReadFile(path string) (io.Reader, error)
ListFiles(directory string) ([]FileInfo, error)
GetURL(path string) (string, error)
```

**Use Cases:**
- S3/object storage backends
- Google Drive, Dropbox integration
- Network shares (SMB, NFS)
- CDN integration

---

### 7. Analytics

Custom analytics and reporting.

**Capabilities:**
- Collect usage statistics
- Generate custom reports
- Export data for external analysis
- Create dashboards

**Required Methods:**
```go
RecordEvent(event AnalyticsEvent) error
GetStats(startDate, endDate time.Time) (*Stats, error)
```

**Use Cases:**
- Watch time analytics
- User activity tracking
- Performance monitoring
- Custom dashboards

---

## Communication Protocol

### HTTP/JSON API

**Protocol**: Plugins communicate via HTTP REST API with JSON payloads

**Rationale**:
- Simple to implement in any language
- No protobuf compilation required
- Easy debugging (human-readable)
- Wide tooling support
- Sufficient performance for most use cases

**Plugin → ViewRA**:
```http
POST /internal/plugin-api/register
Content-Type: application/json

{
  "plugin_id": "tmdb-metadata",
  "hooks": ["metadata.search", "metadata.details"]
}
```

**ViewRA → Plugin**:
```http
POST http://localhost:9001/search
Content-Type: application/json

{
  "query": "The Matrix",
  "year": 1999,
  "media_type": "movie"
}
```

**Future Enhancement**:
- gRPC support for performance-critical plugins (post-1.0)
- Backwards compatible with HTTP/JSON

---

## Rate Limiting & Provider Throttling

### Metadata Provider Rate Limits

**Strategy**: Respect provider headers with background queue

**Implementation**:
```go
type RateLimiter struct {
    provider string
    limit    int
    window   time.Duration
    tokens   chan struct{}
}

func (rl *RateLimiter) Wait(ctx context.Context) error {
    select {
    case <-rl.tokens:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

func (rl *RateLimiter) HandleResponse(resp *http.Response) {
    // Check rate limit headers
    if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining == "0" {
        if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
            resetTime, _ := strconv.ParseInt(reset, 10, 64)
            sleepDuration := time.Unix(resetTime, 0).Sub(time.Now())
            time.Sleep(sleepDuration)
        }
    }
    
    // Check Retry-After header
    if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
        seconds, _ := strconv.Atoi(retryAfter)
        time.Sleep(time.Duration(seconds) * time.Second)
    }
}
```

**Background Queue**:
```go
type MetadataQueue struct {
    jobs     chan MetadataJob
    limiters map[string]*RateLimiter
}

func (mq *MetadataQueue) Enqueue(job MetadataJob) {
    mq.jobs <- job
}

func (mq *MetadataQueue) worker() {
    for job := range mq.jobs {
        limiter := mq.limiters[job.Provider]
        
        // Wait for rate limit
        limiter.Wait(context.Background())
        
        // Execute job
        result, resp := executeMetadataFetch(job)
        
        // Update rate limiter based on response
        limiter.HandleResponse(resp)
        
        // Save result
        saveMetadata(job.MediaID, result)
    }
}
```

**Provider Limits** (examples):
- TMDb: 40 requests per 10 seconds
- TheTVDB: 100 requests per 10 seconds
- MusicBrainz: 1 request per second

---

## Plugin Lifecycle

### 1. Installation

```
Download → Verify Signature → Extract → Register in DB → Load Manifest
```

**Installation Methods:**
- Manual: Drop plugin folder into `/plugins` directory
- CLI: `viewra plugin install <name>`
- API: `POST /api/plugins/install`
- Marketplace: Browse and install from UI (Phase 2)

**Manifest Validation:**
- Check `plugin.yaml` exists and is valid
- Verify required fields (name, version, type, runtime)
- Validate permissions requested
- Check compatibility with ViewRA version

---

### 2. Configuration

After installation, plugins need configuration:

**Configuration Sources:**
1. Default values from manifest schema
2. Environment variables
3. Config file overrides
4. UI settings (saved to database)

**Configuration Schema Example:**
```yaml
config_schema:
  api_key:
    type: string
    required: true
    sensitive: true
    description: "API key for service"
  
  cache_ttl:
    type: integer
    default: 3600
    description: "Cache TTL in seconds"
  
  enabled_features:
    type: array
    items: string
    default: ["movies", "tv_shows"]
```

---

### 3. Activation

**Startup Sequence:**
1. Load plugin metadata from database
2. Start plugin process (if gRPC/HTTP)
3. Call `Initialize(config)` with configuration
4. Wait for `Start()` to complete
5. Begin health checks
6. Mark as active

**Health Checks:**
- Periodic ping every 30 seconds
- Timeout after 5 seconds
- After 3 consecutive failures, mark as unhealthy
- Log health check events to `plugin_events`

---

### 4. Runtime

**Plugin Communication:**
- **gRPC**: Binary protocol, efficient, type-safe
- **HTTP**: REST JSON API, simpler, more compatible
- **WASM**: (Future) Sandboxed execution in-process

**Request Flow:**
```
Core App → Plugin Manager → Plugin Instance → Response → Cache → Return
```

**Caching:**
- Cache responses in `plugin_data` table
- Set `expires_at` based on TTL
- Automatic cache invalidation
- Per-plugin cache strategy

---

### 5. Updates

**Update Process:**
1. Check for new version (manual or automatic)
2. Download new version
3. Verify signature
4. Stop current version
5. Replace files
6. Restart with new version
7. Run migration hooks if needed
8. Verify health

**Version Compatibility:**
- Semantic versioning (MAJOR.MINOR.PATCH)
- Breaking changes require major version bump
- ViewRA API version compatibility matrix

---

### 6. Deactivation/Removal

**Disable (Soft Delete):**
- Stop plugin process
- Mark `enabled = 0` in database
- Keep configuration and data
- Can be re-enabled later

**Uninstall (Hard Delete):**
- Stop plugin process
- Remove plugin files
- Delete from `plugins` table (cascades to plugin_data, plugin_events, plugin_hooks)
- Optional: Keep plugin data for backup

---

## Communication Protocols

### gRPC Protocol

**Advantages:**
- Type-safe with Protocol Buffers
- Efficient binary serialization
- Built-in streaming support
- HTTP/2 multiplexing

**Example Service Definition:**

```protobuf
syntax = "proto3";

package viewra.plugins;

service MetadataProvider {
  rpc Initialize(InitRequest) returns (InitResponse);
  rpc Start(StartRequest) returns (StartResponse);
  rpc Stop(StopRequest) returns (StopResponse);
  rpc Health(HealthRequest) returns (HealthResponse);
  
  rpc Search(SearchRequest) returns (SearchResponse);
  rpc GetDetails(DetailsRequest) returns (MediaMetadata);
  rpc GetImages(ImagesRequest) returns (ImagesResponse);
}

message InitRequest {
  map<string, string> config = 1;
}

message SearchRequest {
  string query = 1;
  string media_type = 2;
  int32 year = 3;
  string language = 4;
}

message SearchResponse {
  repeated SearchResult results = 1;
}

message SearchResult {
  string id = 1;
  string title = 2;
  string original_title = 3;
  int32 year = 4;
  string overview = 5;
  string poster_url = 6;
  float match_score = 7;
}
```

---

### HTTP Protocol

**Advantages:**
- Simple to implement
- Language agnostic
- Easy to debug (curl, Postman)
- Widely supported

**API Endpoints:**

```
POST   /initialize        # Initialize plugin with config
POST   /start             # Start plugin services
POST   /stop              # Stop plugin gracefully
GET    /health            # Health check

POST   /search            # Search for media
GET    /details/:id       # Get detailed metadata
GET    /images/:id        # Get image URLs
```

**Authentication:**
- Each request includes `X-ViewRA-Token` header
- Token generated by core app on plugin registration
- Validated on every request

**Example Request:**

```http
POST /search HTTP/1.1
Host: localhost:8080
X-ViewRA-Token: abc123...
Content-Type: application/json

{
  "query": "The Matrix",
  "media_type": "movie",
  "year": 1999,
  "language": "en"
}
```

**Example Response:**

```json
{
  "results": [
    {
      "id": "603",
      "title": "The Matrix",
      "original_title": "The Matrix",
      "year": 1999,
      "overview": "Set in the 22nd century...",
      "poster_url": "https://image.tmdb.org/t/p/w500/...",
      "match_score": 0.98
    }
  ]
}
```

---

## Event System (Hooks)

### Available Events

**Media Events:**
- `media.added` - New media file added to library
- `media.updated` - Media metadata updated
- `media.deleted` - Media file removed
- `media.metadata_enriched` - External metadata fetched

**Library Events:**
- `library.scan.started` - Library scan initiated
- `library.scan.completed` - Library scan finished
- `library.scan.failed` - Library scan error

**Playback Events:**
- `playback.started` - User started playing media
- `playback.paused` - Playback paused
- `playback.stopped` - Playback stopped
- `playback.progress` - Progress update (every 10 seconds)

**User Events:**
- `user.login` - User logged in
- `user.logout` - User logged out
- `user.created` - New user account created

**Transcode Events:**
- `transcode.started` - Transcode job started
- `transcode.progress` - Progress update
- `transcode.completed` - Transcode finished successfully
- `transcode.failed` - Transcode error

**System Events:**
- `server.started` - ViewRA server started
- `server.stopped` - ViewRA server stopping

---

### Hook Registration

**Database Registration:**
```sql
INSERT INTO plugin_hooks (plugin_id, hook_event, priority, enabled, filter_conditions)
VALUES (1, 'media.added', 100, 1, '{"media_type": "movie"}');
```

**Programmatic Registration (via SDK):**
```go
sdk.RegisterHook("media.added", func(ctx context.Context, event Event) error {
    media := event.Data.(Media)
    
    // Only process movies
    if media.Type != "movie" {
        return nil
    }
    
    // Send notification
    return sendDiscordNotification(
        fmt.Sprintf("New movie added: %s", media.Title),
    )
}, HookOptions{
    Priority: 100,
    FilterConditions: map[string]interface{}{
        "media_type": "movie",
    },
})
```

---

### Event Delivery

**Execution Order:**
1. Query `plugin_hooks` for event type
2. Filter by `enabled = 1` and `p.enabled = 1`
3. Sort by `priority DESC` (higher priority first)
4. Apply `filter_conditions` to event data
5. Call each plugin's hook handler sequentially
6. Log results to `plugin_events`

**Error Handling:**
- If hook fails, log error but continue to next hook
- After 3 consecutive failures, auto-disable hook
- Admin notification on critical failures

**Timeout:**
- Each hook has 30-second timeout
- Configurable per hook in manifest
- Prevents hanging plugins from blocking system

---

## Security Model

### Permission System

**Permission Categories:**

**Database:**
- `database.read` - Read media, libraries, metadata
- `database.write` - Modify media records (metadata only)
- `database.admin` - Full database access (dangerous)

**Filesystem:**
- `filesystem.read` - Read media files
- `filesystem.write` - Write files (thumbnails, transcodes)
- `filesystem.scan` - Scan directories

**Network:**
- `network.http` - Make HTTP/HTTPS requests
- `network.socket` - Open TCP/UDP sockets
- `network.unrestricted` - No network limits (dangerous)

**System:**
- `transcode.execute` - Execute FFmpeg/transcoding
- `user.read` - Read user data (watch history, ratings)
- `user.write` - Modify user data
- `system.admin` - Administrative operations

**Permission Declaration (plugin.yaml):**
```yaml
permissions:
  - network.http
  - database.read
  - filesystem.read
```

**Permission Enforcement:**
- Validated on plugin installation
- User must approve dangerous permissions
- Enforced via middleware/interceptor
- Violations logged and plugin disabled

---

### Sandboxing

**Process Isolation:**
- Each plugin runs in separate process
- Cannot access main app memory
- Limited by OS process permissions

**Resource Limits:**
- CPU: Max 50% of one core (configurable)
- Memory: Max 512MB (configurable)
- Disk I/O: Rate limited
- Network: Rate limited per API

**Future Enhancements:**
- Docker containers for plugins
- WASM sandbox for user scripts
- SELinux/AppArmor profiles

---

### Code Signing

**Plugin Verification:**
1. Author signs plugin with private key
2. Signature included in plugin package
3. ViewRA verifies with author's public key
4. Only signed plugins from trusted authors allowed (optional setting)

**Trust Levels:**
- **Official**: Signed by ViewRA team
- **Verified**: Signed by known community developers
- **Unverified**: Unsigned or unknown author (requires explicit approval)

---

## Plugin SDK

### Go SDK

**Installation:**
```bash
go get github.com/viewra/plugin-sdk-go
```

**Example Plugin:**

```go
package main

import (
    "context"
    "github.com/viewra/plugin-sdk-go/sdk"
    "github.com/viewra/plugin-sdk-go/metadata"
)

type MyMetadataPlugin struct {
    sdk    *sdk.SDK
    apiKey string
}

func (p *MyMetadataPlugin) Initialize(config map[string]interface{}) error {
    p.apiKey = config["api_key"].(string)
    return nil
}

func (p *MyMetadataPlugin) Search(ctx context.Context, req *metadata.SearchRequest) (*metadata.SearchResponse, error) {
    // Search external API
    results, err := searchExternalAPI(req.Query, req.Year)
    if err != nil {
        return nil, err
    }
    
    // Cache results
    p.sdk.Cache.Set(req.Query, results, 3600)
    
    return &metadata.SearchResponse{Results: results}, nil
}

func main() {
    plugin := &MyMetadataPlugin{}
    
    sdk := sdk.New(sdk.Config{
        Name:    "my-metadata-plugin",
        Version: "1.0.0",
        Type:    sdk.TypeMetadataProvider,
    })
    
    plugin.sdk = sdk
    
    metadata.RegisterMetadataProvider(sdk.Server, plugin)
    
    sdk.Serve()
}
```

---

### SDK Features

**Logging:**
```go
sdk.Logger.Info("Searching for media", "query", req.Query)
sdk.Logger.Error("Failed to fetch metadata", "error", err)
sdk.Logger.Debug("Cache hit", "key", cacheKey)
```

**Caching:**
```go
// Set cache with TTL
sdk.Cache.Set(key, value, ttl)

// Get from cache
value, found := sdk.Cache.Get(key)

// Delete from cache
sdk.Cache.Delete(key)
```

**Configuration:**
```go
// Get config value
apiKey := sdk.Config.GetString("api_key")
timeout := sdk.Config.GetInt("timeout", 30)
enabled := sdk.Config.GetBool("enabled", true)
```

**HTTP Client:**
```go
resp, err := sdk.HTTP.Get(ctx, "https://api.example.com/search")
resp, err := sdk.HTTP.PostJSON(ctx, url, body)
```

**Database (Read-Only):**
```go
media, err := sdk.DB.GetMedia(mediaID)
library, err := sdk.DB.GetLibrary(libraryID)
```

**Events:**
```go
// Emit event
sdk.Events.Emit("metadata.fetched", map[string]interface{}{
    "media_id": 123,
    "source": "tmdb",
})

// Listen to events
sdk.Events.On("media.added", func(event sdk.Event) {
    // Handle event
})
```

---

## Development Workflow

### 1. Plugin Development

**Steps:**
1. Clone plugin template repository
2. Implement required interfaces
3. Add configuration schema to `plugin.yaml`
4. Test locally with ViewRA dev instance
5. Build and package plugin
6. Sign plugin (optional)
7. Publish to repository/marketplace

**Testing:**
```bash
# Run plugin in dev mode
go run main.go --dev

# Test with ViewRA
viewra plugin test ./my-plugin

# Run integration tests
go test ./...
```

---

### 2. Plugin Packaging

**Directory Structure:**
```
my-plugin/
├── plugin.yaml           # Plugin manifest
├── bin/
│   ├── plugin-linux-amd64
│   ├── plugin-darwin-amd64
│   └── plugin-windows-amd64.exe
├── README.md
├── LICENSE
└── icon.png
```

**Build:**
```bash
# Build for multiple platforms
GOOS=linux GOARCH=amd64 go build -o bin/plugin-linux-amd64
GOOS=darwin GOARCH=amd64 go build -o bin/plugin-darwin-amd64
GOOS=windows GOARCH=amd64 go build -o bin/plugin-windows-amd64.exe

# Package
tar czf my-plugin-1.0.0.tar.gz my-plugin/

# Sign
viewra-sign my-plugin-1.0.0.tar.gz --key private-key.pem
```

---

### 3. Distribution

**Options:**
1. **GitHub Releases**: Host on GitHub with versioned releases
2. **Plugin Marketplace**: Submit to official marketplace (Phase 2)
3. **Direct Download**: Host on own server
4. **Private Registry**: For internal/enterprise plugins

---

## Performance Optimization

### Caching Strategy

**Plugin Response Cache:**
- Store in `plugin_data` table with `expires_at`
- TTL based on data type:
  - Search results: 1 hour
  - Media metadata: 24 hours
  - Images: 7 days
  - Static data: 30 days

**Example:**
```go
cacheKey := fmt.Sprintf("search:%s:%d", query, year)
if cached, found := p.sdk.Cache.Get(cacheKey); found {
    return cached.(*SearchResponse), nil
}

results := fetchFromAPI(query, year)
p.sdk.Cache.Set(cacheKey, results, 3600)
return results, nil
```

---

### Circuit Breakers

**Prevent Cascading Failures:**
- After 5 consecutive errors, trip circuit breaker
- Stop sending requests for 60 seconds
- After cooldown, try one request
- If successful, close circuit
- If fails, open circuit again

**Implementation:**
```go
breaker := sdk.NewCircuitBreaker(sdk.CircuitBreakerConfig{
    FailureThreshold: 5,
    ResetTimeout:     60 * time.Second,
})

result, err := breaker.Execute(func() (interface{}, error) {
    return plugin.Search(ctx, req)
})
```

---

### Rate Limiting

**Per-Plugin Limits:**
- Requests per second: 10 (configurable)
- Concurrent requests: 5 (configurable)
- Burst capacity: 20 (configurable)

**External API Rate Limits:**
- Track API quota usage
- Implement backoff strategies
- Queue requests when near limit

---

## Monitoring & Debugging

### Health Checks

**Health Status:**
- `healthy` - All systems operational
- `unhealthy` - Plugin not responding
- `unknown` - Never received health check

**Health Check Endpoint:**
```http
GET /health
Response: 200 OK
{
  "status": "healthy",
  "uptime": 3600,
  "version": "1.0.0",
  "checks": {
    "database": "ok",
    "external_api": "ok"
  }
}
```

---

### Logging

**Log Levels:**
- DEBUG: Detailed information for debugging
- INFO: General informational messages
- WARN: Warning messages
- ERROR: Error messages
- CRITICAL: Critical failures requiring immediate attention

**Log Format (JSON):**
```json
{
  "timestamp": "2025-11-11T10:30:00Z",
  "level": "INFO",
  "plugin": "tmdb-metadata",
  "message": "Fetched metadata successfully",
  "media_id": 123,
  "request_id": "abc-123"
}
```

**Centralized Logging:**
- All plugin logs forwarded to main app
- Stored in `plugin_events` table
- Searchable via admin UI
- Exportable for analysis

---

### Metrics

**Key Metrics:**
- Request count per plugin
- Average response time
- Error rate
- Cache hit rate
- Resource usage (CPU, memory)

**Prometheus Integration (Future):**
```
viewra_plugin_requests_total{plugin="tmdb",status="success"} 1234
viewra_plugin_duration_seconds{plugin="tmdb",quantile="0.95"} 0.5
viewra_plugin_errors_total{plugin="tmdb"} 5
```

---

## Best Practices

### For Plugin Developers

1. **Handle Errors Gracefully**: Always return meaningful error messages
2. **Implement Timeouts**: Don't hang indefinitely on external calls
3. **Cache Aggressively**: Reduce load on external APIs
4. **Validate Input**: Never trust input from core app
5. **Log Appropriately**: Use correct log levels
6. **Version Carefully**: Follow semantic versioning
7. **Document Well**: Comprehensive README and API docs
8. **Test Thoroughly**: Unit tests, integration tests, load tests

---

### For ViewRA Developers

1. **Never Block on Plugins**: Always use timeouts
2. **Fail Fast**: If plugin is unhealthy, skip it
3. **Isolate Failures**: One plugin failure shouldn't affect others
4. **Monitor Everything**: Track plugin health, performance, errors
5. **Update Plugins Safely**: Test before deploying updates
6. **Respect Permissions**: Enforce permission model strictly
7. **Provide Good APIs**: Make SDK easy to use
8. **Document Changes**: API changes require migration guides

---

## Future Enhancements

### Phase 2
- Plugin marketplace with ratings and reviews
- Automatic plugin updates
- WASM support for sandboxed scripts
- Python SDK
- JavaScript/TypeScript SDK

### Phase 3
- Visual plugin builder (no-code)
- Plugin templates for common use cases
- Community plugin repository
- Plugin analytics dashboard
- A/B testing for plugins

### Phase 4
- Plugin dependency management
- Plugin composition (chain plugins)
- Distributed plugin execution
- Plugin monetization platform

---

## Appendix

### Example Plugins

1. **TMDb Metadata Provider**: Fetch movie/TV metadata from TMDb
2. **Discord Notifier**: Send notifications to Discord channels
3. **Hardware Transcoder**: NVENC hardware acceleration
4. **Anime Scanner**: Parse anime filenames with absolute numbering
5. **S3 Storage Backend**: Stream media from S3-compatible storage

See `examples/` directory for full implementation of each.

---

### Plugin Manifest Reference

**Complete `plugin.yaml` Example:**

```yaml
# Plugin identity
name: "tmdb-metadata"
version: "1.2.3"
author: "ViewRA Team"
description: "Fetch metadata from The Movie Database"
homepage_url: "https://github.com/viewra/plugin-tmdb"
license: "MIT"

# Plugin configuration
plugin_type: "metadata_provider"
runtime: "grpc"
endpoint: "localhost:50051"

# Compatibility
viewra_min_version: "1.0.0"
viewra_max_version: "2.0.0"

# Capabilities
capabilities:
  - movies
  - tv_shows
  - images

# Configuration schema
config_schema:
  api_key:
    type: string
    required: true
    sensitive: true
    description: "TMDb API v3 Key"
  
  language:
    type: string
    default: "en-US"
    description: "Preferred language for metadata"
  
  include_adult:
    type: boolean
    default: false
    description: "Include adult content in results"
  
  cache_ttl:
    type: integer
    default: 86400
    description: "Cache TTL in seconds"

# Permissions
permissions:
  - network.http
  - database.read
  - filesystem.read

# Resource limits
resources:
  max_memory_mb: 256
  max_cpu_percent: 50
  request_timeout_seconds: 30

# Health check
health_check:
  interval_seconds: 30
  timeout_seconds: 5
  failure_threshold: 3
```

---

### API Version Compatibility

| ViewRA Version | Plugin SDK Version | Breaking Changes |
|----------------|-------------------|------------------|
| 1.0.x          | 1.0.x             | Initial release  |
| 1.1.x          | 1.1.x             | Added WASM support |
| 2.0.x          | 2.0.x             | New event system |

When ViewRA API changes:
- Major version bump = Breaking changes
- Minor version bump = New features (backward compatible)
- Patch version bump = Bug fixes

---

## Conclusion

The plugin system provides a robust, secure, and extensible foundation for ViewRA. By following these architectural guidelines, we can build a thriving ecosystem of community-contributed plugins while maintaining system stability and security.

For questions or contributions, see:
- GitHub: https://github.com/viewra/viewra
- Docs: https://docs.viewra.io/plugins
- Discord: https://discord.gg/viewra
