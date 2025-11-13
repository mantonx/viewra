# ViewRA - Modern Self-Hosted Media Server

> A Plex/Jellyfin alternative built with Go + React, focusing on performance and clean architecture.

**Status**: 🚧 Active Development - Phase 1 Complete, Phase 2 Next
**Developer**: Solo project evolving to open source
**Started**: November 2025

---

## Quick Context for AI Assistants

This is a **media server application** that:
- Scans local media files (movies, TV shows, music)
- Extracts metadata using FFmpeg
- Streams media with adaptive transcoding (DASH)
- Tracks watch progress across devices
- Provides a web UI similar to Plex

**Key Technical Decisions:**
- **Backend**: Go 1.21+ with Gin framework, **SQLite (default) or PostgreSQL**, Domain-Driven Design (DDD)
- **Frontend**: React 19 + TypeScript + TanStack Router/Query + Shadcn UI
- **Media**: FFmpeg for transcoding, Shaka Player for adaptive streaming
- **Architecture**: Clean/DDD with layers: Domain → Application → Infrastructure → Interfaces
- **Database**: Hybrid schema (base `media` table + type-specific tables for movies/TV/music)
- **Plugins**: Extensible plugin system with gRPC/HTTP communication
- **⚠️ IMPORTANT**: All features must work on BOTH SQLite and PostgreSQL

**Current Phase**: Phase 0 - Project Setup  
**Next Phase**: Phase 1 - Core Foundation (libraries, scanning, basic streaming)

---

## Project Structure

```
viewra2/
├── cmd/
│   └── server/           # Main application entry point
│       └── main.go
├── internal/
│   ├── domain/           # Business logic (framework-agnostic)
│   │   ├── library/
│   │   │   ├── entity.go       # Library entity
│   │   │   ├── repository.go   # Repository interface
│   │   │   ├── service.go      # Business logic
│   │   │   ├── types.go        # Enums, constants
│   │   │   └── errors.go       # Domain errors
│   │   ├── media/
│   │   │   ├── entity.go
│   │   │   ├── repository.go
│   │   │   ├── service.go
│   │   │   ├── types.go
│   │   │   └── filename.go     # Domain-specific utility
│   │   └── watch/
│   │       ├── entity.go
│   │       ├── repository.go
│   │       └── service.go
│   ├── application/      # Use cases and DTOs
│   │   ├── library/
│   │   │   ├── create_library.go   # One file per use case
│   │   │   ├── scan_library.go
│   │   │   ├── list_libraries.go
│   │   │   └── dto.go              # All DTOs for this domain
│   │   ├── media/
│   │   │   ├── get_media.go
│   │   │   ├── stream_media.go
│   │   │   └── dto.go
│   │   └── watch/
│   │       ├── update_progress.go
│   │       └── dto.go
│   ├── infrastructure/   # External dependencies
│   │   ├── database/
│   │   │   ├── connection.go
│   │   │   ├── migrate.go
│   │   │   ├── repository/
│   │   │   │   ├── library.go  # Repository implementations
│   │   │   │   ├── media.go
│   │   │   │   └── watch.go
│   │   │   └── sqlc/
│   │   │       ├── schema.sql
│   │   │       └── queries/
│   │   │           ├── library.sql
│   │   │           ├── media.sql
│   │   │           └── watch.sql
│   │   ├── ffmpeg/       # FFmpeg wrapper, transcoding
│   │   ├── filesystem/   # File scanner, watcher
│   │   └── queue/        # Background job queue
│   ├── api/              # HTTP REST API (Phase 1.8 ✅)
│   │   ├── server.go     # Gin server, lifecycle
│   │   ├── handlers/     # HTTP request handlers
│   │   │   ├── library.go  # Library endpoints
│   │   │   ├── media.go    # Media endpoints (read-only)
│   │   │   ├── stream.go   # Streaming with range support
│   │   │   └── errors.go   # Error mapping
│   │   └── routes/       # Route registration
│   │       ├── library.go  # Library routes
│   │       ├── media.go    # Media routes
│   │       └── stream.go   # Streaming route
│   ├── pkg/              # Shared utilities (project-specific)
│   │   ├── config/       # Configuration loading
│   │   ├── logger/       # Structured logging
│   │   ├── errors/       # Error handling utilities
│   │   ├── validator/    # Input validation
│   │   ├── fileutil/     # File operations (hash, size)
│   │   ├── stringutil/   # String manipulation
│   │   └── timeutil/     # Time/date utilities
│   └── testutil/         # Test helpers (not in pkg/)
│       ├── database.go   # Test DB setup
│       ├── fixtures.go   # Common test data
│       └── mock/         # Mock implementations
├── web/                  # React frontend
│   ├── src/
│   │   ├── components/   # Reusable UI components
│   │   ├── routes/       # TanStack Router pages
│   │   ├── lib/          # Utilities, API client
│   │   └── App.tsx
│   └── package.json
├── migrations/           # Database migrations (golang-migrate)
├── tests/                # Integration & E2E tests
│   ├── integration/
│   └── e2e/
├── docs/                 # Comprehensive documentation
│   ├── ARCHITECTURE.md   # DDD layers, code organization
│   ├── DATABASE_SCHEMA.md # Complete schema with all tables
│   ├── API_SPECIFICATION.md # REST API endpoints
│   ├── PLUGIN_ARCHITECTURE.md # Plugin system design
│   ├── TECH_STACK.md     # Technology choices
│   ├── PROJECT_PLAN.md   # 8-phase implementation roadmap
│   ├── RECOMMENDATIONS.md # Agent recommendations & tracking
│   ├── QUICK_REFERENCE.md # Development workflow cheat sheet
│   ├── TESTING.md        # Testing strategy & coverage
│   └── CONVENTIONS.md    # Code style & file naming
├── configs/              # Configuration files
├── .agent.md             # AI assistant guidelines
├── NOTES.md              # Development journal
└── data/                 # Runtime data (gitignored)
    ├── viewra2.db        # SQLite database
    ├── thumbnails/       # Generated thumbnails
    ├── dash/             # Transcoded DASH files
    └── cache/            # Temporary files
```

