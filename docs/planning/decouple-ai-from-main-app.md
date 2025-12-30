# Decouple AI from Main App

## Progress

| Phase | Status | Notes |
|-------|--------|-------|
| Phase 1: Host-Managed Storage | ✅ Complete | SQL + Vector storage with plugin prefixing |
| Phase 2: Capability Broker | ✅ Complete | Generic broker using `ExposeService` RPC |
| Phase 3: AIProvider Interface | ✅ Complete | Already existed as `PluginProvider` proto |
| Phase 4: Provider Plugins | ✅ Complete | Providers already implement `ProviderPlugin` |
| Phase 5: semantic-search | ✅ Complete | Uses capability broker for embedding/chat |
| Phase 6: Remove AI from Main App | ✅ Complete | Deleted domain/ai, host_llm, host_embeddings |
| Phase 7: SDK Cleanup | ✅ Complete | Renamed plugin_ai.proto to vector_search.proto, deleted host_ai.proto |
| Phase 8: Fallback Search | ⏳ Pending | LIKE/FTS search when no search plugin configured |

### Implementation Notes

**Generic Capability Broker** (Phase 2)

The implementation uses a pull model where:
1. Consumer plugin requests capability via `HostPlugins.GetCapabilityProvider()`
2. Host calls provider's `PluginCore.ExposeService()` RPC
3. Provider starts its service on a new broker ID via `broker.AcceptAndServe()`
4. Host returns broker ID to consumer
5. Consumer dials broker ID to get direct gRPC connection to provider

This is more generic than the original plan - any plugin can expose any gRPC service
through capabilities, not just AI providers.

**Key Files:**
- `api/proto/plugin/plugin_core.proto` - Added `ExposeService` RPC
- `api/proto/plugin/host_services.proto` - `HostPlugins` service
- `internal/infrastructure/plugins/host_plugins.go` - Capability registry & broker
- `pkg/plugin/sdk/plugins_client.go` - Consumer SDK
- `pkg/plugin/sdk/provider.go` - Provider `ExposeService` implementation

---

## Overview

Transform AI from a core feature embedded in the main application to a fully opt-in plugin ecosystem. The main app will have zero AI-specific code, with all AI functionality provided by plugins that communicate through a generic capability broker.

## Goals

1. Remove all AI-specific code from main app (domain, infrastructure, settings)
2. Create generic plugin capability system where plugins define and consume capabilities
3. Enable plugin-to-plugin communication via host-brokered connections
4. Plugins define their own settings via JSON Schema (existing pattern)
5. Fallback to LIKE/FTS search when no search plugin is configured

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         MAIN APP                                │
│  - HostStorage (KV + SQL with plugin table prefixing)          │
│  - HostData (media read access)                                │
│  - HostPlugins (capability broker)                             │
│  - HostWeather (generic weather/location context)              │
│  - HostWidgets (plugin UI integration) [FUTURE]                │
│  - HostUserActivity (secure user data access) [FUTURE]         │
│  - HostScheduler (background tasks/events) [FUTURE]            │
│  - NO HostAI, NO HostLLM, NO HostEmbeddings                   │
└─────────────────────────────────────────────────────────────────┘
                              │
              HostPlugins.GetPluginConnection("embedding")
                              │
    ┌─────────────────────────┼─────────────────────────┐
    ▼                         ▼                         ▼
┌─────────────┐       ┌─────────────┐           ┌─────────────┐
│   ollama    │       │   openai    │           │   voyage    │
│ capabilities│       │ capabilities│           │ capabilities│
│ [embedding, │       │ [embedding, │           │ [embedding] │
│  chat]      │       │  chat]      │           │             │
└──────┬──────┘       └─────────────┘           └─────────────┘
       │
       │ Direct gRPC (AIProvider service)
       │
       ├───────────────────────┐
       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐
│ semantic-search │    │ recommendations │ [FUTURE]
│ capabilities:   │    │ capabilities:   │
│ [search]        │    │ [recommendations]│
│ requires:       │    │ requires:       │
│ [embedding]     │    │ [embedding]     │
│                 │    │ uses:           │
│                 │    │ - HostUserActivity
│                 │    │ - HostScheduler │
│                 │    │ - HostWidgets   │
└─────────────────┘    └─────────────────┘
```

## Key Design Decisions

### Capability Routing
- Per-plugin configuration (no global routing table)
- Plugins that need a capability include a `x-viewra-capability` field in their settings schema
- Frontend renders this as a dropdown of all plugins providing that capability
- Shows all providers (enabled or not) with warning if selected provider is disabled

### Plugin Dependencies
- Declared in `plugin.yml` manifest under `requires` field
- Host checks if required capabilities have available providers
- If dependency unmet: show error banner, disable plugin settings, prevent enabling

### Plugin Storage
- Plugins can create tables in main database via `HostStorage.ExecuteSQL()`
- Tables are prefixed with `plugin_{id}_` for namespace isolation
- Plugins track their own schema migrations

### Fallback Search
- `/api/search` checks if `search` capability has a configured provider
- If yes: proxy to plugin's HTTP route
- If no: use built-in LIKE/FTS on title, plot, tagline

---

## Future Considerations: Recommendations Engine

This refactoring lays groundwork for a future recommendations plugin. The following 
preliminary work is included in the current phases to ensure the architecture supports it.

### Recommendations Plugin Overview

A separate plugin from semantic-search that provides personalized recommendations:

```yaml
# plugins/recommendations/plugin.yml
id: recommendations
name: Recommendations
version: 1.0.0
description: AI-powered personalized recommendations
type: enricher
capabilities:
  - recommendations
requires:
  - embedding
optional:
  - chat  # For generating explanations
permissions:
  - user_activity:read  # Requires admin approval
```

### Recommendations Capability Parameters

Single `recommendations` capability with context parameter:

```
GET /api/recommendations?context=<context>&user=<user_id>
```

| Context | Description |
|---------|-------------|
| `continue_watching` | Resume partially watched content |
| `because_you_watched` | Based on recently watched item (requires `item_id` param) |
| `similar_to` | Content-based similarity (requires `item_id` param) |
| `trending` | Popular items across all users |
| `mood_based` | Time of day / weather context |
| `personalized` | Full user profile analysis |
| `discovery` | New content outside user's usual preferences |

### HostUserActivity Service (Future - Phase 10)

Secure service for plugins to access user activity data with privacy protections.

#### Security Model

1. **Pseudonymized User IDs**
   - Plugins receive globally consistent pseudonyms, not real user IDs
   - Host maintains bidirectional mapping
   - Pseudonyms are deterministic (same user = same pseudonym across plugins)
   - Prevents correlation with external data sources

2. **Scoped Access Control**
   - Plugins declare required permissions in manifest
   - Admin grants/denies per data type per plugin
   - Available scopes:
     - `watch_history:read` - What user has watched
     - `ratings:read` - User ratings
     - `favorites:read` - Watchlist/favorites
     - `progress:read` - Playback progress/completion
     - `preferences:read` - Explicit user preferences

3. **Audit Logging**
   - All user data access logged silently
   - Admin can review access logs per plugin
   - Logs include: plugin_id, pseudonym, data_type, timestamp
   - Retention configurable by admin

4. **No Caching Without TTL**
   - Plugins must fetch fresh data or use TTL-limited cache
   - Host enforces maximum cache TTL (configurable, default 1 hour)
   - Plugins declare cache intent, host validates

#### Proto Definition

```protobuf
// HostUserActivity provides secure access to user activity data
// Plugins must have appropriate permissions granted by admin
service HostUserActivity {
  // GetWatchHistory returns user's watch history
  // Requires: watch_history:read permission
  rpc GetWatchHistory(WatchHistoryRequest) returns (WatchHistoryResponse);
  
  // GetRatings returns user's ratings
  // Requires: ratings:read permission
  rpc GetRatings(RatingsRequest) returns (RatingsResponse);
  
  // GetFavorites returns user's watchlist/favorites
  // Requires: favorites:read permission
  rpc GetFavorites(FavoritesRequest) returns (FavoritesResponse);
  
  // GetPlaybackProgress returns completion percentages
  // Requires: progress:read permission
  rpc GetPlaybackProgress(ProgressRequest) returns (ProgressResponse);
  
  // GetUserPreferences returns explicit user preferences
  // Requires: preferences:read permission
  rpc GetUserPreferences(PreferencesRequest) returns (PreferencesResponse);
}

message WatchHistoryRequest {
  string user_pseudonym = 1;
  int32 limit = 2;
  int64 since_timestamp = 3;  // Unix timestamp, 0 = all time
  string media_type = 4;       // Filter by type, empty = all
  int32 cache_ttl_seconds = 5; // 0 = no cache, max enforced by host
}

message WatchHistoryResponse {
  repeated WatchedItem items = 1;
  bool from_cache = 2;
}

message WatchedItem {
  int64 media_id = 1;
  string media_type = 2;
  int64 watched_at = 3;        // Unix timestamp
  float completion = 4;         // 0.0 - 1.0
  int32 watch_count = 5;
}

// Similar messages for Ratings, Favorites, Progress, Preferences...
```

#### SDK Client

```go
// UserActivityClient provides secure access to user data
type UserActivityClient struct {
    client pluginv1.HostUserActivityClient
}

// GetWatchHistory returns user's watch history
// Returns error if plugin lacks watch_history:read permission
func (c *UserActivityClient) GetWatchHistory(ctx context.Context, userPseudonym string, opts WatchHistoryOptions) ([]WatchedItem, error)

