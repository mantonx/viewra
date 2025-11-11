# ViewRA Media Server - Tech Stack

## Overview
ViewRA is a modern, self-hosted media server similar to Plex, Jellyfin, and Emby, built for home server deployments with a focus on performance and developer experience.

## Backend Stack

### Core
- **Language**: Go 1.21+
- **HTTP Router**: Gin
- **Database**: SQLite (production-ready for PostgreSQL/MySQL)
- **Database Migrations**: golang-migrate/migrate (auto-run on startup)
- **Query Builder**: sqlc (compile-time type-safe SQL)

### API & Documentation
- **API Documentation**: Swagger/swaggo (OpenAPI 3.0)
- **API Client Generation**: Orval (generates TypeScript from Swagger)

### Media Processing
- **Transcoding**: FFmpeg
  - Codec: H.264 (maximum compatibility)
  - Strategy: Smart transcoding (direct stream → DASH on-demand)
  - Thumbnail: 320x180 at 10% timestamp
  - Quality: 360p (fast), 720p/1080p (background queue)

### File System
- **File Watching**: fsnotify (auto-detect new/deleted files)
- **Scanning**: Manual trigger + auto-watch

### Development Tools
- **Hot Reload**: Air (backend) + Vite (frontend)
- **Process Manager**: overmind/foreman with Procfile (runs both services)
- **Logging**: slog (structured logging)
  - Format: Human-readable (dev), JSON (production)
  - Output: stdout (12-factor app)
  - Observability: Prometheus metrics endpoint (Phase 6+)
- **Configuration**: Environment variables (.env file)
  - Future: Admin settings UI writes to database (Phase 5+)
  - Env vars always override database settings

## Frontend Stack

### Core
- **Framework**: React 19
- **Language**: TypeScript 5.3+
- **Build Tool**: Vite 5
- **Dev Server Port**: 5173

### Routing & State
- **Router**: TanStack Router (type-safe routing)
- **Data Fetching**: TanStack Query v5
- **HTTP Client**: Native Fetch API
- **API Client**: Orval-generated (from Swagger spec)

### Media Player
- **Player**: Shaka Player
  - Supports: Direct streaming + DASH + HLS
  - Adaptive bitrate streaming
  - Future-ready for DRM

### UI & Styling
- **CSS Framework**: Tailwind CSS
- **Component Library**: shadcn/ui
- **Theme**: System preference + manual override (dark/light)
- **Layout**: Plex-style (sidebar + grid)
- **Responsive**: Mobile-responsive from day 1

### Notifications & Real-time
- **Progress Updates**: Server-Sent Events (SSE)
- **Notifications**: Toast notifications + status page

## Architecture Patterns

### Backend Architecture
- **Pattern**: Domain-Driven Design (DDD)
- **Structure**:
  - `domain/` - Business logic (framework-agnostic)
  - `application/` - Use cases
  - `infrastructure/` - External dependencies (DB, FFmpeg)
  - `interfaces/` - HTTP handlers, API

### Database Schema
- **Pattern**: Hybrid schema
  - Base `media` table (common fields)
  - Specific tables: `movies`, `tv_episodes`, `music_tracks`
  - Foreign key relationships

### Repository Pattern
- Interfaces in domain layer
- Implementations in infrastructure layer
- Dependency injection for testability

## Supported Media Types

### Video Formats
- `.mp4`, `.mkv`, `.avi`, `.mov`, `.webm`

### Audio Formats
- `.mp3`, `.flac`, `.m4a`

### Library Types
1. **Movies**
   - Metadata: Filename + FFmpeg extraction
   - Organization: Flat or folder-based

2. **TV Shows**
   - Parsing: Filename (S01E01) + folder structure
   - Organization: Show → Season → Episode

3. **Music**
   - Metadata: ID3 tags + folder fallback
   - Organization: Artist → Album → Track

## Features

### Core Features
- ✅ Multi-library support (Movies, TV, Music)
- ✅ Smart transcoding (direct → DASH when needed)
- ✅ Watch progress tracking
- ✅ Resume playback
- ✅ Thumbnail generation
- ✅ Real-time scan progress (SSE)
- ✅ Auto-watch file system changes

### Streaming Strategy
- **Primary**: Direct HTTP range requests
- **Fallback**: DASH transcoding for unsupported codecs
- **Transcoding**: Hybrid approach
  - Fast: 360p for immediate playback
  - Background: 720p/1080p in queue