---

## Quick Start (When Implemented)

```bash
# Clone and setup
git clone <repo>
cd viewra2

# Backend
go mod download
migrate -path migrations -database "sqlite3://data/viewra2.db" up
air  # Hot reload

# Frontend
cd web
npm install
npm run dev

# Access
Backend:  http://localhost:3000
Frontend: http://localhost:5173
Swagger:  http://localhost:3000/swagger/index.html
```

---

## Core Concepts

### Media Types
1. **Movies** - Single video files with metadata (title, year, genre, cast)
2. **TV Shows** - Hierarchical: Show → Season → Episode
3. **Music** - Organized: Artist → Album → Track

### Streaming Strategy
- **Direct Stream**: If client supports codec (H.264/AAC)
- **DASH Transcode**: On-demand adaptive streaming
  - 360p: Generated immediately (fast, low quality)
  - 720p/1080p: Background queue (slower, high quality)

### Database Design
- **Hybrid Schema**: Base `media` table + type-specific tables
- **Rich Metadata**: People, credits, collections, genres (normalized)
- **Multi-User Ready**: User-specific watch progress, ratings, playlists
- **Plugin Support**: Plugin registry, data storage, event hooks

### Plugin System
- **7 Plugin Types**: Metadata providers, auth, notifiers, transcoders, scanners, storage backends, analytics
- **Communication**: gRPC or HTTP
- **Security**: Permission-based, sandboxed execution
- **Official Plugins**: TMDb, TheTVDB, MusicBrainz, Discord notifier

---

## Key Files for Understanding

When working with this codebase, refer to:

1. **`docs/ARCHITECTURE.md`** - Understand layers (Domain/Application/Infrastructure/Interfaces)
2. **`docs/DATABASE_SCHEMA.md`** - All tables, relationships, queries (comprehensive!)
3. **`docs/API_SPECIFICATION.md`** - REST endpoints, request/response formats
4. **`docs/PROJECT_PLAN.md`** - 8-phase roadmap with task breakdowns
5. **`docs/RECOMMENDATIONS.md`** - Agent recommendations & implementation tracking
6. **`docs/QUICK_REFERENCE.md`** - Development workflow cheat sheet
7. **`docs/CONVENTIONS.md`** - Code style, file naming, best practices

---

## Development Workflow

### Code Generation
```bash
# Generate type-safe SQL code (sqlc)
sqlc generate

# Generate Swagger docs
swag init -g cmd/server/main.go

# Generate frontend API client (Orval from Swagger)
cd web && npm run generate:api
```

### Database Migrations
```bash
# Create new migration
migrate create -ext sql -dir migrations -seq <name>

# Run migrations
migrate -path migrations -database "sqlite3://data/viewra2.db" up

# Rollback
migrate -path migrations -database "sqlite3://data/viewra2.db" down 1
```

### Testing
```bash
# Backend tests
go test ./...

# Frontend tests
cd web && npm test

# E2E tests (when implemented)
npm run test:e2e
```

---

## Architecture Patterns

