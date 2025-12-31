# Home Screen, Widget System & Recommendations

## Overview

A multi-client home screen system with plugin-based widgets, search providers, trending data, and personalized recommendations. The API is designed to serve web, iOS, Android, Roku, Fire TV, and Smart TV clients from a single endpoint.

### Goals

1. **API-first design** - Single `/api/home` endpoint serves all clients
2. **Plugin-based widgets** - Plugins register home screen sections via settings schema
3. **Search provider system** - Multiple search backends with graceful fallback
4. **Trending capability** - Reusable interface for popularity data (TMDb, future: Trakt)
5. **Recommendations plugin** - AI-powered personalized recommendations with user ratings
6. **User customization** - Reorderable, hideable home sections (per-user)
7. **Multi-client support** - Web, iOS, Android, Roku, Fire TV, Smart TV

### Non-Goals

- Real-time push updates (future iteration)
- Collaborative filtering across users
- Machine learning model training

---

## Part 1: API Design

### Primary Endpoint: `GET /api/home`

Single endpoint returns everything a client needs to render the home screen.

**Query Parameters:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `inline` | bool | `true` | Include item data inline vs. return data URLs only |
| `sections` | string | `all` | Comma-separated section IDs or `all` |

**Headers (Optional):**

| Header | Values | Description |
|--------|--------|-------------|
| `X-Client-Type` | `web`, `ios`, `android`, `roku`, `firetv`, `smarttv` | Client hint for filtering widgets |
| `X-Image-Size` | `small`, `medium`, `large` | Preferred image size |

**Response Structure:**

```json
{
  "sections": [
    {
      "id": "search-hero",
      "type": "search-hero",
      "client_types": ["web", "ios", "android"],
      "position": 0,
      "hidden": false,
      "cache_ttl_seconds": 300,
      "data": {
        "placeholder": "What would you like to watch?",
        "suggestions": [
          {
            "id": "s1",
            "label": "Rainy day picks",
            "icon": "cloud-rain",
            "description": "Cozy films for the weather",
            "style": "accent",
            "action": {
              "type": "search",
              "query": "cozy rainy day movies"
            }
          }
        ],
        "search_url": "/api/search"
      },
      "data_url": "/api/home/sections/search-hero"
    },
    {
      "id": "featured-suggestions",
      "type": "featured-row",
      "client_types": ["roku", "firetv", "smarttv"],
      "position": 0,
      "hidden": false,
      "cache_ttl_seconds": 300,
      "data": {
        "title": "Suggested for You",
        "items": [
          {
            "entity_type": "movie",
            "entity_id": 123,
            "title": "Dune: Part Two",
            "year": 2024,
            "poster": "/api/images/movies/123/poster"
          }
        ]
      }
    },
    {
      "id": "continue-watching",
      "type": "continue-row",
      "client_types": ["all"],
      "position": 1,
      "hidden": false,
      "cache_ttl_seconds": 60,
      "data": {
        "title": "Continue Watching",
        "items": [
          {
            "entity_type": "movie",
            "entity_id": 123,
            "title": "Dune: Part Two",
            "year": 2024,
            "poster": "/api/images/movies/123/poster",
            "backdrop": "/api/images/movies/123/backdrop",
            "progress": {
              "percent": 45,
              "position_seconds": 3240,
              "duration_seconds": 7200
            }
          }
        ]
      }
    },
    {
      "id": "rec-for-you",
      "type": "media-row",
      "client_types": ["all"],
      "position": 2,
      "hidden": false,
      "cache_ttl_seconds": 600,
      "data": {
        "title": "For You",
        "subtitle": "Based on your watch history",
        "empty_state": {
          "title": "Trending",
          "subtitle": "Rate some content to get personalized recommendations"
        },
        "items": [
          {
            "entity_type": "movie",
            "entity_id": 456,
            "title": "Blade Runner 2049",
            "year": 2017,
            "poster": "/api/images/movies/456/poster",
            "reason": "Similar to Dune",
            "rating": null
          }
        ],
        "see_all_url": "/api/recommendations/for-you"
      },
      "data_url": "/api/home/sections/rec-for-you"
    }
  ],
  "preferences": {
    "can_reorder": true,
    "can_hide": true,
    "update_url": "/api/home/preferences"
  },
  "meta": {
    "generated_at": "2024-01-15T10:30:00Z",
    "user_context": {
      "has_watch_history": true,
      "has_ratings": false,
      "time_of_day": "evening",
      "season": "winter"
    }
  }
}
```

**When `inline=false`:**

Sections return metadata only; clients fetch data via `data_url`:

```json
{
  "sections": [
    {
      "id": "rec-for-you",
      "type": "media-row",
      "position": 2,
      "data_url": "/api/home/sections/rec-for-you"
    }
  ]
}
```

**HTTP Cache Headers:**

```
Cache-Control: private, max-age=60
ETag: "abc123"
Vary: Authorization, X-Client-Type, X-Image-Size
```

### Section Refresh: `GET /api/home/sections/{section_id}`

Returns a single section's data for targeted refresh without re-fetching entire home.

```json
{
  "id": "rec-for-you",
  "type": "media-row",
  "data": {
    "title": "For You",
    "items": [...]
  },
  "cache_ttl_seconds": 600
}
```

### Preferences: `/api/home/preferences`

```
GET    /api/home/preferences         # Get user's widget ordering
PUT    /api/home/preferences         # Update ordering/visibility
DELETE /api/home/preferences         # Reset to smart defaults
```

**PUT Request Body:**

```json
{
  "sections": [
    {"id": "continue-watching", "position": 0, "hidden": false},
    {"id": "rec-for-you", "position": 1, "hidden": false},
    {"id": "rec-trending", "position": 2, "hidden": true}
  ]
}
```

### Supporting Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/search` | Search (routes to active provider) |
| `GET` | `/api/search/providers` | List registered search providers |
| `GET` | `/api/search/suggestions` | Get suggestions from active provider |
| `GET` | `/api/trending` | Get trending items matched to library |
| `GET` | `/api/ratings` | List user's ratings |
| `POST` | `/api/ratings` | Create rating |
| `PUT` | `/api/ratings/{entityType}/{entityId}` | Update rating |
| `DELETE` | `/api/ratings/{entityType}/{entityId}` | Remove rating |

---

## Part 2: Widget System

### Widget Types