// Helper to get user pseudonym from real user ID (for HTTP handlers)
func (c *UserActivityClient) GetPseudonym(ctx context.Context, userID string) (string, error)
```

### HostScheduler Service (Future - Phase 11)

Service for plugins to register background tasks and event-triggered jobs.

#### Capabilities

1. **Cron-like Scheduling**
   - Register tasks with cron expressions
   - Host manages execution timing
   - Plugin receives callback when task should run

2. **Event-Triggered Tasks**
   - Subscribe to system events
   - Available events:
     - `user.watch.completed` - User finished watching item
     - `user.watch.started` - User started watching item
     - `user.rating.added` - User rated an item
     - `library.scan.completed` - Library scan finished
     - `media.added` - New media added
   - Plugin receives event payload

3. **Background Job Queue**
   - Submit long-running jobs
   - Host manages job execution
   - Progress reporting
   - Job cancellation

#### Proto Definition

```protobuf
// HostScheduler provides task scheduling and event subscriptions
service HostScheduler {
  // RegisterScheduledTask registers a recurring task
  rpc RegisterScheduledTask(ScheduledTaskRequest) returns (TaskRegistration);
  
  // UnregisterTask removes a registered task
  rpc UnregisterTask(TaskId) returns (Empty);
  
  // SubscribeToEvent registers interest in system events
  rpc SubscribeToEvent(EventSubscription) returns (SubscriptionId);
  
  // UnsubscribeFromEvent removes event subscription
  rpc UnsubscribeFromEvent(SubscriptionId) returns (Empty);
  
  // SubmitBackgroundJob queues a long-running job
  rpc SubmitBackgroundJob(BackgroundJobRequest) returns (JobId);
  
  // GetJobStatus returns status of a background job
  rpc GetJobStatus(JobId) returns (JobStatus);
  
  // CancelJob cancels a running or queued job
  rpc CancelJob(JobId) returns (Empty);
  
  // ReportJobProgress updates progress for current job
  rpc ReportJobProgress(JobProgress) returns (Empty);
}

message ScheduledTaskRequest {
  string task_id = 1;           // Unique ID within plugin
  string cron_expression = 2;   // Standard cron format
  string description = 3;
}

message EventSubscription {
  string event_type = 1;        // e.g., "user.watch.completed"
  string callback_route = 2;    // Plugin HTTP route to call
}

message BackgroundJobRequest {
  string job_type = 1;          // e.g., "recompute_recommendations"
  bytes payload = 2;            // JSON job parameters
  int32 priority = 3;           // Higher = more urgent
  int32 timeout_seconds = 4;    // Max execution time
}

message JobStatus {
  string job_id = 1;
  string status = 2;            // "queued", "running", "completed", "failed", "cancelled"
  float progress = 3;           // 0.0 - 1.0
  string current_step = 4;
  int64 started_at = 5;
  int64 completed_at = 6;
  string error = 7;
}

message JobProgress {
  string job_id = 1;
  float progress = 2;
  string current_step = 3;
}
```

#### Event Payload Example

When `user.watch.completed` fires, plugin receives HTTP POST to callback route:

```json
{
  "event": "user.watch.completed",
  "timestamp": 1703894400,
  "data": {
    "user_pseudonym": "usr_a1b2c3d4",
    "media_id": 12345,
    "media_type": "movie",
    "completion": 0.95,
    "watch_duration_seconds": 7200
  }
}
```

#### SDK Client

```go
// SchedulerClient provides task scheduling and event handling
type SchedulerClient struct {
    client pluginv1.HostSchedulerClient
}

// RegisterCronTask registers a recurring task
func (c *SchedulerClient) RegisterCronTask(ctx context.Context, taskID, cron, description string) error

// SubscribeToEvent registers for system events
// handler is the plugin's HTTP route that will receive event POSTs
func (c *SchedulerClient) SubscribeToEvent(ctx context.Context, eventType, handlerRoute string) (string, error)

// SubmitJob queues a background job and returns job ID
func (c *SchedulerClient) SubmitJob(ctx context.Context, jobType string, payload any, opts JobOptions) (string, error)

// WaitForJob blocks until job completes, returning final status
func (c *SchedulerClient) WaitForJob(ctx context.Context, jobID string) (*JobStatus, error)

// ReportProgress updates progress for the currently executing job
func (c *SchedulerClient) ReportProgress(ctx context.Context, progress float32, step string) error
```

### Recommendations Plugin Architecture

```
plugins/recommendations/
├── plugin.yml
├── main.go
├── go.mod
└── internal/
    ├── plugin.go           # EnricherPlugin + HTTPEnricher implementation
    ├── schema.go           # Settings schema
    ├── storage.go          # User preference cache, computed recommendations
    ├── recommender.go      # Core recommendation logic
    ├── contexts/
    │   ├── continue.go     # Continue watching logic
    │   ├── similar.go      # Content-based similarity
    │   ├── personalized.go # Full profile analysis
    │   ├── trending.go     # Popular items
    │   ├── mood.go         # Time/weather context
    │   └── discovery.go    # Outside-comfort-zone picks
    ├── events.go           # Event handlers for user.watch.completed etc.
    └── jobs.go             # Background job handlers
```

#### Recommendation Flow

1. **Event Trigger**: User finishes watching a movie
2. **Host Scheduler**: Fires `user.watch.completed` event to plugin
3. **Plugin Event Handler**: 
   - Receives event at `/events/watch-completed`
   - Submits background job to recompute recommendations
4. **Background Job**:
   - Fetches user's watch history via `HostUserActivity`
   - Fetches embeddings for watched items via `AIProvider` (embedding capability)
   - Computes updated recommendations
   - Caches results in plugin storage with TTL
5. **API Request**: When user opens app, frontend calls `/api/recommendations`
6. **Plugin Handler**:
   - Returns cached recommendations if fresh
   - Falls back to on-demand computation if cache expired

### Preliminary Work in Current Phases

The following tasks are added to current phases to support the recommendations engine:

#### Phase 2 Additions (Capability Broker)

| Task | Description |
|------|-------------|
| 2.10 | Design `HostUserActivity` proto (not implemented, just schema) |
| 2.11 | Design `HostScheduler` proto (not implemented, just schema) |
| 2.12 | Add `permissions` field to plugin manifest schema |
| 2.13 | Add `optional` capabilities field to manifest (soft dependencies) |

#### Phase 3 Additions (AIProvider Interface)

| Task | Description |
|------|-------------|
| 3.5 | Ensure AIProvider supports batch operations efficiently (for recommendation computation) |
| 3.6 | Add model capability introspection (dimensions, context length) for compatibility checks |

#### Phase 5 Additions (semantic-search)

| Task | Description |
|------|-------------|
| 5.7 | Ensure embeddings storage schema supports future cross-plugin queries |
| 5.8 | Document embedding format/dimensions for recommendations plugin compatibility |

---

## Future Considerations: Widget System

A generic widget system allowing plugins to surface content in predefined UI zones.
This supports the recommendations plugin and enables future plugins to add UI elements.

### Widget System Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         MAIN APP                                │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ HostWidgets Service                                      │   │
│  │  - RegisterWidget()     - Plugin registers a widget     │   │
│  │  - UnregisterWidget()   - Plugin removes a widget       │   │
│  │  - GetWidgetsForZone()  - Frontend/device queries zone  │   │
│  │  - GetWidgetData()      - Fetch data for async widgets  │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│  ┌───────────────────────────┴───────────────────────────┐     │
│  │ User Widget Preferences                                │     │
│  │  - Zone ordering                                       │     │
│  │  - Hidden widgets                                      │     │
│  │  - Per-user customization                              │     │
│  └───────────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        ▼                     ▼                     ▼
┌───────────────┐    ┌───────────────┐    ┌───────────────┐
│recommendations│    │ other-plugin  │    │ future-plugin │
│               │    │               │    │               │
│ Widgets:      │    │ Widgets:      │    │ Widgets:      │
│ - Continue    │    │ - Upcoming    │    │ - ...         │
│ - For You     │    │ - Recently    │    │               │
│ - Similar     │    │   Added       │    │               │
└───────────────┘    └───────────────┘    └───────────────┘
```

### Predefined Zones

| Zone | Location | Description |
|------|----------|-------------|
| `homepage` | Main content area | Primary widgets on home screen |
| `homepage_hero` | Top of homepage | Featured/spotlight content |
| `media_detail_related` | Media detail page | Related content sidebar/section |
| `post_playback` | After video ends | "Up Next" / "More Like This" |
| `search_suggestions` | Search page | Suggestions below results |

### Widget Display Types

| Type | Description | Use Case |
|------|-------------|----------|
| `carousel` | Horizontal scrolling row | Continue watching, recommendations |
| `grid` | Grid layout of items | Browse categories, search results |
| `list` | Vertical list | Episode lists, queue |
| `hero` | Large featured item | Spotlight, featured content |
| `compact` | Small inline widget | Quick stats, minimal display |

### Widget Content

Widgets display media items only:
- Movies
- TV Shows
- TV Episodes
- Music Artists
- Music Albums
- Music Tracks

### Widget Data Loading

Widgets support two data modes:

1. **Embedded Data** - Widget includes media items in registration
   - Good for small, static widgets
   - No additional fetch required
   - Plugin must keep data updated

2. **Async Data** - Widget defines data endpoint
   - Good for large, dynamic widgets
   - Frontend fetches via `GetWidgetData()`
   - Supports pagination

### User Customization

Users can personalize their widget layout:
- Reorder widgets within a zone
- Hide widgets they don't want
- Reset to plugin defaults
- Per-zone preferences

### HostWidgets Proto Definition

