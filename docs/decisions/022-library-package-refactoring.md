# ADR 022: Library Package Refactoring and Simplification

**Status**: Proposed
**Date**: 2025-11-22
**Deciders**: Development Team
**Related**: ADR 011 (Architectural Improvements Phase 1)

## Context

Following ADR 011's successful architectural improvements, we conducted a comprehensive analysis of the library package using specialized analysis agents (Explore and Complexity-Critic). The library package has grown significantly during feature development and now exhibits several complexity issues:

### Current State Analysis

**Package Structure** (3,273 total lines across 18 files):
- **Application Layer** (`internal/application/library/`): 4,887 lines
  - 5 CRUD use cases: 289 lines (~58 lines each)
  - 1 massive scan orchestrator: `scan_library.go` (1,871 lines)
  - Supporting files: DTOs, interfaces, incremental scanner
  - 11 test files: ~1,700 lines

- **Domain Layer** (`internal/domain/library/`): 1,151 lines
  - Entity, service, repository interface, types, errors
  - Well-organized, no major issues

- **Persistence Layer** (`internal/infrastructure/persistence/library/`): 707 lines
  - Repository implementation with dual-database support
  - Reasonable size and structure

### Key Problems Identified

#### 1. **God Object Use Case** (CRITICAL)
`ScanLibraryUseCase` has spiraled into a 1,871-line file with 21 dependencies:
- 14 repository dependencies
- 6 separate image extractor interfaces
- Config, logger, system profile
- 109-line constructor
- Handles: file discovery, checkpointing, hashing, processing, image extraction, cleanup, error handling, progress tracking

**Impact**: Impossible to test, modify, or understand. Violates Single Responsibility Principle.

#### 2. **Interface Explosion** (HIGH)
Separate executor interfaces for trivial operations:
- `CreateLibraryExecutor`, `GetLibraryExecutor`, `UpdateLibraryExecutor`, `DeleteLibraryExecutor`, `ListLibrariesExecutor`
- Each interface has exactly ONE implementation
- Each wraps a single repository call with minimal business logic
- Forces 3-layer navigation (Handler → UseCase → Repository) for simple reads

**Impact**: ~150 lines of ceremony code providing zero value.

#### 3. **Six Redundant Image Extractors** (HIGH)
Six nearly identical interfaces differing only in parameter names:
```go
type ExtractMovieImagesExecutor interface { ... }
type ExtractTVEpisodeImagesExecutor interface { ... }
type ExtractTVShowImagesExecutor interface { ... }
type ExtractTVSeasonImagesExecutor interface { ... }
type ExtractMusicAlbumImagesExecutor interface { ... }
type ExtractMusicArtistImagesExecutor interface { ... }
```

All perform the same operation: extract images for a media entity. The only differences are parameter names and one optional `seasonNumber` field.

**Impact**: Boilerplate explosion, difficult to extend, violates DRY.

#### 4. **Duplicate DTO Conversion Logic** (MEDIUM)
Identical scan job → DTO conversion exists in both:
- `internal/api/handlers/scanjob.go:109-128`
- `internal/application/library/scan_dto.go:56-74`

**Impact**: Maintenance burden, potential for drift.

### Complexity Metrics

**Complexity-Critic Assessment**: HIGH complexity for project scale
- Estimated 30-40% of library package code could be eliminated
- Current: 3,273 lines across 18 files
- Target: ~2,000-2,200 lines across 10-12 focused files
- Potential savings: 1,000-1,300 lines without losing functionality

## Decision

We will refactor the library package to reduce complexity, improve maintainability, and eliminate unnecessary abstractions while preserving clean architecture principles and functionality.

### Refactoring Actions

#### Action 1: Split `scan_library.go` into Focused Components

**Before**: 1,871 lines in single file

**After**: Split into 4-5 focused files:

```
internal/application/library/
├── scan_orchestrator.go      (~300 lines)
│   └── High-level scan coordination and state management
├── scan_file_processor.go    (~400 lines)
│   └── File discovery, checkpointing, hashing logic
├── scan_media_handlers.go    (~450 lines)
│   └── processMovie(), processTVEpisode(), processMusicTrack()
├── scan_image_extraction.go  (~300 lines)
│   └── Image extraction helpers and coordination
└── scan_cleanup.go           (~200 lines)
    └── Stale media cleanup and finaliz ation
```

**Rationale**: Each file focuses on one aspect of scanning, making the codebase navigable and testable.

#### Action 2: Consolidate CRUD Use Cases into Service

**Before**: 5 separate use case files (~150 lines total)
- `create_library.go` (73 lines)
- `get_library.go` (30 lines)
- `list_libraries.go` (30 lines)
- `update_library.go` (72 lines)
- `delete_library.go` (84 lines)

**After**: Single `library_service.go` (~120 lines)

