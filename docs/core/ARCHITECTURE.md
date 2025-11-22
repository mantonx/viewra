# ViewRA Architecture

## Overview

ViewRA follows Domain-Driven Design (DDD) principles with a clean architecture approach. This ensures separation of concerns, testability, and maintainability as the application scales.

## Architecture Layers

### 1. Domain Layer (`internal/domain/`)

**Purpose**: Core business logic, framework-agnostic

**Responsibilities**:
- Define domain entities
- Define repository interfaces (ports)
- Business rules and validation
- Domain services
- Domain errors

**Rules**:
- ❌ No dependencies on infrastructure
- ❌ No framework-specific code
- ❌ No database logic
- ✅ Pure business logic only

**Structure**:
```
internal/domain/
├── library/
│   ├── entity.go           # Library domain entity
│   ├── repository.go       # Repository interface
│   ├── types.go            # Enums, constants
│   ├── service.go          # Business logic
│   └── errors.go           # Domain-specific errors
├── media/
│   ├── entity.go           # Media base entity
│   ├── repository.go       # Repository interface
│   ├── types.go            # Media types, enums
│   ├── service.go          # Media business logic
│   ├── metadata.go         # Metadata extraction
│   └── errors.go           # Domain errors
├── scanner/
│   ├── types.go            # Scan job types
│   ├── coordinator.go      # Scan coordination
│   └── errors.go           # Scanner errors
└── progress/
    ├── entity.go           # Watch progress entity
    ├── repository.go       # Repository interface
    └── service.go          # Progress tracking logic
```

### 2. Application Layer (`internal/application/`)

**Purpose**: Use cases and application services

**Responsibilities**:
- Orchestrate domain services
- Handle application workflows
- Define DTOs (Data Transfer Objects)
- Transaction management

**Rules**:
- ✅ Uses domain layer
- ✅ Coordinates multiple domain services
- ❌ No HTTP/framework logic
- ❌ No direct database access

**Structure**:
```
internal/application/
├── library/
│   ├── create_library.go      # Use case: Create library
│   ├── scan_library.go        # Use case: Scan library
│   ├── list_libraries.go      # Use case: List libraries
│   ├── dto.go                 # Data transfer objects
│   └── interfaces.go          # Repository interfaces
├── media/
│   ├── get_media.go           # Use case: Get media
│   ├── list_media.go          # Use case: List media
│   ├── dto.go                 # DTOs
│   └── interfaces.go          # Repository interfaces
└── progress/
    ├── update_progress.go     # Use case: Update watch progress
    ├── get_progress.go        # Use case: Get progress
    ├── mark_watched.go        # Use case: Mark as watched
    ├── dto.go                 # DTOs
    └── interfaces.go          # Repository interfaces
```

### 3. Infrastructure Layer (`internal/infrastructure/`)

**Purpose**: External dependencies and implementations

**Responsibilities**:
- Repository implementations
- Database access (sqlc)
- FFmpeg integration
- File system operations

**Rules**:
- ✅ Implements domain interfaces
- ✅ Framework-specific code allowed
- ✅ External library integrations

**Structure**:
```
internal/infrastructure/
├── database/
│   ├── connection.go          # Database connection
│   ├── migrate.go             # Migration runner
│   ├── queries/
│   │   ├── postgres/          # PostgreSQL queries
│   │   └── sqlite/            # SQLite queries
│   ├── sqlc_postgres/         # Generated PostgreSQL code
│   └── sqlc_sqlite/           # Generated SQLite code
├── persistence/
│   ├── library/
│   │   └── repository.go      # Library repository impl
│   ├── media/
│   │   └── repository.go      # Media repository impl
│   ├── progress/
│   │   └── repository.go      # Progress repository impl
│   ├── scanjob/
│   │   └── repository.go      # Scan job repository impl
│   └── common/
│       └── helpers.go         # Conversion utilities
├── ffmpeg/
│   ├── client.go              # FFmpeg wrapper
│   └── metadata.go            # Metadata extraction
├── filesystem/
│   ├── scanner.go             # Directory scanner
│   └── coordinator.go         # Scan coordinator
├── pathbrowser/
│   ├── browser.go             # Filesystem browser
│   └── validator.go           # Path validation
└── streaming/
    ├── server.go              # HTTP range request handler
    └── range.go               # Range parsing
```

### 4. API Layer (`internal/api/`)

**Purpose**: HTTP REST API

**Responsibilities**:
- HTTP handlers
- Request/response mapping
- Route registration
- Error handling and HTTP status mapping
- Swagger annotations

