# Home Screen & Widget System

## Overview

A multi-client home screen with plugin-based widgets, search, and personalized content rows. The core app provides essential functionality (continue watching, recently added, favorites, trending) that works without plugins. Plugins enhance the experience with AI-powered features (semantic search, personalized recommendations).

### Design Principles

1. **Core functionality without plugins** - Home screen works out of the box
2. **Plugins enhance, not enable** - AI features are additive
3. **API-first design** - Single `/api/home` endpoint serves all clients
4. **User customization** - Reorderable, hideable rows (per-user)
5. **Multi-client support** - Web, iOS, Android, Roku, Fire TV, Smart TV
6. **Tasteful polish** - Clean, fast, not gimmicky

### Non-Goals

- Real-time push updates (future iteration)
- Collaborative filtering across users
- Machine learning model training
- Flashy animations or gimmicky features

---

## Part 1: Architecture

### Core vs Plugin Responsibilities

| Feature | Core | Plugin |
|---------|------|--------|
| Continue Watching | Yes | - |
| Recently Added | Yes | - |
| Your Favorites | Yes | - |
| Trending | Yes (via TrendingProvider) | TMDb, Trakt provide data |
| User Ratings (up/down/favorite) | Yes | - |
| Search (basic text) | Yes | - |
| Search (semantic/AI) | - | semantic-search |
| For You (personalized) | - | recommendations |
| Because You Watched X | - | recommendations |
| Contextual suggestions | - | semantic-search |

### Default Row Order (by priority)

| Priority | Row | Source | Visibility |
|----------|-----|--------|------------|
| 100 | Search Hero | semantic-search OR core fallback | Always |
| 95 | Continue Watching | Core | If user has in-progress items |
| 90 | For You | recommendations plugin | If plugin installed + user has ratings |
| 85 | Recently Added | Core | If library has content |
| 80 | Because You Watched X | recommendations plugin | If plugin installed + user has favorites |
| 70 | Your Favorites | Core | If user has favorited items |
| 50 | Trending | Core (via TrendingProvider) | If a trending provider is registered |

Users can reorder rows via preferences. Smart defaults hide empty rows.

---

## Part 2: API Design

### Primary Endpoint: `GET /api/home`

Returns all home screen sections for the authenticated user.

**Query Parameters:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `inline` | bool | `true` | Include section data inline vs. return data URLs only |
| `sections` | string | `all` | Comma-separated section IDs or `all` |

**Headers:**

| Header | Values | Description |
|--------|--------|-------------|
| `X-Client-Type` | `web`, `ios`, `android`, `roku`, `firetv`, `smarttv` | Client hint for filtering widgets |
| `X-Image-Size` | `small`, `medium`, `large` | Preferred image size |

**Response:**

```json
{
  "sections": [
    {
      "id": "continue-watching",
      "type": "continue-row",
      "client_types": ["all"],
      "position": 0,
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
            "progress": {
              "percent": 45,
              "position_seconds": 3240,
              "duration_seconds": 7200
            }
          }
        ]
      },
      "data_url": "/api/home/sections/continue-watching"
    },
    {
      "id": "recently-added",
      "type": "media-row",
      "client_types": ["all"],
      "position": 1,
      "cache_ttl_seconds": 300,
      "data": {
        "title": "Recently Added",
        "items": [...]
      }
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

### Section Refresh: `GET /api/home/sections/{section_id}`

Returns a single section's data for targeted refresh.

### Preferences Endpoints

```
GET    /api/home/preferences         # Get user's widget ordering
PUT    /api/home/preferences         # Update ordering/visibility
DELETE /api/home/preferences         # Reset to smart defaults
```

### Core Ratings Endpoints

```
GET    /api/ratings                           # List user's ratings
POST   /api/ratings                           # Create/update rating
DELETE /api/ratings/{entity_type}/{entity_id} # Remove rating
```

**POST /api/ratings body:**
```json
{
  "entity_type": "movie",
  "entity_id": 123,
  "rating": "favorite"
}
```

Rating values: `"up"`, `"down"`, `"favorite"`

### Core Genres Endpoint

```
GET /api/genres?media_type=movie    # List distinct genres in user's library
```

Returns genres for dynamic search chips when semantic-search is unavailable.

---

## Part 3: Database Schema

### Core: Widget Preferences (EXISTS)

```sql
-- migrations/000002_widget_preferences.up.sql
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
```

### Core: User Ratings (NEW)

```sql
-- migrations/000003_user_ratings.up.sql
CREATE TABLE user_ratings (
    id INTEGER PRIMARY KEY,
    user_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id INTEGER NOT NULL,
    rating TEXT NOT NULL CHECK(rating IN ('up', 'down', 'favorite')),
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, entity_type, entity_id)
);

CREATE INDEX idx_user_ratings_user ON user_ratings(user_id);
CREATE INDEX idx_user_ratings_entity ON user_ratings(entity_type, entity_id);
CREATE INDEX idx_user_ratings_user_rating ON user_ratings(user_id, rating);
```

### Core: Recently Added Queries (NEW)

```sql
-- sqlite/movies.sql
-- name: ListRecentlyAddedMovies :many
SELECT m.*, med.*
FROM movies m
JOIN media med ON m.media_id = med.id
WHERE med.is_extra = 0
ORDER BY med.created_at DESC
LIMIT ?;

-- name: ListRecentlyAddedTVEpisodes :many
SELECT e.*, med.*
FROM tv_episodes e
JOIN media med ON e.media_id = med.id
WHERE med.is_extra = 0
ORDER BY med.created_at DESC
LIMIT ?;
```

---

## Part 4: Backend Implementation

### File Structure

```
internal/
├── domain/
│   └── ratings/
│       ├── entity.go           # UserRating entity
│       └── repository.go       # Repository interface
├── application/
│   ├── home/
│   │   ├── service.go          # HomeService (EXISTS, modify)
│   │   ├── continue_watching.go # ContinueWatchingService (NEW)
│   │   └── core_widgets.go     # Core widget definitions (NEW)
│   └── ratings/
│       ├── service.go          # RatingsService (NEW)
│       └── dto.go              # DTOs (NEW)
├── infrastructure/
│   ├── persistence/
│   │   └── ratings/
│   │       └── repository.go   # SQL implementation (NEW)
│   └── plugins/
│       └── registry/
│           └── widget.go       # Add RegisterCoreWidgets() (MODIFY)
└── api/
    ├── handlers/
    │   ├── home.go             # (EXISTS)
    │   └── ratings.go          # Ratings handler (NEW)
    └── routes/
        └── ratings.go          # Ratings routes (NEW)