| Type | Description | Clients | Data Shape |
|------|-------------|---------|------------|
| `search-hero` | Search input + AI suggestion chips | web, ios, android | `{placeholder, suggestions[], search_url}` |
| `featured-row` | Featured items (search hero alternative) | roku, firetv, smarttv | `{title, items[]}` |
| `continue-row` | Continue watching with progress bars | all | `{title, items[] with progress}` |
| `media-row` | Generic horizontal media row | all | `{title, subtitle?, items[], see_all_url?}` |

### Client Type Filtering

Widgets declare which clients should render them via `client_types`:

- `["all"]` - All clients render this widget
- `["web", "ios", "android"]` - Only these clients render this widget
- `["roku", "firetv", "smarttv"]` - Alternative for constrained clients

Clients filter sections by their type. A Roku client ignores `search-hero` and renders `featured-row` instead.

### Widget Registration

Plugins register widgets via `x-viewra-widgets` in their settings schema:

```go
sdk.NewSchema("Semantic Search").
    Meta(sdk.PluginMeta{...}).
    Widgets([]sdk.Widget{
        {
            ID:                 "search-hero",
            Type:               sdk.WidgetTypeSearchHero,
            Location:           sdk.LocationHomepageTop,
            ClientTypes:        []string{"web", "ios", "android"},
            Priority:           100,
            CacheTTLSeconds:    300,
            RequiredCapability: "search_provider",
            SettingsKey:        "show_search_hero",
        },
        {
            ID:                 "featured-suggestions",
            Type:               sdk.WidgetTypeFeaturedRow,
            Location:           sdk.LocationHomepageTop,
            ClientTypes:        []string{"roku", "firetv", "smarttv"},
            Priority:           100,
            CacheTTLSeconds:    300,
            RequiredCapability: "search_provider",
            SettingsKey:        "show_search_hero",
        },
    })
```

### Widget Data Fetching

The `HomeService` calls each plugin to fetch widget data:

```go
type WidgetDataProvider interface {
    GetWidgetData(ctx context.Context, widgetID string, userID string) (map[string]any, error)
}
```

Plugins implement this via HTTP routes or direct gRPC calls.

---

## Part 3: SDK Interfaces

### SearchProvider

```go
// pkg/plugin/sdk/search_provider.go

// SearchProvider defines the interface for plugins providing search functionality.
type SearchProvider interface {
    mustEmbedBase()

    // Search performs a search with the given request
    Search(ctx context.Context, req *SearchProviderRequest) (*SearchProviderResponse, error)

    // GetSuggestions returns contextual suggestions for the search hero
    GetSuggestions(ctx context.Context, req *SuggestionRequest) (*SuggestionResponse, error)

    // GetProviderInfo returns metadata about this search provider
    GetProviderInfo(ctx context.Context) (*SearchProviderInfo, error)
}

// SearchProviderRequest contains search parameters
type SearchProviderRequest struct {
    Query       string   // Search query text
    EntityTypes []string // Filter by media types
    Limit       int      // Max results
    UserID      string   // For context enrichment
}

// SearchProviderResponse contains search results
type SearchProviderResponse struct {
    Results  []SearchResult
    Total    int
    Provider string // Which provider served this result
}

// SearchResult is a single search result
type SearchResult struct {
    EntityType string  // "movie", "tv_show", etc.
    EntityID   int64   // Database ID
    Title      string  // Display title
    Year       int     // Release year
    Poster     string  // Poster URL
    Score      float32 // Relevance score 0.0-1.0
    Reason     string  // Optional: why this matched
}

// SuggestionRequest for getting search suggestions
type SuggestionRequest struct {
    UserID  string // For personalization
    Context string // "homepage", "search_focus"
    Limit   int    // Max suggestions
}

// SuggestionResponse contains rich suggestions
type SuggestionResponse struct {
    Suggestions []Suggestion
}

// Suggestion is a rich search suggestion
type Suggestion struct {
    ID          string           // Unique identifier
    Label       string           // Display text
    Icon        string           // Icon name
    Description string           // Optional subtitle
    Style       string           // "primary", "secondary", "accent"
    Action      SuggestionAction // What happens on click
}

// SuggestionAction defines what a suggestion does
type SuggestionAction struct {
    Type   string            // "search", "filter", "navigate"
    Query  string            // For type="search"
    Filter map[string]string // For type="filter"
    URL    string            // For type="navigate"
}

// SearchProviderInfo contains provider metadata
type SearchProviderInfo struct {
    ID           string   // Provider identifier
    Name         string   // Display name
    Description  string   // What this provider does
    Icon         string   // Icon name
    Priority     int      // Higher = preferred
    Capabilities []string // "natural_language", "suggestions", "context_aware"
}
```

### TrendingProvider

```go
// pkg/plugin/sdk/trending.go

// TrendingProvider provides trending/popular content data.
type TrendingProvider interface {
    mustEmbedBase()

    // GetTrending returns currently trending items
    GetTrending(ctx context.Context, req *TrendingRequest) (*TrendingResponse, error)

    // GetProviderInfo returns metadata about this trending source
    GetProviderInfo(ctx context.Context) (*TrendingProviderInfo, error)
}

// TrendingRequest parameters
type TrendingRequest struct {
    MediaType string // "movie", "tv", "all"
    Window    string // "day", "week"
    Limit     int    // Max results
    Region    string // ISO 3166-1 country code
}

// TrendingResponse results
type TrendingResponse struct {
    Items    []TrendingItem
    Window   string // Time window used
    Source   string // "tmdb", "trakt", etc.
    CachedAt int64  // Unix timestamp when fetched
}

// TrendingItem is a single trending item
type TrendingItem struct {
    ExternalID   string  // e.g., "tmdb:12345"
    MediaType    string  // "movie" or "tv"
    Title        string  // Display title
    Year         int     // Release year
    Popularity   float32 // Provider-specific score
    PosterPath   string  // External poster URL
    Overview     string  // Description
    LocalID      *int64  // Matched local library ID (filled by consumer)
    LocalMatched bool    // Whether matched to local library
}

// TrendingProviderInfo metadata
type TrendingProviderInfo struct {
    ID          string   // "tmdb", "trakt"
    Name        string   // "TMDb Trending"
    Description string   // What this provides
    Windows     []string // Supported time windows
    MediaTypes  []string // Supported media types
    UpdateFreq  string   // "hourly", "daily"
}
```

### Widget Types

