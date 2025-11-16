# ViewRA Project Plan

## Current Status

**Phase**: Phase 4.1 Complete ✅ (November 16, 2025)
**Next**: Phase 4.2 - Task Scheduler & External APIs
**Current Features**: Image handling, NFO parsing, progress tracking, transcoding, cleanup
**Target MVP**: Phase 2 Complete ✅
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
- ✅ Policy-based cleanup (age, idle, failed jobs, orphans)
- ✅ Threshold-based LRU cleanup (disk usage monitoring)
- ✅ Configurable via environment variables (10+ settings)

### Success Criteria ✅ ALL MET
- ✅ Can resume videos from last watched position
- ✅ Videos >90% watched are auto-marked as "watched"
- ✅ Unsupported codecs trigger automatic transcoding
- ✅ Remux completes in < 5 minutes (10x realtime speed)
- ✅ Intelligent strategy selection (4-tier approach)
- ✅ Progressive streaming starts immediately
- ✅ Automatic cleanup prevents disk exhaustion
- ✅ LRU cleanup preserves frequently-used transcodes

### Implementation Summary

**Phase 2.1 - Watch Progress**: ✅ COMPLETE
- All backend layers implemented (Domain, Application, Infrastructure, API)
- Full frontend implementation with VideoPlayer and ContinueWatching
- Per-user progress tracking with auto-mark watched

**Phase 2.2 - On-Demand Transcoding**: ✅ COMPLETE
- 4-tier streaming strategy with codec detection
- Worker pool with idle timeout
- HLS progressive streaming
- Database access tracking for cleanup

**Phase 2.3 - Cleanup System**: ✅ COMPLETE
- CLI tool, API endpoints, automated scheduler
- Policy-based and threshold-based cleanup
- 10+ configurable environment variables
- Disk monitoring and LRU eviction

---

## Phase 3: TV Shows & Music ✅ COMPLETE

**Goal**: Full support for TV shows and music libraries with type-specific metadata

**Status**: ✅ Complete
**Started**: November 15, 2025
**Completed**: November 15, 2025
**Actual Effort**: 1 day

### Key Features

#### Phase 3.1: Movie Repository Implementation ✅ COMPLETE (Nov 15, 2025)
- ✅ Expanded Movie domain model with 20+ metadata fields (Cast, Genre, Plot, ContentRating, MaturityRating, ContentAdvisories, Budget, Revenue, OriginalLanguage, CountryOfOrigin, AwardsSummary, etc.)
- ✅ Implemented all MovieRepository methods (ListMoviesByGenre, ListMoviesByYear, DeleteMovie)
- ✅ Applied clean conversion pattern following Go best practices
- ✅ Created thin wrapper functions for each query type
- ✅ Single source of truth `toMovieDomain()` helper function

**Backend Components**: ✅ Complete
- ✅ Repository: Clean one-line conversions, dual-database support
- ✅ DTOs: Updated MovieResponse with all expanded fields
- ✅ Use Cases: GetMovie, ListMovies, SearchMovies
- ✅ API Handlers: GET /api/movies, GET /api/movies/:id, GET /api/movies/search
- ✅ Routes: Fully wired up in server

**Frontend Components**: ✅ Complete
- ✅ Movies page with grid view, search, and library filtering
- ✅ Updated TypeScript types with all metadata fields
- ✅ API client integration
- ✅ VideoPlayer integration for playback

#### Phase 3.2: TV Show Repository Implementation ✅ COMPLETE (Nov 15, 2025)
- ✅ Full three-level hierarchy support (Shows → Seasons → Episodes)
- ✅ Auto-creates show and season records when adding episodes
- ✅ Episode grouping and ordering by show/season
- ✅ Clean episodeFields pattern for type conversions
- ✅ Increment season episode count automatically

**Backend Components**: ✅ Complete
- ✅ Repository: TVRepository with show/season/episode methods
- ✅ Use Cases: ListTVShows, ListTVEpisodes, GetTVEpisode, SearchTVEpisodes
- ✅ API Handlers: Full TV show/episode endpoints
- ✅ Routes: Fully wired up in server

**Frontend Components**: ✅ Complete (via existing implementation)
- ✅ TV show pages (show grid → season view → episode list)
- ✅ TypeScript types for TV episodes
- ✅ API client integration

