# Dual Database (PostgreSQL/SQLite) Refactoring Plan

## Problem Statement

The codebase supports both PostgreSQL and SQLite through SQLC-generated code. This results in significant code duplication because SQLC generates separate types for each database with different integer sizes:
- PostgreSQL: `int32`, `sql.NullInt32`
- SQLite: `int64`, `sql.NullInt64`

### Current State

- **79 instances** of `router.IsPostgresDB()` branching across the codebase
- Major affected files:
  - `internal/infrastructure/plugins/media_querier.go` (1,736 lines, 20+ branches)
  - `internal/infrastructure/persistence/library/repository.go` (8 branches)
  - `internal/infrastructure/persistence/media/repository.go` (8 branches)
  - `internal/infrastructure/persistence/scanjob/*.go` (10+ branches)

### Existing Solutions

1. **Reflection-based generic mapper** (`common/generic_mapper.go`)
   - Used in `persistence/music/types.go` (74 calls) and `persistence/tvshow/types.go` (62 calls)
   - Pros: DRY, works with any SQLC type
   - Cons: **Slow** - reflection overhead on every field access

2. **Manual duplication** (most of codebase)
   - Pros: Type-safe, fast, explicit
   - Cons: Verbose, error-prone when updating

## Proposed Solutions

### Option 1: Type-Safe Row Adapters (Recommended)

Create unified row structs in `internal/infrastructure/persistence/common/` with constructor functions for each database type.

```go
// common/row_adapters.go

// MovieRow is a unified type for movie data from either database.
type MovieRow struct {
    MediaID          int64
    LibraryID        int64
    Title            string
    Year             sql.NullInt64
    Plot             sql.NullString
    // ... other fields
}

// FromPostgresMovie creates MovieRow from PostgreSQL GetMovieByMediaIDRow.
func FromPostgresMovie(row sqlc_postgres.GetMovieByMediaIDRow, title string) MovieRow {
    return MovieRow{
        MediaID:   int64(row.MediaID),
        LibraryID: int64(row.LibraryID),
        Title:     title,
        Year:      NullInt32ToInt64(row.Year),
        Plot:      row.Plot,
    }
}

// FromSQLiteMovie creates MovieRow from SQLite GetMovieByMediaIDRow.
func FromSQLiteMovie(row sqlc_sqlite.GetMovieByMediaIDRow, title string) MovieRow {
    return MovieRow{
        MediaID:   row.MediaID,
        LibraryID: row.LibraryID,
        Title:     title,
        Year:      row.Year,
        Plot:      row.Plot,
    }
}
```

**Usage:**
```go
func (q *DBMediaQuerier) movieRowToDetails(result any, externalIDs map[string]string) *MediaDetailsInfo {
    var row common.MovieRow
    if q.router.IsPostgresDB() {
        row = common.FromPostgresMovie(result.(sqlc_postgres.GetMovieByMediaIDRow), "")
    } else {
        row = common.FromSQLiteMovie(result.(sqlc_sqlite.GetMovieByMediaIDRow), "")
    }
    
    // Single conversion logic - no duplication
    return &MediaDetailsInfo{
        ID:        row.MediaID,
        Title:     row.Title,
        Year:      common.NullInt64Value(row.Year),
        Plot:      common.NullStringValue(row.Plot),
        // ...
    }
}
```

**Pros:**
- Type-safe, no reflection
- Fast - conversion happens once at boundary
- Conversion logic is DRY
- Easy to test adapters in isolation

**Cons:**
- Need adapter struct per SQLC row type (~20-30 types)
- Two constructor functions per adapter
- Import cycle risk if adapters are in wrong package

### Option 2: Code Generation

Create a code generator that reads SQLC output and generates unified types.

```bash
go generate ./internal/infrastructure/persistence/common/...
```

**Pros:**
- Fully automated
- Guaranteed consistency with SQLC types

**Cons:**
- Complex to implement
- Another build step
- Need to maintain the generator

### Option 3: SQLC Plugin/Override

Configure SQLC to use `int64` for PostgreSQL integer types.

```yaml
# sqlc.yaml
overrides:
  - db_types: ["int4", "int2", "serial"]
    go_type: "int64"
```

**Pros:**
- Fixes at the source
- No additional code needed

**Cons:**
- May break existing code expecting int32
- Doesn't solve NullInt32 vs NullInt64
- Need to verify SQLC supports this fully

### Option 4: Accept Duplication (Current Approach)

Keep the duplication but organize it better by splitting large files.

**Pros:**
- No abstraction overhead
- Explicit and debuggable
- No new patterns to learn

**Cons:**
- Verbose
- Risk of bugs when updating one branch but not the other

## Recommendation

**Short-term (Now):** Option 4 - Split large files for organization without adding abstraction.

**Medium-term (Future Sprint):** Option 1 - Implement type-safe row adapters for the most duplicated types:
1. MovieRow
2. TVShowRow
3. TVEpisodeRow
4. MusicTrackRow
5. MusicAlbumRow
6. MusicArtistRow
7. MediaRow (basic media info)
8. LibraryRow

**Long-term:** Evaluate Option 3 (SQLC overrides) when upgrading SQLC versions.

## Files to Refactor (Priority Order)

1. `internal/infrastructure/plugins/media_querier.go` - Split now, adapt later
2. `internal/infrastructure/persistence/library/repository.go`
3. `internal/infrastructure/persistence/media/repository.go`
4. `internal/infrastructure/persistence/scanjob/repository.go`

## Migration Strategy

1. Create adapter types in `common/row_adapters.go`
2. Add helper functions: `NullInt32ToInt64`, `NullInt64Value`, etc.
3. Migrate one file at a time, starting with `media_querier.go`
4. Remove reflection-based `generic_mapper.go` usage once adapters are in place
5. Add benchmarks to verify performance improvement over reflection

## Estimated Effort

- **Option 1 (Adapters):** 2-3 days for full implementation
- **Option 4 (Split files):** 2-3 hours

## Related Files

- `internal/infrastructure/persistence/common/generic_mapper.go` - Existing reflection solution
- `internal/infrastructure/persistence/common/router.go` - QueryRouter for db branching
- `internal/infrastructure/persistence/common/helpers.go` - Null type helpers