```go
// pkg/plugin/sdk/widgets.go

// Widget defines a UI section a plugin provides for the home screen.
type Widget struct {
    ID                 string         // Unique widget ID
    Type               string         // Widget type
    Location           string         // Where to render
    ClientTypes        []string       // Which clients render this
    Priority           int            // Order (higher = first)
    CacheTTLSeconds    int            // Cache duration
    Config             map[string]any // Widget-specific config
    RequiredCapability string         // Only show if available
    SettingsKey        string         // Settings key for enabled state
}

// Widget locations
const (
    LocationHomepageTop      = "homepage-top"
    LocationHomepageSections = "homepage-sections"
)

// Widget types
const (
    WidgetTypeSearchHero  = "search-hero"
    WidgetTypeFeaturedRow = "featured-row"
    WidgetTypeContinueRow = "continue-row"
    WidgetTypeMediaRow    = "media-row"
)

// Client types
const (
    ClientTypeAll     = "all"
    ClientTypeWeb     = "web"
    ClientTypeIOS     = "ios"
    ClientTypeAndroid = "android"
    ClientTypeRoku    = "roku"
    ClientTypeFireTV  = "firetv"
    ClientTypeSmartTV = "smarttv"
)
```

---

## Part 4: Core Services

### HomeService

Aggregates widgets from all plugins and builds the home response.

```go
// internal/application/home/service.go

type HomeService struct {
    widgetRegistry     *registry.WidgetRegistry
    preferencesRepo    home.PreferencesRepository
    searchProviders    *registry.SearchProviderRegistry
    trendingProviders  *registry.TrendingProviderRegistry
    continueWatching   *progress.ContinueWatchingService
    pluginProxy        *proxy.HTTPProxy
    events             events.Publisher
    logger             *slog.Logger
}

type HomeRequest struct {
    UserID     string
    ClientType string
    Inline     bool
    Sections   []string // Specific sections or empty for all
    ImageSize  string
}

type HomeResponse struct {
    Sections    []*Section           `json:"sections"`
    Preferences *PreferencesInfo     `json:"preferences"`
    Meta        *HomeMeta            `json:"meta"`
}

func (s *HomeService) GetHome(ctx context.Context, req *HomeRequest) (*HomeResponse, error) {
    // 1. Get all registered widgets
    widgets := s.widgetRegistry.GetAll()

    // 2. Filter by client type
    if req.ClientType != "" {
        widgets = filterByClientType(widgets, req.ClientType)
    }

    // 3. Get user preferences
    prefs, _ := s.preferencesRepo.Get(ctx, req.UserID)

    // 4. Apply preferences (ordering, visibility)
    widgets = applyPreferences(widgets, prefs)

    // 5. If no preferences, apply smart defaults
    if prefs == nil {
        widgets = s.applySmartDefaults(ctx, req.UserID, widgets)
    }

    // 6. Fetch data for each widget (parallel)
    sections := s.fetchWidgetData(ctx, widgets, req)

    // 7. Build response
    return &HomeResponse{
        Sections:    sections,
        Preferences: &PreferencesInfo{CanReorder: true, CanHide: true},
        Meta:        s.buildMeta(ctx, req.UserID),
    }, nil
}

func (s *HomeService) fetchWidgetData(ctx context.Context, widgets []*Widget, req *HomeRequest) []*Section {
    sections := make([]*Section, 0, len(widgets))
    var mu sync.Mutex
    var wg sync.WaitGroup

    for _, w := range widgets {
        wg.Add(1)
        go func(widget *Widget) {
            defer wg.Done()

            data, err := s.getWidgetData(ctx, widget, req)
            if err != nil {
                // Log error for admin visibility
                s.logger.Error("widget data fetch failed",
                    "widget_id", widget.ID,
                    "plugin_id", widget.PluginID,
                    "error", err,
                )
                // Emit event for monitoring
                s.events.Emit(events.WidgetError{
                    WidgetID:  widget.ID,
                    PluginID:  widget.PluginID,
                    Error:     err.Error(),
                    Timestamp: time.Now(),
                })
                // Skip this widget silently
                return
            }

            mu.Lock()
            sections = append(sections, &Section{
                ID:              widget.ID,
                Type:            widget.Type,
                ClientTypes:     widget.ClientTypes,
                Position:        widget.Position,
                Hidden:          widget.Hidden,
                CacheTTLSeconds: widget.CacheTTLSeconds,
                Data:            data,
                DataURL:         fmt.Sprintf("/api/home/sections/%s", widget.ID),
            })
            mu.Unlock()
        }(w)
    }

    wg.Wait()

    // Sort by position
    sort.Slice(sections, func(i, j int) bool {
        return sections[i].Position < sections[j].Position
    })

    return sections
}
```

### Smart Default Ordering

New users without preferences get intelligent ordering based on available data:

```go
func (s *HomeService) applySmartDefaults(ctx context.Context, userID string, widgets []*Widget) []*Widget {
    hasWatchHistory := s.continueWatching.HasHistory(ctx, userID)
    hasRatings := s.ratingsRepo.HasRatings(ctx, userID)
    hasWeather := s.weatherService.IsAvailable(ctx, userID)

    for _, w := range widgets {
        switch w.ID {
        case "continue-watching":
            if hasWatchHistory {
                w.Priority += 50 // Boost to top
            } else {
                w.Hidden = true // Hide if empty
            }

        case "rec-for-you":
            if hasRatings || hasWatchHistory {
                w.Priority += 20 // Boost if we have personalization data
            }

        case "rec-contextual":
            if hasWeather {
                w.Priority += 10
            } else {
                w.Hidden = true // Hide if no weather data
            }

        case "rec-trending":
            if !hasRatings && !hasWatchHistory {
                w.Priority += 30 // Boost for new users
            }
        }
    }

    // Sort by adjusted priority
    sort.Slice(widgets, func(i, j int) bool {
        return widgets[i].Priority > widgets[j].Priority
    })

    // Assign positions
    position := 0
    for _, w := range widgets {
        if !w.Hidden {
            w.Position = position
            position++
        }
    }

    return widgets
}
```

### TrendingService

Aggregates trending data and matches against local library:

```go
// internal/application/trending/service.go

type TrendingService struct {
    providerRegistry *registry.TrendingRegistry
    mediaRepo        media.Repository
    cache            cache.Cache
    logger           *slog.Logger
}

func (s *TrendingService) GetTrending(ctx context.Context, mediaType string, limit int) (*TrendingResult, error) {
    // 1. Get trending from best available provider
    provider := s.providerRegistry.GetDefault()
    if provider == nil {
        return nil, ErrNoTrendingProvider
    }

    // 2. Check cache
    cacheKey := fmt.Sprintf("trending:%s:%s", provider.ID, mediaType)
    if cached, ok := s.cache.Get(cacheKey); ok {
        return s.matchToLibrary(ctx, cached.(*TrendingResponse), limit)
    }

    // 3. Fetch from provider
    trending, err := provider.GetTrending(ctx, &sdk.TrendingRequest{
        MediaType: mediaType,
        Window:    "week",
        Limit:     limit * 3, // Get extra to account for non-matches
    })
    if err != nil {
        return nil, fmt.Errorf("fetch trending: %w", err)
    }

    // 4. Cache for 1 hour
    s.cache.Set(cacheKey, trending, time.Hour)

    // 5. Match against local library
    return s.matchToLibrary(ctx, trending, limit)
}

func (s *TrendingService) matchToLibrary(ctx context.Context, trending *sdk.TrendingResponse, limit int) (*TrendingResult, error) {
    matched := make([]TrendingItem, 0)

    for _, item := range trending.Items {
        // Parse external ID (e.g., "tmdb:12345")
        parts := strings.SplitN(item.ExternalID, ":", 2)
        if len(parts) != 2 {
            continue
        }
        source, externalID := parts[0], parts[1]

        // Look up in local library
        localMedia, err := s.mediaRepo.FindByExternalID(ctx, source, externalID, item.MediaType)
        if err != nil || localMedia == nil {
            continue // Not in library
        }

        matched = append(matched, TrendingItem{
            TrendingItem: item,
            LocalID:      &localMedia.ID,
            LocalMatched: true,
        })

        if len(matched) >= limit {
            break
        }
    }

    return &TrendingResult{
        Items:        matched,
        Source:       trending.Source,
        Window:       trending.Window,
        TotalMatched: len(matched),
    }, nil
}
```

---

## Part 5: Database Schema

### Core: Widget Preferences

```sql
-- migrations/postgres/000XXX_widget_preferences.up.sql
-- migrations/000XXX_widget_preferences.up.sql (SQLite)

CREATE TABLE widget_preferences (
    id INTEGER PRIMARY KEY,
    user_id TEXT NOT NULL,
    widget_id TEXT NOT NULL,
    location TEXT NOT NULL,
    position INTEGER NOT NULL,
    hidden BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, widget_id)
);

CREATE INDEX idx_widget_prefs_user_location ON widget_preferences(user_id, location);
```

### Recommendations Plugin: User Ratings

```sql
-- Plugin migration (auto-prefixed with plugin_recommendations_)

CREATE TABLE ratings (
    id INTEGER PRIMARY KEY,
    user_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id INTEGER NOT NULL,
    rating TEXT NOT NULL CHECK(rating IN ('up', 'down', 'favorite')),
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, entity_type, entity_id)
);

CREATE INDEX idx_ratings_user ON ratings(user_id);
CREATE INDEX idx_ratings_entity ON ratings(entity_type, entity_id);
CREATE INDEX idx_ratings_user_rating ON ratings(user_id, rating);
```

---

## Part 6: Recommendations Plugin

### Plugin Structure

```
plugins/recommendations/
├── internal/
│   ├── plugin.go           # Main plugin, implements HTTP handlers
│   ├── schema.go           # Settings schema with widgets
│   ├── types.go            # Domain types
│   ├── ratings.go          # User ratings CRUD
│   ├── recommendations.go  # Recommendation engine
│   ├── popularity.go       # Popularity service (uses trending)
│   └── strategies/
│       ├── interface.go    # Strategy interface
│       ├── for_you.go      # Personalized recommendations
│       ├── similar.go      # "Because you watched X"
│       ├── contextual.go   # Weather/time-based
│       ├── trending.go     # Uses trending capability
│       └── baseline.go     # Recent, top-rated fallbacks
├── config.yml
├── go.mod
├── main.go
└── plugin.yml
```

### Plugin Manifest

```yaml
# plugins/recommendations/plugin.yml
id: recommendations
name: Recommendations
version: 1.0.0
description: AI-powered personalized recommendations with user ratings

provides:
  - recommendations

requires: []

optional:
  - semantic_search  # For AI-powered recommendations
  - trending         # For trending-based recommendations

permissions:
  - storage:sql      # For user ratings
  - data:read        # For media access
  - weather:read     # For contextual recommendations
```

### Settings Schema

```go
func SettingsSchema() *sdk.Schema {
    return sdk.NewSchema("Recommendations Settings").
        Meta(sdk.PluginMeta{
            DisplayName: "Recommendations",
            Description: "Personalized content recommendations",
            Icon:        "sparkles",
        }).
        Property("enabled", sdk.Boolean().
            Title("Enable Recommendations").
            Default(true)).
        Property("show_reasons", sdk.Boolean().
            Title("Show Recommendation Reasons").
            Description("Display short phrases explaining recommendations").
            Default(true)).
        Property("contextual_enabled", sdk.Boolean().
            Title("Contextual Recommendations").
            Description("Adjust based on time, weather, and season").
            Default(true).
            DependsOn("enabled")).
        Widgets([]sdk.Widget{
            {
                ID:              "rec-for-you",
                Type:            sdk.WidgetTypeMediaRow,
                Location:        sdk.LocationHomepageSections,
                ClientTypes:     []string{sdk.ClientTypeAll},
                Priority:        90,
                CacheTTLSeconds: 600,
                Config: map[string]any{
                    "endpoint": "/for-you",
                    "title":    "For You",
                },
                SettingsKey: "enabled",
            },
            {
                ID:              "rec-similar",
                Type:            sdk.WidgetTypeMediaRow,
                Location:        sdk.LocationHomepageSections,
                ClientTypes:     []string{sdk.ClientTypeAll},
                Priority:        80,
                CacheTTLSeconds: 600,
                Config: map[string]any{
                    "endpoint": "/similar",
                    "title":    "Because You Watched",
                },
                SettingsKey: "enabled",
            },
            {
                ID:              "rec-contextual",
                Type:            sdk.WidgetTypeMediaRow,
                Location:        sdk.LocationHomepageSections,
                ClientTypes:     []string{sdk.ClientTypeAll},
                Priority:        70,
                CacheTTLSeconds: 300,
                Config: map[string]any{
                    "endpoint": "/contextual",
                },
                SettingsKey: "contextual_enabled",
            },
            {
                ID:              "rec-trending",
                Type:            sdk.WidgetTypeMediaRow,
                Location:        sdk.LocationHomepageSections,
                ClientTypes:     []string{sdk.ClientTypeAll},
                Priority:        60,
                CacheTTLSeconds: 3600,
                Config: map[string]any{
                    "endpoint": "/trending",
                    "title":    "Trending",
                },
                SettingsKey: "enabled",
            },
        })
}
```

