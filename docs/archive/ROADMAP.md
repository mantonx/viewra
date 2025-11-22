# ViewRA Development Roadmap

**Historical Record of Implementation Progress**

This document provides a high-level overview of completed development phases through mid-November 2025. For current status, see [PROJECT_STATUS.md](PROJECT_STATUS.md). For detailed implementation notes, see git commit history and the codebase itself.

---

## Project Timeline (Historical)

**Project Start**: November 11, 2025
**Phase 0 Complete**: November 11, 2025 (1 day)
**Phase 1 Complete**: November 12, 2025 (1 day)
**Phase 2 Complete**: November 13, 2025 (2 days)
**Phase 3-5**: November 15-21, 2025 (see PROJECT_STATUS.md for details)

**Note**: This roadmap documents the early project history. Phases 4 and 5 were completed ahead of schedule. For the current state and future plans, refer to [PROJECT_STATUS.md](PROJECT_STATUS.md) and [PROJECT_PLAN.md](PROJECT_PLAN.md).

---

## Phase 0: Project Setup ✅ Complete (Nov 11, 2025)

**Duration**: 1 day
**Goal**: Establish development environment and project structure

### What Was Built

#### Documentation
- `ARCHITECTURE.md` - Domain-driven design patterns, layer separation, code organization
- `API_SPECIFICATION.md` - REST endpoint documentation with request/response examples
- `DATABASE_SCHEMA.md` - Complete schema definitions for SQLite and PostgreSQL
- `TECH_STACK.md` - Technology decisions and tooling choices
- `PLUGIN_ARCHITECTURE.md` - Extensible plugin system design
- `CONVENTIONS.md` - File naming, coding standards, anti-patterns
- `TESTING.md` - Testing strategy and coverage tracking
- `QUICK_REFERENCE.md` - 1-page development workflow cheat sheet
- `README.md` - Project overview optimized for AI assistants
- `.agent.md` - AI assistant configuration and guidelines

#### Development Tools
- Air (live reload for Go backend)
- sqlc (type-safe SQL code generation)
- Swagger/swag (OpenAPI 3.0 documentation)
- Orval (TypeScript API client generation)
- ESLint + Biome (frontend linting and formatting)
- golangci-lint (Go static analysis)
- Makefile (common development commands)
- Procfile (coordinated dev workflow)

#### Repository Structure
- Git repository initialized
- `.gitignore` configured
- Go module initialized (`go mod init`)
- Frontend initialized (Vite + React)
- Directory structure created (`internal/`, `cmd/`, `web/`, `migrations/`, `docs/`)

### Key Decisions Made
- **Architecture**: Clean Architecture with DDD principles
- **Backend**: Go 1.21+ with Gin web framework
- **Frontend**: React 19 with TypeScript
- **Database**: Dual support (SQLite for dev, PostgreSQL for production)
- **Code Generation**: sqlc for SQL, Orval for API clients
- **Testing**: In-memory SQLite for fast tests
- **Error Handling**: Sentinel errors with wrapped context

---

## Phase 1: Core Foundation ✅ Complete (Nov 12, 2025)

**Duration**: 1 day (rapid development with AI assistance)
**Goal**: Build complete full-stack media server foundation
**Lines of Code**: ~15,000+ (Backend: ~8,000 | Frontend: ~7,000)
**Test Coverage**: 44.1% overall

### Domain Layer (`internal/domain/`)

#### Libraries
- `entity.go` - Library domain entity (ID, name, path, type, settings)
- `repository.go` - Repository interface (Create, GetByID, List, Update, Delete)
- `types.go` - Enums (MediaType: movie, tv_episode, music_track)
- `errors.go` - Domain errors (ErrLibraryNotFound, ErrInvalidPath, ErrDuplicatePath)

#### Media
- `entity.go` - Media base entity with 40+ fields (title, path, duration, resolution, codecs, etc.)
- `repository.go` - Media repository interface
- `types.go` - Media types, resolution labels, aspect ratios
- `metadata.go` - Metadata calculation functions (aspect ratio, resolution label, source type detection)
- `service.go` - Media business logic

