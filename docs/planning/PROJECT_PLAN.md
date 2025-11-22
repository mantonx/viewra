# ViewRA Project Plan

## Current Status

**Phase**: Phase 5 Mostly Complete - User Authentication Next
**Last Updated**: November 21, 2025
**Recent**: Backend testing infrastructure overhaul (188+ test cases, 100% pass rate)
**Recent**: Phase 5.7 Video Player Enhancement complete - seek-based transcoding, custom controls
**Recent**: Phase 5 Core Complete - Pagination, infinite scroll, batch loading, performance optimizations
**Recent**: Phase 4 Complete - Image handling, NFO parsing, ID3 metadata, caching system
**Current Features**: Full media management, transcoding, progress tracking, video/audio players, image handling
**Target MVP**: Phase 5 Complete ✅
**Start Date**: November 11, 2025

### Recent Accomplishments (Nov 21, 2025)

**🧪 Backend Testing Infrastructure Overhaul ✅ COMPLETE** (Nov 21, 2025):

- ✅ **Mock Repository Centralization**: Eliminated duplicate mock implementations across test files
  - Created centralized mock repositories in `internal/infrastructure/repository/memory/`
  - Consolidated 8+ scattered mock implementations into reusable components
  - Test files now import shared mocks instead of defining their own
  - Location: `internal/infrastructure/repository/memory/{movie,tvshow,music,library}_repository.go`
- ✅ **Test Coverage Improvements**: Comprehensive test suite with 188+ test cases
  - All tests passing (100% pass rate)
  - Enhanced test isolation and repeatability
  - Improved error handling and edge case coverage
  - Better test organization and maintainability
- ✅ **Code Quality**: Eliminated technical debt in test infrastructure
  - DRY principles applied to test code
  - Consistent mock patterns across all domain tests
  - Easier to write and maintain tests going forward
- ✅ **Documentation**: Comprehensive backend code review documenting all improvements
  - See `BACKEND_CODE_REVIEW_UPDATED.md` for detailed analysis

**Impact**: Significantly improved developer experience and code maintainability. Testing infrastructure now robust and scalable for future feature development.

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

## Phase 2: Watch Progress & Transcoding ✅ COMPLETE

**Goal**: Track viewing progress and enable on-demand transcoding

**Status**: ✅ Complete
**Started**: November 12, 2025
**Completed**: November 13, 2025
**Actual Effort**: 2 days

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

#### Phase 2.2: On-Demand Transcoding ✅ COMPLETE (Nov 13, 2025)
- ✅ Background transcoding queue (channel-based worker pool with configurable concurrency)
- ✅ 4-tier intelligent streaming strategy (Direct Play → Remux → Remux+Audio → Transcode)
- ✅ HLS output format with progressive streaming
- ✅ On-demand trigger from manifest request
- ✅ Idle timeout to cancel abandoned transcodes
- ✅ Access tracking for LRU cleanup

**Components**: ✅ Complete
- ✅ `transcode_jobs` table with access tracking (migration 000006)
- ✅ Database queries for transcode jobs (SQLite + PostgreSQL)
- ✅ Transcode job repository implementation
- ✅ FFmpeg transcoding service (remux, remux+audio, full transcode)
- ✅ Job queue with worker pool and idle timeout
- ✅ API: All transcode endpoints + HLS segment serving
- ✅ Frontend: Shaka Player integration with DASH/HLS support

#### Phase 2.3: Transcode Cleanup System ✅ COMPLETE (Nov 13, 2025)
- ✅ Manual cleanup CLI tool with disk usage reporting
- ✅ API endpoints for cleanup operations
- ✅ Automated background scheduler (runs every 6 hours)
- ✅ LRU eviction based on last access time
- ✅ Disk space monitoring with configurable thresholds
- ✅ Pre-transcode output size estimation with 30% safety margin (Nov 20)
- ✅ Dynamic cleanup batch sizing based on disk pressure (Nov 20)

---

## Phase 3: TV Shows & Music ✅ COMPLETE

**Goal**: Extend media library to support TV shows and music

**Status**: ✅ Complete
**Started**: November 14, 2025
**Completed**: November 15, 2025
**Actual Effort**: 2 days