```protobuf
// HostWidgets allows plugins to register UI widgets
service HostWidgets {
  // RegisterWidget adds a widget to a zone
  rpc RegisterWidget(WidgetDefinition) returns (WidgetRegistration);
  
  // UnregisterWidget removes a widget
  rpc UnregisterWidget(WidgetId) returns (Empty);
  
  // UpdateWidget updates widget metadata or embedded data
  rpc UpdateWidget(WidgetUpdate) returns (Empty);
  
  // GetWidgetsForZone returns widgets for a zone (respects user prefs)
  rpc GetWidgetsForZone(ZoneRequest) returns (WidgetList);
  
  // GetWidgetData fetches data for async widgets
  rpc GetWidgetData(WidgetDataRequest) returns (WidgetDataResponse);
}

message WidgetDefinition {
  string widget_id = 1;           // Unique within plugin, e.g., "continue_watching"
  string zone = 2;                // Target zone, e.g., "homepage"
  int32 default_priority = 3;     // Default sort order (lower = higher)
  WidgetMetadata metadata = 4;
  WidgetDataMode data_mode = 5;
  repeated MediaItem embedded_items = 6;  // For embedded mode
  string data_endpoint = 7;               // For async mode (plugin HTTP route)
}

message WidgetMetadata {
  string title = 1;                // Display title, supports {placeholders}
  string subtitle = 2;             // Optional subtitle
  string display_type = 3;         // carousel, grid, list, hero, compact
  string icon = 4;                 // Optional icon name
  int32 max_items = 5;             // Max items to display
  bool user_hideable = 6;          // Can user hide this widget (default true)
  map<string, string> context = 7; // Additional context for title placeholders
}

enum WidgetDataMode {
  EMBEDDED = 0;   // Data included in widget definition
  ASYNC = 1;      // Data fetched via GetWidgetData or plugin endpoint
}

message WidgetRegistration {
  string registration_id = 1;     // Global unique ID
  bool success = 2;
  string error = 3;
}

message WidgetId {
  string widget_id = 1;
  string plugin_id = 2;           // Set by host, not plugin
}

message WidgetUpdate {
  string widget_id = 1;
  WidgetMetadata metadata = 2;           // Optional: update metadata
  repeated MediaItem embedded_items = 3;  // Optional: update embedded data
}

message ZoneRequest {
  string zone = 1;
  string user_id = 2;             // For user-specific preferences
  string device_type = 3;         // web, mobile, tv, etc.
}

message WidgetList {
  repeated Widget widgets = 1;
}

message Widget {
  string registration_id = 1;
  string plugin_id = 2;
  string widget_id = 3;
  WidgetMetadata metadata = 4;
  WidgetDataMode data_mode = 5;
  repeated MediaItem items = 6;   // Populated for embedded mode
  int32 position = 7;             // User-customized position
  bool hidden = 8;                // User hid this widget
}

message WidgetDataRequest {
  string registration_id = 1;
  string user_id = 2;
  int32 limit = 3;
  int32 offset = 4;               // For pagination
  map<string, string> params = 5; // Additional parameters
}

message WidgetDataResponse {
  repeated MediaItem items = 1;
  int32 total_count = 2;
  bool has_more = 3;
}

message MediaItem {
  int64 media_id = 1;
  string media_type = 2;          // movie, tv_show, tv_episode, etc.
  string title = 3;
  string subtitle = 4;            // e.g., "S1 E5" or artist name
  string image_url = 5;
  float progress = 6;             // 0.0-1.0 for continue watching
  map<string, string> metadata = 7;  // Additional display metadata
}
```

### SDK Widget Client

```go
// WidgetsClient allows plugins to register and manage widgets
type WidgetsClient struct {
    client pluginv1.HostWidgetsClient
}

// RegisterWidget registers a widget in a zone
func (c *WidgetsClient) RegisterWidget(ctx context.Context, def WidgetDefinition) (string, error)

// UnregisterWidget removes a widget
func (c *WidgetsClient) UnregisterWidget(ctx context.Context, widgetID string) error

// UpdateWidget updates widget metadata or data
func (c *WidgetsClient) UpdateWidget(ctx context.Context, widgetID string, update WidgetUpdate) error

// UpdateEmbeddedItems is a convenience method to update embedded widget data
func (c *WidgetsClient) UpdateEmbeddedItems(ctx context.Context, widgetID string, items []MediaItem) error
```

### User Preferences Storage

```sql
CREATE TABLE user_widget_preferences (
  user_id TEXT NOT NULL,
  zone TEXT NOT NULL,
  registration_id TEXT NOT NULL,
  position INTEGER,
  hidden BOOLEAN DEFAULT FALSE,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, zone, registration_id)
);
```

### Frontend Widget Rendering

Frontend queries widgets for each zone and renders based on display type:

```typescript
// Fetch widgets for homepage
const { widgets } = await api.getWidgetsForZone('homepage', userId)

// Render each widget
widgets.forEach(widget => {
  switch (widget.metadata.displayType) {
    case 'carousel':
      return <CarouselWidget widget={widget} />
    case 'grid':
      return <GridWidget widget={widget} />
    case 'hero':
      return <HeroWidget widget={widget} />
    // ...
  }
})

// For async widgets, fetch data separately
if (widget.dataMode === 'ASYNC') {
  const data = await api.getWidgetData(widget.registrationId, { limit: 20 })
  widget.items = data.items
}
```

---

## Future Considerations: Third-Party Device API

External devices (TV apps, mobile apps, Kodi plugins) need to discover and display widgets.

### Device API Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                     Third-Party Device                          │
│  (Smart TV, Mobile App, Kodi, etc.)                            │
└─────────────────────────────────────────────────────────────────┘
                              │
                         HTTPS/REST
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     ViewRA Server                               │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ Device API (/api/device/v1/*)                           │   │
│  │  - POST /auth          - Device authentication          │   │
│  │  - GET /capabilities   - Query device capabilities      │   │
│  │  - GET /zones          - List available zones           │   │
│  │  - GET /zones/:zone    - Get widgets for zone           │   │
│  │  - GET /widgets/:id    - Get widget data                │   │
│  │  - GET /media/:id      - Get media details              │   │
│  │  - POST /playback      - Start playback                 │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### Device Authentication

Devices authenticate via:
1. **Device Code Flow** - For TVs/limited input devices
2. **API Token** - For programmatic access
3. **OAuth** - For mobile apps with browser

```
POST /api/device/v1/auth/device-code
Response: {
  "device_code": "abc123",
  "user_code": "ABCD-1234",
  "verification_url": "https://viewra.local/device",
  "expires_in": 600
}

// User enters code in web UI, device polls:
POST /api/device/v1/auth/token
Body: { "device_code": "abc123" }
Response: {
  "access_token": "...",
  "refresh_token": "...",
  "expires_in": 86400
}
```

### Device Capability Negotiation

Devices declare their capabilities, server adapts responses:

```
POST /api/device/v1/capabilities
Body: {
  "device_type": "tv",
  "display_types": ["carousel", "grid", "list"],
  "max_items_per_widget": 20,
  "supports_hero": false,
  "image_sizes": ["thumbnail", "poster"],
  "video_codecs": ["h264", "hevc"],
  "audio_codecs": ["aac", "ac3"]
}
Response: {
  "session_id": "...",
  "acknowledged": true
}
```

### Device Widget API

```
GET /api/device/v1/zones
Response: {
  "zones": [
    {"id": "homepage", "name": "Home"},
    {"id": "homepage_hero", "name": "Featured"},
    ...
  ]
}

GET /api/device/v1/zones/homepage
Headers: X-Device-Session: <session_id>
Response: {
  "widgets": [
    {
      "id": "rec_continue",
      "title": "Continue Watching",
      "display_type": "carousel",
      "items": [...],
      "has_more": true,
      "data_url": "/api/device/v1/widgets/rec_continue"
    },
    ...
  ]
}

GET /api/device/v1/widgets/rec_continue?limit=20&offset=0
Response: {
  "items": [
    {
      "media_id": 123,
      "media_type": "movie",
      "title": "Movie Name",
      "image": {
        "thumbnail": "https://...",
        "poster": "https://..."
      },
      "progress": 0.45,
      "playback_url": "/api/device/v1/playback/123"
    }
  ],
  "total": 50,
  "has_more": true
}
```

### Device Playback Integration

```
POST /api/device/v1/playback
Body: {
  "media_id": 123,
  "media_type": "movie",
  "resume": true
}
Response: {
  "stream_url": "https://viewra.local/stream/...",
  "resume_position": 2700,
  "subtitles": [...],
  "audio_tracks": [...]
}
```

### Security Considerations

1. **Token Scoping** - Device tokens have limited permissions
2. **Rate Limiting** - Per-device rate limits
3. **Audit Logging** - All device API calls logged
4. **Token Revocation** - Admin can revoke device access
5. **HTTPS Required** - No plaintext device API

---

## Phase 1: Host-Managed SQL Storage for Plugins

Provide plugins with managed SQL storage in the host's database. This follows the pattern
used by most mature plugin systems (Grafana, HashiCorp, VS Code) where the host provides
storage primitives rather than plugins managing their own databases.

### Design Principles

1. **Host-managed** - Plugins never get raw database access, only RPC interface
2. **Automatic namespacing** - All table names prefixed with `plugin_{id}_` by host
3. **Safety enforced** - Host parses SQL and blocks dangerous operations
4. **Dual-DB compatible** - SQL must work on both Postgres and SQLite
5. **No driver duplication** - Single SQLite/Postgres driver in host, not per-plugin

### Security Model

**Allowed operations:**
- `CREATE TABLE`, `CREATE INDEX`, `DROP TABLE`, `DROP INDEX`
- `SELECT`, `INSERT`, `UPDATE`, `DELETE`

**Blocked operations:**
- `ATTACH DATABASE`, `DETACH DATABASE`
- `PRAGMA` (except safe read-only ones)
- Access to non-prefixed tables
- Subqueries referencing system/other plugin tables

**Resource limits:**
- Query timeout (configurable, default 30s)
- Max rows returned (configurable, default 10000)
- Storage quota per plugin (future)

### 1.1 Extend Proto - HostStorage SQL Methods

**File:** `api/proto/plugin/host_services.proto`

Add to `HostStorage` service:
```protobuf
service HostStorage {
  // Existing KV methods...

  // ExecuteSQL runs DDL/DML on plugin's namespaced tables.
  // Table names are automatically prefixed with plugin_{id}_
  rpc ExecuteSQL(SQLRequest) returns (SQLExecResult);

  // QuerySQL runs a SELECT query and returns typed results.
  // Table names are automatically prefixed with plugin_{id}_
  rpc QuerySQL(SQLRequest) returns (SQLQueryResult);
}

message SQLRequest {
  string sql = 1;
  repeated SQLValue args = 2;
}

message SQLValue {
  oneof value {
    string string_value = 1;
    int64 int_value = 2;
    double double_value = 3;
    bytes bytes_value = 4;
    bool is_null = 5;
  }
}

message SQLExecResult {
  int64 rows_affected = 1;
  int64 last_insert_id = 2;
}

message SQLQueryResult {
  repeated string columns = 1;
  repeated SQLRow rows = 2;
}

message SQLRow {
  repeated SQLValue values = 1;
}
```

### 1.2 Create SDK Database Client

**File:** `pkg/plugin/sdk/database.go`

```go
// SQLClient provides managed SQL access for plugins.
// All table names are automatically prefixed by the host.
type SQLClient struct {
    client pluginv1.HostStorageClient
}

// Exec executes DDL/DML statements (CREATE, INSERT, UPDATE, DELETE).
func (c *SQLClient) Exec(ctx context.Context, sql string, args ...any) (rowsAffected int64, lastID int64, err error)

// Query executes a SELECT and returns a Rows iterator.
func (c *SQLClient) Query(ctx context.Context, sql string, args ...any) (*Rows, error)

// QueryRow executes a SELECT expected to return at most one row.
func (c *SQLClient) QueryRow(ctx context.Context, sql string, args ...any) *Row

// Migrate runs schema migrations, tracking versions automatically.
// The host stores migration state in plugin_{id}__migrations table.
func (c *SQLClient) Migrate(ctx context.Context, migrations []Migration) error

// Migration represents a schema migration.
type Migration struct {
    Version int
    SQL     string
}

// Rows is an iterator over query results.
type Rows struct { /* ... */ }
func (r *Rows) Next() bool
func (r *Rows) Scan(dest ...any) error
func (r *Rows) Close() error
func (r *Rows) Err() error

