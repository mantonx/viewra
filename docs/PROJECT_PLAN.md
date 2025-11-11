# ViewRA Project Plan

## Overview

This document breaks down the ViewRA media server implementation into manageable phases, milestones, and tasks. Each phase builds upon the previous one, allowing for incremental development and testing.

**Project Status**: Planning Phase  
**Start Date**: November 11, 2025  
**Target MVP**: Phase 1 Complete

---

## Development Phases

### Phase 0: Project Setup (Week 1)
**Goal**: Set up development environment and project structure

#### 0.1 Repository & Structure
- [x] Create documentation (ARCHITECTURE, API_SPECIFICATION, DATABASE_SCHEMA, TECH_STACK, PLUGIN_ARCHITECTURE)
- [x] Create FILE_NAMING.md (comprehensive file and directory naming conventions)
- [x] Create README.md (Claude-friendly project overview)
- [x] Create NOTES.md (personal development journal - replaces formal DEVELOPMENT.md)
- [x] Create .agent.md (AI assistant configuration and coding guidelines)
- [x] Document architectural patterns and decisions:
  - [x] Error handling strategy (sentinel errors + wrapped context)
  - [x] Validation approach (multi-layer: API → Domain → Infrastructure)
  - [x] Transaction patterns (batch processing for bulk operations)
  - [x] Testing strategy (in-memory SQLite + builder pattern)
  - [x] Concurrency patterns (worker pools, context cancellation)
  - [x] Security patterns (path validation, input sanitization)
  - [x] Logging standards (request tracing, log levels)
  - [x] Caching strategy (database cache, Redis plugin interface)
  - [x] Database connection pooling (SQLite WAL mode)
  - [x] API error formats and pagination defaults
- [x] Initialize Git repository
- [x] Set up .gitignore
- [x] Create project directory structure
- [x] Set up Go module (`go mod init`)
- [x] Set up frontend with Vite + React

#### 0.2 Development Tools
- [x] Install and configure Air (hot reload)
- [x] Set up sqlc configuration
- [x] Create Makefile for common tasks
- [x] Create Procfile for dev workflow
- [ ] Set up Swagger/swag configuration
- [ ] Set up Orval configuration (frontend API client)
- [ ] Set up VS Code workspace settings
- [ ] Configure linters (golangci-lint, ESLint)

#### 0.3 Database Setup
- [x] Create migrations directory structure
- [x] Create initial migration (000001_init.up.sql)
- [ ] Install golang-migrate/migrate
- [ ] Set up database connection code
- [ ] Create migration runner
- [ ] Test migrations (up/down)

**Deliverables**:
- Runnable backend skeleton (empty endpoints)
- Runnable frontend (Hello World)
- Database with core tables
- Hot reload working for both backend and frontend

---

### Phase 1: Core Foundation (Weeks 2-4)
**Goal**: Basic library and media management without transcoding

#### 1.1 Domain Layer - Libraries
- [ ] Create `Library` entity (internal/domain/library/entity.go)
- [ ] Create `LibraryRepository` interface
- [ ] Create `LibraryService` with business logic
- [ ] Add validation rules (path exists, no duplicates)
- [ ] Define domain errors

#### 1.2 Domain Layer - Media
- [ ] Create `Media` base entity (internal/domain/media/entity.go)
- [ ] Create `Movie`, `TVEpisode`, `MusicTrack` entities
- [ ] Create `MediaRepository` interface
- [ ] Create `MediaService` with business logic
- [ ] Add file validation logic

#### 1.3 Infrastructure - Database Repositories
- [ ] Implement `LibraryRepository` with sqlc
- [ ] Write SQL queries for libraries (queries/library.sql)
- [ ] Generate sqlc code
- [ ] Implement `MediaRepository` with sqlc
- [ ] Write SQL queries for media (queries/media.sql)
- [ ] Test repository implementations

#### 1.4 Infrastructure - FFmpeg Integration
- [ ] Create FFmpeg client wrapper (internal/infrastructure/ffmpeg/client.go)
- [ ] Implement metadata extraction (duration, codec, resolution)
- [ ] Implement thumbnail generation
- [ ] Add error handling for missing FFmpeg
- [ ] Test with sample video files

