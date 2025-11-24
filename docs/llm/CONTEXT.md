# ViewRA - Quick Context for LLMs

> **TL;DR**: Self-hosted media server | Go backend (DDD) | React frontend | Dual DB (SQLite/PostgreSQL)

## 30-Second Overview

**What**: Plex/Jellyfin alternative for scanning, organizing, and streaming personal media collections
**Stack**: Go 1.25+ backend, TypeScript/React frontend, SQLite or PostgreSQL
**Architecture**: Domain-Driven Design with Clean Architecture principles
**Current Status**: Phase 1 complete (scanning, metadata, basic streaming)

## Project Structure (At a Glance)

```
viewra2/
├── cmd/server/          # Application entry point
├── internal/
│   ├── domain/          # Pure business logic (NO external deps)
│   ├── application/     # Use cases orchestration
│   ├── infrastructure/  # DB, FFmpeg, filesystem implementations
│   └── interfaces/      # HTTP handlers, CLI
├── web/                 # React + TypeScript frontend
├── migrations/          # SQL migrations (SQLite + PostgreSQL)
├── docs/                # Comprehensive documentation
└── .agent.md            # Detailed AI assistant guidelines
```

## Key Concepts

### 1. Architecture Layers

```
Interfaces (HTTP)
    ↓ calls
Application (Use Cases)
    ↓ orchestrates
Domain (Business Logic)     ← Pure Go, no external imports
    ↑ implements
Infrastructure (DB, FFmpeg)
```

**Golden Rule**: Domain NEVER imports external packages (Gin, database/sql, etc.)

### 2. Repository Pattern

```go
// Domain defines the interface
type MediaRepository interface {
    Create(ctx context.Context, media *Media) error
}

// Infrastructure implements it
type mediaRepo struct { queries *db.Queries }

// Main.go wires dependencies
repo := persistence.NewMediaRepository(db)
service := media.NewService(repo)
```

### 3. Dual Database Support

Every SQL query has TWO versions:
- `queries/media.sql` - SQLite with `?` placeholders
- `queries/media_postgres.sql` - PostgreSQL with `$1, $2` placeholders

**Critical**: ALL features must work on both databases.

### 4. Media Types

- **Movies**: Single files with metadata (title, year, genre, cast)
- **TV Shows**: Hierarchical (Show → Season → Episode)
- **Music**: Artist → Album → Track

## Common File Patterns

| Task | Location | Example |
|------|----------|---------|
| Add entity field | `internal/domain/<entity>/entity.go` | `type Media struct { Title string }` |
| Define repo method | `internal/domain/<entity>/repository.go` | `Create(ctx, *Media) error` |
| Implement repo | `internal/infrastructure/persistence/<entity>/repository.go` | Maps domain to SQL |
| Write SQL query | `internal/infrastructure/persistence/<entity>/queries/*.sql` | sqlc queries |
| Create use case | `internal/application/<entity>/<verb>_<noun>.go` | `create_media.go` |
| Add HTTP endpoint | `internal/interfaces/http/<entity>/handler.go` | Gin handlers |
| Shared utility | `internal/pkg/<category>/` | `fileutil`, `stringutil` |

## TypeScript/React Conventions

### Function Style (Strict)
```typescript
// ✅ ALWAYS
const Component = () => { }
const handler = () => { }

// ❌ NEVER
function Component() { }
function handler() { }
```

### Export Style (Strict)
```typescript
// ✅ Exports at end of file
const Component = () => { }
export { Component }

// ❌ Inline exports
export const Component = () => { }
```

### Type Files (Mandatory)
```typescript
// Component.types.ts
interface ComponentProps { title: string }
export type { ComponentProps }

// Component.tsx
import type { ComponentProps } from './Component.types'
```

## Development Workflow

### Creating a Feature (Vertical Slice)

1. **Domain**: Add entity field in `internal/domain/<entity>/entity.go`
2. **Migration**: Create SQL migration (SQLite + PostgreSQL)
3. **Queries**: Write sqlc queries for both databases
4. **Repository**: Implement in `internal/infrastructure/persistence/`
5. **Use Case**: Create in `internal/application/<entity>/`
6. **Handler**: Add HTTP endpoint in `internal/interfaces/http/`
7. **Test**: Write integration test
8. **Audit**: Run `make audit` to catch incomplete implementations

**Never**: Do horizontal layers (all entities, then all repos). Always complete vertically.

### Quick Commands

```bash
make dev              # Start backend + frontend
make test             # Run all tests
make audit            # Find incomplete implementations
sqlc generate         # Generate type-safe SQL code
make migrate-up       # Run database migrations
```

## Critical Rules

### ✅ DO
- Complete features vertically (all layers at once)
- Map all entity fields in repository (no placeholders)
- Test on both SQLite AND PostgreSQL
- Use arrow functions in TypeScript
- Put exports at end of files
- Create *.types.ts for component types

### ❌ DON'T
- Import external packages in domain layer
- Create adapter/wrapper files
- Use empty sql.NullString{} placeholders
- Leave TODOs in repository layer
- Use TypeScript function keyword
- Inline exports in TypeScript

## Anti-Patterns (Red Flags)

```go
// ❌ Empty null assignments (incomplete!)
VideoCodec: sql.NullString{}, // Should map actual field

// ❌ No-op methods (incomplete!)
func (r *repo) Create(...) error { return nil }

// ❌ Domain imports external packages
import "github.com/gin-gonic/gin" // WRONG in domain

// ❌ Adapter wrappers
type UseCases struct { create *CreateUseCase } // Just call directly
```