// Row is a single result row.
type Row struct { /* ... */ }
func (r *Row) Scan(dest ...any) error
```

### 1.3 Update StorageClient

**File:** `pkg/plugin/sdk/host.go`

Add SQL accessor to StorageClient:
```go
// SQL returns a client for managed SQL storage.
// All table names are automatically prefixed with plugin_{id}_
func (c *StorageClient) SQL() *SQLClient
```

### 1.4 Implement Host-Side SQL Handler

**File:** `internal/infrastructure/plugins/host_storage_sql.go`

Implementation responsibilities:
1. **Parse SQL** - Extract table names from the query
2. **Validate operations** - Check against allowlist
3. **Rewrite table names** - Prefix with `plugin_{pluginID}_`
4. **Execute query** - Run on host's database connection
5. **Convert results** - Transform to proto messages

Table name parsing approach:
- Use simple regex for common patterns: `CREATE TABLE`, `FROM`, `INTO`, `UPDATE`, `JOIN`
- For complex queries, use `github.com/pingcap/tidb/parser` (supports both MySQL-like and can be adapted)
- Alternatively, use `github.com/kyleconroy/sqlc`'s parser or write a simple tokenizer

```go
type PluginSQLHandler struct {
    db       *sql.DB
    pluginID string
}

func (h *PluginSQLHandler) ExecuteSQL(ctx context.Context, req *pluginv1.SQLRequest) (*pluginv1.SQLExecResult, error) {
    // 1. Parse and validate SQL
    rewritten, err := h.rewriteSQL(req.Sql)
    if err != nil {
        return nil, err
    }
    
    // 2. Convert args
    args := h.convertArgs(req.Args)
    
    // 3. Execute with timeout
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    result, err := h.db.ExecContext(ctx, rewritten, args...)
    if err != nil {
        return nil, err
    }
    
    rowsAffected, _ := result.RowsAffected()
    lastID, _ := result.LastInsertId()
    
    return &pluginv1.SQLExecResult{
        RowsAffected: rowsAffected,
        LastInsertId: lastID,
    }, nil
}

func (h *PluginSQLHandler) rewriteSQL(sql string) (string, error) {
    // Parse SQL, find table names, prefix with plugin_{h.pluginID}_
    // Validate no forbidden operations
}
```

### 1.5 SQL Parsing Strategy

For table name rewriting, we need to handle:
- `CREATE TABLE name` → `CREATE TABLE plugin_x_name`
- `FROM name` → `FROM plugin_x_name`
- `INTO name` → `INTO plugin_x_name`
- `UPDATE name` → `UPDATE plugin_x_name`
- `JOIN name` → `JOIN plugin_x_name`
- `DROP TABLE name` → `DROP TABLE plugin_x_name`

**Simple approach** (handles 95% of plugin use cases):
```go
// Regex-based rewriting for common patterns
var tablePatterns = []struct {
    pattern *regexp.Regexp
    rewrite string
}{
    {regexp.MustCompile(`(?i)\bCREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)`), "CREATE TABLE $prefix$1"},
    {regexp.MustCompile(`(?i)\bFROM\s+(\w+)`), "FROM $prefix$1"},
    {regexp.MustCompile(`(?i)\bJOIN\s+(\w+)`), "JOIN $prefix$1"},
    {regexp.MustCompile(`(?i)\bINTO\s+(\w+)`), "INTO $prefix$1"},
    {regexp.MustCompile(`(?i)\bUPDATE\s+(\w+)`), "UPDATE $prefix$1"},
    {regexp.MustCompile(`(?i)\bDROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?(\w+)`), "DROP TABLE $prefix$1"},
}
```

**Forbidden patterns:**
```go
var forbiddenPatterns = []*regexp.Regexp{
    regexp.MustCompile(`(?i)\bATTACH\b`),
    regexp.MustCompile(`(?i)\bDETACH\b`),
    regexp.MustCompile(`(?i)\bPRAGMA\b`),
    regexp.MustCompile(`(?i)\bpg_`),           // Postgres system tables
    regexp.MustCompile(`(?i)\bsqlite_`),       // SQLite system tables
    regexp.MustCompile(`(?i)\binformation_schema\b`),
}
```

### 1.6 Migration Tracking

The host automatically creates a migrations tracking table for each plugin:

```sql
-- Created automatically when first migration runs
CREATE TABLE plugin_{id}__migrations (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

Migration execution:
```go
func (c *SQLClient) Migrate(ctx context.Context, migrations []Migration) error {
    // 1. Ensure migrations table exists (host handles prefix)
    c.Exec(ctx, `CREATE TABLE IF NOT EXISTS _migrations (
        version INTEGER PRIMARY KEY,
        applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )`)
    
    // 2. Get current version
    var current int
    c.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM _migrations`).Scan(&current)
    
    // 3. Apply pending migrations in order
    for _, m := range migrations {
        if m.Version <= current {
            continue
        }
        if _, _, err := c.Exec(ctx, m.SQL); err != nil {
            return fmt.Errorf("migration %d failed: %w", m.Version, err)
        }
        c.Exec(ctx, `INSERT INTO _migrations (version) VALUES (?)`, m.Version)
    }
    return nil
}
```

### 1.7 Example Plugin Usage

```go
func (p *SemanticSearchPlugin) Initialize(ctx context.Context, req *pluginv1.InitRequest) (*pluginv1.InitResponse, error) {
    // Get SQL client from storage
    db := p.storage.SQL()
    
    // Run migrations - tables auto-prefixed to plugin_semantic_search_*
    err := db.Migrate(ctx, []sdk.Migration{
        {Version: 1, SQL: `
            CREATE TABLE embeddings (
                entity_type TEXT NOT NULL,
                entity_id INTEGER NOT NULL,
                embedding BLOB NOT NULL,
                text TEXT,
                model TEXT,
                created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                PRIMARY KEY (entity_type, entity_id)
            )
        `},
        {Version: 2, SQL: `CREATE INDEX idx_embeddings_type ON embeddings(entity_type)`},
    })
    if err != nil {
        return nil, fmt.Errorf("migration failed: %w", err)
    }
    
    p.db = db
    return &pluginv1.InitResponse{Success: true}, nil
}

func (p *SemanticSearchPlugin) StoreEmbedding(ctx context.Context, entityType string, entityID int64, embedding []float32) error {
    embBytes := float32SliceToBytes(embedding)
    _, _, err := p.db.Exec(ctx, `
        INSERT INTO embeddings (entity_type, entity_id, embedding)
        VALUES (?, ?, ?)
        ON CONFLICT (entity_type, entity_id) DO UPDATE SET embedding = ?
    `, entityType, entityID, embBytes, embBytes)
    return err
}

func (p *SemanticSearchPlugin) SearchEmbeddings(ctx context.Context, entityType string) ([]Embedding, error) {
    rows, err := p.db.Query(ctx, `
        SELECT entity_id, embedding, text FROM embeddings WHERE entity_type = ?
    `, entityType)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var results []Embedding
    for rows.Next() {
        var e Embedding
        var embBytes []byte
        if err := rows.Scan(&e.EntityID, &embBytes, &e.Text); err != nil {
            return nil, err
        }
        e.Embedding = bytesToFloat32Slice(embBytes)
        results = append(results, e)
    }
    return results, rows.Err()
}
```

### 1.8 Dual-Database Compatibility

Plugins must write SQL compatible with both Postgres and SQLite. Document these constraints:

**Compatible:**
- Basic types: `TEXT`, `INTEGER`, `REAL`, `BLOB`
- `CREATE TABLE`, `CREATE INDEX`
- `PRIMARY KEY`, `UNIQUE`, `NOT NULL`
- `INSERT`, `UPDATE`, `DELETE`, `SELECT`
- `WHERE`, `ORDER BY`, `LIMIT`, `OFFSET`
- `JOIN`, `LEFT JOIN`
- `COUNT`, `MAX`, `MIN`, `SUM`, `AVG`

**Avoid:**
- `RETURNING` (Postgres-specific, SQLite 3.35+ only)
- `ON CONFLICT DO UPDATE` works differently - use `INSERT OR REPLACE` for SQLite
- Array types (Postgres-specific)
- `SERIAL` (use `INTEGER PRIMARY KEY` for auto-increment)
- JSON operators (syntax differs)

**Helper for upsert:**
```go
// SDK provides helper for upsert that works on both DBs
func (c *SQLClient) Upsert(ctx context.Context, table string, columns []string, values []any, conflictColumns []string) error
```

### 1.9 Vector Storage

Host-managed vector storage with automatic indexing for similarity search.
Uses pgvector (PostgreSQL) or sqlite-vec (SQLite) under the hood.

#### Proto API

```protobuf
service HostStorage {
  // ... existing methods ...
  
  // Vector storage methods
  rpc VectorStoreEmbedding(VectorStoreRequest) returns (Empty);
  rpc VectorStoreBatch(VectorStoreBatchRequest) returns (Empty);
  rpc VectorSearch(VectorSearchRequest) returns (VectorSearchResponse);
  rpc VectorGet(VectorQuery) returns (VectorGetResponse);
  rpc VectorDelete(VectorQuery) returns (Empty);
  rpc VectorDeleteByType(VectorTypeQuery) returns (VectorDeleteResponse);
  rpc VectorCount(VectorTypeQuery) returns (VectorCountResponse);
}

