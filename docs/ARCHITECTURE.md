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
│   ├── service.go          # Business logic
│   └── errors.go           # Domain-specific errors
├── media/
│   ├── entity.go           # Media base entity
│   ├── repository.go       # Repository interface
│   ├── service.go          # Media business logic
│   ├── scanner.go          # Scanning logic
│   └── metadata.go         # Metadata extraction interface
└── watch/
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
│   └── dto.go                 # Data transfer objects
├── media/
│   ├── get_media.go           # Use case: Get media
│   ├── stream_media.go        # Use case: Stream media
│   ├── transcode_media.go     # Use case: Transcode media
│   ├── search_media.go        # Use case: Search media
│   └── dto.go                 # DTOs
└── watch/
    ├── update_progress.go     # Use case: Update watch progress
    ├── get_progress.go        # Use case: Get progress
    └── dto.go                 # DTOs
```

### 3. Infrastructure Layer (`internal/infrastructure/`)

**Purpose**: External dependencies and implementations

**Responsibilities**:
- Repository implementations
- Database access (sqlc)
- FFmpeg integration
- File system operations
- Caching

**Rules**:
- ✅ Implements domain interfaces
- ✅ Framework-specific code allowed
- ✅ External library integrations

**Structure**:
```
internal/infrastructure/
├── database/
│   ├── migrations/
│   │   ├── 000001_init.up.sql
│   │   ├── 000001_init.down.sql
│   │   ├── 000002_add_tv_shows.up.sql
│   │   ├── 000002_add_tv_shows.down.sql
│   │   └── 000003_add_music.up.sql
│   ├── sqlc/
│   │   ├── schema.sql         # Schema for sqlc
│   │   ├── queries.sql        # SQL queries
│   │   └── db/                # Generated code
│   ├── connection.go          # Database connection
│   ├── migrate.go             # Migration runner
│   └── repository/
│       ├── library.go         # Library repository impl
│       ├── media.go           # Media repository impl
│       ├── movie.go           # Movie repository impl
│       ├── tv.go              # TV repository impl
│       ├── music.go           # Music repository impl
│       └── watch.go           # Watch progress impl
├── ffmpeg/
│   ├── client.go              # FFmpeg wrapper
│   ├── metadata.go            # Metadata extraction
│   ├── thumbnail.go           # Thumbnail generation
│   ├── transcoder.go          # DASH transcoding
│   └── probe.go               # File probing
├── filesystem/
│   ├── watcher.go             # fsnotify wrapper
│   ├── scanner.go             # Directory scanner
│   └── validator.go           # Path validation
├── storage/
│   ├── local.go               # Local file storage
│   └── paths.go               # Path management
└── queue/
    ├── job.go                 # Job definition
    ├── worker.go              # Job worker
    └── transcode_queue.go     # Transcoding queue
```

### 4. API Layer (`internal/api/`)

**Purpose**: HTTP REST API (renamed from `internal/interfaces/` for Go-idiomatic naming)

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

**Current Structure** (Phase 1.8):

```text
internal/api/
├── server.go                  # Gin server setup, lifecycle, health check
├── handlers/                  # HTTP request handlers
│   ├── library.go             # Library handler (6 methods)
│   ├── media.go               # Media handler (2 methods, read-only)
│   ├── stream.go              # Streaming handler with range support ✅
│   └── errors.go              # Domain error → HTTP status mapping
└── routes/                    # Route registration (scalable organization)
    ├── library.go             # Library routes (6 endpoints)
    ├── media.go               # Media routes (2 endpoints)
    └── stream.go              # Streaming route (1 endpoint) ✅
```

**Future Expansion**:

```text
internal/api/
├── server.go
├── handlers/
│   ├── library.go
│   ├── media.go
│   ├── stream.go              # ✅ Phase 1.8
│   ├── thumbnail.go           # Thumbnail handler (Phase 2)
│   ├── errors.go
│   └── sse.go                 # SSE progress updates (Phase 2)
├── routes/
│   ├── library.go
│   ├── media.go
│   ├── stream.go              # ✅ Phase 1.8
│   └── thumbnail.go           # Thumbnail routes (Phase 2)
└── middleware/                # Future middleware (Phase 2)
    ├── cors.go
    ├── logging.go
    └── recovery.go
```

**Design Principles**:

1. **No Adapters**: Handlers hold individual use case pointers and call them directly (see Rule 5 in `.agent.md`)
2. **Route Organization**: Routes live in separate files for scalability
3. **Error Mapping**: Centralized domain error → HTTP status code mapping in `handlers/errors.go`
4. **Type Safety**: All DTOs defined in application layer, validated at handler level

### 5. Shared Package (`internal/pkg/`)

**Purpose**: Shared utilities and cross-cutting concerns

**Rules**:
- ✅ Use for utilities shared across multiple domains
- ✅ Keep utilities small and focused (single responsibility)
- ✅ Wrap external libraries for easier testing
- ❌ Don't put business logic here
- ❌ Don't create "utils" or "helpers" catch-all packages

**Structure**:
```
internal/pkg/
├── config/
│   ├── config.go              # Configuration struct
│   ├── loader.go              # Load from env/file
│   └── validator.go           # Validate config
├── logger/
│   └── logger.go              # Structured logging wrapper
├── errors/
│   ├── errors.go              # Custom error types
│   └── handler.go             # HTTP error mapping
├── validator/
│   ├── path.go                # Path validation
│   └── url.go                 # URL validation
├── fileutil/
│   ├── hash.go                # File hashing (SHA256)
│   ├── size.go                # Human-readable sizes
│   └── exists.go              # File existence checks
├── stringutil/
│   ├── slug.go                # Slugify, sanitize
│   └── trim.go                # Custom trim functions
├── timeutil/
│   └── parse.go               # Custom time parsing
└── cache/                     # Cache abstraction (future)
    ├── cache.go               # Interface
    └── memory.go              # In-memory implementation
```

**Note**: Keep domain-specific utilities in their domain package (e.g., `internal/domain/media/filename.go` for movie filename parsing)

## Data Flow

### Request Flow (Read Operation)
```
HTTP Request
    ↓
[Middleware]
    ↓
[HTTP Handler] → converts to DTO
    ↓
[Application Service] → validates, orchestrates
    ↓
[Domain Service] → business logic
    ↓
[Repository Interface]
    ↓
[Repository Implementation] → sqlc queries
    ↓
[Database]
    ↓
[Domain Entity] ← returned
    ↓
[DTO] ← mapped
    ↓
[HTTP Response]
```

### Scan Flow (Write Operation)
```
User clicks "Scan Library"
    ↓