#### 1.5 Infrastructure - File System Scanner
- [ ] Create directory scanner (internal/infrastructure/filesystem/scanner.go)
- [ ] Implement filename parser for movies
- [ ] Implement filename parser for TV shows (S01E01 format)
- [ ] Implement filename parser for music (ID3 tags)
- [ ] Add duplicate detection (file hash)

#### 1.6 Application Layer - Use Cases
- [ ] Create library use cases (create, update, delete, list)
- [ ] Create media use cases (get, list, search, delete)
- [ ] Create scan library use case
- [ ] Add DTOs for all use cases
- [ ] Add transaction management

#### 1.7 API Layer - HTTP Handlers
- [ ] Set up Gin router (internal/interfaces/http/server.go)
- [ ] Implement library endpoints (POST, GET, PUT, DELETE /api/libraries)
- [ ] Implement media endpoints (GET, DELETE /api/media)
- [ ] Implement scan endpoint (POST /api/libraries/:id/scan)
- [ ] Add middleware (CORS, logging, recovery)
- [ ] Add Swagger annotations

#### 1.8 API Layer - Media Streaming
- [ ] Implement direct streaming endpoint (GET /api/media/:id/stream)
- [ ] Add HTTP range request support
- [ ] Add proper Content-Type headers
- [ ] Test with video player

#### 1.9 Frontend - Core UI
- [ ] Set up React Router (TanStack Router)
- [ ] Create layout component (sidebar + main content)
- [ ] Create Libraries page (list, create, delete)
- [ ] Create Media page (grid view with thumbnails)
- [ ] Create Media detail page
- [ ] Implement video player (basic HTML5 for now)

#### 1.10 Frontend - API Integration
- [ ] Generate Swagger docs (swag init)
- [ ] Generate TypeScript API client (Orval)
- [ ] Set up TanStack Query
- [ ] Implement library queries and mutations
- [ ] Implement media queries
- [ ] Add error handling and loading states

**Deliverables**:
- Working library management
- File scanning and metadata extraction
- Media browsing with thumbnails
- Direct video streaming (no transcoding yet)
- Basic UI for managing libraries and viewing media

**Test Criteria**:
- Can create a library pointing to a folder
- Scanner correctly identifies movies, TV shows, music
- FFmpeg extracts metadata (duration, codec, resolution)
- Thumbnails generate successfully
- Can browse media in a grid
- Can play videos directly (if codec is H.264/AAC)

---

### Phase 2: Watch Progress & Transcoding (Weeks 5-7)
**Goal**: Add watch progress tracking and DASH transcoding

#### 2.1 Domain Layer - Watch Progress
- [ ] Create `WatchProgress` entity
- [ ] Create `WatchProgressRepository` interface
- [ ] Create `WatchProgressService`
- [ ] Add auto-watched detection (>90% progress)

#### 2.2 Infrastructure - Watch Progress
- [ ] Create watch_progress migration (000004_add_watch_progress.up.sql)
- [ ] Implement `WatchProgressRepository` with sqlc
- [ ] Write SQL queries for progress

#### 2.3 Application Layer - Watch Progress
- [ ] Create update progress use case
- [ ] Create get progress use case
- [ ] Create mark watched/unwatched use cases

#### 2.4 API Layer - Watch Progress
- [ ] Implement progress endpoints (GET, PUT /api/progress)
- [ ] Add mark watched/unwatched endpoints
- [ ] Add Swagger docs

#### 2.5 Infrastructure - Transcoding Queue
- [ ] Create transcode job entity
- [ ] Create transcode_jobs migration
- [ ] Implement job queue (channel-based)
- [ ] Create worker pool for background transcoding

#### 2.6 Infrastructure - DASH Transcoding
- [ ] Implement DASH manifest generation
- [ ] Implement 360p transcoding (fast, low quality)
- [ ] Implement 720p transcoding (background)
- [ ] Implement 1080p transcoding (background)
- [ ] Add progress tracking for jobs
- [ ] Store manifests and segments