message VectorStoreRequest {
  string entity_type = 1;
  int64 entity_id = 2;
  repeated float vector = 3;
  string text = 4;
  string model = 5;
}

message VectorSearchRequest {
  repeated float query_vector = 1;
  repeated string entity_types = 2;
  int32 limit = 3;
  int32 offset = 4;
  float min_similarity = 5;
}

message VectorSearchResult {
  string entity_type = 1;
  int64 entity_id = 2;
  float similarity = 3;
  string text = 4;
}
```

#### SDK API

**File:** `pkg/plugin/sdk/vector.go`

```go
// VectorClient provides managed vector storage for plugins
type VectorClient struct { /* ... */ }

// Store saves or updates an embedding
func (c *VectorClient) Store(ctx context.Context, emb Embedding) error

// StoreBatch saves multiple embeddings efficiently
func (c *VectorClient) StoreBatch(ctx context.Context, embeddings []Embedding) error

// Search performs similarity search
func (c *VectorClient) Search(ctx context.Context, req VectorSearchRequest) (*VectorSearchResponse, error)

// Get retrieves an embedding by entity type and ID
func (c *VectorClient) Get(ctx context.Context, entityType string, entityID int64) (*Embedding, error)

// Delete removes an embedding
func (c *VectorClient) Delete(ctx context.Context, entityType string, entityID int64) error

// DeleteByType removes all embeddings for an entity type
func (c *VectorClient) DeleteByType(ctx context.Context, entityType string) (int64, error)

// Count returns the number of embeddings
func (c *VectorClient) Count(ctx context.Context, entityType string) (int64, error)
```

Access via StorageClient:
```go
vec := storage.Vector()
```

#### Host Implementation

**File:** `internal/infrastructure/plugins/host_storage_vector.go`

- Auto-creates per-plugin embedding tables: `plugin_{id}_embeddings`
- PostgreSQL: Uses `vector(n)` type with HNSW index
- SQLite: Uses BLOB storage + `vec0` virtual table for KNN search
- Fallback: Brute-force cosine similarity if vec0 unavailable
- Tables auto-created on first store with detected dimensions

#### Example Plugin Usage

```go
func (p *SemanticSearchPlugin) Initialize(ctx context.Context, req *pluginv1.InitRequest) (*pluginv1.InitResponse, error) {
    p.vec = p.storage.Vector()
    return &pluginv1.InitResponse{Success: true}, nil
}

func (p *SemanticSearchPlugin) IndexMedia(ctx context.Context, mediaID int64, text string) error {
    // Generate embedding using AI provider (via capability broker)
    embedding, err := p.aiProvider.Embed(ctx, text)
    if err != nil {
        return err
    }
    
    // Store in host-managed vector storage
    return p.vec.Store(ctx, sdk.Embedding{
        EntityType: "movie",
        EntityID:   mediaID,
        Vector:     embedding,
        Text:       text,
        Model:      "nomic-embed-text",
    })
}

func (p *SemanticSearchPlugin) Search(ctx context.Context, query string) ([]sdk.VectorSearchResult, error) {
    // Generate query embedding
    queryVec, err := p.aiProvider.Embed(ctx, query)
    if err != nil {
        return nil, err
    }
    
    // Search
    resp, err := p.vec.Search(ctx, sdk.VectorSearchRequest{
        QueryVector:   queryVec,
        EntityTypes:   []string{"movie", "tv_show"},
        Limit:         20,
        MinSimilarity: 0.5,
    })
    if err != nil {
        return nil, err
    }
    
    return resp.Results, nil
}
```

---

## Phase 2: Generic Plugin Capability Broker ✅ COMPLETED

Replace hardcoded capability aliases with dynamic, plugin-declared capabilities.

### What Was Built

The capability broker enables any plugin to expose gRPC services that other plugins can consume.
The flow is:

```
Consumer Plugin                    Host                         Provider Plugin
      │                             │                                  │
      │ GetCapabilityProvider()     │                                  │
      │────────────────────────────>│                                  │
      │                             │ ExposeService(capability)        │
      │                             │─────────────────────────────────>│
      │                             │                                  │ broker.AcceptAndServe(id)
      │                             │<─────────────────────────────────│ returns broker_id
      │<────────────────────────────│                                  │
      │ broker.Dial(broker_id)      │                                  │
      │═══════════════════════════════════════════════════════════════>│
      │              Direct gRPC connection                            │
```

### Key Implementation Details

**1. ExposeService RPC** (`api/proto/plugin/plugin_core.proto`)
```protobuf
service PluginCore {
  // ... existing RPCs
  rpc ExposeService(ExposeServiceRequest) returns (ExposeServiceResponse);
}

message ExposeServiceRequest {
  string capability = 1;
  string requesting_plugin_id = 2;
}

message ExposeServiceResponse {
  bool success = 1;
  uint32 broker_id = 2;
  string error = 3;
  string service_name = 4;  // e.g., "viewra.plugin.v1.PluginProvider"
}
```

**2. HostPlugins Service** (`api/proto/plugin/host_services.proto`)
```protobuf
service HostPlugins {
  rpc GetCapabilityProvider(CapabilityRequest) returns (CapabilityProviderResponse);
  rpc ListCapabilities(Empty) returns (CapabilityListResponse);
  rpc ListProviders(CapabilityRequest) returns (ProviderListResponse);
}
```

**3. SDK PluginsClient** (`pkg/plugin/sdk/plugins_client.go`)
```go
type PluginsClient struct {
    client pluginv1.HostPluginsClient
    broker GRPCBroker
}

func (c *PluginsClient) GetConnection(ctx, capability) (*grpc.ClientConn, error)
func (c *PluginsClient) GetConnectionPreferred(ctx, capability, preferredPlugin) (*grpc.ClientConn, error)
func (c *PluginsClient) IsAvailable(ctx, capability) bool
func (c *PluginsClient) ListProviders(ctx, capability) ([]PluginProvider, error)
func (c *PluginsClient) ListCapabilities(ctx) ([]Capability, error)
```

**4. Provider ExposeService** (`pkg/plugin/sdk/provider.go`)
```go
func (s *providerCoreServer) ExposeService(ctx, req) (*ExposeServiceResponse, error) {
    brokerID := s.broker.NextId()
    go s.broker.AcceptAndServe(brokerID, func(opts) *grpc.Server {
        srv := grpc.NewServer(opts...)
        pluginv1.RegisterPluginProviderServer(srv, &providerServiceServer{...})
        return srv
    })
    return &ExposeServiceResponse{Success: true, BrokerId: brokerID, ...}, nil
}
```

**5. HostPluginsServer** (`internal/infrastructure/plugins/host_plugins.go`)
- Tracks capabilities from plugin manifests (`provides` field)
- Resolves capability requests to available plugins
- Calls provider's `ExposeService` to get broker ID
- Returns broker ID so consumer can dial

### Files Changed

| File | Changes |
|------|---------|
| `api/proto/plugin/plugin_core.proto` | Added `ExposeService` RPC and messages |
| `api/proto/plugin/host_services.proto` | Added `HostPlugins` service |
| `pkg/plugin/sdk/plugins_client.go` | New SDK client for capability discovery |
| `pkg/plugin/sdk/provider.go` | Implemented `ExposeService` with broker |
| `pkg/plugin/sdk/enricher.go` | Added `Plugins` to `HostServices`, wired up connection |
| `internal/infrastructure/plugins/host_plugins.go` | Capability registry and broker |
| `internal/infrastructure/plugins/grpc_plugin.go` | Added `HostPluginsGRPCPlugin` |
| `internal/infrastructure/plugins/manager.go` | Wired up HostPlugins service |

### 2.7 Add Admin API for Capabilities

**File:** `internal/api/handlers/plugin_capabilities.go`

```go
// GET /api/admin/capabilities
// Returns all capabilities and their providers
func (h *Handler) ListCapabilities(c *gin.Context)