#### Scanner
- `types.go` - ScanResult, FileInfo structures
- `coordinator.go` - Scan coordination logic
- File parsing (movies, TV episodes, extras)
- Hash calculation (SHA-256)
- Incremental scanning support

#### Progress
- `entity.go` - Watch progress entity (user_id, media_id, position, duration)
- `repository.go` - Progress repository interface
- `service.go` - Progress tracking logic with 90% auto-watched

### Application Layer (`internal/application/`)

#### Library Use Cases
- `create_library.go` - Create library with validation
- `list_libraries.go` - List all libraries
- `get_library.go` - Get library by ID
- `update_library.go` - Update library settings
- `delete_library.go` - Delete library
- `scan_library.go` - Scan library for media files (incremental)
- `dto.go` - Data transfer objects
- `interfaces.go` - Repository interfaces

#### Media Use Cases
- `get_media.go` - Get media by ID with all metadata
- `list_media.go` - List media with filters (library, type, search)
- `dto.go` - Media response DTOs
- `interfaces.go` - Repository interfaces

#### Progress Use Cases
- `update_progress.go` - Update watch progress
- `get_progress.go` - Get progress for media
- `mark_watched.go` - Mark as watched/unwatched
- `list_progress.go` - List all progress records
- `dto.go` - Progress DTOs

### Infrastructure Layer (`internal/infrastructure/`)

#### Database
- **Dual Database Support**: SQLite (default) + PostgreSQL
- `connection.go` - Database connection with retry logic
- `migrate.go` - Auto-migration system with backups
- `queries/postgres/*.sql` - PostgreSQL-specific queries
- `queries/sqlite/*.sql` - SQLite-specific queries
- `sqlc_postgres/` - Generated PostgreSQL code (28 files)
- `sqlc_sqlite/` - Generated SQLite code (28 files)

**Migrations**:
- `000001_init.up.sql` - Initial schema (libraries, media, movies, tv_shows, tv_episodes, music_tracks)
- `000002_add_scan_jobs.up.sql` - Scan job tracking
- `000003_add_is_extra.up.sql` - Extras support (deleted scenes, trailers, behind-the-scenes)
- `000004_add_audio_codec.up.sql` - Audio codec column

#### Repositories
- `persistence/library/repository.go` - Library repository (dual DB)
- `persistence/media/repository.go` - Media repository (dual DB)
- `persistence/progress/repository.go` - Progress repository (dual DB)
- `persistence/scanjob/repository.go` - Scan job repository (dual DB)
- `persistence/common/helpers.go` - Null* type conversions

#### FFmpeg Integration
- `ffmpeg/client.go` - FFmpeg wrapper with validation
- `ffmpeg/metadata.go` - Metadata extraction (codec, resolution, bitrate, framerate, duration)
- Extracts video codec, audio codec, resolution, bitrate, framerate
- Calculates aspect ratio and resolution labels (1080p, 4K, etc.)

#### Filesystem
- `filesystem/scanner.go` - Directory scanner with worker pools
- `filesystem/coordinator.go` - Scan coordinator with incremental support
- File hash calculation (SHA-256)
- Movie filename parsing (title, year)
- TV episode parsing (S01E01, 1x01 formats)
- Extras detection (deleted scenes, behind the scenes, trailers, featurettes, interviews, scenes)

#### Path Browser
- `pathbrowser/browser.go` - Secure filesystem browser
- `pathbrowser/validator.go` - 9-layer path validation (absolute path, no traversal, no symlinks, within allowed roots)
- Directory listing with file/folder distinction
- Size and modification time metadata

#### Streaming
- `streaming/server.go` - HTTP range request handler
- `streaming/range.go` - HTTP range parsing (supports multi-range)
- Direct file streaming with seek support

#### Adapters
- `adapters/query_router.go` - Dual database query routing
- Routes queries to PostgreSQL or SQLite based on connection

### API Layer (`internal/api/`)

#### Handlers
- `handlers/library.go` - Library CRUD endpoints (6 methods)
- `handlers/media.go` - Media endpoints (List, Get) (2 methods)
- `handlers/progress.go` - Progress endpoints (8 methods)
- `handlers/browser.go` - Filesystem browser endpoint
- `handlers/scanjob.go` - Scan status and history
- `handlers/helpers.go` - Error mapping, pagination, response helpers

