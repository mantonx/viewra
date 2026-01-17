# Technical Debt

**Last Updated**: January 17, 2026

## Overview

This document tracks known technical debt. For the detailed audit history (Phase 1-4 completed), see [archive/planning/TECHNICAL_DEBT.md](../archive/planning/TECHNICAL_DEBT.md).

## Summary

- **Original audit**: November 22, 2025 - 31 TODO/HACK comments
- **Phases 1-4 completed**: December 29, 2025 - 17 items resolved
- **Remaining items**: 14 (all P2/P3 priority)
- **Debt ratio**: 0.02% (14 comments across ~71k LOC)

## Remaining Items

### Medium Priority (P2)

#### File Hash Field

**Location**: `internal/infrastructure/persistence/media/types.go:80`

```go
FileHash: sql.NullString{}, // TODO: Add Hash field to domain.Media
```

File hash exists in database but not mapped to domain model.

**Effort**: 1 hour | **Value**: Low

### Low Priority (P3)

#### Stereo 3D Detection (3 items)

**Locations**:

- `internal/infrastructure/persistence/media/types.go:98-99`
- `internal/infrastructure/persistence/media/types.go:134-135`

3D format and stereo mode not auto-detected from files.

**Effort**: 2 hours | **Value**: Low (niche use case)

## Architectural Improvements (Deferred)

### 1. ai-search Plugin SDK Refactoring

**Location**: `plugins/ai-search/`
**Effort**: 8-12 hours | **Risk**: High

The ai-search plugin has ~200 lines of boilerplate that could use SDK patterns.

**Why Deferred**: High-risk change affecting plugin-host communication.

### 2. Domain Layer Architecture Violations

**Locations**: `internal/domain/library/repository.go`, `internal/domain/media/repository.go`
**Effort**: 6-10 hours | **Risk**: Medium (52 call sites)

Domain interfaces import `database/sql` and use `*sql.Tx` in method signatures.

**Why Deferred**: Significant refactoring affecting multiple files.

### 3. golang.org/x/text in Domain Layer

**Location**: `internal/domain/common/text.go`
**Status**: Accepted exception

Used for Unicode text normalization. The `golang.org/x/text` packages are quasi-stdlib with no I/O dependencies.

## Code Quality

**Score**: 7.5/10

- All critical features implemented
- Remaining debt is "nice-to-have" enhancements
- No critical bugs or security issues
- Excellent debt ratio (0.02%) compared to industry average (2-5%)