// Response matches CapabilityList proto but as JSON
```

### 2.8 Update Plugin Manager

**File:** `internal/infrastructure/plugins/manager.go`

- Parse `capabilities` and `requires` from manifest on load
- Register capabilities with CapabilityRegistry
- Check `requires` before allowing plugin to be enabled
- Add `GetMissingDependencies(pluginID string) []string` method

### 2.9 Update Server Capability Routing

**File:** `internal/api/server.go`

Update `registerCapabilityAliases` to be dynamic:
- Query CapabilityRegistry for plugins with route-exposing capabilities
- Register aliases based on plugin HTTP routes and capability declarations
- Handle `search` capability specially for fallback behavior

---

## Phase 3: AIProvider Plugin Interface ✅ COMPLETED

Define the contract between AI provider plugins and consumer plugins.

**Already Exists:** The `PluginProvider` service in `api/proto/plugin/provider.proto` provides
this functionality. It includes:
- `GetCapabilities` - returns what the provider supports
- `GenerateEmbedding` / `GenerateEmbeddingBatch` - embedding generation
- `Chat` / `ChatStream` - chat completion
- `ListModels` - available models
- `HealthCheck` - provider health

The SDK provides `ProviderPlugin` interface in `pkg/plugin/sdk/provider.go` and `ServeProvider()`.

### 3.1 Create AIProvider Proto (Already Exists)

**File:** `api/proto/plugin/provider.proto`

```protobuf
syntax = "proto3";
package viewra.plugin.v1;

// AIProvider is implemented by plugins that provide LLM/embedding capabilities
// This is a plugin-to-plugin contract, not a host service
service AIProvider {
  rpc GetCapabilities(Empty) returns (AIProviderCapabilities);
  rpc Embed(EmbedRequest) returns (EmbedResponse);
  rpc EmbedBatch(EmbedBatchRequest) returns (EmbedBatchResponse);
  rpc Chat(ChatRequest) returns (ChatResponse);
  rpc ChatStream(ChatRequest) returns (stream ChatStreamChunk);
  rpc ListModels(Empty) returns (ModelList);
}

message AIProviderCapabilities {
  bool supports_embedding = 1;
  bool supports_chat = 2;
  repeated AIModel embedding_models = 3;
  repeated AIModel chat_models = 4;
}

message AIModel {
  string id = 1;
  string name = 2;
  int32 context_length = 3;
  int32 embedding_dimensions = 4;
}

message EmbedRequest {
  string text = 1;
  string model = 2;  // Optional, uses default if empty
}

message EmbedResponse {
  repeated float embedding = 1;
  int32 dimensions = 2;
  int32 tokens_used = 3;
}

message EmbedBatchRequest {
  repeated string texts = 1;
  string model = 2;
}

message EmbedBatchResponse {
  repeated EmbeddingResult embeddings = 1;
  int32 total_tokens = 2;
}

message EmbeddingResult {
  repeated float embedding = 1;
}

message ChatRequest {
  repeated ChatMessage messages = 1;
  string model = 2;
  float temperature = 3;
  int32 max_tokens = 4;
}

message ChatMessage {
  string role = 1;    // system, user, assistant
  string content = 2;
}

message ChatResponse {
  string content = 1;
  string finish_reason = 2;
  int32 prompt_tokens = 3;
  int32 completion_tokens = 4;
}

message ChatStreamChunk {
  string content = 1;
  bool done = 2;
  string finish_reason = 3;
}

message ModelList {
  repeated AIModel models = 1;
}
```

### 3.2 Create SDK AIProvider Interface

**File:** `pkg/plugin/sdk/ai_provider.go`

```go
// AIProvider is implemented by plugins that provide embedding/chat capabilities
type AIProvider interface {
    GetCapabilities(ctx context.Context) (*AIProviderCapabilities, error)
    Embed(ctx context.Context, text string, model string) ([]float32, error)
    EmbedBatch(ctx context.Context, texts []string, model string) ([][]float32, error)
    Chat(ctx context.Context, messages []ChatMessage, opts ChatOptions) (*ChatResponse, error)
    ChatStream(ctx context.Context, messages []ChatMessage, opts ChatOptions) (<-chan ChatStreamEvent, error)
    ListModels(ctx context.Context) ([]AIModel, error)
}

// Types matching proto messages
type AIProviderCapabilities struct { ... }
type AIModel struct { ... }
type ChatMessage struct { ... }
type ChatOptions struct { ... }
type ChatResponse struct { ... }
type ChatStreamEvent struct { ... }
```

### 3.3 Create SDK AIProvider Client

**File:** `pkg/plugin/sdk/ai_provider_client.go`

```go
// AIProviderClient wraps a gRPC connection to an AIProvider plugin
type AIProviderClient struct {
    client pluginv1.AIProviderClient
}

// NewAIProviderClient creates a client from a broker connection
func NewAIProviderClient(conn *grpc.ClientConn) *AIProviderClient

// Implement all AIProvider methods by delegating to gRPC client
```

### 3.4 Create SDK AIProvider Server

**File:** `pkg/plugin/sdk/ai_provider.go` (continued)

```go
// ServeAIProvider starts a plugin that implements AIProvider
func ServeAIProvider(impl AIProvider, logger hclog.Logger)

// AIProviderGRPCPlugin for go-plugin integration
type AIProviderGRPCPlugin struct { ... }
```

---

## Phase 4: Refactor Provider Plugins ✅ COMPLETED

Update existing provider plugins to implement AIProvider interface.

**Already Complete:** All provider plugins already implement `ProviderPlugin` interface
and use `ServeProvider()`. We updated their manifests to declare specific capabilities.

### 4.1 Update Plugin Manifests ✅

Manifests updated to include specific capabilities in `provides` field:

| Plugin | Capabilities |
|--------|-------------|
| provider-ollama | `embedding`, `chat` |
| provider-openai | `embedding`, `chat` |
| provider-anthropic | `chat` |
| provider-voyage | `embedding` |

### 4.2 Provider Plugins Already Implement ProviderPlugin ✅

All providers already:
- Implement `sdk.ProviderPlugin` interface
- Use `sdk.ServeProvider()` in main.go
- Have settings schemas for API keys, models, etc.
- Implement `ExposeService` (inherited from SDK `providerCoreServer`)

### 4.3 Provider Plugin Structure

Current structure matches the plan:
```
plugins/provider-{name}/
├── plugin.yml           # Manifest with capabilities
├── main.go              # Entry point, calls sdk.ServeProvider()
├── go.mod
└── internal/
    └── plugin.go        # ProviderPlugin implementation
```

---

## Phase 5: Refactor semantic-search Plugin

Update to use capability broker and manage own storage.

### 5.1 Update Manifest

**File:** `plugins/semantic-search/plugin.yml`
```yaml
id: semantic-search
name: Semantic Search
version: 1.0.0
description: AI-powered semantic search for media
type: enricher
capabilities:
  - search
requires:
  - embedding
```

### 5.2 Update Settings Schema

**File:** `plugins/semantic-search/internal/schema.go`

Add capability-select field:
```go
sdk.NewSchema("Semantic Search Settings").
    Property("embedding_provider", sdk.String().
        Title("Embedding Provider").
        Description("Plugin that generates text embeddings").
        CapabilitySelect("embedding").
        Required()).
    Property("similarity_threshold", sdk.Number().
        Title("Similarity Threshold").
        Default(0.5)).
    // ... other settings
```

### 5.3 Use Host-Managed Vector Storage ✅ ALREADY BUILT

**Approach Changed:** Instead of plugins managing their own SQL tables for embeddings,
we built host-managed vector storage in Phase 1. The host uses pgvector (Postgres) or
sqlite-vec (SQLite) for efficient vector similarity search.

**SDK VectorClient** (`pkg/plugin/sdk/vector.go`):
```go
// Get vector client from storage
vec := services.Storage.Vector()

// Store embedding
err := vec.Store(ctx, sdk.Embedding{
    EntityType: "movie",
    EntityID:   mediaID,
    Vector:     embedding,
    Text:       text,
    Model:      "nomic-embed-text",
})

// Search by similarity
results, err := vec.Search(ctx, sdk.VectorSearchRequest{
    QueryVector:   queryVec,
    EntityTypes:   []string{"movie", "tv_show"},
    Limit:         20,
    MinSimilarity: 0.5,
})

// Other operations
embedding, err := vec.Get(ctx, "movie", mediaID)
err := vec.Delete(ctx, "movie", mediaID)
count, err := vec.Count(ctx)
err := vec.DeleteByType(ctx, "movie")  // Delete all of a type
```

**Benefits of host-managed storage:**
- Uses native vector extensions (pgvector/sqlite-vec) for efficient similarity search
- Automatic plugin namespacing (no table prefix needed)
- Consistent storage backend across all plugins
- Host handles migrations and schema

### 5.4 Update AI Client Usage

**File:** `plugins/semantic-search/internal/embedding.go`

Use the capability broker to get a connection to an embedding provider, then use the
generated `PluginProviderClient` to call the provider's methods:

```go
// EmbeddingService generates embeddings using the configured provider
type EmbeddingService struct {
    plugins  *sdk.PluginsClient
    logger   *slog.Logger
}

func (s *EmbeddingService) getProviderClient(ctx context.Context) (pluginv1.PluginProviderClient, error) {
    // Get connection to any plugin providing "embedding" capability
    conn, err := s.plugins.GetConnection(ctx, "embedding")
    if err != nil {
        return nil, fmt.Errorf("failed to connect to embedding provider: %w", err)
    }
    
    // Create typed client from the connection
    return pluginv1.NewPluginProviderClient(conn), nil
}

func (s *EmbeddingService) Embed(ctx context.Context, text string) ([]float32, error) {
    client, err := s.getProviderClient(ctx)
    if err != nil {
        return nil, err
    }
    
    resp, err := client.GenerateEmbedding(ctx, &pluginv1.ProviderEmbeddingRequest{
        Text: text,
    })
    if err != nil {
        return nil, err
    }
    
    return resp.Embedding, nil
}

