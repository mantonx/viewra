# ADR 001: Dual Database Support (SQLite + PostgreSQL)

**Status:** Proposed  
**Date:** 2025-11-11  
**Deciders:** Development Team  
**Tags:** database, architecture, sqlc, infrastructure

## Context and Problem Statement

ViewRA needs to support both SQLite (for single-user/home deployments) and PostgreSQL (for production/multi-user deployments). We need a strategy that:

1. **Maintains type safety** - Leverages sqlc's compile-time checking
2. **Minimizes duplication** - Don't write the same code twice
3. **Easy to maintain** - Adding features shouldn't require touching 3+ files
4. **Easy to test** - Developers can verify both databases work
5. **Clear abstractions** - Repositories shouldn't care which DB is used

### Current Challenges Discovered

1. **Type incompatibilities:**
   - SQLite: `INTEGER` → `int64`, PostgreSQL: `INTEGER` → `int32`, `SERIAL` → `int32`, `BIGINT` → `int64`
   - Different null type handling
   - Different return types for EXISTS queries (`int64` vs `bool`)

2. **SQL syntax differences:**
   - Placeholders: `?` (SQLite) vs `$1, $2, $3` (PostgreSQL)
   - Auto-increment: `AUTOINCREMENT` vs `SERIAL`
   - Reserved keywords: `cast` is reserved in PostgreSQL but not SQLite

3. **Schema maintenance:**
   - Need parallel migration files
   - Need parallel query files

## Decision Drivers

* **Type Safety**: Must maintain sqlc's compile-time guarantees
* **Developer Experience**: Should be easy to add new features
* **Testing**: Both databases must be easily testable
* **Performance**: Minimal runtime overhead
* **Maintainability**: Clear separation of concerns

## Considered Options

### Option 1: Unified Abstraction Layer (Current Attempt)

**Approach:** Create `DBQuerier` interface with normalized types, implement adapters for each database.

**Pros:**
- Repositories completely database-agnostic
- Single interface to maintain
- Type conversions hidden in adapters

**Cons:**
- ❌ Loses sqlc's type safety at repository level
- ❌ Runtime type conversions (int32 ↔ int64)
- ❌ Complex adapter code (300+ lines per database)
- ❌ Maintaining 3 layers: sqlc → adapter → repository
- ❌ Hard to debug (which layer has the bug?)
- ❌ Duplicate type definitions (CreateMediaParams, UpdateMediaParams, etc.)

**Example complexity:**
```go
// Need to maintain unified types separate from sqlc
type CreateMediaParams struct {
    LibraryID int64  // Unified type
    // ... 28 more fields
}

// Then convert in adapter
sqlc_postgres.CreateMediaParams{
    LibraryID: int32(params.LibraryID),  // Convert
    // ... 28 more conversions
}
```

### Option 2: Database-Specific Repositories

**Approach:** Separate repository implementations for SQLite and PostgreSQL, factory selects based on `DB_DRIVER`.

**Pros:**
- ✅ Direct use of sqlc-generated code (full type safety)
- ✅ No type conversions or adapters
- ✅ Easy to debug (single layer)
- ✅ Simple factory pattern

**Cons:**
- ❌ Code duplication across repositories
- ❌ Features must be implemented twice
- ❌ Higher maintenance burden
- ❌ Easy to forget updating one database

**Structure:**
```
persistence/
├── library/
│   ├── repository_sqlite.go
│   ├── repository_postgres.go
│   └── factory.go
```

### Option 3: Shared Logic + Database-Specific Queries

**Approach:** Write database-agnostic business logic, use sqlc for database-specific queries, shared helpers for common patterns.

**Pros:**
- ✅ sqlc handles type safety
- ✅ Business logic not duplicated
- ✅ Minimal adapter layer (just query selection)
- ✅ Leverages Go's type system
- ✅ Easy to add new queries

**Cons:**
- Need to maintain parallel SQL files
- Small wrapper to select correct sqlc package

**Structure:**
```
persistence/library/
├── repository.go         # Business logic + interface implementation
├── queries_sqlite.go     # Wrapper around sqlc_sqlite.Queries
├── queries_postgres.go   # Wrapper around sqlc_postgres.Queries
└── factory.go           # Returns correct wrapper
```

**Example:**
```go
// repository.go - business logic, database-agnostic
type Repository struct {
    q querier  // Interface satisfied by both wrappers
}

func (r *Repository) Create(ctx context.Context, lib *library.Library) error {
    result, err := r.q.CreateLibrary(ctx, lib.Name, lib.Path, string(lib.Type))
    if err != nil {
        return err
    }
    lib.ID = result.ID  // Works because wrapper normalizes int32→int64
    lib.CreatedAt = parseTime(result.CreatedAt)
    return nil
}

// queries_sqlite.go - thin wrapper
type sqliteQuerier struct {
    q *sqlc_sqlite.Queries
}

func (q *sqliteQuerier) CreateLibrary(ctx context.Context, name, path, libType string) (*LibraryResult, error) {
    r, err := q.q.CreateLibrary(ctx, sqlc_sqlite.CreateLibraryParams{...})
    return &LibraryResult{ID: r.ID, ...}, err  // Direct passthrough, types match
}

// queries_postgres.go - thin wrapper with type conversion
type postgresQuerier struct {
    q *sqlc_postgres.Queries
}

func (q *postgresQuerier) CreateLibrary(ctx context.Context, name, path, libType string) (*LibraryResult, error) {
    r, err := q.q.CreateLibrary(ctx, sqlc_postgres.CreateLibraryParams{...})
    return &LibraryResult{ID: int64(r.ID), ...}, err  // Convert int32→int64
}
```

