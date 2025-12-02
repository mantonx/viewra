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
```

## Layers

### Domain (`internal/domain/`)

Core business logic. **No external dependencies** - only standard library.

```text
internal/domain/<entity>/
├── entity.go       # Domain entity
├── repository.go   # Repository interface
├── types.go        # Enums, value objects
├── errors.go       # Domain errors
└── service.go      # Business logic (optional)
```

### Application (`internal/application/`)

Use cases that orchestrate domain logic.

```text
internal/application/<entity>/
├── <verb>_<noun>.go  # Use cases (create_library.go, scan_library.go)
├── dto.go            # Data transfer objects
└── interfaces.go     # Repository interfaces
```

### Infrastructure (`internal/infrastructure/`)

External dependencies: database, FFmpeg, filesystem.

```text
internal/infrastructure/
├── database/           # Connection, migrations, sqlc
├── persistence/<entity>/  # Repository implementations
├── ffmpeg/             # FFmpeg wrapper
└── filesystem/         # File operations
```

### API (`internal/api/`)

HTTP handlers using Gin. Handlers call use cases directly (no adapters).

```text
internal/api/
├── handlers/    # HTTP handlers
├── routes/      # Route registration
└── middleware/  # CORS, logging
```

## Data Flow

```text
HTTP Request → Handler → Use Case → Domain → Repository → Database
                                                              ↓
HTTP Response ← Handler ← Use Case ← Entity ← Repository ← sqlc
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
    postgres *sqlc_postgres.Queries
    sqlite   *sqlc_sqlite.Queries
}
```

### Dependency Injection

```go
// Wire in main.go
mediaRepo := persistence.NewMediaRepository(db)
getMedia := application.NewGetMedia(mediaRepo)
handler := handlers.NewMediaHandler(getMedia)
```

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
- QueryRouter selects correct implementation at runtime

## Frontend

React + TypeScript with TanStack Router and Query:

```text
web/src/
├── components/   # UI components
├── routes/       # File-based routing
├── lib/api/      # Generated API client
└── lib/hooks/    # Custom hooks
```

## Related Docs

- [CONVENTIONS.md](../development/CONVENTIONS.md) - Code style
- [decisions/](../decisions/) - Architecture decisions