func (s *EmbeddingService) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
    client, err := s.getProviderClient(ctx)
    if err != nil {
        return nil, err
    }
    
    resp, err := client.GenerateEmbeddingBatch(ctx, &pluginv1.ProviderEmbeddingBatchRequest{
        Texts: texts,
    })
    if err != nil {
        return nil, err
    }
    
    embeddings := make([][]float32, len(resp.Embeddings))
    for i, e := range resp.Embeddings {
        embeddings[i] = e.Embedding
    }
    return embeddings, nil
}
```

### 5.5 Update Plugin Initialization

**File:** `plugins/semantic-search/internal/plugin.go`

```go
func (p *Plugin) Initialize(ctx context.Context, dataDir string, config []byte, services *sdk.HostServices) error {
    // Store host services for later use
    p.plugins = services.Plugins
    p.vector = services.Storage.Vector()
    p.data = services.Data
    
    // Check if embedding capability is available (soft check - will fail at runtime if not)
    if services.Plugins != nil && !services.Plugins.IsAvailable(ctx, "embedding") {
        p.Log().Warn("embedding capability not available - indexing will fail")
    }
    
    // Initialize embedding service (uses capability broker)
    p.embeddingService = NewEmbeddingService(services.Plugins, p.Log())
    
    // Initialize search service (uses host vector storage)
    p.searchService = NewSearchService(p.vector, p.embeddingService, p.Log())
    
    // Initialize indexing service
    p.indexingService = NewIndexingService(p.vector, p.embeddingService, p.data, p.Log())
    
    return nil
}
```

Key changes:
- Use `services.Storage.Vector()` for vector storage (not custom SQL tables)
- Use `services.Plugins` for capability broker access
- Remove dependency on `services.LLM` and `services.Embeddings` (host AI services)

### 5.6 Keep HTTP Routes

Existing HTTP routes remain:
- `GET /search` - semantic search
- `GET /similar/:type/:id` - find similar items  
- `GET /status` - indexing status
- `POST /index` - trigger indexing

### 5.7 Use ServeEnricher (Not ServeAISearchEnricher)

**File:** `plugins/semantic-search/main.go`

```go
func main() {
    hclogger, logger := sdk.NewLogger("semantic-search")
    plugin := internal.NewPlugin()
    plugin.SetLogger(logger)
    sdk.ServeEnricher(plugin, hclogger)
}
```

---

## Phase 6: Remove AI from Main App

Clean out all AI-specific code from the core application.

### 6.1 Delete Domain AI Package

**Delete:** `internal/domain/ai/` (entire directory)
- `types.go`
- `provider.go`
- `repository.go`
- `errors.go`

### 6.2 Delete Infrastructure AI

**Delete:** `internal/infrastructure/persistence/ai/` (entire directory)
- `embedding_repository.go`

### 6.3 Delete Host AI Services

**Delete:**
- `internal/infrastructure/plugins/host_embeddings.go`
- `internal/infrastructure/plugins/host_llm.go`

### 6.4 Delete AI Settings Handler

**Delete:** `internal/api/handlers/ai_settings.go`

### 6.5 Delete AI Config

**Delete:**
- `internal/application/settings/ai_config.go`
- `internal/application/settings/ai_config_test.go`

### 6.6 Remove AI Settings Definitions

**File:** `internal/domain/settings/definition.go`

Remove these settings:
- `ai.enabled`
- `ai.embedding_provider`
- `ai.chat_provider`
- `ai.max_results`
- `ai.similarity_threshold`

### 6.7 Clean Up Services

**File:** `internal/app/services/services.go`

Remove:
- AI service initialization
- Host LLM server creation
- Host embeddings server creation
- References to embedding repository

### 6.8 Clean Up Repositories

**File:** `internal/app/repositories/repositories.go`

Remove:
- Embedding repository initialization
- Embedding repository from struct

### 6.9 Update HostData Proto

**File:** `api/proto/plugin/host_services.proto`

Remove from `HostData` service:
- `GetMoodTags`
- `SetMoodTags`
- `DeleteMoodTags`

Remove from `MediaDetails` message:
- `mood_tags` field

Remove messages:
- `MoodTag`
- `MoodTagList`
- `SetMoodTagsRequest`

### 6.10 Create Migration to Drop AI Tables

**File:** `migrations/000XXX_remove_ai_tables.up.sql`

```sql
-- Drop embeddings table
DROP TABLE IF EXISTS embeddings;

-- Drop mood_tags table
DROP TABLE IF EXISTS mood_tags;

-- Drop old media_mood_tags if exists
DROP TABLE IF EXISTS media_mood_tags;

-- Drop cascade triggers
DROP TRIGGER IF EXISTS tr_media_delete_embeddings ON media;
DROP TRIGGER IF EXISTS tr_tv_shows_delete_embeddings ON tv_shows;
DROP TRIGGER IF EXISTS tr_tv_seasons_delete_embeddings ON tv_seasons;
DROP TRIGGER IF EXISTS tr_tv_episodes_delete_embeddings ON tv_episodes;
DROP TRIGGER IF EXISTS tr_music_artists_delete_embeddings ON music_artists;
DROP TRIGGER IF EXISTS tr_music_albums_delete_embeddings ON music_albums;
DROP TRIGGER IF EXISTS tr_music_tracks_delete_embeddings ON music_tracks;
```

---

## Phase 7: SDK Cleanup

Remove AI-specific SDK code.

### 7.1 Delete AI Search SDK

**Delete:** `pkg/plugin/sdk/ai_search.go`

This removes:
- `AISearchPlugin` interface
- `AISearchEnricherPlugin` interface
- `ServeAISearchEnricher()`
- All AI search specific types and converters

### 7.2 Delete Old AI Protos

**Delete:**
- `api/proto/plugin/plugin_ai.proto`
- `api/proto/plugin/host_ai.proto`

### 7.3 Update gRPC Plugin Definitions

**File:** `internal/infrastructure/plugins/grpc_plugin.go`

Remove:
- `AISearchGRPCPlugin`
- Related client/server code for AI search

Add:
- `AIProviderGRPCPlugin` for provider plugins

### 7.4 Update Enricher SDK

**File:** `pkg/plugin/sdk/enricher.go`

Update `ServeEnricher` and `ServeEnricherWithExtra`:
- Remove host LLM plugin registration
- Remove host embeddings plugin registration
- Add host plugins (capability broker) registration

Update `HostServices`:
- Remove `LLM *LLMClient`
- Remove `Embeddings *EmbeddingsClient`
- Ensure `Plugins *PluginsClient` is connected

---

## Phase 8: Fallback Search

Implement LIKE/FTS search when no search plugin is configured.

### 8.1 Create Media Search Service

**File:** `internal/application/media/search.go`

```go
// SearchService provides basic text search for media
type SearchService struct {
    db     *sql.DB
    logger *slog.Logger
}

// Search performs LIKE/FTS search on title, plot, tagline
func (s *SearchService) Search(ctx context.Context, query string, limit int) ([]MediaResult, error) {
    // Use LIKE for SQLite, to_tsvector for Postgres
    // Search across: title, original_title, plot, tagline
}

type MediaResult struct {
    ID        int64
    MediaType string
    Title     string
    Year      int
    Score     float32  // Match relevance
}
```

### 8.2 Create Fallback Search Handler

**File:** `internal/api/handlers/search.go`

```go
// SearchHandler handles /api/search requests
type SearchHandler struct {
    pluginManager *plugins.Manager
    searchService *media.SearchService
    logger        *slog.Logger
}