[HTTP Handler] → POST /api/libraries/:id/scan
    ↓
[Application Service: ScanLibrary]
    ↓
├─→ [Domain: Library Service] → validate library exists
├─→ [Infrastructure: FileSystem Scanner] → list files
├─→ [Infrastructure: FFmpeg] → extract metadata
├─→ [Domain: Media Service] → create media entities
├─→ [Repository] → save to database
└─→ [SSE] → emit progress updates
```

### Transcode Flow (Background Job)
```
Media needs transcoding
    ↓
[Application Service] → queue job
    ↓
[Transcode Queue] → add to channel
    ↓
[Worker Goroutine]
    ↓
├─→ [FFmpeg Transcoder] → generate 360p (fast)
├─→ [Repository] → update status
├─→ [SSE] → emit progress
├─→ [FFmpeg Transcoder] → generate 720p (background)
├─→ [FFmpeg Transcoder] → generate 1080p (background)
└─→ [Repository] → mark complete
```

## Dependency Flow

```
┌─────────────────────────────────────────────┐
│           Interfaces (HTTP/CLI)             │
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
    queries *db.Queries
}

func (r *mediaRepository) Create(ctx context.Context, media *Media) error {
    // sqlc implementation
}
```

### 2. Service Pattern
```go
// Domain service
type MediaService struct {
    repo       MediaRepository
    metadataExtractor MetadataExtractor
}

func (s *MediaService) CreateMedia(ctx context.Context, path string) (*Media, error) {
    // Business logic
    metadata, err := s.metadataExtractor.Extract(path)
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
    queries := db.New(db)
    
    // Repositories
    mediaRepo := repository.NewMediaRepository(queries)
    
    // Services
    mediaSvc := domain.NewMediaService(mediaRepo, ffmpegClient)
    
    // Application
    getMediaUseCase := application.NewGetMedia(mediaSvc)
    
    // Handlers
    mediaHandler := handlers.NewMediaHandler(getMediaUseCase)
}
```

## Database Schema Design

### Hybrid Schema Approach

**Base Table** (common fields):
```sql
CREATE TABLE media (
    id INTEGER PRIMARY KEY,
    library_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    file_path TEXT UNIQUE NOT NULL,
    file_size INTEGER,
    duration REAL,
    type TEXT NOT NULL, -- 'movie', 'tv_episode', 'music_track'
    created_at DATETIME,
    updated_at DATETIME,
    FOREIGN KEY (library_id) REFERENCES libraries(id)
);
```

**Specific Tables** (type-specific fields):
```sql
CREATE TABLE movies (
    media_id INTEGER PRIMARY KEY,
    year INTEGER,
    genre TEXT,
    director TEXT,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);

CREATE TABLE tv_episodes (
    media_id INTEGER PRIMARY KEY,
    show_id INTEGER NOT NULL,
    season INTEGER NOT NULL,
    episode INTEGER NOT NULL,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE,
    FOREIGN KEY (show_id) REFERENCES tv_shows(id)
);

CREATE TABLE music_tracks (
    media_id INTEGER PRIMARY KEY,
    artist TEXT,
    album TEXT,
    track_number INTEGER,
    FOREIGN KEY (media_id) REFERENCES media(id) ON DELETE CASCADE
);
```

## Scalability Considerations

### Horizontal Scaling
- Stateless API servers
- Shared database (PostgreSQL)
- Shared file storage (NFS/S3)
- Redis for distributed caching

### Performance
- Database indexes on frequently queried fields
- Connection pooling
- Query optimization with sqlc
- Caching layer (future)

### Future Enhancements
- Message queue for distributed jobs (RabbitMQ/NATS)
- CDN for static assets
- Read replicas for database
- Microservices (if needed)

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
├── validator.go        # Validation logic
└── events.go           # Domain events (future)
```

#### Application Layer
- **Pattern**: `<verb>_<noun>.go` for use cases
- **Example**: `create_library.go`, `scan_library.go`, `update_progress.go`
- **DTOs**: All in `dto.go` per domain

#### Infrastructure Layer
```
internal/infrastructure/database/
├── connection.go       # DB connection setup
├── migrate.go          # Migration runner
├── repository/
│   └── <domain>.go     # One file per domain repository
└── sqlc/
    ├── schema.sql
    └── queries/
        └── <domain>.sql
```

#### Interface Layer
```
internal/interfaces/http/handlers/<domain>/
├── handler.go          # Handler struct + constructor
├── routes.go           # Route registration
├── dto.go              # API-specific DTOs
└── response.go         # Response helpers
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
├── scanner/            # Sub-package for scanning logic
│   ├── scanner.go
│   └── parser.go
├── metadata/           # Sub-package for metadata
│   └── extractor.go
└── transcode/          # Sub-package for transcoding
    └── job.go
```

#### Avoid Anti-Patterns
```
❌ internal/managers/      # Too vague
❌ internal/helpers/        # Catch-all
❌ internal/utils/          # Too generic
❌ internal/common/         # What does this mean?

✅ internal/pkg/fileutil/   # Specific purpose
✅ internal/pkg/validator/  # Clear responsibility
✅ internal/domain/media/   # Domain-driven
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
    "github.com/yourusername/viewra2/internal/pkg/fileutil"
)
```

Use `goimports` to automatically format imports correctly.

#### Avoid Circular Dependencies
```
❌ BAD: media → library → media (circular!)

✅ GOOD: Extract shared concepts or use events
```

**Solutions**:
1. Extract shared concepts to `internal/pkg/`
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
    MediaTypeMusicTrack MediaType = "music_track"
)
```

#### Application Configuration
Centralized in `internal/pkg/config/`:
```go
type Config struct {
    Server   ServerConfig
    Database DatabaseConfig
    FFmpeg   FFmpegConfig
    Storage  StorageConfig
}
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
// movies, TV episodes, and music tracks. It provides scanning,
// metadata extraction, and transcoding capabilities.
package media
```

#### Exported Function Documentation
```go
// CreateLibrary creates a new media library after validating the path exists
// and is accessible. Returns ErrInvalidPath if the path is invalid or
// ErrDuplicatePath if a library with the same path already exists.
func (s *Service) CreateLibrary(ctx context.Context, lib *Library) error {
    // ...
}
```

## Testing Strategy

### Unit Tests
- Domain layer (pure business logic)
- Application services
- Utility functions

### Integration Tests
- Repository implementations
- FFmpeg integration
- File system operations

### E2E Tests
- API endpoints
- Full workflows (scan, transcode, stream)

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

#### Test Helpers
```
internal/testutil/
├── database.go         # Test DB setup
├── fixtures.go         # Common test data
├── assert.go           # Custom assertions
└── mock/               # Mock implementations
    ├── repository.go
    └── service.go