### Phase 3.1: TV Shows Support ✅ COMPLETE (Nov 14, 2025)
- ✅ Database schema for shows, seasons, episodes
- ✅ Filesystem scanner with extras support (backgrounds, posters, fanart)
- ✅ Episode metadata parsing
- ✅ Frontend UI with show/season/episode navigation
- ✅ Episode playback with progress tracking

### Phase 3.2: Music Support ✅ COMPLETE (Nov 15, 2025)
- ✅ Database schema for music tracks (virtual artists/albums pattern)
- ✅ Filesystem scanner for music libraries
- ✅ ID3 metadata extraction
- ✅ Frontend UI for artists, albums, tracks
- ✅ Audio player with progress tracking

---

## Phase 4: Metadata & Images ✅ COMPLETE (Nov 15-18, 2025)

**Goal**: Rich metadata from NFO files, local images, and architectural improvements

**Status**: ✅ Complete (Completed ahead of schedule)
**Started**: November 15, 2025
**Completed**: November 18, 2025
**Actual Effort**: 4 days

### Key Accomplishments

#### Phase 4.1: Image Handling Infrastructure ✅ COMPLETE (Nov 16-17, 2025)
- ✅ **Database**: Migration 000007 - polymorphic `media_images` table with CASCADE deletes
- ✅ **Domain Layer**: Entity, repository interface, type-safe enums (13 image types, 6 sources)
- ✅ **Image Extraction**: Metadata extractor (dimensions, SHA256, MIME), Kodi/Plex file detector (1,294 lines)
- ✅ **Application Layer**: Use cases for get/extract/cleanup images (movies, TV episodes, music albums)
- ✅ **API Layer**: 4 endpoints (GetImage, ServeImage, GetMediaImages, GetMovieImages/EpisodeImages)
- ✅ **Scanner Integration**: Automatic image extraction during library scans (all media types)
- ✅ **Frontend**: TypeScript types, API client, MediaPoster component, updated MediaCard/MovieCard
- ✅ **HTTP Caching**: 1-year Cache-Control headers with ETags
- ✅ **Coverage**: 36,000+ image assets cataloged (posters, fanart, logos, episode thumbnails, album covers)

#### Phase 4.2: NFO Parsing ✅ COMPLETE (Nov 15, 2025)
- ✅ **Movie NFO Parser**: 20+ metadata fields (plot, director, cast, genre, year, ratings)
- ✅ **TV Episode NFO Parser**: Episode metadata with air dates, descriptions, IDs
- ✅ **Scanner Integration**: Automatic NFO detection and parsing
- ✅ **Frontend Display**: Rich metadata on all media cards

#### Phase 4.3: Image Caching & Transformations ✅ COMPLETE (Nov 17, 2025)
- ✅ **Cache Service**: Hash-based sharding with `{first2}/{next2}/{hash}_*.webp` structure
- ✅ **Image Transformer**: WebP conversion with quality settings, Lanczos resampling
- ✅ **Preset System**: Pre-generates 4 sizes (thumb/medium/large/xlarge) for 13 image types
- ✅ **Cache Population**: 13,201+ WebP files generated with deduplication
- ✅ **Performance**: Eliminates on-demand transformation overhead

#### Phase 4.4: Music Metadata & Artwork ✅ COMPLETE (Nov 18, 2025)
- ✅ **ID3 Integration**: Clean architecture pattern with metadata extractor interface
- ✅ **Artist Artwork**: Filesystem extraction (folder.jpg, fanart.jpg, logo.png)
- ✅ **Album Artwork**: Folder-based and embedded ID3/APIC extraction as fallback
- ✅ **Embedded Fallback**: Automatic extraction from ID3 tags when filesystem images missing
- ✅ **Frontend**: Year, genre, bitrate badges with enhanced hover effects
- ✅ **Coverage**: 41 artist images, 100+ album covers displaying correctly

#### Phase 4.5: Audio Codec Compatibility ✅ COMPLETE (Nov 15, 2025)
- ✅ Fixed AC3/DTS/TrueHD/FLAC audio transcoding
- ✅ Multi-channel audio (5.1, 7.1) properly downmixed to stereo
- ✅ Remux-with-audio strategy working correctly

