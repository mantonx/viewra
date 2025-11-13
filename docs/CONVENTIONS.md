# ViewRA Code Conventions

Quick reference for coding standards, file naming, and project organization.

**Last Updated**: 2025-11-12

---

## Core Principles

### 1. Name Files by WHAT, Not WHEN

✅ **Good** (describes responsibility):
```
scanner.go
validator.go
metadata.go
```

❌ **Bad** (temporal naming):
```
config_v2.go
scanner_new.go
utils_final.go
```

### 2. One Responsibility Per File

Files should have a clear, single purpose.

```
✅ hash.go           # File hashing
✅ validator.go      # Input validation

❌ utils.go          # Too broad
❌ helpers.go        # Unclear purpose
```

### 3. Separate Types from Implementation

Define types and interfaces in dedicated files separate from implementations.

---

## Go Backend Conventions

### Domain Layer (`internal/domain/<domain>/`)

**Standard files:**
```
domain/entity_name/
├── entity.go      # Domain entity struct and methods
├── types.go       # Enums, constants, value objects
├── repository.go  # Repository interface
├── errors.go      # Domain-specific errors
├── service.go     # Business logic service (optional)
└── *_test.go      # Tests
```

**Examples:**
- Library domain: entity.go, types.go, repository.go, errors.go
- Media domain: entity.go, types.go, repository.go, metadata.go

### Application Layer (`internal/application/<domain>/`)

**Pattern**: `<verb>_<noun>.go` for use cases

```
application/library/
├── create_library.go   # CreateLibrary use case
├── update_library.go   # UpdateLibrary use case
├── delete_library.go   # DeleteLibrary use case
├── scan_library.go     # ScanLibrary use case
├── dto.go              # All DTOs for this domain
└── interfaces.go       # Repository interfaces needed
```

### Infrastructure Layer (`internal/infrastructure/`)

#### Persistence (`persistence/<domain>/`)

```
persistence/library/
├── repository.go   # Repository implementation
├── helpers.go      # Conversion functions
└── *_test.go       # Integration tests
```

#### Database (`database/`)

```
database/
├── connection.go
├── migrate.go
├── queries/
│   ├── postgres/   # PostgreSQL-specific SQL
│   └── sqlite/     # SQLite-specific SQL
├── sqlc_postgres/  # Generated PostgreSQL code
└── sqlc_sqlite/    # Generated SQLite code
```

**DO NOT** manually edit generated SQLC files.

#### Other Infrastructure

| Directory | Purpose | File Pattern |
|-----------|---------|--------------|
| `ffmpeg/` | FFmpeg integration | `client.go`, `metadata.go` |
| `filesystem/` | File operations | `scanner.go`, `coordinator.go` |
| `streaming/` | Media streaming | `server.go`, `range.go` |

### API Layer (`internal/api/`)

```
api/
├── server.go           # HTTP server setup
├── handlers/
│   ├── library.go      # Library endpoints
│   ├── media.go        # Media endpoints
│   ├── progress.go     # Progress endpoints
│   └── helpers.go      # Shared handler utilities
├── middleware/
│   ├── logger.go       # Request logging
│   └── cors.go         # CORS handling
└── routes/
    └── routes.go       # Route registration
```

### Test Files

**Naming**: `<file>_test.go`

```
// Unit test
repository_test.go

// Integration test (if separating)
repository_integration_test.go
```

**Build tags for integration tests:**
```go
// +build integration

package library_test
```

---

## Database Conventions

### Dual Database Support

ViewRA supports both SQLite (default) and PostgreSQL. Follow these patterns:

**Query Router Pattern:**
```go
result, err := r.router.Route(
    func() (any, error) {
        return r.postgres.CreateLibrary(ctx, pgParams)
    },
    func() (any, error) {
        return r.sqlite.CreateLibrary(ctx, sqParams)
    },
)
```

**Type Conversion Helpers:**
- Use `persistence/common/helpers.go` for Null* conversions
- `NullString(s string)` → `sql.NullString`
- `NullInt64(i int64)` → `sql.NullInt64`
- `ParseNullTime(t pgtype.Timestamp)` → `time.Time`

**Migration Files:**
- Create migrations in pairs: `000N_name.up.sql` and `000N_name.down.sql`
- Keep both `migrations/` (SQLite) and `migrations/postgres/` in sync
- Use type-appropriate syntax (SERIAL vs AUTOINCREMENT)

---

## Frontend Conventions (React + TypeScript)

### Component Organization