**Rules**:
- ✅ Uses application layer use cases directly
- ✅ Framework-specific (Gin)
- ✅ API documentation with Swagger
- ❌ No business logic
- ❌ No adapter/wrapper layers (handlers call use cases directly)

**Structure**:
```
internal/api/
├── server.go                  # Gin server setup
├── handlers/
│   ├── library.go             # Library endpoints
│   ├── media.go               # Media endpoints
│   ├── progress.go            # Progress endpoints
│   ├── browser.go             # Filesystem browser
│   ├── scanjob.go             # Scan job status
│   └── helpers.go             # Shared handler utilities
├── middleware/
│   └── cors.go                # CORS handling
└── routes/
    ├── library.go             # Library routes
    ├── media.go               # Media routes
    ├── progress.go            # Progress routes
    └── browser.go             # Browser routes
```

**Design Principles**:
1. **No Adapters**: Handlers hold individual use case pointers and call them directly
2. **Route Organization**: Routes live in separate files for scalability
3. **Error Mapping**: Centralized domain error → HTTP status code mapping in `handlers/helpers.go`
4. **Type Safety**: All DTOs defined in application layer, validated at handler level

### 5. Shared Package (`internal/pkg/`)

**Purpose**: Shared utilities and cross-cutting concerns

**Rules**:
- ✅ Use for utilities shared across multiple domains
- ✅ Keep utilities small and focused (single responsibility)
- ❌ Don't put business logic here
- ❌ Don't create "utils" or "helpers" catch-all packages

**Note**: Keep domain-specific utilities in their domain package (e.g., `internal/domain/media/metadata.go` for media-specific logic)

## Data Flow

### Request Flow (Read Operation)
```
HTTP Request → Handler → Use Case → Domain Service → Repository → Database
                                                                      ↓
HTTP Response ← Handler ← Use Case ← Domain Entity ← Repository ← sqlc
```

### Write Operation with Validation
```
HTTP Request → Handler (validate request)
                 ↓
             Use Case (orchestrate)
                 ↓
        Domain Service (business rules)
                 ↓
         Domain Entity (validate)
                 ↓
            Repository (persist)
                 ↓
              Database
```

## Dependency Flow

```
┌─────────────────────────────────────────────┐
│           API Layer (HTTP)                  │
│         ↓ depends on ↓                      │
│         Application Layer                    │
│         ↓ depends on ↓                      │
│           Domain Layer                       │
│         ↑ implements ↑                      │
│      Infrastructure Layer                    │
└─────────────────────────────────────────────┘
```

**Key Principle**: Dependencies point inward. Domain has zero external dependencies.

## Key Design Patterns

### 1. Repository Pattern
```go
// Domain defines interface
type MediaRepository interface {
    Create(ctx context.Context, media *Media) error
    GetByID(ctx context.Context, id int64) (*Media, error)
    List(ctx context.Context, filter Filter) ([]*Media, error)
}

// Infrastructure implements
type mediaRepository struct {
    postgres *sqlc_postgres.Queries
    sqlite   *sqlc_sqlite.Queries
    router   *adapters.QueryRouter
}

func (r *mediaRepository) Create(ctx context.Context, media *Media) error {
    // Dual database support via router
}
```

### 2. Service Pattern
```go
// Domain service
type MediaService struct {
    repo       MediaRepository
}

func (s *MediaService) ProcessScanResult(ctx context.Context, result *ScanResult) (*Media, error) {
    // Business logic
    // Validation
    // Create entity
    return s.repo.Create(ctx, media)
}
```

### 3. Dependency Injection
```go
// Wire dependencies in main.go
func main() {
    // Infrastructure
    db := database.Connect()

    // Repositories
    mediaRepo := persistence.NewMediaRepository(db)
    libraryRepo := persistence.NewLibraryRepository(db)

    // Use Cases
    getMedia := application.NewGetMedia(mediaRepo)
    listLibraries := application.NewListLibraries(libraryRepo)

    // Handlers
    mediaHandler := handlers.NewMediaHandler(getMedia, listMedia)
    libraryHandler := handlers.NewLibraryHandler(listLibraries, ...)
}
```

## Database Design

ViewRA uses a **hybrid schema approach** with dual database support (SQLite and PostgreSQL):

- **Base `media` table**: Common fields for all media types
- **Type-specific tables**: `movies`, `tv_episodes` for type-specific fields
- **Dual database support**: Separate query files and generated code for each DB
- **SQLC code generation**: Type-safe SQL queries

See **[DATABASE_SCHEMA.md](./DATABASE_SCHEMA.md)** for complete schema definitions, table structures, and query patterns.

## Code Organization Guidelines