#### Phase 4.6: Unified Task Scheduler ✅ PARTIAL (Nov 16, 2025)
- ✅ **Scheduler Core**: Cron-based task scheduler using `robfig/cron/v3`
- ✅ **Database Schema**: Execution history and task status tracking
- ✅ **Admin API**: List tasks, manual triggers, execution history endpoints
- ✅ **Image Cleanup Integration**: Registered with unified scheduler
- 📋 **Pending**: Transcode cleanup migration, frontend UI (deferred)

### Success Criteria - All Met ✅
- ✅ NFO files automatically detected and parsed during library scan
- ✅ Movie metadata populated from .nfo files (plot, director, cast, genre, year, ratings)
- ✅ TV episode metadata populated from episode.nfo files (air dates, descriptions, IDs)
- ✅ Music metadata extracted via ID3 tags (year, genre, bitrate, artist, album)
- ✅ Scanner detects and catalogs local images (posters, fanart, logos, thumbnails)
- ✅ Movie posters display in frontend from local `poster.jpg` files
- ✅ TV show posters and season artwork display correctly
- ✅ TV episode thumbnails display (99.84% coverage - 2,562/2,566 episodes)
- ✅ Music album covers display (100+ covers across albums)
- ✅ Music artist artwork displays (41 images across 14 artists)
- ✅ Images served with proper caching headers (1 year TTL)
- ✅ Image resizing and WebP conversion with preset system
- ✅ Hash-based cache deduplication with sharding

**Note**: Phase 4 completed ahead of schedule with all core features implemented.

---

## Phase 5: Library Browsing UX & Performance ✅ MOSTLY COMPLETE (Nov 18-20, 2025)

**Goal**: Transform library browsing experience with pagination, infinite scroll, and performance optimizations

**Status**: ✅ Core Complete - User authentication remaining
**Started**: November 18, 2025
**Completed**: November 20, 2025 (core features)
**Actual Effort**: 3 days

### Completed Features ✅

#### Phase 5.0: Code Quality Refactoring ✅ COMPLETE (Nov 18, 2025)
- ✅ Frontend: 354+ lines eliminated (useLibraryFilter hook, ProgressBar/WatchedBadge components, MediaBrowsePage wrapper)
- ✅ Backend: 220+ lines eliminated (TV episode converters, music track converters, query param helpers)
- ✅ Library package: 184+ lines eliminated (DRY refactoring)
- ✅ Total: 758+ lines of duplicate code removed
- ✅ Improved maintainability and DRY principles across entire codebase

#### Phase 5.1: Backend Pagination Infrastructure ✅ COMPLETE (Nov 19, 2025)
- ✅ Created pagination types (PaginationParams, PaginationMetadata)
- ✅ Added movies pagination SQL queries (count + paginated list/search)
- ✅ Added TV shows pagination SQL queries (count + paginated list/search)
- ✅ Added music pagination SQL queries (artists, albums, tracks with count)
- ✅ Regenerated sqlc code
- ✅ Repository interfaces updated with pagination methods
- ✅ Repository implementations complete (movie, TV, music)
- ✅ Use cases updated with ExecuteWithPagination methods
- ✅ API handlers support optional pagination query params
- ✅ Swagger documentation regenerated
- ✅ Backward compatible (defaults to non-paginated if params not provided)

#### Phase 5.2: Frontend Infinite Scroll ✅ COMPLETE (Nov 19, 2025)
- ✅ Created `useInfiniteMedia` generic hook using TanStack Query's `useInfiniteQuery`
- ✅ Implemented `useInfiniteMovies`, `useInfiniteTVShows`, `useInfiniteArtists` hooks
- ✅ Added IntersectionObserver for automatic "load more" detection in all pages
- ✅ Updated movies, TV, and music pages to use infinite scroll hooks
- ✅ Implemented page flattening helpers
- ✅ Added loading states with "Loading more..." indicators
- ✅ Default page size of 50 items