### User Experience
- Single-user (multi-user auth-ready)
- Plex-style interface
- Dark/light theme toggle
- Mobile-responsive
- Toast notifications
- Dedicated status page

## Code Organization

### Package Structure
Following Domain-Driven Design with clear separation:

```
internal/
├── domain/              # Business logic (zero external dependencies)
│   └── <domain>/
│       ├── entity.go        # Entity definitions
│       ├── types.go         # Enums, constants, value objects
│       ├── repository.go    # Repository interface
│       ├── service.go       # Domain service
│       ├── errors.go        # Domain-specific errors
│       └── <utility>.go     # Domain-specific utilities
├── application/         # Use cases
│   └── <domain>/
│       ├── <verb>_<noun>.go # One file per use case
│       └── dto.go           # All DTOs for domain
├── infrastructure/      # External dependencies
│   ├── database/
│   │   ├── repository/      # Repository implementations
│   │   └── sqlc/            # SQL queries & generated code
│   ├── ffmpeg/              # FFmpeg wrapper
│   ├── filesystem/          # File operations
│   └── queue/               # Background jobs
├── interfaces/          # HTTP/CLI
│   └── http/
│       └── handlers/<domain>/
│           ├── handler.go   # Handler + constructor
│           ├── routes.go    # Route registration
│           └── dto.go       # API-specific DTOs
└── pkg/                 # Shared utilities (project-specific)
    ├── config/              # Configuration
    ├── logger/              # Logging wrapper
    ├── errors/              # Error handling
    ├── validator/           # Validation utilities
    ├── fileutil/            # File operations (hash, size)
    ├── stringutil/          # String manipulation
    └── timeutil/            # Time/date utilities
```

### File Naming Conventions
- **Entities**: `entity.go` (main entity in domain)
- **Types**: `types.go` (enums, constants)
- **Use Cases**: `<verb>_<noun>.go` (e.g., `create_library.go`, `scan_library.go`)
- **Tests**: `<filename>_test.go` (next to implementation)
- **Repositories**: `<domain>.go` (one file per domain in infrastructure/database/repository/)

### Code Growth Guidelines
- **File > 500 lines**: Split by responsibility
- **Package > 10 files**: Create sub-packages
- **Duplicated code (3+ times)**: Extract to utility or service
- **Domain-specific utility**: Keep in domain package
- **Shared utility**: Move to `internal/pkg/<category>/`

### Anti-Patterns to Avoid
- ❌ `internal/utils/` or `internal/helpers/` (too generic)
- ❌ `internal/common/` (unclear purpose)
- ❌ `internal/managers/` (god objects)
- ✅ `internal/pkg/fileutil/` (specific, clear purpose)
- ✅ `internal/domain/media/` (domain-driven)

## Deployment

### Development
- Backend: Port 3000
- Frontend: Port 5173 (Vite dev server)
- Auto-reload: Air (backend), Vite (frontend)

### Production
- **Build**: Embedded frontend in Go binary
- **Database**: Auto-migrate on startup
- **Single Binary**: Easy deployment to home servers

### Data Directory Structure
```
/data
  /libraries       # User media (scanned paths)
  /thumbnails      # Generated thumbnails
    /movies
    /tv
    /music
  /dash           # Transcoded DASH files
    /{media-id}
      manifest.mpd
      /360p
      /720p
      /1080p
  /cache          # Temporary files
  viewra2.db      # SQLite database
```

## Configuration

### Environment Variables
```bash
# Server
PORT=3000
DATABASE_URL=sqlite://data/viewra2.db
DATA_DIR=/data
FRONTEND_URL=http://localhost:5173
CORS_ORIGINS=http://localhost:5173,http://localhost:3000

# Logging
LOG_LEVEL=info           # debug, info, warn, error
LOG_FORMAT=json          # json, text
LOG_FILE=                # Optional: file path for logging

# Metadata (Phase 2)
TMDB_API_KEY=            # The Movie Database API key
TVDB_API_KEY=            # TheTVDB API key
METADATA_ENABLED=false   # Enable external metadata fetching
METADATA_AUTO_MATCH=true # Auto-apply high-confidence matches
METADATA_CONFIDENCE=0.8  # Minimum confidence for auto-match

# Debugging
ENV=production           # development, production
DEBUG_ENDPOINTS=false    # Enable /debug/* endpoints (dev only)
```