```

#### Integration Tests
```
tests/
├── integration/
│   ├── api_test.go         # API integration tests
│   ├── scanner_test.go     # Scanner integration tests
│   └── transcode_test.go   # Transcode integration tests
└── e2e/
    ├── library_flow_test.go
    └── watch_flow_test.go
```

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

### Error Wrapping in Services

Always wrap errors with context using `%w`:

```go
// internal/domain/library/service.go
func (s *LibraryService) CreateLibrary(ctx context.Context, lib *Library) error {
    // Validate path
    if !filepath.IsAbs(lib.Path) {
        return fmt.Errorf("library path must be absolute: %w", ErrInvalidPath)
    }
    
    // Check path exists
    if _, err := os.Stat(lib.Path); err != nil {
        return fmt.Errorf("path does not exist or is not accessible: %w", ErrInvalidPath)
    }
    
    // Create library
    if err := s.repo.Create(ctx, lib); err != nil {
        if errors.Is(err, repository.ErrDuplicate) {
            return fmt.Errorf("library already exists at %s: %w", lib.Path, ErrDuplicatePath)
        }
        return fmt.Errorf("failed to create library: %w", err)
    }
    
    return nil
}
```

### Infrastructure Layer - Translate Database Errors

```go
// internal/infrastructure/database/repository/library.go
var ErrDuplicate = errors.New("duplicate entry")

func (r *libraryRepository) Create(ctx context.Context, lib *Library) error {
    _, err := r.queries.InsertLibrary(ctx, ...)
    if err != nil {
        if isSQLiteConstraintError(err) {
            return ErrDuplicate
        }
        return fmt.Errorf("database error creating library: %w", err)
    }
    return nil
}
```

### HTTP Layer - Check Error Types

Use `errors.Is()` to check error types and return appropriate HTTP status codes:

```go
// internal/interfaces/http/handlers/library/handler.go
func (h *LibraryHandler) CreateLibrary(c *gin.Context) {
    err := h.createLibraryUseCase.Execute(ctx, req)
    if err != nil {
        if errors.Is(err, domain.ErrInvalidPath) {
            c.JSON(400, gin.H{
                "error": gin.H{
                    "code":    "INVALID_PATH",
                    "message": err.Error(),
                },
            })
            return
        }
        if errors.Is(err, domain.ErrDuplicatePath) {
            c.JSON(409, gin.H{
                "error": gin.H{
                    "code":    "DUPLICATE_PATH",
                    "message": err.Error(),
                },
            })
            return
        }
        // Unknown error
        logger.Error("failed to create library", "error", err, "request_id", requestID)
        c.JSON(500, gin.H{
            "error": gin.H{
                "code":    "INTERNAL_ERROR",
                "message": "Failed to create library",
            },
        })
        return
    }
    c.JSON(201, response)
}
```

### Error Logging

Log errors at the layer where they're handled:

- **Domain/Application**: Don't log (errors bubble up)
- **Infrastructure**: Log unexpected errors (DB failures, external service errors)
- **HTTP Handlers**: Log all errors with request context

```go
logger.Error("operation failed",
    "error", err,
    "request_id", requestID,
    "library_id", libraryID,
)

## Validation Strategy

### Multi-Layer Validation

Different layers validate different concerns:

#### API Layer (HTTP Handlers)

**Purpose**: Validate request format and basic constraints

- Request structure (valid JSON)
- Required fields present
- Field types correct
- Basic format validation (email format, etc.)

**Fast-fail**: Reject invalid requests immediately without hitting business logic.

```go
// internal/interfaces/http/handlers/library/handler.go
func (h *LibraryHandler) CreateLibrary(c *gin.Context) {
    var req CreateLibraryRequest
    
    // Validate request structure
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{
            "error": gin.H{
                "code":    "INVALID_REQUEST",
                "message": "Invalid request format",
                "fields":  parseValidationErrors(err),
            },
        })
        return
    }
    
    // Basic field validation
    if req.Name == "" || req.Path == "" {
        c.JSON(400, gin.H{
            "error": gin.H{
                "code": "VALIDATION_ERROR",
                "fields": gin.H{
                    "name": "name is required",
                    "path": "path is required",
                },
            },
        })
        return
    }
    
    // Pass to use case...
}
```

#### Domain Layer (Services)

**Purpose**: Validate business rules

- Path exists and is readable
- No duplicate entries
- Valid permissions
- Business constraints

```go
// internal/domain/library/service.go
func (s *LibraryService) CreateLibrary(ctx context.Context, lib *Library) error {
    // Validate path is absolute
    if !filepath.IsAbs(lib.Path) {
        return fmt.Errorf("path must be absolute: %w", ErrInvalidPath)
    }
    
    // Validate path exists and is accessible
    if _, err := os.Stat(lib.Path); err != nil {
        return fmt.Errorf("path does not exist or is not accessible: %w", ErrInvalidPath)
    }
    
    // Check path is readable
    if err := checkPathReadable(lib.Path); err != nil {
        return fmt.Errorf("path is not readable: %w", ErrInvalidPath)
    }
    
    // Prevent path traversal
    cleanPath := filepath.Clean(lib.Path)
    if strings.Contains(cleanPath, "..") {
        return fmt.Errorf("path contains invalid traversal: %w", ErrInvalidPath)
    }
    
    // Check for duplicate path
    existing, err := s.repo.FindByPath(ctx, lib.Path)
    if err == nil && existing != nil {
        return fmt.Errorf("library already exists at path %s: %w", lib.Path, ErrDuplicatePath)
    }
    
    return s.repo.Create(ctx, lib)
}
```

#### Infrastructure Layer

**Purpose**: Database constraints only

- Unique constraints
- Foreign key constraints
- NOT NULL constraints

Let the database enforce these, translate DB errors to domain errors.

---

## Transaction Patterns

### Batch Transactions for Bulk Operations

When inserting many records (e.g., scanning library), use batched transactions:

**Pattern**: Process 10-20 records per transaction