#### 2.7 API Layer - Transcoding
- [ ] Implement transcode request endpoint (POST /api/media/:id/transcode)
- [ ] Implement manifest endpoint (GET /api/media/:id/manifest.mpd)
- [ ] Implement transcode status endpoint (GET /api/media/:id/transcode/status)
- [ ] Add SSE endpoint for real-time progress

#### 2.8 Frontend - Watch Progress
- [ ] Add progress bar to media cards
- [ ] Add resume functionality to player
- [ ] Track playback position during viewing
- [ ] Auto-update progress every 10 seconds
- [ ] Add "Mark Watched" button

#### 2.9 Frontend - DASH Player
- [ ] Replace HTML5 player with Shaka Player
- [ ] Implement adaptive streaming
- [ ] Add quality selector UI
- [ ] Show buffering indicator
- [ ] Handle transcode requests automatically

**Deliverables**:
- Watch progress tracking and resume
- DASH transcoding for unsupported formats
- Adaptive bitrate streaming
- Background transcoding queue
- Real-time transcode progress

**Test Criteria**:
- Watch progress saves and resumes correctly
- Videos >90% watched are marked as "watched"
- Unsupported codecs trigger transcoding
- 360p version generates quickly (< 30 seconds)
- Player switches quality based on bandwidth
- Transcode progress updates in real-time

---

### Phase 3: TV Shows & Music (Weeks 8-10)
**Goal**: Full support for TV shows and music libraries

#### 3.1 Database Migrations
- [ ] Create tv_shows migration (000002_add_tv_shows.up.sql)
- [ ] Create tv_seasons migration
- [ ] Create tv_episodes migration
- [ ] Create music_tracks migration (000003_add_music.up.sql)

#### 3.2 Domain Layer - TV Shows
- [ ] Create `TVShow`, `TVSeason`, `TVEpisode` entities
- [ ] Create TV show repository interfaces
- [ ] Create TV show services
- [ ] Implement episode grouping logic

#### 3.3 Domain Layer - Music
- [ ] Create `MusicTrack` entity (extend base Media)
- [ ] Create music repository interface
- [ ] Create music service
- [ ] Add album/artist grouping logic

#### 3.4 Infrastructure - TV Scanner
- [ ] Implement TV show filename parser (multiple formats)
- [ ] Extract season/episode numbers
- [ ] Group episodes by show and season
- [ ] Handle absolute numbering (anime)
- [ ] Create show/season records automatically

#### 3.5 Infrastructure - Music Scanner
- [ ] Implement ID3 tag reader
- [ ] Extract artist, album, track number, genre
- [ ] Handle FLAC/MP3/M4A formats
- [ ] Group by artist and album

#### 3.6 API Layer - TV Shows
- [ ] Implement TV show endpoints (GET /api/tv/shows)
- [ ] Implement season endpoints (GET /api/tv/shows/:id/seasons/:season)
- [ ] Implement episode endpoints
- [ ] Add filtering and sorting

#### 3.7 API Layer - Music
- [ ] Implement artist endpoints (GET /api/music/artists)
- [ ] Implement album endpoints (GET /api/music/albums)
- [ ] Implement track endpoints (GET /api/music/tracks)
- [ ] Add grouping and filtering

#### 3.8 Frontend - TV Shows
- [ ] Create TV Shows page (show grid)
- [ ] Create Show detail page (seasons + episodes)
- [ ] Create Season view (episode list)
- [ ] Add "Next Episode" functionality
- [ ] Add season progress indicators

#### 3.9 Frontend - Music
- [ ] Create Music page (artist grid)
- [ ] Create Artist page (albums)
- [ ] Create Album page (tracks)
- [ ] Implement audio player component
- [ ] Add playlist queue (basic)

**Deliverables**:
- Full TV show support with seasons and episodes
- Music library with artist/album organization
- TV episode tracking (watched per episode)
- Audio player for music
- Proper grouping and navigation