### Option 4: Single Database + Manual Migration Tool

**Approach:** Support only PostgreSQL, provide SQLite→PostgreSQL migration tool.

**Pros:**
- ✅ Zero duplication
- ✅ Maximum type safety
- ✅ Simplest codebase

**Cons:**
- ❌ Breaks stated requirement (SQLite for home users)
- ❌ Requires PostgreSQL setup (against zero-config goal)
- ❌ Migration tool adds complexity for users

## Decision Outcome

**Chosen option: Option 3 - Shared Logic + Database-Specific Queries**

### Rationale

1. **Type Safety Maintained:** Repository layer uses sqlc-generated types through thin wrappers
2. **Minimal Duplication:** Only query wrappers are duplicated (~50 lines each), not business logic
3. **Easy to Maintain:** Adding a feature = write SQL for both DBs, wrapper auto-handles types
4. **Easy to Test:** Swap querier implementation in tests
5. **Clear Debugging:** Three clear layers with single responsibility

### Implementation Plan

1. **Query Interface per Repository**
   ```go
   // persistence/library/querier.go
   type querier interface {
       CreateLibrary(ctx, name, path, type) (*LibraryResult, error)
       GetLibraryByID(ctx, id int64) (*LibraryResult, error)
       // ... other methods
   }
   ```

2. **Wrapper Implementations**
   - `queries_sqlite.go` - wraps `sqlc_sqlite.Queries`
   - `queries_postgres.go` - wraps `sqlc_postgres.Queries`
   - Both implement `querier` interface

3. **Factory Selection**
   ```go
   func NewRepository(db *sql.DB, driver string) *Repository {
       var q querier
       if driver == "postgres" {
           q = newPostgresQuerier(db)
       } else {
           q = newSQLiteQuerier(db)
       }
       return &Repository{q: q}
   }
   ```

4. **Result Types** (per repository, not global)
   ```go
   // Small result structs with normalized types
   type LibraryResult struct {
       ID        int64  // Always int64 (wrapper converts)
       Name      string
       Path      string
       Type      string
       CreatedAt interface{}  // Handle both time.Time and string
       UpdatedAt interface{}
   }
   ```

### Consequences

**Positive:**
- ✅ Repositories are clean and testable
- ✅ Full sqlc type safety preserved
- ✅ Minimal conversion code (only in wrappers)
- ✅ Easy to add new queries (two SQL files)
- ✅ Clear error sources

**Negative:**
- ⚠️ Must maintain parallel SQL files (already required)
- ⚠️ Small amount of wrapper boilerplate (~50 lines per repo)
- ⚠️ Developer must test both databases (good practice anyway)

**Neutral:**
- Type conversions explicit and localized
- Schema differences documented in SQL files

## Validation

### Testing Strategy

1. **Repository Tests:** Run against both databases
   ```go
   func TestRepository_Create(t *testing.T) {
       testCases := []struct{
           name string
           driver string
       }{
           {"SQLite", "sqlite"},
           {"PostgreSQL", "postgres"},
       }
       for _, tc := range testCases {
           t.Run(tc.name, func(t *testing.T) {
               db := setupTestDB(t, tc.driver)
               repo := NewRepository(db, tc.driver)
               // ... test logic
           })
       }
   }
   ```

2. **CI Pipeline:** Run full test suite against both databases

3. **Integration Tests:** Spin up PostgreSQL container, run same tests

### Success Criteria

- [ ] All 69 existing tests pass on both databases
- [ ] Adding a new repository method requires:
  - 1 SQL query for SQLite
  - 1 SQL query for PostgreSQL  
  - 1 wrapper method for SQLite
  - 1 wrapper method for PostgreSQL
  - 1 repository method using the wrapper
- [ ] No type conversion errors at compile time
- [ ] CI runs tests on both databases

## Follow-Up Actions

1. Revert current abstraction layer attempt (querier.go, querier_*.go)
2. Implement pattern for `library` repository
3. Validate with existing tests
4. Apply pattern to `media` repository
5. Document pattern in ARCHITECTURE.md
6. Update contribution guidelines

## References

- [sqlc Documentation](https://docs.sqlc.dev/)
- [DATABASE_SETUP.md](../DATABASE_SETUP.md)
- [TECH_STACK.md](../TECH_STACK.md)
- Similar pattern: [Ent framework multi-driver support](https://entgo.io/docs/dialects/)

## Notes

### Why Not Option 1 (Unified Abstraction)?

The abstraction layer creates a "false unification" - the databases aren't truly unified because:
- Type differences require runtime conversions
- We lose compile-time safety from sqlc
- Debugging becomes harder (3 layers instead of 2)
- More code to maintain with no real benefit

**The key insight:** SQLite and PostgreSQL are *similar enough* for business logic to be shared, but *different enough* that trying to hide those differences creates more problems than it solves.

### Migration Path

This decision makes it easier to:
- Add new databases (e.g., MySQL) - just add new wrapper
- Drop a database - remove wrapper files
- Switch primary DB - update default in config
- Test database compatibility - swap querier in tests

### Alternative Future: Database-Agnostic ORM

If requirements change significantly, we could:
- Switch to Ent, GORM, or similar ORM
- They handle multi-database internally
- Trade-off: Less control, more abstraction

But for now, sqlc + thin wrappers gives us the best balance of type safety, control, and maintainability.