### Recommendation Strategies

```go
// plugins/recommendations/internal/strategies/interface.go

// Strategy generates recommendations for a user.
type Strategy interface {
    // Name returns the strategy identifier
    Name() string

    // Generate produces recommendations
    Generate(ctx context.Context, req *StrategyRequest) (*StrategyResponse, error)

    // IsAvailable returns whether this strategy can run
    IsAvailable(ctx context.Context) bool

    // Priority for ordering (higher = run first)
    Priority() int
}

type StrategyRequest struct {
    UserID  string
    Limit   int
    Context *RecommendationContext
}

type RecommendationContext struct {
    TimeOfDay string // "morning", "afternoon", "evening", "night"
    DayOfWeek string // "monday", "tuesday", ...
    Season    string // "spring", "summer", "fall", "winter"
    Weather   *WeatherContext
    Holidays  []string
}

type WeatherContext struct {
    Condition   string  // "sunny", "cloudy", "rainy", "snowy"
    Temperature float32 // Celsius
}

type StrategyResponse struct {
    Recommendations []Recommendation
    RowTitle        string // Can be dynamic based on context
    RowSubtitle     string
}
```

### Strategy Implementations

| Strategy | Description | Dependencies | Fallback |
|----------|-------------|--------------|----------|
| `ForYouStrategy` | Personalized blend | Ratings + semantic-search | → TrendingStrategy |
| `SimilarStrategy` | "Because you watched X" | Watch history + semantic-search | → Skip |
| `ContextualStrategy` | Weather/time-appropriate | Weather service | → Skip |
| `TrendingStrategy` | Popular items | Trending provider | → RecentStrategy |
| `RecentStrategy` | Recently added | None | Always available |
| `TopRatedStrategy` | Highest community rated | Ratings data | → RecentStrategy |

### Graceful Degradation

```go
func (e *RecommendationEngine) GetForYou(ctx context.Context, userID string, limit int) (*RecommendationRow, error) {
    // Try AI-powered first
    if e.isSemanticSearchAvailable(ctx) && e.hasUserData(ctx, userID) {
        return e.strategies.ForYou.Generate(ctx, userID, limit)
    }

    // Fall back to ratings-based
    if e.hasUserRatings(ctx, userID) {
        return e.strategies.RatingsBased.Generate(ctx, userID, limit)
    }

    // Fall back to trending
    if e.isTrendingAvailable(ctx) {
        row, err := e.strategies.Trending.Generate(ctx, userID, limit)
        if err == nil {
            row.Title = "Trending"
            row.Subtitle = "Rate some content to get personalized recommendations"
            return row, nil
        }
    }

    // Ultimate fallback: recently added
    return e.strategies.Recent.Generate(ctx, userID, limit)
}
```

---

## Part 7: TMDb Trending Integration

### Plugin Updates

```yaml
# plugins/tmdb/plugin.yml (updated)
provides:
  - enricher
  - trending  # NEW
```

### Implementation

```go
// plugins/tmdb/internal/trending.go

func (p *TMDbPlugin) GetTrending(ctx context.Context, req *sdk.TrendingRequest) (*sdk.TrendingResponse, error) {
    mediaType := req.MediaType
    if mediaType == "" || mediaType == "all" {
        mediaType = "all"
    }

    window := req.Window
    if window == "" {
        window = "week"
    }

    // Call TMDb API: GET /trending/{media_type}/{time_window}
    resp, err := p.client.GetTrending(ctx, mediaType, window)
    if err != nil {
        return nil, fmt.Errorf("tmdb trending: %w", err)
    }

    items := make([]sdk.TrendingItem, 0, len(resp.Results))
    for _, r := range resp.Results {
        mt := r.MediaType
        if mt == "" {
            mt = mediaType // Single type request
        }

        items = append(items, sdk.TrendingItem{
            ExternalID:  fmt.Sprintf("tmdb:%d", r.ID),
            MediaType:   mt,
            Title:       coalesceTitles(r.Title, r.Name),
            Year:        extractYear(r.ReleaseDate, r.FirstAirDate),
            Popularity:  r.Popularity,
            PosterPath:  p.buildPosterURL(r.PosterPath),
            Overview:    r.Overview,
        })
    }

    return &sdk.TrendingResponse{
        Items:    items,
        Window:   window,
        Source:   "tmdb",
        CachedAt: time.Now().Unix(),
    }, nil
}

func (p *TMDbPlugin) GetTrendingProviderInfo(ctx context.Context) (*sdk.TrendingProviderInfo, error) {
    return &sdk.TrendingProviderInfo{
        ID:          "tmdb",
        Name:        "TMDb Trending",
        Description: "Trending movies and TV shows from The Movie Database",
        Windows:     []string{"day", "week"},
        MediaTypes:  []string{"movie", "tv", "all"},
        UpdateFreq:  "daily",
    }, nil
}
```

---

## Part 8: semantic-search Updates

### Plugin Updates

```yaml
# plugins/semantic-search/plugin.yml (updated)
provides:
  - semantic_search
  - search_provider  # NEW
```

### SearchProvider Implementation