```typescript
// ❌ Function keyword
function Component() { } // Use arrow functions

// ❌ Inline exports
export const Component = () => { } // Exports at end
```

## Success Checklist

Before marking ANY feature complete:
- [ ] Works on SQLite AND PostgreSQL
- [ ] All entity fields mapped in repository
- [ ] Integration test passing
- [ ] `make audit` returns 0 issues
- [ ] Can retrieve saved data successfully
- [ ] TypeScript uses arrow functions
- [ ] Exports at end of file

## Common Questions

**Q: Where do I put utility functions?**
A: Shared utilities → `internal/pkg/<category>/`. Domain-specific → Stay in domain package.

**Q: Can domain import database/sql?**
A: NO. Domain only imports standard library.

**Q: How do I handle both databases?**
A: Write separate SQL files with build tags. Run `sqlc generate` for both.

**Q: Should I create adapter files?**
A: NO. Handlers call use cases directly. No wrapper layers.

**Q: When is a feature "done"?**
A: When it passes the 8-point checklist (see Development Workflow section).

## Key Documentation Files

Read these for detailed guidance:
- `.agent.md` - Comprehensive rules and examples
- `docs/ARCHITECTURE.md` - DDD layer breakdown
- `docs/DATABASE_SCHEMA.md` - Complete schema
- `docs/DEVELOPMENT_WORKFLOW.md` - Step-by-step workflows
- `docs/llm/PATTERNS.md` - Common code patterns

## Domain Entities Overview

| Entity | Key Fields | Relations |
|--------|-----------|-----------|
| Library | Name, Path, Type | HasMany Media |
| Media | Title, FilePath, Type, Duration | BelongsTo Library |
| Movie | Genre, Year, Rating | ExtendsMedia |
| TVShow | SeriesTitle, SeasonNum, EpisodeNum | ExtendsMedia |
| WatchProgress | MediaID, UserID, Position, Completed | BelongsTo Media |

## API Endpoints (Key)

```
GET    /api/libraries          # List all libraries
POST   /api/libraries          # Create library
POST   /api/libraries/:id/scan # Trigger scan
GET    /api/media              # List media (paginated)
GET    /api/media/:id          # Get media details
GET    /api/media/:id/stream   # Stream video
POST   /api/watch-progress     # Update watch position
```

## Tech Stack Quick Reference

**Backend**:
- Go 1.25+ (no generics in critical paths)
- Gin (HTTP framework)
- sqlc (type-safe SQL generation)
- golang-migrate (migrations)
- FFmpeg (metadata extraction, transcoding)

**Frontend**:
- TypeScript 5.3+
- React 19
- TanStack Router v1.80+ (file-based routing)
- TanStack Query v5 (data fetching)
- Tailwind CSS + Shadcn UI

**Database**:
- SQLite (default, zero-config)
- PostgreSQL (production scale)

## Testing Strategy

- **Unit Tests**: Domain business logic
- **Integration Tests**: Repository implementations with real DB
- **E2E Tests**: API endpoints with full stack
- **Coverage Target**: 70%+ overall

Use `testutil.SetupTestDB(t)` for integration tests with real database.

## Performance Notes

- Media scanning: ~1000 files/sec
- Transcoding: On-demand DASH (H.264)
- Caching: In-memory for metadata, Redis future
- Database: Indexes on foreign keys, file paths

## Security Considerations

- No authentication yet (single-user Phase 1)
- Multi-user auth in Phase 5
- File path validation to prevent directory traversal
- SQL injection prevention via sqlc prepared statements

## Known Limitations (Current Phase)

- Single-user only
- Basic filename parsing (no complex patterns)
- CPU transcoding only (no GPU)
- No mobile apps (web-responsive only)

## Next Development Phases

1. ✅ **Phase 1**: Core foundation (libraries, scanning, basic streaming)
2. 🚧 **Phase 2**: Watch progress & transcoding
3. 📋 **Phase 3**: Metadata enrichment (TMDb, TheTVDB)
4. 📋 **Phase 4**: Advanced features (collections, search)
5. 📋 **Phase 5**: Multi-user support

## Quick Debugging

```bash
# Check database
sqlite3 viewra.db "SELECT COUNT(*) FROM media;"

# View logs
tail -f logs/viewra.log

# Test endpoint
curl http://localhost:8080/api/libraries

# Check running processes
ps aux | grep viewra
```

## Environment Variables

```bash
# Development
PORT=8080
DATABASE_URL=sqlite://viewra.db
ENV=development
LOG_LEVEL=debug

# Production
ENV=production
LOG_LEVEL=info
DB_DRIVER=postgres
DATABASE_URL=postgres://user:pass@localhost/viewra
```

## Remember

- **Complete features vertically** (all layers, one feature)
- **No external deps in domain** (pure business logic)
- **Support both databases** (SQLite + PostgreSQL)
- **Arrow functions in TypeScript** (const, not function)
- **Exports at end of files** (not inline)
- **Zero tolerance for incomplete implementations** (no placeholders)

This codebase values **clean architecture**, **type safety**, and **complete implementations** over quick shortcuts.

---

**Last Updated**: 2025-11-24
**For More Details**: See `.agent.md` and `docs/` directory