#### Routes
- `routes/library.go` - Library routes (6 endpoints)
- `routes/media.go` - Media routes (2 endpoints + streaming)
- `routes/progress.go` - Progress routes (8 endpoints)
- `routes/browser.go` - Browser route (1 endpoint)

#### Middleware
- `middleware/cors.go` - CORS handling for frontend

#### Swagger Documentation
- All endpoints documented with OpenAPI 3.0
- Request/response schemas defined
- `GET /swagger/index.html` - Interactive API docs

### Frontend (`web/src/`)

#### Components
- `components/library/LibraryCard/` - Library card with icons and stats
- `components/library/LibraryForm/` - Create/edit library form with validation
- `components/library/FilesystemBrowser/` - Secure path browser with breadcrumbs
- `components/media/MediaCard/` - Media card with poster and metadata
- `components/media/MediaDetailsModal/` - Media detail modal
- `components/ui/Button/` - Reusable button component
- `components/ui/Skeleton/` - Loading skeleton component
- `components/common/` - Common utilities

#### Hooks
- `lib/hooks/useLibraries.ts` - Library data fetching
- `lib/hooks/useMedia.ts` - Media data fetching
- `lib/hooks/useProgress.ts` - Progress tracking hooks
- `lib/hooks/useFilesystemBrowser.ts` - Filesystem browser state
- `lib/hooks/useBrowserPreferences.ts` - Browser preferences persistence
- `lib/hooks/useInvalidateLibraries.ts` - Cache invalidation

#### API Client
- `lib/api/generated/` - Orval-generated TypeScript client (auto-sync with OpenAPI spec)
- `lib/api/mutator/` - Custom API instance configuration
- Type-safe API calls with TanStack Query

#### Routes
- `routes/__root.tsx` - Root layout
- `routes/_layout.tsx` - App layout with navigation
- `routes/_layout/libraries.tsx` - Libraries page (`/libraries`)
- `routes/_layout/media.tsx` - Media page (`/media`)
- `routes/index.tsx` - Home page (`/`)

#### State Management
- TanStack Query for server state (caching, invalidation)
- React Context for UI state
- Local storage for preferences

#### Accessibility
- ARIA labels on all interactive elements
- Keyboard navigation support
- Focus management
- Screen reader optimized

### Testing (`*_test.go`)

#### Domain Tests (88.9% coverage)
- `internal/domain/media/entity_test.go` - Media entity tests
- `internal/domain/library/entity_test.go` - Library entity tests
- `internal/domain/progress/entity_test.go` - Progress entity tests
- `internal/domain/scanner/coordinator_test.go` - Scanner tests

#### Application Tests (62.5% coverage)
- `internal/application/library/scan_library_test.go` - Scan use case tests
- `internal/application/media/get_media_test.go` - Get media tests
- `internal/application/media/list_media_test.go` - List media tests
- `internal/application/progress/update_progress_test.go` - Update progress tests
- `internal/application/progress/get_progress_test.go` - Get progress tests
- `internal/application/progress/mark_watched_test.go` - Mark watched tests

#### Infrastructure Tests (72.5% coverage)
- `internal/infrastructure/persistence/media/repository_test.go` - Media repository integration tests
- `internal/infrastructure/persistence/library/repository_test.go` - Library repository integration tests
- `internal/infrastructure/database/connection_test.go` - Database connection tests
- `internal/infrastructure/filesystem/coordinator_test.go` - Filesystem scanner tests

#### API Tests (13.2% coverage)
- `internal/api/handlers/media_test.go` - Media handler tests
- Limited coverage (needs improvement in Phase 2)

### Key Implementation Highlights

#### Dual Database Support
- Query router pattern routes queries to PostgreSQL or SQLite
- Separate query files for each database
- SQLC generates type-safe code for both
- Seamless switching via environment variable

#### Incremental Scanning
- Tracks last scan time per library
- Only scans modified files (compares modification time)
- Skips unchanged files for fast re-scans
- Supports manual full re-scan