```go
// internal/application/library/scan_library.go
func (uc *ScanLibrary) Execute(ctx context.Context, libraryID int64) error {
    files := uc.scanner.FindMediaFiles(libraryPath)
    
    const batchSize = 20
    batch := make([]*domain.Media, 0, batchSize)
    
    for _, file := range files {
        // Extract metadata
        media, err := uc.metadataExtractor.Extract(file)
        if err != nil {
            logger.Warn("failed to extract metadata",
                "file", file,
                "error", err,
            )
            continue
        }
        
        batch = append(batch, media)
        
        // Commit batch when full
        if len(batch) >= batchSize {
            if err := uc.repo.CreateBatch(ctx, batch); err != nil {
                return fmt.Errorf("failed to save batch: %w", err)
            }
            batch = batch[:0]  // Reset
        }
    }
    
    // Save remaining items
    if len(batch) > 0 {
        return uc.repo.CreateBatch(ctx, batch)
    }
    
    return nil
}
```

**Repository Implementation**:

```go
// internal/infrastructure/database/repository/media.go
func (r *mediaRepository) CreateBatch(ctx context.Context, mediaList []*domain.Media) error {
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    defer tx.Rollback()
    
    queries := r.queries.WithTx(tx)
    
    for _, media := range mediaList {
        if err := queries.InsertMedia(ctx, ...); err != nil {
            return fmt.Errorf("failed to insert media: %w", err)
        }
    }
    
    if err := tx.Commit(); err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }
    
    return nil
}
```

**Why batching:**
- ✅ Not too many small transactions (slow)
- ✅ Not one giant transaction (locks table too long)
- ✅ Good error recovery (partial progress saved)
- ✅ Reasonable lock duration

---

## Concurrency Patterns

### Worker Pool for File Scanning

Use worker pool with configurable concurrency based on system resources:

```go
// internal/infrastructure/filesystem/scanner.go
type Scanner struct {
    numWorkers int
}

func NewScanner() *Scanner {
    // Default: runtime.NumCPU() workers
    // Override with env var: SCAN_WORKERS
    workers := runtime.NumCPU()
    if override := os.Getenv("SCAN_WORKERS"); override != "" {
        if w, err := strconv.Atoi(override); err == nil && w > 0 {
            workers = w
        }
    }
    
    return &Scanner{
        numWorkers: workers,
    }
}

func (s *Scanner) ScanDirectory(ctx context.Context, path string) ([]*FileInfo, error) {
    files := make(chan string, 100)
    results := make(chan *FileInfo, 100)
    errors := make(chan error, s.numWorkers)
    
    var wg sync.WaitGroup
    
    // Start workers
    for i := 0; i < s.numWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for filePath := range files {
                select {
                case <-ctx.Done():
                    return
                default:
                    info, err := s.processFile(filePath)
                    if err != nil {
                        errors <- err
                        continue
                    }
                    results <- info
                }
            }
        }()
    }
    
    // Producer: walk directory
    go func() {
        filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
            if err != nil {
                return err
            }
            if !info.IsDir() {
                select {
                case files <- p:
                case <-ctx.Done():
                    return ctx.Err()
                }
            }
            return nil
        })
        close(files)
    }()
    
    // Collector
    go func() {
        wg.Wait()
        close(results)
        close(errors)
    }()
    
    // Collect results
    var fileInfos []*FileInfo
    for info := range results {
        fileInfos = append(fileInfos, info)
    }
    
    return fileInfos, nil
}
```

### Transcoding Worker Pool

Limit concurrent transcoding jobs based on CPU cores:

```go
// internal/infrastructure/queue/transcode_queue.go
type TranscodeQueue struct {
    jobs       chan *TranscodeJob
    maxWorkers int
}

func NewTranscodeQueue() *TranscodeQueue {
    // Default: max(1, cores/2) for CPU-intensive work
    // Override with env var: MAX_TRANSCODE_JOBS
    maxWorkers := max(1, runtime.NumCPU()/2)
    if override := os.Getenv("MAX_TRANSCODE_JOBS"); override != "" {
        if w, err := strconv.Atoi(override); err == nil && w > 0 {
            maxWorkers = w
        }
    }
    
    q := &TranscodeQueue{
        jobs:       make(chan *TranscodeJob, 100),
        maxWorkers: maxWorkers,
    }
    
    // Start workers
    for i := 0; i < maxWorkers; i++ {
        go q.worker()
    }
    
    return q
}

func (q *TranscodeQueue) worker() {
    for job := range q.jobs {
        if err := q.processJob(job); err != nil {
            logger.Error("transcode job failed",
                "job_id", job.ID,
                "error", err,
            )
        }
    }
}
```

### Context Cancellation - Graceful Shutdown

**Strategy**: Finish current operation, then stop

```go
// internal/application/library/scan_library.go
func (uc *ScanLibrary) Execute(ctx context.Context, libraryID int64) error {
    files := uc.scanner.FindMediaFiles(libraryPath)
    
    for _, file := range files {
        // Check for cancellation before processing next file
        select {
        case <-ctx.Done():
            logger.Info("scan cancelled, stopping gracefully",
                "library_id", libraryID,
                "processed", processedCount,
            )
            // Mark scan as cancelled in database
            uc.repo.UpdateScanStatus(ctx, libraryID, "cancelled")
            return ctx.Err()
        default:
            // Continue processing current file
        }
        
        // Process file...
    }
    
    return nil
}
```

**Benefits**:
- ✅ Finish current file completely (clean state)
- ✅ No partial/corrupted data
- ✅ Quick response to cancellation
- ✅ Partial progress is saved and useful

---

## Database Connection Pooling

### SQLite with WAL Mode

**Strategy**: Enable WAL mode for concurrent reads, single writer

```go
// internal/infrastructure/database/connection.go
func Connect(dbPath string) (*sql.DB, error) {
    db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }
    
    // SQLite connection pool settings
    db.SetMaxOpenConns(1)   // Single writer
    db.SetMaxIdleConns(1)   // Keep connection alive
    db.SetConnMaxLifetime(0) // Reuse indefinitely
    
    // Verify connection
    if err := db.Ping(); err != nil {
        return nil, fmt.Errorf("failed to ping database: %w", err)
    }
    
    return db, nil
}
```

**Benefits**:
- ✅ Concurrent reads (multiple API requests)
- ✅ Single writer (SQLite limitation)
- ✅ Good performance for single-user
- ✅ Easy migration to PostgreSQL later (just change pool settings)

---

## Configuration Management

### Environment Variables
```go
type Config struct {
    Port         int    `envconfig:"PORT" default:"3000"`
    DatabaseURL  string `envconfig:"DATABASE_URL" required:"true"`
    DataDir      string `envconfig:"DATA_DIR" default:"/data"`
    LogLevel     string `envconfig:"LOG_LEVEL" default:"info"`
    FrontendURL  string `envconfig:"FRONTEND_URL"`
    CORSOrigins  string `envconfig:"CORS_ORIGINS"`
}
```