### Domain-Driven Design (DDD)
```
┌─────────────────────────────────────────┐
│      Interfaces (HTTP/CLI)              │  ← Handlers, routes, middleware
│         ↓ depends on ↓                  │
│      Application Layer                   │  ← Use cases, DTOs
│         ↓ depends on ↓                  │
│         Domain Layer                     │  ← Entities, business logic
│         ↑ implements ↑                  │
│     Infrastructure Layer                 │  ← Database, FFmpeg, filesystem
└─────────────────────────────────────────┘
```

**Key Rule**: Domain layer has ZERO external dependencies.

### Repository Pattern
```go
// Domain defines interface
type MediaRepository interface {
    Create(ctx context.Context, media *Media) error
    GetByID(ctx context.Context, id int64) (*Media, error)
}

// Infrastructure implements
type mediaRepository struct {
    queries *db.Queries  // sqlc generated
}
```

### Dependency Injection
```go
// Wire dependencies in main.go
db := database.Connect()
mediaRepo := repository.NewMediaRepository(db)
mediaSvc := domain.NewMediaService(mediaRepo)
getMediaUseCase := application.NewGetMedia(mediaSvc)
mediaHandler := handlers.NewMediaHandler(getMediaUseCase)
```

---

## Important Implementation Notes

### File Naming Conventions (for Scanner)
```
Movies:
  ✅ The Matrix (1999).mp4
  ✅ The.Matrix.1999.1080p.BluRay.x264.mp4
  ✅ /movies/The Matrix (1999)/The Matrix (1999).mkv

TV Shows:
  ✅ Breaking Bad/Season 01/Breaking Bad - S01E01.mkv
  ✅ /tv/Breaking.Bad/S01/Breaking.Bad.S01E01.Pilot.mkv
  ✅ /tv/Anime/Attack.on.Titan/Attack.on.Titan.-.01.mkv (absolute numbering)

Music:
  ✅ Pink Floyd/The Dark Side of the Moon/01 - Speak to Me.flac
  ✅ /music/Artist Name/Album Name/Track Number - Track Name.mp3
```

### Environment Variables
```bash
# Server
PORT=3000
DATABASE_URL=sqlite://data/viewra2.db
DATA_DIR=/data

# Development
ENV=development
LOG_LEVEL=debug
FRONTEND_URL=http://localhost:5173
CORS_ORIGINS=http://localhost:5173

# Production (Phase 8)
ENV=production
LOG_LEVEL=info
```

---

## Current Status & Next Steps

### ✅ Completed (Phase 0 & 1)
- [x] Architecture design (DDD with clean layers)
- [x] Database schema (comprehensive, all tables defined)
- [x] API specification (all endpoints documented)
- [x] Tech stack decisions & tooling setup
- [x] Domain entities (Library, Media, Scanner)
- [x] Dual database support (SQLite + PostgreSQL)
- [x] Repository implementations with sqlc
- [x] FFmpeg wrapper for metadata extraction
- [x] File scanner with extras support
- [x] REST API endpoints with streaming
- [x] React UI with TanStack Query/Router
- [x] Watch progress tracking (backend complete)
- [x] Auto-migration system
- [x] **Lines of Code**: ~15,000+ (Backend: ~8,000 | Frontend: ~7,000)
- [x] **Test Coverage**: 44.1% overall

### 🚧 In Progress (Phase 2 - Watch Progress & Transcoding)
- [x] Watch Progress domain & application layer (complete)
- [ ] Watch Progress frontend integration
- [ ] DASH transcoding service
- [ ] Background transcode queue
- [ ] Shaka Player integration
- [ ] Real-time transcode progress (SSE)

### 📋 Before Public Launch (P0 Priority)
- [ ] Remove/resolve all TODO comments
- [ ] Implement or delete no-op repositories
- [ ] Replace fmt.Printf with structured logging
- [ ] Add panic recovery to background goroutines
- [ ] Implement graceful shutdown
- [ ] Boost API test coverage to 70%+

See **[RECOMMENDATIONS.md](docs/RECOMMENDATIONS.md)** for detailed tracking.

---

## Tech Stack Details

### Backend
- **Language**: Go 1.21+
- **HTTP Framework**: Gin
- **Databases**: 
  - **SQLite** (default) - Zero-config, perfect for home/personal use
  - **PostgreSQL** - Production deployments, better multi-user performance
  - Selectable via `DB_DRIVER` environment variable
  - **All SQL must be compatible with both databases**
- **Migrations**: golang-migrate/migrate
- **SQL**: sqlc (compile-time type-safe queries)
- **API Docs**: Swagger/swaggo
- **Logging**: slog (structured JSON logging)