#### Extras Support
- Detects deleted scenes, behind-the-scenes, trailers, featurettes, interviews
- `is_extra` flag in database
- Can filter out extras from main media views

#### Security
- 9-layer path validation prevents directory traversal
- No symlink following
- Absolute paths only
- CORS validation
- SQL injection prevention (prepared statements)

#### Type Safety
- SQLC generates type-safe SQL code
- Orval generates type-safe API clients
- TypeScript strict mode
- Comprehensive DTOs prevent stringly-typed code

---

## Implementation Statistics

### Lines of Code
| Component | Lines | Percentage |
|-----------|-------|------------|
| Backend (Go) | ~8,000 | 53% |
| Frontend (TypeScript/React) | ~7,000 | 47% |
| **Total** | **~15,000** | **100%** |

### Test Coverage
| Layer | Coverage | Status |
|-------|----------|--------|
| Domain | 88.9% | ✅ Excellent |
| Application | 62.5% | ⚠️ Good |
| Infrastructure | 72.5% | ✅ Good |
| API | 13.2% | ❌ Needs Improvement |
| **Overall** | **44.1%** | **⚠️ Moderate** |

### Database Schema
| Table | Columns | Purpose |
|-------|---------|---------|
| `libraries` | 8 | Media library definitions |
| `media` | 40+ | Base media metadata |
| `movies` | 8 | Movie-specific metadata |
| `tv_shows` | 7 | TV show metadata |
| `tv_seasons` | 5 | TV season metadata |
| `tv_episodes` | 10 | TV episode metadata |
| `music_tracks` | 12 | Music track metadata |
| `watch_progress` | 8 | Watch progress tracking |
| `scan_jobs` | 9 | Scan job tracking |

### API Endpoints
| Category | Endpoints | Methods |
|----------|-----------|---------|
| Libraries | 6 | GET, POST, PUT, DELETE, POST (scan) |
| Media | 3 | GET (list), GET (by ID), GET (stream) |
| Progress | 8 | GET, PUT, POST (mark watched) |
| Filesystem | 1 | GET (browse) |
| Scan Jobs | 3 | GET (status), GET (history), GET (stream SSE) |
| Health | 1 | GET (health check) |
| **Total** | **22** | **Various** |

---

## Technical Decisions Archive

### Development Environment
**Decision**: Setup script + Makefile + Procfile
**Rationale**: Balance between automation and flexibility
- `scripts/setup.sh` validates Go, Node, FFmpeg
- `Makefile` provides daily commands (`make dev`, `make test`, `make sqlc`)
- `Procfile` runs backend + frontend together with overmind/foreman

### Database Strategy
**Decision**: Dual database support (SQLite + PostgreSQL)
**Rationale**: SQLite for simplicity, PostgreSQL for scale
- Default: SQLite (zero-config, embedded)
- Production: PostgreSQL (horizontal scaling ready)
- Query router pattern for seamless switching
- SQLC generates code for both

### Code Generation
**Decision**: sqlc for SQL, Orval for API clients
**Rationale**: Type safety reduces runtime errors
- sqlc: Type-safe SQL queries from .sql files
- Orval: TypeScript client from OpenAPI spec
- Auto-sync between backend and frontend
- Catch mismatches at compile time

### Error Handling
**Decision**: Sentinel errors + wrapped context
**Rationale**: Type safety + debugging information
- Sentinel errors for type checking (`errors.Is()`)
- Wrapped context for debugging (`fmt.Errorf("%w")`)
- Domain errors defined in domain layer
- HTTP error mapping in API layer

### Testing Strategy
**Decision**: In-memory SQLite + testcontainers
**Rationale**: Fast tests with real database behavior
- Unit tests: Pure business logic (no DB)
- Integration tests: Real database (in-memory SQLite)
- E2E tests: Full stack (future)

### Validation Strategy
**Decision**: Multi-layer validation (API → Domain → Infrastructure)
**Rationale**: Defense in depth
- API layer: Request validation (missing fields, types)
- Domain layer: Business rules (path must be absolute, library must exist)
- Infrastructure layer: Technical validation (database constraints)