```go
// plugins/semantic-search/internal/suggestions.go

func (p *SemanticSearchPlugin) GetSuggestions(ctx context.Context, req *sdk.SuggestionRequest) (*sdk.SuggestionResponse, error) {
    suggestions := make([]sdk.Suggestion, 0)

    // Get user context
    qc, _ := p.contextEnricher.GetContext(ctx, req.UserID)

    // Weather-based suggestion
    if qc != nil && qc.Weather != nil && qc.Weather.Available {
        suggestion := p.getWeatherSuggestion(qc)
        if suggestion != nil {
            suggestions = append(suggestions, *suggestion)
        }
    }

    // Time-based suggestion
    if qc != nil {
        suggestion := p.getTimeSuggestion(qc)
        if suggestion != nil {
            suggestions = append(suggestions, *suggestion)
        }
    }

    // Holiday suggestion
    if qc != nil && len(qc.Holidays) > 0 {
        suggestion := p.getHolidaySuggestion(qc.Holidays[0])
        if suggestion != nil {
            suggestions = append(suggestions, *suggestion)
        }
    }

    // Genre suggestions (static)
    suggestions = append(suggestions, []sdk.Suggestion{
        {ID: "action", Label: "Action", Icon: "zap", Action: sdk.SuggestionAction{Type: "search", Query: "action movies"}},
        {ID: "comedy", Label: "Comedy", Icon: "smile", Action: sdk.SuggestionAction{Type: "search", Query: "comedy"}},
        {ID: "documentary", Label: "Documentary", Icon: "film", Action: sdk.SuggestionAction{Type: "search", Query: "documentary"}},
    }...)

    // Limit
    limit := req.Limit
    if limit <= 0 {
        limit = 6
    }
    if len(suggestions) > limit {
        suggestions = suggestions[:limit]
    }

    return &sdk.SuggestionResponse{Suggestions: suggestions}, nil
}

func (p *SemanticSearchPlugin) getWeatherSuggestion(qc *QueryContext) *sdk.Suggestion {
    if qc.Weather == nil || !qc.Weather.Available {
        return nil
    }

    switch qc.Weather.Condition {
    case "rainy", "stormy":
        return &sdk.Suggestion{
            ID:          "weather-rainy",
            Label:       "Rainy day picks",
            Icon:        "cloud-rain",
            Description: "Cozy films for the weather",
            Style:       "accent",
            Action:      sdk.SuggestionAction{Type: "search", Query: "cozy rainy day movies"},
        }
    case "sunny", "clear":
        return &sdk.Suggestion{
            ID:          "weather-sunny",
            Label:       "Feel-good films",
            Icon:        "sun",
            Description: "Bright movies for a sunny day",
            Style:       "primary",
            Action:      sdk.SuggestionAction{Type: "search", Query: "uplifting feel-good movies"},
        }
    case "snowy":
        return &sdk.Suggestion{
            ID:          "weather-snowy",
            Label:       "Winter favorites",
            Icon:        "snowflake",
            Description: "Perfect for a snowy day",
            Style:       "accent",
            Action:      sdk.SuggestionAction{Type: "search", Query: "winter snow movies"},
        }
    }
    return nil
}

func (p *SemanticSearchPlugin) getTimeSuggestion(qc *QueryContext) *sdk.Suggestion {
    switch qc.TimeOfDay {
    case "morning":
        return &sdk.Suggestion{
            ID:          "time-morning",
            Label:       "Morning watch",
            Icon:        "sunrise",
            Action:      sdk.SuggestionAction{Type: "search", Query: "light morning movies"},
        }
    case "evening", "night":
        return &sdk.Suggestion{
            ID:          "time-evening",
            Label:       "Evening picks",
            Icon:        "moon",
            Action:      sdk.SuggestionAction{Type: "search", Query: "evening movies to unwind"},
        }
    }
    return nil
}
```

### Widget Registration

```go
// In schema.go
Widgets([]sdk.Widget{
    {
        ID:                 "search-hero",
        Type:               sdk.WidgetTypeSearchHero,
        Location:           sdk.LocationHomepageTop,
        ClientTypes:        []string{sdk.ClientTypeWeb, sdk.ClientTypeIOS, sdk.ClientTypeAndroid},
        Priority:           100,
        CacheTTLSeconds:    300,
        RequiredCapability: "search_provider",
        SettingsKey:        "show_search_hero",
        Config: map[string]any{
            "placeholder":      "What would you like to watch?",
            "show_suggestions": true,
        },
    },
    {
        ID:                 "featured-suggestions",
        Type:               sdk.WidgetTypeFeaturedRow,
        Location:           sdk.LocationHomepageTop,
        ClientTypes:        []string{sdk.ClientTypeRoku, sdk.ClientTypeFireTV, sdk.ClientTypeSmartTV},
        Priority:           100,
        CacheTTLSeconds:    300,
        RequiredCapability: "search_provider",
        SettingsKey:        "show_search_hero",
        Config: map[string]any{
            "title":    "Suggested for You",
            "endpoint": "/suggestions",
        },
    },
})
```

---

## Part 9: Frontend Implementation

### Component Structure

```
web/src/
├── routes/_layout/
│   └── index.tsx                    # Home page with WidgetSlot
├── components/
│   ├── home/
│   │   ├── WidgetSlot.tsx           # Renders widgets for a location
│   │   ├── WidgetRenderer.tsx       # Maps type to component
│   │   ├── CustomizeMode.tsx        # Widget reordering UI
│   │   └── DraggableWidgetList.tsx  # Drag-drop list
│   ├── widgets/
│   │   ├── SearchHeroWidget.tsx     # Search + suggestions
│   │   ├── MediaRowWidget.tsx       # Generic media row
│   │   ├── ContinueRowWidget.tsx    # Continue watching
│   │   └── SuggestionChip.tsx       # Clickable suggestion
│   └── media/
│       ├── RecommendationCard.tsx   # Card with rating actions
│       └── RatingButtons.tsx        # Thumbs up/down/favorite
└── hooks/
    ├── useHome.ts                   # Home data fetching
    ├── useWidgetPreferences.ts      # Preferences management
    └── useRatings.ts                # Rating mutations
```

### Home Page

```typescript
// web/src/routes/_layout/index.tsx

const Index = () => {
  const { data, isLoading } = useHome()
  const [customizeMode, setCustomizeMode] = useState(false)
  const { mutate: updatePreferences } = useUpdateWidgetPreferences()

  if (isLoading) return <HomeSkeleton />

  const topSections = data?.sections.filter(s => 
    s.position === 0 && matchesClientType(s.client_types)
  ) ?? []

  const mainSections = data?.sections.filter(s => 
    s.position > 0 && !s.hidden && matchesClientType(s.client_types)
  ) ?? []

  return (
    <div className="h-full overflow-auto">
      <div className="page-enter">
        {/* Top widgets (search hero) */}
        {topSections.map(section => (
          <WidgetRenderer key={section.id} section={section} />
        ))}

        <div className="p-8 pt-6 space-y-10">
          {/* Header with customize button */}
          <div className="flex justify-between items-center">
            <h1 className={text.heading.lg}>Home</h1>
            {data?.preferences.can_reorder && (
              <Button 
                variant="ghost" 
                onClick={() => setCustomizeMode(!customizeMode)}
              >
                {customizeMode ? 'Done' : 'Customize'}
              </Button>
            )}
          </div>

          {/* Customize mode or normal view */}
          {customizeMode ? (
            <CustomizeMode
              sections={data?.sections ?? []}
              onReorder={(sections) => updatePreferences({ sections })}
              onClose={() => setCustomizeMode(false)}
            />
          ) : (
            mainSections.map(section => (
              <WidgetRenderer key={section.id} section={section} />
            ))
          )}
        </div>
      </div>
    </div>
  )
}
```