func (h *SearchHandler) Search(c *gin.Context) {
    query := c.Query("q")
    
    // Check if search capability is available
    if mapping := h.pluginManager.Capabilities().Resolve("search"); mapping != nil {
        // Proxy to plugin
        h.proxyToPlugin(c, mapping)
        return
    }
    
    // Fallback to basic search
    results, err := h.searchService.Search(c.Request.Context(), query, 20)
    if err != nil {
        c.JSON(500, gin.H{"error": "search failed"})
        return
    }
    
    c.JSON(200, gin.H{"results": results, "fallback": true})
}
```

### 8.3 Update Server Search Routing

**File:** `internal/api/server.go`

Register `/api/search` to use `SearchHandler` which:
1. Checks for `search` capability plugin
2. Proxies if available
3. Falls back to basic search if not

---

## Phase 9: Frontend Changes

Update frontend to remove AI-specific pages and add generic plugins settings.

### 9.1 Delete AI Settings Page

**Delete:** `web/src/views/settings/AISettings/` (entire directory)

### 9.2 Create Plugins Settings Page

**File:** `web/src/views/settings/PluginsSettings/PluginsSettings.tsx`

```tsx
export const PluginsSettings = () => {
  const { plugins, isLoading, error, enablePlugin, disablePlugin } = usePluginsData()

  return (
    <SettingsPage>
      <SettingsPage.Header
        title="Plugins"
        description="Manage plugins and their settings"
      />

      {/* Show warning if any plugins have unmet dependencies */}
      {plugins.some(p => p.missingDependencies?.length > 0) && (
        <Alert variant="warning" className="mb-6">
          Some plugins have unmet dependencies and cannot be enabled.
        </Alert>
      )}

      {/* Plugin cards */}
      {plugins.map(plugin => (
        <PluginCard
          key={plugin.id}
          plugin={plugin}
          onEnable={() => enablePlugin(plugin.id)}
          onDisable={() => disablePlugin(plugin.id)}
        />
      ))}
    </SettingsPage>
  )
}
```

### 9.3 Create Plugin Card Component

**File:** `web/src/views/settings/PluginsSettings/PluginCard.tsx`

```tsx
export const PluginCard = ({ plugin, onEnable, onDisable }) => {
  const hasMissingDeps = plugin.missingDependencies?.length > 0

  return (
    <SettingsPage.Card
      title={plugin.name}
      description={plugin.description}
      className="mt-6"
    >
      {/* Dependency error banner */}
      {hasMissingDeps && (
        <Alert variant="error" className="mb-4">
          <AlertTriangle className="w-4 h-4" />
          <span>
            Requires: {plugin.missingDependencies.map(cap => (
              <code key={cap} className="mx-1">{cap}</code>
            ))}
            <br />
            Enable and configure a plugin that provides these capabilities.
          </span>
        </Alert>
      )}

      {/* Enable/disable toggle */}
      <SettingRow
        type="toggle"
        label="Enabled"
        description="Enable or disable this plugin"
        value={plugin.enabled}
        onChange={(enabled) => enabled ? onEnable() : onDisable()}
        disabled={hasMissingDeps && !plugin.enabled}
      />

      {/* Plugin settings form */}
      {plugin.enabled && plugin.hasSettings && (
        <div className="mt-4 pt-4 border-t border-neutral-200/50 dark:border-white/10">
          <PluginSettingsForm pluginId={plugin.id} />
        </div>
      )}
    </SettingsPage.Card>
  )
}
```

### 9.4 Update PluginSettingsForm for Capability Select

**File:** `web/src/components/settings/forms/PluginSettingsForm/PluginSettingsForm.tsx`

Detect `x-viewra-capability` in schema properties and render as:
- Dropdown of all plugins providing that capability
- Show plugin name + enabled/disabled status
- Warning icon if selected plugin is disabled

```tsx
// In form field rendering
if (property['x-viewra-capability']) {
  const capability = property['x-viewra-capability']
  const providers = useCapabilityProviders(capability)
  
  return (
    <SettingRow
      type="select"
      label={property.title}
      description={property.description}
      value={formData[fieldName]}
      onChange={(value) => handleChange(fieldName, value)}
      options={providers.map(p => ({
        value: p.id,
        label: `${p.name}${!p.enabled ? ' (disabled)' : ''}`
      }))}
    />
  )
}
```

### 9.5 Add SDK CapabilitySelect Helper

**File:** `pkg/plugin/sdk/schema.go`

Add method to Property builder:
```go
// CapabilitySelect marks this field as a capability selector
// Frontend will render as dropdown of plugins providing the capability
func (p *Property) CapabilitySelect(capability string) *Property {
    if p.extensions == nil {
        p.extensions = make(map[string]any)
    }
    p.extensions["x-viewra-capability"] = capability
    return p
}
```

### 9.6 Create Plugins Data Hook

**File:** `web/src/views/settings/PluginsSettings/hooks/usePluginsData.ts`

```tsx
export const usePluginsData = () => {
  const { data, isLoading, error, refetch } = useGetApiPlugins()
  
  const enableMutation = usePostApiPluginsIdEnable()
  const disableMutation = usePostApiPluginsIdDisable()
  
  const plugins = useMemo(() => {
    if (data?.status !== 200) return []
    return data.data.plugins || []
  }, [data])
  
  const enablePlugin = async (id: string) => {
    await enableMutation.mutateAsync({ id })
    refetch()
  }
  
  const disablePlugin = async (id: string) => {
    await disableMutation.mutateAsync({ id })
    refetch()
  }
  
  return { plugins, isLoading, error, enablePlugin, disablePlugin }
}
```

### 9.7 Create Capability Providers Hook

**File:** `web/src/views/settings/PluginsSettings/hooks/useCapabilityProviders.ts`

```tsx
export const useCapabilityProviders = (capability: string) => {
  const { data } = useGetApiAdminCapabilities()
  
  return useMemo(() => {
    if (data?.status !== 200) return []
    const cap = data.data.capabilities?.find(c => c.name === capability)
    return cap?.providers || []
  }, [data, capability])
}
```

### 9.8 Update Routes

**File:** `web/src/routes/_layout/settings.plugins.tsx` (create)

```tsx
import { createFileRoute } from '@tanstack/react-router'
import { PluginsSettings } from '@/views/settings/PluginsSettings'

export const Route = createFileRoute('/_layout/settings/plugins')({
  component: PluginsSettings,
})
```

**Delete:** `web/src/routes/_layout/settings.ai.tsx`

### 9.9 Update Settings Navigation

Update navigation to replace "AI" with "Plugins" in settings sidebar.

---

## File Summary

### New Files (17)

| File | Purpose |
|------|---------|
| `api/proto/plugin/ai_provider.proto` | AIProvider service definition |
| `pkg/plugin/sdk/ai_provider.go` | AIProvider interface + types + server |
| `pkg/plugin/sdk/ai_provider_client.go` | Client for calling AIProvider |
| `pkg/plugin/sdk/plugins_client.go` | Client for capability broker |
| `internal/infrastructure/plugins/host_plugins.go` | Capability broker implementation |
| `internal/api/handlers/plugin_capabilities.go` | Admin API for capabilities |
| `internal/api/handlers/search.go` | Fallback search handler |
| `internal/application/media/search.go` | LIKE/FTS search implementation |
| `migrations/000XXX_add_plugin_capability_routes.up.sql` | Capability routing table |
| `migrations/000XXX_add_plugin_schema_versions.up.sql` | Plugin migration tracking |
| `migrations/000XXX_remove_ai_tables.up.sql` | Drop AI tables |
| `plugins/semantic-search/internal/storage.go` | Plugin's own embeddings storage |
| `web/src/views/settings/PluginsSettings/PluginsSettings.tsx` | Plugin settings page |
| `web/src/views/settings/PluginsSettings/PluginCard.tsx` | Plugin card component |
| `web/src/views/settings/PluginsSettings/hooks/usePluginsData.ts` | Plugins data hook |
| `web/src/views/settings/PluginsSettings/hooks/useCapabilityProviders.ts` | Capability providers hook |
| `web/src/routes/_layout/settings.plugins.tsx` | Plugins route |

### Delete Files (16)

| File | Reason |
|------|--------|
| `internal/domain/ai/types.go` | AI moves to plugins |
| `internal/domain/ai/provider.go` | AI moves to plugins |
| `internal/domain/ai/repository.go` | AI moves to plugins |
| `internal/domain/ai/errors.go` | AI moves to plugins |
| `internal/infrastructure/persistence/ai/embedding_repository.go` | AI moves to plugins |
| `internal/infrastructure/plugins/host_embeddings.go` | Replaced by capability broker |
| `internal/infrastructure/plugins/host_llm.go` | Replaced by capability broker |
| `internal/api/handlers/ai_settings.go` | No more AI settings |
| `internal/application/settings/ai_config.go` | No more AI config |
| `internal/application/settings/ai_config_test.go` | No more AI config |
| `api/proto/plugin/plugin_ai.proto` | Replaced by ai_provider.proto |
| `api/proto/plugin/host_ai.proto` | No host AI service |
| `pkg/plugin/sdk/ai_search.go` | semantic-search specific |
| `web/src/views/settings/AISettings/` | Replaced by PluginsSettings |
| `web/src/routes/_layout/settings.ai.tsx` | Replaced by settings.plugins |

### Modify Files (30+)

| File | Changes |
|------|---------|
| `api/proto/plugin/host_services.proto` | Add HostPlugins, HostStorage SQL, remove mood tags |
| `pkg/plugin/sdk/host.go` | Add SQL methods, Plugins client, remove LLM/Embeddings |
| `pkg/plugin/sdk/enricher.go` | Update HostServices, remove host AI plugins |
| `pkg/plugin/sdk/schema.go` | Add CapabilitySelect method |
| `internal/infrastructure/plugins/capabilities.go` | Dynamic capability tracking |
| `internal/infrastructure/plugins/manager.go` | Track capabilities from manifests |
| `internal/infrastructure/plugins/grpc_plugin.go` | Add AIProvider, remove AISearch |
| `internal/infrastructure/plugins/host_storage.go` | Implement SQL methods |
| `internal/infrastructure/plugins/manifest.go` | Add capabilities, requires fields |
| `internal/domain/settings/definition.go` | Remove AI settings |
| `internal/app/services/services.go` | Remove AI initialization |
| `internal/app/repositories/repositories.go` | Remove embedding repository |
| `internal/api/server.go` | Update capability routing |
| `plugins/provider-ollama/*` | Implement AIProvider |
| `plugins/provider-openai/*` | Implement AIProvider |
| `plugins/provider-voyage/*` | Implement AIProvider |
| `plugins/provider-anthropic/*` | Implement AIProvider |
| `plugins/semantic-search/*` | Use broker, own storage |
| `web/src/components/settings/forms/PluginSettingsForm/*` | Handle x-viewra-capability |
| `web/src/routeTree.gen.ts` | Updated routes |

---

## Execution Order

1. **Phase 1** - Storage SQL (non-breaking, additive)
2. **Phase 2** - Capability broker (non-breaking, additive)
3. **Phase 3** - AIProvider interface (non-breaking, additive)
4. **Phase 4** - Refactor providers (can run alongside old)
5. **Phase 5** - Refactor semantic-search (needs Phase 1-4)
6. **Phase 6** - Remove AI from main app (breaking)
7. **Phase 7** - SDK cleanup (after Phase 6)
8. **Phase 8** - Fallback search (after Phase 6)
9. **Phase 9** - Frontend (can partially parallel with Phase 6-8)

---

## Testing Checklist

### Phase 1-2 Tests
- [ ] Plugin can create table via ExecuteSQL
- [ ] Plugin can query table via QuerySQL
- [ ] Table names are properly prefixed
- [ ] Capability broker returns correct plugin connections
- [ ] Missing capability returns appropriate error

### Phase 3-4 Tests
- [ ] AIProvider plugins start successfully
- [ ] Embed/EmbedBatch work correctly
- [ ] Chat/ChatStream work correctly
- [ ] ListModels returns available models

### Phase 5 Tests
- [ ] semantic-search initializes with embedding provider
- [ ] semantic-search fails gracefully without provider
- [ ] Embeddings stored in plugin's table
- [ ] Search returns correct results

### Phase 6-7 Tests
- [ ] App starts without AI code
- [ ] No AI tables in database
- [ ] Existing functionality unaffected

### Phase 8 Tests
- [ ] Fallback search works without plugin
- [ ] Plugin search proxies correctly
- [ ] Graceful degradation

### Phase 9 Tests
- [ ] Plugins page lists all plugins
- [ ] Enable/disable works
- [ ] Dependency errors shown
- [ ] Capability select dropdowns work
- [ ] Settings save correctly

---

## Migration Notes

- Existing embeddings data will be dropped (confirmed)
- Users will need to re-configure plugins after upgrade
- semantic-search will need to re-index after upgrade
- No data migration needed - clean break