**Test Criteria**:
- TV shows parse correctly (S01E01, 1x01 formats)
- Episodes group by show and season
- Music tracks extract ID3 tags correctly
- Albums group by artist
- Can navigate show → season → episode
- Can track progress per episode
- Audio player works for music

---

### Phase 4: Enhanced Metadata (Weeks 11-13)
**Goal**: Rich metadata from external sources

#### 4.1 Plugin System Foundation
- [ ] Create plugin registry tables (migration)
- [ ] Create plugin manager (internal/infrastructure/plugin/)
- [ ] Implement plugin loading/unloading
- [ ] Add plugin configuration storage
- [ ] Create plugin SDK interfaces

#### 4.2 TMDb Plugin (Movies & TV)
- [ ] Create TMDb metadata provider plugin
- [ ] Implement movie search
- [ ] Implement movie details fetch
- [ ] Implement TV show search
- [ ] Implement episode details
- [ ] Download and cache poster images
- [ ] Download backdrop images

#### 4.3 MusicBrainz Plugin (Music)
- [ ] Create MusicBrainz plugin
- [ ] Implement artist search
- [ ] Implement album search
- [ ] Implement track matching
- [ ] Fetch cover art

#### 4.4 Database Migrations - Rich Metadata
- [ ] Create metadata_cache migration
- [ ] Create people migration (cast/crew)
- [ ] Create movie_credits migration
- [ ] Create episode_credits migration
- [ ] Create collections migration
- [ ] Create images migration

#### 4.5 Domain Layer - People & Credits
- [ ] Create `Person` entity
- [ ] Create credits entities (movie/episode/music)
- [ ] Create repository interfaces
- [ ] Create services for cast/crew management

#### 4.6 Application Layer - Metadata Enrichment
- [ ] Create metadata enrichment use case
- [ ] Implement auto-matching logic
- [ ] Add manual match/override support
- [ ] Implement image download and storage
- [ ] Add metadata refresh functionality

#### 4.7 API Layer - Metadata
- [ ] Add metadata search endpoint (POST /api/metadata/search)
- [ ] Add apply metadata endpoint (POST /api/media/:id/metadata)
- [ ] Add refresh metadata endpoint
- [ ] Add cast/crew endpoints

#### 4.8 Frontend - Metadata UI
- [ ] Add "Identify" button to media detail page
- [ ] Create metadata search modal
- [ ] Display search results with posters
- [ ] Show match confidence scores
- [ ] Display cast and crew on detail page
- [ ] Show collections

**Deliverables**:
- External metadata integration (TMDb, MusicBrainz)
- Rich metadata (cast, crew, plot, ratings)
- High-quality poster/backdrop images
- Collections support (MCU, Star Wars, etc.)
- Manual metadata matching UI

**Test Criteria**:
- TMDb correctly identifies movies by title + year
- TV shows match with high confidence
- Posters and backdrops download successfully
- Cast and crew display correctly
- Collections group related movies
- Can manually override incorrect matches

---

### Phase 5: User Features & Polish (Weeks 14-16)
**Goal**: User experience improvements and multi-user support

#### 5.1 Database Migrations - User Features
- [ ] Create users migration
- [ ] Create watch_history migration
- [ ] Create ratings migration
- [ ] Create tags migration
- [ ] Create playlists migration

#### 5.2 Authentication & Users
- [ ] Create `User` entity
- [ ] Implement JWT authentication
- [ ] Add password hashing (bcrypt)
- [ ] Create login/logout endpoints
- [ ] Add authentication middleware
- [ ] Implement user registration

#### 5.3 Multi-User Support
- [ ] Update watch progress to be per-user
- [ ] Update watch history to be per-user
- [ ] Add user-specific ratings
- [ ] Add user-specific tags
- [ ] Implement user settings/preferences

#### 5.4 User Ratings & Reviews
- [ ] Create ratings UI (star rating component)
- [ ] Implement rating submission
- [ ] Display user rating vs external ratings
- [ ] Add sorting by rating

#### 5.5 Tagging System
- [ ] Create tag management UI
- [ ] Implement tag creation
- [ ] Add tag assignment to media
- [ ] Add filtering by tags
- [ ] Show tag clouds