```

### Core Widget Registration

```go
// internal/application/home/core_widgets.go

package home

import "github.com/mantonx/viewra/pkg/plugin/sdk"

// CoreWidgets returns the built-in widgets provided by the core application.
func CoreWidgets() []sdk.Widget {
    return []sdk.Widget{
        {
            ID:              "continue-watching",
            Type:            sdk.WidgetTypeContinueRow,
            Location:        sdk.LocationHomepageSections,
            ClientTypes:     []string{sdk.ClientTypeAll},
            Priority:        95,
            CacheTTLSeconds: 60,
            Config: map[string]any{
                "title": "Continue Watching",
            },
        },
        {
            ID:              "recently-added",
            Type:            sdk.WidgetTypeMediaRow,
            Location:        sdk.LocationHomepageSections,
            ClientTypes:     []string{sdk.ClientTypeAll},
            Priority:        85,
            CacheTTLSeconds: 300,
            Config: map[string]any{
                "title": "Recently Added",
            },
        },
        {
            ID:              "favorites",
            Type:            sdk.WidgetTypeMediaRow,
            Location:        sdk.LocationHomepageSections,
            ClientTypes:     []string{sdk.ClientTypeAll},
            Priority:        70,
            CacheTTLSeconds: 120,
            Config: map[string]any{
                "title": "Your Favorites",
            },
        },
        {
            ID:                 "trending",
            Type:               sdk.WidgetTypeMediaRow,
            Location:           sdk.LocationHomepageSections,
            ClientTypes:        []string{sdk.ClientTypeAll},
            Priority:           50,
            CacheTTLSeconds:    3600,
            RequiredCapability: "trending",
            Config: map[string]any{
                "title": "Trending",
            },
        },
        {
            ID:              "search-hero-fallback",
            Type:            sdk.WidgetTypeSearchHero,
            Location:        sdk.LocationHomepageTop,
            ClientTypes:     []string{sdk.ClientTypeWeb, sdk.ClientTypeIOS, sdk.ClientTypeAndroid},
            Priority:        50, // Lower than semantic-search's 100
            CacheTTLSeconds: 3600,
            Config: map[string]any{
                "placeholder":       "Search...",
                "show_suggestions":  true,
                "suggestions_type":  "genres", // Dynamic genre chips
            },
        },
    }
}
```

### ContinueWatchingService

```go
// internal/application/home/continue_watching.go

package home

import (
    "context"
    "github.com/mantonx/viewra/internal/domain/home"
    "github.com/mantonx/viewra/internal/domain/progress"
    "github.com/mantonx/viewra/internal/domain/media"
)

type ContinueWatchingServiceImpl struct {
    progressRepo progress.Repository
    mediaRepo    media.Repository
    movieRepo    media.MovieRepository
    tvRepo       media.TVRepository
}

func (s *ContinueWatchingServiceImpl) HasHistory(ctx context.Context, userID string) bool {
    // Check if user has any in-progress items
    items, err := s.progressRepo.ListInProgressByUserID(ctx, parseUserID(userID), 1, 0)
    return err == nil && len(items) > 0
}

func (s *ContinueWatchingServiceImpl) GetContinueWatching(ctx context.Context, userID string, limit int) ([]*home.MediaItem, error) {
    // 1. Get in-progress items from progress repo
    progressItems, err := s.progressRepo.ListInProgressByUserID(ctx, parseUserID(userID), limit, 0)
    if err != nil {
        return nil, err
    }
    
    // 2. Enrich with media details
    items := make([]*home.MediaItem, 0, len(progressItems))
    for _, p := range progressItems {
        mediaInfo, err := s.mediaRepo.GetByID(ctx, p.MediaID)
        if err != nil {
            continue
        }
        
        item := &home.MediaItem{
            EntityType: mediaInfo.Type,
            EntityID:   p.MediaID,
            Title:      mediaInfo.Title,
            Year:       mediaInfo.Year,
            Poster:     fmt.Sprintf("/api/images/%s/%d/poster", mediaInfo.Type, p.MediaID),
            Progress: &home.MediaProgress{
                Percent:         int(float64(p.Position) / float64(p.Duration) * 100),
                PositionSeconds: int(p.Position),
                DurationSeconds: int(p.Duration),
            },
        }
        items = append(items, item)
    }
    
    return items, nil
}
```

### HomeService Updates

The existing HomeService needs these changes:

1. **Register core widgets at startup** - Call `RegisterCoreWidgets()` during initialization
2. **Handle core widget data** - Extend `getBuiltinWidgetData()` to handle all core widgets
3. **Wire ContinueWatchingService** - Currently passed as nil

```go
// internal/application/home/service.go - getBuiltinWidgetData updates

func (s *Service) getBuiltinWidgetData(ctx context.Context, widget *registry.RegisteredWidget, userID string) (map[string]any, error) {
    switch widget.Widget.ID {
    case "continue-watching":
        return s.getContinueWatchingData(ctx, userID)
    case "recently-added":
        return s.getRecentlyAddedData(ctx, userID)
    case "favorites":
        return s.getFavoritesData(ctx, userID)
    case "trending":
        return s.getTrendingData(ctx)
    case "search-hero-fallback":
        return s.getSearchHeroFallbackData(ctx)
    default:
        return map[string]any{"title": widget.Widget.Config["title"]}, nil
    }
}

func (s *Service) getRecentlyAddedData(ctx context.Context, userID string) (map[string]any, error) {
    // Fetch recently added movies and TV episodes, merge, sort by date
    // Return as MediaItem slice
}

func (s *Service) getFavoritesData(ctx context.Context, userID string) (map[string]any, error) {
    // Query user_ratings where rating = 'favorite'
    // Enrich with media details
}

func (s *Service) getTrendingData(ctx context.Context) (map[string]any, error) {
    // Use TrendingService to get trending matched to local library
    if s.trendingService == nil || !s.trendingService.HasProvider() {
        return nil, fmt.Errorf("no trending provider")
    }
    result, err := s.trendingService.GetTrending(ctx, "all", 20)
    // Convert to MediaItem format
}