```go
type LibraryService struct {
    repo      library.Repository
    txManager *TxManager
    logger    *slog.Logger
}

func (s *LibraryService) Create(ctx context.Context, req CreateLibraryRequest) (LibraryResponse, error) {
    // Business logic directly in method
}

func (s *LibraryService) Get(ctx context.Context, id int64) (LibraryResponse, error) {
    lib, err := s.repo.GetByID(ctx, id)
    if err != nil {
        return LibraryResponse{}, err
    }
    return ToLibraryResponse(lib), nil
}

// Update, Delete, List methods...
```

**Rationale**:
- Get/List are trivial pass-throughs with zero business logic
- Create/Update/Delete have business logic that warrants methods, not separate files
- Single service is easier to navigate than 5 scattered files
- Maintains testability while reducing ceremony

#### Action 3: Unify Image Extractor Interfaces

**Before**: 6 separate interfaces

**After**: Single interface with options struct

```go
type ImageExtractor interface {
    Extract(ctx context.Context, opts ImageExtractionOptions) error
}

type ImageExtractionOptions struct {
    FilePath   string
    MediaType  images.MediaType
    EntityType EntityType  // Movie, TVShow, TVSeason, TVEpisode, MusicAlbum, MusicArtist
    EntityID   int
    MediaID    *int         // Optional: only for episodes/tracks
    SeasonNum  int          // Optional: only for TV seasons
}

type EntityType int

const (
    EntityTypeMovie EntityType = iota
    EntityTypeTVShow
    EntityTypeTVSeason
    EntityTypeTVEpisode
    EntityTypeMusicAlbum
    EntityTypeMusicArtist
)
```

**Rationale**:
- Interfaces are 95% identical - consolidation is obvious
- Options struct provides flexibility for entity-specific needs
- Reduces ScanLibraryUseCase constructor from 21 to 16 parameters
- Easier to add new media types (e.g., audiobooks, photos)

#### Action 4: Bundle Related Repository Dependencies

**Before**: 14 individual repository fields in ScanLibraryUseCase

**After**: Group related repositories

```go
type MediaRepositories struct {
    Library    library.Repository
    Media      media.Repository
    Movie      media.MovieRepository
    TV         media.TVRepository
    Music      media.MusicRepository
}

type ScanRepositories struct {
    ScanJob     scanner.ScanJobRepository
    Checkpoint  scanner.CheckpointRepository
    ScanState   scanner.ScanStateRepository
}

type ScanLibraryUseCase struct {
    mediaRepos    *MediaRepositories
    scanRepos     *ScanRepositories
    imageRepo     images.Repository
    imageExtractor ImageExtractor
    scanner       *IncrementalScanner
    cleanup       ImageCleanupExecutor
    config        ScanConfig
    logger        *slog.Logger
}
```

**Rationale**:
- Reduces constructor parameters from 21 to 8
- Groups related dependencies logically
- Easier to mock in tests (mock entire repository group)
- Clearer dependency relationships

#### Action 5: Eliminate DTO Duplication

**Before**: Duplicate conversion in handlers and application layer

**After**: Single source of truth in application layer

```go
// In internal/application/library/scan_dto.go
func ToScanProgressResponse(job *scanner.ScanJob) ScanProgressResponse {
    return ScanProgressResponse{
        JobID:          job.ID,
        Status:         string(job.Status),
        Progress:       job.Progress,
        // ... all 15+ fields
    }
}

// Handlers use application layer function
// Delete duplicate code from handlers/scanjob.go
```

**Rationale**: DRY principle, single source of truth, consistent conversions.

### Implementation Phases

#### Phase 1: Split Scan Use Case (Week 1)
1. Create 5 new focused files
2. Move methods to appropriate files
3. Ensure all tests pass
4. No functional changes, pure refactoring

**Estimated Impact**: -1,500 lines (net after reorganization)

#### Phase 2: Consolidate CRUD (Week 1)
1. Create `library_service.go` with all CRUD methods
2. Update handlers to use service instead of separate use cases
3. Update container wiring
4. Delete 5 use case files
5. Ensure all tests pass

**Estimated Impact**: -150 lines, -4 files

#### Phase 3: Unify Image Extractors (Week 2)
1. Define unified `ImageExtractor` interface
2. Create `ImageExtractionOptions` struct
3. Update image extraction implementations
4. Refactor ScanLibraryUseCase to use single extractor
5. Update tests

**Estimated Impact**: -80 lines, cleaner interface

#### Phase 4: Bundle Repositories (Week 2)
1. Create `MediaRepositories` and `ScanRepositories` structs
2. Update ScanLibraryUseCase constructor
3. Update container builder
4. Update all usage sites

**Estimated Impact**: -50 lines, clearer architecture

#### Phase 5: Eliminate DTO Duplication (Week 2)
1. Keep application layer conversion
2. Update handlers to use application layer function
3. Delete duplicate code