#### 5.6 Playlists
- [ ] Create playlist management UI
- [ ] Implement playlist creation
- [ ] Add media to playlists
- [ ] Reorder playlist items
- [ ] Play entire playlist

#### 5.7 Search & Filtering
- [ ] Implement full-text search across media
- [ ] Add advanced filtering (genre, year, rating)
- [ ] Create search UI with autocomplete
- [ ] Add filter sidebar
- [ ] Implement sorting options

#### 5.8 Dashboard & Stats
- [ ] Create home dashboard
- [ ] Show recently added media
- [ ] Show continue watching
- [ ] Show watch statistics
- [ ] Display library statistics

#### 5.9 Settings & Configuration
- [ ] Create settings page
- [ ] Add library management settings
- [ ] Add transcode quality settings
- [ ] Add theme preferences
- [ ] Add language preferences

**Deliverables**:
- Multi-user support with authentication
- User ratings and reviews
- Tagging and playlist features
- Advanced search and filtering
- Dashboard with personalized recommendations
- Settings UI

**Test Criteria**:
- Multiple users can have separate watch progress
- JWT authentication works correctly
- Ratings save per user
- Tags help organize media
- Playlists play in order
- Search finds media quickly
- Dashboard shows relevant content

---

### Phase 6: Advanced Features (Weeks 17-20)
**Goal**: Advanced playback features and optimizations

#### 6.1 Playback Features
- [ ] Create chapters migration
- [ ] Create playback_markers migration
- [ ] Implement intro/outro detection (placeholder)
- [ ] Add "Skip Intro" button
- [ ] Add "Next Episode" auto-play
- [ ] Implement recap detection

#### 6.2 Subtitles & Audio Tracks
- [ ] Create subtitle_tracks migration
- [ ] Create audio_tracks migration
- [ ] Create external_subtitles migration
- [ ] Implement subtitle extraction from MKV
- [ ] Add subtitle selection in player
- [ ] Add audio track selection in player
- [ ] Support external .srt files

#### 6.3 File System Watching
- [ ] Implement fsnotify integration
- [ ] Add real-time file addition detection
- [ ] Add file deletion detection
- [ ] Auto-scan on file changes
- [ ] Add notification for new media

#### 6.4 Performance Optimizations
- [ ] Add database connection pooling
- [ ] Implement query result caching
- [ ] Add thumbnail lazy loading
- [ ] Optimize large library queries
- [ ] Add database indexes for common queries
- [ ] Implement pagination for all lists

#### 6.5 Mobile Responsiveness
- [ ] Optimize UI for tablets
- [ ] Optimize UI for phones
- [ ] Add touch gestures for player
- [ ] Implement responsive grid layouts
- [ ] Test on various devices

#### 6.6 Notifications
- [ ] Create notification system
- [ ] Add toast notifications for actions
- [ ] Add scan completion notifications
- [ ] Add transcode completion notifications
- [ ] Add new media notifications

**Deliverables**:
- Skip intro/outro functionality
- Multi-audio and subtitle support
- Real-time library updates (fsnotify)
- Performance optimizations for large libraries
- Mobile-friendly interface
- Notification system

**Test Criteria**:
- Intro markers detect correctly
- Subtitles display and sync properly
- Audio tracks switch smoothly
- New files auto-add to library
- UI works well on mobile devices
- Notifications appear for important events

---

### Phase 7: Plugin Ecosystem (Weeks 21-24)
**Goal**: Robust plugin system and community plugins

#### 7.1 Plugin SDK
- [ ] Create Go SDK package
- [ ] Create Python SDK (optional)
- [ ] Write plugin development guide
- [ ] Create plugin template repository
- [ ] Add plugin testing utilities

#### 7.2 Official Plugins
- [ ] TMDb metadata provider (refactor existing)
- [ ] TheTVDB plugin
- [ ] MusicBrainz plugin (refactor existing)
- [ ] Discord notifier plugin
- [ ] Email notifier plugin