#### Phase 3.3: Music Repository Implementation ✅ COMPLETE (Nov 15, 2025)
- ✅ Full artist/album/track support
- ✅ Uses Go generics for clean row conversion (musicTrackRow interface)
- ✅ Methods: ListByLibrary, ListByAlbum, ListByArtist, SearchTracks
- ✅ Album/artist grouping logic

**Backend Components**: ✅ Complete
- ✅ Repository: MusicRepository with artist/album/track methods
- ✅ Use Cases: ListArtists, ListAlbums, ListTracks, GetTrack, SearchTracks
- ✅ API Handlers: Full music endpoints
- ✅ Routes: Fully wired up in server

**Frontend Components**: ✅ Complete (via existing implementation)
- ✅ Music pages (artist grid → albums → tracks)
- ✅ Audio player component integration
- ✅ TypeScript types for music tracks
- ✅ API client integration

### Success Criteria ✅ ALL MET
- ✅ TV shows parse correctly (S01E01, 1x01 formats) - Parser implemented in prior session
- ✅ Episodes group by show and season with proper ordering - Repository auto-creates hierarchy
- ✅ Music tracks ready for ID3 tag extraction - Repository supports all fields
- ✅ Can track watch progress per episode - Existing progress tracking works with all media types
- ✅ Audio player works for music files - Frontend component already integrated

### Implementation Summary

**Phase 3.1 - Movies**: ✅ COMPLETE
- Refactored repository with best-practice conversion patterns
- Expanded domain model and DTOs with comprehensive metadata
- Full API and frontend integration

**Phase 3.2 - TV Shows**: ✅ COMPLETE
- Three-level hierarchy (Shows → Seasons → Episodes)
- Auto-creation of show/season records
- Clean conversion patterns using episodeFields struct

**Phase 3.3 - Music**: ✅ COMPLETE
- Artist/album/track support with generics-based conversion
- Full API and frontend integration
- Ready for ID3 tag extraction in Phase 4

### Notes
- **Metadata Extraction Deferred**: NFO parsing, ID3 tag extraction, and external API integration (TMDb, MusicBrainz) are deferred to Phase 4
- **Database Fields Ready**: All metadata fields exist in the database schema and domain models, ready to be populated when parsers are implemented
- **Parsers Ready**: Filename parsers for movies/TV/music implemented in prior session
- **Build Status**: ✅ Backend compiles, ✅ Frontend builds, ✅ Binary builds successfully

---

## Phase 4: Enhanced Metadata ✅ PARTIAL (Started Nov 15, 2025)

**Goal**: Rich metadata from NFO files and external sources (TMDb, MusicBrainz)

**Status**: NFO & ID3 Integration Complete ✅, External APIs Pending 📋
**Estimated Effort**: 2-3 weeks total (1 week complete, 1-2 weeks remaining)

### Completed Features ✅
- ✅ **NFO Movie Parsing**: Full integration in scanner with 20+ metadata fields
- ✅ **NFO TV Episode Parsing**: Episode metadata with air dates, descriptions, IDs
- ✅ **Music ID3 Integration**: Clean architecture pattern with metadata extractor interface
- ✅ **Frontend Movie Cards**: Rich metadata display (plot, director, genre, year, badges)
- ✅ **Frontend TV Episode Cards**: Air dates, descriptions, IMDb/TVDb indicators
- ✅ **Frontend Music Cards**: Year, genre, bitrate badges with enhanced hover effects
- ✅ **Audio Codec Compatibility Fix** (Nov 15): Fixed AC3/DTS/TrueHD/FLAC audio transcoding
  - Updated `ShouldTranscode` validation to check audio codec compatibility
  - AC3, DTS, TrueHD, FLAC, EAC3 now properly detected and transcoded to AAC stereo
  - Multi-channel audio (5.1, 7.1) now properly downmixed to stereo
  - Remux-with-audio strategy working correctly for incompatible audio

### Pending Features 📋

#### Phase 4.1: Image Handling Infrastructure ✅ CORE COMPLETE (Nov 16, 2025)

**Status**: Core Cataloging Complete - Caching & Transformations Deferred to Phase 4.3

**Implementation Approach**: Hybrid phased rollout
- **Phase 4.1** (DONE): Catalog images by reference, serve from original paths
- **Phase 4.3** (FUTURE): Populate cache, add transformations

See [PHASE_4_1_GAP_ANALYSIS.md](./PHASE_4_1_GAP_ANALYSIS.md) for detailed analysis.

