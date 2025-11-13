# ViewRA Project Plan

## Current Status

**Phase**: Phase 1 Complete ✅ (November 12, 2025)
**Next**: Phase 2 - Watch Progress & Transcoding
**Target MVP**: Phase 2 Complete
**Start Date**: November 11, 2025

---

## Completed Phases

### Phase 0: Project Setup ✅ Complete (Nov 11, 2025)
- Repository structure, documentation (ARCHITECTURE, API_SPECIFICATION, DATABASE_SCHEMA, TECH_STACK, PLUGIN_ARCHITECTURE, CONVENTIONS)
- Development tools (Air, sqlc, Swagger, Orval, ESLint, Biome)
- Git repository, Makefile, Procfile for dev workflow

### Phase 1: Core Foundation ✅ Complete (Nov 12, 2025)
- **Domain Layer**: Libraries, media, scanner entities and business logic
- **Infrastructure**: Dual database support (SQLite + PostgreSQL), FFmpeg integration, filesystem scanner with extras support, path browser with security validation
- **Application Layer**: Use cases for library management, media scanning, filesystem browsing
- **API Layer**: REST endpoints with Swagger docs, streaming with range support, scan progress tracking
- **Frontend**: React UI with TanStack Query/Router, TypeScript API client, accessibility features (keyboard nav, ARIA labels), library management UI, filesystem browser
- **Database**: Auto-migration system with backups, incremental scanner
- **Lines of Code**: ~15,000+ (Backend: ~8,000 | Frontend: ~7,000)
- **Test Coverage**: 44.1% overall (Domain: 88.9%, Application: 62.5%, Infrastructure: 72.5%)

**See [ROADMAP.md](./ROADMAP.md) for detailed implementation history.**

---

## Phase 2: Watch Progress & Transcoding (IN PROGRESS)

**Goal**: Track viewing progress and enable adaptive streaming

**Status**: 75% Complete (Phase 2.1 Complete, Phase 2.2 In Progress)
**Started**: November 12, 2025
**Estimated Effort**: 2-3 weeks

### Key Features

#### Phase 2.1: Watch Progress Tracking ✅ COMPLETE (Nov 13, 2025)
- ✅ Per-user progress tracking (resume from last position)
- ✅ Auto-mark as watched at 90% completion
- ✅ Watch history and recently watched views
- ✅ Progress indicators on media cards
- ✅ Continue Watching section on home page
- ✅ Full video player with progress tracking

**Database**: ✅ Complete
- ✅ `watch_progress` table (in 000001_init.up.sql migration)
- ✅ Domain: `WatchProgress` entity + repository interface
- ✅ Application: Update/get progress use cases with tests
- ✅ Infrastructure: Repository with dual database support
- ✅ API: All progress endpoints (GET, PUT, POST, DELETE)
- ✅ Frontend: Progress bars, resume buttons, video player, Continue Watching section

#### Phase 2.2: DASH Transcoding 🚧 IN PROGRESS (25% Complete)
- ⏳ Background transcoding queue (channel-based worker pool)
- ⏳ Multi-quality transcoding (360p fast, 720p/1080p background)
- ⏳ DASH manifest generation for adaptive streaming
- ⏳ Real-time transcode progress (SSE)

**Components**: 25% Complete
- ✅ `transcode_jobs` table schema (migration 000006 - staged)
- ✅ Database queries for transcode jobs (staged)
- ⏳ Transcode job repository implementation
- ⏳ FFmpeg DASH transcoding service
- ⏳ Job queue with worker pool
- ⏳ API: `POST /api/media/:id/transcode`, `GET /api/media/:id/manifest.mpd`, `GET /api/media/:id/transcode/status`
- ⏳ Frontend: Shaka Player with adaptive streaming, quality selector, buffering indicators

### Success Criteria
- ✅ Can resume videos from last watched position
- ✅ Videos >90% watched are auto-marked as "watched"
- ✅ Unsupported codecs trigger automatic transcoding
- ✅ 360p version generates quickly (< 30 seconds)
- ✅ Player adapts quality based on available bandwidth
- ✅ Transcode progress updates in real-time via SSE

### Implementation Status

**Phase 2.1 - Watch Progress**: ✅ COMPLETE
- All backend layers implemented (Domain, Application, Infrastructure, API)
- Full frontend implementation with VideoPlayer and ContinueWatching
- Ready for integration testing