func (s *Service) getSearchHeroFallbackData(ctx context.Context) (map[string]any, error) {
    // Get distinct genres from library
    genres, err := s.genreService.GetDistinctGenres(ctx, "all", 8)
    // Convert to suggestion chips
    suggestions := make([]sdk.Suggestion, 0, len(genres))
    for _, g := range genres {
        suggestions = append(suggestions, sdk.Suggestion{
            ID:     strings.ToLower(g),
            Label:  g,
            Action: sdk.SuggestionAction{Type: "filter", Filter: map[string]string{"genre": g}},
        })
    }
    return map[string]any{
        "placeholder":  "Search...",
        "suggestions":  suggestions,
        "search_url":   "/api/search",
    }, nil
}
```

---

## Part 5: Recommendations Plugin Updates

### Remove from Plugin

1. **Remove `favorites` widget** - Now in core
2. **Remove ratings table creation** - Now in core migrations
3. **Update to use core ratings table** - Read from `user_ratings` instead of creating own table

### Keep in Plugin (AI-powered only)

```go
// plugins/recommendations/internal/schema.go

func SettingsSchema() *sdk.Schema {
    return sdk.NewSchema("Recommendations Settings").
        Meta(sdk.PluginMeta{
            DisplayName: "Recommendations",
            Description: "AI-powered personalized recommendations",
            Icon:        "sparkles",
        }).
        Property("enabled", sdk.Boolean().
            Title("Enable Recommendations").
            Default(true)).
        Widgets([]sdk.Widget{
            {
                ID:                 "rec-for-you",
                Type:               sdk.WidgetTypeMediaRow,
                Location:           sdk.LocationHomepageSections,
                ClientTypes:        []string{sdk.ClientTypeAll},
                Priority:           90,
                CacheTTLSeconds:    300,
                RequiredCapability: "embedding", // Requires semantic-search
                Config: map[string]any{
                    "endpoint": "/recommendations/for-you",
                    "title":    "For You",
                },
                SettingsKey: "enabled",
            },
            {
                ID:                 "rec-because-you-liked",
                Type:               sdk.WidgetTypeMediaRow,
                Location:           sdk.LocationHomepageSections,
                ClientTypes:        []string{sdk.ClientTypeAll},
                Priority:           80,
                CacheTTLSeconds:    600,
                RequiredCapability: "embedding",
                Config: map[string]any{
                    "endpoint": "/recommendations/because-you-liked",
                    "title":    "Because You Liked",
                },
                SettingsKey: "enabled",
            },
        })
}
```

### Plugin Accesses Core Ratings

```go
// plugins/recommendations/internal/plugin.go

func (p *RecommendationsPlugin) Initialize(ctx context.Context, dataDir string, config []byte, services *sdk.HostServices) error {
    // Don't create ratings table - use core's user_ratings table
    // Access via host data service or direct SQL queries
    
    if services.Storage != nil {
        p.sql = services.Storage.SQL()
        // Query core user_ratings table for recommendations
    }
}
```

---

## Part 6: Frontend Implementation

### Home Screen Layout

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│  [🔍 Search...]                                                │
│                                                                 │
│  [Action] [Comedy] [Sci-Fi] [Drama] [Documentary] [Thriller]   │
│                                                                 │
│  142 Movies  ·  38 TV Shows  ·  256 Albums                     │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Continue Watching                                              │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐       │
│  │        │ │        │ │        │ │        │ │        │  →    │
│  │ ▓▓▓▓░░ │ │ ▓▓░░░░ │ │ ▓▓▓░░░ │ │ ▓░░░░░ │ │ ▓▓▓▓▓░ │       │
│  └────────┘ └────────┘ └────────┘ └────────┘ └────────┘       │
│                                                                 │
│  For You                                                        │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐       │
│  │        │ │        │ │        │ │        │ │        │  →    │
│  └────────┘ └────────┘ └────────┘ └────────┘ └────────┘       │
│                                                                 │
│  Recently Added                                                 │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐       │
│  │        │ │        │ │        │ │        │ │        │  →    │
│  └────────┘ └────────┘ └────────┘ └────────┘ └────────┘       │
│                                                                 │
│  Because You Watched "Breaking Bad"                             │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐       │
│  │        │ │        │ │        │ │        │ │        │  →    │
│  └────────┘ └────────┘ └────────┘ └────────┘ └────────┘       │
│                                                                 │
│  Your Favorites                                                 │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐       │
│  │        │ │        │ │        │ │        │ │        │  →    │
│  └────────┘ └────────┘ └────────┘ └────────┘ └────────┘       │
│                                                                 │
│  Trending                                                       │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐       │
│  │        │ │        │ │        │ │        │ │        │  →    │
│  └────────┘ └────────┘ └────────┘ └────────┘ └────────┘       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Responsive Breakpoints

| Breakpoint | Cards Per Row | Notes |
|------------|---------------|-------|
| < 640px (mobile) | 2-3 | Chips wrap, smaller cards |
| 640-1024px (tablet) | 4-5 | Standard layout |
| 1024-1440px (desktop) | 5-6 | Comfortable spacing |
| > 1440px (large) | 7-8+ | More visible cards |

### File Changes

```
web/src/
├── views/
│   └── home/
│       └── Home.tsx                    # Simplify: remove QuickActions, use widget system
├── components/
│   └── home/
│       ├── widgets/
│       │   ├── WidgetContainer.tsx     # Handle all widget types
│       │   ├── MediaRow.tsx            # Standard media row
│       │   ├── ContinueRow.tsx         # Row with progress bars (NEW)
│       │   ├── SearchHero.tsx          # Search with chips
│       │   └── SuggestionChip.tsx      # Chip component
│       ├── ContinueWatching.tsx        # REMOVE (use widget system)
│       └── index.ts
└── lib/
    └── hooks/
        └── useWidgets.ts               # Add useGenreChips hook
```

### Home.tsx Updates

```tsx
// web/src/views/home/Home.tsx