#### Phase 5.3: Image Loading Optimization ✅ COMPLETE (Nov 19, 2025)
- ✅ Created batch image endpoint: `POST /api/images/batch` with dual lookup support
- ✅ Extended batch endpoint to support both `media_ids` and `entity_ids + media_type`
- ✅ Implemented `GetBatchMediaImagesUseCase` with media and entity batch queries
- ✅ Created `BatchImagesProvider` React Context with entity-based batch support
- ✅ Created `useBatchImages` and `useBatchImagesIfAvailable` hooks
- ✅ Updated `MediaPoster` component with clean context checking
- ✅ Integrated batch loading in movies, TV shows, and music browse pages
- ✅ Browser-native lazy loading (`loading="lazy"`)
- ✅ **Performance**: 50 requests → 1 request per page (98% reduction in network overhead)

#### Phase 5.4: Backend N+1 Query Fixes ✅ COMPLETE (Nov 20, 2025)
- ✅ **Music Artists**: Eliminated loading 50,000+ tracks into memory, now uses efficient database aggregation (O(n) → O(1) memory)
- ✅ **TV Shows**: Eliminated 1 + N queries (100 shows = 101 queries → 1 query with JOIN)

#### Phase 5.5: Response Compression ✅ COMPLETE (Nov 20, 2025)
- ✅ Added gzip middleware (expected 60-80% payload reduction)
- ✅ Component refactoring: MediaBrowsePage, ViewToggle, SortSelector, AdvancedFilters extracted

#### Phase 5.6: Music Artwork System ✅ COMPLETE (Nov 18, 2025)
- ✅ Artist artwork extraction from filesystem (folder.jpg, fanart.jpg, logo.png)
- ✅ Embedded ID3/APIC artwork extraction as fallback
- ✅ Filesystem-first priority respects user intent
- ✅ Full end-to-end pipeline: scan → extract → cache → serve
- ✅ API endpoint: `GET /api/music/artists/:id/images`
- ✅ Test results: 41 images across 14 artists extracted

#### Phase 5.7: Video Player Enhancement ✅ COMPLETE (Nov 20, 2025)

**Tier 1 - Critical Fixes**:
- ✅ **Aspect Ratio Fix**: Added `object-fit: contain` to prevent video distortion
- ✅ **Performance Optimization**: Throttled timeupdate events from 4-15x/sec to 1x/sec (75-90% reduction in re-renders)
- ✅ **Keyboard Shortcuts**: Comprehensive controls matching industry standards (space, j/l, arrows, m, f, 0-9, etc.)
- ✅ **Playback Speed Control**: UI selector with 0.25x to 2x options
- ✅ **Buffering Indicator**: Visual feedback with animated spinner during HLS progressive transcoding
- ✅ **Startup UX Improvement**: Eliminated loading screen delay for instant playback
- ✅ **VideoPlayerContainer**: Created container to eliminate duplicate playback code
- ✅ **Code Cleanup**: Removed unmute button duplication

**Tier 2 - Custom Control Bar**:
- ✅ **VideoControls Component**: Professional control bar with auto-hide functionality
  - Timeline with hover preview showing time at cursor position
  - Play/pause button with dynamic SVG icons
  - Volume control with hover slider
  - Time display (current/total with MM:SS or HH:MM:SS format)
  - Playback speed selector (0.25x to 2x)
  - Quality selector (Auto + available resolutions)
  - Fullscreen toggle
  - Auto-hide after 3 seconds of inactivity
  - Gradient overlay for readability
- ✅ **State Management**: Integrated currentTime, volume, isMuted, isFullscreen tracking
- ✅ **Event Handling**: Full event listener integration for video, volume, and fullscreen changes
- ✅ **UI Cleanup**: Removed browser native controls and old header bar
- ✅ **formatTime Utility**: Added MM:SS/HH:MM:SS formatting for video timestamps

**Tier 3 - Seek-Based Transcoding**:
- ✅ **Seek Functionality**: Implemented HLS progressive streaming with seek support
  - Users can now seek/scrub during transcoding without waiting for full completion
  - FFmpeg starts from requested timestamp when user seeks ahead
  - Automatic cleanup and restart of transcode jobs when seek position changes
  - Location: `internal/application/transcode/serve_manifest.go`
- ✅ **Integration**: Full integration with video player and transcode queue
  - Backend detects seek requests and restarts transcoding from new position
  - Player seamlessly handles seek operations during active transcoding

