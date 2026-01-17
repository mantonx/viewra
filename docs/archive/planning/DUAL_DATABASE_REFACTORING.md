# Dual Database (PostgreSQL/SQLite) Refactoring Plan

**Status**: Complete  
**Last Updated**: 2025-12-30

## Executive Summary

This document outlines the plan to consolidate dual-database support while keeping sqlc and maintaining full PostgreSQL/SQLite compatibility. The goal was to eliminate ~60% of repository code duplication by unifying types and creating a single Querier interface.

**Final Result**: All phases completed successfully. Repository code reduced by ~45%, branching reduced from 162 to 13 instances (all legitimate database-specific SQL).

## Background

### Original Problem (December 2025)
- SQLC generated different types: PostgreSQL (`int32`, `sql.NullInt32`) vs SQLite (`int64`, `sql.NullInt64`)
- 79+ instances of `router.IsPostgresDB()` branching
- Duplicate converters and param builders in every repository

### Phase 1 Completed ✅
We implemented SQLC type overrides in `sqlc.yaml`:
- `int4`, `int2`, `serial` → `int64`
- `bool` → `int64` (SQLite uses INTEGER for booleans)
- `jsonb`, `json` → `string`/`sql.NullString`

**Result**: Model structs and Row types are now **identical** between sqlite/postgres.

### Phase 2 Completed ✅
Fixed Limit/Offset parameter types and field ordering:

1. **Added `::bigint` casts** to all LIMIT/OFFSET parameters in PostgreSQL queries
2. **Created code generator** (`cmd/sqlc-gen/`) for post-processing and unified package generation
3. **Created unified database package** with type aliases and generated Querier

**Result**: Param structs now have identical types AND field order between sqlite/postgres.

### Phase 3 Completed ✅
Migrated all 22 repository packages to use the unified Querier pattern:

| Package | Status |
|---------|--------|
| movie | ✅ Complete |
| media | ✅ Complete |
| image | ✅ Complete |
| people | ✅ Complete |
| studios | ✅ Complete |
| keywords | ✅ Complete |
| location | ✅ Complete |
| search | ✅ Complete |
| analytics | ✅ Complete |
| library | ✅ Complete |
| tvshow | ✅ Complete |
| music | ✅ Complete |
| enrichment | ✅ Complete |
| scanjob | ✅ Complete |
| scanstate | ✅ Complete |
| scheduler | ✅ Complete |
| settings | ✅ Complete |
| user | ✅ Complete |
| progress | ✅ Complete |
| plugins | ✅ Complete |
| transcode | ✅ Complete |
| transcode_analytics | ✅ Complete |

### Phase 4 Completed ✅
Cleanup of deprecated code:

**Files Removed:**
- `internal/infrastructure/persistence/common/generic_mapper.go` - Unused reflection-based mapper
- `internal/infrastructure/persistence/common/router.go` - QueryRouter no longer needed
- `internal/infrastructure/persistence/common/router_test.go` - Tests for removed router

**Files Simplified:**
- `base_repository.go` - Reduced from 258 to 54 lines
  - Removed `sqlite`/`postgres` fields and methods
  - Removed `router` field and `Router()` method
  - Removed `QuerySingle()`, `QueryMany()`, `QueryScalar()`, `ExecuteCommand()` helpers
- `transaction.go` - Added `Q()` method for unified querier access in transactions

### Phase 5 Completed ✅
Additional infrastructure migrations:

- `internal/infrastructure/plugins/querier/` - All files migrated
- `internal/infrastructure/plugins/host/storage.go` - Migrated to unified Querier

## Final Metrics

| Metric | Before | After |
|--------|--------|-------|
| Router branching instances | 162 | 13 |
| Duplicate converters | ~200 | 0 |
| Duplicate param builders | ~100 | 0 |
| `r.Q()` unified calls | 0 | 380+ |
| BaseRepository LOC | 258 | 54 |

### Remaining Branching (13 instances)
All remaining branching is for legitimate database-specific SQL:
- Raw SQL placeholder syntax (`$1, $2` vs `?, ?`) - 1 instance
- PostgreSQL interval syntax - 1 instance  
- PostgreSQL pgvector operations - 10 instances
- Helper method `isPostgres()` - 1 instance

## Migration Pattern

The final pattern used for all repositories:

```go
// repository.go
package example

import (
    "github.com/mantonx/viewra/internal/infrastructure/database/unified"
    "github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

type Repository struct {
    *common.BaseRepository
}

func NewRepository(db *common.BaseRepository) *Repository {
    return &Repository{BaseRepository: db}
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*Entity, error) {
    row, err := r.Q().GetEntityByID(ctx, id)
    if err != nil {
        return nil, r.ConvertNotFoundError(err)
    }
    return rowToDomain(row), nil
}

func (r *Repository) Create(ctx context.Context, entity *Entity) error {
    return r.Q().CreateEntity(ctx, buildCreateParams(entity))
}
```

```go
// converters.go - Single converter per entity type
func rowToDomain(row unified.GetEntityByIDRow) *Entity {
    return &Entity{
        ID:   row.ID,
        Name: row.Name,
        // ...
    }
}

func buildCreateParams(e *Entity) unified.CreateEntityParams {
    return unified.CreateEntityParams{
        Name: e.Name,
        // ...
    }
}
```

## Transaction Support

Transactions now use `tx.Q()` for unified access:

```go
err := common.WithTransaction(r.BaseRepository, ctx, func(tx *common.TransactionContext) error {
    // Create artist
    artist, err := tx.Q().CreateArtist(ctx, artistParams)
    if err != nil {
        return err
    }
    
    // Create album
    _, err = tx.Q().CreateAlbum(ctx, albumParams)
    return err
})
```

## Success Metrics - Final Status

- [x] All repositories using unified.Querier
- [x] 0 duplicate converter functions
- [x] 0 duplicate param builder functions  
- [x] <20 router branching instances (achieved: 13, all legitimate)
- [x] All tests passing
- [x] ~45% reduction in persistence layer code duplication

## Files Reference

### Code Generator
- `cmd/sqlc-gen/main.go` - Entry point and orchestration
- `cmd/sqlc-gen/normalize.go` - Normalizes struct field order between PG/SQLite
- `cmd/sqlc-gen/types.go` - Generates unified type aliases
- `cmd/sqlc-gen/querier.go` - Generates unified Querier wrapper

### Created
- `internal/infrastructure/database/unified/unified.go` - Helper functions
- `internal/infrastructure/database/unified/types.go` - Generated type aliases
- `internal/infrastructure/database/unified/querier_gen.go` - Generated unified Querier

### Deleted
- `internal/infrastructure/persistence/common/generic_mapper.go`
- `internal/infrastructure/persistence/common/router.go`
- `internal/infrastructure/persistence/common/router_test.go`

### Simplified
- `internal/infrastructure/persistence/common/base_repository.go` (258 → 54 LOC)
- `internal/infrastructure/persistence/common/transaction.go` (added Q() method)

## Progress Log

### 2025-12-30
- Phase 1 completed: SQLC type overrides working
- Phase 2 completed: Unified package created
- Phase 3 completed: All 22 repository packages migrated
- Phase 4 completed: Deprecated code removed
- Phase 5 completed: Plugin infrastructure migrated
- **Project complete!**