**Estimated Impact**: -30 lines

### Total Expected Impact

**Code Reduction**:
- Before: 3,273 lines across 18 files
- After: ~2,100 lines across 11-12 files
- Savings: ~1,200 lines (37% reduction)
- Files removed: 6-7 files

**Complexity Reduction**:
- ScanLibraryUseCase constructor: 21 → 8 parameters
- Image extractor interfaces: 6 → 1
- CRUD files: 5 → 1
- Largest file: 1,871 lines → ~450 lines

**Maintainability Improvements**:
- ✅ Single Responsibility: Each file has clear, focused purpose
- ✅ Testability: Smaller components easier to test in isolation
- ✅ Navigability: Fewer files to understand, clearer organization
- ✅ Extensibility: Easier to add new media types or features

## Consequences

### Positive
- ✅ **Massive Complexity Reduction**: 37% fewer lines, 40% fewer files
- ✅ **Improved Maintainability**: Clear file boundaries, focused responsibilities
- ✅ **Better Testability**: Smaller components easier to unit test
- ✅ **Enhanced Developer Experience**: Less navigation, clearer architecture
- ✅ **Preserved Functionality**: Zero behavior changes, pure refactoring
- ✅ **Easier Extensibility**: Adding new media types or scan phases is simpler
- ✅ **Cleaner Dependencies**: Constructor parameters reduced from 21 to 8
- ✅ **DRY Compliance**: Eliminated duplicate conversion logic

### Negative
- ❌ **Refactoring Effort**: 2 weeks of focused refactoring work
- ❌ **Test Updates**: Need to update test imports and mocks
- ❌ **Review Overhead**: Large PR requiring thorough review
- ❌ **Temporary Disruption**: Active feature branches may need rebasing
- ❌ **Learning Curve**: Team needs to understand new structure

### Neutral
- ⚖️ **Architecture Preserved**: Clean architecture layers remain intact
- ⚖️ **Dual-DB Support**: QueryRouter pattern unchanged
- ⚖️ **Repository Pattern**: Interface-based repositories maintained
- ⚖️ **Domain Model**: No changes to domain entities or business rules

### Mitigations

**For Refactoring Risk**:
- Comprehensive test coverage before starting
- Refactor incrementally with CI validation at each step
- Use feature flags if needed to isolate changes

**For Team Disruption**:
- Clear communication about refactoring timeline
- Coordinate with team to minimize active work on library package
- Document new structure with ADR and code comments

**For Review Overhead**:
- Break into smaller PRs aligned with phases
- Provide detailed PR descriptions with before/after comparisons
- Include refactoring verification checklist

## What We're NOT Changing

**Preserved Architecture**:
- ✅ Clean architecture layers (domain → application → infrastructure)
- ✅ Dependency injection via builders
- ✅ Dual-database support (QueryRouter, BaseRepository)
- ✅ Repository pattern with domain interfaces
- ✅ Domain service layer
- ✅ SQLC code generation
- ✅ Structured logging with slog
- ✅ Error handling with sentinel errors
- ✅ Transaction support (from ADR 011)

**Preserved Functionality**:
- ✅ Library CRUD operations
- ✅ Incremental scanning with checkpoints
- ✅ Resume capability after crashes
- ✅ Parallel file discovery and processing
- ✅ Image extraction for all media types
- ✅ Stale media cleanup
- ✅ Progress tracking and reporting
- ✅ Error categorization and handling

## Alternatives Considered

### Alternative 1: Keep Current Structure
**What**: Leave library package as-is, accept complexity

**Pros**:
- Zero refactoring effort
- No risk of introducing bugs
- Team already familiar with structure

**Cons**:
- Complexity continues to grow
- 1,871-line file becomes increasingly unmaintainable
- New developers struggle to understand scan logic
- Adding features becomes progressively harder

**Why Not Chosen**: Technical debt will compound. Current complexity already hinders development velocity.

### Alternative 2: Complete Rewrite
**What**: Redesign scanning architecture from scratch

**Pros**:
- Opportunity to fix fundamental design issues
- Could introduce new patterns (e.g., pipeline, event-driven)
- Fresh start without legacy constraints

**Cons**:
- 4-6 weeks of development time
- High risk of introducing bugs
- May break existing integrations
- Loses battle-tested crash recovery logic
- Requires extensive testing

**Why Not Chosen**: Current architecture is fundamentally sound - it just needs reorganization, not replacement. Rewrite risk outweighs benefits.

### Alternative 3: Delete Executor Interfaces Entirely
**What**: Have handlers call repositories directly, no use case layer

**Pros**:
- Eliminates all ceremony code
- Simplest possible architecture
- Fastest to navigate