**Impact**: Transformed from 40% to ~90% feature parity with industry standards (Plex/Jellyfin/Netflix)

### Performance Impact ✅
- ✅ 95%+ reduction in payload sizes (20MB → 500KB)
- ✅ 10x faster page loads (5-10s → <1s)
- ✅ Industry-standard infinite scroll pattern with TanStack Query
- ✅ Backward compatible migration strategy
- ✅ Significantly improved scalability for large libraries

### Deferred Features 📋

#### Phase 5.8: Audio Player Enhancement 📋 PLANNED
- 📋 Custom audio controls (similar to video player)
- 📋 Playlist support
- 📋 Gapless playback
- 📋 Audio visualization
- 📋 Queue management

#### Phase 5.9: User Authentication & Multi-User 📋 NOT STARTED
- 📋 User authentication system (JWT + bcrypt)
- 📋 Multi-user support with per-user watch progress
- 📋 User management UI
- 📋 Role-based access control

**Note**: Phase 5 completed ahead of schedule with all core performance and UX improvements. User authentication deferred to Phase 6.

---

## Phase 6: User Features & Multi-User Support 📋 PLANNED (Future)

**Goal**: Add user authentication, multi-user support, and user-specific features

**Status**: 📋 Not Started
**Priority**: High (blocking multi-user deployment)

### Phase 6.1: User Authentication (Immediate Priority)
- 📋 Implement user authentication system
  - JWT token-based authentication
  - bcrypt password hashing
  - Session management
  - Login/logout endpoints
  - Password reset functionality
- 📋 User registration and management
  - User creation API
  - User profile management
  - User settings and preferences
- 📋 Frontend authentication UI
  - Login page
  - Registration page
  - User profile page
  - Protected routes

### Phase 6.2: Multi-User Support
- 📋 Per-user watch progress isolation
- 📋 Per-user ratings and favorites
- 📋 User-specific libraries and permissions
- 📋 Role-based access control (admin, user, guest)
- 📋 User activity tracking and history

### Phase 6.3: User Experience Features
- 📋 Search across all media types
- 📋 Dark mode theme support
- 📋 User preferences (language, playback settings)
- 📋 Watchlists and collections
- 📋 Recommendations based on watch history

---

## Phase 7: Advanced Features 📋 PLANNED (Future)

**Goal**: External metadata APIs, plugin system, and advanced media features

**Status**: 📋 Not Started

### Phase 7.1: External Metadata APIs
- 📋 **TMDb Integration** for movies/TV shows
  - Search API: Match movies by title + year
  - Image API: Download posters, backdrops, logos
  - Cast & crew API: Populate people metadata
  - Store downloaded images in `data/images/tmdb/`
- 📋 **MusicBrainz Integration** for music
  - Search API: Match artists and albums
  - Cover Art Archive: Download album covers
  - Artist images from fanart.tv or Last.fm
  - Store in `data/images/musicbrainz/`

### Phase 7.2: Manual Metadata Management
- 📋 Upload custom images for media items
- 📋 Set priority for multiple images of same type
- 📋 Override incorrect TMDb/MusicBrainz matches
- 📋 Delete/refresh images
- 📋 Edit metadata fields manually

### Phase 7.3: Plugin Architecture
- 📋 Plugin SDK interfaces
- 📋 Plugin registry and loading system
- 📋 Configuration storage per plugin
- 📋 Metadata provider plugins (TMDb, MusicBrainz, TVDb, etc.)
- 📋 Notification plugins (Discord, Slack, Email)
- 📋 Webhook support

### Phase 7.4: Advanced Media Features
- 📋 Subtitle support (SRT, ASS, VTT)
- 📋 Multiple audio track selection
- 📋 Chapter markers
- 📋 Intro/outro detection and skipping
- 📋 Watch together / synchronized playback
- 📋 Chromecast / DLNA support

---

## Phase 8: Deployment & Production 📋 PLANNED (Future)

**Goal**: Production-ready deployment with monitoring and operations

**Status**: 📋 Not Started