export const Home = () => {
  const { data: librariesData } = useQuery(getGetApiLibrariesQueryOptions())
  const { data: homeSections, isLoading } = useHomeSections()

  const libraries = librariesData?.status === 200 ? librariesData.data.libraries ?? [] : []
  const movieCount = libraries.filter((l) => l.type === 'movie').length
  const tvCount = libraries.filter((l) => l.type === 'tv').length
  const musicCount = libraries.filter((l) => l.type === 'music').length

  // Separate top widgets (search hero) from content rows
  const topSections = homeSections?.sections?.filter(
    (s) => s.location === 'homepage-top'
  ) ?? []
  
  const contentSections = homeSections?.sections?.filter(
    (s) => s.location === 'homepage-sections'
  ) ?? []

  if (isLoading) return <HomeSkeleton />

  return (
    <div className="h-full overflow-auto">
      <div className="page-enter">
        {/* Hero Section - simplified */}
        <div className="p-8 pb-6">
          {/* Search Hero (from widget system) */}
          {topSections.map((section) => (
            <WidgetContainer key={section.id} section={section} />
          ))}

          {/* Stats - compact inline */}
          <div className="mt-6 text-sm text-neutral-500 dark:text-neutral-400">
            {movieCount} Movies · {tvCount} TV Shows · {musicCount} Albums
          </div>
        </div>

        {/* Content Rows - all via widget system */}
        <div className="px-8 pb-8 space-y-8">
          {contentSections.map((section) => (
            <WidgetContainer key={section.id} section={section} />
          ))}
        </div>

        {/* Customize button - appears on hover */}
        <CustomizeButton />
      </div>
    </div>
  )
}
```

### Rating UI on Media Detail Pages

Add rating buttons to movie/TV detail pages:

```tsx
// web/src/components/media/RatingButtons.tsx

type RatingButtonsProps = {
  entityType: 'movie' | 'tv_show' | 'tv_episode'
  entityId: number
  currentRating?: 'up' | 'down' | 'favorite' | null
}