### File Naming Conventions

#### Domain Layer
```
internal/domain/<domain>/
├── entity.go           # Main entity definition
├── types.go            # Enums, constants, value objects
├── repository.go       # Repository interface
├── service.go          # Domain service
├── errors.go           # Domain-specific errors
└── *_test.go           # Tests
```

#### Application Layer
- **Pattern**: `<verb>_<noun>.go` for use cases
- **Example**: `create_library.go`, `scan_library.go`, `update_progress.go`
- **DTOs**: All in `dto.go` per domain
- **Interfaces**: Repository interfaces in `interfaces.go`

#### Infrastructure Layer
```
internal/infrastructure/persistence/<domain>/
├── repository.go       # Repository implementation
├── helpers.go          # Conversion functions
└── *_test.go           # Integration tests
```

#### API Layer
```
internal/api/handlers/
├── <domain>.go         # Handler for domain
└── helpers.go          # Shared handler utilities

internal/api/routes/
└── <domain>.go         # Route registration
```

### Package Organization Best Practices

#### When to Create New Domain Package
- Feature has its own entities
- Feature has distinct business rules
- Feature can be reasoned about independently

#### When to Split Large Domains
When a domain grows beyond 10 files, consider sub-packages:
```
internal/domain/media/
├── entity.go
├── repository.go
├── service.go
├── metadata/           # Sub-package for metadata
│   └── detector.go
└── *_test.go
```

#### Avoid Anti-Patterns
```
❌ internal/managers/      # Too vague
❌ internal/helpers/        # Catch-all
❌ internal/utils/          # Too generic
❌ internal/common/         # What does this mean?

✅ internal/infrastructure/persistence/common/   # Specific: DB conversion helpers
✅ internal/domain/media/                        # Domain-driven
```

### Dependency Management

#### Import Order
```go
import (
    // 1. Standard library
    "context"
    "fmt"

    // 2. External dependencies (alphabetically)
    "github.com/gin-gonic/gin"

    // 3. Internal packages (alphabetically)
    "github.com/yourusername/viewra2/internal/domain/library"
)
```

Use `goimports` to automatically format imports correctly.

#### Avoid Circular Dependencies
```
❌ BAD: media → library → media (circular!)

✅ GOOD: Extract shared concepts or use interfaces
```

**Solutions**:
1. Extract shared concepts to separate package
2. Use domain events (future)
3. Rethink the relationship (use interfaces)

### Constants & Configuration

#### Domain Constants
Keep constants with the domain they belong to:
```go
// internal/domain/media/types.go
package media

type MediaType string

const (
    MediaTypeMovie      MediaType = "movie"
    MediaTypeTVEpisode  MediaType = "tv_episode"
)
```

### Code Growth Triggers

**When to refactor**:

| Trigger | Action |
|---------|--------|
| File > 500 lines | Split into multiple files by responsibility |
| Package > 10 files | Consider sub-packages |
| Duplicated code (3+ times) | Extract to utility or domain service |
| Complex initialization | Use builder or factory pattern |
| Too many dependencies | Split component, use interface segregation |

### Documentation Standards

#### Package Documentation
```go
// Package media handles all media-related business logic including
// movies, TV episodes, and music tracks.
package media
```

#### Exported Function Documentation
```go
// CreateLibrary creates a new media library after validating the path exists
// and is accessible. Returns ErrInvalidPath if the path is invalid.
func (s *Service) CreateLibrary(ctx context.Context, lib *Library) error {
    // ...
}
```

## Testing Strategy

### Test Levels
- **Unit Tests**: Domain layer (pure business logic), application services, utility functions
- **Integration Tests**: Repository implementations, FFmpeg integration, file system operations
- **E2E Tests**: API endpoints, full workflows (scan, stream)

### Test Organization

#### Test File Placement
Tests live next to implementation:
```
internal/domain/media/
├── entity.go
├── entity_test.go      # Next to implementation
├── service.go
├── service_test.go
└── testdata/           # Test fixtures
    ├── sample.mp4
    └── sample.json
```

#### Integration Tests
```
tests/
├── integration/
│   ├── api_test.go         # API integration tests
│   └── scanner_test.go     # Scanner integration tests
└── e2e/
    └── library_flow_test.go
```

See **[TESTING.md](./TESTING.md)** for complete testing guidelines, current coverage, and test writing patterns.

## Error Handling

### Strategy: Sentinel Errors + Wrapped Context

**Pattern**: Use sentinel errors for type checking with wrapped context for debugging.

### Domain Errors

Define sentinel errors in domain layer:

```go
// internal/domain/library/errors.go
package library

import "errors"

var (
    ErrLibraryNotFound = errors.New("library not found")
    ErrInvalidPath     = errors.New("invalid path")
    ErrDuplicatePath   = errors.New("duplicate library path")
)
```

### Error Wrapping

Always wrap errors with context using `%w`:

```go
if !filepath.IsAbs(lib.Path) {
    return fmt.Errorf("library path must be absolute: %w", ErrInvalidPath)
}

if err := s.repo.Create(ctx, lib); err != nil {
    return fmt.Errorf("failed to create library: %w", err)
}
```

### HTTP Error Mapping

Handlers map domain errors to HTTP status codes:

```go
// internal/api/handlers/helpers.go
func MapErrorToStatus(err error) int {
    switch {
    case errors.Is(err, library.ErrLibraryNotFound):
        return http.StatusNotFound
    case errors.Is(err, library.ErrInvalidPath):
        return http.StatusBadRequest
    default:
        return http.StatusInternalServerError
    }
}
```

## Logging Strategy

### Structured Logging (slog)
```go
logger.Info("scanning library",
    "library_id", libraryID,
    "path", path,
    "files_found", count,
)

logger.Error("transcode failed",
    "media_id", mediaID,
    "error", err,
)
```

### Log Levels
- `debug` - Development, verbose
- `info` - Production default
- `warn` - Warnings, degraded performance
- `error` - Errors, failures

## Security Considerations

### Current Implementation (Single-User)
- CORS validation
- Path traversal prevention (absolute paths only, no `..`)
- Input validation and sanitization
- SQL injection prevention (sqlc prepared statements)

### Future Considerations
For multi-user deployments, see **SCALING.md** for authentication, authorization, and rate limiting strategies.

## API Handler Structure

### Organized by Domain
```go
// internal/api/handlers/library.go
type LibraryHandler struct {
    createLibrary *application.CreateLibraryUseCase
    listLibraries *application.ListLibrariesUseCase
    scanLibrary   *application.ScanLibraryUseCase
}

func NewLibraryHandler(
    create *application.CreateLibraryUseCase,
    list *application.ListLibrariesUseCase,
    scan *application.ScanLibraryUseCase,
) *LibraryHandler {
    return &LibraryHandler{
        createLibrary: create,
        listLibraries: list,
        scanLibrary:   scan,
    }
}

func (h *LibraryHandler) Create(c *gin.Context) {
    var req dto.CreateLibraryRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    result, err := h.createLibrary.Execute(c.Request.Context(), req)
    if err != nil {
        status := helpers.MapErrorToStatus(err)
        c.JSON(status, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusCreated, result)
}
```

### Router Setup
```go
// internal/api/routes/library.go
func RegisterLibraryRoutes(r *gin.Engine, h *handlers.LibraryHandler) {
    libraries := r.Group("/api/libraries")
    {
        libraries.GET("", h.List)
        libraries.POST("", h.Create)
        libraries.GET("/:id", h.Get)
        libraries.PUT("/:id", h.Update)
        libraries.DELETE("/:id", h.Delete)
        libraries.POST("/:id/scan", h.Scan)
    }
}
```

## Frontend Architecture

The frontend uses a component-based architecture with:
- **React 19** + TypeScript
- **TanStack Router** (file-based routing)
- **TanStack Query** (server state)
- **Tailwind CSS** + Shadcn/ui (styling)

**Structure**:
```
web/src/
├── components/           # React components
│   ├── ui/              # Reusable UI components
│   ├── library/         # Library-specific components
│   └── media/           # Media-specific components
├── lib/
│   ├── api/            # API client (Orval-generated)
│   ├── hooks/          # Custom hooks
│   └── utils/          # Utilities
├── routes/             # TanStack Router (file-based)
│   ├── __root.tsx
│   ├── _layout/
│   │   ├── libraries.tsx
│   │   └── media.tsx
│   └── index.tsx
└── contexts/           # React contexts
```

## Additional Documentation

- **[API_SPECIFICATION.md](./API_SPECIFICATION.md)** - Complete API endpoint documentation
- **[DATABASE_SCHEMA.md](./DATABASE_SCHEMA.md)** - Database schema, migrations, queries
- **[TESTING.md](./TESTING.md)** - Testing strategy and coverage
- **[CONVENTIONS.md](./CONVENTIONS.md)** - Coding standards and file naming
- **[QUICK_REFERENCE.md](./QUICK_REFERENCE.md)** - Development workflow cheat sheet
- **SCALING.md** - Horizontal scaling, performance optimization, multi-user considerations

**Last Updated**: 2025-11-12