### Phase 8.1: Docker & Deployment
- 📋 Docker images for backend and frontend
- 📋 Docker Compose for local deployment
- 📋 Kubernetes manifests for production
- 📋 Reverse proxy configuration (nginx, Traefik)
- 📋 SSL/TLS certificate management

### Phase 8.2: Monitoring & Operations
- 📋 Logging infrastructure (structured logs)
- 📋 Metrics collection (Prometheus)
- 📋 Alerting (critical errors, disk space)
- 📋 Health check endpoints
- 📋 Database backup and restore
- 📋 Automated database migrations

### Phase 8.3: Documentation
- 📋 User documentation (installation, usage)
- 📋 Admin documentation (configuration, maintenance)
- 📋 API documentation (complete with examples)
- 📋 Plugin development guide
- 📋 Troubleshooting guide

---

## Milestones

- ✅ **Milestone 1: Basic Media Server** (Nov 12, 2025) - Phase 1 Complete
  - Library management, media scanning, streaming
- ✅ **Milestone 2: Enhanced Viewing** (Nov 13, 2025) - Phase 2 Complete
  - Watch progress, transcoding, Continue Watching
- ✅ **Milestone 3: Full Media Types** (Nov 15, 2025) - Phase 3 Complete
  - TV shows, music support
- ✅ **Milestone 4: Rich Metadata** (Nov 18, 2025) - Phase 4 Complete
  - NFO parsing, image handling, ID3 metadata, caching
- ✅ **Milestone 5: Performance & UX** (Nov 20, 2025) - Phase 5 Mostly Complete
  - Pagination, infinite scroll, video player, performance optimizations
- 📋 **Milestone 6: Multi-User Support** (Future) - Phase 6 Pending
  - User authentication, per-user features
- 📋 **Milestone 7: Production Ready** (Future) - Phases 7-8 Pending
  - External APIs, plugins, deployment, monitoring

---

## Technical Context

### Current Architecture
- **Backend**: Go with Clean Architecture (Domain → Application → Infrastructure → API)
- **Frontend**: React with TanStack Query/Router
- **Database**: Dual support (SQLite for dev, PostgreSQL for prod)
- **Media Processing**: FFmpeg for transcoding, tag library for ID3
- **Image Processing**: WebP conversion with hash-based caching

### Technology Stack
- Go 1.21+, React 18+, TypeScript 5+
- SQLite 3 / PostgreSQL 15+
- FFmpeg 6.0+, Shaka Player, TanStack ecosystem
- Docker (future), Kubernetes (future)

---

## Next Steps

### Immediate Priority: Phase 6.1 - User Authentication System

**Goal**: Implement secure user authentication to enable multi-user support

**Why This Is Next**:
- Image handling is complete (Phase 4 ✅)
- Performance optimizations are done (Phase 5 ✅)
- Multi-user support is blocking for production deployment
- Authentication is a prerequisite for per-user features (watch progress isolation, ratings, preferences)

**Implementation Tasks**:
1. **Backend Authentication** (8-10 hours)
   - Create users table (migration)
   - Implement user entity and repository
   - Add JWT token generation and validation
   - Implement bcrypt password hashing
   - Create login/logout endpoints
   - Add authentication middleware
   - Update existing endpoints to use authenticated user context

2. **User Management API** (4-6 hours)
   - User registration endpoint
   - User profile endpoints (get, update)
   - Password change endpoint
   - User settings endpoints
   - Admin user management endpoints

3. **Frontend Authentication** (6-8 hours)
   - Login page with form validation
   - Registration page
   - Authentication context provider
   - Protected route wrapper
   - User profile page
   - Logout functionality
   - Token storage and refresh logic

4. **Watch Progress Migration** (2-3 hours)
   - Update watch_progress table to include user_id
   - Migrate existing progress to default user
   - Update progress tracking to use authenticated user
   - Update Continue Watching to be user-specific

**Estimated Timeline**: 20-27 hours (2.5-3.5 weeks part-time)

**Success Criteria**:
- ✅ Users can register and login
- ✅ JWT tokens securely generated and validated
- ✅ Protected API endpoints require authentication
- ✅ Frontend redirects unauthenticated users to login
- ✅ Watch progress is per-user
- ✅ Multiple users can use the system independently

