# Viewra Code Conventions

This document outlines the coding conventions and organizational standards for the Viewra project.

## Table of Contents

- [Type Organization](#type-organization)
- [File Structure](#file-structure)
- [Package Organization](#package-organization)
- [Naming Conventions](#naming-conventions)
- [Database Layer Conventions](#database-layer-conventions)

---

## Type Organization

### General Principle

**Separate types from implementation.** Types, structs, and interfaces should be defined in dedicated files separate from their implementations.

### File Naming Convention

Each package should follow this structure:

```
package/
├── types.go          # Struct definitions, type aliases, constants
├── interfaces.go     # Interface definitions (if not part of repository.go)
├── repository.go     # Repository interface (domain layer) or constructor (infrastructure layer)
├── implementation.go # Method implementations (can be multiple files)
├── errors.go         # Error definitions
├── service.go        # Service implementations (if applicable)
└── *_test.go         # Tests for each file
```

### What Goes in `types.go`

**DO include:**
- Struct definitions
- Type aliases (e.g., `type LibraryType string`)
- Constants related to types
- Enums and their values

**DO NOT include:**
- Method implementations
- Constructors (NewXxx functions)
- Business logic

**Example:** [internal/domain/library/types.go](../internal/domain/library/types.go)

```go
package library

// LibraryType represents the type of media content in a library.
type LibraryType string

const (
	LibraryTypeMovies LibraryType = "movies"
	LibraryTypeTV     LibraryType = "tv"
	LibraryTypeMusic  LibraryType = "music"
)
```

**Example:** [internal/infrastructure/persistence/library/types.go](../internal/infrastructure/persistence/library/types.go)

```go
package library

import (
	"database/sql"
	"github.com/viewra/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/viewra/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/viewra/viewra/internal/infrastructure/persistence/adapters"
)

// Repository implements the library.Repository interface using sqlc.
// It supports both SQLite and PostgreSQL through database-specific queriers.
type Repository struct {
	db       *sql.DB
	dbType   string
	sqlite   *sqlc_sqlite.Queries
	postgres *sqlc_postgres.Queries
	adapter  *adapters.TypeAdapter
}
```

---

## File Structure

### Domain Layer (`internal/domain/`)

Domain packages represent business entities and logic.

**Standard files:**

```
domain/entity_name/
├── entity.go      # Domain entity struct and validation methods
├── types.go       # Type aliases, enums, constants
├── repository.go  # Repository interface (contract)
├── errors.go      # Domain-specific errors
├── service.go     # Business logic service
└── *_test.go      # Tests
```

**Examples:**
- [internal/domain/library/](../internal/domain/library/)
- [internal/domain/media/](../internal/domain/media/)

### Infrastructure Layer (`internal/infrastructure/`)

Infrastructure packages implement domain interfaces and interact with external systems.

#### Persistence Layer (`internal/infrastructure/persistence/`)

**Standard files:**

```
persistence/entity_name/
├── types.go       # Repository struct definition
├── repository.go  # Constructor and method implementations
└── *_test.go      # Tests
```

**Examples:**
- [internal/infrastructure/persistence/library/](../internal/infrastructure/persistence/library/)
- [internal/infrastructure/persistence/media/](../internal/infrastructure/persistence/media/)

#### Database Layer (`internal/infrastructure/database/`)

Contains SQLC-generated code and SQL queries. **DO NOT manually edit generated files.**

```
database/
├── sqlc/               # Base SQLC types (int64 IDs, SQLite-compatible)
├── sqlc_postgres/      # PostgreSQL-specific types (int32 IDs)
├── sqlc_sqlite/        # SQLite-specific types (int64 IDs)
└── queries/
    ├── postgres/       # PostgreSQL-specific SQL queries
    │   ├── library.sql
    │   └── media.sql
    └── sqlite/         # SQLite-specific SQL queries
        ├── library.sql
        └── media.sql
```

**SQL Query Organization:**
- Separate PostgreSQL and SQLite queries into dedicated subdirectories
- Use the same filename convention in both directories (e.g., `library.sql`, `media.sql`)
- This separation improves maintainability and makes database-specific queries easy to locate
- SQLC automatically generates Go code based on the directory structure defined in `sqlc.yaml`

---

## Package Organization

### Layered Architecture

Viewra follows a clean architecture with clear layer separation:

```
┌─────────────────────────────────────┐
│     Domain Layer (Business Logic)   │  ← Pure Go, no dependencies
│   internal/domain/                  │
└─────────────────────────────────────┘
              ↑
              │ implements interfaces
              │
┌─────────────────────────────────────┐
│   Infrastructure Layer (Adapters)   │  ← Database, API, external systems
│   internal/infrastructure/          │
│     ├── persistence/                │
│     ├── database/                   │
│     └── api/                        │
└─────────────────────────────────────┘
```

**Dependency Rule:** Dependencies point inward. Domain layer has NO dependencies on infrastructure.

### Package Responsibilities

#### Domain Packages

- Define business entities (structs)
- Define business rules (validation methods)
- Define repository interfaces (contracts)
- Define domain errors
- Contain NO infrastructure concerns (no SQL, no HTTP, no file I/O)

**Example:** [internal/domain/library/entity.go](../internal/domain/library/entity.go)

#### Infrastructure Packages

- Implement domain repository interfaces
- Handle database queries (via SQLC)
- Convert between domain types and database types
- Route operations to appropriate database driver (PostgreSQL or SQLite)

**Example:** [internal/infrastructure/persistence/library/repository.go](../internal/infrastructure/persistence/library/repository.go)

---

## Naming Conventions

### Files

- Use lowercase with underscores for multi-word files: `helpers_test.go`
- Name files after their primary purpose:
  - `types.go` - type definitions
  - `errors.go` - error definitions
  - `repository.go` - repository implementation
  - `service.go` - service implementation
  - `entity.go` - domain entity

### Types

- Use **PascalCase** for exported types: `Library`, `MediaType`, `Repository`
- Use **camelCase** for unexported types: `internalState`
- Use descriptive names that convey purpose

### Variables

- Use **camelCase**: `libraryRepo`, `mediaService`
- Short names for receivers: `r *Repository`, `l *Library`
- Avoid single-letter names except in small scopes (loop indices)

### Constants

- Use **PascalCase** for exported constants: `LibraryTypeMovies`
- Group related constants in blocks

```go
const (
	LibraryTypeMovies LibraryType = "movies"
	LibraryTypeTV     LibraryType = "tv"
	LibraryTypeMusic  LibraryType = "music"
)
```

---

## Database Layer Conventions

### Dual Database Support

Viewra supports both **PostgreSQL** and **SQLite** through separate SQLC-generated packages.

#### Key Differences

| Aspect          | PostgreSQL                          | SQLite                        |
|-----------------|-------------------------------------|-------------------------------|
| ID Type         | `int32` (SERIAL)                    | `int64` (INTEGER)             |
| Package         | `sqlc_postgres`                     | `sqlc_sqlite`                 |
| Query Location  | `queries/postgres/`                 | `queries/sqlite/`             |
| Schema          | `migrations/postgres/`              | `migrations/`                 |

### Query Router Pattern (DRY)

Infrastructure repositories use the **QueryRouter** pattern to eliminate repetitive if/else branching:

```go
// Repository struct includes router
type Repository struct {
	db       *sql.DB
	dbType   string
	sqlite   *sqlc_sqlite.Queries
	postgres *sqlc_postgres.Queries
	router   *common.QueryRouter
}

// Initialize router in constructor
func NewRepository(db *sql.DB, driver string) *Repository {
	r := &Repository{
		db:     db,
		dbType: driver,
		router: common.NewQueryRouter(driver),
	}
	// ...
}

// Use router.Route() for operations that return data
func (r *Repository) GetByID(ctx context.Context, id int64) (*library.Library, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetLibraryByID(ctx, int32(id))
		},
		func() (any, error) {
			return r.sqlite.GetLibraryByID(ctx, id)
		},
	)
	if err != nil {
		return nil, err
	}

	// Type assertion based on database
	if r.router.IsPostgresDB() {
		pgResult := result.(sqlc_postgres.Library)
		return convertToDomain(pgResult), nil
	}

	sqResult := result.(sqlc_sqlite.Library)
	return convertToDomain(sqResult), nil
}

// Use router.RouteVoid() for operations that only return errors
func (r *Repository) Delete(ctx context.Context, id int64) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.DeleteLibrary(ctx, int32(id))
		},
		func() error {
			return r.sqlite.DeleteLibrary(ctx, id)
		},
	)
}
```

**Benefits:**
- ✅ No repetitive `if common.IsPostgres(r.dbType)` blocks
- ✅ Centralized routing logic
- ✅ Type-safe with explicit conversions
- ✅ Easy to add new database operations

### TypeAdapter Utility

For complex type conversions, use the reflection-based `TypeAdapter`:

```go
import "github.com/viewra/viewra/internal/infrastructure/persistence/adapters"

adapter := adapters.NewTypeAdapter()
sqliteModel, err := adapter.Convert(postgresModel, &sqlc_sqlite.Medium{})
```

**Location:** [internal/infrastructure/persistence/adapters/types.go](../internal/infrastructure/persistence/adapters/types.go)

### Common Helpers

Use helper functions for common operations:

```go
import "github.com/viewra/viewra/internal/infrastructure/persistence/common"

// Driver detection
if common.IsPostgres(driver) { /* ... */ }
if common.IsSQLite(driver) { /* ... */ }

// NULL-safe constructors
nullInt := common.NullInt64(someValue)
nullFloat := common.NullFloat64(someFloat)
nullStr := common.NullString(someString)

// Time parsing
parsedTime := common.ParseNullTime(result.CreatedAt)
```

**Location:** [internal/infrastructure/persistence/common/helpers.go](../internal/infrastructure/persistence/common/helpers.go)

### SQL Query Organization

#### Directory Structure

SQL queries must be organized by database type in separate directories:

```
internal/infrastructure/database/queries/
├── postgres/
│   ├── library.sql
│   ├── media.sql
│   └── [feature].sql
└── sqlite/
    ├── library.sql
    ├── media.sql
    └── [feature].sql
```

#### Benefits of Separation

1. **Scalability** - Easy to add database-specific optimizations
2. **Clarity** - No more `_postgres` suffix confusion
3. **Maintainability** - Clear separation of concerns
4. **Parallel Development** - Teams can work on different database implementations independently

#### Adding New Queries

When adding new SQL queries:

1. Create the query file in **both** `postgres/` and `sqlite/` directories
2. Use the same filename in both locations (e.g., `user.sql`)
3. Write database-specific SQL as needed (PostgreSQL and SQLite have different syntax)
4. Update `sqlc.yaml` if adding a new query file
5. Run `sqlc generate` to regenerate Go code
6. Implement repository methods using the generated code

**Example:**

```bash
# Add new feature queries
touch internal/infrastructure/database/queries/postgres/user.sql
touch internal/infrastructure/database/queries/sqlite/user.sql

# Edit both files with appropriate SQL for each database
# Then regenerate
sqlc generate
```

#### Query Naming Conventions

- Use descriptive query names: `GetUserByEmail`, `ListActiveLibraries`
- Follow SQLC naming conventions (capitalized for exported functions)
- Keep parameter names consistent between PostgreSQL and SQLite versions
- Use the same query name in both database implementations when possible

---

## Testing Conventions

### File Naming

- Test files must end with `_test.go`
- Test files should mirror the structure of implementation files:
  - `types.go` → `types_test.go`
  - `repository.go` → `repository_test.go`

### Test Structure

```go
func TestFunctionName(t *testing.T) {
	// Arrange
	// ... setup

	// Act
	// ... execute

	// Assert
	// ... verify
}
```

### Table-Driven Tests

Prefer table-driven tests for multiple scenarios:

```go
func TestLibraryType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		libType  LibraryType
		expected bool
	}{
		{"valid movies", LibraryTypeMovies, true},
		{"valid tv", LibraryTypeTV, true},
		{"invalid type", LibraryType("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.libType.IsValid()
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
```

---

## Code Style

### Formatting

- Use `gofmt` for all Go code (tabs for indentation)
- Line length: aim for 100 characters, hard limit at 120
- Use `goimports` to organize imports

### Comments

- All exported types, functions, and methods must have doc comments
- Doc comments start with the name of the item: `// Library represents...`
- Use complete sentences with proper punctuation

**Example:**

```go
// Library represents a media library containing movies, TV shows, or music.
// It enforces validation rules on creation and modification.
type Library struct {
	ID        int64
	Name      string
	Path      string
	Type      LibraryType
	CreatedAt time.Time
	UpdatedAt time.Time
}
```

### Error Handling

- Always handle errors explicitly
- Return errors up the stack; handle at appropriate level
- Use domain-specific errors for business logic failures

```go
// Domain error definition
var ErrLibraryNotFound = errors.New("library not found")

// Usage in repository
if errors.Is(err, sql.ErrNoRows) {
	return nil, library.ErrLibraryNotFound
}
```

---

## Summary Checklist

When creating a new package, ensure:

- [ ] Types are defined in `types.go`
- [ ] Interfaces are defined in appropriate files (`repository.go` or `interfaces.go`)
- [ ] Implementation methods are separate from type definitions
- [ ] Domain packages have no infrastructure dependencies
- [ ] Infrastructure packages implement domain interfaces
- [ ] All exported items have doc comments
- [ ] Tests follow naming convention (`*_test.go`)
- [ ] Code is formatted with `gofmt`

---

## References

- [ADR 001: Dual Database Support Strategy](./decisions/001-dual-database-support.md)
- [Project Plan](./PROJECT_PLAN.md)
- [Effective Go](https://golang.org/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