**Completed** ✅:
- ✅ **Planning**: ADR 006 (Image Handling Strategy) + ADR 007 (Unified Task Scheduler)
- ✅ **Database**: Migration 000007 - polymorphic `media_images` table with CASCADE deletes
- ✅ **Domain Layer**: Entity, repository interface, type-safe enums (13 image types, 6 sources)
- ✅ **SQLC Queries**: 18 queries for CRUD, cleanup, hash lookup, deduplication
- ✅ **Repository**: Full implementation with dual-database support, hash-based operations
- ✅ **Image Extraction**: Metadata extractor (dimensions, SHA256, MIME), Kodi/Plex file detector
- ✅ **Application Layer**: Use cases for get/extract/cleanup images (movies, TV episodes, music albums)
- ✅ **Lifecycle Management**: CASCADE deletion, cleanup use case with graceful degradation
- ✅ **API Layer**: 4 endpoints (GetImage, ServeImage, GetMediaImages, GetMovieImages/EpisodeImages)
- ✅ **Scanner Integration**: Automatic image extraction during library scans (all media types)
- ✅ **Frontend**: TypeScript types, API client, MediaPoster component, updated MediaCard/MovieCard
- ✅ **HTTP Caching**: 1-year Cache-Control headers with ETags
- ✅ **Documentation**: Gap analysis, lifecycle hooks, cleanup strategy
- ✅ **Image Serving**: Direct serving from original file paths (no cache yet)

**Deferred to Phase 4.3** 📋:
- 📋 **Image Cache Population**: Copy images to `data/cache/images/` with hash-based filenames
- 📋 **Image Deduplication**: Share cache files across identical images (by hash)
- 📋 **Image Transformations**: On-demand resizing (`?width=300`), WebP conversion (`?format=webp`)
- 📋 **Cache Service**: CacheService for copying and transforming images
- 📋 **LRU Eviction**: Disk space monitoring and cache eviction

**Deferred to Phase 4.2** 📋:
- 📋 **Scheduler Integration**: Register cleanup task with unified scheduler
- 📋 **External APIs**: TMDb/MusicBrainz image downloads
- 📋 **Unit Tests**: Image extraction and cleanup logic

**Asset Survey Results**:
- Movies: 2,155 posters, 2,153 fanart, 522 clearlogos, 234 landscapes, 412 actor images, 370 extra thumbs
- TV Shows: Show/season posters, 25,751+ episode thumbnails
- Music: 5,653+ album covers, disc art, artist fanart
- **Total**: ~36,000+ existing image assets ready to catalog

#### Phase 4.2: Task Scheduler & External APIs 📋 (Week 2)

**Scheduler System** (ADR 007):
- 📋 **Unified Scheduler**: Cron-based task scheduler with admin API
- 📋 **Image Cache Cleanup**: Daily orphan detection and removal (3 AM default)
- 📋 **Transcode Cleanup**: Migrate existing cleanup to unified scheduler
- 📋 **Future Tasks**: DB vacuum, library health checks, log rotation
- 📋 **Admin UI**: Task management, manual triggers, execution history

**External API Integration**:
- 📋 **TMDb Integration**: Movie/TV search and details, poster/backdrop downloads, cast and crew information
- 📋 **MusicBrainz Integration**: Artist/album search, track matching, cover art fetching
- 📋 **Image Download**: Fetch missing images from external sources, cache locally
- 📋 **Manual Management**: Upload custom images, set priorities, override matches

#### Phase 4.3: Advanced Features 📋 (Optional)
- 📋 **Plugin System**: Plugin registry, plugin manager, loading/unloading, configuration storage, plugin SDK interfaces
- 📋 **Rich Metadata**: Collections support (MCU, Star Wars), people/credits entities, manual metadata matching UI
- 📋 **Image Transformations**: On-demand resizing, format conversion (WebP), quality control
- 📋 **Image Cleanup**: LRU eviction for downloaded images, orphan detection

### Success Criteria

**Metadata Extraction** ✅
- ✅ NFO files automatically detected and parsed during library scan
- ✅ Movie metadata populated from .nfo files (plot, director, cast, genre, year, ratings)
- ✅ TV episode metadata populated from episode.nfo files (air dates, descriptions, IDs)
- ✅ Music metadata extracted via ID3 tags (year, genre, bitrate, artist, album)
- ✅ Frontend displays rich metadata for all media types
- ✅ Audio codec compatibility properly detected (AC3, DTS, TrueHD, FLAC, multi-channel)