#### 7.3 Plugin Management UI
- [ ] Create plugins page
- [ ] Show installed plugins
- [ ] Add plugin enable/disable toggle
- [ ] Show plugin configuration UI
- [ ] Add plugin installation from file
- [ ] Show plugin health status

#### 7.4 Plugin Security
- [ ] Implement permission system
- [ ] Add plugin signature verification
- [ ] Create sandboxing for plugins
- [ ] Add resource limits (CPU, memory)
- [ ] Implement circuit breakers

#### 7.5 Plugin Marketplace (Optional)
- [ ] Create plugin repository schema
- [ ] Build plugin discovery API
- [ ] Create marketplace UI
- [ ] Add plugin ratings and reviews
- [ ] Implement automatic updates

**Deliverables**:
- Stable plugin SDK
- Multiple official plugins
- Plugin management interface
- Security and sandboxing
- Plugin documentation and examples

**Test Criteria**:
- Can develop new plugin with SDK
- Plugins can be installed/uninstalled
- Plugins have proper permissions
- Resource limits prevent abuse
- Plugin failures don't crash server

---

### Phase 8: Deployment & Production (Weeks 25-26)
**Goal**: Production-ready deployment

#### 8.1 Docker Setup
- [ ] Create Dockerfile for backend
- [ ] Create docker-compose.yml
- [ ] Add volume mounts for data
- [ ] Configure environment variables
- [ ] Test Docker deployment

#### 8.2 Documentation
- [ ] Complete README.md
- [ ] Complete DEVELOPMENT.md
- [ ] Create DEPLOYMENT.md
- [ ] Create USER_GUIDE.md
- [ ] Create FILE_NAMING.md
- [ ] Add CONTRIBUTING.md
- [x] Add LICENSE file (MIT License)

#### 8.3 Production Optimizations
- [ ] Set up proper logging (JSON format)
- [ ] Add health check endpoints
- [ ] Implement graceful shutdown
- [ ] Add metrics collection (optional)
- [ ] Configure CORS for production
- [ ] Enable HTTPS support

#### 8.4 Testing
- [ ] Write unit tests (target 70% coverage)
- [ ] Write integration tests
- [ ] Create E2E test suite
- [ ] Test migration rollbacks
- [ ] Load testing with large libraries

#### 8.5 CI/CD
- [ ] Set up GitHub Actions
- [ ] Add automated tests
- [ ] Add linting checks
- [ ] Create release workflow
- [ ] Automate Docker image builds

**Deliverables**:
- Docker images for easy deployment
- Complete documentation
- Comprehensive test suite
- CI/CD pipeline
- Production-ready application

**Test Criteria**:
- Docker container runs successfully
- Documentation is clear and complete
- Tests pass in CI
- Application handles errors gracefully
- Performance is acceptable with 10,000+ media items

---

## Milestones

### Milestone 1: MVP (End of Phase 2)
- ✅ Libraries and media management
- ✅ Direct streaming
- ✅ Basic transcoding
- ✅ Watch progress tracking
- ✅ Basic web UI

### Milestone 2: Feature Complete (End of Phase 5)
- ✅ TV shows and music support
- ✅ External metadata integration
- ✅ Multi-user support
- ✅ Search and filtering
- ✅ Dashboard and stats

### Milestone 3: Production Ready (End of Phase 8)
- ✅ Plugin system
- ✅ Advanced features (subtitles, chapters)
- ✅ Performance optimizations
- ✅ Complete documentation
- ✅ Docker deployment

---

## Technical Debt & Future Enhancements

### Known Limitations
- Initial single-user only (multi-user in Phase 5)
- Basic filename parsing (no complex patterns initially)
- SQLite only (PostgreSQL support later)
- No mobile apps (web-only)
- No DVR/Live TV support
- No hardware transcoding initially

### Future Considerations
- Mobile apps (React Native or native)
- Hardware-accelerated transcoding (NVENC, QuickSync)
- PostgreSQL support for large deployments
- CDN integration for remote access
- Live TV and DVR functionality
- Download for offline viewing
- Casting support (Chromecast, AirPlay)
- Social features (watch parties, sharing)