```
components/
├── ui/                 # Reusable UI components
│   ├── Button/
│   │   ├── Button.tsx
│   │   └── Button.test.tsx
│   └── Card/
│       ├── Card.tsx
│       └── Card.css
└── library/            # Feature-specific components
    ├── LibraryCard/
    ├── LibraryForm/
    └── LibraryList/
```

**Naming Convention:**
- PascalCase for components: `LibraryCard.tsx`
- Colocate styles: `LibraryCard.css` or `LibraryCard.module.css`
- Export from index when using folders: `Button/index.ts`

### Hooks (`lib/hooks/`)

**Pattern**: `use<Feature>.ts`

```
useLibraries.ts
useMedia.ts
useProgress.ts
useInvalidateLibraries.ts
```

### API Integration

**Generated**: `lib/api/generated/`  
**Custom**: `lib/api/mutator/`

Don't manually edit generated API clients. Regenerate with:
```bash
npm run generate:api
```

### Routes (`routes/`)

Using TanStack Router with file-based routing:
```
routes/
├── __root.tsx          # Root layout
├── _layout.tsx         # App layout
├── _layout/
│   ├── libraries.tsx   # /libraries route
│   └── media.tsx       # /media route
└── index.tsx           # / route
```

---

## Configuration Files

### Root Directory

| File | Purpose |
|------|---------|
| `.gitignore` | Git exclusions |
| `go.mod` / `go.sum` | Go dependencies |
| `Makefile` | Build automation |
| `sqlc.yaml` | SQLC configuration |
| `.air.toml` | Live reload config |

### Migrations Directory

```
migrations/
├── 000001_init.up.sql
├── 000001_init.down.sql
├── 000002_add_feature.up.sql
└── 000002_add_feature.down.sql

migrations/postgres/
├── 000001_init.up.sql
└── ...
```

---

## Anti-Patterns to Avoid

### ❌ Version Suffixes
```
config.go
config_v2.go        # Don't do this
config_new.go       # Don't do this
config_final.go     # Don't do this
```

**Instead**: Rename the old file if keeping it:
```
config.go           # New version
config_legacy.go    # Old version (if needed)
```

### ❌ Catch-All Files
```
utils.go            # Too generic
helpers.go          # Unclear purpose
misc.go             # Dumping ground
```

**Instead**: Use specific names:
```
validator.go        # Input validation
converter.go        # Type conversions
parser.go           # String parsing
```

### ❌ Deep Nesting
```
internal/infrastructure/persistence/repository/impl/concrete/library/v2/repository.go
```

**Instead**: Keep it flat:
```
internal/infrastructure/persistence/library/repository.go
```

### ❌ Mixing Concerns
```go
// ❌ Don't mix domain logic with HTTP handling
func CreateLibraryHandler(w http.ResponseWriter, r *http.Request) {
    // Database logic here
    // Business rules here
    // HTTP response here
}
```

**Instead**: Separate layers:
```go
// ✅ Handler → Use Case → Repository
func (h *LibraryHandler) Create(c *gin.Context) {
    result, err := h.createLibrary.Execute(ctx, request)
    // Just handle HTTP concerns
}
```

---

## Quick Reference

### When to Use Which File Name

| Situation | File Name |
|-----------|-----------|
| Domain entity | `entity.go` |
| Repository interface | `repository.go` (in domain) |
| Repository implementation | `repository.go` (in persistence) |
| DTOs for a domain | `dto.go` |
| Domain-specific errors | `errors.go` |
| Use case logic | `<verb>_<noun>.go` |
| Shared helpers | `helpers.go` (only in persistence/common) |
| Type definitions | `types.go` |
| HTTP handler | `<domain>.go` in handlers/ |
| Tests | `<file>_test.go` |

### Directory Organization

```
viewra/
├── cmd/viewra/         # Application entry point
├── internal/
│   ├── domain/         # Business entities and rules
│   ├── application/    # Use cases
│   ├── infrastructure/ # External integrations
│   └── api/            # HTTP layer
├── migrations/         # SQLite migrations
├── docs/               # Documentation
└── web/                # Frontend application
```

---

For detailed examples, see:
- **[QUICK_REFERENCE.md](./QUICK_REFERENCE.md)** - Development workflow
- **[TESTING.md](./TESTING.md)** - Testing guidelines
- **[INCOMPLETE_IMPLEMENTATIONS.md](./INCOMPLETE_IMPLEMENTATIONS.md)** - Current issues

**Last Updated**: 2025-11-12