### Loading Config
```go
var cfg Config
err := envconfig.Process("", &cfg)
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

### Current (Single-User)
- CORS validation
- Path traversal prevention (absolute paths only, no `..`)
- Input validation and sanitization
- SQL injection prevention (sqlc prepared statements)
- WebP image serving (security + performance)

### Future (Multi-User)
- JWT authentication
- Password hashing (bcrypt)
- RBAC (Role-Based Access Control)
- Rate limiting

---

## Scanner Implementation Details

### File Discovery
```go
// Scan recursively, skip hidden files and system directories
func ScanDirectory(path string) ([]FileInfo, error) {
    var files []FileInfo
    
    err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return err
        }
        
        // Skip hidden files/dirs (start with .)
        if strings.HasPrefix(d.Name(), ".") {
            if d.IsDir() {
                return filepath.SkipDir
            }
            return nil
        }
        
        // Check if video/audio file
        if !d.IsDir() && isMediaFile(path) {
            files = append(files, FileInfo{Path: path, Size: ...})
        }
        
        return nil
    })
    
    return files, err
}
```

### File Hashing Strategy
**Partial hash for fast duplicate detection**:
```go
// Hash first 64KB + last 64KB + file size
func PartialHash(path string) (string, error) {
    f, err := os.Open(path)
    if err != nil {
        return "", err
    }
    defer f.Close()
    
    stat, _ := f.Stat()
    size := stat.Size()
    
    h := sha256.New()
    
    // Hash first 64KB
    buf := make([]byte, 64*1024)
    n, _ := f.Read(buf)
    h.Write(buf[:n])
    
    // Hash last 64KB if file is large enough
    if size > 128*1024 {
        f.Seek(-64*1024, io.SeekEnd)
        n, _ = f.Read(buf)
        h.Write(buf[:n])
    }
    
    // Include file size in hash
    binary.Write(h, binary.LittleEndian, size)
    
    return hex.EncodeToString(h.Sum(nil)), nil
}
```

### Filename Parsing with NFO Support
**Priority order**:
1. Check for `.nfo` file (Kodi/Plex format)
2. Parse filename with regex patterns
3. Fallback to basic parsing

```go
// Parse metadata from NFO or filename
func ParseMetadata(filePath string) (*MediaMetadata, error) {
    // Try NFO file first
    nfoPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".nfo"
    if meta, err := parseNFO(nfoPath); err == nil {
        return meta, nil
    }
    
    // Check for movie.nfo in same directory
    dirNFO := filepath.Join(filepath.Dir(filePath), "movie.nfo")
    if meta, err := parseNFO(dirNFO); err == nil {
        return meta, nil
    }
    
    // Fallback to filename parsing
    return parseFilename(filePath)
}

// Example regex patterns
var moviePattern = regexp.MustCompile(`^(.+?)[\s._\-]*\((\d{4})\)`)
var tvPattern = regexp.MustCompile(`[Ss](\d{1,2})[Ee](\d{1,2})`)
```

### Scan Conflict Resolution
**Smart handling of file changes**:
```go
func ResolveScanConflict(existing *Media, scanned FileInfo) Action {
    // Case 1: File moved (same hash, different path)
    if existing.FileHash == scanned.Hash && existing.FilePath != scanned.Path {
        return UpdatePath{
            MediaID: existing.ID,
            NewPath: scanned.Path,
            // Preserve metadata and watch progress
        }
    }
    
    // Case 2: File replaced (same path, different hash)
    if existing.FilePath == scanned.Path && existing.FileHash != scanned.Hash {
        return ReplaceFile{
            MediaID: existing.ID,
            NewHash: scanned.Hash,
            // Reset transcode status, keep watch progress
        }
    }
    
    // Case 3: Duplicate file (same hash, different path)
    if existing.FileHash == scanned.Hash && existing.FilePath != scanned.Path {
        return MarkDuplicate{
            OriginalID: existing.ID,
            DuplicatePath: scanned.Path,
            // Flag for user review
        }
    }
    
    return NoAction{}
}

// Handle missing files
func MarkMissingFiles(libraryID int, scannedPaths []string) error {
    // Files in DB but not in scan results
    missing := findMissingFiles(libraryID, scannedPaths)
    
    for _, media := range missing {
        media.Status = "unavailable"
        media.LastSeen = time.Now()
        // Keep in database, don't delete
    }
    
    return nil
}
```

### Thumbnail Generation Queue
**Background processing**:
```go
// Thumbnail queue with priority
type ThumbnailQueue struct {
    queue chan ThumbnailJob
    workers int
}

type ThumbnailJob struct {
    MediaID  int
    FilePath string
    Priority int // Higher for recently added
}

func (q *ThumbnailQueue) Start() {
    for i := 0; i < q.workers; i++ {
        go q.worker()
    }
}

func (q *ThumbnailQueue) worker() {
    for job := range q.queue {
        thumbnail, err := generateThumbnail(job.FilePath)
        if err != nil {
            logger.Error("thumbnail generation failed",
                "media_id", job.MediaID,
                "error", err,
            )
            continue
        }
        
        // Convert to WebP
        webp := convertToWebP(thumbnail)
        
        // Save to data/thumbnails/
        saveThumbnail(job.MediaID, webp)
    }
}

// Add job to queue after scan
func QueueThumbnail(mediaID int, filePath string, priority int) {
    thumbnailQueue.queue <- ThumbnailJob{
        MediaID: mediaID,
        FilePath: filePath,
        Priority: priority,
    }
}
```

---

## API Handler Structure

### Organized by Domain
```go
// internal/interfaces/http/handler/library.go
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
    // Handler implementation
}

func (h *LibraryHandler) List(c *gin.Context) {
    // Handler implementation
}