export const RatingButtons = ({ entityType, entityId, currentRating }: RatingButtonsProps) => {
  const { mutate: setRating } = useSetRating()
  const { mutate: deleteRating } = useDeleteRating()

  const handleRate = (rating: 'up' | 'down' | 'favorite') => {
    if (currentRating === rating) {
      deleteRating({ entityType, entityId })
    } else {
      setRating({ entityType, entityId, rating })
    }
  }

  return (
    <div className="flex items-center gap-2">
      <button
        onClick={() => handleRate('up')}
        className={cn(
          'p-2 rounded-lg transition-colors',
          currentRating === 'up' 
            ? 'bg-green-500/20 text-green-500' 
            : 'hover:bg-neutral-100 dark:hover:bg-white/10'
        )}
        aria-label="Like"
      >
        <ThumbsUp className="w-5 h-5" />
      </button>
      <button
        onClick={() => handleRate('down')}
        className={cn(
          'p-2 rounded-lg transition-colors',
          currentRating === 'down'
            ? 'bg-red-500/20 text-red-500'
            : 'hover:bg-neutral-100 dark:hover:bg-white/10'
        )}
        aria-label="Dislike"
      >
        <ThumbsDown className="w-5 h-5" />
      </button>
      <button
        onClick={() => handleRate('favorite')}
        className={cn(
          'p-2 rounded-lg transition-colors',
          currentRating === 'favorite'
            ? 'bg-pink-500/20 text-pink-500'
            : 'hover:bg-neutral-100 dark:hover:bg-white/10'
        )}
        aria-label="Add to Favorites"
      >
        <Heart className={cn('w-5 h-5', currentRating === 'favorite' && 'fill-current')} />
      </button>
    </div>
  )
}
```

---

## Part 7: Implementation Phases

### Phase 1: Core Infrastructure (Backend)

1. **Create user_ratings migration** - `migrations/000003_user_ratings.{up,down}.sql`
2. **Create ratings domain** - `internal/domain/ratings/`
3. **Create ratings repository** - `internal/infrastructure/persistence/ratings/`
4. **Create ratings API** - Handler and routes
5. **Add recently added SQL queries** - Both SQLite and PostgreSQL
6. **Add genres endpoint** - For dynamic search chips

### Phase 2: Core Widgets (Backend)

1. **Create core_widgets.go** - Widget definitions
2. **Implement ContinueWatchingService** - Wire to progress repo
3. **Update HomeService** - Handle all core widget data
4. **Update widget registry** - `RegisterCoreWidgets()` method
5. **Wire in app startup** - Register core widgets on init

### Phase 3: Update Recommendations Plugin

1. **Remove favorites widget** from schema
2. **Remove ratings table creation** from migrations
3. **Update to read from core user_ratings** table
4. **Add `RequiredCapability: "embedding"`** to widgets

### Phase 4: Frontend Updates

1. **Simplify Home.tsx** - Remove hardcoded components
2. **Remove QuickActions**
3. **Create ContinueRow component** - For progress bars
4. **Update WidgetContainer** - Handle all types
5. **Add RatingButtons component**
6. **Add rating buttons to detail pages**
7. **Add useGenreChips hook** - For fallback search

### Phase 5: Polish

1. **Empty states** - Helpful messages, no cartoon illustrations
2. **Loading states** - Progressive row loading
3. **Error handling** - Graceful degradation
4. **Keyboard navigation** - Focus states, tab order
5. **Responsive testing** - All breakpoints

---

## Part 8: File Changes Summary

### Backend - New Files

| File | Description |
|------|-------------|
| `migrations/000003_user_ratings.up.sql` | Core ratings table |
| `migrations/000003_user_ratings.down.sql` | Drop ratings table |
| `migrations/postgres/000003_user_ratings.up.sql` | PostgreSQL version |
| `migrations/postgres/000003_user_ratings.down.sql` | PostgreSQL drop |
| `internal/domain/ratings/entity.go` | UserRating entity |
| `internal/domain/ratings/repository.go` | Repository interface |
| `internal/infrastructure/persistence/ratings/repository.go` | SQL implementation |
| `internal/application/ratings/service.go` | Ratings service |
| `internal/application/ratings/dto.go` | DTOs |
| `internal/application/home/continue_watching.go` | ContinueWatchingService |
| `internal/application/home/core_widgets.go` | Core widget definitions |
| `internal/api/handlers/ratings.go` | Ratings handler |
| `internal/api/routes/ratings.go` | Ratings routes |

### Backend - Modified Files

| File | Changes |
|------|---------|
| `internal/infrastructure/database/queries/sqlite/movies.sql` | Add ListRecentlyAddedMovies |
| `internal/infrastructure/database/queries/postgres/movies.sql` | Add ListRecentlyAddedMovies |
| `internal/infrastructure/database/queries/sqlite/tv_shows.sql` | Add ListRecentlyAddedTVEpisodes |
| `internal/infrastructure/database/queries/postgres/tv_shows.sql` | Add ListRecentlyAddedTVEpisodes |
| `internal/infrastructure/plugins/registry/widget.go` | Add RegisterCoreWidgets() |
| `internal/application/home/service.go` | Handle core widget data |
| `internal/app/handlers/handlers.go` | Wire services, register core widgets |
| `internal/app/services/services.go` | Create ContinueWatchingService |

### Plugin - Modified Files

| File | Changes |
|------|---------|
| `plugins/recommendations/internal/schema.go` | Remove favorites, add RequiredCapability |
| `plugins/recommendations/internal/plugin.go` | Remove ratings table creation, use core table |

### Frontend - New Files

| File | Description |
|------|-------------|
| `web/src/components/home/widgets/ContinueRow.tsx` | Row with progress bars |
| `web/src/components/media/RatingButtons.tsx` | Rating buttons component |
| `web/src/lib/hooks/useRatings.ts` | Rating mutations |

### Frontend - Modified Files

| File | Changes |
|------|---------|
| `web/src/views/home/Home.tsx` | Simplify, remove QuickActions |
| `web/src/components/home/widgets/WidgetContainer.tsx` | Handle all types |
| `web/src/components/home/widgets/SearchHero.tsx` | Support genre chips fallback |
| `web/src/components/home/index.ts` | Remove ContinueWatching export |
| `web/src/lib/hooks/useWidgets.ts` | Add useGenreChips |

### Frontend - Removed Files

| File | Reason |
|------|--------|
| `web/src/components/home/ContinueWatching.tsx` | Replaced by widget system |

---

## Part 9: Testing Checklist

### API Tests

- [ ] `GET /api/home` returns core widgets without plugins
- [ ] `GET /api/home` includes plugin widgets when available
- [ ] `GET /api/home/sections/{id}` returns single section
- [ ] `GET /api/home/preferences` returns user preferences
- [ ] `PUT /api/home/preferences` updates ordering
- [ ] `DELETE /api/home/preferences` resets to defaults
- [ ] `GET /api/ratings` returns user's ratings
- [ ] `POST /api/ratings` creates rating
- [ ] `DELETE /api/ratings/{type}/{id}` removes rating
- [ ] `GET /api/genres` returns distinct genres

### Widget Tests

- [ ] Continue Watching shows in-progress items
- [ ] Continue Watching hidden when no progress
- [ ] Recently Added shows newest content
- [ ] Favorites shows favorited items
- [ ] Favorites hidden when no favorites
- [ ] Trending shows when provider available
- [ ] Trending hidden when no provider
- [ ] Search hero shows AI suggestions when semantic-search available
- [ ] Search hero shows genre chips as fallback

### Frontend Tests

- [ ] Home renders all widget rows
- [ ] Rows scroll horizontally
- [ ] Responsive card counts at breakpoints
- [ ] Customize button appears on hover
- [ ] Preferences modal allows reordering
- [ ] Hidden rows don't appear
- [ ] Empty states display correctly
- [ ] Rating buttons work on detail pages

---

## Decisions Log

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Favorites location | Core | Simple query, doesn't need AI |
| Ratings table | Core | User preference data, not plugin-specific |
| QuickActions | Remove | Doesn't help design, navigation elsewhere |
| Hero section | Simplify | Search + stats only, no welcome text |
| Search fallback | Dynamic genre chips | Based on user's library content |
| Trending source | TrendingProvider registry | Any plugin can contribute |
| Customize button | Hover only | Cleaner default view |
| Rating types | up/down/favorite | Simple but expressive |
| Empty rows | Smart defaults hide | Never show empty sections |
| Gimmicks | Avoid | Tasteful polish, not flashy |

---

## Implementation Progress

### Phase 1: Core Infrastructure (Backend) - COMPLETE

| Task | Status | Notes |
|------|--------|-------|
| Create user_ratings migration | DONE | `migrations/000003_user_ratings.{up,down}.sql` for both SQLite and PostgreSQL |
| Create ratings domain | DONE | `internal/domain/ratings/entity.go`, `repository.go` |
| Create ratings repository | DONE | `internal/infrastructure/persistence/ratings/repository.go` |
| Create ratings service | DONE | `internal/application/ratings/service.go` |
| Create ratings API handler | DONE | `internal/api/handlers/ratings.go` |
| Create ratings routes | DONE | `internal/api/routes/ratings.go` |
| Wire into app startup | DONE | Updated `repositories.go`, `handlers.go`, `server.go` |
| Add SQLC queries | DONE | `queries/sqlite/user_ratings.sql`, `queries/postgres/user_ratings.sql` |
| Add recently added SQL queries | DONE | Added `ListRecentlyAddedMovies` to both backends |
| Add genres endpoint | DONE | Added `ListDistinctMovieGenres` to both backends |

**Cleanup notes:** Consolidated duplicate `getUserID` functions into shared `getUserIDFromContext()` in helpers.go. Added nil check to route registration.

### Phase 2: Core Widgets (Backend) - COMPLETE

| Task | Status | Notes |
|------|--------|-------|
| Create core_widgets.go | DONE | 5 core widgets: continue-watching, recently-added, favorites, trending, search-hero-fallback |
| Implement ContinueWatchingService | DONE | `internal/application/home/continue_watching.go` - wired to progress and media repos |
| Create RecentlyAddedService | DONE | `internal/application/home/services.go` - supports movies + TV shows combined or separate |
| Create FavoritesService | DONE | `internal/application/home/services.go` - supports movies + TV shows |
| Create GenresService | DONE | `internal/application/home/services.go` - uses movie repository |
| Update HomeService | DONE | Added new service interfaces, updated `NewService` constructor, implemented `getBuiltinWidgetData` for all widget types |
| Update buildMeta | DONE | Now checks `HasRatings` via FavoritesService |
| Wire in app startup | DONE | `handlers.go` creates all services, registers core widgets via `RegisterAll()` |
| Add ListRecentlyAdded to MovieRepository | DONE | Domain interface + persistence implementation |
| Add ListDistinctGenres to MovieRepository | DONE | Domain interface + persistence implementation |
| Add ListRecentlyAddedShows to TVRepository | DONE | Domain interface + persistence implementation + SQL queries |

**Architecture notes:**
- Services use domain repository interfaces (clean architecture)
- `RecentlyAddedService` offers: `GetRecentlyAdded()` (combined), `GetRecentlyAddedMovies()`, `GetRecentlyAddedTVShows()`
- `FavoritesService` fetches both movie and TV show favorites
- `MediaItem` has `CreatedAt` field (not serialized) for sorting combined results

**Files created:**
- `internal/application/home/core_widgets.go` - Core widget definitions
- `internal/application/home/continue_watching.go` - ContinueWatchingService implementation
- `internal/application/home/services.go` - RecentlyAddedService, FavoritesService, GenresService implementations

**Files modified:**
- `internal/domain/media/repository.go` - Added `ListRecentlyAdded`, `ListDistinctGenres` to MovieRepository; `ListRecentlyAddedShows` to TVRepository
- `internal/domain/home/types.go` - Added `CreatedAt` field to MediaItem
- `internal/infrastructure/persistence/movie/repository.go` - Implemented new methods
- `internal/infrastructure/persistence/movie/types.go` - Added `recentlyAddedRowToDomain` converter
- `internal/infrastructure/persistence/tvshow/repository.go` - Implemented `ListRecentlyAddedShows`
- `internal/infrastructure/database/queries/sqlite/tv_shows.sql` - Added `ListRecentlyAddedTVShows` query
- `internal/infrastructure/database/queries/postgres/tv_shows.sql` - Added `ListRecentlyAddedTVShows` query
- `internal/application/home/service.go` - Added service interfaces, updated constructor
- `internal/app/handlers/handlers.go` - Create and wire all home services, register core widgets

### Phase 3: Update Recommendations Plugin - COMPLETE

| Task | Status | Notes |
|------|--------|-------|
| Remove favorites widget | DONE | Removed from schema.go, handler, and route |
| Remove ratings table creation | DONE | Removed runMigrations(), plugin now uses core table |
| Remove 'ratings' from capabilities | DONE | Plugin only provides 'recommendations' and 'widgets' now |
| Add RequiredCapability | DONE | Both `rec-for-you` and `rec-because-you-liked` require `embedding` |
| Remove unused GetFavorites method | DONE | Cleaned up recommendations.go |
| Create HostRatings gRPC service | DONE | Plugins access core ratings via SDK.RatingsClient |
| Update plugin to use SDK RatingsClient | DONE | Replaced SQL queries with SDK calls |
| Remove ratings write endpoints from plugin | DONE | Users use core's `/api/ratings` for writes |

**HostRatings Service (new plugin host service):**
The recommendations plugin couldn't directly query the `user_ratings` table because the SDK's SQLClient automatically prefixes table names with `plugin_{id}_`. We solved this by:

1. **Added HostRatings proto service** - `api/proto/plugin/host_services.proto` with ListRatings, GetRatedEntityIDs, HasRatings
2. **Implemented RatingsServer** - `internal/infrastructure/plugins/host/ratings.go`
3. **Added RatingsClient to SDK** - `pkg/plugin/sdk/ratings.go`
4. **Wired into plugin broker** - Updated factory, manager, loader to dispense HostRatings service
5. **Added HostRatingsBrokerId to InitRequest** - Plugins receive broker ID during initialization
6. **Updated SDK HostServices** - Added Ratings field to HostServices struct

**Files created:**
- `internal/infrastructure/plugins/host/ratings.go` - HostRatings gRPC server implementation
- `pkg/plugin/sdk/ratings.go` - RatingsClient SDK wrapper

**Files modified (core):**
- `api/proto/plugin/host_services.proto` - Added HostRatings service and messages
- `api/proto/plugin/plugin_core.proto` - Added host_ratings_broker_id to InitRequest
- `internal/infrastructure/plugins/grpc/plugin.go` - Added HostRatingsPlugin
- `internal/infrastructure/plugins/grpc/factory.go` - Added NewHostRatingsGRPCPlugin
- `internal/infrastructure/plugins/manager/manager.go` - Added hostRatingsServer field and getter
- `internal/infrastructure/plugins/manager/factory.go` - Added factory interface method
- `internal/infrastructure/plugins/manager/loader.go` - Wire HostRatings into plugin map and dispense
- `internal/infrastructure/plugins/service.go` - Export HostRatingsServer type alias
- `internal/app/services/services.go` - Create and pass hostRatingsServer to manager
- `pkg/plugin/sdk/enricher.go` - Added Ratings to HostServices, connect on init
- `pkg/plugin/sdk/widget.go` - Connect to HostRatings on init

**Files modified (plugin):**
- `plugins/recommendations/internal/plugin.go` - Use sdk.RatingsClient directly, removed ratingsService wrapper
- `plugins/recommendations/internal/recommendations.go` - Use sdk.RatingsClient directly
- `plugins/recommendations/internal/schema.go` - Removed favorites widget, added RequiredCapability
- `plugins/recommendations/plugin.yml` - Removed 'ratings' from capabilities.provides

**Files deleted (plugin cleanup):**
- `plugins/recommendations/internal/ratings.go` - Removed unnecessary wrapper, use sdk.RatingsClient directly

### Phase 4: Frontend Updates - COMPLETE

| Task | Status | Notes |
|------|--------|-------|
| Simplify Home.tsx | DONE | Removed hardcoded ContinueWatching and QuickActions, uses widget system |
| Create ContinueRow component | DONE | Renders media cards with progress bars for continue-watching widget |
| Update WidgetContainer | DONE | Routes continue-row type to ContinueRow component |
| Remove legacy ContinueWatching | DONE | Deleted standalone component, widget system handles it |
| Create RatingButtons component | DONE | Thumbs up/down/favorite with API integration |
| Add ratings to MovieCard | DONE | Via MediaMetadata component |
| Add ratings to TVShowCard | DONE | Via MediaMetadata component |
| Generate ratings API client | DONE | Ran `make api-client-gen` |

**Files created:**
- `web/src/components/home/widgets/ContinueRow.tsx` - Continue watching row with progress bars
- `web/src/components/media/RatingButtons/RatingButtons.tsx` - Rating buttons component
- `web/src/components/media/RatingButtons/index.ts` - Export

**Files modified:**
- `web/src/views/home/Home.tsx` - Removed ContinueWatching import, QuickActions component, simplified layout
- `web/src/components/home/widgets/WidgetContainer.tsx` - Route continue-row to ContinueRow
- `web/src/components/home/widgets/index.ts` - Export ContinueRow
- `web/src/components/home/index.ts` - Removed ContinueWatching export
- `web/src/components/media/index.ts` - Export RatingButtons
- `web/src/components/media/MediaMetadata/MediaMetadata.tsx` - Added optional rating prop
- `web/src/components/media/MediaMetadata/MediaMetadata.types.ts` - Added rating interface
- `web/src/components/movies/MovieCard/MovieCard.tsx` - Pass rating prop to MediaMetadata
- `web/src/components/tv/TVShowCard/TVShowCard.tsx` - Pass rating prop to MediaMetadata

**Files deleted:**
- `web/src/components/home/ContinueWatching.tsx` - Replaced by widget system

**Deferred (low priority):**
- `useGenreChips` hook for fallback search when semantic-search unavailable

**Bug fix (2026-01-01):**
- Fixed missing `Location` and `Priority` fields in `home.Section` JSON output
- Backend was not populating these fields when building sections in `service.go`
- Frontend `WidgetContainer` filters by `location` - widgets weren't displaying because `section.location` was empty
- Updated `GetSection()`, `fetchWidgetData()`, and `buildSectionsWithURLs()` to copy `Location` and `Priority` from widget definition

### Phase 5: Polish - IN PROGRESS

| Task | Status | Notes |
|------|--------|-------|
| Horizontal scroll affordances | DONE | ScrollableRow component with gradient fades and nav buttons |
| Movie cards use MovieCard | DONE | Recently Added uses full MovieCard with metadata |
| TV show cards use TVShowCard | DONE | Recently Added uses TVShowCard |
| Remove welcome text/stats | DONE | Cleaner home page layout |
| Missing posters investigation | DONE | First 4 movies lack images in DB (need enrichment) |

**ScrollableRow component (`web/src/components/common/ScrollableRow/`):**
- Gradient fades on edges indicate scrollable content
- Large navigation buttons (48px) appear on hover
- Buttons always on top of cards (`z-[100]`)
- Smooth scroll behavior (80% of viewport width)
- Hidden scrollbar for clean appearance
- Cursor pointer on buttons

**Home page states (`web/src/views/home/Home.tsx`):**
- Loading skeleton with shimmer effect
- Empty state for new users with "Add Library" CTA
- Error state with retry button

**Files created:**
- `web/src/components/common/ScrollableRow/ScrollableRow.tsx`
- `web/src/components/common/ScrollableRow/index.ts`

**Files modified:**
- `web/src/components/common/index.ts` - Export ScrollableRow
- `web/src/components/home/widgets/MediaRow.tsx` - Use ScrollableRow
- `web/src/index.css` - Added `.scrollbar-hide` utility
- `web/src/views/home/Home.tsx` - Added loading, empty, and error states

**Completed polish tasks:**
- [x] Horizontal scroll affordances
- [x] Empty states - Welcome message with "Add Library" CTA
- [x] Loading states - Skeleton with shimmer animation
- [x] Error handling - Error state with retry button

**Remaining polish tasks:**
- [ ] Keyboard navigation - Focus states, tab order
- [ ] Responsive testing - All breakpoints

### Phase 6: Plugin-to-Plugin VectorSearch Communication - COMPLETE

The recommendations plugin can now call `FindSimilar` on the semantic-search plugin to generate AI-powered recommendations. When semantic search is unavailable, it falls back to genre-based recommendations.

**Implementation Summary:**

The `InvokeVectorSearch` RPC enables plugins to invoke vector search methods on other plugins that provide the `vector_search` capability. The recommendations plugin uses this to call `FindSimilar()` on the semantic-search plugin.

| Task | Status | Notes |
|------|--------|-------|
| Add `InvokeVectorSearch` RPC to HostPlugins | DONE | `api/proto/plugin/host_services.proto` |
| Add `ListMediaByGenre` RPC to HostData | DONE | `api/proto/plugin/host_services.proto` |
| Add `VectorSearchClient` to Instance | DONE | `internal/infrastructure/plugins/types/types.go` |
| Add `VectorSearchPlugin` gRPC plugin | DONE | `internal/infrastructure/plugins/grpc/plugin.go` |
| Wire VectorSearch in loader | DONE | `internal/infrastructure/plugins/manager/loader.go` |
| Implement `InvokeVectorSearch` server | DONE | `internal/infrastructure/plugins/host/plugins.go:569` |
| Implement `ListMediaByGenre` server | DONE | `internal/infrastructure/plugins/host/data.go:305` |
| Add `ListMediaByGenre` to querier | DONE | `internal/infrastructure/plugins/querier/querier.go:311` |
| Add `FindSimilar` to SDK PluginsClient | DONE | `pkg/plugin/sdk/plugins_client.go:470` |
| Add `SemanticSearch` to SDK PluginsClient | DONE | `pkg/plugin/sdk/plugins_client.go:524` |
| Add `IsVectorSearchAvailable` to SDK | DONE | `pkg/plugin/sdk/plugins_client.go:457` |
| Add `ListMediaByGenre` to SDK DataClient | DONE | `pkg/plugin/sdk/host.go:138` |
| Add `vector_search` to semantic-search manifest | DONE | `plugins/semantic-search/plugin.yml:25` |
| Implement `getSimilarItems()` in recommendations | DONE | `plugins/recommendations/internal/recommendations.go:182` |
| Implement `getGenreBasedRecommendations()` | DONE | `plugins/recommendations/internal/recommendations.go:253` |

**Architecture:**

```
┌─────────────────────┐         ┌─────────────────────┐
│  recommendations    │         │  semantic-search    │
│      plugin         │         │      plugin         │
├─────────────────────┤         ├─────────────────────┤
│ getSimilarItems()   │         │ VectorSearchPlugin  │
│   │                 │         │   FindSimilar()     │
│   ▼                 │         │   Search()          │
│ sdk.PluginsClient   │         └─────────────────────┘
│   .FindSimilar()    │                   ▲
└─────────────────────┘                   │
          │                               │
          ▼                               │