### Widget Renderer

```typescript
// web/src/components/home/WidgetRenderer.tsx

const widgetComponents: Record<string, React.ComponentType<WidgetProps>> = {
  'search-hero': SearchHeroWidget,
  'featured-row': MediaRowWidget,
  'continue-row': ContinueRowWidget,
  'media-row': MediaRowWidget,
}

type WidgetRendererProps = {
  section: Section
}

const WidgetRenderer = ({ section }: WidgetRendererProps) => {
  const Component = widgetComponents[section.type]
  
  if (!Component) {
    console.warn(`Unknown widget type: ${section.type}`)
    return null
  }

  return <Component section={section} />
}
```

### Customize Mode

```typescript
// web/src/components/home/CustomizeMode.tsx

type CustomizeModeProps = {
  sections: Section[]
  onReorder: (sections: SectionPreference[]) => void
  onClose: () => void
}

const CustomizeMode = ({ sections, onReorder, onClose }: CustomizeModeProps) => {
  const [items, setItems] = useState(
    sections
      .filter(s => s.position > 0) // Exclude top widgets
      .sort((a, b) => a.position - b.position)
  )

  const handleDragEnd = (result: DropResult) => {
    if (!result.destination) return

    const reordered = Array.from(items)
    const [removed] = reordered.splice(result.source.index, 1)
    reordered.splice(result.destination.index, 0, removed)

    setItems(reordered)
  }

  const handleToggle = (id: string) => {
    setItems(items.map(item => 
      item.id === id ? { ...item, hidden: !item.hidden } : item
    ))
  }

  const handleSave = () => {
    onReorder(items.map((item, index) => ({
      id: item.id,
      position: index + 1,
      hidden: item.hidden,
    })))
    onClose()
  }

  const handleReset = () => {
    // API call to delete preferences
    onClose()
  }

  return (
    <div className={cn(glass.card, 'p-6')}>
      <div className="flex justify-between items-center mb-4">
        <div>
          <h2 className={text.heading.md}>Customize Your Home</h2>
          <p className={text.muted}>Drag to reorder, toggle to show/hide</p>
        </div>
        <div className="flex gap-2">
          <Button variant="ghost" onClick={handleReset}>Reset</Button>
          <Button onClick={handleSave}>Save</Button>
        </div>
      </div>

      <DragDropContext onDragEnd={handleDragEnd}>
        <Droppable droppableId="widgets">
          {(provided) => (
            <div ref={provided.innerRef} {...provided.droppableProps}>
              {items.map((item, index) => (
                <Draggable key={item.id} draggableId={item.id} index={index}>
                  {(provided, snapshot) => (
                    <div
                      ref={provided.innerRef}
                      {...provided.draggableProps}
                      className={cn(
                        'flex items-center gap-4 p-4 rounded-lg mb-2',
                        bg.secondary,
                        item.hidden && 'opacity-50',
                        snapshot.isDragging && 'shadow-lg'
                      )}
                    >
                      <div {...provided.dragHandleProps}>
                        <GripVertical className="w-5 h-5 text-muted" />
                      </div>
                      <div className="flex-1">
                        <span className={item.hidden ? text.muted : ''}>
                          {item.data?.title ?? item.id}
                        </span>
                      </div>
                      <Switch
                        checked={!item.hidden}
                        onCheckedChange={() => handleToggle(item.id)}
                      />
                    </div>
                  )}
                </Draggable>
              ))}
              {provided.placeholder}
            </div>
          )}
        </Droppable>
      </DragDropContext>
    </div>
  )
}
```

---

## Part 10: Error Handling

### Widget Failures

When a plugin fails to provide widget data, the widget is silently omitted from the response. Errors are logged and emitted as events for admin visibility.

```go
// In HomeService.fetchWidgetData()
if err != nil {
    s.logger.Error("widget data fetch failed",
        "widget_id", widget.ID,
        "plugin_id", widget.PluginID,
        "error", err,
    )
    s.events.Emit(events.WidgetError{
        WidgetID:  widget.ID,
        PluginID:  widget.PluginID,
        Error:     err.Error(),
        Timestamp: time.Now(),
    })
    return // Skip this widget
}
```

### Admin Monitoring

Admins can monitor widget health via:

1. **Logs** - Standard structured logging
2. **Plugin health endpoint** - `GET /api/plugins/{id}/health`
3. **Events stream** - SSE endpoint for real-time monitoring (if implemented)

---

## Part 11: File Changes Summary

### SDK (pkg/plugin/sdk/)

| File | Action | Description |
|------|--------|-------------|
| `search_provider.go` | CREATE | SearchProvider interface |
| `trending.go` | CREATE | TrendingProvider interface |
| `widgets.go` | CREATE | Widget types and constants |
| `schema.go` | MODIFY | Add `Widgets()` method |
| `serve.go` | MODIFY | Add serve helpers for new interfaces |

### Protobuf (api/proto/plugin/)

| File | Action | Description |
|------|--------|-------------|
| `search_provider.proto` | CREATE | gRPC definitions |
| `trending.proto` | CREATE | gRPC definitions |

### Core Backend (internal/)

| File | Action | Description |
|------|--------|-------------|
| `infrastructure/plugins/registry/search_provider.go` | CREATE | Search provider registry |
| `infrastructure/plugins/registry/trending.go` | CREATE | Trending provider registry |
| `infrastructure/plugins/registry/widget.go` | CREATE | Widget registry |
| `application/home/service.go` | CREATE | Home aggregation service |
| `application/home/preferences.go` | CREATE | Preferences service |
| `application/trending/service.go` | CREATE | Trending aggregation |
| `api/handlers/home.go` | CREATE | Home API handler |
| `api/routes/home.go` | CREATE | Home routes |

### Migrations