func (h *LibraryHandler) Scan(c *gin.Context) {
    // Handler implementation
}
```

### Router Setup
```go
// internal/interfaces/http/router/router.go
func SetupRouter(
    libraryHandler *handler.LibraryHandler,
    mediaHandler *handler.MediaHandler,
) *gin.Engine {
    r := gin.Default()
    
    api := r.Group("/api")
    {
        // Library routes
        libraries := api.Group("/libraries")
        {
            libraries.GET("", libraryHandler.List)
            libraries.POST("", libraryHandler.Create)
            libraries.GET("/:id", libraryHandler.Get)
            libraries.PUT("/:id", libraryHandler.Update)
            libraries.DELETE("/:id", libraryHandler.Delete)
            libraries.POST("/:id/scan", libraryHandler.Scan)
        }
        
        // Media routes
        media := api.Group("/media")
        {
            media.GET("", mediaHandler.List)
            media.GET("/:id", mediaHandler.Get)
            media.DELETE("/:id", mediaHandler.Delete)
            media.GET("/:id/stream", mediaHandler.Stream)
        }
    }
    
    return r
}
```

This structure makes it easy to add `/api/v1/` prefix later:
```go
api := r.Group("/api/v1")
// All routes automatically versioned
```

---

## Frontend Architecture

### Overview

The frontend follows a **feature-based architecture** that mirrors the backend's domain-driven approach. State is split between server state (TanStack Query) and UI state (Zustand).

### Technology Stack

**Core**:
- React 19 + TypeScript
- Vite 5 (build tool)
- TanStack Router (file-based routing)
- TanStack Query v5 (server state)
- Zustand (UI state)

**UI**:
- Shadcn/ui + Radix UI (components)
- Tailwind CSS (styling)
- Lucide React (icons)

**Code Quality**:
- Biome (formatting + linting)
- Vitest + React Testing Library
- TypeScript strict mode

### Directory Structure

```
web/
├── src/
│   ├── features/              # Feature-based organization
│   │   ├── library/
│   │   │   ├── components/    # Library-specific components
│   │   │   ├── hooks/         # useLibraries, useLibraryMutation
│   │   │   ├── stores/        # Zustand stores (if needed)
│   │   │   ├── types.ts       # TypeScript types
│   │   │   └── utils.ts       # Feature utilities
│   │   ├── media/
│   │   │   ├── components/
│   │   │   │   ├── MediaCard.tsx
│   │   │   │   ├── MediaGrid.tsx
│   │   │   │   └── MediaDetail.tsx
│   │   │   ├── hooks/
│   │   │   │   ├── useMedia.ts
│   │   │   │   └── useMediaQuery.ts
│   │   │   └── types.ts
│   │   ├── player/
│   │   │   ├── components/
│   │   │   │   ├── VideoPlayer.tsx
│   │   │   │   ├── PlayerControls.tsx
│   │   │   │   └── QualitySelector.tsx
│   │   │   ├── hooks/
│   │   │   │   ├── usePlayer.ts
│   │   │   │   └── useProgress.ts
│   │   │   └── stores/
│   │   │       └── playerStore.ts
│   │   └── settings/
│   │       ├── components/
│   │       └── hooks/
│   ├── shared/                # Shared across features
│   │   ├── components/
│   │   │   ├── ui/            # Shadcn components
│   │   │   ├── Layout.tsx
│   │   │   ├── Sidebar.tsx
│   │   │   └── ErrorBoundary.tsx
│   │   ├── hooks/
│   │   │   ├── useDebounce.ts
│   │   │   └── useLocalStorage.ts
│   │   └── utils/
│   │       ├── cn.ts          # Class name utility
│   │       └── format.ts      # Formatting utilities
│   ├── lib/                   # Third-party configuration
│   │   ├── api/               # Generated API client (Orval)
│   │   │   ├── generated/     # Auto-generated (gitignored)
│   │   │   └── client.ts      # API client setup
│   │   ├── queryClient.ts     # TanStack Query config
│   │   └── router.ts          # TanStack Router config
│   ├── routes/                # File-based routing
│   │   ├── __root.tsx         # Root layout
│   │   ├── index.tsx          # Home page (/)
│   │   ├── libraries/
│   │   │   ├── index.tsx      # /libraries
│   │   │   └── $libraryId.tsx # /libraries/:libraryId
│   │   ├── media/
│   │   │   ├── index.tsx      # /media
│   │   │   └── $mediaId.tsx   # /media/:mediaId
│   │   ├── player/
│   │   │   └── $mediaId.tsx   # /player/:mediaId
│   │   └── settings/
│   │       └── index.tsx      # /settings
│   ├── App.tsx
│   ├── main.tsx
│   └── index.css
├── public/
├── tests/
│   ├── setup.ts
│   └── mocks/
│       └── handlers.ts        # MSW handlers
├── orval.config.ts            # API client generation config
├── tailwind.config.js
├── tsconfig.json
├── vite.config.ts
└── package.json
```

### State Management Strategy

#### Server State (TanStack Query)

**All backend data** flows through TanStack Query:

```typescript
// features/media/hooks/useMedia.ts
import { useQuery } from '@tanstack/react-query'
import { mediaApi } from '@/lib/api/generated'

export function useMedia(id: string) {
  return useQuery({
    queryKey: ['media', id],
    queryFn: () => mediaApi.getMedia(id),
    staleTime: 5 * 60 * 1000, // 5 minutes
  })
}

export function useMediaList(libraryId?: string) {
  return useQuery({
    queryKey: ['media', 'list', libraryId],
    queryFn: () => mediaApi.listMedia({ libraryId }),
  })
}
```

**Mutations** with automatic invalidation:

```typescript
// features/media/hooks/useMediaMutation.ts
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { mediaApi } from '@/lib/api/generated'

export function useDeleteMedia() {
  const queryClient = useQueryClient()
  
  return useMutation({
    mutationFn: (id: string) => mediaApi.deleteMedia(id),
    onSuccess: () => {
      // Invalidate and refetch media list
      queryClient.invalidateQueries({ queryKey: ['media', 'list'] })
    },
  })
}
```

#### UI State (Zustand)

**For global UI state** that doesn't come from the server:

```typescript
// features/player/stores/playerStore.ts
import { create } from 'zustand'

interface PlayerState {
  isPlaying: boolean
  volume: number
  isMuted: boolean
  currentTime: number
  
  play: () => void
  pause: () => void
  setVolume: (volume: number) => void
  toggleMute: () => void
  seek: (time: number) => void
}

export const usePlayerStore = create<PlayerState>((set) => ({
  isPlaying: false,
  volume: 1,
  isMuted: false,
  currentTime: 0,
  
  play: () => set({ isPlaying: true }),
  pause: () => set({ isPlaying: false }),
  setVolume: (volume) => set({ volume, isMuted: volume === 0 }),
  toggleMute: () => set((state) => ({ isMuted: !state.isMuted })),
  seek: (currentTime) => set({ currentTime }),
}))
```

**Usage in components**:

```typescript
function VideoPlayer({ mediaId }: { mediaId: string }) {
  const { isPlaying, volume, play, pause } = usePlayerStore()
  const { data: media } = useMedia(mediaId)
  
  return (
    <video
      src={media?.streamUrl}
      volume={volume}
      onPlay={play}
      onPause={pause}
    />
  )
}
```

### Routing Pattern (File-based)

Routes automatically map to URLs:

```
routes/
  __root.tsx           → Layout wrapper
  index.tsx            → /
  libraries/
    index.tsx          → /libraries
    $libraryId.tsx     → /libraries/:libraryId
  media/
    index.tsx          → /media
    $mediaId.tsx       → /media/:mediaId
  player/
    $mediaId.tsx       → /player/:mediaId