**Phase 2.2 - DASH Transcoding**: 🚧 IN PROGRESS
- Database schema ready (staged, not committed)
- Next: Implement transcoding service and job queue
- Next: Upgrade to Shaka Player
- Transcoding queue will use Go channels and worker pools for concurrency

**Unstaged Work** (needs commit):
- Modified: MediaCard.tsx, MediaDetailsModal.tsx, routes/index.tsx
- New: VideoPlayer/, ContinueWatching.tsx, Progress/ component
- New: transcode_jobs migrations and queries

---

## Phase 3: TV Shows & Music (Weeks 8-10)

**Goal**: Full support for TV shows and music libraries with type-specific metadata

**Status**: Not Started
**Estimated Effort**: 2-3 weeks

### Key Features
- **Movie Metadata**: Implement MovieRepository (currently no-op), extract year/genre/director from filenames and NFO files
- **TV Show Support**: Implement TVRepository (currently no-op), auto-create show/season records, episode grouping and ordering, "Next Episode" functionality
- **Music Support**: Implement MusicRepository (currently no-op), ID3 tag extraction, album/artist grouping, audio player component
- **Frontend**: TV show pages (show grid → season view → episode list), music pages (artist grid → albums → tracks), playlist queue

### Database
- Tables already exist (`movies`, `tv_shows`, `tv_seasons`, `tv_episodes`, `music_tracks`)
- Need to implement repositories and sqlc queries
- Follow same dual-database pattern as MediaRepository

### Success Criteria
- ✅ TV shows parse correctly (S01E01, 1x01 formats)
- ✅ Episodes group by show and season with proper ordering
- ✅ Music tracks extract ID3 tags (artist, album, track number)
- ✅ Can track watch progress per episode
- ✅ Audio player works for music files

---

## Phase 4: Enhanced Metadata (Weeks 11-13)

**Goal**: Rich metadata from external sources (TMDb, MusicBrainz)

**Status**: Not Started
**Estimated Effort**: 2-3 weeks

### Key Features
- **Plugin System**: Plugin registry, plugin manager, loading/unloading, configuration storage, plugin SDK interfaces
- **TMDb Integration**: Movie/TV search and details, poster/backdrop downloads, cast and crew information
- **MusicBrainz Integration**: Artist/album search, track matching, cover art fetching
- **Rich Metadata**: Collections support (MCU, Star Wars), people/credits entities, manual metadata matching UI

### Success Criteria
- ✅ TMDb correctly identifies movies by title + year
- ✅ Posters and backdrops download successfully
- ✅ Cast and crew display on media detail pages
- ✅ Collections group related movies
- ✅ Can manually override incorrect matches

---

## Phase 5: User Features & Polish (Weeks 14-16)

**Goal**: Multi-user support and UX improvements

**Status**: Not Started
**Estimated Effort**: 2-3 weeks

### Key Features
- **Authentication**: User accounts, JWT authentication, password hashing (bcrypt), login/logout, registration
- **User Features**: Watch history, personal ratings, tags and lists, playlists
- **UX Improvements**: Search across all media, advanced filters, sorting options, dark mode, keyboard shortcuts
- **Polish**: Loading states, error messages, empty states, accessibility audit

### Success Criteria
- ✅ Multiple users can maintain separate watch progress
- ✅ Authentication is secure (JWT + bcrypt)
- ✅ Search works across all media types
- ✅ UI is responsive and accessible

---

## Phase 6: Advanced Features (Weeks 17-20)

**Goal**: Advanced playback and social features

**Status**: Not Started
**Estimated Effort**: 3-4 weeks

### Key Features
- **Advanced Player**: Subtitle support (external SRT/VTT, embedded), multi-audio track support, playback speed controls, chapter markers
- **Social Features**: Watch together (synchronized playback), comments and reviews, recommendations engine
- **Performance**: Thumbnail generation and sprites, preloading and prefetching, CDN support, caching improvements

### Success Criteria
- ✅ Can switch between audio tracks and subtitles
- ✅ Watch Together synchronizes playback across users
- ✅ Recommendations are relevant
- ✅ Thumbnails load quickly

---