| File | Action | Description |
|------|--------|-------------|
| `migrations/postgres/000XXX_widget_preferences.up.sql` | CREATE | Preferences table |
| `migrations/000XXX_widget_preferences.up.sql` | CREATE | SQLite version |

### Plugins

| File | Action | Description |
|------|--------|-------------|
| `plugins/tmdb/plugin.yml` | MODIFY | Add `trending` capability |
| `plugins/tmdb/internal/plugin.go` | MODIFY | Implement TrendingProvider |
| `plugins/tmdb/internal/trending.go` | CREATE | Trending implementation |
| `plugins/semantic-search/plugin.yml` | MODIFY | Add `search_provider` capability |
| `plugins/semantic-search/internal/plugin.go` | MODIFY | Implement SearchProvider |
| `plugins/semantic-search/internal/suggestions.go` | CREATE | Suggestion generation |
| `plugins/semantic-search/internal/schema.go` | MODIFY | Add widgets |
| `plugins/recommendations/` | CREATE | Entire new plugin |

### Frontend (web/src/)

| File | Action | Description |
|------|--------|-------------|
| `routes/_layout/index.tsx` | MODIFY | Integrate widget system |
| `components/home/WidgetSlot.tsx` | CREATE | Widget slot |
| `components/home/WidgetRenderer.tsx` | CREATE | Widget renderer |
| `components/home/CustomizeMode.tsx` | CREATE | Customization UI |
| `components/home/DraggableWidgetList.tsx` | CREATE | Drag-drop list |
| `components/widgets/SearchHeroWidget.tsx` | CREATE | Search hero |
| `components/widgets/MediaRowWidget.tsx` | CREATE | Media row |
| `components/widgets/ContinueRowWidget.tsx` | CREATE | Continue watching |
| `components/widgets/SuggestionChip.tsx` | CREATE | Suggestion chip |
| `components/media/RecommendationCard.tsx` | CREATE | Recommendation card |
| `components/media/RatingButtons.tsx` | CREATE | Rating buttons |
| `hooks/useHome.ts` | CREATE | Home data hook |
| `hooks/useWidgetPreferences.ts` | CREATE | Preferences hook |
| `hooks/useRatings.ts` | CREATE | Ratings hook |

---

## Part 12: Implementation Phases

### Phase 1: SDK & Core Infrastructure (2-3 days)

1. Create SDK interfaces (`search_provider.go`, `trending.go`, `widgets.go`)
2. Update `schema.go` with `Widgets()` method
3. Create protobuf definitions
4. Implement registries (search provider, trending, widget)
5. Create `HomeService` and `TrendingService`
6. Add API handlers and routes
7. Create database migration for widget preferences

### Phase 2: TMDb Trending Integration (0.5 days)

1. Update `plugins/tmdb/plugin.yml`
2. Implement `TrendingProvider` in TMDb plugin
3. Add `/trending` route to TMDb plugin

### Phase 3: semantic-search Updates (1 day)

1. Update `plugins/semantic-search/plugin.yml`
2. Implement `SearchProvider` interface
3. Create `suggestions.go` with context-aware suggestions
4. Add widgets to settings schema

### Phase 4: Frontend Widget System (2-3 days)

1. Create `useHome` hook
2. Implement `WidgetSlot` and `WidgetRenderer`
3. Create widget components (SearchHero, MediaRow, ContinueRow)
4. Implement `CustomizeMode` with drag-drop
5. Update homepage to use widget system

### Phase 5: Recommendations Plugin (3-4 days)

1. Create plugin scaffold
2. Implement user ratings (storage + API)
3. Implement recommendation strategies
4. Add graceful degradation
5. Register widgets

### Phase 6: Frontend Recommendations (1-2 days)

1. Create `RecommendationCard` component
2. Implement `RatingButtons`
3. Create `useRatings` hook
4. Integrate with media cards

### Phase 7: Smart Defaults & Polish (1 day)

1. Implement smart default ordering
2. Add empty states
3. Error handling polish
4. Integration tests

---

## Part 13: Testing Checklist

### API Tests

- [ ] `GET /api/home` returns sections with data inline
- [ ] `GET /api/home?inline=false` returns sections with data URLs
- [ ] `GET /api/home` with `X-Client-Type: roku` filters widgets
- [ ] `GET /api/home/sections/{id}` returns single section
- [ ] `GET /api/home/preferences` returns user preferences
- [ ] `PUT /api/home/preferences` updates ordering
- [ ] `DELETE /api/home/preferences` resets to defaults
- [ ] `GET /api/trending` returns matched items
- [ ] `GET /api/search/suggestions` returns suggestions

### Plugin Tests

- [ ] TMDb plugin provides trending data
- [ ] semantic-search plugin provides suggestions
- [ ] Recommendations plugin serves recommendation rows
- [ ] Widget registration works correctly
- [ ] Failed plugins are silently omitted

### Frontend Tests

- [ ] Homepage renders widgets correctly
- [ ] Customize mode allows reordering
- [ ] Hidden widgets don't appear
- [ ] Search hero shows suggestions
- [ ] Rating buttons work
- [ ] Empty states display correctly

---

## Part 14: Future Considerations

### Real-Time Updates

Future iteration could add:
- WebSocket/SSE for web clients
- Push notifications for iOS/Android
- Polling for constrained clients (Roku)

### Additional Trending Providers

- Trakt integration
- Internal play count analytics
- Community ratings aggregation

### Machine Learning

- Collaborative filtering
- Watch pattern analysis
- Personalized ranking models

### Widget Marketplace

- Third-party widget plugins
- Custom widget types
- Widget theming

---

## Decisions Log

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Search hero fallback | No hero when disabled | Cleaner UX, use navbar search |
| Widget ordering | User-reorderable | Personalization is valuable |
| Rating type | Per-user (thumbs + favorite) | Simple but expressive |
| Empty "For You" | Show Trending with note | Never empty, encourages engagement |
| Widget preferences storage | Core database | UI customization is core feature |
| Trending source | TMDb via capability | Extensible, can add more providers |
| Edit mode UX | Explicit "Customize" button | Clean default view |
| Initial ordering | Smart defaults | Better new user experience |
| Home endpoint inline | Configurable | Flexibility for different clients |
| Client filtering | Separate widgets with `client_types` | Cleaner API |
| Section failures | Silent omission + admin logging | Graceful degradation |
| Real-time updates | Deferred | Focus on core first |