```

**Type-safe params**:

```typescript
// routes/media/$mediaId.tsx
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/media/$mediaId')({
  component: MediaDetail,
})

function MediaDetail() {
  const { mediaId } = Route.useParams() // TypeScript knows mediaId exists!
  const { data: media } = useMedia(mediaId)
  
  return <div>{media?.title}</div>
}
```

### API Client Generation (Orval)

**Automatic generation** from backend Swagger:

```typescript
// orval.config.ts
export default {
  viewra: {
    input: '../docs/swagger/swagger.json', // Backend generates this
    output: {
      mode: 'tags-split',
      target: 'src/lib/api/generated/endpoints.ts',
      client: 'react-query',
      mock: true,
    },
    hooks: {
      afterAllFilesWrite: 'prettier --write',
    },
  },
}
```

**Generated hooks** ready to use:

```typescript
// Auto-generated by Orval
export function useGetMedia(id: string) {
  return useQuery({
    queryKey: getGetMediaQueryKey(id),
    queryFn: () => getMedia(id),
  })
}

export function useCreateLibrary() {
  return useMutation({
    mutationFn: (data: CreateLibraryDto) => createLibrary(data),
  })
}
```

### Component Patterns

#### Feature Components

Co-locate everything for a feature:

```typescript
// features/media/components/MediaCard.tsx
import { Card } from '@/shared/components/ui/card'
import { useMedia } from '../hooks/useMedia'
import { MediaType } from '../types'

interface MediaCardProps {
  mediaId: string
  onClick?: () => void
}

export function MediaCard({ mediaId, onClick }: MediaCardProps) {
  const { data: media, isLoading } = useMedia(mediaId)
  
  if (isLoading) return <Skeleton />
  if (!media) return null
  
  return (
    <Card onClick={onClick}>
      <img src={media.thumbnailUrl} alt={media.title} />
      <h3>{media.title}</h3>
      <p>{media.year}</p>
    </Card>
  )
}
```

#### Shared Components

Components used across multiple features:

```typescript
// shared/components/Layout.tsx
import { Sidebar } from './Sidebar'
import { Outlet } from '@tanstack/react-router'

export function Layout() {
  return (
    <div className="flex h-screen">
      <Sidebar />
      <main className="flex-1 overflow-auto">
        <Outlet /> {/* Renders child routes */}
      </main>
    </div>
  )
}
```

### Testing Strategy

#### Component Tests (Vitest + Testing Library)

```typescript
// features/media/components/MediaCard.test.tsx
import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MediaCard } from './MediaCard'

describe('MediaCard', () => {
  it('renders media information', async () => {
    const queryClient = new QueryClient()
    
    render(
      <QueryClientProvider client={queryClient}>
        <MediaCard mediaId="123" />
      </QueryClientProvider>
    )
    
    expect(await screen.findByText('The Matrix')).toBeInTheDocument()
    expect(screen.getByText('1999')).toBeInTheDocument()
  })
})
```

#### API Mocking (MSW)

```typescript
// tests/mocks/handlers.ts
import { http, HttpResponse } from 'msw'

export const handlers = [
  http.get('/api/media/:id', ({ params }) => {
    return HttpResponse.json({
      id: params.id,
      title: 'The Matrix',
      year: 1999,
      thumbnailUrl: '/thumbnails/123.jpg',
    })
  }),
  
  http.get('/api/libraries', () => {
    return HttpResponse.json([
      { id: '1', name: 'Movies', type: 'movies' },
      { id: '2', name: 'TV Shows', type: 'tv' },
    ])
  }),
]
```

### Code Organization Rules

**File naming**:
- Components: PascalCase (`MediaCard.tsx`)
- Hooks: camelCase with `use` prefix (`useMedia.ts`)
- Utilities: camelCase (`formatDuration.ts`)
- Types: PascalCase for interfaces/types (`MediaType.ts`)

**Import order** (enforced by Biome):
```typescript
// 1. React
import { useState, useEffect } from 'react'

// 2. External libraries
import { useQuery } from '@tanstack/react-query'

// 3. Internal absolute imports (@/)
import { Button } from '@/shared/components/ui/button'
import { useMedia } from '@/features/media/hooks/useMedia'

// 4. Relative imports
import { MediaCard } from './MediaCard'
import type { MediaType } from '../types'
```

**Barrel exports** (index.ts) for cleaner imports:

```typescript
// features/media/components/index.ts
export { MediaCard } from './MediaCard'
export { MediaGrid } from './MediaGrid'
export { MediaDetail } from './MediaDetail'

// Usage:
import { MediaCard, MediaGrid } from '@/features/media/components'
```

### Error Handling & User Feedback

#### Toast Notifications (Sonner)

```typescript
// lib/queryClient.ts
import { toast } from 'sonner'

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 3,
      staleTime: 5 * 60 * 1000,
      onError: (error: Error) => {
        toast.error(error.message || 'Something went wrong')
      },
    },
    mutations: {
      onError: (error: Error) => {
        toast.error(error.message || 'Failed to save changes')
      },
      onSuccess: () => {
        toast.success('Changes saved successfully')
      },
    },
  },
})
```

#### Error Boundaries

```typescript
// shared/components/ErrorBoundary.tsx
import { ErrorBoundary as ReactErrorBoundary } from 'react-error-boundary'

function ErrorFallback({ error, resetErrorBoundary }: ErrorFallbackProps) {
  return (
    <div className="flex flex-col items-center justify-center min-h-screen">
      <h2>Something went wrong</h2>
      <pre className="text-sm text-red-500">{error.message}</pre>
      <Button onClick={resetErrorBoundary}>Try again</Button>
    </div>
  )
}