## Phase 7: Plugin Ecosystem (Weeks 21-24)

**Goal**: Extensible plugin architecture

**Status**: Not Started
**Estimated Effort**: 3-4 weeks

### Key Features
- **Plugin SDK**: Public plugin API, versioning, authentication/authorization for plugins, plugin marketplace UI
- **Core Plugins**: TMDb (movies/TV), MusicBrainz (music), OpenSubtitles (subtitles), Trakt (watch sync)
- **Plugin Manager**: Install/uninstall UI, configuration UI, plugin updates, sandboxing and permissions

### Success Criteria
- ✅ Third-party developers can write plugins
- ✅ Plugins can extend metadata sources
- ✅ Plugin marketplace allows discovery and installation
- ✅ Plugins are sandboxed and secure

---

## Phase 8: Deployment & Production (Weeks 25-26)

**Goal**: Production-ready deployment

**Status**: Not Started
**Estimated Effort**: 1-2 weeks

### Key Features
- **Packaging**: Docker images, Docker Compose setup, Kubernetes manifests (optional), ARM builds (Raspberry Pi)
- **Deployment Guide**: Installation instructions, reverse proxy setup (nginx), SSL/TLS configuration, backup and restore procedures
- **Monitoring**: Logging improvements (JSON format), metrics and health checks, error tracking integration, performance monitoring
- **Documentation**: User guide, administrator guide, API documentation, plugin development guide

### Success Criteria
- ✅ Can deploy with Docker in < 5 minutes
- ✅ Documentation is complete and clear
- ✅ Monitoring provides visibility into system health
- ✅ Backups and restores work reliably

---

## Milestones

### MVP (Minimum Viable Product)
**Target**: Phase 2 Complete
**Features**: Library management, media scanning, streaming, watch progress, transcoding
**Status**: Phase 1 Complete ✅, Phase 2 Pending

### Feature Complete
**Target**: Phase 5 Complete
**Features**: All media types (movies/TV/music), external metadata, multi-user support
**Status**: Not Started

### Production Ready
**Target**: Phase 8 Complete
**Features**: Deployment guides, monitoring, documentation, ARM support
**Status**: Not Started

---

## Technical Context

**Architecture**: See [ARCHITECTURE.md](./ARCHITECTURE.md) for domain-driven design patterns, layer separation, and code organization.

**Tech Stack**: See [TECH_STACK.md](./TECH_STACK.md) for complete tooling and technology decisions.

**Current Stack**:
- **Backend**: Go 1.21+, Gin web framework, sqlc (type-safe SQL), dual database support (SQLite + PostgreSQL)
- **Frontend**: React 19, TypeScript, TanStack Query v5, TanStack Router, Shadcn/ui + Tailwind CSS
- **Media**: FFmpeg 6.0+, DASH streaming, range request support
- **Deployment**: Embedded frontend, single binary, Docker-ready

---

## Next Steps

### Immediate (Phase 2.2 - DASH Transcoding)
1. ✅ ~~Implement `watch_progress` table and migration~~ COMPLETE
2. ✅ ~~Implement WatchProgressRepository with dual database support~~ COMPLETE
3. ✅ ~~Create API endpoints for progress tracking~~ COMPLETE
4. ✅ ~~Add progress bars and resume functionality to frontend~~ COMPLETE
5. ⏳ Test end-to-end progress tracking (in progress)
6. Commit staged transcode infrastructure work
7. Implement TranscodeJobRepository
8. Create FFmpeg DASH transcoding service
9. Implement job queue with worker pool
10. Add transcode API endpoints and SSE support

### Short Term (Phase 2 - Transcoding)
1. Implement transcode job queue (channel-based)
2. Create DASH transcoding with FFmpeg
3. Add transcode API endpoints
4. Integrate Shaka Player for adaptive streaming
5. Test transcoding with various codecs

### Medium Term (Phase 3)
1. Implement MovieRepository (replace no-op)
2. Implement TVRepository (replace no-op)
3. Implement MusicRepository (replace no-op)
4. Create TV show and music UI pages
5. Test with real TV show and music libraries

---

**For Detailed Implementation History**: See [ROADMAP.md](./ROADMAP.md)

**Last Updated**: 2025-11-13 (Status Update: Phase 2.1 Complete, Phase 2.2 In Progress)