**Image Handling** ✅ (Core Complete, Caching Deferred)
- ✅ Scanner detects and catalogs local images (posters, fanart, logos, thumbnails)
- ✅ Movie posters display in frontend from local `poster.jpg` files
- ✅ TV episode thumbnails display from `*-thumb.jpg` files
- ✅ Music album covers display from `folder.jpg` files
- ✅ Images served with proper caching headers (1 year TTL)
- ✅ API can fetch images by media ID
- 📋 Image resizing and WebP conversion (deferred to Phase 4.3)
- 📋 Hash-based cache deduplication (deferred to Phase 4.3)

**External Enrichment** 📋
- 📋 TMDb correctly identifies movies by title + year
- 📋 TMDb posters and backdrops download successfully for missing images
- 📋 MusicBrainz cover art downloads for albums without local artwork
- 📋 Cast and crew display on media detail pages
- 📋 Collections group related movies
- 📋 Can manually override incorrect matches and upload custom images

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
**Status**: ✅ COMPLETE (Phases 1 & 2)

### Feature Complete
**Target**: Phase 5 Complete
**Features**: All media types (movies/TV/music), external metadata, multi-user support
**Status**: Phase 3 Complete ✅ (Media types done), Phase 4 Pending (Metadata extraction)

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

### Immediate (Phase 4.1 - Image Handling Infrastructure)

**See [ADR 006: Image Handling Strategy](./decisions/006-image-handling-strategy.md) for complete design.**

1. Create database migration for `media_images` table
   - Polymorphic design: supports movies, TV shows/seasons/episodes, music artists/albums/tracks
   - Track local files and downloaded images separately
   - Priority system for multiple images per type
   - Store paths (not binary data) for efficient serving

2. Implement `ImageExtractor` service
   - Detect standard Kodi/Plex image naming (poster.jpg, fanart.jpg, clearlogo.png, etc.)
   - Support movie-specific: `.actors/`, `extrathumbs/` folders
   - Support TV-specific: show/season posters, episode thumbnails (`*-thumb.jpg`)
   - Support music-specific: folder.jpg, discart.png, artist fanart
   - Extract image metadata (dimensions, size, MIME type)

3. Integrate image extraction into library scanner
   - Call `ImageExtractor` during movie/TV/music processing
   - Store image metadata in `media_images` table
   - Associate images with correct media/entity IDs
   - Make extraction async/optional to avoid slowing scans

4. Implement image serving API endpoints
   - `GET /api/images/:id/file` - Serve image with caching headers
   - `GET /api/media/:id/images` - Get all images for media item
   - `GET /api/movies/:id/images` - Movie-specific images
   - `GET /api/tv/episodes/:id/images` - Episode thumbnails
   - `GET /api/music/albums/:name/images` - Album artwork
   - Support query params: `?width=300&height=450` (optional resizing)

5. Update frontend to display images
   - MovieCard: Display poster, fanart background, clearlogo overlay
   - TVEpisodeCard: Display episode thumbnails
   - MusicAlbumCard: Display album covers and disc art
   - Implement loading states and placeholder images
   - Gracefully handle missing images (404)

### Short Term (Phase 4.2 - External APIs & Image Enrichment)
1. Implement TMDb integration for movies/TV shows
   - Search API: Match movies by title + year
   - Image API: Download posters, backdrops, logos
   - Cast & crew API: Populate people metadata
   - Store downloaded images in `data/images/tmdb/`
   - Track external_url and local_cache_path in media_images

2. Implement MusicBrainz integration for music
   - Search API: Match artists and albums
   - Cover Art Archive: Download album covers
   - Artist images from fanart.tv or Last.fm
   - Store in `data/images/musicbrainz/`

3. Add manual metadata management UI
   - Upload custom images for media items
   - Set priority for multiple images of same type
   - Override incorrect TMDb/MusicBrainz matches
   - Delete/refresh images

4. Design plugin architecture for metadata providers (optional)
   - Plugin SDK interfaces
   - Plugin registry and loading system
   - Configuration storage per plugin

### Medium Term (Phase 5 - User Features)
1. Implement user authentication (JWT + bcrypt)
2. Add per-user watch progress and ratings
3. Build search across all media types
4. Implement dark mode
5. Add keyboard shortcuts

---

**For Detailed Implementation History**: See [ROADMAP.md](./ROADMAP.md)

**Last Updated**: 2025-11-16 (Reality Check: Phase 4.1 Core Complete, Caching Deferred to Phase 4.3)