### Frontend
- **Framework**: React 19
- **Language**: TypeScript 5.3+
- **Build**: Vite 5
- **Router**: TanStack Router (type-safe)
- **State**: TanStack Query v5
- **UI**: Tailwind CSS + Shadcn UI
- **Video Player**: Shaka Player (DASH/HLS support)

### Infrastructure
- **Transcoding**: FFmpeg 6.0+
- **Streaming**: DASH (H.264, adaptive bitrate)
- **File Watching**: fsnotify
- **Background Jobs**: Go channels + worker pool

---

## Design Philosophy

1. **Developer Experience First**: Type safety, hot reload, code generation
2. **Clean Architecture**: Testable, maintainable, framework-agnostic domain
3. **Performance**: Efficient scanning, background transcoding, caching
4. **Extensibility**: Plugin system for custom functionality
5. **Single Binary**: Embed frontend, easy deployment
6. **Self-Hosted**: No cloud dependencies, runs on home servers

---

## Known Limitations (Current Phase)

- Single-user only (multi-user in Phase 5)
- Basic filename parsing (complex patterns in Phase 3)
- CPU transcoding only (hardware acceleration future)
- No mobile apps (web-responsive only)
- No DVR/Live TV support

---

## Resources

- **Documentation**: All in `docs/` directory
- **Project Plan**: `docs/PROJECT_PLAN.md` (8 phases, 26 weeks)
- **Recommendations**: `docs/RECOMMENDATIONS.md` (agent assessment & tracking)
- **Quick Reference**: `docs/QUICK_REFERENCE.md` (development workflow)
- **Database Schema**: `docs/DATABASE_SCHEMA.md` (all tables, queries)
- **API Spec**: `docs/API_SPECIFICATION.md` (all endpoints)
- **Testing**: `docs/TESTING.md` (strategy & coverage)

---

## License

MIT License - See LICENSE file for details.

ViewRA is open-source software, free to use, modify, and distribute.

---

## Notes for AI Assistants

### When Asked About Implementation:
1. **Check `.agent.md`** first - comprehensive AI assistant guidelines with code organization rules
2. **Check `docs/PROJECT_PLAN.md`** for which phase/task we're on
3. **Refer to `docs/ARCHITECTURE.md`** for layer separation and code organization
4. **Use `docs/DATABASE_SCHEMA.md`** for table structures and queries
5. **Follow DDD principles**: Domain has no external dependencies

### Code Organization Rules:
- **Utilities**: Use `internal/pkg/` for shared utilities (fileutil, stringutil, etc.)
- **Domain utilities**: Keep in domain if only used there (e.g., `media/filename.go`)
- **File naming**: `<verb>_<noun>.go` for use cases (e.g., `create_library.go`)
- **Constants**: Keep with domain in `types.go`
- **Sub-packages**: Create when domain > 10 files (e.g., `media/scanner/`)
- **Avoid**: "utils", "helpers", "managers", "common" packages

### When Writing Code:
- Place entities in `internal/domain/<domain>/entity.go`
- Place types/constants in `internal/domain/<domain>/types.go`
- Place use cases in `internal/application/<domain>/<verb>_<noun>.go`
- Place repositories in `internal/infrastructure/database/repository/<domain>.go`
- Place utilities in `internal/pkg/<category>/` (shared) or domain package (domain-specific)
- Use sqlc for all database queries (type-safe)
- Follow import order: stdlib → external → internal
- **Write SQL that works on BOTH SQLite and PostgreSQL**
- **Test features on BOTH databases before committing**

### Common Patterns:
```go
// Entity (domain layer)
type Media struct {
    ID       int64
    Title    string
    FilePath string
    Type     MediaType
}

// Repository interface (domain layer)
type MediaRepository interface {
    Create(ctx context.Context, media *Media) error
}

// Repository implementation (infrastructure layer)
type mediaRepository struct {
    queries *db.Queries
}

// Use case (application layer)
type CreateMedia struct {
    repo domain.MediaRepository
}

// Handler (interfaces layer)
type MediaHandler struct {
    createMedia *application.CreateMedia
}

// Utility (pkg layer - if shared across domains)
// internal/pkg/fileutil/hash.go
func HashFile(path string) (string, error) { ... }

// Domain-specific utility (domain layer - if only used by media)
// internal/domain/media/filename.go
func ParseMovieFilename(filename string) (title string, year int, error) { ... }
```

---

**Last Updated**: November 11, 2025  
**Current Focus**: Phase 0 - Project Setup