export function ErrorBoundary({ children }: { children: React.ReactNode }) {
  return (
    <ReactErrorBoundary FallbackComponent={ErrorFallback}>
      {children}
    </ReactErrorBoundary>
  )
}
```

#### Loading States

```typescript
// Use Skeleton loaders, not spinners
function MediaGrid() {
  const { data, isLoading } = useMediaList()
  
  if (isLoading) {
    return (
      <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4">
        {Array.from({ length: 12 }).map((_, i) => (
          <Skeleton key={i} className="aspect-[2/3] rounded-lg" />
        ))}
      </div>
    )
  }
  
  return <div>{/* Render media cards */}</div>
}
```

#### Empty States

```typescript
function EmptyLibrary() {
  return (
    <div className="flex flex-col items-center justify-center min-h-[400px]">
      <FilmIcon className="w-16 h-16 text-muted-foreground mb-4" />
      <h3 className="text-xl font-semibold mb-2">No media found</h3>
      <p className="text-muted-foreground mb-6">
        Scan your library to add media files
      </p>
      <Button onClick={handleScan}>Scan Library</Button>
    </div>
  )
}
```

### Theming & Dark Mode

#### Light + Dark Mode Support

```typescript
// features/settings/stores/themeStore.ts
import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface ThemeState {
  mode: 'light' | 'dark' | 'system'
  setMode: (mode: 'light' | 'dark' | 'system') => void
}

export const useThemeStore = create<ThemeState>()(
  persist(
    (set) => ({
      mode: 'system',
      setMode: (mode) => {
        set({ mode })
        // Update document class
        if (mode === 'system') {
          const isDark = window.matchMedia('(prefers-color-scheme: dark)').matches
          document.documentElement.classList.toggle('dark', isDark)
        } else {
          document.documentElement.classList.toggle('dark', mode === 'dark')
        }
      },
    }),
    { name: 'theme' }
  )
)
```

#### Tailwind Configuration

```typescript
// tailwind.config.js
export default {
  darkMode: 'class', // Enable class-based dark mode
  theme: {
    extend: {
      colors: {
        // Custom color palette for both themes
        background: 'hsl(var(--background))',
        foreground: 'hsl(var(--foreground))',
        primary: 'hsl(var(--primary))',
        // ... more colors
      },
    },
  },
}
```

#### CSS Variables

```css
/* index.css */
@layer base {
  :root {
    --background: 0 0% 100%;
    --foreground: 222.2 84% 4.9%;
    --primary: 221.2 83.2% 53.3%;
    /* ... light mode colors */
  }

  .dark {
    --background: 222.2 84% 4.9%;
    --foreground: 210 40% 98%;
    --primary: 217.2 91.2% 59.8%;
    /* ... dark mode colors */
  }
}
```

### Mobile Responsiveness

#### Mobile-First Strategy

Build mobile layouts first, then enhance for larger screens:

```typescript
// Mobile: < 640px (sm)
// Tablet: 640-1024px (md, lg)
// Desktop: > 1024px (xl, 2xl)

function MediaGrid({ media }: MediaGridProps) {
  return (
    <div className="
      grid gap-4
      grid-cols-2          /* Mobile: 2 columns */
      md:grid-cols-3       /* Tablet: 3 columns */
      lg:grid-cols-4       /* Desktop: 4 columns */
      xl:grid-cols-6       /* Large: 6 columns */
    ">
      {media.map((item) => (
        <MediaCard key={item.id} media={item} />
      ))}
    </div>
  )
}
```

#### Touch-Friendly Interactions

```typescript
// Larger tap targets for mobile
function MediaCard({ media }: MediaCardProps) {
  return (
    <button
      className="
        w-full aspect-[2/3] rounded-lg
        min-h-[44px]  /* iOS minimum tap target */
        focus:ring-2 focus:ring-primary
        active:scale-95 transition-transform
      "
    >
      {/* Card content */}
    </button>
  )
}
```

#### Responsive Navigation

```typescript
// Mobile: Bottom nav or hamburger menu
// Desktop: Sidebar

function Layout() {
  return (
    <div className="flex flex-col md:flex-row h-screen">
      {/* Desktop sidebar */}
      <Sidebar className="hidden md:flex" />
      
      {/* Main content */}
      <main className="flex-1 overflow-auto pb-16 md:pb-0">
        <Outlet />
      </main>
      
      {/* Mobile bottom nav */}
      <BottomNav className="md:hidden" />
    </div>
  )
}
```

#### Responsive Player

```typescript
// Different player layouts for different screens
function VideoPlayer({ mediaId }: VideoPlayerProps) {
  return (
    <div className="
      /* Mobile: Fullscreen */
      fixed inset-0 z-50
      
      /* Desktop: Aspect ratio container */
      md:relative md:aspect-video md:z-auto
    ">
      <ShakaPlayer mediaId={mediaId} />
      
      {/* Controls adapt to screen size */}
      <PlayerControls
        layout={isMobile ? 'compact' : 'full'}
      />
    </div>
  )
}
```

### Performance Optimizations

#### Code Splitting

Route-based automatic code splitting via TanStack Router:

```typescript
// Each route file is automatically code-split
// routes/media/$mediaId.tsx only loads when visiting /media/:id
```

#### Query Optimization

```typescript
// Prefetch on hover for instant navigation
function MediaCard({ mediaId }: MediaCardProps) {
  const queryClient = useQueryClient()
  
  const prefetchMedia = () => {
    queryClient.prefetchQuery({
      queryKey: ['media', mediaId],
      queryFn: () => mediaApi.getMedia(mediaId),
    })
  }
  
  return (
    <Card
      onMouseEnter={prefetchMedia}
      onClick={() => navigate(`/media/${mediaId}`)}
    >
      {/* ... */}
    </Card>
  )
}
```

#### Image Optimization

```typescript
// Lazy load images with native loading
<img
  src={media.thumbnailUrl}
  alt={media.title}
  loading="lazy"
  decoding="async"
/>
```

#### PWA & Offline Support

**Decision: Skip PWA, Local Network Only** ✅

No Progressive Web App features needed. This is a local media server accessed over LAN, not a cloud service.

**Rationale:**
- Users need network connection to stream media from server
- Offline support provides no value for local media streaming
- PWA mainly benefits cloud apps with intermittent connectivity
- Reduces complexity and maintenance burden
- Can reconsider in Phase 5 if users request installable app features

#### Performance Monitoring

**Decision: Skip Initially, Add in Phase 6 if Needed** ✅

Basic logging is sufficient for personal/home media server use. Extensive monitoring can be added later if needed.

**Phase 1-5 Approach:**
- Structured logging (JSON format)
- Log to stdout (Docker-friendly)
- Basic error tracking in logs
- Simple request/response timing logs

**Phase 6+ (if performance issues arise):**
- Prometheus metrics (request rate, latency, errors)
- OpenTelemetry tracing
- Grafana dashboards
- Database query performance monitoring

**Rationale:**
- Personal/home use doesn't require production-grade observability
- Premature optimization adds unnecessary complexity
- Can add metrics incrementally when needed
- Focus on core features first

---

## Security Considerations
- HTTPS enforcement
