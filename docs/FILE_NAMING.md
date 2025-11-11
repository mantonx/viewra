# File Naming Conventions

> Quick reference for naming files and directories in the ViewRA project

**Last Updated**: November 11, 2025

---

## Table of Contents

1. [Core Principles](#core-principles)
2. [Go Code Files](#go-code-files)
3. [Directory Structure](#directory-structure)
4. [Media Files (Scanner)](#media-files-scanner)
5. [Frontend Files](#frontend-files)
6. [Configuration Files](#configuration-files)
7. [Anti-Patterns](#anti-patterns)

---

## Core Principles

### Base Names on WHAT, Not WHEN

✅ **Descriptive** (based on responsibility):
```
config.go
loader.go
validator.go
scanner.go
```

❌ **Temporal** (based on when created):
```
config_v2.go
config_new.go
config_enhanced.go
config_final.go
```

### One Responsibility Per File

Files should have a clear, single purpose reflected in the name.

```
✅ hash.go           # File hashing
✅ validator.go      # Input validation
✅ scanner.go        # Directory scanning

❌ utils.go          # Too broad
❌ helpers.go        # Unclear purpose
❌ misc.go           # Catch-all
```

---

## Go Code Files

### Domain Layer (`internal/domain/<domain>/`)

| File | Purpose | Example |
|------|---------|---------|
| `entity.go` | Main entity definition | Library, Media, User |
| `types.go` | Enums, constants, value objects | MediaType, LibraryType |
| `repository.go` | Repository interface | LibraryRepository, MediaRepository |
| `service.go` | Domain service | LibraryService, MediaService |
| `errors.go` | Domain-specific errors | ErrLibraryNotFound |
| `validator.go` | Validation logic | Path validation, title validation |
| `events.go` | Domain events (future) | LibraryScanned, MediaAdded |
| `<specific>.go` | Domain-specific utilities | `filename.go` for parsing movie filenames |

**Example Structure**:
```
internal/domain/library/
├── entity.go           # Library entity
├── types.go            # LibraryType enum
├── repository.go       # LibraryRepository interface
├── service.go          # LibraryService
├── errors.go           # ErrLibraryNotFound, etc.
└── validator.go        # Path validation
```

### Application Layer (`internal/application/<domain>/`)

**Pattern**: `<verb>_<noun>.go` for use cases

| File | Purpose |
|------|---------|
| `create_library.go` | Create library use case |
| `update_library.go` | Update library use case |
| `delete_library.go` | Delete library use case |
| `list_libraries.go` | List libraries use case |
| `scan_library.go` | Scan library use case |
| `get_media.go` | Get media use case |
| `stream_media.go` | Stream media use case |
| `search_media.go` | Search media use case |
| `update_progress.go` | Update watch progress use case |
| `dto.go` | All DTOs for the domain |

**Example Structure**:
```
internal/application/library/
├── create_library.go   # CreateLibrary use case
├── update_library.go   # UpdateLibrary use case
├── delete_library.go   # DeleteLibrary use case
├── list_libraries.go   # ListLibraries use case
├── scan_library.go     # ScanLibrary use case
└── dto.go              # LibraryDTO, CreateLibraryRequest, etc.
```

### Infrastructure Layer (`internal/infrastructure/`)

#### Database

| File | Purpose |
|------|---------|
| `connection.go` | Database connection setup |
| `migrate.go` | Migration runner |
| `repository/<domain>.go` | Repository implementation per domain |

**Example Structure**:
```
internal/infrastructure/database/
├── connection.go
├── migrate.go
├── repository/
│   ├── library.go      # LibraryRepository implementation
│   ├── media.go        # MediaRepository implementation
│   └── watch.go        # WatchRepository implementation
└── sqlc/
    ├── schema.sql
    └── queries/
        ├── library.sql
        ├── media.sql
        └── watch.sql
```

#### Other Infrastructure

| Directory | File Pattern | Purpose |
|-----------|--------------|---------|
| `ffmpeg/` | `<feature>.go` | `client.go`, `metadata.go`, `transcoder.go` |
| `filesystem/` | `<feature>.go` | `scanner.go`, `watcher.go`, `validator.go` |
| `queue/` | `<feature>.go` | `job.go`, `worker.go`, `transcode_queue.go` |
| `storage/` | `<feature>.go` | `local.go`, `paths.go` |

### Interface Layer (`internal/interfaces/http/`)

#### Handlers (`handlers/<domain>/`)

| File | Purpose |
|------|---------|
| `handler.go` | Handler struct + constructor |
| `routes.go` | Route registration |
| `dto.go` | API-specific DTOs (request/response) |
| `response.go` | Response helper functions |
| `<feature>.go` | Special handlers (e.g., `stream.go`, `thumbnail.go`) |

**Example Structure**:
```
internal/interfaces/http/handlers/library/
├── handler.go          # LibraryHandler struct
├── routes.go           # RegisterRoutes(r *gin.Engine)
├── dto.go              # API request/response types
└── response.go         # Helper functions

internal/interfaces/http/handlers/media/
├── handler.go
├── routes.go
├── dto.go
├── stream.go           # Streaming-specific handler
└── thumbnail.go        # Thumbnail-specific handler
```

#### Middleware (`middleware/`)

One file per middleware:
```
internal/interfaces/http/middleware/
├── cors.go
├── logging.go
├── recovery.go
├── auth.go             # Future
└── ratelimit.go        # Future
```

### Shared Packages (`internal/pkg/`)

**Pattern**: `<category>/<feature>.go`

```
internal/pkg/
├── config/
│   ├── config.go       # Config struct
│   ├── loader.go       # Load from env/file
│   └── validator.go    # Validate config
├── logger/
│   └── logger.go       # Logging setup
├── errors/
│   ├── errors.go       # Error types
│   └── handler.go      # HTTP error mapping
├── validator/
│   ├── path.go         # Path validation
│   └── url.go          # URL validation
├── fileutil/
│   ├── hash.go         # File hashing
│   ├── size.go         # Human-readable sizes
│   └── exists.go       # File existence checks
├── stringutil/
│   ├── slug.go         # Slugify
│   └── trim.go         # Custom trim
└── timeutil/
    └── parse.go        # Time parsing
```

### Test Files

**Pattern**: `<filename>_test.go`

Always next to the implementation:
```
internal/domain/media/
├── entity.go
├── entity_test.go      # Next to implementation
├── service.go
├── service_test.go
└── testdata/           # Test fixtures
    ├── sample.mp4
    └── sample.json
```

### Test Utilities (`internal/testutil/`)

```
internal/testutil/
├── database.go         # Test DB setup
├── fixtures.go         # Common test data
├── assert.go           # Custom assertions
└── mock/               # Mock implementations
    ├── repository.go
    └── service.go
```

---

## Directory Structure

### When to Create Sub-Packages

**Trigger**: Domain package grows beyond 10 files

**Before** (too many files):
```
internal/domain/media/
├── entity.go
├── types.go
├── repository.go
├── service.go
├── errors.go
├── validator.go
├── scanner.go
├── parser.go
├── matcher.go
├── extractor.go
├── enricher.go
├── transcoder.go
└── queue.go            # 13 files - too many!
```

**After** (organized with sub-packages):
```
internal/domain/media/
├── entity.go
├── types.go
├── repository.go
├── service.go
├── errors.go
├── scanner/            # Sub-package
│   ├── scanner.go
│   ├── parser.go
│   └── matcher.go
├── metadata/           # Sub-package
│   ├── extractor.go
│   └── enricher.go
└── transcode/          # Sub-package
    ├── job.go
    └── queue.go
```

### Package Naming

✅ **Good Package Names**:
```
library/            # Clear domain
media/              # Clear domain
fileutil/           # Specific utility purpose
validator/          # Clear responsibility
```

❌ **Bad Package Names**:
```
utils/              # Too generic
helpers/            # What kind of helpers?
common/             # Common to what?
managers/           # Too vague
misc/               # Catch-all
```

---

## Media Files (Scanner)

These conventions are for **users' media files** that ViewRA will scan.

### Movies

✅ **Supported Formats**:
```
The Matrix (1999).mp4
The.Matrix.1999.1080p.BluRay.x264.mp4
/movies/The Matrix (1999)/The Matrix (1999).mkv
Inception.2010.mp4
```

**Pattern**: `<Title> (<Year>).<ext>` or `<Title>.<Year>.<quality>.<ext>`

### TV Shows

✅ **Supported Formats**:
```
Breaking Bad/Season 01/Breaking Bad - S01E01.mkv
Breaking.Bad/S01/Breaking.Bad.S01E01.Pilot.mkv
/tv/Anime/Attack.on.Titan/Attack.on.Titan.-.01.mkv
Game.of.Thrones.S01E01.Winter.Is.Coming.mkv
```

**Patterns**:
- Standard: `S##E##` (S01E01)
- Alternative: `#x##` (1x01)
- Absolute: `##` for anime (01, 02, etc.)

### Music

✅ **Supported Formats**:
```
Pink Floyd/The Dark Side of the Moon/01 - Speak to Me.flac
Artist Name/Album Name/Track Number - Track Name.mp3
Beatles, The/Abbey Road/01 - Come Together.mp3
```

**Pattern**: `<Artist>/<Album>/<TrackNumber> - <Title>.<ext>`

### File Extensions

| Media Type | Extensions |
|------------|------------|
| Video | `.mp4`, `.mkv`, `.avi`, `.mov`, `.webm` |
| Audio | `.mp3`, `.flac`, `.m4a`, `.wav`, `.ogg` |

---

## Frontend Files

### Core Pattern

**All frontend files follow this structure**:

```
<name>/
├── <name>.ts           # Implementation
├── <name>.types.ts     # TypeScript types/interfaces
├── <name>.test.ts      # Unit tests
└── index.ts            # Barrel export
```

For React components, use `.tsx` instead of `.ts`:

```
<ComponentName>/
├── <ComponentName>.tsx        # Component implementation
├── <ComponentName>.types.ts   # Props, state, types
├── <ComponentName>.test.tsx   # Component tests
└── index.ts                   # Barrel export
```

### React Components (`web/src/components/`)

**Pattern**: `PascalCase` directory with matching component file

```
web/src/components/
├── MediaCard/
│   ├── MediaCard.tsx          # Component
│   ├── MediaCard.types.ts     # Props & types
│   ├── MediaCard.test.tsx     # Tests
│   └── index.ts               # export { MediaCard } from './MediaCard'
├── LibraryGrid/
│   ├── LibraryGrid.tsx
│   ├── LibraryGrid.types.ts
│   ├── LibraryGrid.test.tsx
│   └── index.ts
├── VideoPlayer/
│   ├── VideoPlayer.tsx
│   ├── VideoPlayer.types.ts
│   ├── VideoPlayer.test.tsx
│   └── index.ts
└── ui/                        # Shadcn components (exception: flat structure)
    ├── button.tsx
    ├── card.tsx
    └── input.tsx
```

**Example Component Structure**:

```typescript
// MediaCard/MediaCard.types.ts
export interface MediaCardProps {
  title: string;
  thumbnailUrl?: string;
  onPlay: () => void;
}

// MediaCard/MediaCard.tsx
import type { MediaCardProps } from './MediaCard.types';

export const MediaCard = ({ title, thumbnailUrl, onPlay }: MediaCardProps) => {
  // Component implementation
};

// MediaCard/index.ts
export { MediaCard } from './MediaCard';
export type { MediaCardProps } from './MediaCard.types';
```

### Hooks (`web/src/hooks/`)

**Pattern**: `use<Name>` directory structure

```
web/src/hooks/
├── useMediaPlayer/
│   ├── useMediaPlayer.ts      # Hook implementation
│   ├── useMediaPlayer.types.ts # Return types, params
│   ├── useMediaPlayer.test.ts  # Hook tests
│   └── index.ts
├── useWatchProgress/
│   ├── useWatchProgress.ts
│   ├── useWatchProgress.types.ts
│   ├── useWatchProgress.test.ts
│   └── index.ts
└── useLibraries/
    ├── useLibraries.ts
    ├── useLibraries.types.ts
    ├── useLibraries.test.ts
    └── index.ts
```

### Routes (`web/src/routes/`)

**Pattern**: TanStack Router file-based routing (exception: flat files)

```
web/src/routes/
├── __root.tsx          # Root layout
├── index.tsx           # Home page (/)
├── libraries.tsx       # /libraries
├── libraries/
│   ├── $id.tsx         # /libraries/:id
│   └── $id/
│       └── edit.tsx    # /libraries/:id/edit
└── media/
    └── $id.tsx         # /media/:id
```

**Note**: Routes follow TanStack Router conventions and don't use the directory pattern.

### Utilities & Helpers (`web/src/lib/`)

**Pattern**: Directory per utility module

```
web/src/lib/
├── api/
│   ├── generated/          # Orval generated (gitignored)
│   └── mutator/
│       ├── custom-instance.ts
│       ├── custom-instance.types.ts
│       ├── custom-instance.test.ts
│       └── index.ts
├── utils/
│   ├── formatters/
│   │   ├── formatters.ts        # Date, time, file size formatting
│   │   ├── formatters.types.ts
│   │   ├── formatters.test.ts
│   │   └── index.ts
│   ├── validators/
│   │   ├── validators.ts        # Input validation helpers
│   │   ├── validators.types.ts
│   │   ├── validators.test.ts
│   │   └── index.ts
│   └── index.ts                 # Re-export all utils
├── constants/
│   ├── constants.ts             # App constants
│   ├── constants.types.ts
│   └── index.ts
└── types/
    ├── common.types.ts          # Shared types
    ├── media.types.ts           # Media-related types
    └── index.ts
```

### Pages/Features (`web/src/features/`)

**Pattern**: Feature-based organization

```
web/src/features/
├── libraries/
│   ├── components/
│   │   ├── LibraryCard/
│   │   │   ├── LibraryCard.tsx
│   │   │   ├── LibraryCard.types.ts
│   │   │   ├── LibraryCard.test.tsx
│   │   │   └── index.ts
│   │   └── ScanButton/
│   │       ├── ScanButton.tsx
│   │       ├── ScanButton.types.ts
│   │       ├── ScanButton.test.tsx
│   │       └── index.ts
│   └── hooks/
│       └── useLibraryScanner/
│           ├── useLibraryScanner.ts
│           ├── useLibraryScanner.types.ts
│           ├── useLibraryScanner.test.ts
│           └── index.ts
└── media/
    ├── components/
    │   └── MediaGrid/
    │       ├── MediaGrid.tsx
    │       ├── MediaGrid.types.ts
    │       ├── MediaGrid.test.tsx
    │       └── index.ts
    └── hooks/
        └── useMediaFilters/
            ├── useMediaFilters.ts
            ├── useMediaFilters.types.ts
            ├── useMediaFilters.test.ts
            └── index.ts
```

### Context Providers (`web/src/context/`)

**Pattern**: `<Name>Provider` directory

```
web/src/context/
├── ThemeProvider/
│   ├── ThemeProvider.tsx
│   ├── ThemeProvider.types.ts
│   ├── ThemeProvider.test.tsx
│   └── index.ts
└── AuthProvider/
    ├── AuthProvider.tsx
    ├── AuthProvider.types.ts
    ├── AuthProvider.test.tsx
    └── index.ts
```

### Naming Rules

| Type | Pattern | Example |
|------|---------|---------|
| **Components** | `PascalCase/` | `MediaCard/`, `VideoPlayer/` |
| **Hooks** | `use<Name>/` | `useMediaPlayer/`, `useAuth/` |
| **Utilities** | `camelCase/` | `formatters/`, `validators/` |
| **Types** | `*.types.ts` | `MediaCard.types.ts` |
| **Tests** | `*.test.ts(x)` | `MediaCard.test.tsx` |
| **Barrel** | `index.ts` | Exports main functionality |

### Import/Export Pattern

**Barrel exports** (`index.ts`):

```typescript
// Good: Named exports
export { MediaCard } from './MediaCard';
export type { MediaCardProps } from './MediaCard.types';

// Bad: Default export
export { default } from './MediaCard';  // ❌ Avoid
```

**Component imports**:

```typescript
// Good: Import from directory
import { MediaCard } from '@/components/MediaCard';
import type { MediaCardProps } from '@/components/MediaCard';

// Bad: Import from file directly
import { MediaCard } from '@/components/MediaCard/MediaCard';  // ❌ Skip barrel
```

---

## Configuration Files

### Root Directory

| File | Purpose |
|------|---------|
| `go.mod` | Go module definition |
| `go.sum` | Go dependencies checksum |
| `.gitignore` | Git ignore rules |
| `.air.toml` | Air hot reload config |
| `sqlc.yaml` | sqlc configuration |
| `Makefile` | Common tasks |
| `.env` | Environment variables (not committed) |
| `.env.example` | Example environment variables |

### Configs Directory (`configs/`)

```
configs/
├── development.yaml    # Dev environment config
├── production.yaml     # Prod environment config
└── test.yaml           # Test environment config
```

### Migrations (`migrations/`)

**Pattern**: `<number>_<name>.up.sql` and `<number>_<name>.down.sql`

```
migrations/
├── 000001_init.up.sql
├── 000001_init.down.sql
├── 000002_add_tv_shows.up.sql
├── 000002_add_tv_shows.down.sql
├── 000003_add_music.up.sql
└── 000003_add_music.down.sql
```

**Naming Rules**:
- 6-digit sequential number
- Lowercase with underscores
- Descriptive name
- Both `.up.sql` and `.down.sql`

---

## Anti-Patterns

### ❌ Version Suffixes

**NEVER use these**:
```
config_old.go
config_new.go
config_v2.go
config_enhanced.go
config_improved.go
config_refactored.go
config_final.go
config_backup.go
service_temp.go
```

**Why**: Git already tracks versions. Update files directly.

### ❌ Generic Names

**NEVER use these**:
```
utils.go            # Too generic
helpers.go          # What kind of helpers?
misc.go             # Catch-all
common.go           # Common to what?
stuff.go            # Meaningless
tools.go            # Too vague
```

**Instead**: Use specific, responsibility-based names:
```
✅ fileutil/hash.go      # File hashing utility
✅ validator/path.go     # Path validation
✅ stringutil/slug.go    # Slugify strings
```

### ❌ Manager/Handler Suffixes Everywhere

**Avoid overuse**:
```
library_manager.go      # Just use service.go
media_manager.go        # Just use service.go
config_handler.go       # Just use loader.go
```

**When to use**:
- `handler.go` - OK for HTTP handlers
- `service.go` - OK for domain services
- `manager.go` - Avoid in domain layer

### ❌ Redundant Directory + File Names

**NEVER do this**:
```
internal/domain/library/library.go          # Redundant
internal/infrastructure/database/database.go # Redundant
```

**Instead**:
```
internal/domain/library/entity.go           # Specific purpose
internal/infrastructure/database/connection.go # What it does
```

### ❌ Deep Nesting

**Avoid**:
```
internal/infrastructure/database/repository/implementation/mysql/library/
```

**Keep it flat**:
```
internal/infrastructure/database/repository/library.go
```

---

## Quick Reference

### Go Files by Layer

| Layer | File Pattern | Example |
|-------|--------------|---------|
| **Domain** | `entity.go`, `types.go`, `repository.go`, `service.go`, `errors.go` | `library/entity.go` |
| **Application** | `<verb>_<noun>.go`, `dto.go` | `create_library.go` |
| **Infrastructure** | `<feature>.go`, `<domain>.go` (repos) | `ffmpeg/transcoder.go` |
| **Interface** | `handler.go`, `routes.go`, `dto.go` | `library/handler.go` |
| **Shared** | `<category>/<feature>.go` | `fileutil/hash.go` |

### Test Files

| Type | Pattern | Location |
|------|---------|----------|
| Unit tests | `<file>_test.go` | Next to implementation |
| Test data | `testdata/` | Next to test files |
| Test utils | `<feature>.go` | `internal/testutil/` |
| Integration | `<feature>_test.go` | `tests/integration/` |
| E2E | `<feature>_test.go` | `tests/e2e/` |

### When File > 500 Lines

Split by responsibility:
```
service.go (1000 lines) →
  service.go (core logic)
  scanner.go (scanning)
  validator.go (validation)
```

### When Package > 10 Files

Create sub-packages:
```
media/ (15 files) →
  media/
    entity.go
    service.go
    scanner/
    metadata/
    transcode/
```

---

## Checklist Before Committing

- [ ] No version suffixes (`_old`, `_new`, `_v2`, `_enhanced`)
- [ ] No generic names (`utils.go`, `helpers.go`, `common.go`)
- [ ] Files named by responsibility, not by when created
- [ ] Test files next to implementation (`<file>_test.go`)
- [ ] Package names are clear and specific
- [ ] No redundant directory + file names
- [ ] No deep nesting (keep it flat)
- [ ] Use cases follow `<verb>_<noun>.go` pattern
- [ ] Domain files follow standard names (`entity.go`, `types.go`, etc.)

---

## See Also

- **ARCHITECTURE.md** - Layer organization and patterns
- **TECH_STACK.md** - Code organization details
- **.agent.md** - AI assistant coding guidelines
- **NOTES.md** - Development journal with commands

---

**Last Updated**: November 11, 2025  
**Version**: 1.0