---

## Resource Requirements

### Development Environment
- Go 1.21+
- Node.js 18+
- FFmpeg 6.0+
- SQLite 3.35+
- 8GB RAM (development)
- 20GB disk space (for test media)

### Production Requirements (Minimum)
- 2 CPU cores
- 2GB RAM
- 50GB disk (OS + app + thumbnails)
- Storage for media (separate mount recommended)

### Production Requirements (Recommended)
- 4+ CPU cores (for transcoding)
- 8GB RAM
- 100GB disk (for transcoded files)
- NAS or external storage for media
- Reverse proxy (Caddy/Nginx) for HTTPS

---

## Risk Assessment

### High Risk
- **FFmpeg dependency**: Different versions/builds may behave differently
  - Mitigation: Document required FFmpeg version, test thoroughly
  
- **Transcoding performance**: CPU-intensive, may overwhelm server
  - Mitigation: Queue system, background workers, resource limits

### Medium Risk
- **Database performance**: Large libraries may slow queries
  - Mitigation: Proper indexes, pagination, consider PostgreSQL
  
- **File format compatibility**: Some codecs may not work
  - Mitigation: Comprehensive format testing, clear documentation

### Low Risk
- **Plugin security**: Malicious plugins could compromise system
  - Mitigation: Permission system, sandboxing, signature verification

---

## Success Metrics

### Phase 1 Success
- Can scan and play videos
- Thumbnails generate correctly
- UI is responsive and intuitive

### Phase 2 Success
- Transcoding works reliably
- Watch progress syncs accurately
- DASH streaming is smooth

### MVP Success (End of Phase 2)
- Can manage libraries
- Can scan and identify media
- Can stream media (direct or transcoded)
- Can track watch progress
- UI is usable and attractive

### Final Success (End of Phase 8)
- Handles 10,000+ media items
- Supports movies, TV, music
- Multi-user with authentication
- Plugin system working
- Documented and deployable
- Community adoption and feedback

---

## Next Steps

1. **Immediate**: Complete Phase 0 (Project Setup)
   - Create README.md
   - Create DEVELOPMENT.md
   - Set up project structure
   - Initialize database

2. **Week 1**: Start Phase 1.1 (Domain Layer - Libraries)
   - Create entity definitions
   - Define repository interfaces
   - Write initial tests

3. **Week 2**: Continue Phase 1 (Infrastructure & API)
   - Implement repositories
   - Create HTTP endpoints
   - Build basic frontend

---

## Questions & Decisions Needed

### Technical Decisions Made ✅

All technical decisions finalized on November 11, 2025:

1. **Development Environment**: Setup script + Makefile (Option D)
   - `scripts/setup.sh` validates environment
   - `Makefile` for daily commands
   - Documented in TECH_STACK.md

2. **Configuration Management**: Environment variables + future admin UI (Option A+)
   - `.env` file for development
   - Admin settings page in Phase 5+ writes to database
   - Env vars always override database settings

3. **Static File Serving**: Go serves from `data/` as WebP (Option B+)
   - All images converted to WebP
   - Served with caching headers
   - Optional Nginx/Caddy reverse proxy

4. **FFmpeg Detection**: Runtime detection with graceful degradation (Option C)
   - App starts without FFmpeg
   - Transcoding features disabled in UI if missing
   - Minimum version: FFmpeg 5.0+

5. **File Hashing**: Partial hash - first 64KB + last 64KB (Option B)
   - Fast duplicate detection
   - Same strategy as Plex/Jellyfin
   - Instant vs 30+ seconds for full hash

6. **Filename Parsing**: Pattern library with NFO support (Option C+)
   - Check `.nfo` files first (Kodi/Plex format)
   - Regex patterns for common formats
   - Graceful fallback to basic parsing

7. **Scan Conflict Resolution**: Smart resolution (Option C)
   - Auto-update moved files (same hash)
   - Mark missing files as unavailable
   - Flag duplicates for user review

8. **Thumbnail Generation**: Background queue (Option B)
   - Fast scan, async thumbnail generation
   - Priority queue (recently added first)
   - Placeholder until ready