### Short Term: Phase 6.2 - External Metadata APIs

**Goal**: Automatically enrich metadata using external APIs

**Why This Matters**:
- Reduces manual metadata management
- Improves media discovery with cast/crew info
- Downloads missing artwork automatically
- Industry-standard feature (Plex, Jellyfin, Emby all support this)

**Implementation Tasks**:
1. **TMDb Integration for Movies/TV Shows** (12-15 hours)
   - API client with rate limiting
   - Search by title + year matching
   - Download posters, backdrops, logos
   - Cast & crew metadata extraction
   - Store in external metadata tables
   - Background job for metadata refresh

2. **MusicBrainz Integration for Music** (8-10 hours)
   - API client with rate limiting
   - Artist and album matching
   - Cover Art Archive integration
   - fanart.tv or Last.fm for artist images
   - Background job for artwork download

3. **Manual Override UI** (6-8 hours)
   - Search and select correct match
   - Upload custom images
   - Set image priorities
   - Refresh/delete metadata

**Estimated Timeline**: 26-33 hours (3-4 weeks part-time)

### Medium Term: Phase 6.3 - User Experience Enhancements

**Priority**: After authentication and metadata APIs are complete

**Key Features**:
1. **Global Search** (8-10 hours)
   - Search across all media types
   - Fuzzy matching and relevance scoring
   - Search history and suggestions
   - Keyboard shortcut (Ctrl+K or /)

2. **Dark Mode** (4-6 hours)
   - Theme context and persistence
   - Dark color palette
   - Theme toggle UI
   - Smooth transitions

3. **Watchlists & Collections** (10-12 hours)
   - User-created collections
   - Watchlist functionality
   - Collection management UI
   - Share collections (future)

4. **Recommendations** (12-15 hours)
   - Based on watch history
   - Similar media suggestions
   - "Because you watched..." sections
   - Genre-based recommendations

**Estimated Timeline**: 34-43 hours (4-5 weeks part-time)

---

## Notes on Completed Work

### Phases 4 & 5 Completed Ahead of Schedule

**Phase 4** was originally estimated at 2-3 weeks but was completed in 4 days (Nov 15-18):
- Aggressive execution with minimal planning overhead
- Reused existing patterns from Phases 1-3
- Image handling system proved simpler than anticipated
- Embedded artwork extraction added as bonus feature

**Phase 5** was originally estimated at 4-6 weeks but core features completed in 3 days (Nov 18-20):
- Code quality refactoring (Phase 5.0) eliminated technical debt upfront
- TanStack Query integration was cleaner than expected
- Batch loading pattern solved N+1 queries elegantly
- Video player enhancements added as polish layer

**Backend Testing Infrastructure** (Nov 21):
- Not originally in project plan but critical for maintainability
- Centralized mock repositories eliminate duplicate test code
- 188+ test cases with 100% pass rate
- Sets foundation for robust future development

### Technical Decisions Made

1. **Virtual Entities for Music**: Keep aggregated artist/album pattern (ADR 012)
2. **Image Caching Strategy**: Hash-based sharding with WebP presets (ADR 006, 007)
3. **Authentication Approach**: JWT + bcrypt (standard, simple, secure)
4. **Metadata Priority**: Filesystem-first, embedded-fallback, external-optional (ADR 008)

### What's Working Well

- Clean Architecture pattern scales well with domain complexity
- TanStack Query handles client state elegantly
- Dual database support (SQLite/PostgreSQL) is seamless
- FFmpeg transcoding strategy (4-tier) handles most media correctly
- Image handling system is robust and performant

### Known Technical Debt

- ✅ Container refactoring (ADR 010) - Still pending but documented
- ✅ Transcode cleanup migration to unified scheduler (ADR 009) - Deferred
- ❌ Unit test coverage below 50% in some areas - Improved with Nov 21 testing work
- ❌ No integration tests for API endpoints yet
- ❌ Error handling could be more consistent

---

**For Detailed Implementation History**: See [ROADMAP.md](./ROADMAP.md)

**Last Updated**: November 21, 2025 (Updated to reflect Phase 4 & 5 completion, backend testing infrastructure overhaul, and updated next steps for Phase 6)