### Concurrency
**Decision**: Worker pools with context cancellation
**Rationale**: Efficient resource usage
- Scanner: Worker pool for parallel file processing
- Context cancellation for graceful shutdown
- Configurable worker count (default: `runtime.NumCPU()`)

### Frontend State
**Decision**: TanStack Query + React Context
**Rationale**: Separation of server state and UI state
- TanStack Query: Server state (caching, revalidation, optimistic updates)
- React Context: UI state (selected items, modals, preferences)
- Local storage: User preferences (theme, view mode)

### API Documentation
**Decision**: OpenAPI 3.0 with Swagger UI
**Rationale**: Interactive docs + client generation
- Swagger annotations in Go handlers
- `swag init` generates OpenAPI spec
- Interactive docs at `/swagger/index.html`
- Orval uses spec to generate TypeScript client

---

## Challenges & Solutions

### Challenge 1: Dual Database Support
**Problem**: Supporting both SQLite and PostgreSQL with different SQL syntax
**Solution**:
- Separate query files (`queries/sqlite/`, `queries/postgres/`)
- SQLC generates database-specific code
- Query router pattern switches at runtime
- Type adapters handle NULL vs zero-value differences

### Challenge 2: Type-Specific Metadata
**Problem**: Movies, TV, and music have different metadata fields
**Solution**:
- Hybrid schema: Base `media` table + type-specific tables
- Foreign key: `movies.media_id` → `media.id`
- Repository implementations initially stubbed as no-ops
- To be completed in Phase 3

### Challenge 3: Incremental Scanning
**Problem**: Full library scans too slow for large collections
**Solution**:
- Track last scan time per library
- Compare file modification time to last scan
- Skip unchanged files
- Configurable: force full re-scan option

### Challenge 4: Frontend Path Security
**Problem**: User-provided paths could access sensitive files
**Solution**:
- 9-layer validation in path browser
- Absolute paths only (no relative paths)
- No directory traversal (`..` not allowed)
- No symlink following
- Whitelist of allowed root directories

### Challenge 5: FFmpeg Metadata Extraction
**Problem**: Not all video files have consistent metadata
**Solution**:
- FFmpeg probe with JSON output
- Graceful fallback for missing fields
- Calculate derived fields (aspect ratio, resolution label)
- Store NULL for missing optional fields

---

## Known Issues & Technical Debt

### Phase 1 Incomplete Items
These items were intentionally deferred to later phases:

1. **Type-Specific Repositories (No-ops)**: MovieRepository, TVRepository, MusicRepository currently stub implementations
   - **Impact**: Movie year, TV season/episode numbers, music track numbers not stored
   - **Resolution**: Phase 3

2. **Frontend Media Display**: API returns media but UI doesn't display it yet
   - **Impact**: Can't browse scanned media in UI
   - **Resolution**: Phase 2 (after watch progress)

3. **Streaming Testing**: Streaming infrastructure exists but minimally tested
   - **Impact**: May have edge cases with seeking/range requests
   - **Resolution**: Phase 2 (during player integration)

4. **API Handler Test Coverage**: Only 13.2% coverage
   - **Impact**: HTTP-level bugs may slip through
   - **Resolution**: Continuous improvement

5. **Thumbnail Generation**: FFmpeg can generate thumbnails but not integrated
   - **Impact**: No thumbnail previews
   - **Resolution**: Phase 4 or 5

### Architecture Decisions to Revisit
- **No Authentication**: Single-user assumption (add in Phase 5)
- **No Caching**: Redis not integrated (add if needed for scale)
- **No Message Queue**: Transcoding will be in-process (Phase 2, may need queue in Phase 6)

---

## Lessons Learned

### What Went Well
1. **Clean Architecture**: Domain-driven design made testing easy and code maintainable
2. **Code Generation**: sqlc and Orval eliminated entire classes of bugs
3. **Dual Database**: Query router pattern worked smoothly, no runtime issues
4. **AI-Assisted Development**: Rapid prototyping with Claude Code
5. **Documentation-First**: Writing docs before code clarified requirements