**Cons**:
- Violates clean architecture
- Business logic mixes with HTTP concerns
- Difficult to test without HTTP infrastructure
- Loses transaction boundary control

**Why Not Chosen**: While some executor interfaces are unnecessary (simple CRUD), the use case pattern remains valuable for complex operations like scanning. Our approach is targeted: consolidate trivial use cases, keep scanning use case but split it.

### Alternative 4: Extract Scanning to Separate Package
**What**: Move `scan_library.go` to `internal/application/scanner/` as a standalone package

**Pros**:
- Clear separation of concerns
- Scanning logic isolated from CRUD
- Could be reused for other entity types

**Cons**:
- Introduces new package with unclear boundaries
- Scanning is library-specific, not general-purpose
- Increases coupling between packages
- More complex import paths

**Why Not Chosen**: Scanning is intrinsically tied to library operations. Better to split within the package than create artificial package boundaries.

## Metrics for Success

### Code Quality (Must Achieve)
- ✅ Zero test failures after refactoring
- ✅ Zero behavior changes (functionally equivalent)
- ✅ All lint checks pass
- ✅ No new static analysis warnings

### Complexity Reduction (Target)
- 🎯 Largest file ≤ 500 lines (down from 1,871)
- 🎯 Total package ≤ 2,200 lines (down from 3,273)
- 🎯 ScanLibraryUseCase constructor ≤ 10 parameters (down from 21)
- 🎯 ≤ 12 files in library package (down from 18)

### Developer Experience (Subjective)
- 🎯 Team members report improved code navigability
- 🎯 New features added with less effort
- 🎯 Onboarding time for new developers reduced

### Performance (Maintain)
- ✅ Scan performance unchanged (±5%)
- ✅ Memory usage unchanged (±5%)
- ✅ Build time remains under 1 minute

## Implementation Plan

### Week 1: Foundation
- **Day 1**: Create new file structure, move scan orchestration logic
- **Day 2**: Move file processing and media handlers
- **Day 3**: Move image extraction and cleanup
- **Day 4**: Consolidate CRUD use cases into service
- **Day 5**: Update tests, validate all passing

### Week 2: Refinement
- **Day 1-2**: Unify image extractor interfaces
- **Day 3**: Bundle repository dependencies
- **Day 4**: Eliminate DTO duplication
- **Day 5**: Final testing and validation

### Week 3: Review & Deployment
- **Day 1-2**: Code review with team
- **Day 3**: Address review feedback
- **Day 4**: Integration testing
- **Day 5**: Merge and deploy

## Related Decisions

- **ADR 011**: Architectural Improvements Phase 1
  - Established transaction support
  - Fixed CORS security
  - Added context timeouts
  - This ADR continues architectural cleanup

- **ADR 010**: Container Refactoring Strategy
  - Eliminated god object anti-pattern
  - Established builder pattern
  - This ADR applies similar principles to library package

- **ADR 014**: Library Scanner Resilience Improvements
  - Added checkpoint-based resumability
  - Implemented incremental scanning
  - This refactoring preserves all resilience features

## References

- Explore Agent Analysis (2025-11-22): Comprehensive library package structure review
- Complexity-Critic Agent Analysis (2025-11-22): Detailed complexity assessment
- ADR 011: Architectural Improvements Phase 1
- ADR 010: Container Refactoring Strategy
- [Clean Architecture Principles](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Single Responsibility Principle](https://en.wikipedia.org/wiki/Single_responsibility_principle)

## Notes

### Why Now?

This refactoring is timely because:

1. **Foundation Solid**: ADR 011 fixed critical issues (transactions, CORS, timeouts). We can now focus on structure.
2. **Complexity Peak**: 1,871-line file is at the point where further growth becomes dangerous.
3. **Team Velocity**: Current complexity is slowing feature development. Refactoring will unlock velocity.
4. **Low Risk**: Pure refactoring with comprehensive tests - risk is manageable.

### Refactoring Philosophy

This ADR follows the principle: **Simplify relentlessly, but preserve what works.**

We're not changing:
- The scanning algorithm (it works well)
- The domain model (it's clean)
- The dual-database support (it's valuable)
- The transaction boundaries (recently added in ADR 011)

We're fixing:
- File organization (1 massive file → 5 focused files)
- Unnecessary abstractions (11 executor interfaces → 2-3 services)
- Dependency complexity (21 constructor params → 8)

### Success Definition

This refactoring succeeds if:
1. **All tests pass** - No behavior changes
2. **Code is shorter** - 30-40% reduction
3. **Team is happier** - Easier to navigate and modify
4. **Future features are faster** - Reduced friction

If we achieve these, we've validated the approach and can apply similar refactoring to other complex areas (e.g., media package, if needed).

---

**Decision**: Proposed
**Next Review**: After Phase 1 completion
**Owner**: Backend Team
**Estimated Timeline**: 2-3 weeks