9. **Database Migrations**: Auto-run with version check (Option B)
   - Only run if database < code version
   - `AUTO_MIGRATE=false` override for production
   - Automatic backup before migrations

10. **API Versioning**: No versioning until 1.0 (Option C+)
    - Single `/api/` namespace pre-1.0
    - Structured handlers ready for `/api/v1/` later
    - Additive changes only

11. **Frontend Build**: Separate dev, embedded production (Option B)
    - Dev: Vite dev server + CORS
    - Production: Embedded in Go binary via `embed.FS`
    - Single binary deployment

12. **Transcode Cleanup**: LRU cache with disk limit (Option C)
    - Default 50GB limit for DASH segments
    - Delete oldest accessed when limit reached
    - Keep job records, clean files only

13. **Watch Progress Sync**: Client batching, 10s sync (Option E)
    - Track locally every 1s (UI updates)
    - Send to backend every 10s
    - Immediate sync on pause/stop/seek

14. **Metadata Rate Limiting**: Respect provider headers + queue (Option D)
    - Background queue for metadata requests
    - Honor `Retry-After` and `X-RateLimit-*` headers
    - Adapts to provider changes

15. **Plugin Protocol**: HTTP/JSON (Option B)
    - Simple REST API with JSON payloads
    - Easy to develop in any language
    - Future gRPC support for performance (post-1.0)

16. **Frontend Error Handling**: Toast + inline + boundaries (Option C)
    - Inline validation errors in forms
    - Toast for system errors/actions
    - Error boundaries for crashes

17. **Hot Reload**: Procfile with overmind/foreman (Option D)
    - Single command runs backend + frontend
    - Clean separate logs
    - Easy service restart

18. **Test Data**: Script downloads samples (Option C)
    - `scripts/download-test-media.sh`
    - Public domain clips (Big Buck Bunny, Sintel)
    - Stored in `test-data/` (gitignored)

19. **Logging Format**: Pretty dev, JSON prod, Prometheus ready (Option B+)
    - Human-readable development logs
    - JSON production logs to stdout
    - Prometheus `/metrics` endpoint (Phase 6+)

20. **Database Backups**: Auto daily + before migrations (Option B)
    - Daily at 3am (configurable)
    - Keep last 7 days
    - Automatic pre-migration backups

21. **Directory Structure**: Phase-appropriate (Option C)
    - Create Phase 0-1 directories initially
    - Add Phase 3+ directories when implementing features
    - Grows with project

All decisions documented across:
- `docs/TECH_STACK.md` - Tooling and technical decisions
- `docs/ARCHITECTURE.md` - Implementation patterns
- `docs/API_SPECIFICATION.md` - API behavior
- `docs/DATABASE_SCHEMA.md` - Data management
- `docs/PLUGIN_ARCHITECTURE.md` - Plugin system
- `.agent.md` - AI assistant guidance

---

### Technical Decisions (Legacy - Now Resolved Above)
- [x] Use PostgreSQL from start or SQLite first? → **SQLite first, PostgreSQL Phase 8**
- [x] Server-Sent Events or WebSockets for real-time? → **SSE (simpler)**
- [x] Embed frontend in binary or separate? → **Embedded for production**
- [x] Support Windows/Mac or Linux-only? → **All platforms**

### Feature Decisions (Legacy - Now Resolved Above)
- [x] Support 4K/8K transcoding? → **Phase 6+ based on demand**
- [x] Include DVR/Live TV? → **Future enhancement**
- [x] Mobile apps or web-only? → **Web-only initially**
- [x] Plugin marketplace or manual install? → **Manual initially**

### Scope Decisions (Legacy - Now Resolved Above)
- [x] How many media items should we target? → **10,000+ items**
- [x] How many simultaneous streams? → **5-10 streams**
- [x] How many concurrent transcodes? → **2-3 concurrent jobs**

---

**Last Updated**: November 11, 2025  
**Status**: Phase 0.1 Complete - Phase 0.2 Development Tools In Progress - Phase 0.3 Database Setup Started