### What Could Be Improved
1. **Test Coverage**: API layer needs more tests before Phase 2
2. **Error Messages**: Need more user-friendly error messages in UI
3. **Performance**: Haven't benchmarked scanner on very large libraries (100k+ files)
4. **Logging**: Need structured logging throughout (currently inconsistent)
5. **Monitoring**: No health checks or metrics yet (Phase 8)

### Architecture Insights
- **Repository pattern**: Worked perfectly, made dual DB support seamless
- **No-op stubs**: Allowed progressive implementation, but risky (missed in initial testing)
- **Vertical slices**: Building complete features end-to-end (domain → API → UI) works better than horizontal layers
- **Type safety**: Caught numerous bugs at compile time (worth the setup cost)

---

## Phase 2: Watch Progress & Transcoding ✅ Complete (Nov 13, 2025)

**Duration**: 2 days (Nov 12-13, 2025)
**Goal**: Enable video playback with progress tracking and on-demand transcoding

### What Was Built

#### Phase 2.1: Watch Progress Tracking
- Database: `watch_progress` table with per-user tracking
- Domain: `WatchProgress` entity with business logic
- Repository: Dual database support (SQLite + PostgreSQL)
- API: 4 endpoints (GET, PUT, POST, DELETE progress)
- Frontend: VideoPlayer component with Shaka Player, progress bars, resume buttons
- Continue Watching section on home page
- Auto-mark watched at 90% completion

#### Phase 2.2: On-Demand Transcoding
- Database: `transcode_jobs` table with access tracking (migration 000006)
- 4-tier streaming strategy:
  - Direct Play (instant for compatible files)
  - Remux (2-5 min, container conversion only)
  - Remux + Audio Downmix (5-10 min, stereo conversion)
  - Full Transcode (20-60 min, full re-encode)
- Worker pool with configurable concurrency
- HLS progressive streaming (playback starts immediately)
- Idle timeout to cancel abandoned transcodes
- Access tracking (last_accessed_at, access_count) for LRU cleanup

#### Phase 2.3: Transcode Cleanup System
- CLI tool: `cmd/transcode-cleanup` with dry-run mode
- API endpoints: disk usage stats + cleanup operations
- Automated background scheduler (runs every 6 hours)
- Policy-based cleanup (age, idle, failed jobs, orphans)
- Threshold-based LRU cleanup when disk > 85%
- 10+ configurable environment variables

### Key Files Created
- `internal/application/transcode/queue.go` - Worker pool
- `internal/application/transcode/cleanup.go` - Cleanup service
- `internal/application/transcode/cleanup_scheduler.go` - Background scheduler
- `internal/infrastructure/transcoding/service.go` - FFmpeg executors
- `internal/infrastructure/transcoding/validation.go` - Strategy selection
- `cmd/transcode-cleanup/main.go` - CLI tool
- `web/src/components/media/VideoPlayer/` - Shaka Player integration
- `migrations/000006_add_transcode_tracking.up.sql` - Access tracking

### Implementation Highlights
- **Smart Strategy Selection**: Detects audio channels and codecs to choose optimal approach
- **Progressive Streaming**: Like Plex/Jellyfin, playback starts immediately when transcoding begins
- **Worker Pool**: Channel-based concurrency with idle timeout
- **Disk Management**: Automated cleanup with LRU eviction and configurable policies
- **Documentation**: Consolidated 30 docs to 20, merged redundant content

### Success Metrics
- ✅ Resume playback from last position
- ✅ Auto-mark watched at 90%
- ✅ Remux completes in < 5 minutes (10x realtime speed)
- ✅ 4-tier intelligent strategy selection
- ✅ Progressive streaming starts immediately
- ✅ Automated cleanup prevents disk exhaustion
- ✅ LRU cleanup preserves frequently-used transcodes

---

## Next: Phase 3 (TV Shows & Music Support)

See [PROJECT_PLAN.md](./PROJECT_PLAN.md) for current project status and upcoming phases.

**Detailed Implementation**: Refer to git commit history for file-by-file changes and implementation notes.

**Last Updated**: 2025-11-13