### Logging Configuration

**Development**:
- Level: `debug`
- Format: Pretty text output
- Output: stdout
- Stack traces: Enabled

**Production**:
- Level: `info`
- Format: JSON
- Output: stdout (Docker captures)
- Stack traces: Disabled (performance)

**Request Tracing**: All logs include `request_id` for tracking requests across operations.

## Performance Considerations

### Optimizations
- Compile-time type safety (sqlc)
- Zero-bundle HTTP client (native Fetch)
- Efficient file watching (fsnotify)
- Background transcoding queue
- In-memory caching (future: Redis)

### Scalability
- PostgreSQL-ready migrations
- Repository pattern for easy DB switching
- Horizontal scaling ready (with shared storage)
- CDN-ready (DASH segments)

## Security

### Current (Single-User)
- CORS configuration
- File path validation (absolute paths, no traversal)
- Input sanitization
- WebP image serving (security + performance)

### Future (Multi-User)
- JWT authentication

---

## Development Workflow

### Initial Setup
- **Setup Script**: `scripts/setup.sh` validates environment (Go, Node, FFmpeg)
- **Makefile**: Daily development commands (`make dev`, `make test`, `make sqlc`)
- **Process Manager**: Procfile with overmind/foreman runs backend + frontend together

### Hot Reload
- **Backend**: Air watches Go files, rebuilds on change
- **Frontend**: Vite dev server with HMR
- **Single Command**: `overmind start` or `make dev` runs both

### Frontend Build Strategy
- **Development**: Separate servers (backend :3000, frontend :5173) with CORS
- **Production**: `vite build` → embed in Go binary via `embed.FS`
- **Deployment**: Single binary with all static assets included

### Configuration
- **Development**: `.env` file (gitignored)
- **Production**: Environment variables
- **Future Admin UI**: Settings stored in database, env vars override

### Test Data
- **Script**: `scripts/download-test-media.sh` fetches public domain samples
- **Location**: `test-data/` (gitignored)
- **Samples**: Big Buck Bunny, Sintel clips for consistent testing

---

## Technical Decisions

### FFmpeg Detection
- **Strategy**: Runtime detection with graceful degradation
- **Behavior**: App starts without FFmpeg, transcoding features disabled in UI
- **Minimum Version**: FFmpeg 5.0+ recommended
- **Validation**: Check on startup, log version and capabilities

### File Hashing
- **Strategy**: Partial hash (first 64KB + last 64KB + file size)
- **Purpose**: Fast duplicate detection during scans
- **Performance**: Instant for large files vs. 30+ seconds for full hash
- **Accuracy**: 99.9% duplicate detection (same as Plex/Jellyfin)

### Database Migrations
- **Strategy**: Auto-run on startup with version check
- **Behavior**: Only run if database version < code version
- **Override**: `AUTO_MIGRATE=false` for manual control in production
- **Backup**: Automatic backup before running migrations

### Database Backups
- **Automatic**: Daily backups at 3am (configurable)
- **Retention**: Keep last 7 days
- **Location**: `data/backups/`
- **Manual**: Admin UI button for on-demand backup
- **Migration Safety**: Automatic backup before migrations

### Static File Serving
- **Images**: Convert all to WebP for storage and serving
- **Location**: `data/thumbnails/`, `data/posters/`, `data/backdrops/`
- **Server**: Go serves files with caching headers and range request support
- **Production**: Optional Nginx/Caddy reverse proxy for performance

### Transcode File Cleanup
- **Strategy**: LRU cache with configurable disk limit
- **Default Limit**: 50GB for transcoded DASH segments
- **Behavior**: Delete oldest accessed files when limit reached
- **Job Records**: Keep in database for history, clean files only

---

## Future (Multi-User)
- JWT authentication
- Role-based access control
- Rate limiting
- HTTPS via reverse proxy (Caddy recommended)

## Testing Strategy

### Approach
- Unit tests for domain logic
- Integration tests for repositories
- E2E tests for critical paths
- In-memory SQLite for test database

## Version Control

### Branching
- `main` - production-ready
- `develop` - active development
- Feature branches for new features

## Decision Log

All architectural decisions documented in:
- `docs/ARCHITECTURE.md`
- `docs/decisions/` (ADRs - Architecture Decision Records)