┌─────────────────────────────────────────┴─────┐
│                    HOST                        │
├───────────────────────────────────────────────┤
│  HostPlugins.InvokeVectorSearch()             │
│    │                                          │
│    ├─► findVectorSearchProvider()             │
│    │     └─► capabilities["vector_search"]    │
│    │                                          │
│    └─► instance.VectorSearchClient.FindSimilar│
└───────────────────────────────────────────────┘
```

**SDK Usage in Recommendations Plugin:**

```go
// Check if vector search is available
if s.plugins.IsVectorSearchAvailable(ctx) {
    // Use semantic search for high-quality similarity
    results, _, err := s.plugins.FindSimilar(ctx, entityType, entityID, limit)
    if err == nil {
        return convertResults(results)
    }
}

// Fallback to genre-based recommendations
return s.getGenreBasedRecommendationsForItem(ctx, baseItem, exclude, limit)
```

**Fallback Flow:**

1. Check `IsVectorSearchAvailable()` - returns true if semantic-search plugin is enabled
2. If available, call `FindSimilar()` via `InvokeVectorSearch` RPC
3. If unavailable or fails, use `ListMediaByGenre()` for genre-based fallback
4. Genre fallback queries movies/shows matching the base item's genres

---

## Master Implementation Tracker

This section tracks the full implementation across all parts of the home-screen-recommendations documentation.

### Overall Status

| Part | Description | Status | Progress |
|------|-------------|--------|----------|
| Part 1 | Core infrastructure, ratings, widgets, plugin comms | COMPLETE | 6/6 phases |
| Part 2 | UX enhancements (continue watching, hero, polish) | PENDING | 0/5 phases |
| Part 4 | Collaborative filtering algorithms (SAR, BPR, hybrid) | PENDING | 0/4 phases |

### Part 1 Phases (COMPLETE)

| Phase | Description | Status |
|-------|-------------|--------|
| Phase 1 | Core Infrastructure (ratings, migrations, API) | COMPLETE |
| Phase 2 | Core Widgets (continue-watching, recently-added, favorites) | COMPLETE |
| Phase 3 | Update Recommendations Plugin (use SDK, HostRatings) | COMPLETE |
| Phase 4 | Frontend Widget System | COMPLETE |
| Phase 5 | Frontend Polish (scroll, loading, error states) | PARTIAL |
| Phase 6 | Plugin-to-Plugin VectorSearch Communication | COMPLETE |

**Part 1 Remaining:**
- [ ] Keyboard navigation (focus states, arrow key navigation)
- [ ] Responsive testing (all breakpoints)

### Part 2 Phases (PENDING)

| Phase | Description | Status | Est. Effort |
|-------|-------------|--------|-------------|
| Phase 1 | Continue Watching Redesign | PENDING | 1-1.5 days |
| Phase 2 | Hero Backdrop | PENDING | 0.5 days |
| Phase 3 | Row Polish (NewBadge, counts, MarkWatched) | PENDING | 0.5 days |
| Phase 4 | Empty States Enhancement | COMPLETE | - |
| Phase 5 | Distinguishing Features (time-based ordering, keyboard) | PENDING | 0.5 days |

**Part 2 Tasks:**
- [ ] Add progress data to continue watching response (backend)
- [ ] Create `FormatRemainingTime()` helper
- [ ] Create `ContinueWatchingCard` component (horizontal 16:9 with progress bar)
- [ ] Update `ContinueRow` to use horizontal cards
- [ ] Add `EpisodeContext` type for TV episode info
- [ ] Add `HeroData` to home response (backdrop, greeting, date)
- [ ] Create `HeroBackdrop` component
- [ ] Create `NewBadge` component
- [ ] Create `MarkWatchedButton` component
- [ ] Add item counts to row headers
- [ ] Add keyboard navigation to `ScrollableRow`
- [ ] Add focus styles to cards

### Part 4 Phases (PENDING)

| Phase | Description | Status | Est. Effort |
|-------|-------------|--------|-------------|
| Phase 1 | User Embedding Average | PENDING | 1-2 days |
| Phase 1.5 | SAR Collaborative Filtering | PENDING | 1-2 days |
| Phase 2 | BPR Collaborative Filtering | DEFERRED | 3-5 days |
| Phase 3 | Hybrid Scoring | PENDING | 1-2 days |
| Phase 4 | Enhancements (cold start, persistence) | PENDING | 2-3 days |

**Part 4 Tasks:**
- [ ] Create `HostProgress` gRPC service (expose watch history to plugins)
- [ ] Add `ProgressClient` to plugin SDK
- [ ] Create `user_embedding.go` (taste profiles from liked items)
- [ ] Create `cf/sar.go` (SAR algorithm implementation)
- [ ] Create `cf/types.go` (interaction types)
- [ ] Create `hybrid.go` (combine CF + semantic + exploration)
- [ ] Add unit tests for SAR, hybrid scoring
- [ ] Implement cold start handling (popular items for new users)
- [ ] Integrate implicit feedback (watch completion %)
- [ ] Add model persistence

**Note:** BPR is deferred until SAR is tested. If SAR provides sufficient recommendation quality, BPR may not be needed.

### Infrastructure Dependencies

| Dependency | Required By | Status |
|------------|-------------|--------|
| HostProgress gRPC service | Part 4 (SAR, implicit feedback) | PENDING |
| Plugin Files SDK | Part 4 (model persistence) | EXISTS |
| pgvector | Part 4 (factor storage) | EXISTS |

### Execution Order

| Order | Task | Part | Effort |
|-------|------|------|--------|
| 1 | Continue Watching Redesign | Part 2 Phase 1 | 1-1.5 days |
| 2 | Hero Backdrop | Part 2 Phase 2 | 0.5 days |
| 3 | Row Polish | Part 2 Phase 3 | 0.5 days |
| 4 | Keyboard & Responsive | Part 1 Phase 5 + Part 2 Phase 5 | 0.5 days |
| 5 | HostProgress Service | Infrastructure | 0.5-1 day |
| 6 | User Embedding Average | Part 4 Phase 1 | 1-2 days |
| 7 | SAR Collaborative Filtering | Part 4 Phase 1.5 | 1-2 days |
| 8 | Hybrid Scoring | Part 4 Phase 3 | 1-2 days |
| 9 | CF Unit Tests | Part 4 | 0.5 days |
| 10 | CF Enhancements | Part 4 Phase 4 | 2-3 days |
| 11 | BPR (if needed) | Part 4 Phase 2 | 3-5 days |

**Total Estimated Effort: 8-10 days (core) or 13-15 days (with BPR)**
