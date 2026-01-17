# ViewRA Architecture

## Overview

ViewRA uses Clean Architecture with Domain-Driven Design (DDD). Dependencies flow inward - outer layers depend on inner layers, never the reverse.

```text
┌─────────────────────────────────────────────┐
│           API Layer (HTTP)                  │
│         ↓ depends on ↓                      │
│         Application Layer                   │
│         ↓ depends on ↓                      │
│           Domain Layer                      │
│         ↑ implements ↑                      │
│      Infrastructure Layer                   │
└─────────────────────────────────────────────┘
        ↕ communicates via gRPC ↕
┌─────────────────────────────────────────────┐
│              Plugin System                  │
│   (TMDb, MusicBrainz, Semantic Search...)   │
└─────────────────────────────────────────────┘
```

## Layers

### Domain (`internal/domain/`)

Core business logic. **No external dependencies** - only standard library.

16 modules: library, media, user, settings, search, home, ratings, progress, streaming, transcode, scanner, scheduler, enrichment, events, images, common

```text
internal/domain/<entity>/
├── entity.go       # Domain entity
├── repository.go   # Repository interface
├── types.go        # Enums, value objects
├── errors.go       # Domain errors
└── service.go      # Business logic (optional)
```

### Application (`internal/application/`)

Use cases and services that orchestrate domain logic.

22 modules including: library, media, auth, search, home, ratings, plugins, settings, transcode, trending, etc.

```text
internal/application/<entity>/
├── <verb>_<noun>.go  # Use cases (create_library.go)
├── <entity>_service.go  # Service for CRUD operations
├── dto.go            # Data transfer objects
└── interfaces.go     # Repository interfaces
```

### Infrastructure (`internal/infrastructure/`)

External dependencies: database, FFmpeg, filesystem, streaming.

```text
internal/infrastructure/
├── database/           # Connection, migrations, sqlc
│   ├── queries/sqlite/     # SQLite queries
│   ├── queries/postgres/   # PostgreSQL queries
│   ├── sqlc_sqlite/        # Generated SQLite code
│   ├── sqlc_postgres/      # Generated PostgreSQL code
│   └── unified/            # Unified query router
├── persistence/<entity>/   # Repository implementations
├── ffmpeg/             # FFmpeg wrapper for transcoding
├── filesystem/         # File operations, scanning
└── streaming/          # HLS streaming service
```

### App (`internal/app/`)

Dependency injection and application lifecycle.

```text
internal/app/
├── container.go        # DI container - wires all dependencies
├── lifecycle/          # Startup/shutdown management
├── config/             # Configuration loading
├── handlers/           # Handler factory functions
├── repositories/       # Repository factory functions
├── services/           # Service factory functions
├── usecases/           # Use case factory functions
├── scheduler_setup.go  # Background task registration
└── scheduler_tasks_gen.go  # Generated task registration
```

### API (`internal/api/`)

HTTP handlers using Gin. Handlers call use cases directly (no adapters).

```text
internal/api/
├── handlers/       # HTTP handlers (library.go, media.go, auth.go, home.go...)
├── routes/         # Route registration
└── middleware/     # Auth, CORS, rate limiting, logging
```

## Plugin System

gRPC-based plugin architecture for extensibility.

```text
plugins/                    # Plugin implementations
├── tmdb/                   # Movie/TV metadata
├── musicbrainz/            # Music metadata
├── semantic-search/        # AI-powered search
├── recommendations/        # Personalized recommendations
├── ai-features/            # AI configuration + Ollama
└── ai-provider-*/          # AI providers (Anthropic, OpenAI, Voyage)

pkg/plugin/sdk/             # Plugin SDK for building plugins
api/proto/plugin/           # gRPC protocol definitions
├── plugin_core.proto       # Core plugin lifecycle
├── enricher.proto          # Metadata enrichment
├── search_provider.proto   # Search capabilities
├── trending.proto          # Trending content
└── vector_search.proto     # Vector/semantic search
```

### Plugin Categories

- **Enrichers**: Fetch metadata (TMDb, MusicBrainz)
- **Search Providers**: Semantic search, vector search
- **AI Providers**: Embedding and chat models (Ollama, OpenAI, Anthropic)
- **Recommendations**: Personalized content suggestions
- **Trending**: External trending data sources

## Data Flow

```text
HTTP Request → Handler → Use Case → Domain → Repository → Database
                                                              ↓
HTTP Response ← Handler ← Use Case ← Entity ← Repository ← sqlc

Plugin Flow:
Scanner → Enrichment Queue → Plugin Manager → gRPC → Plugin
                                                        ↓
                          ← Enriched Metadata ← gRPC ← Plugin
```

## Key Patterns

### Repository Pattern

```go
// Domain defines interface
type MediaRepository interface {
    Create(ctx context.Context, media *Media) error
    GetByID(ctx context.Context, id int64) (*Media, error)
}

// Infrastructure implements
type mediaRepository struct {
    router  *unified.QueryRouter
    postgres *sqlc_postgres.Queries
    sqlite   *sqlc_sqlite.Queries
}
```

### Dependency Injection (Container)

```go
// internal/app/container.go
type Container struct {
    // Repositories
    LibraryRepo    library.Repository
    MediaRepo      media.Repository
    // Services
    LibraryService *library.LibraryService
    // Use Cases
    ScanLibrary    *library.ScanOrchestrator
}

func NewContainer(db *sql.DB, config *Config) *Container {
    // Wire everything together
}
```

### Service vs Use Case

- **Service**: CRUD operations combining related methods (`LibraryService`)
- **Use Case**: Single-purpose operation (`GetNextEpisodeUseCase`)

### Error Handling

```go
// Domain errors
var ErrLibraryNotFound = errors.New("library not found")

// Wrap with context
return fmt.Errorf("failed to create: %w", err)

// Map to HTTP status in handlers
func MapErrorToStatus(err error) int {
    switch {
    case errors.Is(err, library.ErrLibraryNotFound):
        return http.StatusNotFound
    default:
        return http.StatusInternalServerError
    }
}
```

## Database

Dual support for SQLite (default) and PostgreSQL:

- Separate query files: `queries/sqlite/`, `queries/postgres/`
- Generated code: `sqlc_sqlite/`, `sqlc_postgres/`
- Unified QueryRouter selects correct implementation at runtime
- Migrations in `migrations/` (SQLite) and `migrations/postgres/`

## Frontend

React + TypeScript with TanStack Router and Query:

```text
web/src/
├── components/       # UI components (876 files)
│   ├── common/       # Shared components
│   ├── home/         # Home screen widgets
│   ├── library/      # Library components
│   └── player/       # Video/audio player
├── routes/           # File-based routing (TanStack Router)
├── lib/
│   ├── api/          # Generated API client
│   └── hooks/        # Custom React hooks
└── styles/           # CSS with design tokens
```

## Related Docs

- [CONVENTIONS.md](../development/CONVENTIONS.md) - Code style
- [decisions/](../decisions/) - Architecture decisions (30 ADRs)
- [../guides/PLUGIN_DEVELOPMENT.md](../guides/PLUGIN_DEVELOPMENT.md) - Building plugins
